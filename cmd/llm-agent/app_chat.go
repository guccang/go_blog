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
	bridge           *Bridge
	fromAgent        string
	appUser          string
	cortanaRequestID string
	buf              strings.Builder
	lastEventTime    time.Time
	audioSent        bool
}

var inlineAudioTagPattern = regexp.MustCompile(`(?is)<audio\b[^>]*\bsrc\s*=\s*"data:audio/([^;"]+);base64,([^"]+)"[^>]*>.*?</audio>`)
var cortanaActionPlanTagPattern = regexp.MustCompile(`(?is)\[CORTANA_ACTION_PLAN\]\s*(\{.*\})`)
var cortanaActionPlanFencePattern = regexp.MustCompile("(?is)```(?:cortana|cortana_plan|json)\\s*(\\{.*\\})\\s*```")

func buildCortanaOutputPrompt() string {
	return `
## Cortana 输出协议
- 当前请求来自 Cortana 文本入口，目标是驱动语音回复和 Live2D 动作。
- 优先给出自然、简洁、适合口播的中文回复，不要写多余前缀，不要解释你在遵循协议。
- 在正文最后额外附加一个动作计划块，格式必须严格如下：
[CORTANA_ACTION_PLAN]
{
  "speech_text": "最终要播报的文本",
  "expression": "happy",
  "fallback_expression": "happy",
  "expression_hold_ms": 1600,
  "mood": "warm",
  "actions": [
    {"motion": "IdleWave", "delay": 0, "index": 0, "hold_ms": 1800},
    {"motion": "Tap", "delay": 1800, "index": 0, "hold_ms": 900, "resume_to_idle": true},
    {"motion": "Idle", "delay": 3600, "index": 0}
  ]
}
- ` + "`speech_text`" + ` 必须与正文口播内容一致或是正文的自然口语化版本。
- ` + "`expression`" + ` 目前只用这三个值之一：` + "`happy`" + `、` + "`sad`" + `、` + "`surprised`" + `。
- ` + "`fallback_expression`" + ` 可选，表示短暂表情结束后回落到哪个基础表情；默认 ` + "`happy`" + `。
- ` + "`expression_hold_ms`" + ` 可选，表示当前表情保持多久后再切回 ` + "`fallback_expression`" + `。
- ` + "`mood`" + ` 可选，用来描述整体气质，例如 ` + "`warm`" + `、` + "`calm`" + `、` + "`alert`" + `、` + "`playful`" + `。
- ` + "`motion`" + ` 优先只用这些语义动作名：` + "`Idle`" + `、` + "`IdleAlt`" + `、` + "`IdleWave`" + `、` + "`Tap`" + `。
- ` + "`index`" + ` 可选，表示同一动作组的具体变体序号；不写时默认 0。
- ` + "`delay`" + ` 单位毫秒，表示从语音开始播放后的触发时间；动作数量控制在 1-4 个。
- ` + "`hold_ms`" + ` 可选，表示该动作预期持续的时间窗口，便于前端安排回落动作。
- ` + "`resume_to_idle`" + ` 可选，为 true 时表示该动作结束后可自动回到基础待机。
- 如果是问候、欢迎、轻松语气，优先用 ` + "`IdleWave`" + ` / ` + "`happy`" + `。
- 如果是解释说明或长回复，优先用 ` + "`Idle`" + `、` + "`IdleAlt`" + ` 交替，避免频繁 ` + "`Tap`" + `。
- 如果是道歉、遗憾、安慰，使用 ` + "`sad`" + `，动作以 ` + "`Idle`" + ` / ` + "`IdleAlt`" + ` 为主。
- 如果有强调、惊讶、兴奋、重大提醒，可以使用 ` + "`surprised`" + ` 并插入一次 ` + "`Tap`" + `。
- 不要输出 markdown 代码围栏，不要输出多个动作计划块，不要遗漏 ` + "`speech_text`" + `。`
}

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

func parseCortanaActionPlanJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	if nested, ok := payload["cortana_action_plan"].(map[string]any); ok {
		payload = nested
	}
	if payload == nil {
		return nil
	}
	if _, ok := payload["expression"]; ok {
		return payload
	}
	if _, ok := payload["actions"]; ok {
		return payload
	}
	if _, ok := payload["speech_text"]; ok {
		return payload
	}
	return nil
}

func extractCortanaActionPlan(text string) (string, map[string]any) {
	text = strings.TrimSpace(text)
	if text == "" {
		return text, nil
	}

	patterns := []*regexp.Regexp{
		cortanaActionPlanTagPattern,
		cortanaActionPlanFencePattern,
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) < 2 {
			continue
		}
		payload := parseCortanaActionPlanJSON(matches[1])
		if payload == nil {
			continue
		}
		cleaned := strings.TrimSpace(pattern.ReplaceAllString(text, ""))
		return cleaned, payload
	}
	return text, nil
}

func (s *AppSink) trySynthesizeAudioReply(text string, actionPlan map[string]any) bool {
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
	if !s.trySendAudioReplyWithSpeechText(raw, text, actionPlan) {
		return false
	}
	s.audioSent = true
	log.Printf("[AppSink] synthesized audio reply sent to=%s", s.appUser)
	return true
}

func (s *AppSink) trySendAudioReply(raw string) bool {
	return s.trySendAudioReplyWithSpeechText(raw, "", nil)
}

func (s *AppSink) trySendAudioReplyWithSpeechText(raw, speechText string, actionPlan map[string]any) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		log.Printf("[AppSink] parse audio_reply failed: %v", err)
		return false
	}
	audioBase64 := strings.TrimSpace(fmt.Sprint(payload["audio_base64"]))
	if audioBase64 == "" {
		return false
	}
	audioFormat := strings.TrimSpace(fmt.Sprint(payload["audio_format"]))
	if audioFormat == "" {
		audioFormat = "mp3"
	}
	if len(actionPlan) == 0 {
		if nested, ok := payload["cortana_action_plan"].(map[string]any); ok && len(nested) > 0 {
			actionPlan = nested
		} else {
			candidate := map[string]any{}
			if expression := strings.TrimSpace(fmt.Sprint(payload["expression"])); expression != "" {
				candidate["expression"] = expression
			}
			if actions, ok := payload["actions"].([]any); ok && len(actions) > 0 {
				candidate["actions"] = actions
			}
			if len(candidate) > 0 {
				actionPlan = candidate
			}
		}
	}
	return s.sendAudioRichMessage(audioBase64, audioFormat, speechText, actionPlan)
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
	if !s.trySendAudioReplyWithSpeechText(result.Result, text, nil) {
		return false
	}

	s.audioSent = true
	log.Printf("[AppSink] leaked textual TextToAudio tool call recovered for to=%s", s.appUser)
	return true
}

func (s *AppSink) trySendInlineAudioFromText(text string, actionPlan map[string]any) bool {
	matches := inlineAudioTagPattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(matches) != 3 {
		return false
	}
	audioFormat := normalizeAudioFormat(matches[1])
	audioBase64 := strings.TrimSpace(matches[2])
	if audioBase64 == "" {
		return false
	}
	return s.sendAudioRichMessage(audioBase64, audioFormat, text, actionPlan)
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

func (s *AppSink) sendAudioRichMessage(audioBase64, audioFormat, speechText string, actionPlan map[string]any) bool {
	meta := map[string]any{
		"audio_base64": audioBase64,
		"audio_format": audioFormat,
		"input_mode":   "tts_reply",
		"speech_text":  strings.TrimSpace(speechText),
	}
	if strings.TrimSpace(s.cortanaRequestID) != "" {
		meta["cortana_request_id"] = strings.TrimSpace(s.cortanaRequestID)
	}
	if len(actionPlan) > 0 {
		meta["cortana_action_plan"] = actionPlan
	}
	args, _ := json.Marshal(map[string]any{
		"to_user":      s.appUser,
		"content":      "[语音回复]",
		"message_type": "audio",
		"meta":         meta,
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
	cortanaTextMode := false
	cortanaRequestID := ""
	if inbound, ok := parseAppInboundMessage(content); ok && inbound != nil {
		if strings.EqualFold(strings.TrimSpace(inbound.MessageType), "audio") {
			preferAudioReply = true
		}
		if inbound.Attachment != nil && strings.EqualFold(strings.TrimSpace(inbound.Attachment.InputMode), "voice_audio") {
			preferAudioReply = true
		}
		if inbound.Meta != nil {
			if strings.EqualFold(strings.TrimSpace(fmt.Sprint(inbound.Meta["input_mode"])), "cortana_text") {
				preferAudioReply = true
				cortanaTextMode = true
			}
			cortanaRequestID = strings.TrimSpace(fmt.Sprint(inbound.Meta["cortana_request_id"]))
			replyMode := strings.TrimSpace(fmt.Sprint(inbound.Meta["reply_mode"]))
			if strings.EqualFold(replyMode, "audio") || strings.EqualFold(replyMode, "audio_preferred") {
				preferAudioReply = true
			}
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
		if cortanaTextMode {
			systemPrompt += buildCortanaOutputPrompt() + "\n"
		}
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
			if cortanaTextMode {
				freshPrompt += buildCortanaOutputPrompt() + "\n"
			}
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
		bridge:           b,
		fromAgent:        fromAgent,
		appUser:          appUser,
		cortanaRequestID: cortanaRequestID,
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

	result, cortanaActionPlan := extractCortanaActionPlan(result)
	if cortanaActionPlan != nil {
		if speechText := strings.TrimSpace(fmt.Sprint(cortanaActionPlan["speech_text"])); speechText != "" {
			result = speechText
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

	if !sink.AudioSent() && sink.trySendInlineAudioFromText(result, cortanaActionPlan) {
		log.Printf("[App] extracted inline audio reply for %s, skip text reply", appUser)
		return
	}

	if ctx.PreferAudioReply && !sink.AudioSent() && sink.trySynthesizeAudioReply(result, cortanaActionPlan) {
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
