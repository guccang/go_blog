package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
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
	return results
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
	return SendLLMRequestWithFallback(&cfg, o.cfg.Fallbacks, o.fallbackCooldown(), messages, tools)
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
	return SendStreamingLLMRequestWithFallback(&cfg, o.cfg.Fallbacks, o.fallbackCooldown(), messages, tools, onChunk, o.cfg.LLMCallIntervalSec)
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

// Execute 事件驱动 DAG 调度执行所有子任务
func (o *Orchestrator) Execute(
	taskCtx context.Context,
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

	// 创建父 context，从外部 taskCtx 派生，用户停止时级联取消所有子任务
	if taskCtx == nil {
		taskCtx = context.Background()
	}
	parentCtx, parentCancel := context.WithCancel(taskCtx)
	defer parentCancel()

	// 初始化 DAG 调度器
	scheduler := newDAGScheduler(plan)
	completedResults := make(map[string]string)
	var completedResultsMu sync.Mutex

	// 并发控制
	maxP := o.cfg.MaxParallelSubtasks
	if maxP <= 0 {
		maxP = 3
	}
	sem := make(chan struct{}, maxP)

	// 结果收集
	type resultMsg struct {
		result  SubTaskResult
		session *TaskSession
	}
	resultCh := make(chan resultMsg, len(plan.SubTasks))

	var wg sync.WaitGroup
	var eventMu sync.Mutex
	aborted := false

	safeSendEvent := func(event, text string) {
		eventMu.Lock()
		defer eventMu.Unlock()
		sendEvent(event, text)
	}

	// scheduleTask 调度单个子任务执行
	scheduleTask := func(st SubTaskPlan) {
		session, ok := childSessions[st.ID]
		if !ok {
			log.Printf("[Orchestrator] warn: no session for subtask %s", st.ID)
			return
		}

		// 检查是否应跳过
		if skip, reason := scheduler.shouldSkip(st); skip {
			session.SetStatus("skipped")
			session.SetError(reason)
			o.store.Save(session)
			safeSendEvent("subtask_skip", fmt.Sprintf("[%s] %s — 跳过（%s）", st.ID, st.Title, reason))
			resultCh <- resultMsg{
				result: SubTaskResult{
					SubTaskID: st.ID,
					Title:     st.Title,
					Status:    "skipped",
					Error:     reason,
				},
				session: session,
			}
			return
		}

		// 检查是否应延迟
		if scheduler.shouldDefer(st) {
			session.SetStatus("deferred")
			session.SetError("前置任务仍在异步执行中")
			o.store.Save(session)
			safeSendEvent("subtask_defer", fmt.Sprintf("[%s] %s — 等待前置任务完成", st.ID, st.Title))
			resultCh <- resultMsg{
				result: SubTaskResult{
					SubTaskID: st.ID,
					Title:     st.Title,
					Status:    "deferred",
					Error:     "前置任务仍在异步执行中",
				},
				session: session,
			}
			return
		}

		// 构建兄弟结果上下文
		completedResultsMu.Lock()
		siblingContext := buildSiblingContext(st.DependsOn, completedResults)
		completedResultsMu.Unlock()
		taskIdx := indexOf(plan.SubTasks, st.ID)

		sem <- struct{}{}
		wg.Add(1)
		go func(st SubTaskPlan, sess *TaskSession, sibCtx string, tIdx int) {
			defer func() { <-sem; wg.Done() }()

			// 创建 steer channel 并注册句柄
			steerCh := make(chan string, 1)
			o.activeHandlesMu.Lock()
			o.activeHandles[st.ID] = &SubTaskHandle{SubTaskID: st.ID, SteerCh: steerCh}
			o.activeHandlesMu.Unlock()
			defer func() {
				o.activeHandlesMu.Lock()
				delete(o.activeHandles, st.ID)
				o.activeHandlesMu.Unlock()
			}()

			safeSendEvent("subtask_start", fmt.Sprintf("[%d/%d] %s\n描述: %s", tIdx+1, len(plan.SubTasks), st.Title, st.Description))

			result := o.executeSubTask(parentCtx, taskID, st, sess, sibCtx, tools, safeSendEvent, steerCh)

			// 检测异步工具调用
			if result.Status == "done" {
				asyncInfos := detectAsyncResults(sess)
				if len(asyncInfos) > 0 {
					result.Status = "async"
					result.AsyncSessions = asyncInfos
					sess.SetStatus("async")
					o.store.Save(sess)
					var asyncDetails []string
					for _, a := range asyncInfos {
						detail := fmt.Sprintf("%s→%s", a.ToolName, a.SessionID)
						if a.Message != "" {
							detail += ": " + a.Message
						}
						asyncDetails = append(asyncDetails, detail)
					}
					log.Printf("[Orchestrator] async detected: subtask=%s sessions=%v", st.ID, asyncDetails)
					safeSendEvent("subtask_async", fmt.Sprintf("[%d/%d] %s — 异步执行中\n%s",
						tIdx+1, len(plan.SubTasks), st.Title, strings.Join(asyncDetails, "\n")))
				}
			}

			// 处理失败
			if result.Status == "failed" {
				safeSendEvent("subtask_fail", fmt.Sprintf("[%s] %s — 失败: %s", st.ID, st.Title, result.Error))

				decision := o.handleSubTaskFailure(&st, result.Error, completedResults, rootSession, safeSendEvent)

				switch decision.Action {
				case "retry":
					safeSendEvent("retry_detail", fmt.Sprintf("[%s] 重试原因: %s\n原始错误: %s", st.ID, decision.Reason, result.Error))
					safeSendEvent("subtask_start", fmt.Sprintf("[%d/%d] 重试: %s", tIdx+1, len(plan.SubTasks), st.Title))
					result = o.executeSubTask(parentCtx, taskID, st, sess, sibCtx, tools, safeSendEvent, steerCh)

				case "modify":
					modifiedSubtask := st
					modifiedSubtask.Description = decision.Modifications
					safeSendEvent("modify_detail", fmt.Sprintf("[%s] 修改后重试\n原描述: %s\n新描述: %s", st.ID, truncate(st.Description, 200), truncate(decision.Modifications, 200)))
					safeSendEvent("subtask_start", fmt.Sprintf("[%d/%d] 修改后重试: %s", tIdx+1, len(plan.SubTasks), st.Title))
					result = o.executeSubTask(parentCtx, taskID, modifiedSubtask, sess, sibCtx, tools, safeSendEvent, steerCh)

				case "skip":
					result.Status = "skipped"
					sess.SetStatus("skipped")
					o.store.Save(sess)
					safeSendEvent("subtask_skip", fmt.Sprintf("[%s] %s — 已跳过", st.ID, st.Title))

				case "abort":
					result.Status = "failed"
					sess.SetStatus("failed")
					o.store.Save(sess)
					safeSendEvent("subtask_fail", fmt.Sprintf("编排终止: %s", decision.Reason))
					parentCancel() // 级联取消所有运行中的子任务
					aborted = true
				}
			}

			if result.Status == "done" {
				safeSendEvent("subtask_done", fmt.Sprintf("[%d/%d] %s — 完成", tIdx+1, len(plan.SubTasks), st.Title))
				if result.Result != "" {
					safeSendEvent("subtask_result", fmt.Sprintf("[%s] 结果: %s", st.ID, truncate(result.Result, 500)))
				}
			}

			resultCh <- resultMsg{result: result, session: sess}
		}(st, session, siblingContext, taskIdx)
	}

	// 调度初始无依赖任务
	initialTasks := scheduler.getInitialTasks()
	log.Printf("[Orchestrator] DAG scheduler: %d initial tasks, %d total", len(initialTasks), len(plan.SubTasks))
	for _, st := range initialTasks {
		scheduleTask(st)
	}

	// 事件循环：收集结果 + 解锁后续任务 + 动态计划修订
	var allResults []SubTaskResult
	completedCount := 0
	totalTasks := len(plan.SubTasks)
	revisionCount := 0
	maxRevisions := o.cfg.MaxPlanRevisions
	if maxRevisions <= 0 {
		maxRevisions = 3
	}
	// 每完成 revisionCheckInterval 个任务检查一次修订
	revisionCheckInterval := 2
	lastRevisionCheck := 0

	for completedCount < totalTasks && !aborted {
		msg := <-resultCh
		completedCount++
		allResults = append(allResults, msg.result)

		// 更新 completedResults（enriched with key tool data）
		if msg.result.Status == "done" {
			enrichedResult := msg.result.Result
			if msg.session != nil {
				keyData := extractKeyToolData(msg.session)
				if keyData != "" {
					enrichedResult += "\n\n" + keyData
				}
			}
			completedResultsMu.Lock()
			completedResults[msg.result.SubTaskID] = enrichedResult
			completedResultsMu.Unlock()
		}

		// 解锁后续任务
		unblocked := scheduler.markDone(msg.result.SubTaskID, msg.result)
		for _, st := range unblocked {
			if !aborted {
				scheduleTask(st)
			}
		}

		// 动态计划修订检查
		if !aborted && revisionCount < maxRevisions && completedCount-lastRevisionCheck >= revisionCheckInterval {
			// 收集剩余未执行的子任务
			scheduler.mu.Lock()
			var remaining []SubTaskPlan
			for _, st := range plan.SubTasks {
				if !scheduler.completedSet[st.ID] && !scheduler.failedSet[st.ID] && !scheduler.asyncSet[st.ID] {
					remaining = append(remaining, st)
				}
			}
			scheduler.mu.Unlock()

			if len(remaining) > 0 {
				completedResultsMu.Lock()
				crCopy := make(map[string]string, len(completedResults))
				for k, v := range completedResults {
					crCopy[k] = v
				}
				completedResultsMu.Unlock()

			revCfg := o.bridge.activeLLM.Get()
				revResult, err := EvaluateAndRevisePlan(
					&revCfg, rootSession.Title, plan, crCopy, remaining, tools,
					rootSession.Account, o.cfg.Fallbacks, o.fallbackCooldown(),
				)
				lastRevisionCheck = completedCount

				if err == nil && revResult.Action == "revise" && revResult.Plan != nil {
					revisionCount++
					log.Printf("[Orchestrator] 计划修订 #%d: %s", revisionCount, revResult.Reason)
					safeSendEvent("plan_revised", fmt.Sprintf("计划修订 #%d: %s", revisionCount, revResult.Reason))

					// 记录修订
					var added, removed, modified []string
					oldIDs := make(map[string]bool)
					for _, st := range plan.SubTasks {
						oldIDs[st.ID] = true
					}
					newIDs := make(map[string]bool)
					for _, st := range revResult.Plan.SubTasks {
						newIDs[st.ID] = true
						if !oldIDs[st.ID] {
							added = append(added, st.ID)
						} else {
							modified = append(modified, st.ID)
						}
					}
					for id := range oldIDs {
						if !newIDs[id] {
							removed = append(removed, id)
						}
					}

					rootSession.AddPlanRevision(PlanRevision{
						Version:       revisionCount,
						Reason:        revResult.Reason,
						AddedTasks:    added,
						RemovedTasks:  removed,
						ModifiedTasks: modified,
						Timestamp:     time.Now(),
					})

					// 更新计划
					plan = revResult.Plan
					rootSession.Plan = plan
					o.store.Save(rootSession)

					// 为新增子任务创建 session
					for _, st := range plan.SubTasks {
						if _, exists := childSessions[st.ID]; !exists {
							child := NewChildSession(rootSession, st.Title, st.Description)
							child.ID = st.ID
							childSessions[st.ID] = child
							rootSession.AddChildID(st.ID)
							o.store.Save(child)
						}
					}

					// 更新调度器的计划并重新计算总任务数
					scheduler.mu.Lock()
					scheduler.plan = plan
					scheduler.mu.Unlock()
					totalTasks = len(plan.SubTasks)

					// 调度新解锁的任务
					scheduler.mu.Lock()
					for _, st := range plan.SubTasks {
						if !scheduler.scheduledSet[st.ID] && !scheduler.completedSet[st.ID] && !scheduler.failedSet[st.ID] && !scheduler.asyncSet[st.ID] {
							if scheduler.allDepsResolved(st) {
								scheduler.scheduledSet[st.ID] = true
								scheduler.mu.Unlock()
								scheduleTask(st)
								scheduler.mu.Lock()
							}
						}
					}
					scheduler.mu.Unlock()
				}
			}
		}

		safeSendEvent("progress", fmt.Sprintf("[%d/%d 子任务已处理]", completedCount, totalTasks))
	}

	// 等待所有已调度的 goroutine 完成
	wg.Wait()

	// 收集可能在 wg.Wait 期间到达的剩余结果
	close(resultCh)
	for msg := range resultCh {
		allResults = append(allResults, msg.result)
	}

	return allResults
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

	// 构建子任务的 system prompt
	var systemContent strings.Builder
	subtaskPrompt := loadWorkspaceFile(o.cfg.WorkspaceDir, "SUBTASK.md", defaultSubtaskPrompt)
	systemContent.WriteString(subtaskPrompt)
	systemContent.WriteString("\n\n")
	systemContent.WriteString(fmt.Sprintf("当前用户账号: %s\n", session.Account))
	systemContent.WriteString(fmt.Sprintf("当前日期: %s\n", time.Now().Format("2006-01-02")))
	systemContent.WriteString(fmt.Sprintf("当前输出token预算: %d tokens。\n\n", o.bridge.activeLLM.Get().MaxTokens))

	// 注入 agent 能力描述（可用模型/编码工具），让子任务 LLM 知道工具参数的合法值
	agentBlock := o.bridge.getAgentDescriptionBlock()
	if agentBlock != "" {
		systemContent.WriteString(agentBlock)
		systemContent.WriteString("\n")
	}

	systemContent.WriteString(fmt.Sprintf("## 子任务: %s\n", subtask.Title))
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
		filteredTools = o.bridge.ApplySubtaskPolicy(tools, subtask.ToolsHint)
	}
	// 排除虚拟工具 plan_and_execute
	filteredTools = excludeVirtualTools(filteredTools)
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

	var finalText string

	for i := 0; i < maxIter; i++ {
		// 级联取消检查
		if ctx.Err() != nil {
			log.Printf("[Orchestrator] ✗ 子任务取消 id=%s reason=%v", subtask.ID, ctx.Err())
			session.SetStatus("cancelled")
			session.SetError("cancelled")
			o.store.Save(session)
			return SubTaskResult{
				SubTaskID: subtask.ID,
				Title:     subtask.Title,
				Status:    "failed",
				Error:     "cancelled",
			}
		}

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

		// 子任务消息压缩（复用 sanitizeProcessMessages，子任务共享相同预算策略）
		if len(messages) > 15 || estimateChars(messages) > 120000 {
			before := len(messages)
			messages = sanitizeProcessMessages(messages)
			if len(messages) != before {
				log.Printf("[Orchestrator] subtask=%s 消息压缩: %d → %d", subtask.ID, before, len(messages))
			}
		}

		log.Printf("[Orchestrator] subtask=%s 迭代 %d/%d messages=%d", subtask.ID, i+1, maxIter, len(messages))

		// 非阻塞检查 steer 消息
		if steerCh != nil {
			select {
			case steerMsg := <-steerCh:
				steerContent := fmt.Sprintf("[编排器指令] %s", steerMsg)
				messages = append(messages, Message{Role: "system", Content: steerContent})
				session.AppendMessage(Message{Role: "system", Content: steerContent})
				sendEvent("subtask_steer", fmt.Sprintf("[%s] 收到编排器指令: %s", subtask.ID, steerMsg))
				log.Printf("[Orchestrator] subtask=%s steer injected: %s", subtask.ID, steerMsg)
			default:
			}
		}

		// LLM 请求（子任务不需要流式，带降级链 + context）
		llmStart := time.Now()
		text, toolCalls, err := o.sendLLMCtx(ctx, messages, filteredTools)
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

		// 本轮业务失败记录（用于循环后扩展兄弟工具）
		var bizFailedTools []string
		var bizFailedMsgs []string

		// 执行工具调用
		for tcIdx, tc := range toolCalls {
			originalName := o.bridge.resolveToolName(tc.Function.Name)

			sendEvent("tool_call", fmt.Sprintf("[%s] 调用 %s (%d/%d)\n参数: %s", subtask.ID, originalName, tcIdx+1, len(toolCalls), tc.Function.Arguments))
			log.Printf("[Orchestrator] subtask=%s → 调用工具: %s args=%s",
				subtask.ID, originalName, tc.Function.Arguments)

			start := time.Now()
			tcResult, err := o.bridge.CallToolCtx(ctx, originalName, json.RawMessage(tc.Function.Arguments))
			duration := time.Since(start)

			// 动态扩展截止时间：实际调用了长时间工具时，确保后续迭代不会误判超时
			if isLongRunningTool(originalName) {
				newDeadline := time.Now().Add(longTimeout + 60*time.Second)
				if newDeadline.After(deadline) {
					log.Printf("[Orchestrator] subtask=%s 长工具 %s 耗时 %v，扩展截止时间 +%v",
						subtask.ID, originalName, duration, longTimeout+60*time.Second)
					deadline = newDeadline
				}
			}

			var result string
			var toAgent, fromAgent string
			if tcResult != nil {
				result = tcResult.Result
				toAgent = tcResult.AgentID
				fromAgent = tcResult.FromID
			}

			success := true
			if err != nil {
				success = false
				result = fmt.Sprintf("工具调用失败: %v", err)
				log.Printf("[Orchestrator] subtask=%s ✗ 工具失败: %s →agent=%s duration=%v error=%v",
					subtask.ID, originalName, toAgent, duration, err)
				sendEvent("tool_result", fmt.Sprintf("❌ [%s] %s 失败 →%s (%.1fs): %v", subtask.ID, originalName, toAgent, duration.Seconds(), err))
			} else {
				log.Printf("[Orchestrator] subtask=%s ← 工具返回: %s →agent=%s ←from=%s duration=%v resultLen=%d result=%s",
					subtask.ID, originalName, toAgent, fromAgent, duration, len(result), result)
				sendEvent("tool_result", fmt.Sprintf("✅ [%s] %s [%s→%s] (%.1fs)\n结果: %s", subtask.ID, originalName, toAgent, fromAgent, duration.Seconds(), truncate(result, 300)))
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

			// 检测业务失败（transport 成功但 result JSON 中 success:false）
			if err == nil && result != "" {
				var bizResult struct {
					Success bool   `json:"success"`
					Error   string `json:"error"`
				}
				if json.Unmarshal([]byte(result), &bizResult) == nil && !bizResult.Success && bizResult.Error != "" {
					bizFailedTools = append(bizFailedTools, originalName)
					bizFailedMsgs = append(bizFailedMsgs, bizResult.Error)
					log.Printf("[Orchestrator] subtask=%s 业务失败检测: %s → %s", subtask.ID, originalName, bizResult.Error)
				}
			}

			// ExecuteCode 特殊处理：只取 stdout，避免结构化 JSON 污染 context
			if originalName == "ExecuteCode" && result != "" {
				stdout, _ := parseExecuteCodeResult(result)
				if stdout != "" {
					result = stdout
				}
			}

			// 追加 tool 消息（截断防止 context 膨胀）
			toolMsg := Message{
				Role:       "tool",
				Content:    truncateToolResult(result, i),
				ToolCallID: tc.ID,
			}
			session.AppendMessage(toolMsg)
			messages = append(messages, toolMsg)
		}

		// 工具业务失败 → 扩展同 agent 兄弟工具，让 LLM 自行决策修复参数或切换工具
		if len(bizFailedTools) > 0 {
			existingSet := make(map[string]bool, len(filteredTools))
			for _, t := range filteredTools {
				existingSet[t.Function.Name] = true
			}
			var newToolNames []string
			for _, failedTool := range bizFailedTools {
				siblings := o.bridge.getSiblingTools(failedTool)
				for _, s := range siblings {
					if !existingSet[s.Function.Name] {
						existingSet[s.Function.Name] = true
						filteredTools = append(filteredTools, s)
						newToolNames = append(newToolNames, o.bridge.resolveToolName(s.Function.Name))
					}
				}
			}
			if len(newToolNames) > 0 {
				var failInfo strings.Builder
				for idx, name := range bizFailedTools {
					failInfo.WriteString(fmt.Sprintf("- %s: %s\n", name, bizFailedMsgs[idx]))
				}
				hint := fmt.Sprintf("以下工具返回业务失败:\n%s已补充同 Agent 的替代工具: %s\n你可以选择修复参数重试原工具，或使用替代工具完成任务。",
					failInfo.String(), strings.Join(newToolNames, ", "))
				messages = append(messages, Message{Role: "system", Content: hint})
				log.Printf("[Orchestrator] subtask=%s 业务失败扩展: 新增 %d 个兄弟工具: %v", subtask.ID, len(newToolNames), newToolNames)
				sendEvent("tool_expand", fmt.Sprintf("[%s] 工具业务失败，补充兄弟工具: %s", subtask.ID, strings.Join(newToolNames, ", ")))
			}
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

	// 检测关键工具的业务级失败：tool 调用成功（网络层）但返回 JSON 中 success=false（业务层）
	// 按时间倒序找到每个工具名的最后一次调用，若最后一次仍 success:false 则判定失败
	var criticalFailure string
	if len(toolCallRecords) > 0 {
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
		o.store.Save(session)

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

			result := o.executeSubTask(context.Background(), "", subtask, child, siblingContext, tools, sendEvent, nil)
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

	summary := o.Synthesize(rootSession, children, allResults, rootSession.Title, sendEvent)

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
	filteredTools := excludeVirtualTools(tools)

	// 继续 agentic loop
	maxIter := o.cfg.SubTaskMaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	var finalText string
	for i := 0; i < maxIter; i++ {
		log.Printf("[Resume] subtask=%s 迭代 %d/%d messages=%d", subtask.ID, i+1, maxIter, len(messages))
		text, toolCalls, err := o.sendLLM(messages, filteredTools)
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
			originalName := o.bridge.resolveToolName(tc.Function.Name)
			tcResult, err := o.bridge.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			var result string
			if tcResult != nil {
				result = tcResult.Result
				log.Printf("[Resume] tool_call: %s →agent=%s ←from=%s", originalName, tcResult.AgentID, tcResult.FromID)
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

			toolMsg := Message{Role: "tool", Content: truncateToolResult(result, i), ToolCallID: tc.ID}
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

// excludeVirtualTools 排除虚拟工具（plan_and_execute, execute_skill 等）
// 子任务不应调用这些虚拟工具，应直接使用实际工具
func excludeVirtualTools(tools []LLMTool) []LLMTool {
	var filtered []LLMTool
	for _, tool := range tools {
		switch tool.Function.Name {
		case "plan_and_execute", "execute_skill", "get_skill_detail":
			// 跳过虚拟工具
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
