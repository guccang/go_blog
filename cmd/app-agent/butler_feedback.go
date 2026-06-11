package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 养成度（亲密度）默认值与反馈调参权重（对应 plan §7 养成系统 + §7.4 反馈闭环）。
const (
	butlerAffinityStart = 50
	butlerAffinityMin   = 0
	butlerAffinityMax   = 100

	butlerDeltaHelpful   = 3  // 「有帮助」正反馈
	butlerDeltaSeen      = 1  // 播报被看到（轻微养成）
	butlerDeltaTooFreq   = -5 // 「太频繁」负反馈
	butlerDeltaBadTiming = -4 // 「时机不对」负反馈
)

// butlerFeedbackKinds 反馈类型 → 中文标签（用于写入记忆，便于用户审阅与判官评审）。
var butlerFeedbackKinds = map[string]string{
	"helpful":      "有帮助",
	"too_frequent": "太频繁",
	"bad_timing":   "时机不对",
	"seen":         "已查看",
}

// ButlerAffinity 单账号养成度状态（持久化，跨会话累积，避免“交互一百次和第一次没区别”）。
type ButlerAffinity struct {
	Score          int   `json:"score"`
	Helpful        int   `json:"helpful"`
	TooFrequent    int   `json:"too_frequent"`
	BadTiming      int   `json:"bad_timing"`
	BroadcastsSeen int   `json:"broadcasts_seen"`
	UpdatedAt      int64 `json:"updated_at"`
}

func defaultButlerAffinity() ButlerAffinity {
	return ButlerAffinity{Score: butlerAffinityStart, UpdatedAt: time.Now().UnixMilli()}
}

func clampAffinityScore(v int) int {
	if v < butlerAffinityMin {
		return butlerAffinityMin
	}
	if v > butlerAffinityMax {
		return butlerAffinityMax
	}
	return v
}

// affinityDeltaForKind 返回某反馈类型对养成度的增量。
func affinityDeltaForKind(kind string) int {
	switch kind {
	case "helpful":
		return butlerDeltaHelpful
	case "seen":
		return butlerDeltaSeen
	case "too_frequent":
		return butlerDeltaTooFreq
	case "bad_timing":
		return butlerDeltaBadTiming
	default:
		return 0
	}
}

// applyButlerFeedback 在一份养成度状态上应用一次反馈，返回新状态（纯函数，便于测试）。
func applyButlerFeedback(cur ButlerAffinity, kind string) ButlerAffinity {
	out := cur
	switch kind {
	case "helpful":
		out.Helpful++
	case "too_frequent":
		out.TooFrequent++
	case "bad_timing":
		out.BadTiming++
	case "seen":
		out.BroadcastsSeen++
	}
	out.Score = clampAffinityScore(out.Score + affinityDeltaForKind(kind))
	out.UpdatedAt = time.Now().UnixMilli()
	return out
}

// ButlerAffinityStore 按 account 持久化养成度（镜像 AppPreferencesStore 模式）。
type ButlerAffinityStore struct {
	mu    sync.RWMutex
	path  string
	items map[string]ButlerAffinity
}

func NewButlerAffinityStore(path string) *ButlerAffinityStore {
	store := &ButlerAffinityStore{
		path:  strings.TrimSpace(path),
		items: make(map[string]ButlerAffinity),
	}
	store.load()
	return store
}

func (s *ButlerAffinityStore) Get(account string) ButlerAffinity {
	account = strings.TrimSpace(account)
	if s == nil || account == "" {
		return defaultButlerAffinity()
	}
	s.mu.RLock()
	cur, ok := s.items[account]
	s.mu.RUnlock()
	if ok {
		return cur
	}
	return defaultButlerAffinity()
}

// Apply 应用一次反馈并持久化，返回新状态。
func (s *ButlerAffinityStore) Apply(account, kind string) ButlerAffinity {
	account = strings.TrimSpace(account)
	if s == nil || account == "" {
		return defaultButlerAffinity()
	}
	s.mu.Lock()
	cur, ok := s.items[account]
	if !ok {
		cur = defaultButlerAffinity()
	}
	next := applyButlerFeedback(cur, kind)
	s.items[account] = next
	s.mu.Unlock()
	s.save()
	return next
}

func (s *ButlerAffinityStore) load() {
	if s == nil || s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil || len(data) == 0 {
		return
	}
	var raw map[string]ButlerAffinity
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for account, item := range raw {
		account = strings.TrimSpace(account)
		if account == "" {
			continue
		}
		item.Score = clampAffinityScore(item.Score)
		s.items[account] = item
	}
}

func (s *ButlerAffinityStore) save() {
	if s == nil || s.path == "" {
		return
	}
	s.mu.RLock()
	data, err := json.MarshalIndent(s.items, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, append(data, '\n'), 0644)
}

type butlerFeedbackRequest struct {
	UserID      string `json:"user_id"`
	BroadcastID string `json:"broadcast_id"`
	Kind        string `json:"kind"`
	Text        string `json:"text"`
}

// HandleButlerFeedback 处理播报反馈（plan §7.4 反馈闭环）。
// POST /api/app/butler/feedback {user_id, broadcast_id, kind, text}
// 效果：更新养成度 + 把反馈写入记忆（journal 总记；负反馈追加 MEMORY，
// 经 cortana 快照注入与每周判官回写，自然收敛主动行为——“管家”而非“闹钟”）。
func (h *Handler) HandleButlerFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req butlerFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	account := strings.TrimSpace(req.UserID)
	if account == "" || !h.validateAppSession(r, account) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	kind := strings.TrimSpace(strings.ToLower(req.Kind))
	label, ok := butlerFeedbackKinds[kind]
	if !ok {
		writeButlerError(w, http.StatusBadRequest, fmt.Sprintf("unknown feedback kind: %s", req.Kind))
		return
	}

	affinity := h.affinity.Apply(account, kind)
	h.writeFeedbackMemory(account, kind, label, req.BroadcastID, req.Text)
	h.syncAffinityToCompanionState(account, affinity.Score)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"affinity": affinity,
	})
}

// HandleButlerAffinity 返回当前养成度。GET /api/app/butler/affinity?user_id=
func (h *Handler) HandleButlerAffinity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	account := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if account == "" || !h.validateAppSession(r, account) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"affinity": h.affinity.Get(account),
	})
}

// writeFeedbackMemory 把反馈写入用户记忆（best-effort，失败不影响反馈本身）。
func (h *Handler) writeFeedbackMemory(account, kind, label, broadcastID, text string) {
	if h.affinity == nil {
		return
	}
	snippet := strings.TrimSpace(text)
	if rs := []rune(snippet); len(rs) > 40 {
		snippet = string(rs[:40]) + "…"
	}
	journal := fmt.Sprintf("[播报反馈:%s] 对播报「%s」", label, snippet)
	if _, _, err := h.callBlogAgentTool(account, "RawMemoryJournal", map[string]any{
		"account": account,
		"content": journal,
	}); err != nil {
		log.Printf("[Butler] 写反馈 journal 失败 account=%s err=%v", account, err)
	}

	// 负反馈追加 MEMORY，让 cortana 决策与每周判官长期据此收敛。
	if kind == "too_frequent" || kind == "bad_timing" {
		note := fmt.Sprintf("用户对管家主动播报反馈“%s”（播报:%s）：%s", label, defaultButlerText(broadcastID), snippet)
		if _, _, err := h.callBlogAgentTool(account, "RawMemoryAppend", map[string]any{
			"account": account,
			"file":    "MEMORY",
			"content": note,
		}); err != nil {
			log.Printf("[Butler] 写反馈 MEMORY 失败 account=%s err=%v", account, err)
		}
	}
}

func defaultButlerText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "未知"
	}
	return s
}

// syncAffinityToCompanionState 把养成度写入 llm-agent 工作区的 companion_state（plan §7.3：养成度存 companion_state）。
// 读-改-写，只更新 affinity 字段，保留 llm-agent 写入的其它字段；best-effort，失败不影响反馈。
func (h *Handler) syncAffinityToCompanionState(account string, score int) {
	baseDir := strings.TrimSpace(h.cfg.LLMWorkspaceDir)
	account = strings.TrimSpace(account)
	if baseDir == "" || account == "" {
		return
	}
	path := filepath.Join(baseDir, "users", account, "memory", "cortana_companion_state.json")
	state := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &state)
	}
	state["affinity"] = score
	state["updated_at"] = time.Now().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Printf("[Butler] companion_state 目录创建失败 account=%s err=%v", account, err)
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		log.Printf("[Butler] 写 companion_state 养成度失败 account=%s err=%v", account, err)
	}
}
