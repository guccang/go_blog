package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"uap"
)

// AssistantTaskPayload assistant_chat 任务的 payload
type AssistantTaskPayload struct {
	TaskType      string    `json:"task_type"`      // "assistant_chat"
	Messages      []Message `json:"messages"`       // 对话历史（仅含最后一条 user 消息）
	SelectedTools []string  `json:"selected_tools"` // 用户选择的工具
	Account       string    `json:"account"`        // 用户账号
	Query         string    `json:"query"`          // 用户问题
}

// LLMRequestPayload llm_request 任务的 payload（go_blog 同步 LLM 请求代理）
type LLMRequestPayload struct {
	TaskType      string    `json:"task_type"`      // "llm_request"
	Messages      []Message `json:"messages"`       // 预构建的消息列表
	Account       string    `json:"account"`        // 用户账号
	SelectedTools []string  `json:"selected_tools"` // 指定工具（nil=全部）
	NoTools       bool      `json:"no_tools"`       // true=不使用工具
}

// ResumeTaskPayload resume_task 任务的 payload（断点续传）
type ResumeTaskPayload struct {
	TaskType      string `json:"task_type"`       // "resume_task"
	RootSessionID string `json:"root_session_id"` // 要恢复的根会话 ID
	Account       string `json:"account"`
}

// AssistantEventPayload MsgTaskEvent 的事件数据
type AssistantEventPayload struct {
	Event string `json:"event"` // "chunk" | "tool_info" | "plan_start" | "plan_done" | "subtask_start" | "subtask_done" | "subtask_fail" | "subtask_skip" | "failure_decision" | "synthesis" | "resume" | "resume_info"
	Text  string `json:"text"`
}

// handleAssistantTask 处理 assistant_chat 任务：流式 LLM + 工具调用循环 + 任务拆解支持
func (b *Bridge) handleAssistantTask(taskID string, payload *AssistantTaskPayload) {
	log.Printf("[Assistant] task=%s account=%s query=%s", taskID, payload.Account, payload.Query)

	// 发送 task_accepted
	b.client.Send(&uap.Message{
		Type: uap.MsgTaskAccepted,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskAcceptedPayload{
			TaskID: taskID,
		}),
		Ts: time.Now().UnixMilli(),
	})

	// 创建会话存储和根会话
	store := NewSessionStore(b.cfg.SessionDir)
	rootSession := NewRootSession(taskID, payload.Query, payload.Account)

	// 构建 system prompt（含任务拆解指引）
	systemPrompt := b.buildAssistantSystemPrompt(payload.Account)

	// 初始化消息列表
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: payload.Query},
	}

	// 记录到 session
	rootSession.AppendMessage(messages[0])
	rootSession.AppendMessage(messages[1])

	// 获取工具列表（先按用户选择过滤，再智能路由）
	tools := b.filterToolsBySelection(payload.SelectedTools)
	if len(tools) > 15 {
		tools = b.routeTools(payload.Query, tools)
	}

	// 注入 plan_and_execute 虚拟工具
	tools = append(tools, planAndExecuteTool)

	// 发送工具数量信息
	toolCountMsg := fmt.Sprintf("[🔧 本次加载 %d 个工具]", len(tools))
	b.sendTaskEvent(taskID, "tool_info", toolCountMsg)

	// 工具调用循环
	maxIter := b.cfg.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	var finalErr error

	for i := 0; i < maxIter; i++ {
		log.Printf("[Assistant] iteration %d/%d, messages=%d", i+1, maxIter, len(messages))

		// 流式 LLM 请求，每个 chunk 通过 MsgTaskEvent 发回
		text, toolCalls, err := SendStreamingLLMRequest(&b.cfg.LLM, messages, tools, func(chunk string) {
			b.sendTaskEvent(taskID, "chunk", chunk)
		})
		if err != nil {
			log.Printf("[Assistant] LLM error: %v", err)
			finalErr = err
			b.sendTaskEvent(taskID, "chunk", fmt.Sprintf("\n\n抱歉，AI 服务暂时不可用: %v", err))
			break
		}

		// 记录 assistant 消息到 session
		assistantMsg := Message{Role: "assistant", Content: text, ToolCalls: toolCalls}
		rootSession.AppendMessage(assistantMsg)

		// 无工具调用 → 对话结束
		if len(toolCalls) == 0 {
			rootSession.SetResult(text)
			break
		}

		// 检查是否调用了 plan_and_execute
		planCallIdx := -1
		for idx, tc := range toolCalls {
			if tc.Function.Name == "plan_and_execute" {
				planCallIdx = idx
				break
			}
		}

		if planCallIdx >= 0 {
			// 进入复杂任务处理流程
			var reasoning string
			var args struct {
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal([]byte(toolCalls[planCallIdx].Function.Arguments), &args); err == nil {
				reasoning = args.Reasoning
			}
			log.Printf("[Assistant] plan_and_execute triggered: %s", reasoning)

			// 执行复杂任务（plan → orchestrate → synthesize）
			result := b.handleComplexTask(taskID, payload, rootSession, store, tools)

			// 发送结果作为 chunk
			b.sendTaskEvent(taskID, "chunk", result)
			break
		}

		// 普通工具调用 → 追加 assistant 消息
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   text,
			ToolCalls: toolCalls,
		})

		// 执行每个工具调用
		for _, tc := range toolCalls {
			originalName := unsanitizeToolName(tc.Function.Name)

			toolInfoMsg := fmt.Sprintf("[Calling tool %s with args %s]", originalName, tc.Function.Arguments)
			b.sendTaskEvent(taskID, "tool_info", toolInfoMsg)

			log.Printf("[Assistant] tool_call: %s args=%s", originalName, tc.Function.Arguments)

			start := time.Now()
			result, err := b.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			duration := time.Since(start)

			success := true
			if err != nil {
				log.Printf("[Assistant] tool_call %s failed: %v", originalName, err)
				result = fmt.Sprintf("工具调用失败: %v", err)
				success = false
			}

			// 记录工具调用到 session
			rootSession.RecordToolCall(ToolCallRecord{
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
			rootSession.AppendMessage(toolMsg)
			messages = append(messages, toolMsg)
		}

		// 最后一次迭代
		if i == maxIter-1 {
			b.sendTaskEvent(taskID, "chunk", "\n\n抱歉，处理过程过于复杂，请尝试简化您的请求。")
		}
	}

	// 保存根会话
	if finalErr != nil {
		rootSession.SetStatus("failed")
		rootSession.SetError(finalErr.Error())
	} else {
		rootSession.SetStatus("done")
	}
	store.Save(rootSession)
	store.SaveIndex(rootSession, nil)

	// 发送 task_complete
	status := "success"
	errMsg := ""
	if finalErr != nil {
		status = "failed"
		errMsg = finalErr.Error()
	}

	b.client.Send(&uap.Message{
		Type: uap.MsgTaskComplete,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskCompletePayload{
			TaskID: taskID,
			Status: status,
			Error:  errMsg,
		}),
		Ts: time.Now().UnixMilli(),
	})

	log.Printf("[Assistant] task=%s completed status=%s", taskID, status)
}

// handleComplexTask 处理复杂任务：规划 → 编排 → 汇总
func (b *Bridge) handleComplexTask(
	taskID string,
	payload *AssistantTaskPayload,
	rootSession *TaskSession,
	store *SessionStore,
	tools []LLMTool,
) string {
	sendEvent := func(event, text string) {
		b.sendTaskEvent(taskID, event, text)
	}

	// ① 规划阶段
	sendEvent("plan_start", "正在分析任务...")

	maxSubTasks := b.cfg.MaxSubTasks
	if maxSubTasks <= 0 {
		maxSubTasks = 10
	}

	plan, err := PlanTask(&b.cfg.LLM, payload.Query, tools, payload.Account, maxSubTasks)
	if err != nil {
		log.Printf("[Assistant] planning failed: %v", err)
		sendEvent("plan_done", fmt.Sprintf("任务规划失败: %v", err))
		return fmt.Sprintf("抱歉，任务规划失败: %v", err)
	}

	rootSession.Plan = plan
	store.Save(rootSession)

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

	// ② 为每个子任务创建 ChildSession
	childSessions := make(map[string]*TaskSession)
	for _, st := range plan.SubTasks {
		child := NewChildSession(rootSession, st.Title, st.Description)
		child.ID = st.ID // 使用计划中的 ID，方便关联
		childSessions[st.ID] = child
		rootSession.AddChildID(st.ID)
		store.Save(child)
	}
	store.Save(rootSession)

	// ③ 编排执行
	orchestrator := NewOrchestrator(b, store)
	results := orchestrator.Execute(taskID, rootSession, childSessions, tools, sendEvent)

	// ④ 汇总
	summary := orchestrator.Synthesize(rootSession, results, payload.Query, sendEvent)

	rootSession.SetStatus("done")
	rootSession.SetResult(summary)
	rootSession.Summary = summary
	store.Save(rootSession)

	// 保存索引
	var childList []*TaskSession
	for _, c := range childSessions {
		childList = append(childList, c)
	}
	store.SaveIndex(rootSession, childList)

	return summary
}

// handleResumeTask 处理断点续传请求
func (b *Bridge) handleResumeTask(taskID string, payload *ResumeTaskPayload) {
	log.Printf("[Resume] task=%s resuming root_session=%s", taskID, payload.RootSessionID)

	// 发送 task_accepted
	b.client.Send(&uap.Message{
		Type: uap.MsgTaskAccepted,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskAcceptedPayload{
			TaskID: taskID,
		}),
		Ts: time.Now().UnixMilli(),
	})

	store := NewSessionStore(b.cfg.SessionDir)
	orchestrator := NewOrchestrator(b, store)

	// 获取工具列表
	tools := b.getLLMTools()
	tools = append(tools, planAndExecuteTool)

	sendEvent := func(event, text string) {
		b.sendTaskEvent(taskID, event, text)
	}

	result, err := orchestrator.Resume(payload.RootSessionID, tools, sendEvent)

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		log.Printf("[Resume] failed: %v", err)
		b.sendTaskEvent(taskID, "chunk", fmt.Sprintf("恢复失败: %v", err))
	} else {
		b.sendTaskEvent(taskID, "chunk", result)
	}

	b.client.Send(&uap.Message{
		Type: uap.MsgTaskComplete,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskCompletePayload{
			TaskID: taskID,
			Status: status,
			Error:  errMsg,
		}),
		Ts: time.Now().UnixMilli(),
	})

	log.Printf("[Resume] task=%s completed status=%s", taskID, status)
}

// handleLLMRequestTask 处理 llm_request 任务：直接使用调用方预构建的消息列表 + 工具调用循环
func (b *Bridge) handleLLMRequestTask(taskID string, payload *LLMRequestPayload) {
	log.Printf("[LLMRequest] task=%s account=%s messages=%d noTools=%v", taskID, payload.Account, len(payload.Messages), payload.NoTools)

	// 发送 task_accepted
	b.client.Send(&uap.Message{
		Type: uap.MsgTaskAccepted,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskAcceptedPayload{
			TaskID: taskID,
		}),
		Ts: time.Now().UnixMilli(),
	})

	// 直接使用调用方提供的消息列表（不注入额外 system prompt）
	messages := make([]Message, len(payload.Messages))
	copy(messages, payload.Messages)

	// 确定工具列表
	var tools []LLMTool
	if payload.NoTools {
		tools = nil
	} else if len(payload.SelectedTools) > 0 {
		tools = b.filterToolsBySelection(payload.SelectedTools)
	} else {
		tools = b.getLLMTools()
	}

	// 工具调用循环
	maxIter := b.cfg.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	var finalText string
	var finalErr error

	for i := 0; i < maxIter; i++ {
		log.Printf("[LLMRequest] iteration %d/%d, messages=%d", i+1, maxIter, len(messages))

		text, toolCalls, err := SendLLMRequest(&b.cfg.LLM, messages, tools)
		if err != nil {
			log.Printf("[LLMRequest] LLM error: %v", err)
			finalErr = err
			break
		}

		// 无工具调用 → 对话结束
		if len(toolCalls) == 0 {
			finalText = text
			break
		}

		// 如果 NoTools 但 LLM 仍然返回了工具调用，忽略并取文本
		if payload.NoTools {
			finalText = text
			break
		}

		// 有工具调用 → 追加 assistant 消息
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   text,
			ToolCalls: toolCalls,
		})

		// 执行每个工具调用
		for _, tc := range toolCalls {
			originalName := unsanitizeToolName(tc.Function.Name)
			log.Printf("[LLMRequest] tool_call: %s args=%s", originalName, tc.Function.Arguments)

			result, err := b.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				log.Printf("[LLMRequest] tool_call %s failed: %v", originalName, err)
				result = fmt.Sprintf("工具调用失败: %v", err)
			}

			messages = append(messages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// 最后一次迭代
		if i == maxIter-1 {
			finalText = "工具调用已完成(达到最大迭代限制)"
		}
	}

	// 发送 task_complete（含结果文本）
	status := "success"
	errMsg := ""
	if finalErr != nil {
		status = "failed"
		errMsg = finalErr.Error()
	}

	b.client.Send(&uap.Message{
		Type: uap.MsgTaskComplete,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskCompletePayload{
			TaskID: taskID,
			Status: status,
			Error:  errMsg,
			Result: finalText,
		}),
		Ts: time.Now().UnixMilli(),
	})

	log.Printf("[LLMRequest] task=%s completed status=%s resultLen=%d", taskID, status, len(finalText))
}

// sendTaskEvent 发送任务进度事件
func (b *Bridge) sendTaskEvent(taskID, event, text string) {
	eventData := mustMarshal(AssistantEventPayload{
		Event: event,
		Text:  text,
	})

	b.client.Send(&uap.Message{
		Type: uap.MsgTaskEvent,
		ID:   uap.NewMsgID(),
		From: b.cfg.AgentID,
		To:   "go_blog",
		Payload: mustMarshal(uap.TaskEventPayload{
			TaskID: taskID,
			Event:  json.RawMessage(eventData),
		}),
		Ts: time.Now().UnixMilli(),
	})
}

// buildAssistantSystemPrompt 构建 assistant 的系统提示（含任务拆解指引）
func (b *Bridge) buildAssistantSystemPrompt(account string) string {
	var sb strings.Builder
	sb.WriteString(b.cfg.SystemPromptPrefix)
	sb.WriteString("\n\n")

	today := time.Now().Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("当前用户: %s\n", account))
	sb.WriteString(fmt.Sprintf("当前日期: %s\n", today))

	// 任务拆解指引
	sb.WriteString(`

## 任务拆解能力
当你判断用户的请求包含多个独立步骤，且这些步骤之间有明确的依赖关系时，
你应该调用 plan_and_execute 工具来拆解和编排执行。

适合拆解的场景：
- 需要先获取数据，再基于数据做分析，再基于分析创建内容
- 需要同时处理多个独立的子目标
- 任务步骤超过3步且有前后依赖

不需要拆解的场景：
- 简单问答（"今天几号"）
- 单一工具调用（"创建一个提醒"）
- 可以在一次对话中直接完成的任务
`)

	// 并发获取上下文数据
	type ctxResult struct {
		label string
		data  string
	}

	ch := make(chan ctxResult, 2)
	done := make(chan struct{}, 2)

	go func() {
		args, _ := json.Marshal(map[string]string{"account": account, "date": today})
		data, err := b.callToolWithTimeout("RawGetTodosByDate", args, 3*time.Second)
		if err == nil && data != "" {
			ch <- ctxResult{label: "今日待办", data: data}
		}
		done <- struct{}{}
	}()

	go func() {
		args, _ := json.Marshal(map[string]string{"account": account, "date": today})
		data, err := b.callToolWithTimeout("RawGetExerciseByDate", args, 3*time.Second)
		if err == nil && data != "" {
			ch <- ctxResult{label: "今日运动", data: data}
		}
		done <- struct{}{}
	}()

	// 等待两个 goroutine 完成
	<-done
	<-done
	close(ch)

	var ctxParts []string
	for r := range ch {
		ctxParts = append(ctxParts, fmt.Sprintf("[%s]\n%s", r.label, r.data))
	}

	if len(ctxParts) > 0 {
		sb.WriteString("\n用户当前数据:\n")
		sb.WriteString(strings.Join(ctxParts, "\n\n"))
	}

	return sb.String()
}
