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
		// 续接对话：追加 user 消息
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
			systemPrompt, _ := b.buildAssistantSystemPrompt(payload.Account, payload.Query, b.getLLMTools(), nil)
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

// defaultTaskGuide 任务指引的默认内容（workspace/TASK_GUIDE.md 不存在时的 fallback）
var defaultTaskGuide = `
使用account:%s账户填充字段，不要向用户询问使用哪个字段了直接使用,account填充。
## 任务拆解能力
当你判断用户的请求确实需要多种不同类型的工具协作，且无法用一次 ExecuteCode 完成时，
你应该调用 plan_and_execute 工具来拆解和编排执行。

**任务处理流程：**

1. **优先尝试直接完成**
   - 单一工具调用：直接执行
   - 多个数据查询+分析：使用 ExecuteCode 编写 Python 代码，在代码内通过 call_tool() 批量调用工具并处理数据，一步完成
   - 只有 ExecuteCode 无法覆盖时，才考虑拆解

2. **确需拆解时调用 plan_and_execute**
   - 不同类型工具协作（如先编码再部署、先查数据再生成报告文件）
   - 多个完全独立的目标需要并行处理

**原则：** 能用一次 ExecuteCode 搞定的任务不要拆解。

不需要拆解的场景：
- 简单问答
- 单一工具调用
- 数据查询和分析（即使涉及多个数据源，ExecuteCode 内部可以批量调用）

适合拆解的场景：
- 先编码开发，再部署上线
- 同时处理多个完全不同类型的任务

## 错误处理

当工具调用失败时：
1. 分析错误原因，修正参数后重试
2. 如重试仍失败，尝试替代方案
3. 如无替代方案，向用户说明失败原因和建议

**ExecuteCode 错误的特殊规则：**
- ExecuteCode 返回 Python syntax error 或运行时错误时，**必须修正代码后再次调用 ExecuteCode**
- **严禁**因为代码报错就放弃沙箱，转而逐个直接调用工具
- 代码错误只需要修复代码本身，沙箱执行路径不可绕过
`

// buildAssistantSystemPrompt 构建 assistant 的系统提示（按 DisclosureLevel 条件注入各区块）
// policyResult 为 nil 时默认 LevelTwo（兼容旧调用）
// 返回 prompt 文本和各区块字符统计
func (b *Bridge) buildAssistantSystemPrompt(account, query string, tools []LLMTool, policyResult *PolicyResult) (string, []PromptSection) {
	// ??????? selectedSkills
	if policyResult == nil {
		pr := b.ApplyPolicyPipeline(query, tools)
		policyResult = &pr
		tools = pr.Tools
	}

	level := policyResult.Level
	var selectedSkills []SkillEntry
	selectedSkills = policyResult.SelectedSkills

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

	// 人设系统提示（始终保留）
	var personaContent string
	if b.persona != nil {
		personaContent = b.persona.BuildSystemPrompt()
	} else {
		personaContent = loadWorkspaceFile(b.cfg.WorkspaceDir, "PERSONA.md", b.cfg.SystemPromptPrefix)
	}
	personaContent += "\n\n"

	now := time.Now()
	today := now.Format("2006-01-02")
	personaContent += fmt.Sprintf("account: %s\n", account)
	personaContent += fmt.Sprintf("当前时间: %s %s\n", now.Format("2006-01-02 15:04"), chineseWeekday(now.Weekday()))
	personaContent += fmt.Sprintf("当前输出token预算: %d tokens。使用 ExecuteCode 时注意控制 Python 代码长度，复杂逻辑拆分为多次调用，避免单次代码过长被截断导致语法错误。\n", b.cfg.LLM.MaxTokens)
	writeSection("人设/基础", personaContent)

	// Agent 描述：LevelZero 跳过，LevelOne/LevelTwo 按工具过滤
	if level >= LevelOne {
		// 计算被 skill 接管的 agent 映射（agent_id → skill_name）
		skillAgents := make(map[string]string)
		if len(selectedSkills) > 0 {
			b.catalogMu.RLock()
			for _, skill := range selectedSkills {
				for _, toolName := range skill.Tools {
					if agentID, ok := b.toolCatalog[toolName]; ok {
						if _, already := skillAgents[agentID]; !already {
							skillAgents[agentID] = skill.Name
						}
					}
				}
			}
			b.catalogMu.RUnlock()
		}
		agentBlock := b.getFilteredAgentDescriptionBlock(tools, skillAgents)
		writeSection("Agent能力", agentBlock)
	}

	// Skill 目录（始终注入，提示通过 execute_skill 工具调用）
	if b.skillMgr != nil {
		catalog := b.skillMgr.BuildCatalogWithToolHint()
		writeSection("Skill目录", catalog)
	}

	// 长期记忆（始终保留）
	if b.memoryMgr != nil {
		memoryBlock := b.memoryMgr.BuildPromptBlock()
		writeSection("长期记忆", memoryBlock)
	}

	// 用户规则
	if b.memoryMgr != nil {
		rulesBlock := b.memoryMgr.BuildRulePromptBlock()
		writeSection("用户规则", rulesBlock)
	}

	// 任务拆解指引（始终保留）
	taskGuide := loadWorkspaceFile(b.cfg.WorkspaceDir, "TASK_GUIDE.md", defaultTaskGuide)
	writeSection("任务指引", "\n"+taskGuide)

	// 上下文数据：仅 LevelTwo 时按原逻辑注入
	if level == LevelTwo {
		needContextData := len(selectedSkills) == 0
		for _, s := range selectedSkills {
			if s.Name == "blog-data-opt" {
				needContextData = true
				break
			}
		}

		if needContextData {
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

			<-done
			<-done
			close(ch)

			var ctxParts []string
			for r := range ch {
				ctxParts = append(ctxParts, fmt.Sprintf("[%s]\n%s", r.label, r.data))
			}

			if len(ctxParts) > 0 {
				ctxContent := "\n用户当前数据:\n" + strings.Join(ctxParts, "\n\n")
				writeSection("上下文数据", ctxContent)
			}
		}
	}

	return sb.String(), sections
}

// chineseWeekday 返回中文星期名称
func chineseWeekday(w time.Weekday) string {
	names := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return names[w]
}
