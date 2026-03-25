package main

import (
	"fmt"
	"strings"
	"time"
)

// ========================= RequestTrace 请求级追踪 =========================

// RequestTrace 请求级追踪：记录 LLM 轮次、工具调用路径，方便定位问题
type RequestTrace struct {
	TaskID    string
	Source    string
	Query     string
	StartTime time.Time
	Rounds    []TraceRound // 每轮 LLM 调用
}

// TraceRound 单轮 LLM 调用记录
type TraceRound struct {
	Index         int             // 第几轮（从1开始）
	LLMDurationMs int64          // LLM 响应耗时（毫秒）
	TextLen       int             // LLM 返回文本长度
	ToolCalls     []TraceToolCall // 本轮工具调用
}

// TraceToolCall 单次工具调用记录
type TraceToolCall struct {
	ToolName   string
	Arguments  string // 截断后的参数摘要
	Success    bool
	DurationMs int64
	ResultLen  int
}

// Summary 输出结构化追踪摘要
func (t *RequestTrace) Summary() string {
	if t == nil || len(t.Rounds) == 0 {
		return ""
	}
	totalDuration := time.Since(t.StartTime)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Trace] taskID=%s source=%s 共%d轮 总耗时%s query=%s\n",
		t.TaskID, t.Source, len(t.Rounds), fmtDuration(totalDuration), truncate(t.Query, 80)))

	for _, r := range t.Rounds {
		llmDur := fmtDuration(time.Duration(r.LLMDurationMs) * time.Millisecond)
		if len(r.ToolCalls) == 0 {
			sb.WriteString(fmt.Sprintf("  Round[%d] LLM=%s textLen=%d → 无工具调用（最终回复）\n", r.Index, llmDur, r.TextLen))
		} else {
			var tcParts []string
			for _, tc := range r.ToolCalls {
				status := "✅"
				if !tc.Success {
					status = "❌"
				}
				tcDur := fmtDuration(time.Duration(tc.DurationMs) * time.Millisecond)
				part := fmt.Sprintf("%s(%s %s", tc.ToolName, status, tcDur)
				if tc.Arguments != "" {
					part += " " + tc.Arguments
				}
				part += ")"
				tcParts = append(tcParts, part)
			}
			sb.WriteString(fmt.Sprintf("  Round[%d] LLM=%s → %d个工具: %s\n",
				r.Index, llmDur, len(r.ToolCalls), strings.Join(tcParts, ", ")))
		}
	}
	return sb.String()
}
