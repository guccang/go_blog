package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// subtaskEventSink 子任务中 execute_skill 使用的 EventSink 适配器
type subtaskEventSink struct {
	sendEvent func(event, text string)
}

func (s *subtaskEventSink) OnChunk(text string)        {}
func (s *subtaskEventSink) OnEvent(event, text string) { s.sendEvent(event, text) }
func (s *subtaskEventSink) Streaming() bool            { return false }

// SubTaskResult 子任务执行结果
type SubTaskResult struct {
	SubTaskID     string             `json:"sub_task_id"`
	Title         string             `json:"title"`
	Status        string             `json:"status"` // done/failed/skipped/async/deferred
	Result        string             `json:"result"`
	Error         string             `json:"error,omitempty"`
	AsyncSessions []AsyncSessionInfo `json:"async_sessions,omitempty"`
}

// AsyncSessionInfo 异步会话信息（从工具调用结果中检测）
type AsyncSessionInfo struct {
	ToolName  string `json:"tool_name"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

const keyToolDataHeader = "关键工具返回数据（后续步骤必须引用，禁止编造）:"

func canonicalToolName(toolName string) string {
	toolName = strings.TrimSpace(toolName)
	if dot := strings.LastIndex(toolName, "."); dot >= 0 && dot+1 < len(toolName) {
		return toolName[dot+1:]
	}
	return toolName
}

func isTerminalSessionTool(toolName string) bool {
	switch canonicalToolName(toolName) {
	case "AcpStartSession", "CodegenStartSession", "DeployProject", "DeployAdhoc":
		return true
	default:
		return false
	}
}

func buildTerminalToolSummary(toolName, result string) string {
	summary := strings.TrimSpace(result)
	var parsed struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Project      string `json:"project"`
			ProjectDir   string `json:"project_dir"`
			SessionID    string `json:"session_id"`
			URL          string `json:"url"`
			DeployTarget string `json:"deploy_target"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return summary
	}

	var parts []string
	if parsed.Message != "" {
		parts = append(parts, parsed.Message)
	}
	if parsed.Status != "" {
		parts = append(parts, "status="+parsed.Status)
	}
	if parsed.Data.Project != "" {
		parts = append(parts, "project="+parsed.Data.Project)
	}
	if parsed.Data.ProjectDir != "" {
		parts = append(parts, "project_dir="+parsed.Data.ProjectDir)
	}
	if parsed.Data.SessionID != "" {
		parts = append(parts, "session_id="+parsed.Data.SessionID)
	}
	if parsed.Data.URL != "" {
		parts = append(parts, "url="+parsed.Data.URL)
	}
	if parsed.Data.DeployTarget != "" {
		parts = append(parts, "deploy_target="+parsed.Data.DeployTarget)
	}
	if len(parts) == 0 {
		return summary
	}
	return fmt.Sprintf("%s 完成: %s", canonicalToolName(toolName), strings.Join(parts, ", "))
}

func buildRecentMessageContext(session *TaskSession, maxMessages int) string {
	if session == nil || maxMessages <= 0 {
		return ""
	}
	session.mu.Lock()
	messages := make([]Message, len(session.Messages))
	copy(messages, session.Messages)
	session.mu.Unlock()

	var kept []Message
	for i := len(messages) - 1; i >= 0 && len(kept) < maxMessages; i-- {
		msg := messages[i]
		if msg.Role == "system" {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
			continue
		}
		kept = append(kept, msg)
	}
	if len(kept) == 0 {
		return ""
	}

	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}

	var sb strings.Builder
	for _, msg := range kept {
		sb.WriteString(fmt.Sprintf("[%s]\n%s\n\n", msg.Role, truncate(strings.TrimSpace(msg.Content), 800)))
	}
	return strings.TrimSpace(sb.String())
}

// extractKeyToolData 从工具返回中提取 project_dir、session_id 等关键字段。
func extractKeyToolData(session *TaskSession) string {
	if session == nil {
		return ""
	}
	session.mu.Lock()
	records := make([]ToolCallRecord, len(session.ToolCalls))
	copy(records, session.ToolCalls)
	session.mu.Unlock()

	var parts []string
	for _, rec := range records {
		if !rec.Success || rec.Result == "" {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(rec.Result), &parsed); err != nil {
			continue
		}

		keyFields := []string{"project_dir", "session_id", "url", "port", "project", "deploy_target"}
		extracted := make(map[string]string)
		for _, key := range keyFields {
			if val, ok := parsed[key]; ok && val != nil {
				extracted[key] = fmt.Sprintf("%v", val)
			}
		}
		if data, ok := parsed["data"].(map[string]interface{}); ok {
			for _, key := range keyFields {
				if val, ok := data[key]; ok && val != nil {
					extracted[key] = fmt.Sprintf("%v", val)
				}
			}
		}

		if len(extracted) == 0 {
			continue
		}
		var kvs []string
		for _, k := range keyFields {
			if v, ok := extracted[k]; ok {
				kvs = append(kvs, fmt.Sprintf("%s=%s", k, v))
			}
		}
		parts = append(parts, fmt.Sprintf("- %s: %s", rec.ToolName, strings.Join(kvs, ", ")))
	}

	if len(parts) == 0 {
		return ""
	}
	return keyToolDataHeader + "\n" + strings.Join(parts, "\n")
}

func buildSubTaskResultText(summary, keyData string) string {
	summary = strings.TrimSpace(summary)
	keyData = strings.TrimSpace(keyData)
	switch {
	case keyData == "":
		return summary
	case summary == "":
		return keyData
	default:
		return keyData + "\n\n结果摘要:\n" + summary
	}
}

func detectAsyncResults(session *TaskSession) []AsyncSessionInfo {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	records := make([]ToolCallRecord, len(session.ToolCalls))
	copy(records, session.ToolCalls)
	session.mu.Unlock()

	var results []AsyncSessionInfo
	for _, rec := range records {
		if !rec.Success {
			continue
		}
		var parsed struct {
			Success   bool   `json:"success"`
			Status    string `json:"status"`
			SessionID string `json:"session_id"`
			Message   string `json:"message"`
		}
		if err := json.Unmarshal([]byte(rec.Result), &parsed); err != nil {
			continue
		}
		if !parsed.Success || parsed.SessionID == "" {
			continue
		}
		switch parsed.Status {
		case "completed", "failed":
			continue
		case "in_progress", "started", "queued":
			results = append(results, AsyncSessionInfo{
				ToolName:  rec.ToolName,
				SessionID: parsed.SessionID,
				Message:   parsed.Message,
			})
		default:
			results = append(results, AsyncSessionInfo{
				ToolName:  rec.ToolName,
				SessionID: parsed.SessionID,
				Message:   parsed.Message,
			})
		}
	}
	return dedupeAsyncSessions(results)
}

// Orchestrator 现仅负责隔离子任务运行和 runtime attachment/mailbox 注入。
type Orchestrator struct {
	bridge *Bridge
	cfg    *Config
	store  *SessionStore
}

func NewOrchestrator(bridge *Bridge, store *SessionStore) *Orchestrator {
	return &Orchestrator{
		bridge: bridge,
		cfg:    bridge.cfg,
		store:  store,
	}
}

func (o *Orchestrator) fallbackCooldown() time.Duration {
	sec := o.cfg.FallbackCooldownSec
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

func (o *Orchestrator) sendLLMCtx(ctx context.Context, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	cfg := o.bridge.activeLLM.Get()
	if len(o.cfg.Fallbacks) == 0 {
		return SendLLMRequestCtx(ctx, &cfg, messages, tools)
	}
	candidates := make([]*LLMConfig, 0, 1+len(o.cfg.Fallbacks))
	candidates = append(candidates, &cfg)
	for i := range o.cfg.Fallbacks {
		candidates = append(candidates, &o.cfg.Fallbacks[i])
	}
	cooldown := o.fallbackCooldown()
	var lastErr error
	for _, candidate := range candidates {
		if globalCooldown.isCoolingDown(candidate) {
			continue
		}
		text, toolCalls, err := SendLLMRequestCtx(ctx, candidate, messages, tools)
		if err == nil {
			return text, toolCalls, nil
		}
		lastErr = err
		if ctx != nil && ctx.Err() != nil {
			return "", nil, err
		}
		globalCooldown.setCooldown(candidate, cooldown)
	}
	return "", nil, fmt.Errorf("all models failed: %v", lastErr)
}

func (o *Orchestrator) enqueueAttachment(rootID, targetSessionID, sourceSessionID string, kind AttachmentKind, title, content string, meta map[string]string) {
	if o.store == nil {
		return
	}
	rootID = strings.TrimSpace(rootID)
	targetSessionID = strings.TrimSpace(targetSessionID)
	content = strings.TrimSpace(content)
	if rootID == "" || targetSessionID == "" || content == "" {
		return
	}
	msg := newMailboxEntry(rootID, targetSessionID, sourceSessionID, string(kind), title, content, meta)
	if err := o.store.WriteToMailbox(rootID, targetSessionID, msg); err != nil {
		log.Printf("[Orchestrator] warn: enqueue mailbox failed root=%s target=%s kind=%s err=%v", rootID, targetSessionID, kind, err)
	}
}

func (o *Orchestrator) enqueueRuntimeMessage(rootID, targetSessionID, sourceSessionID string, kind RuntimeAttachmentKind, title, content string, meta map[string]string) {
	o.enqueueAttachment(rootID, targetSessionID, sourceSessionID, kind, title, content, meta)
}

func (o *Orchestrator) queueDependencyContext(session *TaskSession, subtask SubTaskPlan, siblingContext string) {
	siblingContext = strings.TrimSpace(siblingContext)
	if session == nil || siblingContext == "" {
		return
	}
	meta := map[string]string{
		"subtask_id":   subtask.ID,
		"context_mode": subtask.EffectiveContextMode(),
	}
	o.enqueueAttachment(session.RootID, session.ID, session.ParentID, AttachmentKindDependencyResult, "前置结果", siblingContext, meta)
}

func (o *Orchestrator) queueForkContext(session *TaskSession, parent *TaskSession, subtask SubTaskPlan) {
	if session == nil || parent == nil || subtask.EffectiveContextMode() != "fork" {
		return
	}
	content := buildRecentMessageContext(parent, 6)
	if content == "" {
		return
	}
	o.enqueueAttachment(
		session.RootID,
		session.ID,
		parent.ID,
		AttachmentKindDependencyResult,
		"父任务最近上下文",
		content,
		map[string]string{
			"subtask_id":   subtask.ID,
			"context_mode": "fork",
		},
	)
}

func buildTaskNotificationContent(result SubTaskResult, childSession *TaskSession) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("子任务[%s] %s\n", result.SubTaskID, fallbackText(strings.TrimSpace(result.Title), result.SubTaskID)))
	sb.WriteString(fmt.Sprintf("状态: %s\n", fallbackText(strings.TrimSpace(result.Status), "unknown")))
	if result.Result != "" {
		sb.WriteString("结果摘要:\n")
		sb.WriteString(truncate(result.Result, 2400))
		sb.WriteString("\n")
	}
	if childSession != nil {
		if keyData := strings.TrimSpace(extractKeyToolData(childSession)); keyData != "" && !strings.Contains(result.Result, keyData) {
			sb.WriteString("\n")
			sb.WriteString(keyData)
			sb.WriteString("\n")
		}
	}
	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("\n错误: %s\n", truncate(result.Error, 600)))
	}
	if len(result.AsyncSessions) > 0 {
		sb.WriteString("\n异步会话:\n")
		for _, info := range result.AsyncSessions {
			sb.WriteString(fmt.Sprintf("- %s session_id=%s", fallbackText(strings.TrimSpace(info.ToolName), "async_tool"), info.SessionID))
			if strings.TrimSpace(info.Message) != "" {
				sb.WriteString(" ")
				sb.WriteString(truncate(strings.TrimSpace(info.Message), 200))
			}
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

func (o *Orchestrator) enqueueTaskNotification(parentSession *TaskSession, subtask SubTaskPlan, result SubTaskResult, childSession *TaskSession) {
	if parentSession == nil {
		return
	}
	meta := map[string]string{
		"subtask_id":   subtask.ID,
		"status":       result.Status,
		"context_mode": subtask.EffectiveContextMode(),
	}
	content := buildTaskNotificationContent(result, childSession)
	o.enqueueAttachment(parentSession.RootID, parentSession.ID, childSession.ID, AttachmentKindTaskNotification, "子任务状态通知", content, meta)
}

func (o *Orchestrator) drainMailboxAttachments(sessionRT *QuerySession, session *TaskSession) int {
	if o.store == nil || sessionRT == nil || session == nil {
		return 0
	}
	msgs, err := o.store.DrainMailbox(session.RootID, session.ID)
	if err != nil {
		log.Printf("[Orchestrator] warn: drain mailbox failed root=%s session=%s err=%v", session.RootID, session.ID, err)
		return 0
	}
	return sessionRT.InjectAttachments(attachmentsFromMailbox(msgs), session)
}

func (o *Orchestrator) drainRuntimeMailbox(sessionRT *SessionRuntime, session *TaskSession) int {
	return o.drainMailboxAttachments(sessionRT, session)
}

func (o *Orchestrator) saveSessionCheckpoint(sessionRT *QuerySession, session *TaskSession, query string, promptCtx SystemPromptContext, trace *RequestTrace) {
	if o.store == nil || sessionRT == nil || session == nil {
		return
	}
	if trace != nil {
		trace.RefreshFromSession(session)
	}
	o.saveSession(session)
	snapshot := sessionRT.Snapshot(session.RootID, session.ID, query, session.Status, promptCtx)
	if err := o.store.SaveRuntimeSnapshot(snapshot); err != nil {
		log.Printf("[Orchestrator] warn: save runtime state failed session=%s err=%v", session.ID, err)
	}
	if trace != nil {
		if err := o.store.SaveRequestTrace(trace); err != nil {
			log.Printf("[Orchestrator] warn: save trace failed session=%s err=%v", session.ID, err)
		}
	}
}

func (o *Orchestrator) persistSessionRuntimeState(sessionRT *SessionRuntime, session *TaskSession, query string, promptCtx PromptContext) {
	o.saveSessionCheckpoint(sessionRT, session, query, promptCtx, nil)
}

func (o *Orchestrator) updateRuntimeSnapshotStatus(session *TaskSession, query string, promptCtx SystemPromptContext) {
	if o.store == nil || session == nil {
		return
	}
	if snapshot, err := o.store.LoadRuntimeSnapshot(session.RootID, session.ID); err == nil && snapshot != nil {
		snapshot.Query = query
		snapshot.Status = session.Status
		if strings.TrimSpace(snapshot.PromptContext.SystemPrompt) == "" {
			snapshot.PromptContext = promptCtx
		}
		if err := o.store.SaveRuntimeSnapshot(*snapshot); err != nil {
			log.Printf("[Orchestrator] warn: update runtime state failed session=%s err=%v", session.ID, err)
		}
		return
	}
	if err := o.store.SaveRuntimeSnapshot(RuntimeSnapshot{
		RootID:        session.RootID,
		SessionID:     session.ID,
		Query:         query,
		Status:        session.Status,
		PromptContext: promptCtx,
	}); err != nil {
		log.Printf("[Orchestrator] warn: create runtime state failed session=%s err=%v", session.ID, err)
	}
}

func (o *Orchestrator) persistRuntimeStateStatus(session *TaskSession, query string, promptCtx PromptContext) {
	o.updateRuntimeSnapshotStatus(session, query, promptCtx)
}

func (o *Orchestrator) executeSubTask(
	ctx context.Context,
	taskID string,
	subtask SubTaskPlan,
	session *TaskSession,
	parentSession *TaskSession,
	siblingContext string,
	tools []LLMTool,
	promptOverride string,
	sendEvent func(event, text string),
	trace *RequestTrace,
) SubTaskResult {
	subtaskStart := time.Now()
	session.SetStatus("running")
	log.Printf("[Orchestrator] ▶ 子任务开始 id=%s title=%s desc=%s", subtask.ID, subtask.Title, subtask.Description)

	toolView := o.bridge.buildSubTaskToolRuntimeView(tools, subtask.ToolsHint)
	filteredTools := toolView.Visible()
	if trace != nil {
		trace.RefreshFromSession(session)
		trace.SetDescription(subtask.Description)
		trace.SetToolView(toolView)
		trace.RecordPath("subtask_start", fmt.Sprintf("title=%s", subtask.Title), map[string]string{
			"context_mode": subtask.EffectiveContextMode(),
		})
		trace.RecordPath("subtask_tool_view", fmt.Sprintf("policy=%s visible=%d all=%d", toolView.Policy, len(toolView.VisibleTools), len(toolView.AllTools)), map[string]string{
			"hints":          strings.Join(toolView.Hints, ","),
			"matched_skills": strings.Join(toolView.MatchedSkills, ","),
		})
	}

	var systemContent strings.Builder
	if strings.TrimSpace(promptOverride) != "" {
		systemContent.WriteString(strings.TrimSpace(promptOverride))
	} else {
		basePrompt, _ := o.bridge.buildAssistantSystemPrompt(session.Account)
		systemContent.WriteString(basePrompt)
		systemContent.WriteString("\n\n")
		systemContent.WriteString(fmt.Sprintf("## 当前子任务: %s\n", subtask.Title))
		systemContent.WriteString(fmt.Sprintf("%s\n", subtask.Description))

		if o.bridge.skillMgr != nil && len(subtask.ToolsHint) > 0 {
			matched := o.bridge.skillMgr.MatchByTools(subtask.ToolsHint)
			if len(matched) > 0 {
				systemContent.WriteString(o.bridge.skillMgr.BuildSkillBlock(matched))
			}
		}
		if toolRef := o.bridge.buildToolParamReference(filteredTools); toolRef != "" {
			systemContent.WriteString(toolRef)
		}
		systemContent.WriteString("\n## 工具使用规范\n")
		systemContent.WriteString("- 只使用上方列出的工具，通过 function calling 直接调用\n")
		systemContent.WriteString("- 禁止通过 HTTP 请求、API 直连、或其他间接方式访问 agent 服务\n")
		systemContent.WriteString("- 调用工具前，参考上方工具参数参考中的参数定义\n")
	}

	messages := []Message{
		{Role: "system", Content: systemContent.String()},
		{Role: "user", Content: subtask.Description},
	}
	promptCtx := PromptContext{
		Account:      session.Account,
		Source:       session.Source,
		SystemPrompt: systemContent.String(),
	}

	session.AppendMessage(messages[0])
	session.AppendMessage(messages[1])
	if trace != nil {
		trace.RecordPath("subtask_prompt_ready", fmt.Sprintf("messages=%d prompt_len=%d", len(messages), len(systemContent.String())), nil)
	}
	o.queueDependencyContext(session, subtask, siblingContext)
	o.queueForkContext(session, parentSession, subtask)
	o.saveSession(session)

	timeout := time.Duration(o.cfg.SubTaskTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	longTimeout := time.Duration(o.cfg.LongToolTimeoutSec) * time.Second
	if longTimeout <= 0 {
		longTimeout = 600 * time.Second
	}
	if hasLongRunningToolHint(subtask.ToolsHint) && longTimeout+60*time.Second > timeout {
		timeout = longTimeout + 60*time.Second
	}

	finalText, loopErr := o.runSubTaskLoop(
		ctx,
		taskID,
		subtask,
		session,
		messages,
		toolView,
		promptCtx,
		sendEvent,
		nil,
		time.Now().Add(timeout),
		trace,
	)
	if loopErr != nil {
		session.SetStatus("failed")
		session.SetError(loopErr.Error())
		o.saveSession(session)
		o.persistRuntimeStateStatus(session, subtask.Description, promptCtx)
		if trace != nil {
			trace.RecordPath("subtask_finish", "子任务失败退出", map[string]string{"status": "failed"})
			trace.Finish(session.Status, finalText, loopErr)
			if err := o.store.SaveRequestTrace(trace); err != nil {
				log.Printf("[Orchestrator] warn: save subtask trace failed session=%s err=%v", session.ID, err)
			}
		}
		return SubTaskResult{
			SubTaskID: subtask.ID,
			Title:     subtask.Title,
			Status:    "failed",
			Error:     loopErr.Error(),
		}
	}

	if asyncInfos := detectAsyncResults(session); len(asyncInfos) > 0 {
		fullResult := buildSubTaskResultText(finalText, extractKeyToolData(session))
		session.SetStatus("async")
		session.SetResult(fullResult)
		o.saveSession(session)
		o.persistRuntimeStateStatus(session, subtask.Description, promptCtx)
		if trace != nil {
			trace.RecordPath("subtask_finish", "子任务进入异步状态", map[string]string{"status": "async"})
			trace.Finish(session.Status, fullResult, nil)
			if err := o.store.SaveRequestTrace(trace); err != nil {
				log.Printf("[Orchestrator] warn: save subtask trace failed session=%s err=%v", session.ID, err)
			}
		}
		log.Printf("[Orchestrator] ◀ 子任务异步完成 id=%s duration=%v", subtask.ID, time.Since(subtaskStart))
		return SubTaskResult{
			SubTaskID:     subtask.ID,
			Title:         subtask.Title,
			Status:        "async",
			Result:        fullResult,
			AsyncSessions: asyncInfos,
		}
	}

	fullResult := buildSubTaskResultText(finalText, extractKeyToolData(session))
	session.SetStatus("done")
	session.SetResult(fullResult)
	o.saveSession(session)
	o.persistRuntimeStateStatus(session, subtask.Description, promptCtx)
	if trace != nil {
		trace.RecordPath("subtask_finish", "子任务完成", map[string]string{"status": "done"})
		trace.Finish(session.Status, fullResult, nil)
		if err := o.store.SaveRequestTrace(trace); err != nil {
			log.Printf("[Orchestrator] warn: save subtask trace failed session=%s err=%v", session.ID, err)
		}
	}

	log.Printf("[Orchestrator] ◀ 子任务完成 id=%s duration=%v resultLen=%d", subtask.ID, time.Since(subtaskStart), len(fullResult))
	return SubTaskResult{
		SubTaskID: subtask.ID,
		Title:     subtask.Title,
		Status:    "done",
		Result:    fullResult,
	}
}

func excludeVirtualTools(tools []LLMTool, _ []string) []LLMTool {
	var filtered []LLMTool
	for _, tool := range tools {
		name := tool.Function.Name
		switch name {
		case "execute_skill":
		default:
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func hasLongRunningToolHint(hints []string) bool {
	for _, h := range hints {
		if isLongRunningTool(h) {
			return true
		}
	}
	return false
}

func sortChildSessionIDs(children map[string]*TaskSession) []string {
	ids := make([]string, 0, len(children))
	for id := range children {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
