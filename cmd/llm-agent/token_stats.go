package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// TokenUsage 单次 LLM 调用的 token 用量
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Model            string
	Timestamp        time.Time
}

// ModelTokenStats 单模型统计
type ModelTokenStats struct {
	Prompt     int64 `json:"prompt"`
	Completion int64 `json:"completion"`
	Total      int64 `json:"total"`
	Calls      int64 `json:"calls"`
}

// TokenStats 全局 token 统计（线程安全）
type TokenStats struct {
	mu              sync.Mutex
	TotalPrompt     int64                      `json:"total_prompt"`
	TotalCompletion int64                      `json:"total_completion"`
	TotalTokens     int64                      `json:"total_tokens"`
	CallCount       int64                      `json:"call_count"`
	ByModel         map[string]*ModelTokenStats `json:"by_model"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	persistPath     string
}

// NewTokenStats 创建 token 统计器
func NewTokenStats(persistPath string) *TokenStats {
	return &TokenStats{
		ByModel:     make(map[string]*ModelTokenStats),
		persistPath: persistPath,
	}
}

// Add 累加一次 LLM 调用的 token 用量
func (ts *TokenStats) Add(usage TokenUsage) {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.TotalPrompt += int64(usage.PromptTokens)
	ts.TotalCompletion += int64(usage.CompletionTokens)
	ts.TotalTokens += int64(usage.TotalTokens)
	ts.CallCount++
	ts.UpdatedAt = time.Now()

	// 分模型统计
	if usage.Model != "" {
		ms, ok := ts.ByModel[usage.Model]
		if !ok {
			ms = &ModelTokenStats{}
			ts.ByModel[usage.Model] = ms
		}
		ms.Prompt += int64(usage.PromptTokens)
		ms.Completion += int64(usage.CompletionTokens)
		ms.Total += int64(usage.TotalTokens)
		ms.Calls++
	}

	log.Printf("[TokenStats] model=%s prompt=%d completion=%d total=%d | 累计: prompt=%d completion=%d total=%d calls=%d",
		usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens,
		ts.TotalPrompt, ts.TotalCompletion, ts.TotalTokens, ts.CallCount)

	// 自动持久化
	ts.saveLocked()
}

// Summary 返回人类可读的 token 用量摘要
func (ts *TokenStats) Summary() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.CallCount == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 Token 用量: 输入 %s / 输出 %s / 共 %s (%d次调用)",
		formatTokenCount(ts.TotalPrompt),
		formatTokenCount(ts.TotalCompletion),
		formatTokenCount(ts.TotalTokens),
		ts.CallCount))

	// 分模型明细
	if len(ts.ByModel) > 1 {
		sb.WriteString("\n")
		for model, ms := range ts.ByModel {
			sb.WriteString(fmt.Sprintf("  · %s: %s (%d次)", model, formatTokenCount(ms.Total), ms.Calls))
		}
	}

	return sb.String()
}

// Reset 重置所有计数
func (ts *TokenStats) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.TotalPrompt = 0
	ts.TotalCompletion = 0
	ts.TotalTokens = 0
	ts.CallCount = 0
	ts.ByModel = make(map[string]*ModelTokenStats)
	ts.UpdatedAt = time.Now()
	ts.saveLocked()
}

// Save 持久化到 JSON 文件
func (ts *TokenStats) Save() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.saveLocked()
}

// saveLocked 内部持久化（调用方需持有锁）
func (ts *TokenStats) saveLocked() {
	if ts.persistPath == "" {
		return
	}
	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		log.Printf("[TokenStats] marshal error: %v", err)
		return
	}
	if err := os.WriteFile(ts.persistPath, data, 0644); err != nil {
		log.Printf("[TokenStats] save error: %v", err)
	}
}

// Load 从 JSON 文件恢复统计数据
func (ts *TokenStats) Load() {
	if ts.persistPath == "" {
		return
	}
	data, err := os.ReadFile(ts.persistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[TokenStats] load error: %v", err)
		}
		return
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	var loaded TokenStats
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("[TokenStats] parse error: %v", err)
		return
	}

	ts.TotalPrompt = loaded.TotalPrompt
	ts.TotalCompletion = loaded.TotalCompletion
	ts.TotalTokens = loaded.TotalTokens
	ts.CallCount = loaded.CallCount
	ts.UpdatedAt = loaded.UpdatedAt
	if loaded.ByModel != nil {
		ts.ByModel = loaded.ByModel
	}

	log.Printf("[TokenStats] loaded: prompt=%d completion=%d total=%d calls=%d",
		ts.TotalPrompt, ts.TotalCompletion, ts.TotalTokens, ts.CallCount)
}

// formatTokenCount 格式化 token 数量（带千分位逗号）
func formatTokenCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// 从右往左每 3 位加逗号
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
