package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mcp"
	log "mylog"
	"net/http"
	"time"
)

// SendSyncLLMRequest sends a synchronous (non-streaming) LLM request with tool calling support
// This is designed for backend services like agent that don't need HTTP streaming
func SendSyncLLMRequest(messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequest: account=%s, messages=%d", account, len(messages))

	config := GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}

	// Get available MCP tools (uses extractFunctionName via GetInnerMCPToolsProcessed)
	availableTools := mcp.GetInnerMCPToolsProcessed()
	log.DebugF(log.ModuleLLM, "Available tools for sync LLM: %d", len(availableTools))

	// Keep track of messages in struct format
	currentMessages := make([]Message, len(messages))
	copy(currentMessages, messages)

	// Tool calling loop
	maxIterations := 10
	var finalResponse string

	for iteration := 0; iteration < maxIterations; iteration++ {
		// Sanitize messages before sending
		sanitizedMessages := SanitizeMessages(currentMessages)

		// Convert to API format
		apiMessages := convertMessagesToAPI(sanitizedMessages)

		// Build request
		requestBody := map[string]interface{}{
			"model":       config.Model,
			"messages":    apiMessages,
			"tools":       availableTools,
			"temperature": config.Temperature,
			"stream":      false,
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return "", fmt.Errorf("marshal request failed: %w", err)
		}

		// Log the full LLM request for debugging
		log.DebugF(log.ModuleLLM, "LLM Request Body (iteration %d): %s", iteration, string(jsonData))

		// Create HTTP request
		req, err := http.NewRequest("POST", config.BaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("create request failed: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.APIKey)

		// Send request
		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send request failed: %w", err)
		}

		// Read response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read response failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.ErrorF(log.ModuleLLM, "LLM API error: %d, body: %s", resp.StatusCode, string(body))
			return "", fmt.Errorf("LLM API error: %d", resp.StatusCode)
		}

		// Parse response using LLMResponse type
		var llmResp LLMResponse
		if err := json.Unmarshal(body, &llmResp); err != nil {
			return "", fmt.Errorf("parse response failed: %w\nBody: %s", err, string(body))
		}

		if len(llmResp.Choices) == 0 {
			return "", fmt.Errorf("empty response from LLM")
		}

		choice := llmResp.Choices[0]
		toolCalls := choice.Message.ToolCalls

		// If no tool calls, return content
		if len(toolCalls) == 0 {
			finalResponse = choice.Message.Content
			log.DebugF(log.ModuleLLM, "LLM sync response (no tools): %d chars", len(finalResponse))
			break
		}

		// Process tool calls
		log.DebugF(log.ModuleLLM, "LLM requested %d tool calls", len(toolCalls))

		// Add assistant message (with tool calls) to history
		assistantMsg := Message{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		currentMessages = append(currentMessages, assistantMsg)

		// Execute each tool call
		for _, toolCall := range toolCalls {
			toolName := toolCall.Function.Name
			toolArgs := toolCall.Function.Arguments
			log.MessageF(log.ModuleLLM, "Sync calling tool: %s with args: %s", toolName, toolArgs)

			// Parse tool arguments
			var parsedArgs map[string]interface{}
			if err := json.Unmarshal([]byte(toolArgs), &parsedArgs); err != nil {
				log.ErrorF(log.ModuleLLM, "Failed to parse tool args: %v", err)
				parsedArgs = make(map[string]interface{})
			}

			// Ensure account parameter
			if _, ok := parsedArgs["account"]; !ok {
				parsedArgs["account"] = account
			}

			// Call tool
			result := mcp.CallMCPTool(toolName, parsedArgs)
			log.MessageF(log.ModuleLLM, "Tool result: %v", result)

			// Add tool result to messages
			toolResult := fmt.Sprintf("%v", result.Result)
			if !result.Success {
				toolResult = "Error: " + result.Error
			}

			// Add tool result message
			toolMsg := Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    toolResult,
			}
			currentMessages = append(currentMessages, toolMsg)
		}

		// If last iteration, set default response
		if iteration == maxIterations-1 {
			finalResponse = "工具调用已完成"
		}
	}

	return finalResponse, nil
}

// convertMessagesToAPI converts Message slice to API format
func convertMessagesToAPI(messages []Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if msg.ToolCallId != "" {
			m["tool_call_id"] = msg.ToolCallId
		}
		if len(msg.ToolCalls) > 0 {
			m["tool_calls"] = msg.ToolCalls
		}
		result[i] = m
	}
	return result
}
