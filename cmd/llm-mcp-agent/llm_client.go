package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ========================= LLM 消息结构 =========================

// Message LLM 对话消息
type Message struct {
	Role       string     `json:"role"`                  // "system", "user", "assistant", "tool"
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

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("http request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM] API error status=%d body=%s", resp.StatusCode, string(body))
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
	return choice.Message.Content, choice.Message.ToolCalls, nil
}
