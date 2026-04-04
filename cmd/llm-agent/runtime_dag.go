package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type subtaskResultMsg struct {
	result  SubTaskResult
	session *TaskSession
}

type DAGExecutionRuntime struct {
	orchestrator  *Orchestrator
	taskID        string
	rootSession   *TaskSession
	childSessions map[string]*TaskSession
	tools         []LLMTool
	sendEvent     func(event, text string)

	plan       *TaskPlan
	startTime  time.Time
	planLogger *PlanLogger

	parentCtx    context.Context
	parentCancel context.CancelFunc

	scheduler          *dagScheduler
	completedResults   map[string]string
	completedResultsMu sync.Mutex

	sem      chan struct{}
	resultCh chan subtaskResultMsg
	wg       sync.WaitGroup
	eventMu  sync.Mutex

	aborted           bool
	allResults        []SubTaskResult
	completedCount    int
	totalTasks        int
	revisionCount     int
	maxRevisions      int
	revisionCheckStep int
	lastRevisionCheck int
}

func (o *Orchestrator) Execute(
	taskCtx context.Context,
	taskID string,
	rootSession *TaskSession,
	childSessions map[string]*TaskSession,
	tools []LLMTool,
	sendEvent func(event, text string),
) []SubTaskResult {
	rt := newDAGExecutionRuntime(o, taskCtx, taskID, rootSession, childSessions, tools, sendEvent)
	return rt.Run()
}

func newDAGExecutionRuntime(
	o *Orchestrator,
	taskCtx context.Context,
	taskID string,
	rootSession *TaskSession,
	childSessions map[string]*TaskSession,
	tools []LLMTool,
	sendEvent func(event, text string),
) *DAGExecutionRuntime {
	plan := rootSession.Plan
	if taskCtx == nil {
		taskCtx = context.Background()
	}
	parentCtx, parentCancel := context.WithCancel(taskCtx)

	maxP := o.cfg.MaxParallelSubtasks
	if maxP <= 0 {
		maxP = 3
	}
	maxRevisions := o.cfg.MaxPlanRevisions
	if maxRevisions <= 0 {
		maxRevisions = 3
	}

	return &DAGExecutionRuntime{
		orchestrator:      o,
		taskID:            taskID,
		rootSession:       rootSession,
		childSessions:     childSessions,
		tools:             tools,
		sendEvent:         sendEvent,
		plan:              plan,
		startTime:         time.Now(),
		parentCtx:         parentCtx,
		parentCancel:      parentCancel,
		scheduler:         newDAGScheduler(plan),
		completedResults:  make(map[string]string),
		sem:               make(chan struct{}, maxP),
		resultCh:          make(chan subtaskResultMsg, len(plan.SubTasks)),
		totalTasks:        len(plan.SubTasks),
		maxRevisions:      maxRevisions,
		revisionCheckStep: 2,
	}
}

func (rt *DAGExecutionRuntime) Run() []SubTaskResult {
	if rt.plan == nil || len(rt.plan.SubTasks) == 0 {
		return nil
	}
	defer rt.parentCancel()

	rt.initLogger()
	defer rt.closeLogger()

	initialTasks := rt.scheduler.getInitialTasks()
	log.Printf("[Orchestrator] DAG scheduler: %d initial tasks, %d total", len(initialTasks), len(rt.plan.SubTasks))
	for _, st := range initialTasks {
		rt.scheduleTask(st)
	}

	rt.collectResults()
	rt.waitAndDrain()
	rt.logEnd()
	return rt.allResults
}

func (rt *DAGExecutionRuntime) initLogger() {
	planLogger, err := CreatePlanLogger(rt.taskID)
	if err != nil {
		log.Printf("[Orchestrator] failed to create plan logger: %v", err)
		return
	}
	rt.planLogger = planLogger
	rt.planLogger.LogStart(rt.rootSession.Title, rt.rootSession.Account, rt.plan)
}

func (rt *DAGExecutionRuntime) closeLogger() {
	if rt.planLogger != nil {
		rt.planLogger.Close()
	}
}

func (rt *DAGExecutionRuntime) logEnd() {
	if rt.planLogger != nil {
		rt.planLogger.LogEnd(rt.allResults, time.Since(rt.startTime))
	}
}

func (rt *DAGExecutionRuntime) safeSendEvent(event, text string) {
	rt.eventMu.Lock()
	defer rt.eventMu.Unlock()
	rt.sendEvent(event, text)
}

func (rt *DAGExecutionRuntime) scheduleTask(st SubTaskPlan) {
	session, ok := rt.childSessions[st.ID]
	if !ok {
		log.Printf("[Orchestrator] warn: no session for subtask %s", st.ID)
		return
	}

	if skip, reason := rt.scheduler.shouldSkip(st); skip {
		session.SetStatus("skipped")
		session.SetError(reason)
		rt.orchestrator.saveSession(session)
		rt.safeSendEvent("subtask_skip", fmt.Sprintf("[%s] %s — 跳过（%s）", st.ID, st.Title, reason))
		rt.resultCh <- subtaskResultMsg{
			result:  SubTaskResult{SubTaskID: st.ID, Title: st.Title, Status: "skipped", Error: reason},
			session: session,
		}
		return
	}

	if rt.scheduler.shouldDefer(st) {
		session.SetStatus("deferred")
		session.SetError("前置任务仍在异步执行中")
		rt.orchestrator.saveSession(session)
		rt.safeSendEvent("subtask_defer", fmt.Sprintf("[%s] %s — 等待前置任务完成", st.ID, st.Title))
		rt.resultCh <- subtaskResultMsg{
			result:  SubTaskResult{SubTaskID: st.ID, Title: st.Title, Status: "deferred", Error: "前置任务仍在异步执行中"},
			session: session,
		}
		return
	}

	rt.completedResultsMu.Lock()
	siblingContext := buildSiblingContext(st.DependsOn, rt.completedResults)
	rt.completedResultsMu.Unlock()
	taskIdx := indexOf(rt.plan.SubTasks, st.ID)

	rt.sem <- struct{}{}
	rt.wg.Add(1)
	go func(st SubTaskPlan, sess *TaskSession, sibCtx string, tIdx int) {
		defer func() { <-rt.sem; rt.wg.Done() }()
		rt.resultCh <- rt.runScheduledTask(st, sess, sibCtx, tIdx)
	}(st, session, siblingContext, taskIdx)
}

func (rt *DAGExecutionRuntime) runScheduledTask(st SubTaskPlan, sess *TaskSession, sibCtx string, taskIdx int) subtaskResultMsg {
	steerCh := make(chan string, 1)
	rt.orchestrator.activeHandlesMu.Lock()
	rt.orchestrator.activeHandles[st.ID] = &SubTaskHandle{SubTaskID: st.ID, SteerCh: steerCh}
	rt.orchestrator.activeHandlesMu.Unlock()
	defer func() {
		rt.orchestrator.activeHandlesMu.Lock()
		delete(rt.orchestrator.activeHandles, st.ID)
		rt.orchestrator.activeHandlesMu.Unlock()
	}()

	rt.safeSendEvent("subtask_start", fmt.Sprintf("[%d/%d] %s\n描述: %s", taskIdx+1, len(rt.plan.SubTasks), st.Title, st.Description))
	subTaskStart := time.Now()
	if rt.planLogger != nil {
		rt.planLogger.LogSubTaskStart(st.ID, st.Title)
	}

	result := rt.orchestrator.executeSubTask(rt.parentCtx, rt.taskID, st, sess, sibCtx, rt.tools, rt.safeSendEvent, steerCh)
	if rt.planLogger != nil {
		rt.planLogger.LogSubTaskEnd(st.ID, result.Status, result.Result, time.Since(subTaskStart))
	}

	result = rt.detectAsync(st, sess, taskIdx, result)
	result = rt.handleFailure(st, sess, sibCtx, taskIdx, steerCh, result)

	if result.Status == "done" {
		rt.safeSendEvent("subtask_done", fmt.Sprintf("[%d/%d] %s — 完成", taskIdx+1, len(rt.plan.SubTasks), st.Title))
		if result.Result != "" {
			rt.safeSendEvent("subtask_result", fmt.Sprintf("[%s] 结果: %s", st.ID, truncate(result.Result, 500)))
		}
	}

	return subtaskResultMsg{result: result, session: sess}
}

func (rt *DAGExecutionRuntime) detectAsync(st SubTaskPlan, sess *TaskSession, taskIdx int, result SubTaskResult) SubTaskResult {
	if result.Status != "done" {
		return result
	}
	asyncInfos := detectAsyncResults(sess)
	if len(asyncInfos) == 0 {
		return result
	}
	result.Status = "async"
	result.AsyncSessions = asyncInfos
	sess.SetStatus("async")
	rt.orchestrator.saveSession(sess)
	var asyncDetails []string
	for _, a := range asyncInfos {
		detail := fmt.Sprintf("%s→%s", a.ToolName, a.SessionID)
		if a.Message != "" {
			detail += ": " + a.Message
		}
		asyncDetails = append(asyncDetails, detail)
	}
	log.Printf("[Orchestrator] async detected: subtask=%s sessions=%v", st.ID, asyncDetails)
	rt.safeSendEvent("subtask_async", fmt.Sprintf("[%d/%d] %s — 异步执行中\n%s",
		taskIdx+1, len(rt.plan.SubTasks), st.Title, strings.Join(asyncDetails, "\n")))
	return result
}

func (rt *DAGExecutionRuntime) handleFailure(
	st SubTaskPlan,
	sess *TaskSession,
	sibCtx string,
	taskIdx int,
	steerCh <-chan string,
	result SubTaskResult,
) SubTaskResult {
	if result.Status != "failed" {
		return result
	}

	rt.safeSendEvent("subtask_fail", fmt.Sprintf("[%s] %s — 失败: %s", st.ID, st.Title, result.Error))
	decision := rt.orchestrator.handleSubTaskFailure(&st, result.Error, rt.completedResults, rt.rootSession, rt.safeSendEvent)

	switch decision.Action {
	case "retry":
		rt.safeSendEvent("retry_detail", fmt.Sprintf("[%s] 重试原因: %s\n原始错误: %s", st.ID, decision.Reason, result.Error))
		rt.safeSendEvent("subtask_start", fmt.Sprintf("[%d/%d] 重试: %s", taskIdx+1, len(rt.plan.SubTasks), st.Title))
		return rt.orchestrator.executeSubTask(rt.parentCtx, rt.taskID, st, sess, sibCtx, rt.tools, rt.safeSendEvent, steerCh)
	case "modify":
		modifiedSubtask := st
		modifiedSubtask.Description = decision.Modifications
		rt.safeSendEvent("modify_detail", fmt.Sprintf("[%s] 修改后重试\n原描述: %s\n新描述: %s", st.ID, truncate(st.Description, 200), truncate(decision.Modifications, 200)))
		rt.safeSendEvent("subtask_start", fmt.Sprintf("[%d/%d] 修改后重试: %s", taskIdx+1, len(rt.plan.SubTasks), st.Title))
		return rt.orchestrator.executeSubTask(rt.parentCtx, rt.taskID, modifiedSubtask, sess, sibCtx, rt.tools, rt.safeSendEvent, steerCh)
	case "skip":
		result.Status = "skipped"
		sess.SetStatus("skipped")
		rt.orchestrator.saveSession(sess)
		rt.safeSendEvent("subtask_skip", fmt.Sprintf("[%s] %s — 已跳过", st.ID, st.Title))
	case "abort":
		result.Status = "failed"
		sess.SetStatus("failed")
		rt.orchestrator.saveSession(sess)
		rt.safeSendEvent("subtask_fail", fmt.Sprintf("编排终止: %s", decision.Reason))
		rt.parentCancel()
		rt.aborted = true
	}
	return result
}

func (rt *DAGExecutionRuntime) collectResults() {
	for rt.completedCount < rt.totalTasks && !rt.aborted {
		msg := <-rt.resultCh
		rt.completedCount++
		rt.allResults = append(rt.allResults, msg.result)
		rt.updateCompletedResults(msg)
		rt.unblockTasks(msg.result)
		rt.maybeRevisePlan()
		rt.safeSendEvent("progress", fmt.Sprintf("[%d/%d 子任务已处理]", rt.completedCount, rt.totalTasks))
	}
}

func (rt *DAGExecutionRuntime) updateCompletedResults(msg subtaskResultMsg) {
	if msg.result.Status != "done" {
		return
	}
	enrichedResult := msg.result.Result
	if msg.session != nil {
		keyData := extractKeyToolData(msg.session)
		if keyData != "" {
			enrichedResult += "\n\n" + keyData
		}
	}
	rt.completedResultsMu.Lock()
	rt.completedResults[msg.result.SubTaskID] = enrichedResult
	rt.completedResultsMu.Unlock()
}

func (rt *DAGExecutionRuntime) unblockTasks(result SubTaskResult) {
	unblocked := rt.scheduler.markDone(result.SubTaskID, result)
	for _, st := range unblocked {
		if !rt.aborted {
			rt.scheduleTask(st)
		}
	}
}

func (rt *DAGExecutionRuntime) maybeRevisePlan() {
	if rt.aborted || rt.revisionCount >= rt.maxRevisions || rt.completedCount-rt.lastRevisionCheck < rt.revisionCheckStep {
		return
	}

	remaining := rt.remainingSubTasks()
	if len(remaining) == 0 {
		return
	}

	crCopy := rt.copyCompletedResults()
	revCfg := rt.orchestrator.bridge.activeLLM.Get()
	revResult, err := EvaluateAndRevisePlan(
		&revCfg, rt.rootSession.Title, rt.plan, crCopy, remaining, rt.tools,
		rt.rootSession.Account, rt.orchestrator.cfg.Fallbacks, rt.orchestrator.fallbackCooldown(),
	)
	rt.lastRevisionCheck = rt.completedCount
	if err != nil || revResult.Action != "revise" || revResult.Plan == nil {
		return
	}

	rt.revisionCount++
	log.Printf("[Orchestrator] 计划修订 #%d: %s", rt.revisionCount, revResult.Reason)
	rt.safeSendEvent("plan_revised", fmt.Sprintf("计划修订 #%d: %s", rt.revisionCount, revResult.Reason))
	rt.applyPlanRevision(revResult)
}

func (rt *DAGExecutionRuntime) remainingSubTasks() []SubTaskPlan {
	rt.scheduler.mu.Lock()
	defer rt.scheduler.mu.Unlock()
	var remaining []SubTaskPlan
	for _, st := range rt.plan.SubTasks {
		if !rt.scheduler.completedSet[st.ID] && !rt.scheduler.failedSet[st.ID] && !rt.scheduler.asyncSet[st.ID] {
			remaining = append(remaining, st)
		}
	}
	return remaining
}

func (rt *DAGExecutionRuntime) copyCompletedResults() map[string]string {
	rt.completedResultsMu.Lock()
	defer rt.completedResultsMu.Unlock()
	crCopy := make(map[string]string, len(rt.completedResults))
	for k, v := range rt.completedResults {
		crCopy[k] = v
	}
	return crCopy
}

func (rt *DAGExecutionRuntime) applyPlanRevision(revResult *PlanRevisionResult) {
	var added, removed, modified []string
	oldIDs := make(map[string]bool)
	for _, st := range rt.plan.SubTasks {
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

	rt.rootSession.AddPlanRevision(PlanRevision{
		Version:       rt.revisionCount,
		Reason:        revResult.Reason,
		AddedTasks:    added,
		RemovedTasks:  removed,
		ModifiedTasks: modified,
		Timestamp:     time.Now(),
	})

	rt.plan = revResult.Plan
	rt.rootSession.Plan = rt.plan
	rt.orchestrator.saveSession(rt.rootSession)

	for _, st := range rt.plan.SubTasks {
		if _, exists := rt.childSessions[st.ID]; !exists {
			child := NewChildSession(rt.rootSession, st.Title, st.Description)
			child.ID = st.ID
			rt.childSessions[st.ID] = child
			rt.rootSession.AddChildID(st.ID)
			rt.orchestrator.saveSession(child)
		}
	}

	rt.scheduler.mu.Lock()
	rt.scheduler.plan = rt.plan
	rt.scheduler.mu.Unlock()
	rt.totalTasks = len(rt.plan.SubTasks)
	rt.scheduleUnlockedTasks()
}

func (rt *DAGExecutionRuntime) scheduleUnlockedTasks() {
	rt.scheduler.mu.Lock()
	defer rt.scheduler.mu.Unlock()
	for _, st := range rt.plan.SubTasks {
		if !rt.scheduler.scheduledSet[st.ID] && !rt.scheduler.completedSet[st.ID] && !rt.scheduler.failedSet[st.ID] && !rt.scheduler.asyncSet[st.ID] {
			if rt.scheduler.allDepsResolved(st) {
				rt.scheduler.scheduledSet[st.ID] = true
				go rt.scheduleTask(st)
			}
		}
	}
}

func (rt *DAGExecutionRuntime) waitAndDrain() {
	rt.wg.Wait()
	close(rt.resultCh)
	for msg := range rt.resultCh {
		rt.allResults = append(rt.allResults, msg.result)
	}
}
