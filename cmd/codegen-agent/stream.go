package main

import (
	"encoding/json"
	"fmt"
	"log"
)

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
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	ID    string          `json:"id,omitempty"`
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
				if len(text) > 500 {
					text = text[:500] + "..."
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
