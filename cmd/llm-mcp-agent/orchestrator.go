package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ========================= 编排器 =========================

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
	return results
}

// Orchestrator 任务编排器
type Orchestrator struct {
	bridge *Bridge
	cfg    *Config
	store  *SessionStore
}

// NewOrchestrator 创建编排器
func NewOrchestrator(bridge *Bridge, store *SessionStore) *Orchestrator {
	return &Orchestrator{
		bridge: bridge,
		cfg:    bridge.cfg,
		store:  store,
	}
}

// Execute 按 DAG 拓扑排序执行所有子任务
func (o *Orchestrator) Execute(
	taskID string,
	rootSession *TaskSession,
	childSessions map[string]*TaskSession,
	tools []LLMTool,
	sendEvent func(event, text string),
) []SubTaskResult {
	plan := rootSession.Plan
	if plan == nil || len(plan.SubTasks) == 0 {
		return nil
	}

	// 拓扑排序
	layers := topologicalSort(plan.SubTasks)
	log.Printf("[Orchestrator] topological sort: %d layers", len(layers))

	// 已完成子任务的结果（供后续子任务引用）
	completedResults := make(map[string]string)
	var allResults []SubTaskResult

	for layerIdx, layer := range layers {
		log.Printf("[Orchestrator] executing layer %d/%d: %v", layerIdx+1, len(layers), layer)

		for _, subtaskID := range layer {
			// 查找子任务计划
			var subtask *SubTaskPlan
			for i := range plan.SubTasks {
				if plan.SubTasks[i].ID == subtaskID {
					subtask = &plan.SubTasks[i]
					break
				}
			}
			if subtask == nil {
				continue
			}

			// 查找对应的 session
			session, ok := childSessions[subtaskID]
			if !ok {
				log.Printf("[Orchestrator] warn: no session for subtask %s", subtaskID)
				continue
			}

			// 检查依赖是否被跳过/失败
			skipDueToDepFailure := false
			for _, depID := range subtask.DependsOn {
				for _, r := range allResults {
					if r.SubTaskID == depID && (r.Status == "skipped" || r.Status == "failed") {
						skipDueToDepFailure = true
						break
					}
				}
				if skipDueToDepFailure {
					break
				}
			}

			if skipDueToDepFailure {
				session.SetStatus("skipped")
				session.SetError("依赖任务失败或被跳过")
				o.store.Save(session)
				sendEvent("subtask_skip", fmt.Sprintf("[%s] %s — 跳过（依赖任务未完成）", subtask.ID, subtask.Title))
				allResults = append(allResults, SubTaskResult{
					SubTaskID: subtask.ID,
					Title:     subtask.Title,
					Status:    "skipped",
					Error:     "依赖任务失败或被跳过",
				})
				continue
			}

			// 检查依赖是否为 async/deferred
			deferDueToAsync := false
			if !skipDueToDepFailure {
				for _, depID := range subtask.DependsOn {
					for _, r := range allResults {
						if r.SubTaskID == depID && (r.Status == "async" || r.Status == "deferred") {
							deferDueToAsync = true
							break
						}
					}
					if deferDueToAsync {
						break
					}
				}
			}

			if deferDueToAsync {
				session.SetStatus("deferred")
				session.SetError("前置任务仍在异步执行中")
				o.store.Save(session)
				sendEvent("subtask_defer", fmt.Sprintf("[%s] %s — 等待前置任务完成", subtask.ID, subtask.Title))
				allResults = append(allResults, SubTaskResult{
					SubTaskID: subtask.ID,
					Title:     subtask.Title,
					Status:    "deferred",
					Error:     "前置任务仍在异步执行中",
				})
				continue
			}

			// 构建兄弟结果上下文
			siblingContext := buildSiblingContext(subtask.DependsOn, completedResults)

			// 发送进度事件
			taskIdx := indexOf(plan.SubTasks, subtask.ID)
			sendEvent("subtask_start", fmt.Sprintf("[%d/%d] %s\n描述: %s", taskIdx+1, len(plan.SubTasks), subtask.Title, subtask.Description))

			// 执行子任务
			result := o.executeSubTask(taskID, *subtask, session, siblingContext, tools, sendEvent)

			// 检测异步工具调用
			if result.Status == "done" {
				asyncInfos := detectAsyncResults(session)
				if len(asyncInfos) > 0 {
					result.Status = "async"
					result.AsyncSessions = asyncInfos
					session.SetStatus("async")
					o.store.Save(session)
					var asyncDetails []string
					for _, a := range asyncInfos {
						detail := fmt.Sprintf("%s→%s", a.ToolName, a.SessionID)
						if a.Message != "" {
							detail += ": " + a.Message
						}
						asyncDetails = append(asyncDetails, detail)
					}
					log.Printf("[Orchestrator] async detected: subtask=%s sessions=%v", subtask.ID, asyncDetails)
					sendEvent("subtask_async", fmt.Sprintf("[%d/%d] %s — 异步执行中\n%s",
						taskIdx+1, len(plan.SubTasks), subtask.Title, strings.Join(asyncDetails, "\n")))
				}
			}

			// 处理失败
			if result.Status == "failed" {
				sendEvent("subtask_fail", fmt.Sprintf("[%s] %s — 失败: %s", subtask.ID, subtask.Title, result.Error))

				decision := o.handleSubTaskFailure(subtask, result.Error, completedResults, rootSession, sendEvent)

				switch decision.Action {
				case "retry":
					sendEvent("retry_detail", fmt.Sprintf("[%s] 重试原因: %s\n原始错误: %s", subtask.ID, decision.Reason, result.Error))
					sendEvent("subtask_start", fmt.Sprintf("[%d/%d] 重试: %s", taskIdx+1, len(plan.SubTasks), subtask.Title))
					result = o.executeSubTask(taskID, *subtask, session, siblingContext, tools, sendEvent)

				case "modify":
					// 用修改后的描述重试
					modifiedSubtask := *subtask
					modifiedSubtask.Description = decision.Modifications
					sendEvent("modify_detail", fmt.Sprintf("[%s] 修改后重试\n原描述: %s\n新描述: %s", subtask.ID, truncate(subtask.Description, 200), truncate(decision.Modifications, 200)))
					sendEvent("subtask_start", fmt.Sprintf("[%d/%d] 修改后重试: %s", taskIdx+1, len(plan.SubTasks), subtask.Title))
					result = o.executeSubTask(taskID, modifiedSubtask, session, siblingContext, tools, sendEvent)

				case "skip":
					result.Status = "skipped"
					session.SetStatus("skipped")
					o.store.Save(session)
					sendEvent("subtask_skip", fmt.Sprintf("[%s] %s — 已跳过", subtask.ID, subtask.Title))

				case "abort":
					result.Status = "failed"
					session.SetStatus("failed")
					o.store.Save(session)
					sendEvent("subtask_fail", fmt.Sprintf("编排终止: %s", decision.Reason))
					allResults = append(allResults, result)
					// 中止后续执行
					return allResults
				}
			}

			if result.Status == "done" {
				completedResults[subtask.ID] = result.Result
				sendEvent("subtask_done", fmt.Sprintf("[%d/%d] %s — 完成", taskIdx+1, len(plan.SubTasks), subtask.Title))
				if result.Result != "" {
					sendEvent("subtask_result", fmt.Sprintf("[%s] 结果: %s", subtask.ID, truncate(result.Result, 500)))
				}
			}

			allResults = append(allResults, result)

			// 整体进度计数
			sendEvent("progress", fmt.Sprintf("[%d/%d 子任务已处理]", len(allResults), len(plan.SubTasks)))
		}
	}

	return allResults
}

// executeSubTask 执行单个子任务的 agentic loop
func (o *Orchestrator) executeSubTask(
	taskID string,
	subtask SubTaskPlan,
	session *TaskSession,
	siblingContext string,
	tools []LLMTool,
	sendEvent func(event, text string),
) SubTaskResult {
	subtaskStart := time.Now()
	session.SetStatus("running")
	log.Printf("[Orchestrator] ▶ 子任务开始 id=%s title=%s desc=%s",
		subtask.ID, subtask.Title, truncate(subtask.Description, 150))

	// 构建子任务的 system prompt
	var systemContent strings.Builder
	systemContent.WriteString("你正在执行一个子任务。必须通过调用工具来完成任务。\n")
	systemContent.WriteString("如果工具调用失败，请分析原因并尝试修正参数后重试。不要仅用文字回复而不调用工具。\n")
	systemContent.WriteString("直接执行，不要反问。\n\n")
	systemContent.WriteString(fmt.Sprintf("当前用户账号: %s\n", session.Account))
	systemContent.WriteString(fmt.Sprintf("当前日期: %s\n\n", time.Now().Format("2006-01-02")))
	systemContent.WriteString(fmt.Sprintf("## 子任务: %s\n", subtask.Title))
	systemContent.WriteString(fmt.Sprintf("%s\n", subtask.Description))

	if siblingContext != "" {
		systemContent.WriteString("\n## 前置任务结果（可直接引用）\n")
		systemContent.WriteString(siblingContext)
	}

	// 初始化消息
	messages := []Message{
		{Role: "system", Content: systemContent.String()},
		{Role: "user", Content: subtask.Description},
	}

	// 记录到 session
	session.AppendMessage(messages[0])
	session.AppendMessage(messages[1])

	// 过滤工具（如果有 tools_hint）
	filteredTools := tools
	if len(subtask.ToolsHint) > 0 {
		filteredTools = filterToolsByHint(tools, subtask.ToolsHint)
		// 如果过滤后为空，回退到全部工具
		if len(filteredTools) == 0 {
			filteredTools = tools
		}
	}
	// 排除虚拟工具 plan_and_execute
	filteredTools = excludePlanTool(filteredTools)
	log.Printf("[Orchestrator] 子任务 %s 工具: %d 个 (hint=%v)", subtask.ID, len(filteredTools), subtask.ToolsHint)

	maxIter := o.cfg.SubTaskMaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	timeout := time.Duration(o.cfg.SubTaskTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	// 子任务涉及长时间工具时，自动扩展超时
	if hasLongRunningToolHint(subtask.ToolsHint) {
		longTimeout := time.Duration(o.cfg.LongToolTimeoutSec) * time.Second
		if longTimeout <= 0 {
			longTimeout = 600 * time.Second
		}
		// 子任务超时 = 长工具超时 + 额外裕量（LLM 思考 + 多轮迭代）
		if longTimeout+60*time.Second > timeout {
			timeout = longTimeout + 60*time.Second
		}
	}
	deadline := time.Now().Add(timeout)

	var finalText string

	for i := 0; i < maxIter; i++ {
		// 超时检查
		if time.Now().After(deadline) {
			log.Printf("[Orchestrator] ✗ 子任务超时 id=%s duration=%v", subtask.ID, time.Since(subtaskStart))
			sendEvent("subtask_timeout", fmt.Sprintf("[%s] %s — 执行超时 (%s)", subtask.ID, subtask.Title, fmtDuration(time.Since(subtaskStart))))
			session.SetStatus("failed")
			session.SetError("subtask timeout")
			o.store.Save(session)
			return SubTaskResult{
				SubTaskID: subtask.ID,
				Title:     subtask.Title,
				Status:    "failed",
				Error:     "subtask timeout",
			}
		}

		log.Printf("[Orchestrator] subtask=%s 迭代 %d/%d messages=%d", subtask.ID, i+1, maxIter, len(messages))

		// LLM 请求（子任务不需要流式）
		llmStart := time.Now()
		text, toolCalls, err := SendLLMRequest(&o.cfg.LLM, messages, filteredTools)
		llmDuration := time.Since(llmStart)

		if err != nil {
			log.Printf("[Orchestrator] ✗ 子任务 %s LLM失败 duration=%v error=%v", subtask.ID, llmDuration, err)
			sendEvent("subtask_llm_error", fmt.Sprintf("[%s] %s — LLM调用失败: %v", subtask.ID, subtask.Title, err))
			session.SetStatus("failed")
			session.SetError(err.Error())
			o.store.Save(session)
			return SubTaskResult{
				SubTaskID: subtask.ID,
				Title:     subtask.Title,
				Status:    "failed",
				Error:     err.Error(),
			}
		}

		// 记录 assistant 消息
		assistantMsg := Message{
			Role:      "assistant",
			Content:   text,
			ToolCalls: toolCalls,
		}
		session.AppendMessage(assistantMsg)

		// 无工具调用 → 子任务完成
		if len(toolCalls) == 0 {
			log.Printf("[Orchestrator] ✓ 子任务 %s 对话结束（无工具调用） textLen=%d", subtask.ID, len(text))
			finalText = text
			break
		}

		messages = append(messages, assistantMsg)

		// 执行工具调用
		for tcIdx, tc := range toolCalls {
			originalName := unsanitizeToolName(tc.Function.Name)

			sendEvent("tool_call", fmt.Sprintf("[%s] 调用 %s (%d/%d)\n参数: %s", subtask.ID, originalName, tcIdx+1, len(toolCalls), tc.Function.Arguments))
			log.Printf("[Orchestrator] subtask=%s → 调用工具: %s args=%s",
				subtask.ID, originalName, truncate(tc.Function.Arguments, 200))

			start := time.Now()
			result, err := o.bridge.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			duration := time.Since(start)

			success := true
			if err != nil {
				success = false
				result = fmt.Sprintf("工具调用失败: %v", err)
				log.Printf("[Orchestrator] subtask=%s ✗ 工具失败: %s duration=%v error=%v",
					subtask.ID, originalName, duration, err)
				sendEvent("tool_result", fmt.Sprintf("❌ [%s] %s 失败 (%.1fs): %v", subtask.ID, originalName, duration.Seconds(), err))
			} else {
				log.Printf("[Orchestrator] subtask=%s ← 工具返回: %s duration=%v resultLen=%d result=%s",
					subtask.ID, originalName, duration, len(result), truncate(result, 200))
				sendEvent("tool_result", fmt.Sprintf("✅ [%s] %s 完成 (%.1fs)\n结果: %s", subtask.ID, originalName, duration.Seconds(), truncate(result, 300)))
			}

			// 记录工具调用
			session.RecordToolCall(ToolCallRecord{
				ID:         tc.ID,
				ToolName:   originalName,
				Arguments:  tc.Function.Arguments,
				Result:     result,
				Success:    success,
				DurationMs: duration.Milliseconds(),
				Timestamp:  time.Now(),
				Iteration:  i,
			})

			// 追加 tool 消息
			toolMsg := Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			session.AppendMessage(toolMsg)
			messages = append(messages, toolMsg)
		}

		// 最后一次迭代
		if i == maxIter-1 {
			finalText = text
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
		o.store.Save(session)

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

	// 标记完成
	session.SetStatus("done")
	session.SetResult(finalText)
	o.store.Save(session)

	log.Printf("[Orchestrator] ◀ 子任务完成 id=%s duration=%v resultLen=%d",
		subtask.ID, time.Since(subtaskStart), len(finalText))

	return SubTaskResult{
		SubTaskID: subtask.ID,
		Title:     subtask.Title,
		Status:    "done",
		Result:    finalText,
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

	decision, err := MakeFailureDecision(&o.cfg.LLM, *subtask, errorMsg, completedResults)
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
	results []SubTaskResult,
	originalQuery string,
	sendEvent func(event, text string),
) string {
	log.Printf("[Orchestrator] ── 汇总开始 results=%d query=%s", len(results), truncate(originalQuery, 100))
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

	synthesisPrompt := fmt.Sprintf(`请基于以下子任务执行结果，为用户生成一个完整、简洁的回复。

%s

要求：
1. 整合所有子任务的结果为统一的回复
2. 如果有失败或跳过的任务，简要说明
3. 回复应直接面向用户，不要暴露内部的子任务结构
4. 保持简洁，控制在500字以内`, context.String())

	messages := []Message{
		{Role: "user", Content: synthesisPrompt},
	}

	resp, _, err := SendLLMRequest(&o.cfg.LLM, messages, nil)
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

// Resume 断点续传：从持久化数据恢复并继续执行
func (o *Orchestrator) Resume(
	rootSessionID string,
	tools []LLMTool,
	sendEvent func(event, text string),
) (string, error) {
	log.Printf("[Resume] ▶ 开始恢复 rootSessionID=%s", rootSessionID)
	resumeStart := time.Now()
	sendEvent("resume", "正在恢复任务...")

	// 加载会话树
	rootSession, children, err := o.store.LoadTree(rootSessionID)
	if err != nil {
		return "", fmt.Errorf("load session tree: %v", err)
	}
	log.Printf("[Resume] 加载会话树 subtasks=%d children=%d", len(rootSession.ChildIDs), len(children))

	if rootSession.Plan == nil {
		return "", fmt.Errorf("root session has no plan")
	}

	// 分析子任务状态
	completedResults := make(map[string]string)
	var pendingSubTasks []SubTaskPlan

	for _, subtask := range rootSession.Plan.SubTasks {
		child, ok := children[subtask.ID]
		if !ok {
			// 会话丢失，当作 pending
			pendingSubTasks = append(pendingSubTasks, subtask)
			continue
		}

		switch child.Status {
		case "done":
			completedResults[subtask.ID] = child.Result
			sendEvent("resume_info", fmt.Sprintf("[%s] %s — 已完成，跳过", subtask.ID, subtask.Title))

		case "running":
			// 从断点恢复
			sendEvent("resume_info", fmt.Sprintf("[%s] %s — 从断点恢复", subtask.ID, subtask.Title))
			siblingContext := buildSiblingContext(subtask.DependsOn, completedResults)
			result := o.resumeSubTask(child, subtask, siblingContext, tools, sendEvent)
			if result.Status == "done" {
				completedResults[subtask.ID] = result.Result
			}

		case "failed":
			// 检查是否有 retry 决策
			shouldRetry := false
			for _, d := range rootSession.FailureDecisions {
				if d.SubTaskID == subtask.ID && d.Action == "retry" {
					shouldRetry = true
					break
				}
			}
			if shouldRetry {
				pendingSubTasks = append(pendingSubTasks, subtask)
			} else {
				sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前已失败，跳过", subtask.ID, subtask.Title))
			}

		case "pending":
			pendingSubTasks = append(pendingSubTasks, subtask)

		case "skipped":
			sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前已跳过", subtask.ID, subtask.Title))

		case "async":
			// 之前是异步状态，当作 pending 重新评估
			sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前为异步，重新评估", subtask.ID, subtask.Title))
			pendingSubTasks = append(pendingSubTasks, subtask)

		case "deferred":
			// 之前被推迟，当作 pending 重新评估
			sendEvent("resume_info", fmt.Sprintf("[%s] %s — 之前被推迟，重新评估", subtask.ID, subtask.Title))
			pendingSubTasks = append(pendingSubTasks, subtask)
		}
	}

	// 执行剩余的 pending 子任务
	if len(pendingSubTasks) > 0 {
		sendEvent("resume_info", fmt.Sprintf("继续执行 %d 个未完成的子任务", len(pendingSubTasks)))

		// 为 pending 子任务创建/获取 session
		for _, subtask := range pendingSubTasks {
			child, ok := children[subtask.ID]
			if !ok {
				child = NewChildSession(rootSession, subtask.Title, subtask.Description)
				child.ID = subtask.ID // 使用计划中的 ID
				children[subtask.ID] = child
				rootSession.AddChildID(subtask.ID)
			}

			siblingContext := buildSiblingContext(subtask.DependsOn, completedResults)
			taskIdx := indexOf(rootSession.Plan.SubTasks, subtask.ID)
			sendEvent("subtask_start", fmt.Sprintf("[%d/%d] %s", taskIdx+1, len(rootSession.Plan.SubTasks), subtask.Title))

			result := o.executeSubTask("", subtask, child, siblingContext, tools, sendEvent)
			if result.Status == "done" {
				completedResults[subtask.ID] = result.Result
				sendEvent("subtask_done", fmt.Sprintf("[%s] %s — 完成", subtask.ID, subtask.Title))
			}
		}
	}

	// 汇总
	var allResults []SubTaskResult
	for _, subtask := range rootSession.Plan.SubTasks {
		child, ok := children[subtask.ID]
		if !ok {
			continue
		}
		allResults = append(allResults, SubTaskResult{
			SubTaskID: subtask.ID,
			Title:     subtask.Title,
			Status:    child.Status,
			Result:    child.Result,
			Error:     child.Error,
		})
	}

	summary := o.Synthesize(rootSession, allResults, rootSession.Title, sendEvent)

	rootSession.SetStatus("done")
	rootSession.SetResult(summary)
	rootSession.Summary = summary
	o.store.Save(rootSession)

	// 保存索引
	var childList []*TaskSession
	for _, c := range children {
		childList = append(childList, c)
	}
	o.store.SaveIndex(rootSession, childList)

	log.Printf("[Resume] ◀ 恢复完成 duration=%v summaryLen=%d", time.Since(resumeStart), len(summary))
	return summary, nil
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
		return o.executeSubTask("", subtask, session, siblingContext, tools, sendEvent)
	}

	// 检查最后一条消息
	lastMsg := messages[len(messages)-1]
	switch lastMsg.Role {
	case "assistant":
		if len(lastMsg.ToolCalls) > 0 {
			// 工具调用中断：重新执行工具
			for _, tc := range lastMsg.ToolCalls {
				originalName := unsanitizeToolName(tc.Function.Name)
				sendEvent("tool_info", fmt.Sprintf("[%s] 恢复工具调用: %s", subtask.ID, originalName))

				result, err := o.bridge.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
				if err != nil {
					result = fmt.Sprintf("工具调用失败: %v", err)
				}

				toolMsg := Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				}
				session.AppendMessage(toolMsg)
				messages = append(messages, toolMsg)
			}
		} else {
			// 已完成但未标记
			session.SetStatus("done")
			session.SetResult(lastMsg.Content)
			o.store.Save(session)
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

	// 排除虚拟工具
	filteredTools := excludePlanTool(tools)

	// 继续 agentic loop
	maxIter := o.cfg.SubTaskMaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	var finalText string
	for i := 0; i < maxIter; i++ {
		log.Printf("[Resume] subtask=%s 迭代 %d/%d messages=%d", subtask.ID, i+1, maxIter, len(messages))
		text, toolCalls, err := SendLLMRequest(&o.cfg.LLM, messages, filteredTools)
		if err != nil {
			session.SetStatus("failed")
			session.SetError(err.Error())
			o.store.Save(session)
			return SubTaskResult{
				SubTaskID: subtask.ID,
				Title:     subtask.Title,
				Status:    "failed",
				Error:     err.Error(),
			}
		}

		assistantMsg := Message{Role: "assistant", Content: text, ToolCalls: toolCalls}
		session.AppendMessage(assistantMsg)

		if len(toolCalls) == 0 {
			finalText = text
			break
		}

		messages = append(messages, assistantMsg)

		for _, tc := range toolCalls {
			originalName := unsanitizeToolName(tc.Function.Name)
			result, err := o.bridge.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				result = fmt.Sprintf("工具调用失败: %v", err)
			}
			toolMsg := Message{Role: "tool", Content: result, ToolCallID: tc.ID}
			session.AppendMessage(toolMsg)
			messages = append(messages, toolMsg)
		}
	}

	session.SetStatus("done")
	session.SetResult(finalText)
	o.store.Save(session)

	log.Printf("[Resume] ◀ 子任务恢复完成 id=%s duration=%v resultLen=%d",
		subtask.ID, time.Since(resumeStart), len(finalText))

	return SubTaskResult{
		SubTaskID: subtask.ID,
		Title:     subtask.Title,
		Status:    "done",
		Result:    finalText,
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

// filterToolsByHint 根据 tools_hint 过滤工具
func filterToolsByHint(tools []LLMTool, hints []string) []LLMTool {
	hintSet := make(map[string]bool, len(hints))
	for _, h := range hints {
		hintSet[h] = true
		hintSet[sanitizeToolName(h)] = true
	}

	var filtered []LLMTool
	for _, tool := range tools {
		if hintSet[tool.Function.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// excludePlanTool 排除虚拟工具 plan_and_execute
func excludePlanTool(tools []LLMTool) []LLMTool {
	var filtered []LLMTool
	for _, tool := range tools {
		if tool.Function.Name != "plan_and_execute" {
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
