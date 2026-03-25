package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
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

// 全局 LLM 调用时间记录
var globalLastLLMCall time.Time

// ========================= 模型降级冷却 =========================

// modelCooldown 全局模型冷却追踪
type modelCooldown struct {
	mu        sync.RWMutex
	cooldowns map[string]time.Time // "baseURL|model" → 冷却到期时间
}

var globalCooldown = &modelCooldown{cooldowns: make(map[string]time.Time)}

func cooldownKey(cfg *LLMConfig) string {
	return cfg.BaseURL + "|" + cfg.EffectiveModel()
}

func (mc *modelCooldown) isCoolingDown(cfg *LLMConfig) bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	expiry, ok := mc.cooldowns[cooldownKey(cfg)]
	return ok && time.Now().Before(expiry)
}

func (mc *modelCooldown) setCooldown(cfg *LLMConfig, d time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.cooldowns[cooldownKey(cfg)] = time.Now().Add(d)
	log.Printf("[LLM-Fallback] 模型 %s 进入冷却 %v", cfg.EffectiveModel(), d)
}

// SendLLMRequestWithFallback 带降级链的同步 LLM 请求
func SendLLMRequestWithFallback(primary *LLMConfig, fallbacks []LLMConfig, cooldown time.Duration, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	candidates := make([]*LLMConfig, 0, 1+len(fallbacks))
	candidates = append(candidates, primary)
	for i := range fallbacks {
		candidates = append(candidates, &fallbacks[i])
	}

	var lastErr error
	for _, cfg := range candidates {
		if globalCooldown.isCoolingDown(cfg) {
			log.Printf("[LLM-Fallback] 跳过冷却中的模型 %s", cfg.EffectiveModel())
			continue
		}
		text, toolCalls, err := SendLLMRequest(cfg, messages, tools)
		if err == nil {
			return text, toolCalls, nil
		}
		lastErr = err
		globalCooldown.setCooldown(cfg, cooldown)
		log.Printf("[LLM-Fallback] 模型 %s 失败: %v, 尝试下一个", cfg.EffectiveModel(), err)
	}
	return "", nil, fmt.Errorf("all models failed, last error: %v", lastErr)
}

// SendStreamingLLMRequestWithFallback 带降级链的流式 LLM 请求
func SendStreamingLLMRequestWithFallback(primary *LLMConfig, fallbacks []LLMConfig, cooldown time.Duration, messages []Message, tools []LLMTool, onChunk func(string), intervalSec int) (string, []ToolCall, error) {
	candidates := make([]*LLMConfig, 0, 1+len(fallbacks))
	candidates = append(candidates, primary)
	for i := range fallbacks {
		candidates = append(candidates, &fallbacks[i])
	}

	var lastErr error
	for _, cfg := range candidates {
		if globalCooldown.isCoolingDown(cfg) {
			log.Printf("[LLM-Fallback] 跳过冷却中的模型 %s", cfg.EffectiveModel())
			continue
		}
		text, toolCalls, err := SendStreamingLLMRequest(cfg, messages, tools, onChunk, intervalSec)
		if err == nil {
			return text, toolCalls, nil
		}
		lastErr = err
		globalCooldown.setCooldown(cfg, cooldown)
		log.Printf("[LLM-Fallback] 流式模型 %s 失败: %v, 尝试下一个", cfg.EffectiveModel(), err)
	}
	return "", nil, fmt.Errorf("all models failed (streaming), last error: %v", lastErr)
}

// ========================= LLM 消息结构 =========================

// Message LLM 对话消息
type Message struct {
	Role       string     `json:"role"`                   // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`                // 文本内容
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
	Usage   *llmUsage   `json:"usage,omitempty"`
}

type llmUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// 全局 token 统计器
var globalTokenStats *TokenStats

// SetTokenStats 注入全局 token 统计器（由 Bridge 初始化时调用）
func SetTokenStats(ts *TokenStats) {
	globalTokenStats = ts
}

// recordTokenUsage 记录 token 用量到全局统计器
func recordTokenUsage(usage *llmUsage, model string) {
	if globalTokenStats == nil || usage == nil {
		return
	}
	globalTokenStats.Add(TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		Model:            model,
		Timestamp:        time.Now(),
	})
}

type llmChoice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type llmError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ========================= LLM 上下文日志 =========================

// logLLMContext 打印 LLM 调用时的完整上下文（消息 + 工具列表），不截断
func logLLMContext(tag string, cfg *LLMConfig, messages []Message, tools []LLMTool) {
	log.Printf("[LLM-Context][%s] ========== LLM 调用上下文 ==========", tag)
	log.Printf("[LLM-Context][%s] model=%s max_tokens=%d temperature=%.2f", tag, cfg.EffectiveModel(), cfg.MaxTokens, cfg.Temperature)
	log.Printf("[LLM-Context][%s] messages 数量: %d", tag, len(messages))

	for i, m := range messages {
		log.Printf("[LLM-Context][%s] msg[%d] role=%s content_len=%d content:\n%s",
			tag, i, m.Role, len(m.Content), m.Content)

		// 打印 assistant 消息中的 tool_calls
		if len(m.ToolCalls) > 0 {
			for j, tc := range m.ToolCalls {
				log.Printf("[LLM-Context][%s] msg[%d].tool_calls[%d] id=%s name=%s args=%s",
					tag, i, j, tc.ID, tc.Function.Name, tc.Function.Arguments)
			}
		}

		// 打印 tool 消息的 tool_call_id
		if m.ToolCallID != "" {
			log.Printf("[LLM-Context][%s] msg[%d] tool_call_id=%s", tag, i, m.ToolCallID)
		}
	}

	// 打印工具列表（含参数 schema 摘要）
	if len(tools) > 0 {
		var toolNames []string
		for _, t := range tools {
			toolNames = append(toolNames, t.Function.Name)
		}
		log.Printf("[LLM-Context][%s] tools(%d): %s", tag, len(tools), strings.Join(toolNames, ", "))

		// 打印每个工具的参数 schema 概要（property keys + required）
		for _, t := range tools {
			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			}
			if err := json.Unmarshal(t.Function.Parameters, &schema); err == nil && len(schema.Properties) > 0 {
				var propKeys []string
				for k := range schema.Properties {
					propKeys = append(propKeys, k)
				}
				log.Printf("[LLM-Context][%s]   %s: params={%s} required=%v",
					tag, t.Function.Name, strings.Join(propKeys, ", "), schema.Required)
			}
		}
	} else {
		log.Printf("[LLM-Context][%s] tools: (none)", tag)
	}
	log.Printf("[LLM-Context][%s] ========== 上下文结束 ==========", tag)
}

// ========================= LLM API 客户端 =========================

// stripThinkTags 移除 LLM 响应中的 <think>...</think> 标签（某些模型的思考过程）
func stripThinkTags(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "</think>")
		if end < 0 {
			// 没有闭合标签，移除 <think> 到末尾
			s = strings.TrimSpace(s[:start])
			break
		}
		s = s[:start] + s[start+end+8:]
	}
	return strings.TrimSpace(s)
}

// SendLLMRequest 发送 LLM 请求（同步），返回响应文本和工具调用
func SendLLMRequest(cfg *LLMConfig, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	// 构建消息摘要用于日志
	var msgSummary []string
	for _, m := range messages {
		msgSummary = append(msgSummary, fmt.Sprintf("%s(%d)", m.Role, len(m.Content)))
	}
	log.Printf("[LLM] → 同步请求 model=%s messages=[%s] tools=%d",
		cfg.EffectiveModel(), strings.Join(msgSummary, ","), len(tools))

	// 打印完整上下文
	logLLMContext("sync", cfg, messages, tools)

	reqStart := time.Now()

	reqBody := llmRequest{
		Model:       cfg.EffectiveModel(),
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

	// 记录 token 用量
	recordTokenUsage(llmResp.Usage, cfg.EffectiveModel())

	// 构建工具调用摘要
	var tcNames []string
	for _, tc := range choice.Message.ToolCalls {
		tcNames = append(tcNames, tc.Function.Name)
	}
	log.Printf("[LLM] ← 同步响应 duration=%v finish=%s textLen=%d toolCalls=%d tools=%v",
		duration, choice.FinishReason, len(choice.Message.Content), len(choice.Message.ToolCalls), tcNames)
	if choice.Message.Content != "" {
		log.Printf("[LLM] ← 响应文本:\n%s", choice.Message.Content)
	}
	for _, tc := range choice.Message.ToolCalls {
		log.Printf("[LLM] ← tool_call: name=%s args=%s", tc.Function.Name, tc.Function.Arguments)
	}

	// 同步请求也检测 max_tokens 截断
	if choice.FinishReason == "length" && len(choice.Message.ToolCalls) > 0 {
		lastTC := &choice.Message.ToolCalls[len(choice.Message.ToolCalls)-1]
		if !json.Valid([]byte(lastTC.Function.Arguments)) {
			log.Printf("[LLM] ⚠ 同步响应 tool_call arguments 被 max_tokens 截断: %s", lastTC.Function.Name)
			choice.Message.ToolCalls = choice.Message.ToolCalls[:len(choice.Message.ToolCalls)-1]
			choice.Message.Content += "\n\n[系统警告] 你的上一次工具调用因 max_tokens 限制被截断，代码未完整生成。请精简代码后重试，或拆分为多次调用。"
		}
	}

	return stripThinkTags(choice.Message.Content), choice.Message.ToolCalls, nil
}

// SendLLMRequestCtx context 感知的同步 LLM 请求，支持级联取消
func SendLLMRequestCtx(ctx context.Context, cfg *LLMConfig, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, fmt.Errorf("cancelled before LLM request: %v", err)
	}

	var msgSummary []string
	for _, m := range messages {
		msgSummary = append(msgSummary, fmt.Sprintf("%s(%d)", m.Role, len(m.Content)))
	}
	log.Printf("[LLM] → 同步请求(ctx) model=%s messages=[%s] tools=%d",
		cfg.EffectiveModel(), strings.Join(msgSummary, ","), len(tools))

	logLLMContext("sync-ctx", cfg, messages, tools)

	reqStart := time.Now()

	reqBody := llmRequest{
		Model:       cfg.EffectiveModel(),
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
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[LLM] ✗ HTTP 请求失败(ctx) duration=%v error=%v", time.Since(reqStart), err)
		return "", nil, fmt.Errorf("http request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM] ✗ API错误(ctx) status=%d duration=%v body=%s", resp.StatusCode, time.Since(reqStart), string(body))
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

	// 记录 token 用量
	recordTokenUsage(llmResp.Usage, cfg.EffectiveModel())

	var tcNames []string
	for _, tc := range choice.Message.ToolCalls {
		tcNames = append(tcNames, tc.Function.Name)
	}
	log.Printf("[LLM] ← 同步响应(ctx) duration=%v finish=%s textLen=%d toolCalls=%d tools=%v",
		duration, choice.FinishReason, len(choice.Message.Content), len(choice.Message.ToolCalls), tcNames)
	if choice.Message.Content != "" {
		log.Printf("[LLM] ← 响应文本(ctx):\n%s", choice.Message.Content)
	}
	for _, tc := range choice.Message.ToolCalls {
		log.Printf("[LLM] ← tool_call(ctx): name=%s args=%s", tc.Function.Name, tc.Function.Arguments)
	}

	if choice.FinishReason == "length" && len(choice.Message.ToolCalls) > 0 {
		lastTC := &choice.Message.ToolCalls[len(choice.Message.ToolCalls)-1]
		if !json.Valid([]byte(lastTC.Function.Arguments)) {
			log.Printf("[LLM] ⚠ 同步响应(ctx) tool_call arguments 被 max_tokens 截断: %s", lastTC.Function.Name)
			choice.Message.ToolCalls = choice.Message.ToolCalls[:len(choice.Message.ToolCalls)-1]
			choice.Message.Content += "\n\n[系统警告] 你的上一次工具调用因 max_tokens 限制被截断，代码未完整生成。请精简代码后重试，或拆分为多次调用。"
		}
	}

	return stripThinkTags(choice.Message.Content), choice.Message.ToolCalls, nil
}

// SendStreamingLLMRequest 发送流式 LLM 请求，逐 chunk 回调 onChunk，同时检测 tool_call
// 内置重试逻辑，遇到 unexpected EOF 等瞬态错误会自动重试
func SendStreamingLLMRequest(cfg *LLMConfig, messages []Message, tools []LLMTool, onChunk func(string), intervalSec int) (string, []ToolCall, error) {
	const maxRetries = 2

	// 调用间隔控制
	if intervalSec > 0 {
		elapsed := time.Since(globalLastLLMCall)
		if wait := time.Duration(intervalSec)*time.Second - elapsed; wait > 0 {
			waitSec := int(wait.Seconds())
			if waitSec >= 10 {
				onChunk(fmt.Sprintf("⏳ 等待 %d 秒后继续...\n", waitSec))
			} else if waitSec >= 5 {
				onChunk(fmt.Sprintf("⏳ 等待 %d 秒...\n", waitSec))
			} else if waitSec >= 1 {
				onChunk(fmt.Sprintf("⏳ %d 秒...\n", waitSec))
			}
			log.Printf("[LLM] 调用间隔控制，等待 %.1fs", wait.Seconds())
			time.Sleep(wait)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[LLM] retry attempt %d/%d after error: %v", attempt, maxRetries, lastErr)
			time.Sleep(time.Duration(attempt) * time.Second) // 递增退避：1s, 2s
		}

		text, toolCalls, err := sendStreamingLLMRequestOnce(cfg, messages, tools, onChunk)
		if err == nil {
			globalLastLLMCall = time.Now()
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
		cfg.EffectiveModel(), strings.Join(msgSummary, ","), len(tools))

	// 打印完整上下文
	logLLMContext("stream", cfg, messages, tools)

	reqStart := time.Now()

	reqBody := struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Tools       []LLMTool `json:"tools,omitempty"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
		Temperature float64   `json:"temperature,omitempty"`
		Stream      bool      `json:"stream"`
	}{
		Model:       cfg.EffectiveModel(),
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
	text, toolCalls, streamUsage, err := parseStreamingResponse(resp.Body, onChunk)
	duration := time.Since(reqStart)

	if err != nil {
		log.Printf("[LLM] ✗ 流式解析失败 duration=%v error=%v", duration, err)
		return "", nil, err
	}

	// 记录 token 用量
	recordTokenUsage(streamUsage, cfg.EffectiveModel())

	var tcNames []string
	for _, tc := range toolCalls {
		tcNames = append(tcNames, tc.Function.Name)
	}
	log.Printf("[LLM] ← 流式响应完成 duration=%v textLen=%d toolCalls=%d tools=%v",
		duration, len(text), len(toolCalls), tcNames)
	if text != "" {
		log.Printf("[LLM] ← 响应文本(stream):\n%s", text)
	}
	for _, tc := range toolCalls {
		log.Printf("[LLM] ← tool_call(stream): name=%s args=%s", tc.Function.Name, tc.Function.Arguments)
	}

	return stripThinkTags(text), toolCalls, nil
}

// parseStreamingResponse 解析 SSE 流式响应，提取文本、tool_calls 和 usage
func parseStreamingResponse(body io.Reader, onChunk func(string)) (string, []ToolCall, *llmUsage, error) {
	scanner := bufio.NewScanner(body)
	// 增大 buffer 以处理大 chunk
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	var fullText strings.Builder
	var toolCalls []ToolCall
	var streamUsage *llmUsage
	// 用于累积 tool_call 的增量数据
	toolCallBuilders := make(map[int]*ToolCall)
	truncatedByMaxTokens := false

	// think 标签过滤状态机
	inThink := false        // 当前是否在 <think> 块内
	var thinkBuf string     // 缓冲可能的标签片段

	// emitChunk 将非 think 内容发送给回调
	emitChunk := func(s string) {
		if s != "" && onChunk != nil {
			onChunk(s)
		}
	}

	// processContent 过滤 <think> 标签，只输出非 think 内容
	processContent := func(content string) {
		fullText.WriteString(content)
		remaining := thinkBuf + content
		thinkBuf = ""

		for len(remaining) > 0 {
			if inThink {
				// 在 think 块内，查找 </think>
				if idx := strings.Index(remaining, "</think>"); idx >= 0 {
					inThink = false
					remaining = remaining[idx+8:]
				} else {
					// 可能 </think> 被切分，保留尾部
					if len(remaining) >= 8 {
						// 检查尾部是否可能是 </think> 的前缀
						for i := 1; i < 8 && i <= len(remaining); i++ {
							if strings.HasPrefix("</think>", remaining[len(remaining)-i:]) {
								thinkBuf = remaining[len(remaining)-i:]
								remaining = remaining[:len(remaining)-i]
								break
							}
						}
					}
					remaining = ""
				}
			} else {
				// 不在 think 块内，查找 <think>
				if idx := strings.Index(remaining, "<think>"); idx >= 0 {
					emitChunk(remaining[:idx])
					inThink = true
					remaining = remaining[idx+7:]
				} else {
					// 可能 <think> 被切分，保留尾部
					safe := remaining
					for i := 1; i < 7 && i <= len(remaining); i++ {
						if strings.HasPrefix("<think>", remaining[len(remaining)-i:]) {
							safe = remaining[:len(remaining)-i]
							thinkBuf = remaining[len(remaining)-i:]
							break
						}
					}
					emitChunk(safe)
					remaining = ""
				}
			}
		}
	}

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
			Usage *llmUsage `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 捕获流式 usage（部分 API 在最后一个 chunk 或独立 chunk 中返回）
		if chunk.Usage != nil {
			streamUsage = chunk.Usage
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		// 检测 finish_reason: "length"（max_tokens 截断）
		if fr := chunk.Choices[0].FinishReason; fr != nil && *fr == "length" {
			truncatedByMaxTokens = true
			log.Printf("[LLM] ⚠ 响应被 max_tokens 截断 (finish_reason=length)")
		}

		delta := chunk.Choices[0].Delta

		// 文本 chunk（通过 processContent 过滤 think 标签）
		if delta.Content != "" {
			processContent(delta.Content)
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
		return "", nil, nil, fmt.Errorf("read stream: %v", err)
	}

	// 刷新 thinkBuf 中残留的非 think 内容
	if thinkBuf != "" && !inThink {
		emitChunk(thinkBuf)
	}

	// 收集完整的 tool_calls
	for i := 0; i < len(toolCallBuilders); i++ {
		if tc, ok := toolCallBuilders[i]; ok && tc.ID != "" {
			toolCalls = append(toolCalls, *tc)
		}
	}

	// max_tokens 截断检测：校验 tool_call arguments JSON 完整性
	if truncatedByMaxTokens && len(toolCalls) > 0 {
		lastTC := &toolCalls[len(toolCalls)-1]
		if !json.Valid([]byte(lastTC.Function.Arguments)) {
			log.Printf("[LLM] ⚠ 最后一个 tool_call arguments JSON 不完整（被 max_tokens 截断）: %s args_tail=%s",
				lastTC.Function.Name, tailStr(lastTC.Function.Arguments, 100))
			// 移除被截断的 tool_call，避免下游解析失败
			toolCalls = toolCalls[:len(toolCalls)-1]
			// 在文本中追加截断警告，让 LLM 知道发生了什么
			warning := "\n\n[系统警告] 你的上一次工具调用因 max_tokens 限制被截断，代码未完整生成。请精简代码后重试，或拆分为多次调用。"
			fullText.WriteString(warning)
			if onChunk != nil {
				onChunk(warning)
			}
		}
	}

	return fullText.String(), toolCalls, streamUsage, nil
}

// tailStr 返回字符串末尾 n 个字符（用于日志）
func tailStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return "..." + string(runes[len(runes)-n:])
}
