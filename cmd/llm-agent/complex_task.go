package main

import (
	"fmt"
	"strings"
)

// buildAsyncAcknowledgment 构建异步任务即时确认消息
func buildAsyncAcknowledgment(results []SubTaskResult) string {
	var sb strings.Builder
	sb.WriteString("📋 任务已派发，进度将通过微信推送\n\n")

	for _, r := range results {
		switch r.Status {
		case "done":
			sb.WriteString(fmt.Sprintf("✅ %s\n", r.Title))
		case "failed":
			sb.WriteString(fmt.Sprintf("❌ %s: %s\n", r.Title, r.Error))
		case "skipped":
			sb.WriteString(fmt.Sprintf("⏭ %s\n", r.Title))
		case "async":
			var sids []string
			for _, a := range dedupeAsyncSessions(r.AsyncSessions) {
				sids = append(sids, a.SessionID)
			}
			sb.WriteString(fmt.Sprintf("⏳ %s (后台执行中: %s)\n", r.Title, strings.Join(sids, ", ")))
		case "deferred":
			sb.WriteString(fmt.Sprintf("⏸ %s (等待前置任务完成)\n", r.Title))
		}
	}
	return sb.String()
}
