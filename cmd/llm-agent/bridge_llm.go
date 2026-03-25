package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// sendLLM 带降级链的同步 LLM 请求
func (b *Bridge) sendLLM(messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	cfg := b.activeLLM.Get()
	if len(b.cfg.Fallbacks) == 0 {
		return SendLLMRequest(&cfg, messages, tools)
	}
	return SendLLMRequestWithFallback(&cfg, b.cfg.Fallbacks, b.fallbackCooldown(), messages, tools)
}

// sendStreamingLLM 带降级链的流式 LLM 请求
func (b *Bridge) sendStreamingLLM(messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	cfg := b.activeLLM.Get()
	if len(b.cfg.Fallbacks) == 0 {
		return SendStreamingLLMRequest(&cfg, messages, tools, onChunk, b.cfg.LLMCallIntervalSec)
	}
	return SendStreamingLLMRequestWithFallback(&cfg, b.cfg.Fallbacks, b.fallbackCooldown(), messages, tools, onChunk, b.cfg.LLMCallIntervalSec)
}

// GetLLMConfigForSource 返回指定来源渠道的 LLM 配置（primary + fallbacks）
// 无配置则返回全局默认
func (b *Bridge) GetLLMConfigForSource(source string) (*LLMConfig, []LLMConfig) {
	if sc, ok := b.sourceLLMs[source]; ok {
		return &sc.LLM, sc.Fallbacks
	}
	cfg := b.activeLLM.Get()
	return &cfg, b.cfg.Fallbacks
}

// sendLLMWithConfig 使用指定配置的同步 LLM 请求
func (b *Bridge) sendLLMWithConfig(cfg *LLMConfig, fallbacks []LLMConfig, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	if len(fallbacks) == 0 {
		return SendLLMRequest(cfg, messages, tools)
	}
	return SendLLMRequestWithFallback(cfg, fallbacks, b.fallbackCooldown(), messages, tools)
}

// sendStreamingLLMWithConfig 使用指定配置的流式 LLM 请求
func (b *Bridge) sendStreamingLLMWithConfig(cfg *LLMConfig, fallbacks []LLMConfig, messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	if len(fallbacks) == 0 {
		return SendStreamingLLMRequest(cfg, messages, tools, onChunk, b.cfg.LLMCallIntervalSec)
	}
	return SendStreamingLLMRequestWithFallback(cfg, fallbacks, b.fallbackCooldown(), messages, tools, onChunk, b.cfg.LLMCallIntervalSec)
}

// llmCompactMemory 使用 LLM 整理记忆：合并重复、提取模式、保留重要摘要
func (b *Bridge) llmCompactMemory(entries []MemoryEntry) ([]MemoryEntry, error) {
	// 构建当前记忆文本
	var memoryText strings.Builder
	for _, entry := range entries {
		memoryText.WriteString(fmt.Sprintf("[%s][%s] %s: %s\n", entry.Date, entry.Category, entry.Source, entry.Content))
	}

	prompt := fmt.Sprintf(`你是一个记忆整理助手。以下是 AI Agent 积累的 %d 条工作记忆，需要压缩整理。

规则：
1. 合并重复的错误记录，只保留一条并注明出现次数
2. 将多条相关错误提炼为一条 [pattern] 类型的经验总结
3. [solution] [pattern] [preference] 类型的记忆优先保留完整内容
4. [error] 类型只保留有代表性的，删除重复的
5. [auto_skill] 类型全部保留
6. 目标：压缩到 %d 条以内

输出格式（每条一行，严格遵循）：
[日期][类别] 来源: 内容

类别只能是: error, solution, pattern, preference, auto_skill
日期格式: 2006-01-02

当前记忆：
%s`, len(entries), len(entries)*2/3, memoryText.String())

	messages := []Message{
		{Role: "system", Content: "你是记忆整理助手，负责压缩和整理 AI Agent 的工作记忆。只输出整理后的记忆条目，不要输出其他内容。"},
		{Role: "user", Content: prompt},
	}

	text, _, err := b.sendLLM(messages, nil)
	if err != nil {
		return nil, fmt.Errorf("LLM compact: %v", err)
	}

	// 解析 LLM 输出为 MemoryEntry
	compacted := parseLLMCompactOutput(text)
	if len(compacted) == 0 {
		return nil, fmt.Errorf("LLM compact returned empty result")
	}

	log.Printf("[Memory] LLM 整理: %d → %d 条", len(entries), len(compacted))
	return compacted, nil
}

// llmCompactRules 使用 LLM 整理用户规则：去重、合并、精简
func (b *Bridge) llmCompactRules(content string) (string, error) {
	prompt := fmt.Sprintf(`你是一个规则整理助手。以下是用户给 AI 助手设定的规则和提醒，其中可能有重复或相似的内容。

请整理这些规则：
1. 合并含义相同或相似的规则
2. 删除完全重复的
3. 保持原始意图不变
4. 语言精简清晰

只输出整理后的规则内容，不要输出其他说明。

当前规则：
%s`, content)

	messages := []Message{
		{Role: "system", Content: "你是规则整理助手，负责去重合并用户设定的 AI 助手行为规则。只输出整理后的规则。"},
		{Role: "user", Content: prompt},
	}

	text, _, err := b.sendLLM(messages, nil)
	if err != nil {
		return "", fmt.Errorf("LLM compact rules: %v", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("LLM compact rules returned empty")
	}

	log.Printf("[Memory] LLM 规则整理: %d → %d 字符", len(content), len(text))
	return text, nil
}

// parseLLMCompactOutput 解析 LLM 压缩输出
// 格式: [2026-03-19][pattern] tool_call: 内容
func parseLLMCompactOutput(text string) []MemoryEntry {
	var entries []MemoryEntry
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "[") {
			continue
		}

		// 解析 [date][category] source: content
		closeDateBracket := strings.Index(line[1:], "]")
		if closeDateBracket < 0 {
			continue
		}
		date := line[1 : closeDateBracket+1]

		rest := line[closeDateBracket+2:]
		if !strings.HasPrefix(rest, "[") {
			continue
		}

		closeCatBracket := strings.Index(rest[1:], "]")
		if closeCatBracket < 0 {
			continue
		}
		category := rest[1 : closeCatBracket+1]

		afterCat := strings.TrimSpace(rest[closeCatBracket+2:])
		source := "unknown"
		content := afterCat
		if colonIdx := strings.Index(afterCat, ":"); colonIdx > 0 {
			source = strings.TrimSpace(afterCat[:colonIdx])
			content = strings.TrimSpace(afterCat[colonIdx+1:])
		}

		if content != "" {
			entries = append(entries, MemoryEntry{
				Date:     date,
				Category: category,
				Source:   source,
				Content:  content,
			})
		}
	}

	return entries
}

// WarmupLLM 预热 LLM 连接，提前建立 TCP+TLS 连接，避免首次请求 EOF
func WarmupLLM(cfg *LLMConfig) {
	url := fmt.Sprintf("%s/models", cfg.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[LLM-MCP] warmup: create request failed: %v", err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[LLM-MCP] warmup: request failed (non-critical): %v", err)
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // 消费 body 以确保连接可被复用
	log.Printf("[LLM-MCP] warmup: LLM connection established (status=%d)", resp.StatusCode)
}
