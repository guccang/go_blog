package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

type ComplexTaskRuntime struct {
	bridge      *Bridge
	task        *TaskContext
	rootSession *TaskSession
	store       *SessionStore
	tools       []LLMTool
	completed   string
	sendEvent   func(event, text string)
	start       time.Time
	query       string
	plan        *TaskPlan
}

func (b *Bridge) handleComplexTask(
	ctx *TaskContext,
	rootSession *TaskSession,
	store *SessionStore,
	tools []LLMTool,
	completedWork string,
) string {
	rt := &ComplexTaskRuntime{
		bridge:      b,
		task:        ctx,
		rootSession: rootSession,
		store:       store,
		tools:       tools,
		completed:   completedWork,
		sendEvent: func(event, text string) {
			ctx.Sink.OnEvent(event, text)
		},
		start: time.Now(),
		query: ctx.Query,
	}
	if rt.query == "" {
		rt.query = rootSession.Title
	}
	return rt.Run()
}

func (rt *ComplexTaskRuntime) Run() string {
	log.Printf("[ComplexTask] ▶ 开始复杂任务处理 taskID=%s query=%s", rt.task.TaskID, truncate(rt.query, 100))

	if rt.isCancelled("规划前") {
		return "任务已停止。"
	}

	rt.sendEvent("plan_start", "正在分析任务...")
	if err := rt.planTask(); err != nil {
		log.Printf("[ComplexTask] ✗ 任务规划失败 error=%v", err)
		rt.sendEvent("plan_done", fmt.Sprintf("任务规划失败: %v", err))
		rt.rootSession.SetStatus("failed")
		rt.rootSession.SetError(err.Error())
		rt.store.Save(rt.rootSession)
		rt.store.SaveIndex(rt.rootSession, nil)
		return fmt.Sprintf("抱歉，任务规划失败: %v", err)
	}

	if rt.isCancelled("审查前") {
		return "任务已停止。"
	}

	rt.reviewPlan()
	childSessions := rt.createChildSessions()
	results, cancelled := rt.executePlan(childSessions)
	if cancelled {
		return "任务已停止。"
	}

	if summary, ok := rt.finishAsync(results, childSessions); ok {
		return summary
	}

	return rt.synthesize(results, childSessions)
}

func (rt *ComplexTaskRuntime) isCancelled(stage string) bool {
	if rt.task.Ctx != nil && rt.task.Ctx.Err() != nil {
		log.Printf("[ComplexTask] ✗ %s任务被取消 taskID=%s", stage, rt.task.TaskID)
		rt.rootSession.SetStatus("cancelled")
		rt.store.Save(rt.rootSession)
		rt.store.SaveIndex(rt.rootSession, nil)
		return true
	}
	return false
}

func (rt *ComplexTaskRuntime) planTask() error {
	maxSubTasks := rt.bridge.cfg.MaxSubTasks
	if maxSubTasks <= 0 {
		maxSubTasks = 10
	}

	var skillBlock string
	if rt.bridge.skillMgr != nil {
		availableSkills := rt.bridge.skillMgr.GetAvailableSkills()
		if len(availableSkills) > 0 {
			skillBlock = rt.bridge.skillMgr.BuildSkillBlock(availableSkills)
		}
	}

	planStart := time.Now()
	activeCfg := rt.bridge.activeLLM.Get()
	plan, err := PlanTask(&activeCfg, rt.query, rt.tools, rt.task.Account, maxSubTasks, rt.completed, skillBlock, rt.bridge.cfg.Fallbacks, rt.bridge.fallbackCooldown())
	planDuration := time.Since(planStart)
	if err != nil {
		return err
	}

	rt.plan = plan
	rt.rootSession.Plan = plan
	rt.store.Save(rt.rootSession)
	rt.bridge.hooks.FirePlanCreated(rt.task, plan)

	log.Printf("[ComplexTask] ✓ 任务规划完成 duration=%v subtasks=%d mode=%s", planDuration, len(plan.SubTasks), plan.ExecutionMode)
	for i, st := range plan.SubTasks {
		log.Printf("[ComplexTask]   子任务[%d] id=%s title=%s depends=%v tools_hint=%v",
			i+1, st.ID, st.Title, st.DependsOn, st.ToolsHint)
	}

	rt.sendEvent("plan_timing", fmt.Sprintf("任务规划完成，耗时 %s，拆解为 %d 个子任务", fmtDuration(planDuration), len(plan.SubTasks)))
	rt.sendEvent("plan_done", rt.buildPlanSummary(plan.SubTasks))
	rt.sendPlanDetails(plan.SubTasks)
	return nil
}

func (rt *ComplexTaskRuntime) reviewPlan() {
	rt.sendEvent("plan_review_start", "正在审查计划参数...")
	agentCapabilities := rt.bridge.getAgentDescriptionBlock()
	reviewStart := time.Now()
	reviewCfg := rt.bridge.activeLLM.Get()
	review, err := ReviewPlan(&reviewCfg, rt.query, rt.plan, rt.tools, rt.task.Account, agentCapabilities, rt.bridge.cfg.Fallbacks, rt.bridge.fallbackCooldown())
	reviewDuration := time.Since(reviewStart)

	if err != nil {
		log.Printf("[ComplexTask] ⚠ 计划审查失败 error=%v，继续执行原计划", err)
		rt.sendEvent("plan_review_result", fmt.Sprintf("计划审查跳过: %v", err))
		rt.sendEvent("review_timing", fmt.Sprintf("审查完成，耗时 %s", fmtDuration(reviewDuration)))
		return
	}

	if review.Action == "optimize" && review.Plan != nil {
		log.Printf("[ComplexTask] 计划已优化: %s", review.Reason)
		rt.plan = review.Plan
		rt.rootSession.Plan = rt.plan
		rt.store.Save(rt.rootSession)
		rt.sendEvent("plan_review_result", fmt.Sprintf("计划已优化: %s", review.Reason))
		rt.sendEvent("plan_done", rt.buildPlanSummary(rt.plan.SubTasks))
		rt.sendPlanDetails(rt.plan.SubTasks)
	} else {
		reason := "审查通过"
		if review != nil && review.Reason != "" {
			reason = review.Reason
		}
		log.Printf("[ComplexTask] 计划审查通过: %s", reason)
		rt.sendEvent("plan_review_result", fmt.Sprintf("计划审查通过: %s", reason))
	}

	rt.sendEvent("review_timing", fmt.Sprintf("审查完成，耗时 %s", fmtDuration(reviewDuration)))
}

func (rt *ComplexTaskRuntime) createChildSessions() map[string]*TaskSession {
	childSessions := make(map[string]*TaskSession)
	for _, st := range rt.plan.SubTasks {
		child := NewChildSession(rt.rootSession, st.Title, st.Description)
		child.ID = st.ID
		childSessions[st.ID] = child
		rt.rootSession.AddChildID(st.ID)
		rt.store.Save(child)
	}
	rt.store.Save(rt.rootSession)
	return childSessions
}

func (rt *ComplexTaskRuntime) executePlan(childSessions map[string]*TaskSession) ([]SubTaskResult, bool) {
	log.Printf("[ComplexTask] ── 开始编排执行 ──")
	execStart := time.Now()
	orchestrator := NewOrchestrator(rt.bridge, rt.store)
	results := orchestrator.Execute(rt.task.Ctx, rt.task.TaskID, rt.rootSession, childSessions, rt.tools, rt.sendEvent)
	execDuration := time.Since(execStart)

	cancelled := rt.task.Ctx != nil && rt.task.Ctx.Err() != nil
	if cancelled {
		log.Printf("[ComplexTask] ✗ 任务被取消 taskID=%s", rt.task.TaskID)
		rt.rootSession.SetStatus("cancelled")
		rt.store.Save(rt.rootSession)
		rt.store.SaveIndex(rt.rootSession, childSessionList(childSessions))
		return results, true
	}

	for _, r := range results {
		rt.bridge.hooks.FireSubTaskDone(rt.task, r)
		if child, ok := childSessions[r.SubTaskID]; ok {
			child.mu.Lock()
			childCalls := make([]ToolCallRecord, len(child.ToolCalls))
			copy(childCalls, child.ToolCalls)
			child.mu.Unlock()
			for _, tc := range childCalls {
				if tc.Success {
					rt.rootSession.RecordToolCall(tc)
				}
			}
		}
	}

	doneCount, failCount, skipCount, asyncCount, deferCount := 0, 0, 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "done":
			doneCount++
		case "failed":
			failCount++
		case "skipped":
			skipCount++
		case "async":
			asyncCount++
		case "deferred":
			deferCount++
		}
	}
	log.Printf("[ComplexTask] ✓ 编排执行完成 duration=%v total=%d done=%d failed=%d skipped=%d async=%d deferred=%d",
		execDuration, len(results), doneCount, failCount, skipCount, asyncCount, deferCount)
	rt.sendEvent("progress", fmt.Sprintf("编排执行完成，耗时 %s — 成功:%d 失败:%d 跳过:%d",
		fmtDuration(execDuration), doneCount, failCount, skipCount))
	return results, false
}

func (rt *ComplexTaskRuntime) finishAsync(results []SubTaskResult, childSessions map[string]*TaskSession) (string, bool) {
	asyncCount, deferCount := 0, 0
	for _, r := range results {
		if r.Status == "async" {
			asyncCount++
		}
		if r.Status == "deferred" {
			deferCount++
		}
	}
	if asyncCount == 0 {
		return "", false
	}

	log.Printf("[ComplexTask] async subtasks detected: async=%d deferred=%d, skip synthesis", asyncCount, deferCount)
	summary := buildAsyncAcknowledgment(results)
	finalizeRootSession(rt.store, rt.rootSession, "async", summary, childSessions)
	log.Printf("[ComplexTask] ◀ 异步任务确认 taskID=%s duration=%v async=%d deferred=%d",
		rt.task.TaskID, time.Since(rt.start), asyncCount, deferCount)
	return summary, true
}

func (rt *ComplexTaskRuntime) synthesize(results []SubTaskResult, childSessions map[string]*TaskSession) string {
	log.Printf("[ComplexTask] ── 开始汇总 ──")
	synthStart := time.Now()
	orchestrator := NewOrchestrator(rt.bridge, rt.store)
	summary := orchestrator.Synthesize(rt.rootSession, childSessions, results, rt.query, rt.sendEvent)
	synthDuration := time.Since(synthStart)
	log.Printf("[ComplexTask] ✓ 汇总完成 duration=%v summaryLen=%d", synthDuration, len(summary))

	finalizeRootSession(rt.store, rt.rootSession, "done", summary, childSessions)

	log.Printf("[ComplexTask] ◀ 复杂任务完成 taskID=%s duration=%v", rt.task.TaskID, time.Since(rt.start))
	return summary
}

func (rt *ComplexTaskRuntime) buildPlanSummary(subtasks []SubTaskPlan) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("拆解为 %d 个子任务: ", len(subtasks)))
	for i, st := range subtasks {
		if i > 0 {
			sb.WriteString(" → ")
		}
		sb.WriteString(fmt.Sprintf("(%d)%s", i+1, st.Title))
	}
	return sb.String()
}

func (rt *ComplexTaskRuntime) sendPlanDetails(subtasks []SubTaskPlan) {
	for i, st := range subtasks {
		var detail strings.Builder
		detail.WriteString(fmt.Sprintf("📌 子任务[%d/%d] %s\n", i+1, len(subtasks), st.Title))
		detail.WriteString(fmt.Sprintf("  描述: %s\n", st.Description))
		if len(st.DependsOn) > 0 {
			detail.WriteString(fmt.Sprintf("  依赖: %s\n", strings.Join(st.DependsOn, ", ")))
		}
		if len(st.ToolsHint) > 0 {
			detail.WriteString(fmt.Sprintf("  工具: %s\n", strings.Join(st.ToolsHint, ", ")))
		}
		if len(st.ToolParams) > 0 {
			detail.WriteString("  参数:\n")
			for k, v := range st.ToolParams {
				detail.WriteString(fmt.Sprintf("    - %s: %v\n", k, v))
			}
		}
		rt.sendEvent("plan_detail", detail.String())
	}
}
