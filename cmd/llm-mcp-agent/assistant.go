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
	TaskType      string   `json:"task_type"`       // "assistant_chat"
	Messages      []Message `json:"messages"`        // 对话历史（仅含最后一条 user 消息）
	SelectedTools []string `json:"selected_tools"`  // 用户选择的工具
	Account       string   `json:"account"`         // 用户账号
	Query         string   `json:"query"`           // 用户问题
}

// LLMRequestPayload llm_request 任务的 payload（go_blog 同步 LLM 请求代理）
type LLMRequestPayload struct {
	TaskType      string    `json:"task_type"`       // "llm_request"
	Messages      []Message `json:"messages"`        // 预构建的消息列表
	Account       string    `json:"account"`         // 用户账号
	SelectedTools []string  `json:"selected_tools"`  // 指定工具（nil=全部）
	NoTools       bool      `json:"no_tools"`        // true=不使用工具
}

// AssistantEventPayload MsgTaskEvent 的事件数据
type AssistantEventPayload struct {
	Event string `json:"event"` // "chunk" | "tool_info"
	Text  string `json:"text"`
}

// handleAssistantTask 处理 assistant_chat 任务：流式 LLM + 工具调用循环
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

	// 构建 system prompt
	systemPrompt := b.buildAssistantSystemPrompt(payload.Account)

	// 初始化消息列表
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: payload.Query},
	}

	// 获取工具列表（先按用户选择过滤，再智能路由）
	tools := b.filterToolsBySelection(payload.SelectedTools)
	if len(tools) > 15 {
		tools = b.routeTools(payload.Query, tools)
	}

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
			// 发送错误信息给前端
			b.sendTaskEvent(taskID, "chunk", fmt.Sprintf("\n\n抱歉，AI 服务暂时不可用: %v", err))
			break
		}

		// 无工具调用 → 对话结束
		if len(toolCalls) == 0 {
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

			// 发送工具调用状态
			toolInfoMsg := fmt.Sprintf("[Calling tool %s with args %s]", originalName, tc.Function.Arguments)
			b.sendTaskEvent(taskID, "tool_info", toolInfoMsg)

			log.Printf("[Assistant] tool_call: %s args=%s", originalName, tc.Function.Arguments)

			result, err := b.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				log.Printf("[Assistant] tool_call %s failed: %v", originalName, err)
				result = fmt.Sprintf("工具调用失败: %v", err)
			}

			// 追加 tool 消息
			messages = append(messages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}

		// 最后一次迭代
		if i == maxIter-1 {
			b.sendTaskEvent(taskID, "chunk", "\n\n抱歉，处理过程过于复杂，请尝试简化您的请求。")
		}
	}

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

// buildAssistantSystemPrompt 构建 assistant 的系统提示（复用 chat.go 的上下文获取逻辑）
func (b *Bridge) buildAssistantSystemPrompt(account string) string {
	var sb strings.Builder
	sb.WriteString(b.cfg.SystemPromptPrefix)
	sb.WriteString("\n\n")

	today := time.Now().Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("当前用户: %s\n", account))
	sb.WriteString(fmt.Sprintf("当前日期: %s\n", today))

	// 复用 chat.go 的并发上下文获取
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
