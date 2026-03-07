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
	Event string `json:"event"` // "chunk" | "tool_info" | "plan_start" | "plan_done" | "plan_review_start" | "plan_review_result" | "subtask_start" | "subtask_done" | "subtask_fail" | "subtask_skip" | "failure_decision" | "synthesis" | "resume" | "resume_info"
	Text  string `json:"text"`
}

// handleAssistantTask 处理 assistant_chat 任务：流式 LLM + 工具调用循环 + 任务拆解支持
func (b *Bridge) handleAssistantTask(taskID string, payload *AssistantTaskPayload) {
	log.Printf("[Assistant] task=%s account=%s query=%s", taskID, payload.Account, payload.Query)

	// 发送 task_accepted
	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskAccepted,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      "go_blog",
		Payload: mustMarshal(uap.TaskAcceptedPayload{TaskID: taskID}),
		Ts:      time.Now().UnixMilli(),
	})

	ctx := &TaskContext{
		TaskID:        taskID,
		Account:       payload.Account,
		Query:         payload.Query,
		Source:        "web",
		SelectedTools: payload.SelectedTools,
		Sink:          &StreamingSink{bridge: b, taskID: taskID},
	}

	_, err := b.processTask(ctx)

	// 发送 task_complete
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskComplete,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      "go_blog",
		Payload: mustMarshal(uap.TaskCompletePayload{TaskID: taskID, Status: status, Error: errMsg}),
		Ts:      time.Now().UnixMilli(),
	})

	log.Printf("[Assistant] task=%s completed status=%s", taskID, status)
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

// handleLLMRequestTask 处理 llm_request 任务：使用预构建消息 + 工具调用循环
func (b *Bridge) handleLLMRequestTask(taskID string, payload *LLMRequestPayload) {
	log.Printf("[LLMRequest] task=%s account=%s messages=%d noTools=%v", taskID, payload.Account, len(payload.Messages), payload.NoTools)

	// 发送 task_accepted
	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskAccepted,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      "go_blog",
		Payload: mustMarshal(uap.TaskAcceptedPayload{TaskID: taskID}),
		Ts:      time.Now().UnixMilli(),
	})

	ctx := &TaskContext{
		TaskID:        taskID,
		Account:       payload.Account,
		Source:        "llm_request",
		Messages:      payload.Messages,
		SelectedTools: payload.SelectedTools,
		NoTools:       payload.NoTools,
		Sink:          &LLMRequestSink{bridge: b, taskID: taskID},
	}

	result, err := b.processTask(ctx)

	// 发送 task_complete（含结果文本）
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskComplete,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      "go_blog",
		Payload: mustMarshal(uap.TaskCompletePayload{TaskID: taskID, Status: status, Error: errMsg, Result: result}),
		Ts:      time.Now().UnixMilli(),
	})

	log.Printf("[LLMRequest] task=%s completed status=%s resultLen=%d", taskID, status, len(result))
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
	sb.WriteString(fmt.Sprintf("account: %s\n", account))
	sb.WriteString(fmt.Sprintf("当前日期: %s\n", today))

	// 任务拆解指引
	sb.WriteString(`
使用account:%s账户填充字段，不要向用户询问使用哪个字段了直接使用,account填充。
## 任务拆解能力
当你判断用户的请求包含多个独立步骤，且这些步骤之间有明确的依赖关系时，
你应该调用 plan_and_execute 工具来拆解和编排执行。

**任务处理流程：**

1. **初步判断**
   - 分析任务复杂度，决定是否拆解
   - 简单任务：直接调用工具执行
   - 复杂任务：进入规划阶段

2. **任务规划**
   - 评估现有工具是否能完成任务
   - 收集完成任务所需的信息
   - 将复杂任务拆解为可执行的简单子任务

3. **执行与整合**
   - 按序执行简单任务
   - 多个并行任务完成后整合结果
   - 确保任务执行的完整性和连贯性
   - 将最终汇总结果反馈给用户

**原则：** 先探索信息，再拆解任务，最后整合汇报。

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
