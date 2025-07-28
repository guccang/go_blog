package llm

import (
	"bufio"
	"bytes"
	"config"
	"control"
	"encoding/json"
	"fmt"
	"io"
	"mcp"
	"module"
	log "mylog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// LLM Configuration
type LLMConfig struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
}

type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []mcp.ToolCall `json:"tool_calls,omitempty"`
	ToolCallId string         `json:"tool_call_id,omitempty"`
}

// Choice represents a choice in LLM response
type Choice struct {
	Index        int       `json:"index"`
	Message      Message   `json:"message"`
	LogProbs     *struct{} `json:"logprobs"`
	FinishReason string    `json:"finish_reason"`
}

// Usage represents the usage statistics in LLM response
type Usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
}

// LLMRequest represents request to LLM API
type LLMRequest struct {
	Model       string        `json:"model"`
	Messages    []Message     `json:"messages"`
	Tools       []mcp.LLMTool `json:"tools,omitempty"`
	Temperature float64       `json:"temperature"`
}

// LLMResponse represents response from LLM API
type LLMResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint"`
}

var llmConfig = LLMConfig{}

func Info() {
	fmt.Println("info llm v1.0")
}

// getConfigWithDefault 获取配置值，如果为空则使用默认值
func getConfigWithDefault(key, defaultValue string) string {
	value := config.GetConfig(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func InitConfig() {
	llmConfig = LLMConfig{
		APIKey:      getConfigWithDefault("deepseek_api_key", os.Getenv("OPENAI_API_KEY")),
		BaseURL:     getConfigWithDefault("deepseek_api_url", "https://api.deepseek.com/v1/chat/completions"),
		Model:       "deepseek-chat",
		Temperature: 0.3,
	}
}

func Init() error {
	InitConfig()
	return nil
}

func ProcessRequest(r *http.Request, w http.ResponseWriter) int {
	if r.Method != http.MethodPost {
		log.WarnF("Invalid method %s for assistant chat from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return http.StatusMethodNotAllowed
	}

	// 读取请求体
	log.Debug("Reading assistant chat request body...")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.ErrorF("Error reading assistant chat request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
	defer r.Body.Close()

	log.DebugF("Received assistant chat request body: %d bytes", len(body))

	// 解析请求
	var request struct {
		Messages    []Message `json:"messages"`
		Stream      bool      `json:"stream"`
		Tools       []string  `json:"selected_tools,omitempty"`
		TypingSpeed string    `json:"typing_speed,omitempty"` // 打字机速度设置
	}

	if err := json.Unmarshal(body, &request); err != nil {
		log.ErrorF("Error parsing assistant chat request body: %v", err)
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return http.StatusBadRequest
	}

	log.InfoF("Assistant chat request parsed: %d messages, stream=%t, tools=%v", len(request.Messages), request.Stream, request.Tools)

	// 提取最后一条用户消息作为查询
	var userQuery string
	for i := len(request.Messages) - 1; i >= 0; i-- {
		if request.Messages[i].Role == "user" {
			userQuery = request.Messages[i].Content
			break
		}
	}

	if userQuery == "" {
		log.WarnF("No user message found in conversation")
		http.Error(w, "No user query found", http.StatusBadRequest)
		return http.StatusBadRequest
	}

	log.DebugF("Extracted user query: %s", userQuery)

	// 保存对话到博客
	log.Debug("Starting background conversation save to blog...")
	go saveConversationToBlog(request.Messages)

	// 设置流式响应头
	log.Debug("Setting up streaming response headers...")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.ErrorF("Streaming not supported by response writer")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}

	// 使用流式处理查询，直接转发LLM的流式响应
	log.InfoF("Processing query with streaming LLM: %s", userQuery)
	err = processQueryStreaming(userQuery, request.Tools, w, flusher)
	if err != nil {
		log.ErrorF("Streaming ProcessQuery failed: %v", err)
		fmt.Fprintf(w, "data: Error processing query: %v\n\n", err)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return http.StatusInternalServerError
	}

	// 发送完成信号
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	log.Debug("=== Assistant Chat Request Completed (MCP Mode) ===")

	return http.StatusOK
}

// 保存对话到博客
// 保存LLM完整响应到日记
func saveLLMResponseToDiary(userQuery, llmResponse string) {
	if userQuery == "" || llmResponse == "" {
		return
	}

	// 获取当前日期，使用日记格式
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	diaryTitle := fmt.Sprintf("AI_assistant_%s", dateStr)

	log.DebugF("准备保存LLM响应到日记: %s", diaryTitle)

	// 构建新的对话记录内容
	newEntry := fmt.Sprintf(`

### 🤖 AI助手对话 (%s)

**用户问题：**
%s

**AI回复：**
%s

---
`, now.Format("15:04:05"), userQuery, llmResponse)

	// 检查是否已存在当天日记
	existingBlog := control.GetBlog(diaryTitle)
	var finalContent string

	if existingBlog != nil {
		// 追加到现有日记
		log.DebugF("发现已存在的日记，追加内容")
		finalContent = existingBlog.Content + newEntry

		// 修改现有博客
		blogData := &module.UploadedBlogData{
			Title:    diaryTitle,
			Content:  finalContent,
			Tags:     existingBlog.Tags,
			AuthType: existingBlog.AuthType,
			Encrypt:  existingBlog.Encrypt,
		}
		control.ModifyBlog(blogData)
		log.InfoF("LLM响应已追加到现有日记: %s", diaryTitle)
	} else {
		// 创建新的日记
		log.DebugF("创建新的日记")
		finalContent = fmt.Sprintf(`# %s 日记

*今日开始记录...*%s`, dateStr, newEntry)

		// 创建新博客 - 使用日记权限
		blogData := &module.UploadedBlogData{
			Title:    diaryTitle,
			Content:  finalContent,
			Tags:     "日记|AI助手|自动生成",
			AuthType: module.EAuthType_diary, // 使用日记权限
		}
		control.AddBlog(blogData)
		log.InfoF("LLM响应已保存到新日记: %s", diaryTitle)
	}
}

// ProcessQuery uses LLM and MCP server tools to process query
func processQuery(query string, selectedTools []string) (string, error) {
	log.DebugF("llm === Processing Query with LLM and MCP Tools ===")
	log.DebugF("llm Query: %s", query)
	log.DebugF("llm Selected tools: %v", selectedTools)

	// Initialize messages
	messages := []Message{
		{
			Role:    "system",
			Content: "你是一个万能助手，自行决定是否调用工具获取数据，当你得到工具返回结果后，就不需要调用相同工具了，最后返回简单直接的结果给用户。",
		},
		{
			Role:    "user",
			Content: query,
		},
	}

	log.InfoF("llm request: %v, selected_tools: %v", messages, selectedTools)

	// Get available tools
	availableTools := mcp.GetAvailableLLMTools(selectedTools)

	// Initial LLM call
	response, err := callLLM(messages, availableTools)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %v", err)
	}

	log.DebugF("llm response callLLM response=%v", response)

	finalText := []string{}
	message := response.Choices[0].Message
	if message.Content != "" {
		finalText = append(finalText, message.Content)
	}
	log.InfoF("llm choices[0] message: %v", message)

	// Tool calling loop with max iterations
	maxCall := 25
	for len(message.ToolCalls) > 0 && maxCall > 0 {
		maxCall--
		log.DebugF("Tool calling iteration, remaining: %d", maxCall)

		// Process each tool call
		for _, toolCall := range message.ToolCalls {
			toolName := toolCall.Function.Name
			toolArgs := make(map[string]interface{})

			// Parse tool arguments
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs); err != nil {
				log.ErrorF("Failed to parse tool arguments: %v", err)
				continue
			}

			// Call the tool
			log.InfoF("toocall begin: %s %v", toolName, toolArgs)
			result := mcp.CallMCPTool(toolName, toolArgs)
			finalText = append(finalText, fmt.Sprintf("[Calling tool %s with args %v]\n", toolName, toolArgs))
			log.InfoF("toocall result: %s %v %v", toolName, toolArgs, result)

			// Add tool call and result to message history
			messages = append(messages, Message{
				Role:      "assistant",
				ToolCalls: []mcp.ToolCall{toolCall},
			})

			messages = append(messages, Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    fmt.Sprintf("%v", result.Result),
			})
		}

		// Next LLM call with updated messages
		log.InfoF("toolcall send to llm: %v", messages)
		response, err = callLLM(messages, availableTools)
		if err != nil {
			log.ErrorF("LLM call failed in tool loop: %v", err)
			break
		}

		message = response.Choices[0].Message
		log.InfoF("toolcall llm response: %v", message)
		if message.Content != "" {
			finalText = append(finalText, message.Content)
		}
	}

	// Join final text parts with double newlines to preserve markdown structure
	result := strings.Join(finalText, "\n")
	log.InfoF("llm Final result length: %d characters result=%s", len(result), result)
	return result, nil
}

// callLLM makes a request to the LLM API
func callLLM(messages []Message, tools []mcp.LLMTool) (*LLMResponse, error) {
	request := LLMRequest{
		Model:       llmConfig.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: llmConfig.Temperature,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	log.DebugF("llm request llmConfig.BaseURL=%s requestJSON=%s", llmConfig.BaseURL, string(requestJSON))
	req, err := http.NewRequest("POST", llmConfig.BaseURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+llmConfig.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	log.DebugF("llm response raw body data resp.Body=%v body=%s", resp.Body, string(body))

	var response LLMResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	log.DebugF("llm response unmarshal response=%v", response)

	return &response, nil
}

// 原有的保存对话功能，现在重构为保存用户问题的占位符
func saveConversationToBlog(messages []Message) {
	if len(messages) == 0 {
		return
	}

	// 获取用户的最后一条消息
	var userMessage string
	for _, msg := range messages {
		if msg.Role == "user" {
			userMessage = msg.Content
		}
	}

	if userMessage == "" {
		return
	}

	log.DebugF("保存用户问题到对话记录: %s", userMessage)
	// 这里可以预先保存用户问题，实际的LLM响应将由saveLLMResponseToDiary处理
}

// processQueryStreaming 支持工具调用的流式处理LLM响应
func processQueryStreaming(query string, selectedTools []string, w http.ResponseWriter, flusher http.Flusher) error {
	log.DebugF("=== Streaming LLM Processing Started with Tool Support ===")
	log.DebugF("Query: %s", query)
	log.DebugF("Selected tools: %v", selectedTools)

	// Initialize messages
	messages := []Message{
		{
			Role:    "system",
			Content: "你是一个万能助手，自行决定是否调用工具获取数据，当你得到工具返回结果后，就不需要调用相同工具了，最后返回简单直接的结果给用户。",
		},
		{
			Role:    "user",
			Content: query,
		},
	}

	// Get available tools
	availableTools := mcp.GetAvailableLLMTools(selectedTools)
	log.DebugF("Available LLM tools: %d", len(availableTools))

	var fullResponse strings.Builder

	// Initial LLM call
	_, toolCalls, err := sendStreamingLLMRequest(messages, availableTools, w, flusher, &fullResponse)
	if err != nil {
		log.ErrorF("Initial streaming LLM request failed: %v", err)
		return fmt.Errorf("initial streaming LLM request failed: %v", err)
	}

	// Tool calling loop with max iterations
	maxCall := 25
	for len(toolCalls) > 0 && maxCall > 0 {
		maxCall--
		log.DebugF("Tool calling iteration, remaining: %d", maxCall)

		// Process tool calls
		log.DebugF("Processing %d tool calls", len(toolCalls))
		for _, toolCall := range toolCalls {
			// Log tool call status but don't send to client to keep response clean
			log.DebugF(fmt.Sprintf("\n[Calling tool %s with args %s]\n", toolCall.Function.Name, toolCall.Function.Arguments))

			toolName := toolCall.Function.Name
			toolArgs := make(map[string]interface{})

			fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(fmt.Sprintf("[Calling tool %s with args %s]", toolCall.Function.Name, toolCall.Function.Arguments)))
			flusher.Flush()

			// Parse tool arguments with validation
			if toolCall.Function.Arguments == "" {
				log.WarnF("Tool call %s has empty arguments, skipping", toolName)
				continue
			}

			// Validate JSON format first
			if !isValidJSON(toolCall.Function.Arguments) {
				log.ErrorF("Tool call %s has invalid JSON arguments: %s", toolName, toolCall.Function.Arguments)
				continue
			}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs); err != nil {
				log.ErrorF("Failed to parse tool arguments for %s: %v, args: %s", toolName, err, toolCall.Function.Arguments)
				continue
			}

			// Call the tool
			log.InfoF("Tool call begin: %s %v", toolName, toolArgs)
			result := mcp.CallMCPTool(toolName, toolArgs)
			log.InfoF("Tool call result: %s %v %v", toolName, toolArgs, result)

			// Add tool call and result to message history
			messages = append(messages, Message{
				Role:      "assistant",
				ToolCalls: []mcp.ToolCall{toolCall},
			})

			messages = append(messages, Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    fmt.Sprintf("%v", result.Result),
			})

			// Add tool call info to full response for saving
			fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %v]\n", toolName, result.Result))

			// Tool result is now processed through LLM, no need to add directly to response
		}

		// Next LLM call with updated messages
		log.InfoF("Tool calls processed, sending next LLM request")
		_, toolCalls, err = sendStreamingLLMRequest(messages, availableTools, w, flusher, &fullResponse)
		if err != nil {
			log.ErrorF("LLM call failed in tool loop: %v", err)
			break
		}
		log.InfoF("Next LLM response received, tool calls: %d", len(toolCalls))
	}

	// Send completion signal to client
	log.DebugF("Tool processing complete, sending DONE signal")
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Save complete response to diary
	go saveLLMResponseToDiary(query, fullResponse.String())
	return nil
}

// sendStreamingLLMRequest 发送流式LLM请求并检测工具调用
func sendStreamingLLMRequest(messages []Message, availableTools []mcp.LLMTool, w http.ResponseWriter, flusher http.Flusher, fullResponse *strings.Builder) (string, []mcp.ToolCall, error) {
	// Create LLM request with streaming enabled
	requestBody := map[string]interface{}{
		"model":       llmConfig.Model,
		"messages":    messages,
		"tools":       availableTools,
		"temperature": llmConfig.Temperature,
		"stream":      true, // 启用流式响应
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.ErrorF("Error marshaling LLM request: %v", err)
		return "", nil, fmt.Errorf("error marshaling request: %v", err)
	}

	log.DebugF("Sending streaming request to LLM API: %s", llmConfig.BaseURL)

	// Create HTTP request to LLM API
	req, err := http.NewRequest("POST", llmConfig.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.ErrorF("Error creating LLM request: %v", err)
		return "", nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+llmConfig.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	// Send request with streaming support
	client := &http.Client{
		Timeout: 300 * time.Second, // 5分钟超时
	}

	resp, err := client.Do(req)
	if err != nil {
		log.ErrorF("Error sending request to LLM API: %v", err)
		return "", nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.ErrorF("LLM API returned status %d: %s", resp.StatusCode, string(body))
		return "", nil, fmt.Errorf("LLM API error: %d", resp.StatusCode)
	}

	log.DebugF("Received streaming response from LLM API, processing...")

	// Process the streaming response
	return processStreamingResponseWithToolDetection(resp.Body, w, flusher, fullResponse)
}

// processStreamingResponseWithToolDetection 处理流式响应并检测工具调用
func processStreamingResponseWithToolDetection(responseBody io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, fullResponse *strings.Builder) (string, []mcp.ToolCall, error) {
	log.DebugF("Starting streaming response processing with tool detection")
	scanner := bufio.NewScanner(responseBody)
	var responseContent strings.Builder
	var toolCalls []mcp.ToolCall
	var currentToolCall *mcp.ToolCall

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle SSE format: "data: ..."
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Handle completion signal
			if data == "[DONE]" {
				log.DebugF("LLM streaming completed")
				break
			}

			// Parse JSON chunk
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.WarnF("Failed to parse streaming chunk: %v", err)
				continue
			}

			// Extract content from chunk
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {

						// Handle regular content
						if content, ok := delta["content"].(string); ok && content != "" {
							log.DebugF("Tool-aware streaming: forwarding content chunk: %s", content)
							// Forward content to client immediately
							fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(content))
							flusher.Flush()

							// Accumulate for processing and saving
							responseContent.WriteString(content)
							fullResponse.WriteString(content)
						}

						// Handle tool calls
						if toolCallsRaw, ok := delta["tool_calls"].([]interface{}); ok {
							for _, toolCallRaw := range toolCallsRaw {
								if toolCallMap, ok := toolCallRaw.(map[string]interface{}); ok {
									// Parse tool call
									if err := parseToolCallFromDelta(toolCallMap, &currentToolCall, &toolCalls); err != nil {
										log.ErrorF("Failed to parse tool call: %v", err)
									}
								}
							}
						}

						// Check for finish reason
						if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" && finishReason != "null" {
							log.DebugF("Finish reason: %s", finishReason)
							if finishReason == "tool_calls" {
								log.DebugF("Tool calls detected, finishing content streaming")
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF("Error reading streaming response: %v", err)
		return "", nil, fmt.Errorf("error reading stream: %v", err)
	}

	log.DebugF("Streaming response processed. Content length: %d, Tool calls: %d", responseContent.Len(), len(toolCalls))
	return responseContent.String(), toolCalls, nil
}

// parseToolCallFromDelta 解析增量工具调用数据
func parseToolCallFromDelta(toolCallMap map[string]interface{}, currentToolCall **mcp.ToolCall, toolCalls *[]mcp.ToolCall) error {
	index, hasIndex := toolCallMap["index"].(float64)
	if !hasIndex {
		log.WarnF("Tool call chunk missing index, skipping")
		return nil
	}

	// Initialize new tool call if needed
	if *currentToolCall == nil || int(index) != len(*toolCalls) {
		*currentToolCall = &mcp.ToolCall{}
		if id, ok := toolCallMap["id"].(string); ok {
			(*currentToolCall).ID = id
		}
		if typeStr, ok := toolCallMap["type"].(string); ok && typeStr == "function" {
			(*currentToolCall).Type = typeStr
		}
	}

	// Parse function details
	if function, ok := toolCallMap["function"].(map[string]interface{}); ok {
		if name, ok := function["name"].(string); ok {
			(*currentToolCall).Function.Name = name
		}
		if arguments, ok := function["arguments"].(string); ok {
			(*currentToolCall).Function.Arguments += arguments
		}
	}

	// If this tool call seems complete, add it to the list
	if (*currentToolCall).ID != "" && (*currentToolCall).Function.Name != "" && (*currentToolCall).Function.Arguments != "" {
		// Validate that arguments is valid JSON before adding to list
		if isValidJSON((*currentToolCall).Function.Arguments) {
			// Check if this tool call is already in the list
			found := false
			for i, tc := range *toolCalls {
				if tc.ID == (*currentToolCall).ID {
					(*toolCalls)[i] = **currentToolCall // Update existing
					found = true
					break
				}
			}
			if !found {
				*toolCalls = append(*toolCalls, **currentToolCall)
			}
		} else {
			log.DebugF("Tool call arguments not yet complete: %s", (*currentToolCall).Function.Arguments)
		}
	}

	return nil
}

// isValidJSON 检查字符串是否为有效的JSON
func isValidJSON(str string) bool {
	var js interface{}
	return json.Unmarshal([]byte(str), &js) == nil
}

// forwardStreamingResponse 转发LLM的流式响应到客户端
func forwardStreamingResponse(responseBody io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, originalQuery string) error {
	scanner := bufio.NewScanner(responseBody)
	var fullResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle SSE format: "data: ..."
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Handle completion signal
			if data == "[DONE]" {
				log.DebugF("LLM streaming completed")
				// 保存完整响应到日记
				go saveLLMResponseToDiary(originalQuery, fullResponse.String())
				return nil
			}

			// Parse JSON chunk
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.WarnF("Failed to parse streaming chunk: %v", err)
				continue
			}

			// Extract content from chunk
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							// Forward content to client immediately
							fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(content))
							flusher.Flush()

							// Accumulate for saving
							fullResponse.WriteString(content)
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF("Error reading streaming response: %v", err)
		return fmt.Errorf("error reading stream: %v", err)
	}

	// Save final response
	go saveLLMResponseToDiary(originalQuery, fullResponse.String())
	return nil
}
