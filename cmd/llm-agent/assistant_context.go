package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const assistantRecordHeader = "【执行记录】"

type AssistantRecordInput struct {
	Query         string
	DisplayResult string
	Status        string
	RootSession   *TaskSession
	ChildSessions map[string]*TaskSession
	Results       []SubTaskResult
	FinalErr      error
}

func buildPersistentAssistantRecord(input AssistantRecordInput) string {
	if !shouldPersistStructuredAssistantRecord(input) {
		return strings.TrimSpace(input.DisplayResult)
	}

	query := strings.TrimSpace(input.Query)
	if query == "" && input.RootSession != nil {
		query = strings.TrimSpace(input.RootSession.Title)
	}

	doneItems, asyncItems, failedItems, deferredItems := summarizeSubTaskStates(input.Results)
	asyncSessions := collectAsyncSessionInfos(input.RootSession, input.ChildSessions, input.Results)
	keyFacts := collectKeyToolFacts(input.RootSession, input.ChildSessions)
	conclusion := deriveAssistantConclusion(input.DisplayResult, input.Status, input.Results, input.FinalErr)
	recovery := buildRecoverySuggestion(asyncSessions, keyFacts, failedItems)

	var sb strings.Builder
	sb.WriteString(assistantRecordHeader)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("状态: %s\n", assistantRecordStatus(input.Status, input.FinalErr)))
	sb.WriteString(fmt.Sprintf("用户请求: %s\n", fallbackText(query, "（未记录用户请求）")))
	sb.WriteString(fmt.Sprintf("最终结论: %s\n", fallbackText(conclusion, "（未生成最终结论）")))
	sb.WriteString(fmt.Sprintf("已完成: %s\n", joinSummaryItems(doneItems, "无")))
	sb.WriteString(fmt.Sprintf("进行中: %s\n", summarizeAsyncSessions(asyncItems, asyncSessions)))
	sb.WriteString(fmt.Sprintf("失败点: %s\n", joinSummaryItems(failedItems, "无")))
	if len(deferredItems) > 0 {
		sb.WriteString(fmt.Sprintf("待继续: %s\n", joinSummaryItems(deferredItems, "无")))
	}
	sb.WriteString(fmt.Sprintf("恢复建议: %s\n", recovery))

	if len(input.Results) > 0 {
		sb.WriteString("\n子任务记录:\n")
		for idx, result := range input.Results {
			sb.WriteString(fmt.Sprintf("%d. %s\n", idx+1, fallbackText(strings.TrimSpace(result.Title), result.SubTaskID)))
			sb.WriteString(fmt.Sprintf("状态: %s\n", fallbackText(strings.TrimSpace(result.Status), "unknown")))
			if desc := subTaskDescription(input.RootSession, input.ChildSessions, result.SubTaskID); desc != "" {
				sb.WriteString(fmt.Sprintf("描述: %s\n", truncate(desc, 240)))
			}

			detail := buildSubTaskDetail(result)
			if detail != "" {
				sb.WriteString(fmt.Sprintf("关键结果: %s\n", detail))
			}

			asyncLine := formatAsyncSessionLine(result.AsyncSessions)
			if asyncLine != "" {
				sb.WriteString(fmt.Sprintf("异步会话: %s\n", asyncLine))
			}

			if idx < len(input.Results)-1 {
				sb.WriteString("--\n")
			}
		}
	}

	if len(keyFacts) > 0 {
		sb.WriteString("\n关键工具返回:\n")
		for _, fact := range keyFacts {
			sb.WriteString("- ")
			sb.WriteString(fact)
			sb.WriteString("\n")
		}
	}

	record := strings.TrimSpace(sb.String())
	if len([]rune(record)) > 6000 {
		record = truncate(record, 6000) + "\n...[执行记录已截断]"
	}
	return record
}

func shouldPersistStructuredAssistantRecord(input AssistantRecordInput) bool {
	if input.FinalErr != nil {
		return true
	}
	status := strings.TrimSpace(input.Status)
	if status == "async" || status == "failed" || status == "deferred" {
		return true
	}
	if len(input.Results) > 0 {
		return true
	}
	if input.RootSession != nil {
		input.RootSession.mu.Lock()
		toolCallCount := len(input.RootSession.ToolCalls)
		input.RootSession.mu.Unlock()
		if toolCallCount > 0 {
			return true
		}
	}
	return false
}

func assistantRecordStatus(status string, finalErr error) string {
	if finalErr != nil {
		return "failed"
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return "done"
	}
	return status
}

func deriveAssistantConclusion(displayResult, status string, results []SubTaskResult, finalErr error) string {
	if finalErr != nil {
		return truncate(finalErr.Error(), 400)
	}

	cleaned := normalizeAssistantDisplayResult(displayResult)
	if cleaned != "" {
		return truncate(cleaned, 500)
	}

	switch strings.TrimSpace(status) {
	case "async":
		asyncInfos := collectAsyncSessionInfos(nil, nil, results)
		if len(asyncInfos) > 0 {
			return fmt.Sprintf("已发起 %d 个后台任务，后续需要继续查询执行状态。", len(asyncInfos))
		}
		return "任务已进入后台执行，后续需要继续查询执行状态。"
	case "failed":
		for _, result := range results {
			if strings.TrimSpace(result.Status) == "failed" {
				if errText := strings.TrimSpace(result.Error); errText != "" {
					return truncate(errText, 400)
				}
			}
		}
		return "任务执行失败，需要根据失败子任务继续处理。"
	default:
		for _, result := range results {
			if strings.TrimSpace(result.Status) == "done" && strings.TrimSpace(result.Result) != "" {
				return truncate(result.Result, 500)
			}
		}
	}
	return "任务已处理完成。"
}

func normalizeAssistantDisplayResult(display string) string {
	display = strings.TrimSpace(display)
	if display == "" {
		return ""
	}

	boilerplates := []string{
		"任务已派发",
		"进度将通过微信推送",
		"后台执行中",
	}
	for _, marker := range boilerplates {
		if strings.Contains(display, marker) {
			return ""
		}
	}
	return display
}

func summarizeSubTaskStates(results []SubTaskResult) (doneItems, asyncItems, failedItems, deferredItems []string) {
	for _, result := range results {
		title := fallbackText(strings.TrimSpace(result.Title), result.SubTaskID)
		switch strings.TrimSpace(result.Status) {
		case "done":
			doneItems = append(doneItems, title)
		case "async":
			asyncItems = append(asyncItems, title)
		case "failed":
			failedItems = append(failedItems, title)
		case "deferred":
			deferredItems = append(deferredItems, title)
		}
	}
	return
}

func joinSummaryItems(items []string, empty string) string {
	if len(items) == 0 {
		return empty
	}
	if len(items) > 4 {
		items = append(items[:4], fmt.Sprintf("等%d项", len(items)))
	}
	return strings.Join(items, "；")
}

func summarizeAsyncSessions(asyncItems []string, sessions []AsyncSessionInfo) string {
	if len(sessions) == 0 {
		return joinSummaryItems(asyncItems, "无")
	}

	lines := make([]string, 0, len(sessions))
	for _, info := range sessions {
		line := info.ToolName
		if line == "" {
			line = "异步任务"
		}
		line += fmt.Sprintf("[session_id=%s]", info.SessionID)
		lines = append(lines, line)
	}
	return joinSummaryItems(lines, "无")
}

func buildRecoverySuggestion(asyncSessions []AsyncSessionInfo, keyFacts, failedItems []string) string {
	if len(asyncSessions) > 0 {
		var sessionIDs []string
		for _, info := range asyncSessions {
			sessionIDs = append(sessionIDs, info.SessionID)
		}
		return fmt.Sprintf("优先使用 DeployGetStatus 查询 session_id=%s；继续执行时复用已有 session_id、project_dir 和 deploy_target，不要重复创建任务。",
			strings.Join(sessionIDs, ", "))
	}
	if len(failedItems) > 0 {
		return fmt.Sprintf("从失败子任务继续处理：%s；先复用上面的关键工具返回，再决定是否重试。", joinSummaryItems(failedItems, "无"))
	}
	if len(keyFacts) > 0 {
		return "继续执行时直接引用上面的关键工具返回，不要重新猜测 session_id、project_dir、deploy_target 或端口。"
	}
	return "继续执行时先阅读本条执行记录中的已完成步骤和失败点，再决定下一步操作。"
}

func buildSubTaskDetail(result SubTaskResult) string {
	switch strings.TrimSpace(result.Status) {
	case "failed":
		if errText := strings.TrimSpace(result.Error); errText != "" {
			return truncate(errText, 240)
		}
	case "async":
		if msg := firstAsyncMessage(result.AsyncSessions); msg != "" {
			return truncate(msg, 240)
		}
	case "deferred":
		if errText := strings.TrimSpace(result.Error); errText != "" {
			return truncate(errText, 240)
		}
	}

	if text := strings.TrimSpace(result.Result); text != "" {
		return truncate(text, 240)
	}
	if errText := strings.TrimSpace(result.Error); errText != "" {
		return truncate(errText, 240)
	}
	return ""
}

func firstAsyncMessage(infos []AsyncSessionInfo) string {
	for _, info := range infos {
		if msg := strings.TrimSpace(info.Message); msg != "" {
			return msg
		}
	}
	return ""
}

func formatAsyncSessionLine(infos []AsyncSessionInfo) string {
	if len(infos) == 0 {
		return ""
	}

	parts := make([]string, 0, len(infos))
	for _, info := range dedupeAsyncSessions(infos) {
		part := fallbackText(strings.TrimSpace(info.ToolName), "异步任务")
		part += fmt.Sprintf("[session_id=%s]", info.SessionID)
		if msg := strings.TrimSpace(info.Message); msg != "" {
			part += ": " + truncate(msg, 120)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "；")
}

func collectAsyncSessionInfos(root *TaskSession, children map[string]*TaskSession, results []SubTaskResult) []AsyncSessionInfo {
	var all []AsyncSessionInfo
	for _, result := range results {
		all = append(all, result.AsyncSessions...)
	}
	if root != nil {
		all = append(all, detectAsyncResults(root)...)
	}
	for _, child := range children {
		all = append(all, detectAsyncResults(child)...)
	}
	return dedupeAsyncSessions(all)
}

func dedupeAsyncSessions(infos []AsyncSessionInfo) []AsyncSessionInfo {
	seen := make(map[string]struct{})
	out := make([]AsyncSessionInfo, 0, len(infos))
	for _, info := range infos {
		sessionID := strings.TrimSpace(info.SessionID)
		if sessionID == "" {
			continue
		}
		toolName := strings.TrimSpace(info.ToolName)
		key := toolName + "|" + sessionID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		info.ToolName = toolName
		info.SessionID = sessionID
		info.Message = strings.TrimSpace(info.Message)
		out = append(out, info)
	}
	return out
}

func collectKeyToolFacts(root *TaskSession, children map[string]*TaskSession) []string {
	var facts []string
	seen := make(map[string]struct{})

	appendFacts := func(prefix string, session *TaskSession) {
		if session == nil {
			return
		}
		session.mu.Lock()
		records := make([]ToolCallRecord, len(session.ToolCalls))
		copy(records, session.ToolCalls)
		session.mu.Unlock()

		for _, record := range records {
			fact := summarizeToolCallRecord(record)
			if fact == "" {
				continue
			}
			if prefix != "" {
				fact = prefix + fact
			}
			if _, ok := seen[fact]; ok {
				continue
			}
			seen[fact] = struct{}{}
			facts = append(facts, fact)
			if len(facts) >= 8 {
				return
			}
		}
	}

	appendFacts("", root)
	if len(facts) >= 8 {
		return facts
	}

	var childIDs []string
	for childID := range children {
		childIDs = append(childIDs, childID)
	}
	sort.Strings(childIDs)
	for _, childID := range childIDs {
		appendFacts(childID+": ", children[childID])
		if len(facts) >= 8 {
			return facts
		}
	}
	return facts
}

func summarizeToolCallRecord(record ToolCallRecord) string {
	fields := collectInterestingFields(record.Result, record.Arguments)
	if len(fields) > 0 {
		return fmt.Sprintf("%s: %s", record.ToolName, strings.Join(fields, ", "))
	}

	if !record.Success {
		errText := extractResultText(record.Result, "error", "message")
		if errText == "" {
			errText = strings.TrimSpace(record.Result)
		}
		errText = truncate(errText, 160)
		if errText == "" {
			return ""
		}
		return fmt.Sprintf("%s: error=%s", record.ToolName, errText)
	}

	msg := extractResultText(record.Result, "message", "summary", "result")
	if msg == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", record.ToolName, truncate(msg, 160))
}

func collectInterestingFields(result, args string) []string {
	var fields []string
	seen := make(map[string]struct{})
	appendFields := func(raw string, keys []string) {
		parsed, ok := decodeJSONMap(raw)
		if !ok {
			return
		}
		candidates := []map[string]any{parsed}
		if data, ok := parsed["data"].(map[string]any); ok {
			candidates = append(candidates, data)
		}
		for _, key := range keys {
			for _, candidate := range candidates {
				value, ok := candidate[key]
				if !ok || value == nil {
					continue
				}
				text := fmt.Sprintf("%s=%s", key, truncate(formatJSONValue(value), 120))
				if _, exists := seen[text]; exists {
					continue
				}
				seen[text] = struct{}{}
				fields = append(fields, text)
			}
		}
	}

	appendFields(result, []string{"session_id", "status", "project", "deploy_target", "project_dir", "artifact_file", "artifact_path", "url", "port"})
	appendFields(args, []string{"project", "deploy_target", "project_dir", "url", "port"})
	return fields
}

func decodeJSONMap(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || (!strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[")) {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func extractResultText(raw string, keys ...string) string {
	parsed, ok := decodeJSONMap(raw)
	if !ok {
		return ""
	}

	candidates := []map[string]any{parsed}
	if data, ok := parsed["data"].(map[string]any); ok {
		candidates = append(candidates, data)
	}

	for _, key := range keys {
		for _, candidate := range candidates {
			value, ok := candidate[key]
			if !ok || value == nil {
				continue
			}
			text := strings.TrimSpace(formatJSONValue(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func formatJSONValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func subTaskDescription(root *TaskSession, children map[string]*TaskSession, subTaskID string) string {
	if child, ok := children[subTaskID]; ok {
		child.mu.Lock()
		desc := child.Description
		child.mu.Unlock()
		if strings.TrimSpace(desc) != "" {
			return desc
		}
	}
	return ""
}

func fallbackText(text, fallback string) string {
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func appendFinalAssistantRecord(session *TaskSession, content string) {
	content = strings.TrimSpace(content)
	if session == nil || content == "" {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if n := len(session.Messages); n > 0 {
		last := session.Messages[n-1]
		if last.Role == "assistant" && strings.TrimSpace(last.Content) == content {
			return
		}
	}
	session.Messages = append(session.Messages, Message{Role: "assistant", Content: content})
}

func persistedAssistantContent(ctx *TaskContext, fallback string) string {
	if ctx != nil {
		content := strings.TrimSpace(ctx.PersistedAssistant)
		if strings.HasPrefix(content, assistantRecordHeader) {
			return content
		}
	}
	return fallback
}

func parseAssistantRecordSummary(content string) (status, inProgress, failed string, ok bool) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, assistantRecordHeader) {
		return "", "", "", false
	}

	lines := strings.Split(content, "\n")
	findLine := func(prefix string) string {
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
		return ""
	}

	return findLine("状态: "), findLine("进行中: "), findLine("失败点: "), true
}
