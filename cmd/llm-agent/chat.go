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

// fmtDuration 格式化耗时为易读字符串
func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fmin", d.Minutes())
}

// ========================= WechatSink =========================

// WechatSink 微信实时进度推送 + 结果缓冲
type WechatSink struct {
	bridge        *Bridge
	fromAgent     string
	wechatUser    string
	buf           strings.Builder // 缓冲最终结果
	lastEventTime time.Time       // 节流：两次普通事件推送间隔至少 1 秒
}

func (s *WechatSink) OnChunk(text string) { s.buf.WriteString(text) }

// isImportantEvent 判断是否为重要事件（不受节流限制）
func isImportantEvent(event string) bool {
	switch event {
	case "plan_done", "plan_detail", "plan_review_start", "plan_review_result",
		"subtask_start", "subtask_done",
		"subtask_fail", "subtask_skip", "subtask_async", "subtask_defer",
		"subtask_thinking", "subtask_response",
		"tool_call", "tool_result", "tool_progress", "failure_decision",
		"task_complete", "task_cancelled", "task_forced_summary",
		"plan_timing", "review_timing",
		"synthesis_done",
		"subtask_timeout", "subtask_llm_error",
		"progress", "retry_detail", "modify_detail",
		"route_info",
		"skill_start", "skill_tool_call", "skill_tool_result", "skill_done":
		return true
	}
	return false
}

func (s *WechatSink) OnEvent(event, text string) {
	// 重要事件不受节流限制；普通事件间隔至少 1 秒
	if !isImportantEvent(event) && time.Since(s.lastEventTime) < 1*time.Second {
		return
	}

	var msg string
	switch event {
	case "thinking":
		msg = "🤔 " + text
	case "tool_info":
		msg = text
	case "plan_start":
		msg = "🔍 " + text
	case "plan_done":
		msg = "📋 " + text
	case "plan_detail":
		msg = text
	case "plan_review_start":
		msg = "🔍 " + text
	case "plan_review_result":
		msg = "✅ " + text
	case "subtask_start":
		msg = "▶ " + text
	case "subtask_done":
		msg = "✅ " + text
	case "subtask_fail":
		msg = "❌ " + text
	case "subtask_skip":
		msg = "⏭ " + text
	case "subtask_result":
		msg = "📄 " + text
	case "subtask_thinking":
		msg = "🤔 " + text
	case "subtask_response":
		msg = "💬 " + text
	case "tool_call":
		msg = "🔧 " + text
	case "tool_result":
		msg = text
	case "tool_progress":
		msg = text
	case "failure_decision":
		msg = "🔄 " + text
	case "synthesis":
		msg = "📝 " + text
	case "subtask_async":
		msg = "⏳ " + text
	case "subtask_defer":
		msg = "⏸ " + text
	case "task_complete":
		msg = "✅ " + text
	case "task_cancelled":
		msg = "🛑 " + text
	case "task_forced_summary":
		msg = "⚠ " + text
	case "plan_timing":
		msg = "⏱ " + text
	case "review_timing":
		msg = "⏱ " + text
	case "synthesis_done":
		msg = "📝 " + text
	case "subtask_timeout":
		msg = "⏰ " + text
	case "subtask_llm_error":
		msg = "💥 " + text
	case "progress":
		msg = "📊 " + text
	case "retry_detail":
		msg = "🔄 " + text
	case "modify_detail":
		msg = "✏ " + text
	case "route_info":
		msg = "🧭 " + text
	case "skill_start":
		msg = "🎯 " + text
	case "skill_tool_call":
		msg = "⚙ " + text
	case "skill_tool_result":
		msg = text
	case "skill_done":
		msg = "✅ " + text
	default:
		return
	}

	if err := s.bridge.client.SendTo(s.fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      s.wechatUser,
		Content: msg,
	}); err != nil {
		log.Printf("[WechatSink] send progress failed: %v", err)
	}
	s.lastEventTime = time.Now()
}

func (s *WechatSink) Streaming() bool { return false }
func (s *WechatSink) Result() string  { return s.buf.String() }

// ========================= 命令识别 =========================

// isConversationResetCommand 判断是否为对话重置命令
func isConversationResetCommand(content string) bool {
	content = strings.TrimSpace(content)
	resetCommands := []string{"/reset", "新对话", "重新开始", "清除上下文", "reset", "new chat"}
	for _, cmd := range resetCommands {
		if strings.EqualFold(content, cmd) {
			return true
		}
	}
	return false
}

// isContextCommand 判断是否为上下文查看命令
func isContextCommand(content string) bool {
	content = strings.TrimSpace(content)
	cmds := []string{"/context", "上下文", "context"}
	for _, cmd := range cmds {
		if strings.EqualFold(content, cmd) {
			return true
		}
	}
	return false
}

// isStopCommand 判断是否为停止任务命令
func isStopCommand(content string) bool {
	content = strings.TrimSpace(content)
	stopCommands := []string{"停止", "取消", "stop", "cancel"}
	for _, cmd := range stopCommands {
		if strings.EqualFold(content, cmd) {
			return true
		}
	}
	return false
}

// ========================= 微信消息处理 =========================

// handleWechatMessage 处理微信消息：维护对话上下文 → processTask → 回复
// 使用通用 ChatSessionManager 管理会话
func (b *Bridge) handleWechatMessage(fromAgent, wechatUser, content string) {
	log.Printf("[Wechat] from=%s user=%s content=%s", fromAgent, wechatUser, content)

	// 0. Claude Mode 命令: /claude 进入
	if isClaudeCommand(content) {
		b.handleClaudeCommand(fromAgent, wechatUser, content)
		return
	}

	// 0.1 Claude Mode 内置命令: cc exit / cc stop / cc plan / cc code
	if cmd, ok := isClaudeModeCommand(content); ok {
		session := b.sessionMgr.Get("wechat", wechatUser)
		if session != nil && session.ClaudeMode {
			switch cmd {
			case "exit":
				b.exitClaudeMode(session, fromAgent, wechatUser)
			case "stop":
				b.stopClaudeSession(session, fromAgent, wechatUser)
			case "plan":
				b.handleModeSwitch(session, fromAgent, wechatUser, "plan")
			case "code":
				b.handleModeSwitch(session, fromAgent, wechatUser, "code")
			case "status":
				b.handleClaudeStatus(session, fromAgent, wechatUser)
			case "model":
				b.handleClaudeModel(session, fromAgent, wechatUser)
			case "verbose":
				b.handleVerbositySwitch(session, fromAgent, wechatUser, 2)
			case "brief":
				b.handleVerbositySwitch(session, fromAgent, wechatUser, 0)
			case "normal":
				b.handleVerbositySwitch(session, fromAgent, wechatUser, 1)
			}
			return
		}
		// 不在 Claude Mode 中，当做普通消息继续
	}

	// 0.2 Claude Mode 消息路由: 权限回复或转发 prompt
	existingSession := b.sessionMgr.Get("wechat", wechatUser)
	if existingSession != nil && existingSession.ClaudeMode {
		if existingSession.HasPendingPermission() {
			// 有待处理的权限请求 → 当做权限回复
			b.handlePermissionReply(existingSession, fromAgent, wechatUser, content)
		} else {
			// 无 pending → 当做新 prompt 发给 Claude
			go b.handleClaudeModeMessage(existingSession, fromAgent, wechatUser, content)
		}
		return
	}

	// 1. 检查是否为重置命令
	if isConversationResetCommand(content) {
		b.sessionMgr.Reset("wechat", wechatUser)
		b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
			Channel: "wechat",
			To:      wechatUser,
			Content: "已开始新对话。",
		})
		log.Printf("[Wechat] conversation reset for user=%s", wechatUser)
		return
	}

	// 2. 停止命令检查（在 processing.Lock 之前！不需要等锁）
	if isStopCommand(content) {
		session, _ := b.sessionMgr.GetOrCreate("wechat", wechatUser, wechatUser)
		if session.CancelRunning() {
			b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
				Channel: "wechat",
				To:      wechatUser,
				Content: "已停止当前任务。",
			})
			log.Printf("[Wechat] task cancelled for user=%s", wechatUser)
		} else {
			b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
				Channel: "wechat",
				To:      wechatUser,
				Content: "当前没有正在执行的任务。",
			})
			log.Printf("[Wechat] no running task to cancel for user=%s", wechatUser)
		}
		return
	}

	// 3. 上下文查看命令
	if isContextCommand(content) {
		reply := b.buildContextDebugInfo("wechat", wechatUser)
		b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
			Channel: "wechat",
			To:      wechatUser,
			Content: reply,
		})
		return
	}

	// 4. 获取或创建会话
	session, isNew := b.sessionMgr.GetOrCreate("wechat", wechatUser, wechatUser)

	// 序列化同一用户的消息处理（后到的消息等前一个完成）
	session.processing.Lock()
	defer session.processing.Unlock()

	// 即时反馈：区分新/续会话
	var feedbackMsg string
	if isNew {
		feedbackMsg = "⏳ 收到消息，开始新对话..."
	} else {
		session.mu.Lock()
		turnNum := session.TurnCount + 1
		session.mu.Unlock()
		feedbackMsg = fmt.Sprintf("⏳ 收到消息，继续对话（第%d轮）...\n发送「新对话」可清空上下文", turnNum)
	}
	b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatUser,
		Content: feedbackMsg,
	})

	// 4. 构建/追加消息
	session.mu.Lock()
	session.LastActiveAt = time.Now()

	if isNew || len(session.Messages) == 0 {
		// 新会话：构建 system prompt + 第一条 user 消息
		systemPrompt, promptSections := b.buildAssistantSystemPrompt(wechatUser)
		// 注入微信用户 ID（用于 LLM 创建定时任务时传入正确的 wechat_user）
		systemPrompt += fmt.Sprintf("\n当前微信用户ID(wechat_user): %s\n", wechatUser)
		session.Messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}
		session.PromptSections = promptSections
		log.Printf("[Wechat] 新会话 sessionID=%s user=%s", session.SessionID, wechatUser)
	} else {
		// 续接对话：刷新 system prompt（反映最新工具和 agent 状态）+ 追加 user 消息
		if len(session.Messages) > 0 && session.Messages[0].Role == "system" {
			freshPrompt, promptSections := b.buildAssistantSystemPrompt(wechatUser)
			freshPrompt += fmt.Sprintf("\n当前微信用户ID(wechat_user): %s\n", wechatUser)
			session.Messages[0].Content = freshPrompt
			session.PromptSections = promptSections
		}
		session.Messages = append(session.Messages, Message{Role: "user", Content: content})
		log.Printf("[Wechat] 续接会话 sessionID=%s user=%s turn=%d msgCount=%d (system prompt已刷新)",
			session.SessionID, wechatUser, session.TurnCount, len(session.Messages))
	}

	// 5. 上下文压缩
	session.Messages = CompactMessages(session.Messages, b.sessionMgr.maxMessages)

	// 6. 复制消息快照（避免 processTask 执行期间被修改）
	messagesCopy := make([]Message, len(session.Messages))
	copy(messagesCopy, session.Messages)

	taskID := fmt.Sprintf("%s_%d", session.SessionID, session.TurnCount)
	session.TurnCount++
	session.mu.Unlock()

	// 7. 构建 TaskContext（传入完整对话历史）
	sink := &WechatSink{
		bridge:     b,
		fromAgent:  fromAgent,
		wechatUser: wechatUser,
	}

	// 创建可取消 context
	goctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session.SetCancel(cancel)
	defer session.SetCancel(nil) // 任务结束后清除

	ctx := &TaskContext{
		Ctx:      goctx,
		TaskID:   taskID,
		Account:  wechatUser,
		Query:    content,
		Source:   "wechat",
		Messages: messagesCopy,
		Sink:     sink,
	}

	taskStart := time.Now()
	result, _ := b.processTask(ctx)
	taskDuration := time.Since(taskStart)

	// 任务被取消时，不发送后续事件和结果
	if goctx.Err() != nil {
		log.Printf("[Wechat] task cancelled, skip sending result user=%s taskID=%s", wechatUser, taskID)
		return
	}

	// 发送完成耗时事件
	sink.OnEvent("task_complete", fmt.Sprintf("处理完成，耗时 %s", fmtDuration(taskDuration)))

	if result == "" {
		result = "抱歉，未能生成回复。"
	}

	// 8. 将 assistant 回复追加到对话历史
	session.mu.Lock()
	session.Messages = append(session.Messages, Message{Role: "assistant", Content: result})
	session.mu.Unlock()

	// 持久化会话
	if err := b.sessionMgr.SaveSession(session); err != nil {
		log.Printf("[Wechat] save session failed: %v", err)
	}

	// 9. 发送结果
	err := b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatUser,
		Content: result,
	})
	if err != nil {
		log.Printf("[Wechat] send reply failed: %v", err)
	} else {
		log.Printf("[Wechat] reply sent to %s via %s (%d chars)", wechatUser, fromAgent, len(result))
	}
}

// buildContextDebugInfo 构建当前 session 的上下文结构概览
func (b *Bridge) buildContextDebugInfo(source, userID string) string {
	session := b.sessionMgr.Get(source, userID)
	if session == nil {
		return "当前无活跃会话。发送任意消息开始新对话。"
	}

	session.mu.Lock()
	sessionID := session.SessionID
	lastActive := session.LastActiveAt
	turnCount := session.TurnCount
	maxTurns := b.sessionMgr.maxTurns
	msgs := make([]Message, len(session.Messages))
	copy(msgs, session.Messages)
	promptSections := make([]PromptSection, len(session.PromptSections))
	copy(promptSections, session.PromptSections)
	session.mu.Unlock()

	var sb strings.Builder

	// 基本信息
	sb.WriteString("📋 Session Context Debug\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("🔑 会话: %s_%s (%s)\n", source, userID, sessionID))
	sb.WriteString(fmt.Sprintf("⏱ 活跃: %s | 轮次: %d/%d\n", lastActive.Format("2006-01-02 15:04"), turnCount, maxTurns))
	sb.WriteString(fmt.Sprintf("📨 消息数: %d\n", len(msgs)))

	// 当前 LLM 提供商和模型
	provider, modelKey, modelID := b.activeLLM.GetInfo()
	cfg := b.activeLLM.Get()
	sb.WriteString(fmt.Sprintf("\n🤖 LLM: %s/%s (%s)\n", provider, modelKey, modelID))
	sb.WriteString(fmt.Sprintf("  max_tokens=%d temperature=%.2f\n", cfg.MaxTokens, cfg.Temperature))
	if sc, ok := b.sourceLLMs[source]; ok {
		sb.WriteString(fmt.Sprintf("  渠道覆盖: %s/%s\n", sc.LLM.Provider, sc.LLM.Model))
	}

	// System Prompt 各层字符统计（使用构建时记录的数据）
	if len(msgs) > 0 && msgs[0].Role == "system" {
		sysChars := len([]rune(msgs[0].Content))
		sb.WriteString(fmt.Sprintf("\n📝 System Prompt (%s chars):\n", formatTokenCount(int64(sysChars))))

		if len(promptSections) > 0 {
			for _, sec := range promptSections {
				sb.WriteString(fmt.Sprintf("  · %s: %d chars\n", sec.Name, sec.Chars))
			}
		} else {
			sb.WriteString(fmt.Sprintf("  · 全部: %d chars (无分段数据)\n", sysChars))
		}
	}

	// 消息历史
	sb.WriteString("\n💬 消息历史:\n")
	for i, msg := range msgs {
		charCount := len([]rune(msg.Content))
		extra := ""
		if len(msg.ToolCalls) > 0 {
			var toolNames []string
			for _, tc := range msg.ToolCalls {
				toolNames = append(toolNames, tc.Function.Name)
			}
			extra = fmt.Sprintf(" (tools: %s)", strings.Join(toolNames, ","))
		}
		if msg.ToolCallID != "" {
			extra = " (tool_result)"
		}
		sb.WriteString(fmt.Sprintf("  [%d] %s: %d chars%s\n", i, msg.Role, charCount, extra))
	}

	// 工具统计（按 agent 分组）
	b.catalogMu.RLock()
	toolCount := len(b.toolCatalog)
	agentToolCount := make(map[string][]string) // agentID → tool names
	for toolName, agentID := range b.toolCatalog {
		agentToolCount[agentID] = append(agentToolCount[agentID], toolName)
	}
	b.catalogMu.RUnlock()

	sb.WriteString(fmt.Sprintf("\n🔧 工具: %d 个\n", toolCount))
	for agentID, tools := range agentToolCount {
		if len(tools) <= 3 {
			sb.WriteString(fmt.Sprintf("  %s(%d): %s\n", agentID, len(tools), strings.Join(tools, ", ")))
		} else {
			sb.WriteString(fmt.Sprintf("  %s(%d): %s, ...\n", agentID, len(tools), strings.Join(tools[:3], ", ")))
		}
	}

	// Token 统计
	if globalTokenStats != nil {
		if tokenSummary := globalTokenStats.Summary(); tokenSummary != "" {
			sb.WriteString("\n")
			sb.WriteString(tokenSummary)
			sb.WriteString("\n")
		}
	}

	// 总字符数
	totalChars := 0
	for _, msg := range msgs {
		totalChars += len([]rune(msg.Content))
	}
	sb.WriteString(fmt.Sprintf("\n💾 总字符数: %s chars (预算: 120,000)", formatTokenCount(int64(totalChars))))

	return sb.String()
}

// callToolWithTimeout 带超时的工具调用
func (b *Bridge) callToolWithTimeout(toolName string, args json.RawMessage, timeout time.Duration) (string, error) {
	if _, ok := b.getToolAgent(toolName); !ok {
		return "", fmt.Errorf("tool %s not in catalog", toolName)
	}

	type result struct {
		data string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		tcResult, err := b.CallTool(toolName, args)
		var data string
		if tcResult != nil {
			data = tcResult.Result
		}
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout after %v", timeout)
	}
}
