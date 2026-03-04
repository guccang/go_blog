package llm

import (
	"codegen"
	"context"
	log "mylog"
	"time"
)

// ProgressCallback 进度回调类型
// eventType: "start" / "tool_call" / "tool_result"
type ProgressCallback func(eventType string, detail string)

// SendSyncLLMRequest sends a synchronous LLM request via llm-mcp-agent
func SendSyncLLMRequest(messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequest via agent: account=%s, messages=%d", account, len(messages))
	return codegen.SendSyncLLMTask(messages, account, nil, false, 3*time.Minute)
}

// SendSyncLLMRequestWithProgress sends a synchronous LLM request via llm-mcp-agent (progress callback ignored)
func SendSyncLLMRequestWithProgress(messages []Message, account string, callback ProgressCallback) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithProgress via agent: account=%s, messages=%d", account, len(messages))
	return codegen.SendSyncLLMTask(messages, account, nil, false, 3*time.Minute)
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

// SendSyncLLMRequestWithContext sends a synchronous LLM request with context support via llm-mcp-agent
func SendSyncLLMRequestWithContext(ctx context.Context, messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithContext via agent: account=%s, messages=%d", account, len(messages))
	return codegen.SendSyncLLMTask(messages, account, nil, false, 3*time.Minute)
}
