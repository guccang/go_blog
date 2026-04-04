package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 上下文字符预算常量
const (
	processMaxTotalChars = 150000 // 总字符预算
	processMaxMessages   = 40     // 最大消息数
)

// truncate 截断字符串用于日志显示（UTF-8 安全，按字符数截断）
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// truncateToolResult 截断工具结果，防止上下文溢出
// 前 3 轮 max 3000 字符，之后 max 1500 字符
func truncateToolResult(result string, iteration int) string {
	maxLen := 3000
	if iteration >= 3 {
		maxLen = 1500
	}
	return truncateToolResultWithLimit(result, maxLen, iteration)
}

// estimateChars 估算消息列表的总字符数
func estimateChars(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Arguments)
		}
	}
	return total
}

// sanitizeProcessMessages 参考 blog-agent SanitizeMessages 模式
// 从末尾向前保留消息，超出字符预算或消息数上限时停止
// 始终保留 system prompt（messages[0]）
func sanitizeProcessMessages(messages []Message) []Message {
	return sanitizeMessagesWithBudget(messages, processMaxMessages, processMaxTotalChars)
}

// formatExecuteCodeEvent 格式化 ExecuteCode 工具调用的展示
func formatExecuteCodeEvent(argsJSON string, idx, total int) string {
	var args struct {
		Code        string   `json:"code"`
		Description string   `json:"description"`
		ToolsHint   []string `json:"tools_hint"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("调用 ExecuteCode (%d/%d)\n参数: %s", idx, total, argsJSON)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🐍 ExecuteCode (%d/%d)", idx, total))
	if args.Description != "" {
		sb.WriteString(fmt.Sprintf("\n说明: %s", args.Description))
	}
	if len(args.ToolsHint) > 0 {
		sb.WriteString(fmt.Sprintf("\n工具: %s", strings.Join(args.ToolsHint, ", ")))
	}

	// 限制代码显示长度，防止超出微信消息长度限制
	const maxCodeDisplay = 190000 // 字符数（预留头部空间，微信限制约204800）
	code := args.Code
	codeRunes := []rune(code)
	if len(codeRunes) > maxCodeDisplay {
		code = string(codeRunes[:maxCodeDisplay]) + "\n# ... [代码已截断，共" + fmt.Sprintf("%d", len(codeRunes)) + "字符]"
	}

	sb.WriteString(fmt.Sprintf("\n```python\n%s\n```", code))
	return sb.String()
}

// parseExecuteCodeResult 解析 execute-code-agent 返回的结构化 JSON
// 返回 (stdout 给 LLM, tool_calls 摘要给用户展示)
func parseExecuteCodeResult(resultJSON string) (string, string) {
	var execResult struct {
		Success    bool   `json:"success"`
		Stdout     string `json:"stdout"`
		Stderr     string `json:"stderr"`
		DurationMs int64  `json:"duration_ms"`
		ToolCalls  []struct {
			Tool     string `json:"tool"`
			AgentID  string `json:"agent_id"`
			Success  bool   `json:"success"`
			Duration int64  `json:"duration_ms"`
			Error    string `json:"error"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &execResult); err != nil {
		// 不是结构化 JSON，原样返回
		return resultJSON, ""
	}

	// 构建工具调用链摘要
	var summary string
	if len(execResult.ToolCalls) > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📡 内部调用 %d 个工具:", len(execResult.ToolCalls)))
		for _, tc := range execResult.ToolCalls {
			status := "✅"
			if !tc.Success {
				status = "❌"
			}
			agent := tc.AgentID
			if agent == "" {
				agent = "?"
			}
			sb.WriteString(fmt.Sprintf("\n  %s %s →%s (%dms)", status, tc.Tool, agent, tc.Duration))
			if tc.Error != "" {
				sb.WriteString(fmt.Sprintf(" %s", truncate(tc.Error, 80)))
			}
		}
		summary = sb.String()
	}

	return execResult.Stdout, summary
}

// extractExecuteCodeStderr 从 ExecuteCode 的结构化结果中提取 stderr 错误详情
// 用于 ExecuteCode 失败时将具体错误信息传递给 LLM，使其能修正代码
func extractExecuteCodeStderr(resultJSON string) string {
	var execResult struct {
		Stderr    string `json:"stderr"`
		ErrorType string `json:"error_type"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &execResult); err == nil && execResult.Stderr != "" {
		return execResult.Stderr
	}

	return ""
}
