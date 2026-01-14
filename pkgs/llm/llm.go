// Package llm provides LLM (Large Language Model) integration functionality.
//
// This package is organized into the following modules:
//   - config.go: Configuration management (LLMConfig, InitConfig)
//   - message.go: Message types and sanitization (Message, SanitizeMessages)
//   - diary.go: Diary/blog saving functionality (SaveLLMResponseToDiary)
//   - stream.go: Stream processing and tool detection (ProcessStreamingResponseWithToolDetection)
//   - client.go: LLM API client (SendStreamingLLMRequest)
//   - tool_handler.go: Tool execution loop (ToolExecutor, ProcessQueryStreaming)
//   - http_handler.go: HTTP request handling (ProcessRequest)
package llm

import (
	log "mylog"
)

// Info prints module version information
func Info() {
	log.Debug(log.ModuleLLM, "info llm v1.0")
}

// Init initializes the LLM module
func Init() error {
	InitConfig()
	return nil
}
