package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"uap"
)

// AppSink App 实时进度推送 + 结果缓冲
type AppSink struct {
	bridge        *Bridge
	fromAgent     string
	appUser       string
	buf           strings.Builder
	lastEventTime time.Time
}

func (s *AppSink) OnChunk(text string) { s.buf.WriteString(text) }

func (s *AppSink) OnEvent(event, text string) {
	if !isImportantEvent(event) && time.Since(s.lastEventTime) < 1*time.Second {
		return
	}

	var msg string
	switch event {
	case "thinking":
		msg = "思考: " + text
	case "tool_info", "plan_detail", "tool_result", "tool_progress", "skill_tool_result":
		msg = text
	case "plan_start":
		msg = "计划开始: " + text
	case "plan_done":
		msg = "计划完成: " + text
	case "plan_review_start":
		msg = "计划审查: " + text
	case "plan_review_result":
		msg = "审查结果: " + text
	case "subtask_start":
		msg = "子任务开始: " + text
	case "subtask_done":
		msg = "子任务完成: " + text
	case "subtask_fail":
		msg = "子任务失败: " + text
	case "subtask_skip":
		msg = "子任务跳过: " + text
	case "subtask_result":
		msg = "子任务结果: " + text
	case "subtask_thinking":
		msg = "子任务思考: " + text
	case "subtask_response":
		msg = "子任务回复: " + text
	case "tool_call":
		msg = "工具调用: " + text
	case "failure_decision":
		msg = "降级决策: " + text
	case "synthesis":
		msg = "结果整理: " + text
	case "subtask_async":
		msg = "异步子任务: " + text
	case "subtask_defer":
		msg = "延后处理: " + text
	case "task_complete":
		msg = "任务完成: " + text
	case "task_cancelled":
		msg = "任务取消: " + text
	case "task_forced_summary":
		msg = "强制总结: " + text
	case "plan_timing", "review_timing":
		msg = "耗时: " + text
	case "synthesis_done":
		msg = "整理完成: " + text
	case "subtask_timeout":
		msg = "子任务超时: " + text
	case "subtask_llm_error":
		msg = "模型错误: " + text
	case "progress":
		msg = "进度: " + text
	case "retry_detail":
		msg = "重试: " + text
	case "modify_detail":
		msg = "修改: " + text
	case "route_info":
		msg = "路由: " + text
	case "skill_start":
		msg = "技能开始: " + text
	case "skill_tool_call":
		msg = "技能工具调用: " + text
	case "skill_done":
		msg = "技能完成: " + text
	default:
		return
	}

	if err := s.bridge.client.SendTo(s.fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "app",
		To:      s.appUser,
		Content: msg,
	}); err != nil {
		log.Printf("[AppSink] send progress failed: %v", err)
	}
	s.lastEventTime = time.Now()
}

func (s *AppSink) Streaming() bool { return false }
func (s *AppSink) Result() string  { return s.buf.String() }

func (b *Bridge) handleAppMessage(fromAgent, appUser, content string) {
	log.Printf("[App] from=%s user=%s content=%s", fromAgent, appUser, content)

	// 确保账户 workspace 目录存在（多账户支持）
	if b.cfg.WorkspaceDir != "" {
		EnsureAccountWorkspace(b.cfg.WorkspaceDir, appUser)
	}

	if isConversationResetCommand(content) {
		b.sessionMgr.Reset("app", appUser)
		b.sendApp(fromAgent, appUser, "已开始新对话。")
		log.Printf("[App] conversation reset for user=%s", appUser)
		return
	}

	if isStopCommand(content) {
		session, _ := b.sessionMgr.GetOrCreate("app", appUser, appUser)
		if session.CancelRunning() {
			b.sendApp(fromAgent, appUser, "已停止当前任务。")
			log.Printf("[App] task cancelled for user=%s", appUser)
		} else {
			b.sendApp(fromAgent, appUser, "当前没有正在执行的任务。")
			log.Printf("[App] no running task to cancel for user=%s", appUser)
		}
		return
	}

	if isContextCommand(content) {
		reply := b.buildContextDebugInfo("app", appUser)
		b.sendApp(fromAgent, appUser, reply)
		return
	}

	session, isNew := b.sessionMgr.GetOrCreate("app", appUser, appUser)

	session.processing.Lock()
	defer session.processing.Unlock()

	feedbackMsg := "收到消息，开始处理..."
	if !isNew {
		session.mu.Lock()
		turnNum := session.TurnCount + 1
		session.mu.Unlock()
		feedbackMsg = fmt.Sprintf("收到消息，继续对话（第%d轮）...\n发送“新对话”或 /reset 可清空上下文。", turnNum)
	}
	b.sendApp(fromAgent, appUser, feedbackMsg)

	session.mu.Lock()
	session.LastActiveAt = time.Now()

	if isNew || len(session.Messages) == 0 {
		systemPrompt, promptSections := b.buildAssistantSystemPrompt(appUser)
		systemPrompt += fmt.Sprintf("\n当前App用户ID(app_user): %s\n", appUser)
		session.Messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}
		session.PromptSections = promptSections
		log.Printf("[App] 新会话 sessionID=%s user=%s", session.SessionID, appUser)
	} else {
		if len(session.Messages) > 0 && session.Messages[0].Role == "system" {
			freshPrompt, promptSections := b.buildAssistantSystemPrompt(appUser)
			freshPrompt += fmt.Sprintf("\n当前App用户ID(app_user): %s\n", appUser)
			session.Messages[0].Content = freshPrompt
			session.PromptSections = promptSections
		}
		session.Messages = append(session.Messages, Message{Role: "user", Content: content})
		log.Printf("[App] 续接会话 sessionID=%s user=%s turn=%d msgCount=%d", session.SessionID, appUser, session.TurnCount, len(session.Messages))
	}

	session.Messages = CompactMessages(session.Messages, b.sessionMgr.maxMessages)

	messagesCopy := make([]Message, len(session.Messages))
	copy(messagesCopy, session.Messages)

	taskID := fmt.Sprintf("%s_%d", session.SessionID, session.TurnCount)
	session.TurnCount++
	session.mu.Unlock()

	sink := &AppSink{
		bridge:    b,
		fromAgent: fromAgent,
		appUser:   appUser,
	}

	goctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session.SetCancel(cancel)
	defer session.SetCancel(nil)

	ctx := &TaskContext{
		Ctx:      goctx,
		TaskID:   taskID,
		Account:  appUser,
		Query:    content,
		Source:   "app",
		Messages: messagesCopy,
		Sink:     sink,
	}

	taskStart := time.Now()
	result, _ := b.processTask(ctx)
	taskDuration := time.Since(taskStart)

	if goctx.Err() != nil {
		log.Printf("[App] task cancelled, skip sending result user=%s taskID=%s", appUser, taskID)
		return
	}

	sink.OnEvent("task_complete", fmt.Sprintf("处理完成，耗时 %s", fmtDuration(taskDuration)))

	if result == "" {
		result = "抱歉，未能生成回复。"
	}

	session.mu.Lock()
	session.Messages = append(session.Messages, Message{Role: "assistant", Content: result})
	session.mu.Unlock()

	if err := b.sessionMgr.SaveSession(session); err != nil {
		log.Printf("[App] save session failed: %v", err)
	}

	const maxAppSize = 200 * 1024
	appResult := result
	if len(result) > maxAppSize {
		appResult = result[:maxAppSize] + "\n\n...(回复内容过长已截断，完整内容已保存在对话历史中)"
		log.Printf("[App] result truncated: %d -> %d chars", len(result), len(appResult))
	}

	if err := b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "app",
		To:      appUser,
		Content: appResult,
	}); err != nil {
		log.Printf("[App] send reply failed: %v", err)
	} else {
		log.Printf("[App] reply sent to %s via %s (%d chars)", appUser, fromAgent, len(result))
	}
}

func (b *Bridge) sendApp(fromAgent, appUser, content string) {
	const maxAppSize = 200 * 1024
	if len(content) > maxAppSize {
		content = content[:maxAppSize] + "\n\n...(内容过长已截断)"
		log.Printf("[App] content truncated for user=%s", appUser)
	}
	b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "app",
		To:      appUser,
		Content: content,
	})
}
