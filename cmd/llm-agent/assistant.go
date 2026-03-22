package main

import (
	"context"
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

// CronReminderPayload cron_reminder 任务的 payload（定时提醒）
type CronReminderPayload struct {
	Message    string `json:"message"`     // 提醒内容
	Account    string `json:"account"`     // 用户账号
	WechatUser string `json:"wechat_user"` // 微信用户标识
}

// CronQueryPayload cron_query 任务的 payload（定时查询）
type CronQueryPayload struct {
	Query      string `json:"query"`       // 查询问题
	Account    string `json:"account"`     // 用户账号
	WechatUser string `json:"wechat_user"` // 微信用户（有值则发送结果到微信）
}

// AssistantEventPayload MsgTaskEvent 的事件数据
type AssistantEventPayload struct {
	Event string `json:"event"` // "chunk" | "tool_info" | "plan_start" | "plan_done" | "plan_review_start" | "plan_review_result" | "subtask_start" | "subtask_done" | "subtask_fail" | "subtask_skip" | "failure_decision" | "synthesis" | "resume" | "resume_info"
	Text  string `json:"text"`
}

// handleAssistantTask 处理 assistant_chat 任务：流式 LLM + 工具调用循环 + 任务拆解支持
func (b *Bridge) handleAssistantTask(taskID string, payload *AssistantTaskPayload) {
	log.Printf("[Assistant] task=%s account=%s query=%s", taskID, payload.Account, payload.Query)

	// Web 来源使用 ChatSession 管理多轮对话
	session, isNew := b.sessionMgr.GetOrCreate("web", payload.Account, payload.Account)

	session.mu.Lock()
	session.LastActiveAt = time.Now()
	if isNew || len(session.Messages) == 0 {
		// 新会话：Messages 由 processTask 构建
		session.Messages = nil
	} else {
		// 续接对话：刷新 system prompt + 追加 user 消息
		if len(session.Messages) > 0 && session.Messages[0].Role == "system" {
			freshPrompt, _ := b.buildAssistantSystemPrompt(payload.Account)
			session.Messages[0].Content = freshPrompt
		}
		session.Messages = append(session.Messages, Message{Role: "user", Content: payload.Query})
		session.Messages = CompactMessages(session.Messages, b.sessionMgr.maxMessages)
	}
	session.TurnCount++
	session.mu.Unlock()

	ctx := &TaskContext{
		Ctx:           context.Background(),
		TaskID:        taskID,
		Account:       payload.Account,
		Query:         payload.Query,
		Source:        "web",
		SelectedTools: payload.SelectedTools,
		Sink:          &StreamingSink{bridge: b, taskID: taskID},
	}

	// 如果有历史消息，传入作为上下文
	if !isNew {
		session.mu.Lock()
		if len(session.Messages) > 0 {
			messagesCopy := make([]Message, len(session.Messages))
			copy(messagesCopy, session.Messages)
			ctx.Messages = messagesCopy
		}
		session.mu.Unlock()
	}

	result, err := b.processTask(ctx)

	// 将 assistant 回复追加到会话历史
	if result != "" {
		session.mu.Lock()
		// 如果是新会话，需要先补上 system + user 消息
		if isNew || len(session.Messages) == 0 {
			systemPrompt, _ := b.buildAssistantSystemPrompt(payload.Account)
			session.Messages = []Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: payload.Query},
			}
		}
		session.Messages = append(session.Messages, Message{Role: "assistant", Content: result})
		session.mu.Unlock()

		// 持久化会话
		if saveErr := b.sessionMgr.SaveSession(session); saveErr != nil {
			log.Printf("[Assistant] save session failed: %v", saveErr)
		}
	}

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

	ctx := &TaskContext{
		Ctx:           context.Background(),
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

// handleCronReminder 处理 cron_reminder 定时提醒任务：发微信通知 + 回发 task_complete 到 corn-agent
func (b *Bridge) handleCronReminder(taskID, sourceAgent string, payload *CronReminderPayload) {
	log.Printf("[CronReminder] task=%s account=%s wechat_user=%s message=%s",
		taskID, payload.Account, payload.WechatUser, payload.Message)

	status := "success"
	errMsg := ""

	wechatAgentID := b.findWechatAgent()
	if wechatAgentID == "" {
		status = "failed"
		errMsg = "no wechat-agent online"
		log.Printf("[CronReminder] task=%s failed: %s", taskID, errMsg)
	} else {
		b.sendWechat(wechatAgentID, payload.WechatUser, "⏰ "+payload.Message)
		log.Printf("[CronReminder] task=%s sent to wechat-agent=%s", taskID, wechatAgentID)
	}

	// 发送 task_complete 到 sourceAgent（corn-agent），而非 "go_blog"
	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskComplete,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      sourceAgent,
		Payload: mustMarshal(uap.TaskCompletePayload{TaskID: taskID, Status: status, Error: errMsg}),
		Ts:      time.Now().UnixMilli(),
	})

	log.Printf("[CronReminder] task=%s completed status=%s", taskID, status)
}

// handleCronQuery 处理 cron_query 定时查询任务：驱动 LLM + 工具调用循环执行查询，再发微信 + 回 task_complete
func (b *Bridge) handleCronQuery(taskID, sourceAgent string, payload *CronQueryPayload) {
	log.Printf("[CronQuery] task=%s account=%s wechat_user=%s query=%s",
		taskID, payload.Account, payload.WechatUser, payload.Query)

	ctx := &TaskContext{
		Ctx:     context.Background(),
		TaskID:  taskID,
		Account: payload.Account,
		Query:   payload.Query,
		Source:  "cron_query",
		Sink:    &LLMRequestSink{bridge: b, taskID: taskID},
	}

	result, err := b.processTask(ctx)

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		log.Printf("[CronQuery] task=%s processTask failed: %v", taskID, err)
	} else {
		log.Printf("[CronQuery] task=%s processTask done, resultLen=%d", taskID, len(result))
	}

	// 如果指定了微信用户，发送查询结果到微信
	if payload.WechatUser != "" && result != "" {
		wechatAgentID := b.findWechatAgent()
		if wechatAgentID == "" {
			log.Printf("[CronQuery] task=%s no wechat-agent online, skip sending", taskID)
		} else {
			b.sendWechat(wechatAgentID, payload.WechatUser, result)
			log.Printf("[CronQuery] task=%s sent result to wechat user=%s", taskID, payload.WechatUser)
		}
	}

	// 发送 task_complete 到 sourceAgent（cron-agent）
	b.client.Send(&uap.Message{
		Type:    uap.MsgTaskComplete,
		ID:      uap.NewMsgID(),
		From:    b.cfg.AgentID,
		To:      sourceAgent,
		Payload: mustMarshal(uap.TaskCompletePayload{TaskID: taskID, Status: status, Error: errMsg, Result: result}),
		Ts:      time.Now().UnixMilli(),
	})

	log.Printf("[CronQuery] task=%s completed status=%s", taskID, status)
}

// findWechatAgent 查找在线的 wechat-agent ID
func (b *Bridge) findWechatAgent() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	for id, info := range b.agentInfo {
		name := strings.ToLower(info.Name)
		idLower := strings.ToLower(id)
		if strings.Contains(name, "wechat") || strings.Contains(idLower, "wechat") {
			return id
		}
	}
	return ""
}

// buildAssistantSystemPrompt 构建固定的系统提示词（不随请求内容变化）
// 结构：人设 → 用户规则 → Agent 目录 → Skill 目录 → 长期记忆 → 时间/账号信息
func (b *Bridge) buildAssistantSystemPrompt(account string) (string, []PromptSection) {
	var sb strings.Builder
	var sections []PromptSection

	// 辅助函数：写入一个段并记录字符数
	writeSection := func(name, content string) {
		if content == "" {
			return
		}
		chars := len([]rune(content))
		sb.WriteString(content)
		sections = append(sections, PromptSection{Name: name, Chars: chars})
	}

	// 1. 人设
	var personaContent string
	if b.persona != nil {
		personaContent = b.persona.BuildSystemPrompt()
	} else {
		personaContent = loadWorkspaceFile(b.cfg.WorkspaceDir, "PERSONA.md", b.cfg.SystemPromptPrefix)
	}
	personaContent += "\n\n"

	now := time.Now()
	personaContent += fmt.Sprintf("account: %s\n", account)
	personaContent += fmt.Sprintf("当前时间: %s %s\n", now.Format("2006-01-02 15:04"), chineseWeekday(now.Weekday()))
	personaContent += fmt.Sprintf("当前输出token预算: %d tokens。使用 ExecuteCode 时注意控制 Python 代码长度，复杂逻辑拆分为多次调用，避免单次代码过长被截断导致语法错误。\n", b.cfg.LLM.MaxTokens)
	writeSection("人设/基础", personaContent)

	// 2. 用户规则
	if b.memoryMgr != nil {
		rulesBlock := b.memoryMgr.BuildRulePromptBlock()
		writeSection("用户规则", rulesBlock)
	}

	// 3. Agent 目录（简要列表 + get_agent_tools 获取详情的说明）
	agentBlock := b.buildAgentDirectory()
	writeSection("Agent目录", agentBlock)

	// 4. Skill 目录（简要列表 + get_skill_detail 获取详情的说明）
	if b.skillMgr != nil {
		catalog := b.skillMgr.BuildCatalogWithToolHint()
		writeSection("Skill目录", catalog)
	}

	// 5. 长期记忆
	if b.memoryMgr != nil {
		memoryBlock := b.memoryMgr.BuildPromptBlock()
		writeSection("长期记忆", memoryBlock)
	}

	return sb.String(), sections
}

// chineseWeekday 返回中文星期名称
func chineseWeekday(w time.Weekday) string {
	names := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return names[w]
}
