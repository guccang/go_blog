package llm

import (
	"config"
	"encoding/json"
	"fmt"
	"mcp"
	log "mylog"
	"net/http"
	"net/url"
	"strings"
)

// ToolExecutor handles tool call execution loop
type ToolExecutor struct {
	account        string
	maxCalls       int
	availableTools []mcp.LLMTool
	writer         http.ResponseWriter
	flusher        http.Flusher
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(account string, tools []mcp.LLMTool, w http.ResponseWriter, flusher http.Flusher) *ToolExecutor {
	return &ToolExecutor{
		account:        account,
		maxCalls:       25,
		availableTools: tools,
		writer:         w,
		flusher:        flusher,
	}
}

// ExecuteToolLoop executes the main tool calling loop
func (te *ToolExecutor) ExecuteToolLoop(query string, selectedTools []string) error {
	log.DebugF(log.ModuleLLM, "=== Streaming LLM Processing Started with Tool Support ===")
	log.DebugF(log.ModuleLLM, "Query: account=%s %s", te.account, query)
	log.DebugF(log.ModuleLLM, "Selected tools: %v", selectedTools)

	maxSelected := GetMaxSelectedTools()
	if len(selectedTools) > maxSelected {
		log.WarnF(log.ModuleLLM, "Selected tools count is too large, max is %d", maxSelected)
		selectedTools = selectedTools[:maxSelected]
	}

	sysPrompt := fmt.Sprintf("使用%s账号作为参数,你是一个万能助手，自行决定是否调用工具获取数据，当你得到工具返回结果后，就不需要调用相同工具了，最后返回简单直接的结果给用户。", te.account)

	// Initialize messages
	messages := []Message{
		{
			Role:    "system",
			Content: sysPrompt,
		},
		{
			Role:    "user",
			Content: query,
		},
	}

	// Get available tools
	availableTools := mcp.GetAvailableLLMTools(selectedTools)
	log.DebugF(log.ModuleLLM, "Available LLM tools: %d", len(availableTools))

	var fullResponse strings.Builder

	// Initial LLM call
	_, toolCalls, err := SendStreamingLLMRequest(messages, availableTools, te.writer, te.flusher, &fullResponse)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Initial streaming LLM request failed: %v", err)
		return fmt.Errorf("initial streaming LLM request failed: %v", err)
	}

	// Tool calling loop with max iterations
	maxCall := te.maxCalls
	for len(toolCalls) > 0 && maxCall > 0 {
		maxCall--
		log.DebugF(log.ModuleLLM, "Tool calling iteration, remaining: %d", maxCall)

		// Process tool calls
		log.DebugF(log.ModuleLLM, "Processing %d tool calls", len(toolCalls))
		for _, toolCall := range toolCalls {
			// Log tool call status but don't send to client to keep response clean
			log.DebugF(log.ModuleLLM, fmt.Sprintf("\n[Calling tool %s with args %s]\n", toolCall.Function.Name, toolCall.Function.Arguments))

			toolName := toolCall.Function.Name
			toolArgs := make(map[string]interface{})

			fmt.Fprintf(te.writer, "data: %s\n\n", url.QueryEscape(fmt.Sprintf("[Calling tool %s with args %s]", toolCall.Function.Name, toolCall.Function.Arguments)))
			te.flusher.Flush()

			// Parse tool arguments with validation
			if toolCall.Function.Arguments == "" {
				log.WarnF(log.ModuleLLM, "Tool call %s has empty arguments, skipping", toolName)
				continue
			}

			// Validate JSON format first
			if !IsValidJSON(toolCall.Function.Arguments) {
				log.ErrorF(log.ModuleLLM, "Tool call %s has invalid JSON arguments: %s", toolName, toolCall.Function.Arguments)
				continue
			}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs); err != nil {
				log.ErrorF(log.ModuleLLM, "Failed to parse tool arguments for %s: %v, args: %s", toolName, err, toolCall.Function.Arguments)
				continue
			}

			// Call the tool
			log.InfoF(log.ModuleLLM, "Tool call begin: %s %v", toolName, toolArgs)
			result := mcp.CallMCPTool(toolName, toolArgs)
			log.InfoF(log.ModuleLLM, "Tool call result: %s %v %v", toolName, toolArgs, result)

			// Add tool call and result to message history
			messages = append(messages, Message{
				Role:      "assistant",
				ToolCalls: []mcp.ToolCall{SanitizeToolCall(toolCall)},
			})

			toolContent := TruncateString(fmt.Sprintf("%v", result.Result), MaxToolResultChars)
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    toolContent,
			})

			// Add tool call info to full response for saving
			save := config.GetConfigWithAccount(config.GetAdminAccount(), "assistant_save_mcp_result")
			// len(result.Result) < 32 indicates short result, not privacy data, can be stored in Assistant_xxx
			if strings.ToLower(save) == "true" || len(fmt.Sprintf("%v", result.Result)) < 32 {
				fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %v]\n", toolName, result.Result))
			} else {
				// Don't display tool callback returned data, involves privacy, sending to LLM is fine, but caching and displaying on UI is problematic
				fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %s]\n", toolName, "###$#&$#*$)@$&$%&$())!@###"))
			}
		}

		// Next LLM call with updated messages
		log.InfoF(log.ModuleLLM, "Tool calls processed, sending next LLM request")
		_, toolCalls, err = SendStreamingLLMRequest(messages, availableTools, te.writer, te.flusher, &fullResponse)
		if err != nil {
			log.ErrorF(log.ModuleLLM, "LLM call failed in tool loop: %v", err)
			break
		}
		log.InfoF(log.ModuleLLM, "Next LLM response received, tool calls: %d", len(toolCalls))
	}

	// Send completion signal to client
	log.DebugF(log.ModuleLLM, "Tool processing complete, sending DONE signal")
	fmt.Fprintf(te.writer, "data: [DONE]\n\n")
	te.flusher.Flush()

	// Save complete response to diary
	go SaveLLMResponseToDiary(te.account, query, fullResponse.String())
	return nil
}

// ProcessQueryStreaming is a convenience function wrapping ToolExecutor
func ProcessQueryStreaming(account string, query string, selectedTools []string, w http.ResponseWriter, flusher http.Flusher) error {
	executor := NewToolExecutor(account, nil, w, flusher)
	return executor.ExecuteToolLoop(query, selectedTools)
}
