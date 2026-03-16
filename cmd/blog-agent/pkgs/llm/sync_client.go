package llm

import (
	"codegen"
	"context"
	log "mylog"
	"strings"
	"time"
)

// ProgressCallback 进度回调类型
// eventType: "start" / "thinking" / "tool_call" / "tool_result"
type ProgressCallback func(eventType string, detail string)

// SendSyncLLMRequest sends a synchronous LLM request via llm-mcp-agent
func SendSyncLLMRequest(messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequest via agent: account=%s, messages=%d", account, len(messages))
	return codegen.SendSyncLLMTask(messages, account, nil, false, 3*time.Minute)
}

// SendSyncLLMRequestWithProgress sends a synchronous LLM request via llm-mcp-agent with progress callback
func SendSyncLLMRequestWithProgress(messages []Message, account string, callback ProgressCallback) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithProgress via agent: account=%s, messages=%d", account, len(messages))

	var progressCb codegen.LLMProgressCallback
	if callback != nil {
		progressCb = func(event, text string) {
			switch event {
			case "thinking":
				callback("thinking", text)
			case "tool_info":
				// 提取工具名：从 "[Calling tool X with args ...]" 中提取工具名
				toolName := extractToolName(text)
				if toolName != "" {
					callback("tool_call", toolName)
				}
			}
		}
	}
	return codegen.SendSyncLLMTaskWithProgress(messages, account, nil, false, 3*time.Minute, progressCb)
}

// extractToolName 从 tool_info 文本中提取工具名
// 输入格式: "[Calling tool X with args ...]" → 返回 "X"
// 输入格式: "[🔧 本次加载 N 个工具]" → 返回 ""（忽略非调用类事件）
func extractToolName(text string) string {
	const prefix = "[Calling tool "
	if !strings.HasPrefix(text, prefix) {
		return ""
	}
	rest := text[len(prefix):]
	if idx := strings.Index(rest, " with args "); idx > 0 {
		return rest[:idx]
	}
	// 没有 args 部分，取到 "]"
	if idx := strings.Index(rest, "]"); idx > 0 {
		return rest[:idx]
	}
	return ""
}

// SendSyncLLMRequestNoTools sends a simple LLM request without any tools via llm-mcp-agent
func SendSyncLLMRequestNoTools(ctx context.Context, messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestNoTools via agent: account=%s, messages=%d", account, len(messages))
	return codegen.SendSyncLLMTask(messages, account, nil, true, 3*time.Minute)
}

// SendSyncLLMRequestWithSelectedTools sends an LLM request with only selected tools via llm-mcp-agent
func SendSyncLLMRequestWithSelectedTools(ctx context.Context, messages []Message, account string, selectedTools []string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithSelectedTools via agent: account=%s, selectedTools=%v", account, selectedTools)
	return codegen.SendSyncLLMTask(messages, account, selectedTools, false, 3*time.Minute)
}

// ToolCallEvent 工具调用事件（从进度回调中捕获）
type ToolCallEvent struct {
	ToolName string // 工具名称
	RawText  string // 原始事件文本
}

// SendSyncLLMRequestWithSelectedToolsAndCallback 带进度回调的工具调用 LLM 请求
// 在执行过程中通过 callback 捕获每次工具调用事件
func SendSyncLLMRequestWithSelectedToolsAndCallback(ctx context.Context, messages []Message, account string, selectedTools []string, callback func(event ToolCallEvent)) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithSelectedToolsAndCallback via agent: account=%s, selectedTools=%v", account, selectedTools)

	var progressCb codegen.LLMProgressCallback
	if callback != nil {
		progressCb = func(event, text string) {
			if event == "tool_info" {
				toolName := extractToolName(text)
				if toolName != "" {
					callback(ToolCallEvent{ToolName: toolName, RawText: text})
				}
			}
		}
	}
	return codegen.SendSyncLLMTaskWithProgress(messages, account, selectedTools, false, 3*time.Minute, progressCb)
}

// SendSyncLLMRequestWithContext sends a synchronous LLM request with context support via llm-mcp-agent
func SendSyncLLMRequestWithContext(ctx context.Context, messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithContext via agent: account=%s, messages=%d", account, len(messages))
	return codegen.SendSyncLLMTask(messages, account, nil, false, 3*time.Minute)
}
