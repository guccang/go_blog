package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// TaskSummary 任务总结
type TaskSummary struct {
	FilesWritten  []string
	FilesEdited   []string
	FilesRead     []string
	CommandsRun   []string
	ResultText    string // 最终结果文本（Claude Code result / OpenCode 最后一段 assistant）
	AssistantText string // 累积的 assistant 文本输出
	TotalCost     float64
	TotalTokens   int
	TokensIn      int
	Duration      float64
	NumTurns      int
}

// UpdateFromEvent 从事件更新总结
func (s *TaskSummary) UpdateFromEvent(event *StreamEvent) {
	if event == nil {
		return
	}

	// 收集费用和 token 统计
	if event.CostUSD > 0 {
		s.TotalCost += event.CostUSD
	}
	if event.TokensOut > 0 {
		s.TotalTokens += event.TokensOut
	}
	if event.TokensIn > 0 {
		s.TokensIn += event.TokensIn
	}
	if event.Duration > 0 {
		s.Duration = event.Duration // 取最后一次（result 事件的是总时长）
	}
	if event.NumTurns > 0 {
		s.NumTurns = event.NumTurns
	}

	// 收集 AI 的文本输出（assistant 类型）
	if event.Type == "assistant" && event.Text != "" {
		s.AssistantText += event.Text + "\n"
	}

	// 收集最终结果文本（Claude Code 的 result 事件）
	if event.Type == "result" && event.Text != "" {
		s.ResultText = event.Text
	}

	// 收集工具调用信息
	if event.ToolName != "" && event.ToolInput != "" {
		var input map[string]interface{}
		json.Unmarshal([]byte(event.ToolInput), &input)
		switch event.ToolName {
		case "write", "Write":
			if fp := extractFilePath(input); fp != "" {
				s.FilesWritten = append(s.FilesWritten, fp)
			}
		case "edit", "Edit", "edit_file":
			if fp := extractFilePath(input); fp != "" {
				s.FilesEdited = append(s.FilesEdited, fp)
			}
		case "read", "Read", "read_file":
			if fp := extractFilePath(input); fp != "" {
				s.FilesRead = append(s.FilesRead, fp)
			}
		case "bash", "Bash", "run_command":
			if cmd, ok := input["command"].(string); ok && cmd != "" {
				s.CommandsRun = append(s.CommandsRun, cmd)
			}
		}
	}
}

// extractFilePath 从工具输入中提取文件路径
func extractFilePath(input map[string]interface{}) string {
	for _, key := range []string{"file_path", "filePath", "path", "file"} {
		if fp, ok := input[key].(string); ok && fp != "" {
			return fp
		}
	}
	return ""
}

// GenerateReport 生成总结报告
func (s *TaskSummary) GenerateReport() string {
	var lines []string
	lines = append(lines, "📋 任务完成报告")
	lines = append(lines, "━━━━━━━━━━━━━━━━")

	// 最重要：展示 AI 完成了什么（result 或 assistant 的最后内容）
	resultText := s.ResultText
	if resultText == "" {
		// 没有 result 事件（如 OpenCode），使用 assistant 最后输出
		resultText = s.lastAssistantBlock()
	}
	if resultText != "" {
		// 限制长度，但给足够空间展示完整内容
		if len(resultText) > 3000 {
			resultText = resultText[:3000] + "\n..."
		}
		lines = append(lines, "")
		lines = append(lines, "📝 完成内容:")
		lines = append(lines, resultText)
		lines = append(lines, "")
		lines = append(lines, "━━━━━━━━━━━━━━━━")
	}

	// 文件变更统计
	writtenFiles := uniqueStrings(s.FilesWritten)
	editedFiles := uniqueStrings(s.FilesEdited)
	if len(writtenFiles) > 0 {
		lines = append(lines, fmt.Sprintf("✏️ 新建文件 (%d):", len(writtenFiles)))
		for _, f := range writtenFiles {
			lines = append(lines, fmt.Sprintf("   • %s", shortenPath(f)))
		}
	}
	if len(editedFiles) > 0 {
		lines = append(lines, fmt.Sprintf("✏️ 编辑文件 (%d):", len(editedFiles)))
		for _, f := range editedFiles {
			lines = append(lines, fmt.Sprintf("   • %s", shortenPath(f)))
		}
	}

	if len(s.FilesRead) > 0 {
		lines = append(lines, fmt.Sprintf("📖 读取文件 (%d)", len(uniqueStrings(s.FilesRead))))
	}

	if len(s.CommandsRun) > 0 {
		lines = append(lines, fmt.Sprintf("💻 执行命令 (%d)", len(s.CommandsRun)))
	}

	// 统计信息
	var stats []string
	if s.TotalCost > 0 {
		stats = append(stats, fmt.Sprintf("$%.4f", s.TotalCost))
	}
	if s.TotalTokens > 0 {
		stats = append(stats, fmt.Sprintf("%d tokens", s.TotalTokens))
	}
	if s.NumTurns > 0 {
		stats = append(stats, fmt.Sprintf("%d turns", s.NumTurns))
	}
	if s.Duration > 0 {
		secs := s.Duration / 1000
		if secs >= 60 {
			stats = append(stats, fmt.Sprintf("%.1f min", secs/60))
		} else {
			stats = append(stats, fmt.Sprintf("%.0fs", secs))
		}
	}
	if len(stats) > 0 {
		lines = append(lines, fmt.Sprintf("📊 %s", strings.Join(stats, " | ")))
	}

	return strings.Join(lines, "\n")
}

// lastAssistantBlock 提取 assistant 文本的最后一段有意义内容
func (s *TaskSummary) lastAssistantBlock() string {
	text := strings.TrimSpace(s.AssistantText)
	if text == "" {
		return ""
	}
	// 取最后 3000 字符作为最终总结
	if len(text) > 3000 {
		text = text[len(text)-3000:]
		// 找到第一个完整段落开始
		if idx := strings.Index(text, "\n"); idx >= 0 && idx < 200 {
			text = text[idx+1:]
		}
	}
	return text
}

// shortenPath 缩短路径显示，保留最后两级
func shortenPath(p string) string {
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	if len(parts) <= 2 {
		return p
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// claudeStreamMsg Claude Code stream-json 输出的消息格式
type claudeStreamMsg struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Result    string          `json:"result,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	CostUSD   float64         `json:"cost_usd,omitempty"`
	Duration  float64         `json:"duration_ms,omitempty"`
	TokensIn  int             `json:"input_tokens,omitempty"`
	TokensOut int             `json:"output_tokens,omitempty"`
	NumTurns  int             `json:"num_turns,omitempty"`
}

type claudeContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"` // thinking block 内容
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	ID       string          `json:"id,omitempty"`
}

type claudeAssistantMessage struct {
	Role    string          `json:"role"`
	Content []claudeContent `json:"content"`
}

// parseStreamLine 解析一行 stream-json 输出
func parseStreamLine(line string) *StreamEvent {
	var msg claudeStreamMsg
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		log.Printf("[DEBUG] skip unparseable line: %s", line)
		return nil
	}

	switch msg.Type {
	case "system":
		return &StreamEvent{
			Type:      "system",
			Text:      "系统初始化",
			SessionID: msg.SessionID,
		}

	case "assistant":
		var assistMsg claudeAssistantMessage
		if msg.Message != nil {
			json.Unmarshal(msg.Message, &assistMsg)
		}
		for _, block := range assistMsg.Content {
			switch block.Type {
			case "thinking":
				if block.Thinking != "" {
					return &StreamEvent{Type: "thinking", Text: block.Thinking}
				}
			case "text":
				if block.Text != "" {
					return &StreamEvent{Type: "assistant", Text: block.Text}
				}
			case "tool_use":
				inputStr := string(block.Input)
				return &StreamEvent{
					Type:      "tool",
					ToolName:  block.Name,
					ToolInput: inputStr,
					Text:      formatToolAction(block.Name, inputStr),
				}
			}
		}
		if msg.Subtype != "" {
			return &StreamEvent{Type: "system", Text: msg.Subtype}
		}
		return nil

	case "result":
		return &StreamEvent{
			Type:      "result",
			Text:      msg.Result,
			SessionID: msg.SessionID,
			CostUSD:   msg.CostUSD,
			TokensIn:  msg.TokensIn,
			TokensOut: msg.TokensOut,
			Duration:  msg.Duration,
			NumTurns:  msg.NumTurns,
			Done:      true,
		}

	case "user":
		var userMsg struct {
			Role    string `json:"role"`
			Content []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
				IsError bool   `json:"is_error"`
			} `json:"content"`
		}
		if msg.Message != nil {
			json.Unmarshal(msg.Message, &userMsg)
		}
		for _, block := range userMsg.Content {
			if block.Type == "tool_result" && block.Content != "" {
				text := block.Content
				if len(text) > 2000 {
					text = text[:2000] + "..."
				}
				eventType := "system"
				if block.IsError {
					eventType = "error"
					text = "⚠️ " + text
				}
				return &StreamEvent{Type: eventType, Text: text}
			}
		}
		return nil

	default:
		return nil
	}
}

// formatToolAction 格式化工具操作为可读文本
func formatToolAction(toolName, input string) string {
	var args map[string]interface{}
	json.Unmarshal([]byte(input), &args)

	switch toolName {
	case "Write", "write_file", "write":
		if path, ok := args["file_path"].(string); ok {
			return fmt.Sprintf("✏️ 写入 %s", path)
		}
		return "✏️ 写入文件"
	case "Read", "read_file", "read":
		if path, ok := args["file_path"].(string); ok {
			return fmt.Sprintf("📖 读取 %s", path)
		}
		return "📖 读取文件"
	case "Edit", "edit_file":
		if path, ok := args["file_path"].(string); ok {
			return fmt.Sprintf("✏️ 编辑 %s", path)
		}
		return "✏️ 编辑文件"
	case "Bash", "bash", "run_command":
		if cmd, ok := args["command"].(string); ok {
			if len(cmd) > 80 {
				cmd = cmd[:80] + "..."
			}
			return fmt.Sprintf("💻 执行: %s", cmd)
		}
		return "💻 执行命令"
	default:
		return fmt.Sprintf("🔧 %s", toolName)
	}
}

// openCodeMsg OpenCode --format json NDJSON 事件格式
// 实际格式:
//
//	{"type":"step_start", "part":{"type":"step-start",...}}
//	{"type":"text",       "part":{"type":"text","text":"你好！",...}}
//	{"type":"tool_use",   "part":{"type":"tool","tool":"bash","callID":"...","state":{"status":"completed","input":{...},"output":"...","title":"..."}}}
//	{"type":"step_finish","part":{"type":"step-finish","reason":"stop","cost":0.0003,"tokens":{"input":33,"output":35,...}}}
type openCodeMsg struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionID,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
	Part      json.RawMessage `json:"part,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// openCodePart part 内容块
type openCodePart struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Tool   string `json:"tool,omitempty"`   // tool_use: 工具名称 (bash, read, write...)
	CallID string `json:"callID,omitempty"` // tool_use: 调用ID
	Reason string `json:"reason,omitempty"` // step_finish: stop / tool-calls
	// tool_use: 工具调用状态（含 input/output）
	State *openCodeToolState `json:"state,omitempty"`
	// step_finish: 费用和 token 统计
	Cost   float64             `json:"cost,omitempty"`
	Tokens *openCodePartTokens `json:"tokens,omitempty"`
}

// openCodeToolState tool_use 中的 state 块
type openCodeToolState struct {
	Status string          `json:"status"` // completed / error
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
	Title  string          `json:"title,omitempty"`
}

// openCodePartTokens token 统计
type openCodePartTokens struct {
	Total     int `json:"total,omitempty"`
	Input     int `json:"input,omitempty"`
	Output    int `json:"output,omitempty"`
	Reasoning int `json:"reasoning,omitempty"`
}

// parseOpenCodeLine 解析一行 OpenCode NDJSON 输出
func parseOpenCodeLine(line string) *StreamEvent {
	var msg openCodeMsg
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		log.Printf("[DEBUG] opencode: skip unparseable line: %.200s", line)
		return nil
	}

	var part openCodePart
	if msg.Part != nil {
		json.Unmarshal(msg.Part, &part)
	}

	switch msg.Type {
	case "text":
		if part.Text != "" {
			return &StreamEvent{Type: "assistant", Text: part.Text}
		}
		return nil

	case "tool_use":
		// tool_use 包含完整的工具调用和结果
		toolName := part.Tool
		if toolName == "" {
			toolName = "unknown"
		}

		// 调试：打印 state.Input 原始内容
		if part.State != nil && part.State.Input != nil {
			log.Printf("[DEBUG] opencode tool_use: tool=%s, input=%s", toolName, string(part.State.Input))
		}

		// 从 state.input 提取输入参数
		inputStr := ""
		if part.State != nil {
			inputStr = string(part.State.Input)
		}

		// 构造工具事件
		event := &StreamEvent{
			Type:      "tool",
			ToolName:  toolName,
			ToolInput: inputStr,
			Text:      formatOpenCodeToolAction(toolName, part.State),
		}

		// 如果工具执行出错，标记为 error
		if part.State != nil && part.State.Status == "error" {
			event.Type = "error"
			event.Text = fmt.Sprintf("⚠️ %s: %s", toolName, part.State.Error)
		}

		return event

	case "step_start":
		return &StreamEvent{Type: "system", Text: "开始新的推理步骤..."}

	case "step_finish":
		ev := &StreamEvent{
			Type:    "result",
			CostUSD: part.Cost,
			Done:    false, // 中间步骤，进程退出时由 TaskComplete 发送 Done:true
		}
		if part.Tokens != nil {
			ev.TokensIn = part.Tokens.Input
			ev.TokensOut = part.Tokens.Output
		}
		return ev

	case "error":
		text := msg.Error
		if text == "" {
			text = "unknown error"
		}
		return &StreamEvent{Type: "error", Text: "⚠️ " + text}

	default:
		return nil
	}
}

// formatOpenCodeToolAction 格式化 OpenCode 工具调用为可读文本
func formatOpenCodeToolAction(toolName string, state *openCodeToolState) string {
	if state == nil {
		return fmt.Sprintf("🔧 %s", toolName)
	}

	var inputMap map[string]interface{}
	if state.Input != nil {
		json.Unmarshal(state.Input, &inputMap)
	}

	switch toolName {
	case "bash":
		if cmd, ok := inputMap["command"].(string); ok {
			desc := ""
			if d, ok := inputMap["description"].(string); ok && d != "" {
				desc = fmt.Sprintf("\n说明: %s", d)
			}
			if len(cmd) > 80 {
				cmd = cmd[:80] + "..."
			}
			return fmt.Sprintf("💻 命令: %s%s", cmd, desc)
		}
		if state.Title != "" {
			return fmt.Sprintf("💻 %s", state.Title)
		}
		return "💻 执行命令"
	case "read":
		if fp, ok := inputMap["filePath"].(string); ok {
			return fmt.Sprintf("📖 文件: %s", fp)
		}
		if fp, ok := inputMap["file"].(string); ok {
			return fmt.Sprintf("📖 文件: %s", fp)
		}
		if fp, ok := inputMap["path"].(string); ok {
			return fmt.Sprintf("📖 文件: %s", fp)
		}
		if state.Title != "" {
			return fmt.Sprintf("📖 %s", state.Title)
		}
		return "📖 读取文件"
	case "write":
		if fp, ok := inputMap["filePath"].(string); ok {
			return fmt.Sprintf("✏️ 文件: %s", fp)
		}
		if fp, ok := inputMap["file"].(string); ok {
			return fmt.Sprintf("✏️ 文件: %s", fp)
		}
		if state.Title != "" {
			return fmt.Sprintf("✏️ %s", state.Title)
		}
		return "✏️ 写入文件"
	case "edit":
		if fp, ok := inputMap["filePath"].(string); ok {
			return fmt.Sprintf("✏️ 编辑: %s", fp)
		}
		if state.Title != "" {
			return fmt.Sprintf("✏️ %s", state.Title)
		}
		return "✏️ 编辑文件"
	case "glob":
		if pat, ok := inputMap["pattern"].(string); ok {
			return fmt.Sprintf("🔍 搜索文件: %s", pat)
		}
		return "🔍 搜索文件"
	case "grep":
		if pat, ok := inputMap["pattern"].(string); ok {
			return fmt.Sprintf("🔍 搜索内容: %s", pat)
		}
		return "🔍 搜索内容"
	default:
		if state.Title != "" {
			return fmt.Sprintf("🔧 %s: %s", toolName, state.Title)
		}
		return fmt.Sprintf("🔧 %s", toolName)
	}
}

// parseOpenCodeStderr 解析 OpenCode stderr 输出行
// OpenCode 的进度信息（工具调用、命令执行、模型信息）输出到 stderr
// 典型行格式:
//
//	> build · deepseek-reasoner        (模型/阶段信息)
//	$ ls -la                           (命令执行)
//	命令输出内容...                     (命令结果)
//	(空行)
func parseOpenCodeStderr(line string) *StreamEvent {
	trimmed := strings.TrimSpace(line)

	// 跳过空行
	if trimmed == "" {
		return nil
	}

	// "$ command" → 工具调用事件
	if strings.HasPrefix(trimmed, "$ ") {
		cmd := strings.TrimPrefix(trimmed, "$ ")
		return &StreamEvent{
			Type:     "tool",
			ToolName: "bash",
			Text:     fmt.Sprintf("💻 执行: %s", cmd),
		}
	}

	// "> phase · model" → 系统信息（模型/阶段切换）
	if strings.HasPrefix(trimmed, "> ") {
		return &StreamEvent{
			Type: "system",
			Text: trimmed,
		}
	}

	// 其他内容行 → 作为系统信息透传（命令输出等）
	// 限制长度避免刷屏
	if len(trimmed) > 500 {
		trimmed = trimmed[:500] + "..."
	}
	return &StreamEvent{
		Type: "system",
		Text: trimmed,
	}
}
