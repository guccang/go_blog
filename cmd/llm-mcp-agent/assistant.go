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

// defaultTaskGuide 任务指引的默认内容（workspace/TASK_GUIDE.md 不存在时的 fallback）
var defaultTaskGuide = `
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

## Code Execution 模式（优先使用）

当任务需要调用 2 个及以上工具时，**必须优先使用 ExecuteCode 工具**，而不是逐个调用工具。

**使用方式：** 调用 ExecuteCode，传入 Python 代码，代码中通过 call_tool(name, args) 调用 MCP 工具。

**示例 — 获取本周运动数据并汇总：**
` + "`" + "`" + "`" + `python
import json
# 获取当前日期
date_info = call_tool("RawCurrentDate", {})
# 获取本周运动数据
exercise = call_tool("RawGetExerciseRange", {"account": "xxx", "start": "2025-03-03", "end": "2025-03-09"})
# 获取待办
todos = call_tool("RawGetTodosByDate", {"account": "xxx", "date": "2025-03-09"})
# 在 Python 中处理数据，只输出最终结果
print(f"日期: {date_info}")
print(f"运动数据: {exercise}")
print(f"待办: {todos}")
` + "`" + "`" + "`" + `

**关键规则：**
1. 只有 print() 的内容会返回给你，中间变量不会占用上下文
2. call_tool 失败会抛异常；用 safe_call_tool(name, args, default) 可在失败时返回默认值
3. 循环批量操作特别适合 ExecuteCode（如遍历日期范围逐天查询）
4. 单一工具调用（如只调用 RawCurrentDate）可以直接调用，不需要 ExecuteCode
5. **工具返回值类型不确定**：call_tool 返回值可能是 str（纯文本/markdown）或 dict/list（结构化数据），**绝对不要假设返回类型**。正确做法：先 print(type(result), result[:200]) 查看格式，或者直接 print(result) 输出原始内容让我来分析
6. **禁止对字符串调用 .get()/.items() 等 dict 方法**，必须先用 isinstance(result, dict) 判断类型
7. **ExecuteCode 失败必须修正重试**：当 ExecuteCode 返回 Python syntax error 或运行时错误时，**必须分析错误原因、修正 Python 代码后再次调用 ExecuteCode**。严禁因为代码报错就放弃沙箱执行，转而逐个直接调用工具或用其他方式绕过。沙箱是数据处理的正确路径，代码错误只需要修复代码本身

**何时直接调用工具（不用 ExecuteCode）：**
- 只需要调用 1 个工具
- CodegenStartSession / CodegenSendMessage（编码会话工具）
- DeployProject / DeployPipeline（部署工具）

## 编码消息传递规则（最高优先级）

当用户消息包含编码需求时，**必须将用户的原始消息原文直接作为 prompt 传给 CodegenStartSession**，严禁修改、缩写、翻译或重新措辞。

编码 agent（Claude Code）具备完整的理解和执行能力，你不需要替它做任何预处理。

**正确做法：**
- 用户说："写一个helloworld网页" → prompt="写一个helloworld网页"
- 用户说："重构登录模块，把密码改成bcrypt加密" → prompt="重构登录模块，把密码改成bcrypt加密"

**错误做法：**
- ❌ 把用户消息改写成更详细的技术方案后传入
- ❌ 把"编码"两字去掉后传入
- ❌ 把用户消息拆成多段分别调用

**混合任务（如"编码XX然后部署到YY"）的处理：**
- 第一步：调用 CodegenStartSession，prompt = 用户消息中的编码部分原文
- 第二步：编码完成后，调用 DeployProject 部署
- 关键：编码和部署是两个独立的工具调用，不要把部署指令混入编码 prompt
`

// buildAssistantSystemPrompt 构建 assistant 的系统提示（含任务拆解指引）
func (b *Bridge) buildAssistantSystemPrompt(account string) string {
	var sb strings.Builder

	persona := loadWorkspaceFile(b.cfg.WorkspaceDir, "PERSONA.md", b.cfg.SystemPromptPrefix)
	sb.WriteString(persona)
	sb.WriteString("\n\n")

	today := time.Now().Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("account: %s\n", account))
	sb.WriteString(fmt.Sprintf("当前日期: %s\n", today))

	// 注入 agent 能力描述
	agentBlock := b.getAgentDescriptionBlock()
	if agentBlock != "" {
		sb.WriteString(agentBlock)
	}

	// 任务拆解指引
	taskGuide := loadWorkspaceFile(b.cfg.WorkspaceDir, "TASK_GUIDE.md", defaultTaskGuide)
	sb.WriteString("\n")
	sb.WriteString(taskGuide)

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
