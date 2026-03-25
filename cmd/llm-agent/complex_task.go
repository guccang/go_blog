package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// handleComplexTask 处理复杂任务：规划 → 编排 → 汇总
func (b *Bridge) handleComplexTask(
	ctx *TaskContext,
	rootSession *TaskSession,
	store *SessionStore,
	tools []LLMTool,
	completedWork string, // 简单路径中已完成的工具调用摘要（可为空）
) string {
	complexStart := time.Now()
	sendEvent := func(event, text string) {
		ctx.Sink.OnEvent(event, text)
	}

	query := ctx.Query
	if query == "" {
		query = rootSession.Title
	}

	// ① 规划阶段
	log.Printf("[ComplexTask] ▶ 开始复杂任务处理 taskID=%s query=%s", ctx.TaskID, truncate(query, 100))

	// 检查是否被用户取消
	if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
		log.Printf("[ComplexTask] ✗ 规划前任务被取消 taskID=%s", ctx.TaskID)
		rootSession.SetStatus("cancelled")
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
		return "任务已停止。"
	}

	sendEvent("plan_start", "正在分析任务...")

	maxSubTasks := b.cfg.MaxSubTasks
	if maxSubTasks <= 0 {
		maxSubTasks = 10
	}

	// 使用可用 skill（过滤掉 agent 离线的）构建规划指引
	var skillBlock string
	if b.skillMgr != nil {
		availableSkills := b.skillMgr.GetAvailableSkills()
		if len(availableSkills) > 0 {
			skillBlock = b.skillMgr.BuildSkillBlock(availableSkills)
		}
	}

	planStart := time.Now()
	activeCfg := b.activeLLM.Get()
	plan, err := PlanTask(&activeCfg, query, tools, ctx.Account, maxSubTasks, completedWork, skillBlock, b.cfg.Fallbacks, b.fallbackCooldown())
	planDuration := time.Since(planStart)

	if err != nil {
		log.Printf("[ComplexTask] ✗ 任务规划失败 duration=%v error=%v", planDuration, err)
		sendEvent("plan_done", fmt.Sprintf("任务规划失败: %v", err))
		// 保存失败状态
		rootSession.SetStatus("failed")
		rootSession.SetError(err.Error())
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
		return fmt.Sprintf("抱歉，任务规划失败: %v", err)
	}

	// 打印计划详情
	log.Printf("[ComplexTask] ✓ 任务规划完成 duration=%v subtasks=%d mode=%s", planDuration, len(plan.SubTasks), plan.ExecutionMode)
	for i, st := range plan.SubTasks {
		log.Printf("[ComplexTask]   子任务[%d] id=%s title=%s depends=%v tools_hint=%v",
			i+1, st.ID, st.Title, st.DependsOn, st.ToolsHint)
	}

	rootSession.Plan = plan
	store.Save(rootSession)

	// 触发计划创建 hook
	b.hooks.FirePlanCreated(ctx, plan)

	// 发送规划耗时
	sendEvent("plan_timing", fmt.Sprintf("任务规划完成，耗时 %s，拆解为 %d 个子任务", fmtDuration(planDuration), len(plan.SubTasks)))

	// 发送计划摘要
	var planSummary strings.Builder
	planSummary.WriteString(fmt.Sprintf("拆解为 %d 个子任务: ", len(plan.SubTasks)))
	for i, st := range plan.SubTasks {
		if i > 0 {
			planSummary.WriteString(" → ")
		}
		planSummary.WriteString(fmt.Sprintf("(%d)%s", i+1, st.Title))
	}
	sendEvent("plan_done", planSummary.String())

	// 发送每个子任务的详细信息（含 tool_params）
	sendPlanDetails := func(subtasks []SubTaskPlan) {
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
			sendEvent("plan_detail", detail.String())
		}
	}
	sendPlanDetails(plan.SubTasks)

	// 检查是否被用户取消
	if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
		log.Printf("[ComplexTask] ✗ 审查前任务被取消 taskID=%s", ctx.TaskID)
		rootSession.SetStatus("cancelled")
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
		return "任务已停止。"
	}

	// ② LLM审查计划（含 agent 能力信息，确保工具参数完整性）
	sendEvent("plan_review_start", "正在审查计划参数...")
	agentCapabilities := b.getAgentDescriptionBlock()
	reviewStart := time.Now()
	reviewCfg := b.activeLLM.Get()
	review, err := ReviewPlan(&reviewCfg, query, plan, tools, ctx.Account, agentCapabilities, b.cfg.Fallbacks, b.fallbackCooldown())
	reviewDuration := time.Since(reviewStart)
	if err != nil {
		log.Printf("[ComplexTask] ⚠ 计划审查失败 error=%v，继续执行原计划", err)
		sendEvent("plan_review_result", fmt.Sprintf("计划审查跳过: %v", err))
	} else if review.Action == "optimize" && review.Plan != nil {
		log.Printf("[ComplexTask] 计划已优化: %s", review.Reason)
		plan = review.Plan
		rootSession.Plan = plan
		store.Save(rootSession)
		sendEvent("plan_review_result", fmt.Sprintf("计划已优化: %s", review.Reason))
		// 重新展示优化后的计划摘要
		var optimizedSummary strings.Builder
		optimizedSummary.WriteString(fmt.Sprintf("优化后 %d 个子任务: ", len(plan.SubTasks)))
		for i, st := range plan.SubTasks {
			if i > 0 {
				optimizedSummary.WriteString(" → ")
			}
			optimizedSummary.WriteString(fmt.Sprintf("(%d)%s", i+1, st.Title))
		}
		sendEvent("plan_done", optimizedSummary.String())
		// 重新展示优化后的子任务详情
		sendPlanDetails(plan.SubTasks)
	} else {
		reason := "审查通过"
		if review != nil && review.Reason != "" {
			reason = review.Reason
		}
		log.Printf("[ComplexTask] 计划审查通过: %s", reason)
		sendEvent("plan_review_result", fmt.Sprintf("计划审查通过: %s", reason))
	}
	sendEvent("review_timing", fmt.Sprintf("审查完成，耗时 %s", fmtDuration(reviewDuration)))

	// ③ 为每个子任务创建 ChildSession
	childSessions := make(map[string]*TaskSession)
	for _, st := range plan.SubTasks {
		child := NewChildSession(rootSession, st.Title, st.Description)
		child.ID = st.ID
		childSessions[st.ID] = child
		rootSession.AddChildID(st.ID)
		store.Save(child)
	}
	store.Save(rootSession)

	// ④ 编排执行
	log.Printf("[ComplexTask] ── 开始编排执行 ──")
	execStart := time.Now()
	orchestrator := NewOrchestrator(b, store)
	results := orchestrator.Execute(ctx.Ctx, ctx.TaskID, rootSession, childSessions, tools, sendEvent)
	execDuration := time.Since(execStart)

	// 检查是否被用户取消
	cancelled := ctx.Ctx != nil && ctx.Ctx.Err() != nil
	if cancelled {
		log.Printf("[ComplexTask] ✗ 任务被取消 taskID=%s", ctx.TaskID)
		rootSession.SetStatus("cancelled")
		store.Save(rootSession)
		var childList []*TaskSession
		for _, c := range childSessions {
			childList = append(childList, c)
		}
		store.SaveIndex(rootSession, childList)
		return "任务已停止。"
	}

	// 触发子任务完成 hook + 汇总子任务工具调用到 rootSession
	for _, r := range results {
		b.hooks.FireSubTaskDone(ctx, r)

		// 将子任务的成功 ToolCalls 汇总到 rootSession（供 FireTaskEnd 统计使用）
		if child, ok := childSessions[r.SubTaskID]; ok {
			child.mu.Lock()
			childCalls := make([]ToolCallRecord, len(child.ToolCalls))
			copy(childCalls, child.ToolCalls)
			child.mu.Unlock()
			for _, tc := range childCalls {
				if tc.Success {
					rootSession.RecordToolCall(tc)
				}
			}
		}
	}

	// 统计结果
	var doneCount, failCount, skipCount, asyncCount, deferCount int
	var asyncInfos []AsyncSessionInfo
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
			asyncInfos = append(asyncInfos, r.AsyncSessions...)
		case "deferred":
			deferCount++
		}
	}
	log.Printf("[ComplexTask] ✓ 编排执行完成 duration=%v total=%d done=%d failed=%d skipped=%d async=%d deferred=%d",
		execDuration, len(results), doneCount, failCount, skipCount, asyncCount, deferCount)

	// 发送编排执行完成统计
	sendEvent("progress", fmt.Sprintf("编排执行完成，耗时 %s — 成功:%d 失败:%d 跳过:%d",
		fmtDuration(execDuration), doneCount, failCount, skipCount))

	// 检测异步子任务 → 跳过 Synthesize，返回即时确认
	if asyncCount > 0 {
		log.Printf("[ComplexTask] async subtasks detected: async=%d deferred=%d, skip synthesis", asyncCount, deferCount)
		_ = asyncInfos // asyncInfos 已包含在 results 中

		summary := buildAsyncAcknowledgment(results)
		rootSession.SetStatus("async")
		rootSession.SetResult(summary)
		rootSession.Summary = summary
		store.Save(rootSession)

		var childList []*TaskSession
		for _, c := range childSessions {
			childList = append(childList, c)
		}
		store.SaveIndex(rootSession, childList)

		totalDuration := time.Since(complexStart)
		log.Printf("[ComplexTask] ◀ 异步任务确认 taskID=%s duration=%v async=%d deferred=%d",
			ctx.TaskID, totalDuration, asyncCount, deferCount)
		return summary
	}

	// ⑤ 汇总（仅在无异步子任务时执行）
	log.Printf("[ComplexTask] ── 开始汇总 ──")
	synthStart := time.Now()
	summary := orchestrator.Synthesize(rootSession, childSessions, results, query, sendEvent)
	synthDuration := time.Since(synthStart)
	log.Printf("[ComplexTask] ✓ 汇总完成 duration=%v summaryLen=%d", synthDuration, len(summary))

	rootSession.SetStatus("done")
	rootSession.SetResult(summary)
	rootSession.Summary = summary
	store.Save(rootSession)

	// 保存索引（含子会话）
	var childList []*TaskSession
	for _, c := range childSessions {
		childList = append(childList, c)
	}
	store.SaveIndex(rootSession, childList)

	totalDuration := time.Since(complexStart)
	log.Printf("[ComplexTask] ◀ 复杂任务完成 taskID=%s duration=%v (plan=%v exec=%v synth=%v)",
		ctx.TaskID, totalDuration, planDuration, execDuration, synthDuration)

	return summary
}

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
			for _, a := range r.AsyncSessions {
				sids = append(sids, a.SessionID)
			}
			sb.WriteString(fmt.Sprintf("⏳ %s (后台执行中: %s)\n", r.Title, strings.Join(sids, ", ")))
		case "deferred":
			sb.WriteString(fmt.Sprintf("⏸ %s (等待前置任务完成)\n", r.Title))
		}
	}
	return sb.String()
}
