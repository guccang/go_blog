// Package llm provides LLM (Large Language Model) integration functionality.
//
// This package delegates all LLM calls to llm-agent via gateway.
// Kept modules:
//   - message.go: Message types and sanitization (Message, SanitizeMessages)
//   - sync_client.go: Bridge functions that route to llm-agent via codegen.SendSyncLLMTask
//   - http_handler.go: Web assistant SSE route (ProcessRequest)
//   - agent_bridge.go: Web assistant streaming bridge (ProcessRequestViaAgent)
//   - ai_skill.go: Pluggable AI skills system (operates blog data, no LLM calls)
package llm

// Init initializes the LLM module
func Init() error {
	return nil
}
