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
	RequestBytes     int64 // HTTP 请求体字节数
	ResponseBytes    int64 // HTTP 响应体字节数
}

// ModelTokenStats 单模型统计
type ModelTokenStats struct {
	Prompt     int64 `json:"prompt"`
	Completion int64 `json:"completion"`
	Total      int64 `json:"total"`
	Calls      int64 `json:"calls"`
	ReqBytes   int64 `json:"req_bytes"`
	RespBytes  int64 `json:"resp_bytes"`
	Errors5xx  int64 `json:"errors_5xx"`   // 5xx 错误次数
	Today5xx   int64 `json:"today_5xx"`    // 当日 5xx 错误次数
}

// TokenStats 全局 token 统计（线程安全）
type TokenStats struct {
	mu              sync.Mutex
	TotalPrompt     int64                      `json:"total_prompt"`
	TotalCompletion int64                      `json:"total_completion"`
	TotalTokens     int64                      `json:"total_tokens"`
	CallCount       int64                      `json:"call_count"`
	TotalReqBytes   int64                      `json:"total_req_bytes"`
	TotalRespBytes  int64                      `json:"total_resp_bytes"`
	Total5xxErrors  int64                      `json:"total_5xx_errors"`  // 5xx 错误总次数
	ByModel         map[string]*ModelTokenStats `json:"by_model"`
	// 当日统计
	TodayDate      string                     `json:"today_date"`
	TodayTokens    int64                      `json:"today_tokens"`
	TodayCallCount int64                      `json:"today_call_count"`
	TodayReqBytes  int64                      `json:"today_req_bytes"`
	TodayRespBytes int64                      `json:"today_resp_bytes"`
	Today5xxErrors int64                       `json:"today_5xx_errors"`   // 当日 5xx 错误次数
	TodayByModel   map[string]*ModelTokenStats `json:"today_by_model"`

	UpdatedAt   time.Time `json:"updated_at"`
	persistPath string
}

// NewTokenStats 创建 token 统计器
func NewTokenStats(persistPath string) *TokenStats {
	return &TokenStats{
		ByModel:      make(map[string]*ModelTokenStats),
		TodayByModel: make(map[string]*ModelTokenStats),
		persistPath:  persistPath,
	}
}

// Add 累加一次 LLM 调用的 token 用量
func (ts *TokenStats) Add(usage TokenUsage) {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.RequestBytes == 0 && usage.ResponseBytes == 0 {
		return
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	// 总量累计
	ts.TotalPrompt += int64(usage.PromptTokens)
	ts.TotalCompletion += int64(usage.CompletionTokens)
	ts.TotalTokens += int64(usage.TotalTokens)
	ts.TotalReqBytes += usage.RequestBytes
	ts.TotalRespBytes += usage.ResponseBytes
	ts.CallCount++
	ts.UpdatedAt = time.Now()

	// 当日累计（日期变化时重置）
	today := time.Now().Format("2006-01-02")
	if ts.TodayDate != today {
		ts.TodayDate = today
		ts.TodayTokens = 0
		ts.TodayCallCount = 0
		ts.TodayReqBytes = 0
		ts.TodayRespBytes = 0
		ts.Today5xxErrors = 0
		ts.TodayByModel = make(map[string]*ModelTokenStats)
	}
	ts.TodayTokens += int64(usage.TotalTokens)
	ts.TodayReqBytes += usage.RequestBytes
	ts.TodayRespBytes += usage.ResponseBytes
	ts.TodayCallCount++

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
		ms.ReqBytes += usage.RequestBytes
		ms.RespBytes += usage.ResponseBytes

		// 当日分模型
		dms, ok := ts.TodayByModel[usage.Model]
		if !ok {
			dms = &ModelTokenStats{}
			ts.TodayByModel[usage.Model] = dms
		}
		dms.Total += int64(usage.TotalTokens)
		dms.Calls++
		dms.ReqBytes += usage.RequestBytes
		dms.RespBytes += usage.ResponseBytes
	}

	log.Printf("[TokenStats] model=%s prompt=%d completion=%d total=%d req=%s resp=%s | 累计: prompt=%d completion=%d total=%d calls=%d req=%s resp=%s",
		usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens,
		formatBytes(usage.RequestBytes), formatBytes(usage.ResponseBytes),
		ts.TotalPrompt, ts.TotalCompletion, ts.TotalTokens, ts.CallCount,
		formatBytes(ts.TotalReqBytes), formatBytes(ts.TotalRespBytes))

	// 自动持久化
	ts.saveLocked()
}

// Add5xxError 累加一次 5xx 错误计数
func (ts *TokenStats) Add5xxError(model string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.Total5xxErrors++
	ts.UpdatedAt = time.Now()

	// 当日累计（日期变化时重置）
	today := time.Now().Format("2006-01-02")
	if ts.TodayDate != today {
		ts.TodayDate = today
		ts.Today5xxErrors = 0
		// 同时重置所有模型的当日 5xx 计数
		for _, ms := range ts.ByModel {
			ms.Today5xx = 0
		}
	}
	ts.Today5xxErrors++

	// 更新对应模型的 5xx 错误计数
	ms, ok := ts.ByModel[model]
	if !ok {
		ms = &ModelTokenStats{}
		ts.ByModel[model] = ms
	}
	ms.Errors5xx++
	ms.Today5xx++

	log.Printf("[TokenStats] 5xx error for model=%s | model_total=%d model_today=%d global_total=%d global_today=%d",
		model, ms.Errors5xx, ms.Today5xx, ts.Total5xxErrors, ts.Today5xxErrors)

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

	// 检查当日数据是否过期
	today := time.Now().Format("2006-01-02")
	todayTokens := ts.TodayTokens
	todayCallCount := ts.TodayCallCount
	todayReqBytes := ts.TodayReqBytes
	todayRespBytes := ts.TodayRespBytes
	today5xxErrors := ts.Today5xxErrors
	if ts.TodayDate != today {
		todayTokens = 0
		todayCallCount = 0
		todayReqBytes = 0
		todayRespBytes = 0
		today5xxErrors = 0
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 Token: %s/%s (%d/%d次)\n",
		formatTokenCount(todayTokens),
		formatTokenCount(ts.TotalTokens),
		todayCallCount,
		ts.CallCount))
	sb.WriteString(fmt.Sprintf("📡 流量: ↑%s ↓%s / ↑%s ↓%s",
		formatBytes(todayReqBytes),
		formatBytes(todayRespBytes),
		formatBytes(ts.TotalReqBytes),
		formatBytes(ts.TotalRespBytes)))

	// 5xx 错误统计（总计）
	if ts.Total5xxErrors > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️ 5xx错误: 今日 %d / 总计 %d", today5xxErrors, ts.Total5xxErrors))
	}

	// 分模型明细
	if len(ts.ByModel) > 1 {
		sb.WriteString("\n")
		for model, ms := range ts.ByModel {
			var dayTotal int64
			var dayCalls int64
			if ts.TodayDate == today {
				if dms, ok := ts.TodayByModel[model]; ok {
					dayTotal = dms.Total
					dayCalls = dms.Calls
				}
			}
			err5xxStr := ""
			if ms.Errors5xx > 0 {
				err5xxStr = fmt.Sprintf(" ⚠️5xx:%d/%d", ms.Today5xx, ms.Errors5xx)
			}
			sb.WriteString(fmt.Sprintf("\n· %s\n  %s/%s (%d/%d次)%s",
				model, formatTokenCount(dayTotal), formatTokenCount(ms.Total), dayCalls, ms.Calls, err5xxStr))
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
	ts.TotalReqBytes = 0
	ts.TotalRespBytes = 0
	ts.Total5xxErrors = 0
	ts.ByModel = make(map[string]*ModelTokenStats)
	ts.TodayDate = ""
	ts.TodayTokens = 0
	ts.TodayCallCount = 0
	ts.TodayReqBytes = 0
	ts.TodayRespBytes = 0
	ts.Today5xxErrors = 0
	ts.TodayByModel = make(map[string]*ModelTokenStats)
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
	ts.TotalReqBytes = loaded.TotalReqBytes
	ts.TotalRespBytes = loaded.TotalRespBytes
	ts.Total5xxErrors = loaded.Total5xxErrors
	ts.UpdatedAt = loaded.UpdatedAt
	if loaded.ByModel != nil {
		ts.ByModel = loaded.ByModel
	}
	ts.TodayDate = loaded.TodayDate
	ts.TodayTokens = loaded.TodayTokens
	ts.TodayCallCount = loaded.TodayCallCount
	ts.TodayReqBytes = loaded.TodayReqBytes
	ts.TodayRespBytes = loaded.TodayRespBytes
	ts.Today5xxErrors = loaded.Today5xxErrors
	if loaded.TodayByModel != nil {
		ts.TodayByModel = loaded.TodayByModel
	}

	log.Printf("[TokenStats] loaded: prompt=%d completion=%d total=%d calls=%d req=%s resp=%s 5xx=%d",
		ts.TotalPrompt, ts.TotalCompletion, ts.TotalTokens, ts.CallCount,
		formatBytes(ts.TotalReqBytes), formatBytes(ts.TotalRespBytes), ts.Total5xxErrors)
}

// formatTokenCount 格式化 token 数量（大数字用 M 显示，更易读）
// 超过 100 万（1,000,000）用 "X.XXM" 格式，100 万以下用千分位逗号
func formatTokenCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
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

// formatBytes 格式化字节数为人类可读格式（KB/MB/GB）
func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
