package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ========================= 编排器 =========================

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

// extractKeyToolData 从子任务的 ToolCallRecords 中提取关键结构化字段
// 用于 enriched sibling context，让后续依赖子任务能看到 project_dir、session_id 等数据
func extractKeyToolData(session *TaskSession) string {
	session.mu.Lock()
	records := make([]ToolCallRecord, len(session.ToolCalls))
	copy(records, session.ToolCalls)
	session.mu.Unlock()

	var parts []string
	for _, rec := range records {
		if !rec.Success || rec.Result == "" {
			continue
		}
		// 尝试 JSON 解析 result
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(rec.Result), &parsed); err != nil {
			continue
		}

		// 提取顶层或 data 子对象中的关键字段
		keyFields := []string{"project_dir", "session_id", "url", "port", "project", "deploy_target"}
		extracted := make(map[string]string)

		// 顶层字段
		for _, key := range keyFields {
			if val, ok := parsed[key]; ok && val != nil {
				extracted[key] = fmt.Sprintf("%v", val)
			}
		}
		// data 子对象字段
		if data, ok := parsed["data"].(map[string]interface{}); ok {
			for _, key := range keyFields {
				if val, ok := data[key]; ok && val != nil {
					extracted[key] = fmt.Sprintf("%v", val)
				}
			}
		}

		if len(extracted) > 0 {
			var kvs []string
			for k, v := range extracted {
				kvs = append(kvs, fmt.Sprintf("%s=%s", k, v))
			}
			parts = append(parts, fmt.Sprintf("- %s: %s", rec.ToolName, strings.Join(kvs, ", ")))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return "关键工具返回数据（后续子任务必须引用，禁止编造）:\n" + strings.Join(parts, "\n")
}

// detectAsyncResults 从子任务的工具调用记录中检测异步会话
// 通过工具响应中的 status 字段判断：completed/failed 视为同步，in_progress/started 视为异步
func detectAsyncResults(session *TaskSession) []AsyncSessionInfo {
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

		// 通过 status 字段通用判断（不硬编码工具名）
		switch parsed.Status {
		case "completed", "failed":
			// 工具已同步完成，不视为异步
			log.Printf("[Orchestrator] tool %s returned status=%s session=%s, sync completed",
				rec.ToolName, parsed.Status, parsed.SessionID)
			continue
		case "in_progress", "started":
			// 工具确认任务进行中，视为异步
			results = append(results, AsyncSessionInfo{
				ToolName:  rec.ToolName,
				SessionID: parsed.SessionID,
				Message:   parsed.Message,
			})
		default:
			// 无 status 字段（向后兼容未升级的 agent）→ 保持原逻辑视为异步
			log.Printf("[Orchestrator] tool %s has session_id=%s but no status field, treating as async (compat)",
				rec.ToolName, parsed.SessionID)
			results = append(results, AsyncSessionInfo{
				ToolName:  rec.ToolName,
				SessionID: parsed.SessionID,
				Message:   parsed.Message,
			})
		}
	}
	return dedupeAsyncSessions(results)
}

// defaultSubtaskPrompt 子任务 system prompt 的默认内容（workspace/SUBTASK.md 不存在时的 fallback）
var defaultSubtaskPrompt = `你正在执行一个子任务。必须通过调用工具来完成任务。
- 如果任务需要调用多个工具或处理数据，优先使用 ExecuteCode 编写 Python 代码（代码内通过 call_tool() 调用工具）
- call_tool 返回值类型不确定（可能是 str 或 dict），使用前先检查类型
- 工具调用失败时，分析原因并修正参数重试。ExecuteCode 代码报错时修正代码重试，不要放弃沙箱转而逐个调工具
- 直接执行，不要反问
- 回复包含执行结果和关键数据，供后续任务引用`

// SubTaskHandle 子任务运行时句柄（用于 Steer 重定向）
type SubTaskHandle struct {
	SubTaskID string
	SteerCh   chan string
}

// Orchestrator 任务编排器
type Orchestrator struct {
	bridge          *Bridge
	cfg             *Config
	store           *SessionStore
	activeHandles   map[string]*SubTaskHandle
	activeHandlesMu sync.Mutex
}

// NewOrchestrator 创建编排器
func NewOrchestrator(bridge *Bridge, store *SessionStore) *Orchestrator {
	return &Orchestrator{
		bridge:        bridge,
		cfg:           bridge.cfg,
		store:         store,
		activeHandles: make(map[string]*SubTaskHandle),
	}
}

// fallbackCooldown 返回配置的降级冷却时长
func (o *Orchestrator) fallbackCooldown() time.Duration {
	sec := o.cfg.FallbackCooldownSec
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

// sendLLM 带降级链的同步 LLM 请求
func (o *Orchestrator) sendLLM(messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	cfg := o.bridge.activeLLM.Get()
	if len(o.cfg.Fallbacks) == 0 {
		return SendLLMRequest(&cfg, messages, tools)
	}
	return SendLLMRequestWithFallback(&cfg, o.cfg.Fallbacks, o.fallbackCooldown(), messages, tools, o.bridge.cfg.Providers)
}

// sendLLMCtx 带降级链 + context 的同步 LLM 请求
func (o *Orchestrator) sendLLMCtx(ctx context.Context, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	cfg := o.bridge.activeLLM.Get()
	if len(o.cfg.Fallbacks) == 0 {
		return SendLLMRequestCtx(ctx, &cfg, messages, tools)
	}
	// 降级链中逐个尝试，每个都带 context
	candidates := make([]*LLMConfig, 0, 1+len(o.cfg.Fallbacks))
	candidates = append(candidates, &cfg)
	for i := range o.cfg.Fallbacks {
		candidates = append(candidates, &o.cfg.Fallbacks[i])
	}
	cooldown := o.fallbackCooldown()
	var lastErr error
	for _, cfg := range candidates {
		if globalCooldown.isCoolingDown(cfg) {
			continue
		}
		text, toolCalls, err := SendLLMRequestCtx(ctx, cfg, messages, tools)
		if err == nil {
			return text, toolCalls, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return "", nil, err // context cancelled, don't try more
		}
		globalCooldown.setCooldown(cfg, cooldown)
	}
	return "", nil, fmt.Errorf("all models failed: %v", lastErr)
}

// sendStreamingLLM 带降级链的流式 LLM 请求
func (o *Orchestrator) sendStreamingLLM(messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	cfg := o.bridge.activeLLM.Get()
	if len(o.cfg.Fallbacks) == 0 {
		return SendStreamingLLMRequest(&cfg, messages, tools, onChunk, o.cfg.LLMCallIntervalSec)
	}
	return SendStreamingLLMRequestWithFallback(&cfg, o.cfg.Fallbacks, o.fallbackCooldown(), messages, tools, onChunk, o.cfg.LLMCallIntervalSec, o.bridge.cfg.Providers)
}

// ========================= 事件驱动 DAG 调度器 =========================

// dagScheduler 管理 DAG 中子任务的依赖解锁
type dagScheduler struct {
	plan         *TaskPlan
	completedSet map[string]bool
	failedSet    map[string]bool
	asyncSet     map[string]bool
	scheduledSet map[string]bool
	resultMap    map[string]SubTaskResult
	mu           sync.Mutex
}

func newDAGScheduler(plan *TaskPlan) *dagScheduler {
	return &dagScheduler{
		plan:         plan,
		completedSet: make(map[string]bool),
		failedSet:    make(map[string]bool),
		asyncSet:     make(map[string]bool),
		scheduledSet: make(map[string]bool),
		resultMap:    make(map[string]SubTaskResult),
	}
}

// markDone 标记子任务完成，返回新解锁的子任务列表
func (ds *dagScheduler) markDone(id string, result SubTaskResult) []SubTaskPlan {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.resultMap[id] = result
	switch result.Status {
	case "done":
		ds.completedSet[id] = true
	case "failed", "skipped":
		ds.failedSet[id] = true
	case "async", "deferred":
		ds.asyncSet[id] = true
	}

	// 查找新解锁的子任务
	var unblocked []SubTaskPlan
	for _, st := range ds.plan.SubTasks {
		if ds.scheduledSet[st.ID] || ds.completedSet[st.ID] || ds.failedSet[st.ID] || ds.asyncSet[st.ID] {
			continue
		}
		if ds.allDepsResolved(st) {
			unblocked = append(unblocked, st)
			ds.scheduledSet[st.ID] = true
		}
	}
	return unblocked
}

// allDepsResolved 检查子任务的所有依赖是否已解决
func (ds *dagScheduler) allDepsResolved(st SubTaskPlan) bool {
	for _, dep := range st.DependsOn {
		if !ds.completedSet[dep] && !ds.failedSet[dep] && !ds.asyncSet[dep] {
			return false
		}
	}
	return true
}

// getInitialTasks 返回无依赖的初始任务
func (ds *dagScheduler) getInitialTasks() []SubTaskPlan {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	var initial []SubTaskPlan
	for _, st := range ds.plan.SubTasks {
		if len(st.DependsOn) == 0 {
			initial = append(initial, st)
			ds.scheduledSet[st.ID] = true
		}
	}
	return initial
}

// shouldSkip 检查子任务是否因依赖失败而应跳过
func (ds *dagScheduler) shouldSkip(st SubTaskPlan) (skip bool, reason string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	for _, dep := range st.DependsOn {
		if ds.failedSet[dep] {
			return true, "依赖任务失败或被跳过"
		}
	}
	return false, ""
}

// shouldDefer 检查子任务是否因依赖异步而应延迟
func (ds *dagScheduler) shouldDefer(st SubTaskPlan) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	for _, dep := range st.DependsOn {
		if ds.asyncSet[dep] {
			return true
		}
	}
	return false
}

// executeSubTask 执行单个子任务的 agentic loop
func (o *Orchestrator) executeSubTask(
	ctx context.Context,
	taskID string,
	subtask SubTaskPlan,
	session *TaskSession,
	siblingContext string,
	tools []LLMTool,
	sendEvent func(event, text string),
	steerCh <-chan string,
) SubTaskResult {
	subtaskStart := time.Now()
	session.SetStatus("running")
	log.Printf("[Orchestrator] ▶ 子任务开始 id=%s title=%s desc=%s",
		subtask.ID, subtask.Title, subtask.Description)

	toolView := o.bridge.buildSubTaskToolRuntimeView(tools, subtask.ToolsHint)
	filteredTools := toolView.Visible()
	log.Printf("[Orchestrator] 子任务 %s 工具视图: visible=%d all=%d hints=%v", subtask.ID, len(filteredTools), len(toolView.AllTools), subtask.ToolsHint)

	// 构建子任务的 system prompt（使用与主任务相同的函数）
	basePrompt, _ := o.bridge.buildAssistantSystemPrompt(session.Account)
	var systemContent strings.Builder
	systemContent.WriteString(basePrompt)
	systemContent.WriteString("\n\n")
	systemContent.WriteString(fmt.Sprintf("## 当前子任务: %s\n", subtask.Title))
	systemContent.WriteString(fmt.Sprintf("%s\n", subtask.Description))

	if siblingContext != "" {
		systemContent.WriteString("\n## 前置任务结果（可直接引用）\n")
		systemContent.WriteString(siblingContext)
	}

	// 注入与子任务相关的 skill 指引
	if o.bridge.skillMgr != nil && len(subtask.ToolsHint) > 0 {
		matched := o.bridge.skillMgr.MatchByTools(subtask.ToolsHint)
		if len(matched) > 0 {
			skillBlock := o.bridge.skillMgr.BuildSkillBlock(matched)
			systemContent.WriteString(skillBlock)
		}
	}

	// 注入工具参数参考（让子任务 LLM 在 ExecuteCode 中写 call_tool 时有正确的工具名和参数参考）
	toolRef := o.bridge.buildToolParamReference(filteredTools)
	if toolRef != "" {
		systemContent.WriteString(toolRef)
	}

	// 工具使用指引：必须通过 function calling 调用工具，不要尝试 HTTP/API 直连
	systemContent.WriteString("\n## 工具使用规范\n")
	systemContent.WriteString("- 只使用上方列出的工具，通过 function calling 直接调用\n")
	systemContent.WriteString("- 禁止通过 HTTP 请求、API 直连、或其他间接方式访问 agent 服务\n")
	systemContent.WriteString("- 调用工具前，参考上方「工具参数参考」中的参数定义\n")

	// 初始化消息
	messages := []Message{
		{Role: "system", Content: systemContent.String()},
		{Role: "user", Content: subtask.Description},
	}

	// 记录到 session
	session.AppendMessage(messages[0])
	session.AppendMessage(messages[1])

	maxIter := o.cfg.SubTaskMaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	timeout := time.Duration(o.cfg.SubTaskTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	// 子任务涉及长时间工具时，自动扩展超时
	longTimeout := time.Duration(o.cfg.LongToolTimeoutSec) * time.Second
	if longTimeout <= 0 {
		longTimeout = 600 * time.Second
	}
	if hasLongRunningToolHint(subtask.ToolsHint) {
		// 子任务超时 = 长工具超时 + 额外裕量（LLM 思考 + 多轮迭代）
		if longTimeout+60*time.Second > timeout {
			timeout = longTimeout + 60*time.Second
		}
	}
	deadline := time.Now().Add(timeout)
	finalText, loopErr := o.runSubTaskLoop(ctx, taskID, subtask, session, messages, toolView, sendEvent, steerCh, deadline)
	if loopErr != nil {
		status := "failed"
		errText := loopErr.Error()
		if errors.Is(loopErr, errSubTaskCancelled) {
			status = "cancelled"
			errText = "cancelled"
		}
		if errors.Is(loopErr, errSubTaskTimeout) {
			errText = "subtask timeout"
		}
		session.SetStatus(status)
		session.SetError(errText)
		o.saveSession(session)
		return SubTaskResult{
			SubTaskID: subtask.ID,
			Title:     subtask.Title,
			Status:    "failed",
			Error:     errText,
		}
	}

	// 任务完成度判定：如果子任务有 tools_hint 但一个工具都没成功调用过，降级为 "failed"
	session.mu.Lock()
	toolCallRecords := make([]ToolCallRecord, len(session.ToolCalls))
	copy(toolCallRecords, session.ToolCalls)
	session.mu.Unlock()

	hasSuccessfulToolCall := false
	for _, rec := range toolCallRecords {
		if rec.Success {
			hasSuccessfulToolCall = true
			break
		}
	}

	if !hasSuccessfulToolCall && len(subtask.ToolsHint) > 0 && len(toolCallRecords) == 0 {
		session.SetStatus("failed")
		session.SetError("子任务未调用任何工具即结束，可能是前置任务失败或参数缺失")
		o.saveSession(session)

		log.Printf("[Orchestrator] ◀ 子任务降级为失败 id=%s reason=no_tool_calls duration=%v",
			subtask.ID, time.Since(subtaskStart))

		return SubTaskResult{
			SubTaskID: subtask.ID,
			Title:     subtask.Title,
			Status:    "failed",
			Result:    finalText,
			Error:     "子任务未调用任何工具即结束",
		}
	}

	// 检测关键工具的业务级失败：tool 调用成功（网络层）但返回 JSON 中 success=false（业务层）
	// 只检查 ToolsHint 中指定的核心工具，非核心工具的业务失败不影响子任务判定
	var criticalFailure string
	if len(toolCallRecords) > 0 {
		// 构建 ToolsHint 查找表（支持裸名匹配 canonical name 的后缀）
		hintSet := make(map[string]bool, len(subtask.ToolsHint))
		for _, hint := range subtask.ToolsHint {
			hintSet[hint] = true
		}

		lastCallByTool := make(map[string]ToolCallRecord)
		for _, rec := range toolCallRecords {
			if rec.Success {
				lastCallByTool[rec.ToolName] = rec // 后出现的覆盖先出现的，即保留最后一次
			}
		}
		for _, rec := range lastCallByTool {
			if rec.Result == "" {
				continue
			}
			// 如果有 ToolsHint，只检查 hint 中的工具
			if len(hintSet) > 0 {
				isHinted := hintSet[rec.ToolName]
				if !isHinted {
					// 尝试裸名匹配（rec.ToolName 可能是 "agentID.ToolName" 格式）
					if dot := strings.LastIndex(rec.ToolName, "."); dot >= 0 {
						isHinted = hintSet[rec.ToolName[dot+1:]]
					}
				}
				if !isHinted {
					continue // 非核心工具，跳过
				}
			}
			var toolResult struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			}
			if json.Unmarshal([]byte(rec.Result), &toolResult) == nil && !toolResult.Success && toolResult.Error != "" {
				criticalFailure = fmt.Sprintf("%s 业务失败: %s", rec.ToolName, toolResult.Error)
				break
			}
		}
	}

	if criticalFailure != "" {
		session.SetStatus("failed")
		session.SetError(criticalFailure)
		o.saveSession(session)

		log.Printf("[Orchestrator] ◀ 子任务业务级失败 id=%s reason=%s duration=%v",
			subtask.ID, criticalFailure, time.Since(subtaskStart))

		return SubTaskResult{
			SubTaskID: subtask.ID,
			Title:     subtask.Title,
			Status:    "failed",
			Result:    finalText,
			Error:     criticalFailure,
		}
	}

	// 标记完成
	session.SetStatus("done")

	// 构建完整结果：LLM 回复 + 关键工具数据
	fullResult := finalText
	keyData := extractKeyToolData(session)
	if keyData != "" {
		if finalText != "" {
			fullResult = finalText + "\n\n" + keyData
		} else {
			fullResult = keyData
		}
	}

	session.SetResult(fullResult)
	o.saveSession(session)

	log.Printf("[Orchestrator] ◀ 子任务完成 id=%s duration=%v resultLen=%d",
		subtask.ID, time.Since(subtaskStart), len(fullResult))

	return SubTaskResult{
		SubTaskID: subtask.ID,
		Title:     subtask.Title,
		Status:    "done",
		Result:    fullResult,
	}
}

// handleSubTaskFailure LLM 驱动的失败决策
func (o *Orchestrator) handleSubTaskFailure(
	subtask *SubTaskPlan,
	errorMsg string,
	completedResults map[string]string,
	rootSession *TaskSession,
	sendEvent func(event, text string),
) *FailureDecision {
	sendEvent("failure_decision", fmt.Sprintf("子任务 [%s] 失败，正在决策...", subtask.ID))

	failCfg := o.bridge.activeLLM.Get()
	decision, err := MakeFailureDecision(&failCfg, *subtask, errorMsg, completedResults, o.cfg.Fallbacks, o.fallbackCooldown())
	if err != nil {
		decision = &FailureDecision{
			SubTaskID: subtask.ID,
			Action:    "skip",
			Reason:    fmt.Sprintf("decision error: %v", err),
			Timestamp: time.Now(),
		}
	}

	rootSession.AddFailureDecision(*decision)
	o.store.Save(rootSession)

	sendEvent("failure_decision", fmt.Sprintf("决策: %s（%s）", decision.Action, decision.Reason))
	log.Printf("[Orchestrator] failure decision for %s: action=%s reason=%s", subtask.ID, decision.Action, decision.Reason)

	return decision
}

// Synthesize 汇总所有子任务结果
func (o *Orchestrator) Synthesize(
	rootSession *TaskSession,
	childSessions map[string]*TaskSession,
	results []SubTaskResult,
	originalQuery string,
	sendEvent func(event, text string),
) string {
	log.Printf("[Orchestrator] ── 汇总开始 results=%d query=%s", len(results), originalQuery)
	synthStart := time.Now()
	sendEvent("synthesis", "正在整理最终结果...")

	var context strings.Builder
	context.WriteString(fmt.Sprintf("## 原始请求\n%s\n\n", originalQuery))
	context.WriteString("## 子任务执行结果\n")

	for _, r := range results {
		statusEmoji := "✅"
		switch r.Status {
		case "failed":
			statusEmoji = "❌"
		case "skipped":
			statusEmoji = "⏭️"
		}
		context.WriteString(fmt.Sprintf("\n### %s [%s] %s\n", statusEmoji, r.SubTaskID, r.Title))
		if r.Result != "" {
			// 截断过长结果
			result := r.Result
			if len(result) > 2000 {
				result = result[:2000] + "\n...(结果已截断)"
			}
			context.WriteString(result)
			context.WriteString("\n")
		}
		if r.Error != "" {
			context.WriteString(fmt.Sprintf("错误: %s\n", r.Error))
		}
	}

	// 注入关键工具数据：仅在子任务结果过短时补充（结果完整时无需重复注入）
	for _, r := range results {
		if r.Status == "done" && len(r.Result) < 50 {
			if cs, ok := childSessions[r.SubTaskID]; ok {
				keyData := extractKeyToolData(cs)
				if keyData != "" {
					context.WriteString(keyData)
				}
			}
		}
	}

	synthesisPrompt := fmt.Sprintf(`请基于以下子任务执行结果，为用户生成一个完整的回复。

%s

要求：
1. 整合所有子任务的结果为统一的回复
2. 如果结果包含数据表格或统计数据，保留完整数据，不要压缩
3. 如果有失败或跳过的任务，简要说明
4. 回复直接面向用户，不要暴露内部子任务结构
5. 使用 markdown 格式，便于阅读
6. 严格基于子任务结果中的实际数据回复，禁止编造
7. 如果部署工具返回 success:false，必须如实报告部署失败，不要说"已成功部署"`, context.String())

	messages := []Message{
		{Role: "user", Content: synthesisPrompt},
	}

	resp, _, err := o.sendLLM(messages, nil)
	if err != nil {
		log.Printf("[Orchestrator] ✗ 汇总LLM失败 duration=%v error=%v", time.Since(synthStart), err)
		// 降级：直接拼接子任务结果
		var fallback strings.Builder
		for _, r := range results {
			if r.Status == "done" && r.Result != "" {
				fallback.WriteString(r.Result)
				fallback.WriteString("\n\n")
			}
		}
		return fallback.String()
	}

	log.Printf("[Orchestrator] ✓ 汇总完成 duration=%v summaryLen=%d", time.Since(synthStart), len(resp))
	sendEvent("synthesis_done", fmt.Sprintf("结果汇总完成，耗时 %s", fmtDuration(time.Since(synthStart))))
	return resp
}

// resumeSubTask 从断点恢复子任务
func (o *Orchestrator) resumeSubTask(
	session *TaskSession,
	subtask SubTaskPlan,
	siblingContext string,
	tools []LLMTool,
	sendEvent func(event, text string),
) SubTaskResult {
	log.Printf("[Resume] ▶ 恢复子任务 id=%s messages=%d lastRole=%s",
		subtask.ID, len(session.Messages), func() string {
			if len(session.Messages) > 0 {
				return session.Messages[len(session.Messages)-1].Role
			}
			return "none"
		}())
	resumeStart := time.Now()
	session.SetStatus("running")

	// 从 session.Messages 恢复
	messages := make([]Message, len(session.Messages))
	copy(messages, session.Messages)

	if len(messages) == 0 {
		// 没有任何消息，当作新任务执行
		return o.executeSubTask(context.Background(), "", subtask, session, siblingContext, tools, sendEvent, nil)
	}

	// 检查最后一条消息
	lastMsg := messages[len(messages)-1]
	switch lastMsg.Role {
	case "assistant":
		if len(lastMsg.ToolCalls) > 0 {
			// 工具调用中断：重新执行工具
			for _, tc := range lastMsg.ToolCalls {
				originalName := o.bridge.resolveToolName(tc.Function.Name)
				sendEvent("tool_info", fmt.Sprintf("[%s] 恢复工具调用: %s", subtask.ID, originalName))

				tcResult, err := o.bridge.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
				var result string
				if tcResult != nil {
					result = tcResult.Result
					log.Printf("[Orchestrator] resume tool_call: %s →agent=%s ←from=%s", originalName, tcResult.AgentID, tcResult.FromID)
				}
				if err != nil {
					result = fmt.Sprintf("工具调用失败: %v", err)
				}

				// ExecuteCode 特殊处理：只取 stdout
				if originalName == "ExecuteCode" && result != "" {
					stdout, _ := parseExecuteCodeResult(result)
					if stdout != "" {
						result = stdout
					}
				}

				toolMsg := Message{
					Role:       "tool",
					Content:    truncateToolResult(result, 0),
					ToolCallID: tc.ID,
				}
				session.AppendMessage(toolMsg)
				messages = append(messages, toolMsg)
			}
		} else {
			// 已完成但未标记
			session.SetStatus("done")
			session.SetResult(lastMsg.Content)
			o.saveSession(session)
			return SubTaskResult{
				SubTaskID: subtask.ID,
				Title:     subtask.Title,
				Status:    "done",
				Result:    lastMsg.Content,
			}
		}
	case "tool":
		// 工具结果已有，继续下一轮 LLM
	}

	toolView := o.bridge.buildSubTaskToolRuntimeView(tools, subtask.ToolsHint)
	finalText, loopErr := o.runSubTaskLoop(context.Background(), "", subtask, session, messages, toolView, sendEvent, nil, time.Time{})
	if loopErr != nil {
		session.SetStatus("failed")
		session.SetError(loopErr.Error())
		o.saveSession(session)
		return SubTaskResult{
			SubTaskID: subtask.ID,
			Title:     subtask.Title,
			Status:    "failed",
			Error:     loopErr.Error(),
		}
	}

	session.SetStatus("done")
	session.SetResult(finalText)
	o.saveSession(session)

	log.Printf("[Resume] ◀ 子任务恢复完成 id=%s duration=%v resultLen=%d",
		subtask.ID, time.Since(resumeStart), len(finalText))

	return SubTaskResult{
		SubTaskID: subtask.ID,
		Title:     subtask.Title,
		Status:    "done",
		Result:    finalText,
	}
}

// SteerSubTask 向运行中的子任务注入引导消息
func (o *Orchestrator) SteerSubTask(subtaskID, guidance string) error {
	o.activeHandlesMu.Lock()
	handle, ok := o.activeHandles[subtaskID]
	o.activeHandlesMu.Unlock()
	if !ok {
		return fmt.Errorf("subtask %s not active or not found", subtaskID)
	}
	select {
	case handle.SteerCh <- guidance:
		log.Printf("[Orchestrator] steer sent to subtask %s: %s", subtaskID, guidance)
		return nil
	default:
		return fmt.Errorf("subtask %s steer channel full", subtaskID)
	}
}

// ========================= 工具函数 =========================

// topologicalSort DAG 拓扑排序，返回执行层级
func topologicalSort(subtasks []SubTaskPlan) [][]string {
	// 构建入度表和邻接表
	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	for _, st := range subtasks {
		if _, ok := inDegree[st.ID]; !ok {
			inDegree[st.ID] = 0
		}
		for _, dep := range st.DependsOn {
			adj[dep] = append(adj[dep], st.ID)
			inDegree[st.ID]++
		}
	}

	var layers [][]string

	for len(inDegree) > 0 {
		// 找出所有入度为 0 的节点
		var layer []string
		for id, deg := range inDegree {
			if deg == 0 {
				layer = append(layer, id)
			}
		}

		if len(layer) == 0 {
			// 存在循环依赖，将剩余节点全部放入最后一层
			log.Printf("[Orchestrator] warn: circular dependency detected, forcing remaining tasks")
			var remaining []string
			for id := range inDegree {
				remaining = append(remaining, id)
			}
			layers = append(layers, remaining)
			break
		}

		layers = append(layers, layer)

		// 移除已处理节点，更新入度
		for _, id := range layer {
			delete(inDegree, id)
			for _, next := range adj[id] {
				if _, ok := inDegree[next]; ok {
					inDegree[next]--
				}
			}
		}
	}

	return layers
}

// buildSiblingContext 构建已完成兄弟任务的结果上下文
func buildSiblingContext(dependsOn []string, completedResults map[string]string) string {
	if len(dependsOn) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, depID := range dependsOn {
		result, ok := completedResults[depID]
		if !ok {
			continue
		}
		// 截断过长结果
		if len(result) > 3000 {
			result = result[:3000] + "\n...(已截断)"
		}
		sb.WriteString(fmt.Sprintf("### 任务 %s 的结果:\n%s\n\n", depID, result))
	}
	return sb.String()
}

// excludeVirtualTools 排除子任务不应递归调用的规划类工具。
// 保留普通执行工具（如 ExecuteCode），让子任务拥有更接近 Claude Code 的当前轮工具视图。
func excludeVirtualTools(tools []LLMTool, toolsHint []string) []LLMTool {
	var filtered []LLMTool
	for _, tool := range tools {
		name := tool.Function.Name
		switch {
		case name == "plan_and_execute":
			// 总是跳过
		default:
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// hasLongRunningToolHint 检查子任务的 tools_hint 是否包含长时间运行的工具
func hasLongRunningToolHint(hints []string) bool {
	for _, h := range hints {
		if isLongRunningTool(h) {
			return true
		}
	}
	return false
}

// indexOf 查找子任务在计划中的位置
func indexOf(subtasks []SubTaskPlan, id string) int {
	for i, st := range subtasks {
		if st.ID == id {
			return i
		}
	}
	return -1
}
