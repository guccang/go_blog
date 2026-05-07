package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type CortanaCompanionState struct {
	CurrentGoal      string    `json:"current_goal,omitempty"`
	LastTopic        string    `json:"last_topic,omitempty"`
	LastUserMood     string    `json:"last_user_mood,omitempty"`
	LastUserMessage  string    `json:"last_user_message,omitempty"`
	LastAssistantMsg string    `json:"last_assistant_msg,omitempty"`
	NextFocus        string    `json:"next_focus,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func cortanaCompanionStatePath(baseDir, account string) string {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(account) == "" {
		return ""
	}
	return filepath.Join(GetAccountWorkspace(baseDir, account), "memory", "cortana_companion_state.json")
}

func loadCortanaCompanionState(baseDir, account string) *CortanaCompanionState {
	path := cortanaCompanionStatePath(baseDir, account)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var state CortanaCompanionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return normalizeCortanaCompanionState(&state)
}

func saveCortanaCompanionState(baseDir, account string, state *CortanaCompanionState) error {
	path := cortanaCompanionStatePath(baseDir, account)
	if path == "" || state == nil {
		return nil
	}
	state = normalizeCortanaCompanionState(state)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func normalizeCortanaCompanionState(state *CortanaCompanionState) *CortanaCompanionState {
	if state == nil {
		return nil
	}
	out := *state
	out.CurrentGoal = strings.TrimSpace(out.CurrentGoal)
	out.LastTopic = strings.TrimSpace(out.LastTopic)
	out.LastUserMood = strings.TrimSpace(out.LastUserMood)
	out.LastUserMessage = strings.TrimSpace(out.LastUserMessage)
	out.LastAssistantMsg = strings.TrimSpace(out.LastAssistantMsg)
	out.NextFocus = strings.TrimSpace(out.NextFocus)
	out.Summary = strings.TrimSpace(out.Summary)
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = time.Now()
	}
	return &out
}

func buildCortanaCompanionPrompt(state *CortanaCompanionState) string {
	var sb strings.Builder
	sb.WriteString("\n## Cortana 陪伴模式\n")
	sb.WriteString("- 当前渠道是 Cortana 陪伴对话。你的首要目标是让交流自然、连续、像熟悉用户近况的真人朋友，而不是每轮都像全新客服。\n")
	sb.WriteString("- 优先承接上轮的目标、情绪、未完成事项和你已经给出的建议；用户只说短句时，默认是在延续刚才的话题。\n")
	sb.WriteString("- 以下陪伴状态是历史摘要，只用于理解连续话题；如果里面出现早上、上午、中午、下午、晚上、深夜、凌晨、今天、明天、待会等相对时间词，不得用它判断当前时段，当前时段只能看本轮系统提示里的 `当前时间`。\n")
	sb.WriteString("- 少用模板化开场，避免重复自我介绍、重复总结背景、重复说教。\n")
	sb.WriteString("- 回复要口语化、具体、克制，像身边懂情况的人继续接话。\n")
	if state == nil {
		sb.WriteString("- 当前还没有稳定的陪伴状态，先自然建立上下文，并在回复里留下可延续的下一步。\n")
		return sb.String()
	}
	sb.WriteString("- 以下是当前已知的连续对话状态，请优先承接这些信息，不要丢掉主线。\n")
	if state.CurrentGoal != "" {
		sb.WriteString(fmt.Sprintf("- 当前主线目标: %s\n", state.CurrentGoal))
	}
	if state.LastTopic != "" {
		sb.WriteString(fmt.Sprintf("- 最近话题: %s\n", state.LastTopic))
	}
	if state.LastUserMood != "" {
		sb.WriteString(fmt.Sprintf("- 用户最近状态: %s\n", state.LastUserMood))
	}
	if state.NextFocus != "" {
		sb.WriteString(fmt.Sprintf("- 建议延续的下一步: %s\n", state.NextFocus))
	}
	if state.Summary != "" {
		sb.WriteString(fmt.Sprintf("- 连续性摘要: %s\n", state.Summary))
	}
	return sb.String()
}

func updateCortanaCompanionState(prev *CortanaCompanionState, userText, assistantText string) *CortanaCompanionState {
	state := normalizeCortanaCompanionState(prev)
	if state == nil {
		state = &CortanaCompanionState{}
	}

	userText = strings.TrimSpace(userText)
	assistantText = strings.TrimSpace(assistantText)
	if userText != "" {
		state.LastUserMessage = shortCortanaStateText(userText, 80)
		if goal := inferCortanaGoal(userText); goal != "" {
			state.CurrentGoal = goal
		}
		if topic := inferCortanaTopic(userText); topic != "" {
			state.LastTopic = topic
		}
		state.LastUserMood = inferCortanaMood(userText)
	}
	if assistantText != "" {
		state.LastAssistantMsg = shortCortanaStateText(assistantText, 100)
		if next := inferCortanaNextFocus(assistantText); next != "" {
			state.NextFocus = next
		}
	}
	state.UpdatedAt = time.Now()
	state.Summary = buildCortanaStateSummary(state)
	return normalizeCortanaCompanionState(state)
}

func inferCortanaGoal(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	taskMarkers := []string{"帮我", "需要", "想", "打算", "准备", "计划", "下一步", "怎么", "如何", "今天", "待会", "提醒", "安排"}
	for _, marker := range taskMarkers {
		if strings.Contains(lower, marker) || strings.Contains(text, marker) {
			return shortCortanaStateText(text, 60)
		}
	}
	return ""
}

func inferCortanaTopic(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return shortCortanaStateText(firstSentence(text), 36)
}

func inferCortanaMood(text string) string {
	switch {
	case cortanaContainsAny(text, "累", "烦", "焦虑", "难受", "崩", "麻了", "头疼", "压力", "困"):
		return "有点累或有压力"
	case cortanaContainsAny(text, "开心", "高兴", "顺利", "搞定", "不错", "兴奋"):
		return "状态偏积极"
	case cortanaContainsAny(text, "急", "马上", "来不及", "赶紧", "尽快"):
		return "比较着急"
	default:
		return "状态平稳"
	}
}

func inferCortanaNextFocus(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := splitSentences(text)
	for _, line := range lines {
		if cortanaContainsAny(line, "下一步", "先", "建议", "不如", "可以先", "先把", "先去", "先做") {
			return shortCortanaStateText(line, 48)
		}
	}
	if len(lines) > 0 {
		return shortCortanaStateText(lines[0], 48)
	}
	return ""
}

func buildCortanaStateSummary(state *CortanaCompanionState) string {
	if state == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if state.CurrentGoal != "" {
		parts = append(parts, "当前目标是"+state.CurrentGoal)
	}
	if state.LastTopic != "" && state.LastTopic != state.CurrentGoal {
		parts = append(parts, "最近在聊"+state.LastTopic)
	}
	if state.LastUserMood != "" {
		parts = append(parts, "用户状态"+state.LastUserMood)
	}
	if state.NextFocus != "" {
		parts = append(parts, "下一步可接"+state.NextFocus)
	}
	return strings.Join(parts, "；")
}

func shortCortanaStateText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 {
		return ""
	}
	rs := []rune(text)
	if len(rs) <= limit {
		return text
	}
	return string(rs[:limit]) + "..."
}

func firstSentence(text string) string {
	for _, sep := range []string{"。", "！", "？", "\n", ".", "!", "?"} {
		if idx := strings.Index(text, sep); idx >= 0 {
			return strings.TrimSpace(text[:idx])
		}
	}
	return strings.TrimSpace(text)
}

func splitSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '。', '！', '？', '\n', '.', '!', '?':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func cortanaContainsAny(text string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

func isShortCortanaUtterance(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && utf8.RuneCountInString(text) <= 12
}
