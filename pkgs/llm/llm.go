package llm

import (
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
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
		Tools    []string  `json:"selected_tools,omitempty"`
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

	// 使用MCP ProcessQuery处理查询
	log.InfoF("Processing query with MCP: %s", userQuery)
	result, err := processQuery(userQuery, request.Tools)
	if err != nil {
		log.ErrorF("MCP ProcessQuery failed: %v", err)
		fmt.Fprintf(w, "data: Error processing query: %v\n\n", err)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return http.StatusInternalServerError
	}

	log.InfoF("MCP ProcessQuery completed, result length: %d characters result=%s", len(result), result)

	// 保存完整的LLM响应到当天日记
	log.Debug("Saving LLM response to daily diary...")
	go saveLLMResponseToDiary(userQuery, result)

	// 以流式方式发送结果，保持原有的换行和空格格式
	// 按行处理，保留换行符
	lines := strings.Split(result, "\n")
	for lineIdx, line := range lines {
		if line == "" {
			// 发送空行（换行符）
			fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape("\n"))
			flusher.Flush()
			time.Sleep(30 * time.Millisecond)
		} else {
			// 按词发送每一行的内容，但保留行内的空格结构
			words := strings.Fields(line)
			for i, word := range words {
				if i < len(words)-1 {
					fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(word+" "))
				} else {
					// 最后一个词，如果不是最后一行，则加换行符
					if lineIdx < len(lines)-1 {
						fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(word+"\n"))
					} else {
						fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(word))
					}
				}
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
		}
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
