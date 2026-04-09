package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ========================= 数据模型 =========================

// TaskSession 任务会话（对标 OpenClaw 的 Agent Session）
type TaskSession struct {
	// 身份标识
	ID       string `json:"id"`        // 唯一会话ID
	ParentID string `json:"parent_id"` // 父会话ID（根任务为空）
	RootID   string `json:"root_id"`   // 根任务会话ID
	Depth    int    `json:"depth"`     // 层级深度 (0=根)

	// 任务信息
	Title       string `json:"title"`
	Description string `json:"description"`
	Account     string `json:"account"`
	Source      string `json:"source,omitempty"`

	// 执行状态
	Status     string     `json:"status"` // pending/running/done/failed/skipped
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`

	// 对话历史
	Messages []Message `json:"messages"`

	// 工具调用记录
	ToolCalls []ToolCallRecord `json:"tool_calls"`

	// 结果
	Result  string `json:"result,omitempty"`
	Summary string `json:"summary,omitempty"`

	mu sync.Mutex `json:"-"` // 并发保护
}

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	ID         string    `json:"id"`
	ToolName   string    `json:"tool_name"`
	Arguments  string    `json:"arguments"`
	Result     string    `json:"result"`
	Success    bool      `json:"success"`
	DurationMs int64     `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
	Iteration  int       `json:"iteration"` // 第几轮 agentic loop
}

// SessionIndex 索引文件结构（用于列表页）
type SessionIndex struct {
	RootID        string              `json:"root_id"`
	Title         string              `json:"title"`
	Account       string              `json:"account"`
	Status        string              `json:"status"`
	CreatedAt     time.Time           `json:"created_at"`
	FinishedAt    *time.Time          `json:"finished_at,omitempty"`
	TotalSessions int                 `json:"total_sessions"`
	DoneCount     int                 `json:"done_count"`
	FailedCount   int                 `json:"failed_count"`
	SkippedCount  int                 `json:"skipped_count"`
	AsyncCount    int                 `json:"async_count"`
	DeferredCount int                 `json:"deferred_count"`
	Children      []SessionIndexChild `json:"children"`
}

// SessionIndexChild 索引中的子任务摘要
type SessionIndexChild struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
}

// ========================= 会话构造器 =========================

// newSessionID 生成 8 位十六进制 ID
func newSessionID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// NewRootSession 创建根任务会话
func NewRootSession(taskID, title, account string) *TaskSession {
	now := time.Now()
	id := taskID // 使用 gateway 分配的 taskID 作为 rootSession ID
	return &TaskSession{
		ID:        id,
		RootID:    id,
		Depth:     0,
		Title:     title,
		Account:   account,
		Status:    "running",
		StartedAt: &now,
		Messages:  make([]Message, 0),
		ToolCalls: make([]ToolCallRecord, 0),
	}
}

// NewChildSession 创建子任务会话
func NewChildSession(parent *TaskSession, title, description string) *TaskSession {
	id := newSessionID()
	return &TaskSession{
		ID:          id,
		ParentID:    parent.ID,
		RootID:      parent.RootID,
		Depth:       parent.Depth + 1,
		Title:       title,
		Description: description,
		Account:     parent.Account,
		Source:      parent.Source,
		Status:      "pending",
		Messages:    make([]Message, 0),
		ToolCalls:   make([]ToolCallRecord, 0),
	}
}

// ========================= 会话操作方法 =========================

// AppendMessage 追加消息到会话（线程安全）
func (s *TaskSession) AppendMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, msg)
}

// RecordToolCall 记录工具调用（线程安全）
func (s *TaskSession) RecordToolCall(record ToolCallRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolCalls = append(s.ToolCalls, record)
}

// SetStatus 设置状态
func (s *TaskSession) SetStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	if status == "running" && s.StartedAt == nil {
		now := time.Now()
		s.StartedAt = &now
	}
	if status == "done" || status == "failed" || status == "skipped" || status == "async" || status == "deferred" {
		now := time.Now()
		s.FinishedAt = &now
	}
}

// SetError 设置错误信息
func (s *TaskSession) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Error = err
}

// SetResult 设置结果
func (s *TaskSession) SetResult(result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Result = result
}

// DurationMs 计算执行耗时（毫秒）
func (s *TaskSession) DurationMs() int64 {
	if s.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if s.FinishedAt != nil {
		end = *s.FinishedAt
	}
	return end.Sub(*s.StartedAt).Milliseconds()
}

// ========================= 持久化存储 =========================

// SessionStore 会话存储（本地文件系统）
type SessionStore struct {
	baseDir string
}

// NewSessionStore 创建会话存储
func NewSessionStore(baseDir string) *SessionStore {
	return &SessionStore{baseDir: baseDir}
}

// Save 保存单个会话到文件
func (s *SessionStore) Save(session *TaskSession) error {
	session.mu.Lock()
	data, err := json.MarshalIndent(session, "", "  ")
	session.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal session: %v", err)
	}

	dir := filepath.Join(s.baseDir, session.RootID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir %s: %v", dir, err)
	}

	path := filepath.Join(dir, fmt.Sprintf("session_%s.json", session.ID))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %v", err)
	}

	log.Printf("[SessionStore] saved session %s (root=%s, status=%s)", session.ID, session.RootID, session.Status)
	return nil
}

// Load 加载单个会话
func (s *SessionStore) Load(rootID, sessionID string) (*TaskSession, error) {
	path := filepath.Join(s.baseDir, rootID, fmt.Sprintf("session_%s.json", sessionID))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %v", err)
	}

	var session TaskSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %v", err)
	}

	return &session, nil
}

// SaveIndex 保存索引文件（用于列表页展示）
func (s *SessionStore) SaveIndex(root *TaskSession, children []*TaskSession) error {
	index := SessionIndex{
		RootID:  root.RootID,
		Title:   root.Title,
		Account: root.Account,
		Status:  root.Status,
	}

	if root.StartedAt != nil {
		index.CreatedAt = *root.StartedAt
	}
	index.FinishedAt = root.FinishedAt

	index.TotalSessions = 1 + len(children)

	for _, child := range children {
		switch child.Status {
		case "done":
			index.DoneCount++
		case "failed":
			index.FailedCount++
		case "skipped":
			index.SkippedCount++
		case "async":
			index.AsyncCount++
		case "deferred":
			index.DeferredCount++
		}

		index.Children = append(index.Children, SessionIndexChild{
			ID:         child.ID,
			Title:      child.Title,
			Status:     child.Status,
			DurationMs: child.DurationMs(),
		})
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %v", err)
	}

	dir := filepath.Join(s.baseDir, root.RootID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %v", err)
	}

	path := filepath.Join(dir, "index.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write index: %v", err)
	}

	log.Printf("[SessionStore] saved index for root=%s (%d children)", root.RootID, len(children))
	return nil
}

// ListSessions 列出所有会话索引（按创建时间倒序）
func (s *SessionStore) ListSessions() ([]SessionIndex, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %v", err)
	}

	var indices []SessionIndex
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		indexPath := filepath.Join(s.baseDir, entry.Name(), "index.json")
		data, err := os.ReadFile(indexPath)
		if err != nil {
			continue // 跳过无索引的目录
		}
		var index SessionIndex
		if err := json.Unmarshal(data, &index); err != nil {
			continue
		}
		indices = append(indices, index)
	}

	// 按创建时间倒序
	sort.Slice(indices, func(i, j int) bool {
		return indices[i].CreatedAt.After(indices[j].CreatedAt)
	})

	return indices, nil
}

// ListRunningSessions 扫描所有 index.json，返回 status=="running" 的 rootID 列表
func (s *SessionStore) ListRunningSessions() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %v", err)
	}

	var runningIDs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		indexPath := filepath.Join(s.baseDir, entry.Name(), "index.json")
		data, err := os.ReadFile(indexPath)
		if err != nil {
			continue
		}
		var index SessionIndex
		if err := json.Unmarshal(data, &index); err != nil {
			continue
		}
		if index.Status == "running" {
			runningIDs = append(runningIDs, index.RootID)
		}
	}

	return runningIDs, nil
}
