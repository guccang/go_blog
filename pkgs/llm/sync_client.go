package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mcp"
	log "mylog"
	"net/http"
	"strings"
	"time"
)

const maxContentLength = 50000 // 最大内容长度（字符）

// truncateContent 截断过长的内容
func truncateContent(content string) string {
	if len(content) <= maxContentLength {
		return content
	}
	return content[:maxContentLength] + fmt.Sprintf("\n\n... [内容已截断，原长度: %d 字符]", len(content))
}

// ProgressCallback 进度回调类型
// eventType: "start" / "tool_call" / "tool_result"
type ProgressCallback func(eventType string, detail string)

// SendSyncLLMRequest sends a synchronous (non-streaming) LLM request with tool calling support
func SendSyncLLMRequest(messages []Message, account string) (string, error) {
	return SendSyncLLMRequestWithContext(context.Background(), messages, account)
}

// SendSyncLLMRequestWithProgress sends a synchronous LLM request with progress callback
func SendSyncLLMRequestWithProgress(messages []Message, account string, callback ProgressCallback) (string, error) {
	return sendSyncLLMRequestInternal(context.Background(), messages, account, callback)
}

// SendSyncLLMRequestWithContext sends a synchronous LLM request with context support
func SendSyncLLMRequestWithContext(ctx context.Context, messages []Message, account string) (string, error) {
	return sendSyncLLMRequestInternal(ctx, messages, account, nil)
}

// sendSyncLLMRequestInternal is the shared implementation for sync LLM requests with optional progress callback
func sendSyncLLMRequestInternal(ctx context.Context, messages []Message, account string, callback ProgressCallback) (string, error) {
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
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

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

		// 诊断日志：记录请求体大小和上下文长度
		requestSizeKB := len(jsonData) / 1024
		totalContentLen := 0
		for _, msg := range currentMessages {
			totalContentLen += len(msg.Content)
		}
		log.MessageF(log.ModuleLLM, "[LLM诊断] 请求体大小: %d KB, 消息数: %d, 内容总长度: %d 字符, 工具数: %d, 迭代: %d",
			requestSizeKB, len(currentMessages), totalContentLen, len(availableTools), iteration)

		// Log the full LLM request for debugging
		log.DebugF(log.ModuleLLM, "LLM Request Body (iteration %d): %s", iteration, string(jsonData))

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", config.BaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("create request failed: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.APIKey)

		// Send request
		client := &http.Client{Timeout: 600 * time.Second} // 10分钟
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send request failed: %w", err)
		}
		defer resp.Body.Close() // 确保 Body 总是被关闭

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read response failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.ErrorF(log.ModuleLLM, "LLM API error: %d, body: %s", resp.StatusCode, string(body))

			// 5xx 错误时尝试 fallback
			if resp.StatusCode >= 500 && FallbackToNext() {
				config = GetConfig()
				log.WarnF(log.ModuleLLM, "Primary LLM failed with %d, retrying with fallback: %s", resp.StatusCode, GetActiveProvider())
				continue // 使用新 config 重试当前迭代
			}

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
		var lastToolInfo []string
		for _, toolCall := range toolCalls {
			toolName := toolCall.Function.Name
			toolArgs := toolCall.Function.Arguments
			lastToolInfo = append(lastToolInfo, fmt.Sprintf("%s(%s)", toolName, toolArgs))
			log.MessageF(log.ModuleLLM, "Sync calling tool: %s with args: %s", toolName, toolArgs)

			// Progress callback: tool_call
			if callback != nil {
				callback("tool_call", toolName)
			}

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

			// Progress callback: tool_result
			if callback != nil {
				callback("tool_result", toolName+" 完成")
			}

			// Add tool result to messages
			toolResult := fmt.Sprintf("%v", result.Result)
			if !result.Success {
				toolResult = "Error: " + result.Error
			}
			// 截断过长的工具结果
			if len(toolResult) > maxContentLength {
				log.WarnF(log.ModuleLLM, "[内容截断] 工具 %s 返回过长: %d 字符 -> %d 字符",
					toolName, len(toolResult), maxContentLength)
				toolResult = truncateContent(toolResult)
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
			finalResponse = fmt.Sprintf("工具调用已完成(达到最大迭代限制)。最后调用的工具: %s", strings.Join(lastToolInfo, "; "))
		}
	}

	return finalResponse, nil
}

// SendSyncLLMRequestNoTools sends a simple LLM request without any tools (for tool selection phase)
func SendSyncLLMRequestNoTools(ctx context.Context, messages []Message, account string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestNoTools: account=%s, messages=%d", account, len(messages))

	config := GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Sanitize and convert messages
	sanitizedMessages := SanitizeMessages(messages)
	apiMessages := convertMessagesToAPI(sanitizedMessages)

	// Build request WITHOUT tools
	requestBody := map[string]interface{}{
		"model":       config.Model,
		"messages":    apiMessages,
		"temperature": config.Temperature,
		"stream":      false,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", config.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	// Send request
	client := &http.Client{Timeout: 600 * time.Second} // 10分钟
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close() // 确保 Body 总是被关闭

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.ErrorF(log.ModuleLLM, "LLM API error: %d, body: %s", resp.StatusCode, string(body))

		// 5xx 错误时尝试 fallback
		if resp.StatusCode >= 500 && FallbackToNext() {
			config = GetConfig()
			log.WarnF(log.ModuleLLM, "LLM NoTools fallback to: %s", GetActiveProvider())
			requestBody["model"] = config.Model
			requestBody["temperature"] = config.Temperature
			jsonData2, _ := json.Marshal(requestBody)
			req2, _ := http.NewRequestWithContext(ctx, "POST", config.BaseURL, bytes.NewBuffer(jsonData2))
			req2.Header.Set("Content-Type", "application/json")
			req2.Header.Set("Authorization", "Bearer "+config.APIKey)
			resp2, err2 := client.Do(req2)
			if err2 == nil {
				defer resp2.Body.Close()
				body2, _ := io.ReadAll(resp2.Body)
				if resp2.StatusCode == http.StatusOK {
					var llmResp2 LLMResponse
					if err3 := json.Unmarshal(body2, &llmResp2); err3 == nil && len(llmResp2.Choices) > 0 {
						return llmResp2.Choices[0].Message.Content, nil
					}
				}
			}
		}

		return "", fmt.Errorf("LLM API error: %d", resp.StatusCode)
	}

	// Parse response
	var llmResp LLMResponse
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return "", fmt.Errorf("parse response failed: %w\nBody: %s", err, string(body))
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return llmResp.Choices[0].Message.Content, nil
}

// SendSyncLLMRequestWithSelectedTools sends an LLM request with only selected tools
func SendSyncLLMRequestWithSelectedTools(ctx context.Context, messages []Message, account string, selectedTools []string) (string, error) {
	log.DebugF(log.ModuleLLM, "SendSyncLLMRequestWithSelectedTools: account=%s, selectedTools=%v", account, selectedTools)

	config := GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}

	// Get only selected MCP tools
	var availableTools []mcp.LLMTool
	if selectedTools == nil || len(selectedTools) == 0 {
		// Fallback: use all tools
		availableTools = mcp.GetInnerMCPToolsProcessed()
	} else {
		availableTools = mcp.GetAvailableLLMTools(selectedTools)
	}
	log.DebugF(log.ModuleLLM, "Using %d tools for LLM call", len(availableTools))

	// Keep track of messages
	currentMessages := make([]Message, len(messages))
	copy(currentMessages, messages)

	// Tool calling loop
	maxIterations := 10
	var finalResponse string

	for iteration := 0; iteration < maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		sanitizedMessages := SanitizeMessages(currentMessages)
		apiMessages := convertMessagesToAPI(sanitizedMessages)

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

		// 诊断日志：记录请求体大小和上下文长度
		requestSizeKB := len(jsonData) / 1024
		totalContentLen := 0
		for _, msg := range currentMessages {
			totalContentLen += len(msg.Content)
		}
		log.MessageF(log.ModuleLLM, "[LLM诊断-SelectedTools] 请求体大小: %d KB, 消息数: %d, 内容总长度: %d 字符, 工具数: %d, 迭代: %d",
			requestSizeKB, len(currentMessages), totalContentLen, len(availableTools), iteration)

		req, err := http.NewRequestWithContext(ctx, "POST", config.BaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("create request failed: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.APIKey)

		client := &http.Client{Timeout: 600 * time.Second} // 10分钟
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send request failed: %w", err)
		}
		defer resp.Body.Close() // 确保 Body 总是被关闭

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read response failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.ErrorF(log.ModuleLLM, "LLM API error: %d", resp.StatusCode)

			// 5xx 错误时尝试 fallback
			if resp.StatusCode >= 500 && FallbackToNext() {
				config = GetConfig()
				log.WarnF(log.ModuleLLM, "LLM SelectedTools fallback to: %s", GetActiveProvider())
				continue // 使用新 config 重试当前迭代
			}

			return "", fmt.Errorf("LLM API error: %d", resp.StatusCode)
		}

		var llmResp LLMResponse
		if err := json.Unmarshal(body, &llmResp); err != nil {
			return "", fmt.Errorf("parse response failed: %w", err)
		}

		if len(llmResp.Choices) == 0 {
			return "", fmt.Errorf("empty response from LLM")
		}

		choice := llmResp.Choices[0]
		toolCalls := choice.Message.ToolCalls

		if len(toolCalls) == 0 {
			finalResponse = choice.Message.Content
			break
		}

		// Add assistant message
		assistantMsg := Message{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		currentMessages = append(currentMessages, assistantMsg)

		// Execute tool calls
		var lastToolInfo []string
		for _, toolCall := range toolCalls {
			toolName := toolCall.Function.Name
			toolArgs := toolCall.Function.Arguments
			lastToolInfo = append(lastToolInfo, fmt.Sprintf("%s(%s)", toolName, toolArgs))

			var parsedArgs map[string]interface{}
			if err := json.Unmarshal([]byte(toolArgs), &parsedArgs); err != nil {
				parsedArgs = make(map[string]interface{})
			}

			if _, ok := parsedArgs["account"]; !ok {
				parsedArgs["account"] = account
			}

			result := mcp.CallMCPTool(toolName, parsedArgs)

			toolResult := fmt.Sprintf("%v", result.Result)
			if !result.Success {
				toolResult = "Error: " + result.Error
			}
			// 截断过长的工具结果
			if len(toolResult) > maxContentLength {
				log.WarnF(log.ModuleLLM, "[内容截断] 工具 %s 返回过长: %d 字符 -> %d 字符",
					toolName, len(toolResult), maxContentLength)
				toolResult = truncateContent(toolResult)
			}

			toolMsg := Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    toolResult,
			}
			currentMessages = append(currentMessages, toolMsg)
		}

		if iteration == maxIterations-1 {
			finalResponse = fmt.Sprintf("工具调用已完成(达到最大迭代限制)。最后调用的工具: %s", strings.Join(lastToolInfo, "; "))
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
