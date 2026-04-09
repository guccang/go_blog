package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const requestTraceVersion = 2

// RequestTrace 请求级追踪：记录任务描述、执行路径、LLM 轮次和工具调用明细。
type RequestTrace struct {
	Version         int                 `json:"version"`
	TaskID          string              `json:"task_id"`
	RootID          string              `json:"root_id"`
	SessionID       string              `json:"session_id"`
	ParentSessionID string              `json:"parent_session_id,omitempty"`
	Source          string              `json:"source"`
	Scope           string              `json:"scope"`
	Title           string              `json:"title,omitempty"`
	Description     string              `json:"description,omitempty"`
	Query           string              `json:"query"`
	Status          string              `json:"status"`
	StartTime       time.Time           `json:"start_time"`
	UpdatedAt       time.Time           `json:"updated_at"`
	FinishedAt      *time.Time          `json:"finished_at,omitempty"`
	ToolView        TraceToolView       `json:"tool_view,omitempty"`
	ExecutionPath   []TracePathStep     `json:"execution_path,omitempty"`
	Events          []TraceEvent        `json:"events,omitempty"`
	Rounds          []TraceRound        `json:"rounds,omitempty"`
	ChildSessions   []TraceChildSession `json:"child_sessions,omitempty"`
	ResultPreview   string              `json:"result_preview,omitempty"`
	Error           string              `json:"error,omitempty"`
}

type TraceToolView struct {
	Policy          string            `json:"policy,omitempty"`
	MatchedSkills   []string          `json:"matched_skills,omitempty"`
	Hints           []string          `json:"hints,omitempty"`
	AllTools        []string          `json:"all_tools,omitempty"`
	VisibleTools    []string          `json:"visible_tools,omitempty"`
	DiscoveredTools []string          `json:"discovered_tools,omitempty"`
	SourceReasons   map[string]string `json:"source_reasons,omitempty"`
}

type TracePathStep struct {
	Name   string            `json:"name"`
	Detail string            `json:"detail,omitempty"`
	Meta   map[string]string `json:"meta,omitempty"`
	At     time.Time         `json:"at"`
}

type TraceEvent struct {
	Kind      string            `json:"kind"`
	Title     string            `json:"title,omitempty"`
	Detail    string            `json:"detail,omitempty"`
	Iteration int               `json:"iteration,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
	At        time.Time         `json:"at"`
}

type TraceChildSession struct {
	SessionID       string     `json:"session_id"`
	ParentSessionID string     `json:"parent_session_id,omitempty"`
	Scope           string     `json:"scope,omitempty"`
	Title           string     `json:"title,omitempty"`
	Status          string     `json:"status,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

// TraceRound 单轮 LLM 调用记录。
type TraceRound struct {
	Index            int                     `json:"index"`
	LLMDurationMs    int64                   `json:"llm_duration_ms"`
	TextLen          int                     `json:"text_len"`
	AssistantPreview string                  `json:"assistant_preview,omitempty"`
	ToolCallNames    []string                `json:"tool_call_names,omitempty"`
	VisibleTools     []string                `json:"visible_tools,omitempty"`
	MailboxInjected  int                     `json:"mailbox_injected,omitempty"`
	Compaction       *RuntimeCompactMetadata `json:"compaction,omitempty"`
	ToolCalls        []TraceToolCall         `json:"tool_calls,omitempty"`
}

// TraceToolCall 单次工具调用记录。
type TraceToolCall struct {
	ToolName      string `json:"tool_name"`
	ToolCallID    string `json:"tool_call_id,omitempty"`
	ScopeID       string `json:"scope_id,omitempty"`
	Arguments     string `json:"arguments,omitempty"`
	Success       bool   `json:"success"`
	DurationMs    int64  `json:"duration_ms"`
	ResultLen     int    `json:"result_len"`
	ResultPreview string `json:"result_preview,omitempty"`
	BusinessErr   string `json:"business_error,omitempty"`
	ToAgent       string `json:"to_agent,omitempty"`
	FromAgent     string `json:"from_agent,omitempty"`
}

func NewRequestTrace(taskID, source, scope, query string, session *TaskSession) *RequestTrace {
	now := time.Now()
	trace := &RequestTrace{
		Version:   requestTraceVersion,
		TaskID:    strings.TrimSpace(taskID),
		Source:    strings.TrimSpace(source),
		Scope:     strings.TrimSpace(scope),
		Query:     strings.TrimSpace(query),
		Status:    "running",
		StartTime: now,
		UpdatedAt: now,
	}
	trace.RefreshFromSession(session)
	return trace
}

func (t *RequestTrace) RefreshFromSession(session *TaskSession) {
	if t == nil || session == nil {
		return
	}
	t.RootID = strings.TrimSpace(session.RootID)
	t.SessionID = strings.TrimSpace(session.ID)
	t.ParentSessionID = strings.TrimSpace(session.ParentID)
	if strings.TrimSpace(session.Title) != "" {
		t.Title = strings.TrimSpace(session.Title)
	}
	if strings.TrimSpace(session.Description) != "" {
		t.Description = strings.TrimSpace(session.Description)
	}
	if strings.TrimSpace(session.Status) != "" {
		t.Status = strings.TrimSpace(session.Status)
	}
	if session.StartedAt != nil && !session.StartedAt.IsZero() {
		t.StartTime = *session.StartedAt
	}
	if session.FinishedAt != nil {
		finished := *session.FinishedAt
		t.FinishedAt = &finished
	}
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) SetDescription(description string) {
	if t == nil || strings.TrimSpace(description) == "" {
		return
	}
	t.Description = strings.TrimSpace(description)
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) SetToolView(view *ToolRuntimeView) {
	if t == nil || view == nil {
		return
	}
	t.ToolView = TraceToolView{
		Policy:          strings.TrimSpace(view.Policy),
		MatchedSkills:   cloneStringSlice(view.MatchedSkills),
		Hints:           cloneStringSlice(view.Hints),
		AllTools:        traceToolNames(view.AllTools),
		VisibleTools:    traceToolNames(view.VisibleTools),
		DiscoveredTools: traceDiscoveredToolNames(view.DiscoveredTools),
		SourceReasons:   cloneStringMap(view.SourceReasons),
	}
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) RecordPath(name, detail string, meta map[string]string) {
	if t == nil {
		return
	}
	t.ExecutionPath = append(t.ExecutionPath, TracePathStep{
		Name:   strings.TrimSpace(name),
		Detail: truncate(strings.TrimSpace(detail), 400),
		Meta:   cloneStringMap(meta),
		At:     time.Now(),
	})
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) RecordEvent(kind, title, detail string, iteration int, meta map[string]string) {
	if t == nil {
		return
	}
	t.Events = append(t.Events, TraceEvent{
		Kind:      strings.TrimSpace(kind),
		Title:     strings.TrimSpace(title),
		Detail:    truncate(strings.TrimSpace(detail), 1200),
		Iteration: iteration,
		Meta:      cloneStringMap(meta),
		At:        time.Now(),
	})
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) EnsureRound(index int) *TraceRound {
	if t == nil {
		return nil
	}
	for i := range t.Rounds {
		if t.Rounds[i].Index == index {
			return &t.Rounds[i]
		}
	}
	t.Rounds = append(t.Rounds, TraceRound{Index: index})
	sort.Slice(t.Rounds, func(i, j int) bool {
		return t.Rounds[i].Index < t.Rounds[j].Index
	})
	for i := range t.Rounds {
		if t.Rounds[i].Index == index {
			return &t.Rounds[i]
		}
	}
	return nil
}

func (t *RequestTrace) RecordRoundLLM(index int, duration time.Duration, text string, toolCalls []ToolCall, visibleTools []LLMTool) {
	round := t.EnsureRound(index)
	if round == nil {
		return
	}
	round.LLMDurationMs = duration.Milliseconds()
	round.TextLen = len(text)
	round.AssistantPreview = truncate(strings.TrimSpace(text), 300)
	round.ToolCallNames = traceToolCallNames(toolCalls)
	round.VisibleTools = traceToolNames(visibleTools)
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) RecordRoundMailbox(index, injected int, attachments []Attachment) {
	round := t.EnsureRound(index)
	if round == nil || injected <= 0 {
		return
	}
	round.MailboxInjected += injected
	var titles []string
	for _, att := range attachments {
		label := strings.TrimSpace(att.Title)
		if label == "" {
			label = string(att.Kind)
		}
		titles = append(titles, label)
	}
	t.RecordEvent("mailbox_injected", "运行时上下文注入",
		fmt.Sprintf("round=%d 注入 %d 条 attachment: %s", index, injected, strings.Join(titles, ", ")),
		index, map[string]string{"attachment_count": fmt.Sprintf("%d", injected)})
}

func (t *RequestTrace) RecordRoundCompaction(index int, meta *RuntimeCompactMetadata) {
	if t == nil || meta == nil {
		return
	}
	round := t.EnsureRound(index)
	if round == nil {
		return
	}
	compaction := *meta
	round.Compaction = &compaction
	t.RecordEvent("context_compaction", "上下文压缩",
		fmt.Sprintf("round=%d messages %d→%d chars %d→%d",
			index, meta.BeforeMessages, meta.AfterMessages, meta.BeforeChars, meta.AfterChars),
		index,
		map[string]string{
			"reason":            meta.Reason,
			"tool_results_trim": fmt.Sprintf("%d", meta.ToolResultsTrimed),
		},
	)
}

func (t *RequestTrace) RecordChildSession(session *TaskSession, scope, title, status string) {
	if t == nil || session == nil {
		return
	}
	for i := range t.ChildSessions {
		if t.ChildSessions[i].SessionID == session.ID {
			t.ChildSessions[i].Status = strings.TrimSpace(status)
			t.ChildSessions[i].Title = fallbackText(strings.TrimSpace(title), t.ChildSessions[i].Title)
			t.ChildSessions[i].Scope = fallbackText(strings.TrimSpace(scope), t.ChildSessions[i].Scope)
			t.ChildSessions[i].FinishedAt = session.FinishedAt
			t.UpdatedAt = time.Now()
			return
		}
	}
	child := TraceChildSession{
		SessionID:       session.ID,
		ParentSessionID: session.ParentID,
		Scope:           strings.TrimSpace(scope),
		Title:           fallbackText(strings.TrimSpace(title), session.Title),
		Status:          fallbackText(strings.TrimSpace(status), session.Status),
		StartedAt:       session.StartedAt,
		FinishedAt:      session.FinishedAt,
	}
	t.ChildSessions = append(t.ChildSessions, child)
	t.UpdatedAt = time.Now()
}

func (t *RequestTrace) Finish(status, result string, err error) {
	if t == nil {
		return
	}
	t.Status = strings.TrimSpace(status)
	t.ResultPreview = truncate(strings.TrimSpace(result), 1200)
	if err != nil {
		t.Error = err.Error()
	}
	now := time.Now()
	t.FinishedAt = &now
	t.UpdatedAt = now
}

// Summary 输出结构化追踪摘要。
func (t *RequestTrace) Summary() string {
	if t == nil || len(t.Rounds) == 0 {
		return ""
	}
	totalDuration := time.Since(t.StartTime)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Trace] taskID=%s session=%s scope=%s source=%s 共%d轮 总耗时%s query=%s\n",
		t.TaskID, fallbackText(t.SessionID, t.RootID), fallbackText(t.Scope, "unknown"), t.Source, len(t.Rounds), fmtDuration(totalDuration), truncate(t.Query, 80)))

	for _, r := range t.Rounds {
		llmDur := fmtDuration(time.Duration(r.LLMDurationMs) * time.Millisecond)
		if len(r.ToolCalls) == 0 {
			sb.WriteString(fmt.Sprintf("  Round[%d] LLM=%s textLen=%d → 无工具调用（最终回复）\n", r.Index, llmDur, r.TextLen))
			continue
		}
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
			if tc.BusinessErr != "" {
				part += " biz=" + truncate(tc.BusinessErr, 60)
			}
			part += ")"
			tcParts = append(tcParts, part)
		}
		sb.WriteString(fmt.Sprintf("  Round[%d] LLM=%s → %d个工具: %s\n",
			r.Index, llmDur, len(r.ToolCalls), strings.Join(tcParts, ", ")))
	}
	return sb.String()
}

func (t *RequestTrace) Markdown() string {
	if t == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# 任务执行轨迹\n\n")
	sb.WriteString("## 基本信息\n")
	sb.WriteString(traceMetadataTable(t))
	sb.WriteString("\n")

	if chart := t.buildMermaidFlowchart(); chart != "" {
		sb.WriteString("## 执行流程图\n")
		sb.WriteString("```mermaid\n")
		sb.WriteString(chart)
		sb.WriteString("\n```\n\n")
	}

	if toolView := t.traceToolViewMarkdown(); toolView != "" {
		sb.WriteString("## 工具视图\n")
		sb.WriteString(toolView)
		sb.WriteString("\n")
	}

	if len(t.ExecutionPath) > 0 {
		sb.WriteString("## 执行路径\n")
		for idx, step := range t.ExecutionPath {
			sb.WriteString(fmt.Sprintf("%d. `%s`", idx+1, fallbackText(step.Name, "unknown")))
			if step.Detail != "" {
				sb.WriteString(": ")
				sb.WriteString(step.Detail)
			}
			if len(step.Meta) > 0 {
				sb.WriteString(" | ")
				sb.WriteString(traceMetaLine(step.Meta))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(t.Rounds) > 0 {
		sb.WriteString("## 轮次明细\n")
		for _, round := range t.Rounds {
			sb.WriteString(fmt.Sprintf("### Round %d\n", round.Index))
			sb.WriteString(fmt.Sprintf("- LLM耗时: %s\n", fmtDuration(time.Duration(round.LLMDurationMs)*time.Millisecond)))
			sb.WriteString(fmt.Sprintf("- 文本长度: %d\n", round.TextLen))
			if round.AssistantPreview != "" {
				sb.WriteString(fmt.Sprintf("- Assistant摘要: %s\n", round.AssistantPreview))
			}
			if len(round.VisibleTools) > 0 {
				sb.WriteString(fmt.Sprintf("- 可见工具: %s\n", strings.Join(round.VisibleTools, ", ")))
			}
			if round.MailboxInjected > 0 {
				sb.WriteString(fmt.Sprintf("- 注入上下文: %d\n", round.MailboxInjected))
			}
			if round.Compaction != nil {
				sb.WriteString(fmt.Sprintf("- 上下文压缩: %s, messages %d→%d, chars %d→%d\n",
					round.Compaction.Reason,
					round.Compaction.BeforeMessages, round.Compaction.AfterMessages,
					round.Compaction.BeforeChars, round.Compaction.AfterChars))
			}
			if len(round.ToolCalls) > 0 {
				sb.WriteString("- 工具调用:\n")
				for _, tc := range round.ToolCalls {
					sb.WriteString(fmt.Sprintf("  - %s | success=%t | duration=%s | result_len=%d\n",
						tc.ToolName, tc.Success, fmtDuration(time.Duration(tc.DurationMs)*time.Millisecond), tc.ResultLen))
					if tc.Arguments != "" {
						sb.WriteString(fmt.Sprintf("    args: %s\n", tc.Arguments))
					}
					if tc.BusinessErr != "" {
						sb.WriteString(fmt.Sprintf("    biz_error: %s\n", tc.BusinessErr))
					}
					if tc.ResultPreview != "" {
						sb.WriteString(fmt.Sprintf("    result: %s\n", tc.ResultPreview))
					}
				}
			}
			sb.WriteString("\n")
		}
	}

	if len(t.ChildSessions) > 0 {
		sb.WriteString("## 子任务\n")
		for _, child := range t.ChildSessions {
			sb.WriteString(fmt.Sprintf("- %s | %s | status=%s | title=%s\n",
				child.SessionID, fallbackText(child.Scope, "subtask"), fallbackText(child.Status, "unknown"), truncate(child.Title, 120)))
		}
		sb.WriteString("\n")
	}

	if len(t.Events) > 0 {
		sb.WriteString("## 事件时间线\n")
		sb.WriteString("| 时间 | 类型 | 标题 | 明细 |\n")
		sb.WriteString("| --- | --- | --- | --- |\n")
		for _, event := range t.Events {
			detail := escapeMarkdownTable(event.Detail)
			if len(event.Meta) > 0 {
				metaLine := traceMetaLine(event.Meta)
				if detail == "" {
					detail = escapeMarkdownTable(metaLine)
				} else {
					detail = escapeMarkdownTable(detail + " | " + metaLine)
				}
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				event.At.Format("15:04:05.000"),
				escapeMarkdownTable(fallbackText(event.Kind, "-")),
				escapeMarkdownTable(fallbackText(event.Title, "-")),
				fallbackText(detail, "-")))
		}
		sb.WriteString("\n")
	}

	if t.ResultPreview != "" {
		sb.WriteString("## 最终结果摘要\n")
		sb.WriteString(t.ResultPreview)
		sb.WriteString("\n")
	}
	if t.Error != "" {
		sb.WriteString("\n## 错误\n")
		sb.WriteString(t.Error)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (t *RequestTrace) buildMermaidFlowchart() string {
	if t == nil || len(t.ExecutionPath) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")
	for idx, step := range t.ExecutionPath {
		label := fmt.Sprintf("%d. %s", idx+1, fallbackText(step.Name, "unknown"))
		if step.Detail != "" {
			label += "<br/>" + truncate(step.Detail, 160)
		}
		if len(step.Meta) > 0 {
			label += "<br/>" + truncate(traceMetaLine(step.Meta), 160)
		}
		sb.WriteString(fmt.Sprintf("  n%d[\"%s\"]\n", idx, escapeMermaidLabel(label)))
		if idx > 0 {
			sb.WriteString(fmt.Sprintf("  n%d --> n%d\n", idx-1, idx))
		}
	}
	return sb.String()
}

func (t *RequestTrace) traceToolViewMarkdown() string {
	if t == nil {
		return ""
	}
	var lines []string
	if t.ToolView.Policy != "" {
		lines = append(lines, fmt.Sprintf("- policy: %s", t.ToolView.Policy))
	}
	if len(t.ToolView.MatchedSkills) > 0 {
		lines = append(lines, fmt.Sprintf("- matched_skills: %s", strings.Join(t.ToolView.MatchedSkills, ", ")))
	}
	if len(t.ToolView.Hints) > 0 {
		lines = append(lines, fmt.Sprintf("- hints: %s", strings.Join(t.ToolView.Hints, ", ")))
	}
	if len(t.ToolView.AllTools) > 0 {
		lines = append(lines, fmt.Sprintf("- all_tools(%d): %s", len(t.ToolView.AllTools), strings.Join(t.ToolView.AllTools, ", ")))
	}
	if len(t.ToolView.VisibleTools) > 0 {
		lines = append(lines, fmt.Sprintf("- visible_tools(%d): %s", len(t.ToolView.VisibleTools), strings.Join(t.ToolView.VisibleTools, ", ")))
	}
	if len(t.ToolView.DiscoveredTools) > 0 {
		lines = append(lines, fmt.Sprintf("- discovered_tools(%d): %s", len(t.ToolView.DiscoveredTools), strings.Join(t.ToolView.DiscoveredTools, ", ")))
	}
	if len(t.ToolView.SourceReasons) > 0 {
		lines = append(lines, fmt.Sprintf("- source_reasons: %s", traceMetaLine(t.ToolView.SourceReasons)))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func traceMetadataTable(t *RequestTrace) string {
	rows := [][2]string{
		{"task_id", t.TaskID},
		{"root_id", t.RootID},
		{"session_id", t.SessionID},
		{"parent_session_id", t.ParentSessionID},
		{"scope", t.Scope},
		{"source", t.Source},
		{"status", t.Status},
		{"title", t.Title},
		{"description", t.Description},
		{"query", t.Query},
		{"started_at", formatTraceTime(t.StartTime)},
		{"updated_at", formatTraceTime(t.UpdatedAt)},
	}
	if t.FinishedAt != nil {
		rows = append(rows, [2]string{"finished_at", formatTraceTime(*t.FinishedAt)})
	}

	var sb strings.Builder
	sb.WriteString("| 字段 | 值 |\n")
	sb.WriteString("| --- | --- |\n")
	for _, row := range rows {
		if strings.TrimSpace(row[1]) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", row[0], escapeMarkdownTable(row[1])))
	}
	return sb.String()
}

func traceToolNames(tools []LLMTool) []string {
	if len(tools) == 0 {
		return nil
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Function.Name) == "" {
			continue
		}
		names = append(names, strings.TrimSpace(tool.Function.Name))
	}
	return names
}

func traceDiscoveredToolNames(tools map[string]LLMTool) []string {
	if len(tools) == 0 {
		return nil
	}
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, strings.TrimSpace(name))
	}
	sort.Strings(names)
	return names
}

func traceToolCallNames(toolCalls []ToolCall) []string {
	if len(toolCalls) == 0 {
		return nil
	}
	names := make([]string, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if strings.TrimSpace(tc.Function.Name) == "" {
			continue
		}
		names = append(names, strings.TrimSpace(tc.Function.Name))
	}
	return names
}

func traceMetaLine(meta map[string]string) string {
	if len(meta) == 0 {
		return ""
	}
	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, meta[key]))
	}
	return strings.Join(parts, ", ")
}

func escapeMermaidLabel(s string) string {
	replacer := strings.NewReplacer(
		"\"", "'",
		"`", "'",
		"\n", "<br/>",
		"\r", "",
		"[", "(",
		"]", ")",
		"{", "(",
		"}", ")",
	)
	return replacer.Replace(s)
}

func escapeMarkdownTable(s string) string {
	s = strings.ReplaceAll(s, "\n", "<br/>")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func formatTraceTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format("2006-01-02 15:04:05.000")
}

func appendUniqueTraceChild(children []TraceChildSession, child TraceChildSession) []TraceChildSession {
	if strings.TrimSpace(child.SessionID) == "" {
		return children
	}
	for i := range children {
		if children[i].SessionID != child.SessionID {
			continue
		}
		if child.Title != "" {
			children[i].Title = child.Title
		}
		if child.Scope != "" {
			children[i].Scope = child.Scope
		}
		if child.Status != "" {
			children[i].Status = child.Status
		}
		if child.StartedAt != nil {
			children[i].StartedAt = child.StartedAt
		}
		if child.FinishedAt != nil {
			children[i].FinishedAt = child.FinishedAt
		}
		return children
	}
	return append(children, child)
}

func currentRoundIndex(trace *RequestTrace) int {
	if trace == nil || len(trace.Rounds) == 0 {
		return 1
	}
	maxIndex := 1
	for _, round := range trace.Rounds {
		if round.Index > maxIndex {
			maxIndex = round.Index
		}
	}
	return maxIndex
}

func traceAttachmentKinds(attachments []Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	kinds := make([]string, 0, len(attachments))
	for _, att := range attachments {
		label := strings.TrimSpace(att.Title)
		if label == "" {
			label = string(att.Kind)
		}
		kinds = append(kinds, label)
	}
	return strings.Join(kinds, ",")
}
