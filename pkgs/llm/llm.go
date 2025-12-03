package llm

import (
	"auth"
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

var max_selected_tools = 50

// New: clamp configuration to prevent context overflow
const (
	maxToolResultChars    = 4000   // per tool result passed back to the model
	maxToolArgumentsChars = 4000   // per tool-call arguments embedded in assistant message
	maxMessageChars       = 8000   // per message content clamp
	maxMessagesToSend     = 60     // overall message count cap
	maxTotalCharsBudget   = 200000 // rough total-char budget for all messages
)

// New: helper to truncate strings with a marker
func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "... [truncated]"
}

// New: sanitize a single tool call (limit arguments size)
func sanitizeToolCall(tc mcp.ToolCall) mcp.ToolCall {
	if len(tc.Function.Arguments) > maxToolArgumentsChars {
		tc.Function.Arguments = truncateString(tc.Function.Arguments, maxToolArgumentsChars)
	}
	return tc
}

// New: sanitize/prune messages to stay within budget (default limits)
func sanitizeMessages(original []Message) []Message {
	return sanitizeMessagesWithLimits(original, maxMessageChars, maxTotalCharsBudget, maxMessagesToSend)
}

// New: same logic with adjustable limits for retry
func sanitizeMessagesWithLimits(original []Message, perMessageMax, totalBudget, maxMsgs int) []Message {
	var totalChars int
	var resultReversed []Message

	// Preserve the first system message if present
	var system *Message
	if len(original) > 0 && original[0].Role == "system" {
		sys := original[0]
		if len(sys.Content) > perMessageMax {
			sys.Content = truncateString(sys.Content, perMessageMax)
		}
		system = &sys
	}

	// Walk from end to start so we keep the most recent turns
	for i := len(original) - 1; i >= 0; i-- {
		if system != nil && i == 0 {
			continue
		}

		msg := original[i]

		// Clamp message content
		if msg.Content != "" && len(msg.Content) > perMessageMax {
			msg.Content = truncateString(msg.Content, perMessageMax)
		}

		// Clamp any tool calls embedded in assistant message
		if len(msg.ToolCalls) > 0 {
			sanitizedCalls := make([]mcp.ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				sanitizedCalls = append(sanitizedCalls, sanitizeToolCall(tc))
			}
			msg.ToolCalls = sanitizedCalls
		}

		// Rough contribution to budget
		approx := len(msg.Content)
		for _, tc := range msg.ToolCalls {
			approx += len(tc.Function.Name) + len(tc.Function.Arguments) + len(tc.ID)
		}

		// Enforce message count cap (reserve one slot for system if any)
		if len(resultReversed) >= maxMsgs-1 {
			break
		}
		// Enforce total char budget
		if totalChars+approx > totalBudget {
			break
		}

		totalChars += approx
		resultReversed = append(resultReversed, msg)
	}

	// Reverse back to chronological order
	for i, j := 0, len(resultReversed)-1; i < j; i, j = i+1, j-1 {
		resultReversed[i], resultReversed[j] = resultReversed[j], resultReversed[i]
	}

	if system != nil {
		return append([]Message{*system}, resultReversed...)
	}
	return resultReversed
}

func Info() {
	log.Debug(log.ModuleLLM, "info llm v1.0")
}

// getConfigWithDefault 获取配置值，如果为空则使用默认值
func getConfigWithDefault(key, defaultValue string) string {
	value := config.GetConfigWithAccount(config.GetAdminAccount(), key)
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
	log.InfoF(log.ModuleLLM, "Init config %v", llmConfig)
}

func Init() error {
	InitConfig()
	return nil
}

func ProcessRequest(r *http.Request, w http.ResponseWriter) int {
	if r.Method != http.MethodPost {
		log.WarnF(log.ModuleLLM, "Invalid method %s for assistant chat from %s", r.Method, r.RemoteAddr)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return http.StatusMethodNotAllowed
	}

	// 读取请求体
	log.Debug(log.ModuleLLM, "Reading assistant chat request body...")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Error reading assistant chat request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
	defer r.Body.Close()

	log.DebugF(log.ModuleLLM, "Received assistant chat request body: %d bytes", len(body))

	// 解析请求
	var request struct {
		Messages    []Message `json:"messages"`
		Stream      bool      `json:"stream"`
		Tools       []string  `json:"selected_tools,omitempty"`
		TypingSpeed string    `json:"typing_speed,omitempty"` // 打字机速度设置
	}

	if err := json.Unmarshal(body, &request); err != nil {
		log.ErrorF(log.ModuleLLM, "Error parsing assistant chat request body: %v", err)
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return http.StatusBadRequest
	}

	log.InfoF(log.ModuleLLM, "Assistant chat request parsed: %d messages, stream=%t, tools=%v", len(request.Messages), request.Stream, request.Tools)

	// 提取最后一条用户消息作为查询
	var userQuery string
	for i := len(request.Messages) - 1; i >= 0; i-- {
		if request.Messages[i].Role == "user" {
			userQuery = request.Messages[i].Content
			break
		}
	}

	if userQuery == "" {
		log.WarnF(log.ModuleLLM, "No user message found in conversation")
		http.Error(w, "No user query found", http.StatusBadRequest)
		return http.StatusBadRequest
	}

	log.DebugF(log.ModuleLLM, "Extracted user query: %s", userQuery)

	// 保存对话到博客
	//log.Debug("Starting background conversation save to blog...")
	//go saveConversationToBlog(request.Messages)

	// 设置流式响应头
	log.Debug(log.ModuleLLM, "Setting up streaming response headers...")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.ErrorF(log.ModuleLLM, "Streaming not supported by response writer")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}

	// 使用流式处理查询，直接转发LLM的流式响应
	session, err := r.Cookie("session")
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Error getting session cookie: %v", err)
		http.Error(w, "Error getting session cookie", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
	account := auth.GetAccountBySession(session.Value)
	log.InfoF(log.ModuleLLM, "Processing query with streaming LLM: account=%s %s", account, userQuery)
	err = processQueryStreaming(account, userQuery, request.Tools, w, flusher)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Streaming ProcessQuery failed: %v", err)
		fmt.Fprintf(w, "data: Error processing query: %v\n\n", err)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return http.StatusInternalServerError
	}

	// 发送完成信号
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	log.Debug(log.ModuleLLM, "=== Assistant Chat Request Completed (MCP Mode) ===")

	return http.StatusOK
}

// 保存对话到博客
// 保存LLM完整响应到日记
func saveLLMResponseToDiary(account, userQuery, llmResponse string) {
	if userQuery == "" || llmResponse == "" {
		return
	}

	// 获取当前日期，使用日记格式
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	diaryTitle := fmt.Sprintf("AI_assistant_%s", dateStr)

	log.DebugF(log.ModuleLLM, "准备保存LLM响应到日记: %s", diaryTitle)

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
	existingBlog := control.GetBlog(account, diaryTitle)
	var finalContent string

	if existingBlog != nil {
		// 追加到现有日记
		log.DebugF(log.ModuleLLM, "发现已存在的日记，追加内容")
		finalContent = existingBlog.Content + newEntry

		// 修改现有博客
		blogData := &module.UploadedBlogData{
			Title:    diaryTitle,
			Content:  finalContent,
			Tags:     existingBlog.Tags,
			AuthType: existingBlog.AuthType,
			Encrypt:  existingBlog.Encrypt,
		}
		control.ModifyBlog(account, blogData)
		log.InfoF(log.ModuleLLM, "LLM响应已追加到现有日记: %s", diaryTitle)
	} else {
		// 创建新的日记
		log.DebugF(log.ModuleLLM, "创建新的日记")
		finalContent = fmt.Sprintf(`# %s 日记

*今日开始记录...*%s`, dateStr, newEntry)

		// 创建新博客 - 使用日记权限
		blogData := &module.UploadedBlogData{
			Title:    diaryTitle,
			Content:  finalContent,
			Tags:     "日记|AI助手|自动生成",
			AuthType: module.EAuthType_diary, // 使用日记权限
		}
		control.AddBlog(account, blogData)
		log.InfoF(log.ModuleLLM, "LLM响应已保存到新日记: %s", diaryTitle)
	}
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

	log.DebugF(log.ModuleLLM, "保存用户问题到对话记录: %s", userMessage)
	// 这里可以预先保存用户问题，实际的LLM响应将由saveLLMResponseToDiary处理
}

// processQueryStreaming 支持工具调用的流式处理LLM响应
func processQueryStreaming(account string, query string, selectedTools []string, w http.ResponseWriter, flusher http.Flusher) error {
	log.DebugF(log.ModuleLLM, "=== Streaming LLM Processing Started with Tool Support ===")
	log.DebugF(log.ModuleLLM, "Query: account=%s %s", account, query)
	log.DebugF(log.ModuleLLM, "Selected tools: %v", selectedTools)
	if len(selectedTools) > max_selected_tools {
		log.WarnF(log.ModuleLLM, "Selected tools count is too large, max is %d", max_selected_tools)
		selectedTools = selectedTools[:max_selected_tools]
	}
	sys_promopt := fmt.Sprintf("使用%s账号作为参数,你是一个万能助手，自行决定是否调用工具获取数据，当你得到工具返回结果后，就不需要调用相同工具了，最后返回简单直接的结果给用户。", account)

	// Initialize messages
	messages := []Message{
		{
			Role:    "system",
			Content: sys_promopt,
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
	_, toolCalls, err := sendStreamingLLMRequest(messages, availableTools, w, flusher, &fullResponse)
	if err != nil {
		log.ErrorF(log.ModuleLLM, "Initial streaming LLM request failed: %v", err)
		return fmt.Errorf("initial streaming LLM request failed: %v", err)
	}

	// Tool calling loop with max iterations
	maxCall := 25
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

			fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(fmt.Sprintf("[Calling tool %s with args %s]", toolCall.Function.Name, toolCall.Function.Arguments)))
			flusher.Flush()

			// Parse tool arguments with validation
			if toolCall.Function.Arguments == "" {
				log.WarnF(log.ModuleLLM, "Tool call %s has empty arguments, skipping", toolName)
				continue
			}

			// Validate JSON format first
			if !isValidJSON(toolCall.Function.Arguments) {
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
				ToolCalls: []mcp.ToolCall{sanitizeToolCall(toolCall)},
			})

			toolContent := truncateString(fmt.Sprintf("%v", result.Result), maxToolResultChars)
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallId: toolCall.ID,
				Content:    toolContent,
			})

			// Add tool call info to full response for saving
			save := config.GetConfigWithAccount(config.GetAdminAccount(), "assistant_save_mcp_result")
			// len(result.Result) < 32 表示结果很短，不会是隐私数据，因此可以存入Assistant_xxx中
			if strings.ToLower(save) == "true" || len(fmt.Sprintf("%v", result.Result)) < 32 {
				fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %v]\n", toolName, result.Result))
			} else {
				// 不显示工具回调返回的数据，设计隐私，发送给llm无问题，但是缓存后显示在UI上就很麻烦了。
				fullResponse.WriteString(fmt.Sprintf("\n[Tool %s called with result: %s]\n", toolName, "###$#&$#*$)@$&$%&$())!@###"))
			}

		}

		// Next LLM call with updated messages
		log.InfoF(log.ModuleLLM, "Tool calls processed, sending next LLM request")
		_, toolCalls, err = sendStreamingLLMRequest(messages, availableTools, w, flusher, &fullResponse)
		if err != nil {
			log.ErrorF(log.ModuleLLM, "LLM call failed in tool loop: %v", err)
			break
		}
		log.InfoF(log.ModuleLLM, "Next LLM response received, tool calls: %d", len(toolCalls))
	}

	// Send completion signal to client
	log.DebugF(log.ModuleLLM, "Tool processing complete, sending DONE signal")
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Save complete response to diary
	go saveLLMResponseToDiary(account, query, fullResponse.String())
	return nil
}

// sendStreamingLLMRequest 发送流式LLM请求并检测工具调用
func sendStreamingLLMRequest(messages []Message, availableTools []mcp.LLMTool, w http.ResponseWriter, flusher http.Flusher, fullResponse *strings.Builder) (string, []mcp.ToolCall, error) {
	// Prepare sanitized messages to fit context budget
	sanitizedMessages := sanitizeMessages(messages)

	attempts := []struct {
		perMessageMax int
		totalBudget   int
		maxMsgs       int
	}{
		{maxMessageChars, maxTotalCharsBudget, maxMessagesToSend},
		{4000, 100000, 40}, // stricter fallback
		{2000, 60000, 30},  // most strict fallback
	}

	for idx, lim := range attempts {
		if idx > 0 {
			// recompute with stricter limits
			sanitizedMessages = sanitizeMessagesWithLimits(messages, lim.perMessageMax, lim.totalBudget, lim.maxMsgs)
		}

		// Create LLM request with streaming enabled
		requestBody := map[string]interface{}{
			"model":       llmConfig.Model,
			"messages":    sanitizedMessages,
			"tools":       availableTools,
			"temperature": llmConfig.Temperature,
			"stream":      true, // 启用流式响应
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			log.ErrorF(log.ModuleLLM, "Error marshaling LLM request: %v", err)
			return "", nil, fmt.Errorf("error marshaling request: %v", err)
		}

		log.DebugF(log.ModuleLLM, "Sending streaming request to LLM API: %s (attempt %d)", llmConfig.BaseURL, idx+1)

		// Create HTTP request to LLM API
		req, err := http.NewRequest("POST", llmConfig.BaseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.ErrorF(log.ModuleLLM, "Error creating LLM request: %v", err)
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
			log.ErrorF(log.ModuleLLM, "Error sending request to LLM API: %v", err)
			return "", nil, fmt.Errorf("error sending request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.ErrorF(log.ModuleLLM, "LLM API returned status %d: %s", resp.StatusCode, string(body))
			// retry only on context-length style errors, otherwise fail fast
			if resp.StatusCode == http.StatusBadRequest && (strings.Contains(strings.ToLower(string(body)), "maximum context length") || strings.Contains(strings.ToLower(string(body)), "context")) {
				if idx < len(attempts)-1 {
					log.WarnF(log.ModuleLLM, "Retrying with stricter message limits due to context length error")
					continue
				}
			}
			return "", nil, fmt.Errorf("LLM API error: %d", resp.StatusCode)
		}

		// Ensure body is closed after processing
		defer resp.Body.Close()

		log.DebugF(log.ModuleLLM, "Received streaming response from LLM API, processing...")

		// Process the streaming response
		return processStreamingResponseWithToolDetection(resp.Body, w, flusher, fullResponse)
	}

	return "", nil, fmt.Errorf("failed to send request after retries")
}

// processStreamingResponseWithToolDetection 处理流式响应并检测工具调用
func processStreamingResponseWithToolDetection(responseBody io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, fullResponse *strings.Builder) (string, []mcp.ToolCall, error) {
	log.DebugF(log.ModuleLLM, "Starting streaming response processing with tool detection")
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
				log.DebugF(log.ModuleLLM, "LLM streaming completed")
				break
			}

			// Parse JSON chunk
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.WarnF(log.ModuleLLM, "Failed to parse streaming chunk: %v", err)
				continue
			}

			// Extract content from chunk
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {

						// Handle regular content
						if content, ok := delta["content"].(string); ok && content != "" {
							log.DebugF(log.ModuleLLM, "Tool-aware streaming: forwarding content chunk: %s", content)
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
										log.ErrorF(log.ModuleLLM, "Failed to parse tool call: %v", err)
									}
								}
							}
						}

						// Check for finish reason
						if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" && finishReason != "null" {
							log.DebugF(log.ModuleLLM, "Finish reason: %s", finishReason)
							if finishReason == "tool_calls" {
								log.DebugF(log.ModuleLLM, "Tool calls detected, finishing content streaming")
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF(log.ModuleLLM, "Error reading streaming response: %v", err)
		return "", nil, fmt.Errorf("error reading stream: %v", err)
	}

	log.DebugF(log.ModuleLLM, "Streaming response processed. Content length: %d, Tool calls: %d", responseContent.Len(), len(toolCalls))
	return responseContent.String(), toolCalls, nil
}

// parseToolCallFromDelta 解析增量工具调用数据
func parseToolCallFromDelta(toolCallMap map[string]interface{}, currentToolCall **mcp.ToolCall, toolCalls *[]mcp.ToolCall) error {
	index, hasIndex := toolCallMap["index"].(float64)
	if !hasIndex {
		log.WarnF(log.ModuleLLM, "Tool call chunk missing index, skipping")
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
			log.DebugF(log.ModuleLLM, "Tool call arguments not yet complete: %s", (*currentToolCall).Function.Arguments)
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
func forwardStreamingResponse(account string, responseBody io.ReadCloser, w http.ResponseWriter, flusher http.Flusher, originalQuery string) error {
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
				log.DebugF(log.ModuleLLM, "LLM streaming completed")
				// 保存完整响应到日记
				go saveLLMResponseToDiary(account, originalQuery, fullResponse.String())
				return nil
			}

			// Parse JSON chunk
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.WarnF(log.ModuleLLM, "Failed to parse streaming chunk: %v", err)
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
		log.ErrorF(log.ModuleLLM, "Error reading streaming response: %v", err)
		return fmt.Errorf("error reading stream: %v", err)
	}

	// Save final response
	go saveLLMResponseToDiary(account, originalQuery, fullResponse.String())
	return nil
}
