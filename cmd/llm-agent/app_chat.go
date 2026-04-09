package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
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
	audioSent     bool
}

var inlineAudioTagPattern = regexp.MustCompile(`(?is)<audio\b[^>]*\bsrc\s*=\s*"data:audio/([^;"]+);base64,([^"]+)"[^>]*>.*?</audio>`)

func (s *AppSink) OnChunk(text string) { s.buf.WriteString(text) }

func (s *AppSink) OnEvent(event, text string) {
	if event == "audio_reply" {
		if s.trySendAudioReply(text) {
			s.audioSent = true
			s.lastEventTime = time.Now()
			return
		}
	}

	if !isImportantEvent(event) && time.Since(s.lastEventTime) < 1*time.Second {
		return
	}

	var msg string
	switch event {
	case "thinking":
		msg = "思考: " + text
	case "tool_info", "tool_result", "tool_progress", "skill_tool_result":
		msg = text
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
func (s *AppSink) AudioSent() bool { return s.audioSent }

func (s *AppSink) trySynthesizeAudioReply(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	agentID, ok := s.bridge.getToolAgent("TextToAudio")
	if !ok {
		log.Printf("[AppSink] TextToAudio tool not found for audio synthesis")
		return false
	}
	args, _ := json.Marshal(map[string]any{
		"text": text,
	})
	result, err := s.bridge.callRemoteAgent(context.Background(), "TextToAudio", agentID, args, nil)
	if err != nil {
		log.Printf("[AppSink] synthesize audio reply failed: %v", err)
		return false
	}
	raw := ""
	if result != nil {
		raw = result.Result
	}
	if raw == "" {
		return false
	}
	if !s.trySendAudioReply(raw) {
		return false
	}
	s.audioSent = true
	log.Printf("[AppSink] synthesized audio reply sent to=%s", s.appUser)
	return true
}

func (s *AppSink) trySendAudioReply(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	var result struct {
		AudioBase64 string `json:"audio_base64"`
		AudioFormat string `json:"audio_format"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		log.Printf("[AppSink] parse audio_reply failed: %v", err)
		return false
	}
	if strings.TrimSpace(result.AudioBase64) == "" {
		return false
	}
	audioFormat := strings.TrimSpace(result.AudioFormat)
	if audioFormat == "" {
		audioFormat = "mp3"
	}
	return s.sendAudioRichMessage(result.AudioBase64, audioFormat)
}

func (s *AppSink) trySendAudioReplyFromToolCalls(toolCalls []ToolCall) bool {
	for _, tc := range toolCalls {
		if !s.trySendAudioReplyFromToolCall(tc) {
			continue
		}
		return true
	}
	return false
}

func (s *AppSink) trySendAudioReplyFromToolCall(tc ToolCall) bool {
	if s.bridge.resolveToolName(tc.Function.Name) != "TextToAudio" {
		return false
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		log.Printf("[AppSink] parse legacy TextToAudio args failed: %v", err)
		return false
	}

	text := firstNonEmptyMapString(args, "text", "content", "input")
	if text == "" {
		log.Printf("[AppSink] legacy TextToAudio args missing text")
		return false
	}

	payload := map[string]any{
		"text": text,
	}
	if voice := firstNonEmptyMapString(args, "voice", "voice_id"); voice != "" {
		payload["voice"] = voice
	}
	if audioFormat := firstNonEmptyMapString(args, "audio_format", "format"); audioFormat != "" {
		payload["audio_format"] = audioFormat
	}

	agentID, ok := s.bridge.getToolAgent("TextToAudio")
	if !ok {
		log.Printf("[AppSink] TextToAudio tool not found for leaked legacy tool call")
		return false
	}

	rawArgs, _ := json.Marshal(payload)
	result, err := s.bridge.callRemoteAgent(context.Background(), "TextToAudio", agentID, rawArgs, nil)
	if err != nil {
		log.Printf("[AppSink] execute leaked TextToAudio tool call failed: %v", err)
		return false
	}
	if result == nil || strings.TrimSpace(result.Result) == "" {
		return false
	}
	if !s.trySendAudioReply(result.Result) {
		return false
	}

	s.audioSent = true
	log.Printf("[AppSink] leaked textual TextToAudio tool call recovered for to=%s", s.appUser)
	return true
}

func (s *AppSink) trySendInlineAudioFromText(text string) bool {
	matches := inlineAudioTagPattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(matches) != 3 {
		return false
	}
	audioFormat := normalizeAudioFormat(matches[1])
	audioBase64 := strings.TrimSpace(matches[2])
	if audioBase64 == "" {
		return false
	}
	return s.sendAudioRichMessage(audioBase64, audioFormat)
}

func normalizeAudioFormat(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "mpeg":
		return "mp3"
	case "x-wav":
		return "wav"
	default:
		if v == "" {
			return "mp3"
		}
		return v
	}
}

func firstNonEmptyMapString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func (s *AppSink) sendAudioRichMessage(audioBase64, audioFormat string) bool {
	args, _ := json.Marshal(map[string]any{
		"to_user":      s.appUser,
		"content":      "[语音回复]",
		"message_type": "audio",
		"meta": map[string]any{
			"audio_base64": audioBase64,
			"audio_format": audioFormat,
			"input_mode":   "tts_reply",
		},
	})
	if _, err := s.bridge.callRemoteAgent(context.Background(), "app.SendRichMessage", s.fromAgent, args, nil); err != nil {
		log.Printf("[AppSink] send audio rich message failed: %v", err)
		return false
	}
	log.Printf("[AppSink] audio rich message sent to=%s format=%s", s.appUser, audioFormat)
	return true
}

func (b *Bridge) handleAppMessage(fromAgent, appUser, content string) {
	log.Printf("[App] from=%s user=%s content=%s", fromAgent, appUser, content)

	// 去除 delegation token 前缀（格式：[delegation:xxx]actual content）
	// 注意：不再依赖 delegation token 做权限验证，改用 AuthenticatedUser 方案
	// 但仍需要从 content 中去除前缀，避免 token 被发送给 LLM
	if strings.HasPrefix(content, "[delegation:") {
		endIdx := strings.Index(content, "]")
		if endIdx > 12 { // "[delegation:" 长度为 12
			content = content[endIdx+1:]
			log.Printf("[App] stripped delegation token prefix from content for user=%s", appUser)
		}
	}

	goctx, cancel := context.WithCancel(context.Background())
	goctx = WithAuthenticatedUser(goctx, appUser)
	defer cancel()

	preferAudioReply := false
	if inbound, ok := parseAppInboundMessage(content); ok && inbound != nil {
		if strings.EqualFold(strings.TrimSpace(inbound.MessageType), "audio") {
			preferAudioReply = true
		}
		if inbound.Attachment != nil && strings.EqualFold(strings.TrimSpace(inbound.Attachment.InputMode), "voice_audio") {
			preferAudioReply = true
		}
	}

	content = b.preprocessAppMessage(goctx, fromAgent, appUser, content)

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
		systemPrompt, promptSections := b.buildAssistantSystemPromptForQuery(appUser, content, true)
		systemPrompt += fmt.Sprintf("\n当前App用户ID(app_user): %s\n", appUser)
		session.Messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}
		session.PromptSections = promptSections
		log.Printf("[App] 新会话 sessionID=%s user=%s", session.SessionID, appUser)
	} else {
		if len(session.Messages) > 0 && session.Messages[0].Role == "system" {
			freshPrompt, promptSections := b.buildAssistantSystemPromptForQuery(appUser, content, true)
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

	session.SetCancel(cancel)
	defer session.SetCancel(nil)

	ctx := &TaskContext{
		Ctx:              goctx,
		TaskID:           taskID,
		Account:          appUser,
		Query:            content,
		Source:           "app",
		PreferAudioReply: preferAudioReply,
		Messages:         messagesCopy,
		Sink:             sink,
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

	cleanResult, leakedToolCalls := extractLegacyToolCallBlocks(result)
	if len(leakedToolCalls) > 0 {
		var toolNames []string
		for _, tc := range leakedToolCalls {
			toolNames = append(toolNames, b.resolveToolName(tc.Function.Name))
		}
		log.Printf("[App] stripped leaked textual tool calls user=%s tools=%v", appUser, toolNames)
		result = cleanResult
		if !sink.AudioSent() {
			sink.trySendAudioReplyFromToolCalls(leakedToolCalls)
		}
	}

	if strings.TrimSpace(result) == "" {
		if sink.AudioSent() {
			result = "[已发送语音回复]"
		} else {
			result = "抱歉，未能生成回复。"
		}
	}

	assistantContent := persistedAssistantContent(ctx, result)
	session.mu.Lock()
	session.Messages = append(session.Messages, Message{Role: "assistant", Content: assistantContent})
	session.mu.Unlock()

	if err := b.sessionMgr.SaveSession(session); err != nil {
		log.Printf("[App] save session failed: %v", err)
	}

	if !sink.AudioSent() && sink.trySendInlineAudioFromText(result) {
		log.Printf("[App] extracted inline audio reply for %s, skip text reply", appUser)
		return
	}

	if ctx.PreferAudioReply && !sink.AudioSent() && sink.trySynthesizeAudioReply(result) {
		log.Printf("[App] synthesized audio reply for %s, skip text reply", appUser)
		return
	}

	if sink.AudioSent() {
		log.Printf("[App] audio reply already sent to %s, skip duplicate text reply", appUser)
		return
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
