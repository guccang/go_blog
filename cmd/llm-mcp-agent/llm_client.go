package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// 共享 HTTP 客户端，避免每次请求都重新建立 TCP/TLS 连接
var llmHTTPClient = &http.Client{
	Timeout: 180 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:     &tls.Config{},
		MaxIdleConns:         10,
		MaxIdleConnsPerHost:  5,
		IdleConnTimeout:      90 * time.Second,
		TLSHandshakeTimeout:  15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableKeepAlives:    false,
		ForceAttemptHTTP2:    true,
	},
}

// ========================= LLM 消息结构 =========================

// Message LLM 对话消息
type Message struct {
	Role       string     `json:"role"`                   // "system", "user", "assistant", "tool"
	Content    string     `json:"content,omitempty"`      // 文本内容
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant 返回的工具调用
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool 消息的关联 ID
}

// ToolCall LLM 返回的工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用详情
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// LLMTool LLM function calling 工具定义
type LLMTool struct {
	Type     string      `json:"type"` // "function"
	Function LLMFunction `json:"function"`
}

// LLMFunction 工具函数定义
type LLMFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ========================= LLM API 请求/响应 =========================

type llmRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []LLMTool `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type llmResponse struct {
	Choices []llmChoice `json:"choices"`
	Error   *llmError   `json:"error,omitempty"`
}

type llmChoice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type llmError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ========================= LLM API 客户端 =========================

// SendLLMRequest 发送 LLM 请求（同步），返回响应文本和工具调用
func SendLLMRequest(cfg *LLMConfig, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	// 构建消息摘要用于日志
	var msgSummary []string
	for _, m := range messages {
		msgSummary = append(msgSummary, fmt.Sprintf("%s(%d)", m.Role, len(m.Content)))
	}
	log.Printf("[LLM] → 同步请求 model=%s messages=[%s] tools=%d",
		cfg.Model, strings.Join(msgSummary, ","), len(tools))

	reqStart := time.Now()

	reqBody := llmRequest{
		Model:       cfg.Model,
		Messages:    messages,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %v", err)
	}

	url := fmt.Sprintf("%s/chat/completions", cfg.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[LLM] ✗ HTTP 请求失败 duration=%v error=%v", time.Since(reqStart), err)
		return "", nil, fmt.Errorf("http request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM] ✗ API错误 status=%d duration=%v body=%s", resp.StatusCode, time.Since(reqStart), string(body))
		return "", nil, fmt.Errorf("API error status=%d: %s", resp.StatusCode, string(body))
	}

	var llmResp llmResponse
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return "", nil, fmt.Errorf("parse response: %v", err)
	}

	if llmResp.Error != nil {
		return "", nil, fmt.Errorf("LLM error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
	}

	if len(llmResp.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices in response")
	}

	choice := llmResp.Choices[0]
	duration := time.Since(reqStart)

	// 构建工具调用摘要
	var tcNames []string
	for _, tc := range choice.Message.ToolCalls {
		tcNames = append(tcNames, tc.Function.Name)
	}
	log.Printf("[LLM] ← 同步响应 duration=%v finish=%s textLen=%d toolCalls=%d tools=%v",
		duration, choice.FinishReason, len(choice.Message.Content), len(choice.Message.ToolCalls), tcNames)

	return choice.Message.Content, choice.Message.ToolCalls, nil
}

// SendStreamingLLMRequest 发送流式 LLM 请求，逐 chunk 回调 onChunk，同时检测 tool_call
// 内置重试逻辑，遇到 unexpected EOF 等瞬态错误会自动重试
func SendStreamingLLMRequest(cfg *LLMConfig, messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	const maxRetries = 2

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[LLM] retry attempt %d/%d after error: %v", attempt, maxRetries, lastErr)
			time.Sleep(time.Duration(attempt) * time.Second) // 递增退避：1s, 2s
		}

		text, toolCalls, err := sendStreamingLLMRequestOnce(cfg, messages, tools, onChunk)
		if err == nil {
			return text, toolCalls, nil
		}

		// 只对瞬态错误重试（EOF、连接重置等）
		errMsg := err.Error()
		if strings.Contains(errMsg, "unexpected EOF") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "broken pipe") {
			lastErr = err
			continue
		}

		// 非瞬态错误直接返回
		return "", nil, err
	}
	return "", nil, fmt.Errorf("after %d retries: %v", maxRetries, lastErr)
}

// sendStreamingLLMRequestOnce 单次流式 LLM 请求
func sendStreamingLLMRequestOnce(cfg *LLMConfig, messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	// 构建消息摘要用于日志
	var msgSummary []string
	for _, m := range messages {
		msgSummary = append(msgSummary, fmt.Sprintf("%s(%d)", m.Role, len(m.Content)))
	}
	log.Printf("[LLM] → 流式请求 model=%s messages=[%s] tools=%d",
		cfg.Model, strings.Join(msgSummary, ","), len(tools))

	reqStart := time.Now()

	reqBody := struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Tools       []LLMTool `json:"tools,omitempty"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
		Temperature float64   `json:"temperature,omitempty"`
		Stream      bool      `json:"stream"`
	}{
		Model:       cfg.Model,
		Messages:    messages,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
		Stream:      true,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %v", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", cfg.BaseURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[LLM] ✗ HTTP 请求失败 duration=%v error=%v", time.Since(reqStart), err)
		return "", nil, fmt.Errorf("http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[LLM] ✗ API错误 status=%d duration=%v body=%s", resp.StatusCode, time.Since(reqStart), string(body))
		return "", nil, fmt.Errorf("API error status=%d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[LLM] ← 流式响应开始 首字节耗时=%v", time.Since(reqStart))
	text, toolCalls, err := parseStreamingResponse(resp.Body, onChunk)
	duration := time.Since(reqStart)

	if err != nil {
		log.Printf("[LLM] ✗ 流式解析失败 duration=%v error=%v", duration, err)
		return "", nil, err
	}

	var tcNames []string
	for _, tc := range toolCalls {
		tcNames = append(tcNames, tc.Function.Name)
	}
	log.Printf("[LLM] ← 流式响应完成 duration=%v textLen=%d toolCalls=%d tools=%v",
		duration, len(text), len(toolCalls), tcNames)

	return text, toolCalls, nil
}

// parseStreamingResponse 解析 SSE 流式响应，提取文本和 tool_calls
func parseStreamingResponse(body io.Reader, onChunk func(string)) (string, []ToolCall, error) {
	scanner := bufio.NewScanner(body)
	// 增大 buffer 以处理大 chunk
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	var fullText strings.Builder
	var toolCalls []ToolCall
	// 用于累积 tool_call 的增量数据
	toolCallBuilders := make(map[int]*ToolCall)

	for scanner.Scan() {
		line := scanner.Text()
		// log.Printf("[LLM Debug] Raw line: %s", line) // optional: too verbose, but we can print first few lines
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string                   `json:"content"`
					ToolCalls []map[string]interface{} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// 文本 chunk
		if delta.Content != "" {
			fullText.WriteString(delta.Content)
			if onChunk != nil {
				onChunk(delta.Content)
			}
		}

		// tool_call 增量
		for _, tcRaw := range delta.ToolCalls {
			idx := 0
			if idxF, ok := tcRaw["index"].(float64); ok {
				idx = int(idxF)
			}

			tc, exists := toolCallBuilders[idx]
			if !exists {
				tc = &ToolCall{}
				toolCallBuilders[idx] = tc
			}

			if id, ok := tcRaw["id"].(string); ok {
				tc.ID = id
			}
			if t, ok := tcRaw["type"].(string); ok {
				tc.Type = t
			}
			if fn, ok := tcRaw["function"].(map[string]interface{}); ok {
				if name, ok := fn["name"].(string); ok {
					tc.Function.Name = name
				}
				if args, ok := fn["arguments"].(string); ok {
					tc.Function.Arguments += args
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[LLM Debug] Scanner error: %v, text gathered so far: %s", err, fullText.String())
		return "", nil, fmt.Errorf("read stream: %v", err)
	}

	// 收集完整的 tool_calls
	for i := 0; i < len(toolCallBuilders); i++ {
		if tc, ok := toolCallBuilders[i]; ok && tc.ID != "" {
			toolCalls = append(toolCalls, *tc)
		}
	}

	return fullText.String(), toolCalls, nil
}
