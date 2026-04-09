package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	audioSent     bool
	pendingAudio  string
}

func (s *WechatSink) OnChunk(text string) { s.buf.WriteString(text) }

// isImportantEvent 判断是否为重要事件（不受节流限制）
func isImportantEvent(event string) bool {
	switch event {
	case "subtask_start", "subtask_done",
		"subtask_fail", "subtask_skip", "subtask_async", "subtask_defer",
		"subtask_thinking", "subtask_response",
		"tool_call", "tool_result", "tool_progress",
		"task_complete", "task_cancelled", "task_forced_summary",
		"subtask_timeout", "subtask_llm_error",
		"progress", "retry_detail", "modify_detail",
		"route_info",
		"skill_start", "skill_tool_call", "skill_tool_result", "skill_done":
		return true
	}
	return false
}

func (s *WechatSink) OnEvent(event, text string) {
	if event == "audio_reply" {
		s.pendingAudio = text
		return
	}

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

	// 确保账户 workspace 目录存在（多账户支持）
	if b.cfg.WorkspaceDir != "" {
		EnsureAccountWorkspace(b.cfg.WorkspaceDir, wechatUser)
	}

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
			case "help":
				b.handleClaudeHelp(session, fromAgent, wechatUser)
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
	if isProjectManagementHelpCommand(content) {
		b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
			Channel: "wechat",
			To:      wechatUser,
			Content: buildProjectManagementHelp(),
		})
		return
	}

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
		systemPrompt, promptSections := b.buildAssistantSystemPromptForQuery(wechatUser, content, true)
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
			freshPrompt, promptSections := b.buildAssistantSystemPromptForQuery(wechatUser, content, true)
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

	cleanResult, leakedToolCalls := extractLegacyToolCallBlocks(result)
	if len(leakedToolCalls) > 0 {
		var toolNames []string
		for _, tc := range leakedToolCalls {
			toolNames = append(toolNames, b.resolveToolName(tc.Function.Name))
		}
		log.Printf("[Wechat] stripped leaked textual tool calls user=%s tools=%v", wechatUser, toolNames)
		result = cleanResult
		if !sink.AudioSent() {
			sink.trySendAudioReplyFromToolCalls(leakedToolCalls)
		}
	}

	if strings.TrimSpace(result) == "" {
		if sink.AudioSent() {
			result = "[voice reply sent]"
		} else {
			result = "抱歉，未能生成回复。"
		}
	}

	if !sink.AudioSent() && strings.TrimSpace(sink.pendingAudio) != "" && sink.trySendAudioReply(sink.pendingAudio, result) {
		log.Printf("[Wechat] sent pending audio reply for %s", wechatUser)
	}

	// 8. 将 assistant 回复追加到对话历史
	assistantContent := persistedAssistantContent(ctx, result)
	session.mu.Lock()
	session.Messages = append(session.Messages, Message{Role: "assistant", Content: assistantContent})
	session.mu.Unlock()

	// 持久化会话
	if err := b.sessionMgr.SaveSession(session); err != nil {
		log.Printf("[Wechat] save session failed: %v", err)
	}

	if !sink.AudioSent() && sink.trySendInlineAudioFromText(result) {
		log.Printf("[Wechat] extracted inline audio reply for %s, skip text reply", wechatUser)
		return
	}

	if sink.AudioSent() {
		log.Printf("[Wechat] audio reply already sent to %s, skip duplicate text reply", wechatUser)
		return
	}

	// 9. 截断过长内容（企业微信应用消息限制 256KB）
	const maxWechatSize = 200 * 1024 // 200KB 安全边界
	wechatResult := result
	if len(result) > maxWechatSize {
		wechatResult = result[:maxWechatSize] + "\n\n...(回复内容过长已截断，完整内容已保存在对话历史中)"
		log.Printf("[Wechat] result truncated: %d -> %d chars", len(result), len(wechatResult))
	}

	// 10. 发送结果
	err := b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatUser,
		Content: wechatResult,
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
	account := session.Account
	lastActive := session.LastActiveAt
	turnCount := session.TurnCount
	maxTurns := b.sessionMgr.maxTurns
	msgs := make([]Message, len(session.Messages))
	copy(msgs, session.Messages)
	promptSections := make([]PromptSection, len(session.PromptSections))
	copy(promptSections, session.PromptSections)
	claudeMode := session.ClaudeMode
	claudeProject := session.ClaudeProject
	claudeCurrentMode := session.ClaudeCurrentMode
	session.mu.Unlock()

	var sb strings.Builder

	totalChars := 0
	for _, msg := range msgs {
		totalChars += len([]rune(msg.Content))
	}

	sb.WriteString("📋 Context\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("会话: %s/%s\n", source, userID))
	sb.WriteString(fmt.Sprintf("chat_session: %s\n", sessionID))
	if account != "" {
		sb.WriteString(fmt.Sprintf("account: %s\n", account))
	}
	sb.WriteString(fmt.Sprintf("活跃: %s | 轮次: %d/%d\n", lastActive.Format("2006-01-02 15:04"), turnCount, maxTurns))
	sb.WriteString(fmt.Sprintf("消息: %d | 总字符: %s\n", len(msgs), formatTokenCount(int64(totalChars))))
	if claudeMode {
		mode := "claude"
		if strings.TrimSpace(claudeCurrentMode) != "" {
			mode = "claude/" + strings.TrimSpace(claudeCurrentMode)
		}
		if strings.TrimSpace(claudeProject) != "" {
			sb.WriteString(fmt.Sprintf("模式: %s (%s)\n", mode, claudeProject))
		} else {
			sb.WriteString(fmt.Sprintf("模式: %s\n", mode))
		}
	} else {
		sb.WriteString("模式: normal chat\n")
	}

	if b.activeLLM != nil {
		provider, modelKey, modelID := b.activeLLM.GetInfo()
		cfg := b.activeLLM.Get()
		sb.WriteString(fmt.Sprintf("\n🤖 LLM: %s/%s (%s)\n", provider, modelKey, modelID))
		sb.WriteString(fmt.Sprintf("  max_tokens=%d temperature=%.2f\n", cfg.MaxTokens, cfg.Temperature))
		if sc, ok := b.sourceLLMs[source]; ok {
			sb.WriteString(fmt.Sprintf("  渠道覆盖: %s/%s\n", sc.LLM.Provider, sc.LLM.Model))
		}
	}

	if len(msgs) > 0 && msgs[0].Role == "system" {
		sysChars := len([]rune(msgs[0].Content))
		sb.WriteString(fmt.Sprintf("\n📝 对话级 System Prompt: %s chars\n", formatTokenCount(int64(sysChars))))
		if len(promptSections) > 0 {
			for _, sec := range trimPromptSections(promptSections, 6) {
				sb.WriteString(fmt.Sprintf("  · %s: %d chars\n", sec.Name, sec.Chars))
			}
			if len(promptSections) > 6 {
				sb.WriteString(fmt.Sprintf("  · 其余: %d sections\n", len(promptSections)-6))
			}
		} else {
			sb.WriteString(fmt.Sprintf("  · 全部: %d chars (无分段数据)\n", sysChars))
		}
	}

	runtimeInfo := b.loadLatestTaskContextDebug(sessionID)
	if runtimeInfo != nil {
		sb.WriteString("\n🧠 最新任务运行时:\n")
		sb.WriteString(fmt.Sprintf("  root: %s [%s]\n", runtimeInfo.RootID, fallbackText(strings.TrimSpace(runtimeInfo.RootSession.Status), "unknown")))
		sb.WriteString(fmt.Sprintf("  query: %s\n", truncate(runtimeInfo.Query, 120)))
		sb.WriteString(fmt.Sprintf("  transcript: %d msgs | tool_calls: %d | child_sessions: %s\n",
			len(runtimeInfo.RootSession.Messages),
			len(runtimeInfo.RootSession.ToolCalls),
			summarizeChildSessionStatuses(runtimeInfo.ChildSessions),
		))
		sb.WriteString(fmt.Sprintf("  runtime: snapshot=%s | attachments=%s | compact=%d | mailbox(root=%d,total=%d)\n",
			boolWord(runtimeInfo.Snapshot != nil),
			summarizeAttachmentKinds(runtimeInfo.Attachments),
			len(runtimeInfo.CompactHistory),
			runtimeInfo.RootMailboxPending,
			runtimeInfo.TotalMailboxPending,
		))
		if len(runtimeInfo.PendingToolCalls) > 0 {
			sb.WriteString(fmt.Sprintf("  pending_tool_calls: %s\n", strings.Join(runtimeInfo.PendingToolCalls, ", ")))
		}
		if compactSummary := summarizeLatestCompaction(runtimeInfo.CompactHistory); compactSummary != "" {
			sb.WriteString(fmt.Sprintf("  最近压缩: %s\n", compactSummary))
		}
		if preview := summarizeLatestAssistantRecord(runtimeInfo.RootSession.Messages); preview != "" {
			sb.WriteString(fmt.Sprintf("  最近 assistant: %s\n", preview))
		}
		if promptSummary := summarizeRuntimePrompt(runtimeInfo.PromptContext); promptSummary != "" {
			sb.WriteString(fmt.Sprintf("  prompt: %s\n", promptSummary))
		}
		taskPreviews := previewContextMessages(runtimeInfo.RootSession.Messages, 4)
		if len(taskPreviews) > 0 {
			sb.WriteString("\n🧵 最近任务消息:\n")
			for _, line := range taskPreviews {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("\n🧠 最新任务运行时: 暂无持久化任务记录\n")
	}

	chatPreviews := previewContextMessages(msgs, 4)
	if len(chatPreviews) > 0 {
		sb.WriteString("\n💬 最近对话消息:\n")
		for _, line := range chatPreviews {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

type contextTaskDebug struct {
	RootID              string
	Query               string
	RootSession         *TaskSession
	Snapshot            *RuntimeSnapshot
	PromptContext       SystemPromptContext
	Attachments         []Attachment
	CompactHistory      []RuntimeCompactMetadata
	ChildSessions       []*TaskSession
	RootMailboxPending  int
	TotalMailboxPending int
	PendingToolCalls    []string
}

func (b *Bridge) loadLatestTaskContextDebug(chatSessionID string) *contextTaskDebug {
	if b == nil || b.cfg == nil || strings.TrimSpace(b.cfg.SessionDir) == "" || strings.TrimSpace(chatSessionID) == "" {
		return nil
	}

	rootIDs := findTaskRootsByChatSession(b.cfg.SessionDir, chatSessionID, 6)
	if len(rootIDs) == 0 {
		return nil
	}

	store := NewSessionStore(b.cfg.SessionDir)
	for _, rootID := range rootIDs {
		rootSession, err := store.Load(rootID, rootID)
		if err != nil || rootSession == nil {
			continue
		}
		snapshot, _ := store.LoadRuntimeSnapshot(rootID, rootID)
		childSessions := loadChildSessionsForRoot(store, b.cfg.SessionDir, rootID)
		rootPending, totalPending := summarizeMailboxPending(store, rootID, childSessions)

		query := strings.TrimSpace(rootSession.Title)
		promptCtx := SystemPromptContext{}
		var attachments []Attachment
		var compactHistory []RuntimeCompactMetadata
		if snapshot != nil {
			if strings.TrimSpace(snapshot.Query) != "" {
				query = strings.TrimSpace(snapshot.Query)
			}
			promptCtx = snapshot.PromptContext
			attachments = cloneAttachments(snapshot.Attachments)
			if len(snapshot.CompactHistory) > 0 {
				compactHistory = make([]RuntimeCompactMetadata, len(snapshot.CompactHistory))
				copy(compactHistory, snapshot.CompactHistory)
			}
		}

		return &contextTaskDebug{
			RootID:              rootID,
			Query:               query,
			RootSession:         rootSession,
			Snapshot:            snapshot,
			PromptContext:       promptCtx,
			Attachments:         attachments,
			CompactHistory:      compactHistory,
			ChildSessions:       childSessions,
			RootMailboxPending:  rootPending,
			TotalMailboxPending: totalPending,
			PendingToolCalls:    findPendingToolCallNames(rootSession.Messages),
		}
	}
	return nil
}

func findTaskRootsByChatSession(sessionDir, chatSessionID string, limit int) []string {
	if strings.TrimSpace(sessionDir) == "" || strings.TrimSpace(chatSessionID) == "" {
		return nil
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil
	}

	type rootCandidate struct {
		rootID  string
		turn    int
		modTime time.Time
	}

	prefix := chatSessionID + "_"
	var candidates []rootCandidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		turn := -1
		if n, err := strconv.Atoi(strings.TrimPrefix(name, prefix)); err == nil {
			turn = n
		}
		candidates = append(candidates, rootCandidate{
			rootID:  name,
			turn:    turn,
			modTime: info.ModTime(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].turn != candidates[j].turn {
			return candidates[i].turn > candidates[j].turn
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	rootIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		rootIDs = append(rootIDs, candidate.rootID)
	}
	return rootIDs
}

func loadChildSessionsForRoot(store *SessionStore, sessionDir, rootID string) []*TaskSession {
	if store == nil || strings.TrimSpace(sessionDir) == "" || strings.TrimSpace(rootID) == "" {
		return nil
	}
	rootDir := filepath.Join(sessionDir, rootID)
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
	}

	var children []*TaskSession
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "session_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		sessionID := strings.TrimSuffix(strings.TrimPrefix(name, "session_"), ".json")
		if sessionID == rootID {
			continue
		}
		child, err := store.Load(rootID, sessionID)
		if err != nil || child == nil {
			continue
		}
		children = append(children, child)
	}

	sort.Slice(children, func(i, j int) bool {
		iTime := time.Time{}
		jTime := time.Time{}
		if children[i].StartedAt != nil {
			iTime = *children[i].StartedAt
		}
		if children[j].StartedAt != nil {
			jTime = *children[j].StartedAt
		}
		return iTime.After(jTime)
	})
	return children
}

func summarizeMailboxPending(store *SessionStore, rootID string, childSessions []*TaskSession) (rootPending, totalPending int) {
	if store == nil || strings.TrimSpace(rootID) == "" {
		return 0, 0
	}
	sessionIDs := []string{rootID}
	for _, child := range childSessions {
		if child != nil && strings.TrimSpace(child.ID) != "" {
			sessionIDs = append(sessionIDs, child.ID)
		}
	}
	for _, sessionID := range sessionIDs {
		msgs, err := store.PeekMailbox(rootID, sessionID)
		if err != nil {
			continue
		}
		if sessionID == rootID {
			rootPending = len(msgs)
		}
		totalPending += len(msgs)
	}
	return rootPending, totalPending
}

func trimPromptSections(sections []PromptSection, limit int) []PromptSection {
	if limit <= 0 || len(sections) <= limit {
		return sections
	}
	trimmed := make([]PromptSection, limit)
	copy(trimmed, sections[:limit])
	return trimmed
}

func summarizeChildSessionStatuses(children []*TaskSession) string {
	if len(children) == 0 {
		return "0"
	}
	counts := make(map[string]int)
	for _, child := range children {
		status := strings.TrimSpace(child.Status)
		if status == "" {
			status = "unknown"
		}
		counts[status]++
	}
	keys := make([]string, 0, len(counts))
	for status := range counts {
		keys = append(keys, status)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, status := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", status, counts[status]))
	}
	return fmt.Sprintf("%d (%s)", len(children), strings.Join(parts, ", "))
}

func summarizeAttachmentKinds(attachments []Attachment) string {
	if len(attachments) == 0 {
		return "0"
	}
	counts := make(map[string]int)
	for _, attachment := range attachments {
		kind := strings.TrimSpace(string(attachment.Kind))
		if kind == "" {
			kind = "runtime_context"
		}
		counts[kind]++
	}
	keys := make([]string, 0, len(counts))
	for kind := range counts {
		keys = append(keys, kind)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, kind := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", kind, counts[kind]))
	}
	return fmt.Sprintf("%d (%s)", len(attachments), strings.Join(parts, ", "))
}

func summarizeLatestCompaction(history []RuntimeCompactMetadata) string {
	if len(history) == 0 {
		return ""
	}
	last := history[len(history)-1]
	return fmt.Sprintf("%s %d→%d msgs, %s→%s chars",
		fallbackText(strings.TrimSpace(last.Reason), "compact"),
		last.BeforeMessages,
		last.AfterMessages,
		formatTokenCount(int64(last.BeforeChars)),
		formatTokenCount(int64(last.AfterChars)),
	)
}

func summarizeRuntimePrompt(promptCtx SystemPromptContext) string {
	if strings.TrimSpace(promptCtx.SystemPrompt) == "" {
		return ""
	}
	summary := fmt.Sprintf("%s chars", formatTokenCount(int64(len([]rune(promptCtx.SystemPrompt)))))
	if len(promptCtx.Sections) > 0 {
		parts := make([]string, 0, len(promptCtx.Sections))
		for _, sec := range trimPromptSections(promptCtx.Sections, 3) {
			parts = append(parts, fmt.Sprintf("%s=%d", sec.Name, sec.Chars))
		}
		summary += " | " + strings.Join(parts, ", ")
		if len(promptCtx.Sections) > 3 {
			summary += fmt.Sprintf(", +%d sections", len(promptCtx.Sections)-3)
		}
	}
	return summary
}

func summarizeLatestAssistantRecord(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		if status, inProgress, failed, ok := parseAssistantRecordSummary(msg.Content); ok {
			return fmt.Sprintf("状态=%s | 进行中=%s | 失败点=%s",
				fallbackText(status, "unknown"),
				fallbackText(inProgress, "无"),
				fallbackText(failed, "无"),
			)
		}
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			return truncate(content, 120)
		}
	}
	return ""
}

func previewContextMessages(messages []Message, limit int) []string {
	if limit <= 0 {
		return nil
	}
	previews := make([]string, 0, limit)
	for i := len(messages) - 1; i >= 0 && len(previews) < limit; i-- {
		msg := messages[i]
		if msg.Role == "system" {
			continue
		}
		previews = append(previews, previewContextMessage(msg))
	}
	for i, j := 0, len(previews)-1; i < j; i, j = i+1, j-1 {
		previews[i], previews[j] = previews[j], previews[i]
	}
	return previews
}

func previewContextMessage(msg Message) string {
	content := strings.TrimSpace(msg.Content)
	switch {
	case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
		names := make([]string, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			names = append(names, tc.Function.Name)
		}
		return fmt.Sprintf("[assistant] tool_call: %s", strings.Join(names, ", "))
	case msg.Role == "tool":
		if content == "" {
			return "[tool] (empty)"
		}
		return fmt.Sprintf("[tool] %s", truncate(content, 100))
	case isRuntimeAttachmentMessage(content):
		firstLine := content
		if idx := strings.Index(firstLine, "\n"); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		return fmt.Sprintf("[attachment] %s", truncate(firstLine, 100))
	case msg.Role == "assistant":
		if status, inProgress, failed, ok := parseAssistantRecordSummary(content); ok {
			return fmt.Sprintf("[assistant] 状态=%s | 进行中=%s | 失败点=%s",
				fallbackText(status, "unknown"),
				fallbackText(inProgress, "无"),
				fallbackText(failed, "无"),
			)
		}
	}
	if content == "" {
		content = "(empty)"
	}
	return fmt.Sprintf("[%s] %s", msg.Role, truncate(content, 100))
}

func isRuntimeAttachmentMessage(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), "[运行时上下文/")
}

func findPendingToolCallNames(messages []Message) []string {
	pending := make(map[string]string)
	for _, msg := range messages {
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				pending[tc.ID] = tc.Function.Name
			}
			continue
		}
		if msg.Role == "tool" && msg.ToolCallID != "" {
			delete(pending, msg.ToolCallID)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	names := make([]string, 0, len(pending))
	seen := make(map[string]bool)
	for _, name := range pending {
		if strings.TrimSpace(name) == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func boolWord(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
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
