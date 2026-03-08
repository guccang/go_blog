package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
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
		"tool_result", "failure_decision",
		"task_complete", "task_forced_summary",
		"plan_timing", "review_timing",
		"synthesis_done",
		"subtask_timeout", "subtask_llm_error",
		"progress", "retry_detail", "modify_detail":
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
		msg = text // 已包含格式化内容
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
	case "tool_call":
		msg = "🔧 " + text
	case "tool_result":
		msg = text // 已包含格式化内容
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

// ========================= 微信对话上下文管理 =========================

// WechatConversation 单个微信用户的对话会话
type WechatConversation struct {
	mu           sync.Mutex // 保护 Messages 等字段
	processing   sync.Mutex // 序列化同一用户的消息处理，避免并发交错
	WechatUser   string
	SessionID    string    // 首次创建时生成，同一会话复用
	Messages     []Message // 完整对话历史（system + user/assistant 交替）
	LastActiveAt time.Time
	TurnCount    int
}

// WechatConversationManager 管理所有微信用户的对话上下文
type WechatConversationManager struct {
	mu            sync.RWMutex
	conversations map[string]*WechatConversation // wechatUser → conversation
	timeout       time.Duration                  // 会话超时
	maxMessages   int                            // 单会话最大消息数
	maxTurns      int                            // 单会话最大对话轮次
}

// NewWechatConversationManager 创建微信对话管理器
func NewWechatConversationManager(timeout time.Duration, maxMessages, maxTurns int) *WechatConversationManager {
	return &WechatConversationManager{
		conversations: make(map[string]*WechatConversation),
		timeout:       timeout,
		maxMessages:   maxMessages,
		maxTurns:      maxTurns,
	}
}

// GetOrCreate 获取现有对话或创建新对话（超时自动重置）
func (m *WechatConversationManager) GetOrCreate(wechatUser string) (*WechatConversation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[wechatUser]
	if exists && time.Since(conv.LastActiveAt) < m.timeout && conv.TurnCount < m.maxTurns {
		return conv, false // 复用现有会话
	}

	// 超时、超轮次或不存在 → 创建新对话
	conv = &WechatConversation{
		WechatUser:   wechatUser,
		SessionID:    "wechat_" + newSessionID(),
		LastActiveAt: time.Now(),
	}
	m.conversations[wechatUser] = conv
	return conv, true // 新会话
}

// Reset 显式重置某用户的对话
func (m *WechatConversationManager) Reset(wechatUser string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conversations, wechatUser)
}

// CleanupExpired 清理所有过期对话
func (m *WechatConversationManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []string
	for user, conv := range m.conversations {
		if time.Since(conv.LastActiveAt) >= m.timeout {
			expired = append(expired, user)
		}
	}
	for _, user := range expired {
		delete(m.conversations, user)
	}
	if len(expired) > 0 {
		log.Printf("[WechatConv] 清理 %d 个过期会话", len(expired))
	}
}

// isConversationResetCommand 判断是否为对话重置命令
func isConversationResetCommand(content string) bool {
	content = strings.TrimSpace(content)
	resetCommands := []string{"新对话", "重新开始", "清除上下文", "reset", "new chat"}
	for _, cmd := range resetCommands {
		if strings.EqualFold(content, cmd) {
			return true
		}
	}
	return false
}

// compactWechatMessages 压缩微信对话消息，防止上下文溢出
// 保留 system prompt + 最近的消息，将旧消息压缩为摘要
func compactWechatMessages(messages []Message, maxMessages int) []Message {
	if len(messages) <= maxMessages {
		return messages
	}

	// 保留 system 消息（messages[0]）
	systemMsg := messages[0]

	// 保留最近 keepCount 条消息
	keepCount := maxMessages * 2 / 3
	if keepCount < 6 {
		keepCount = 6
	}

	recentMsgs := messages[len(messages)-keepCount:]
	oldMsgs := messages[1 : len(messages)-keepCount] // 跳过 system

	// 构建旧消息摘要
	var summaryParts []string
	for _, msg := range oldMsgs {
		switch msg.Role {
		case "user":
			summaryParts = append(summaryParts, "用户: "+truncate(msg.Content, 100))
		case "assistant":
			summaryParts = append(summaryParts, "AI: "+truncate(msg.Content, 150))
		}
	}

	compactedMsg := Message{
		Role: "user",
		Content: fmt.Sprintf("[之前的对话摘要（已压缩 %d 条消息）]\n%s",
			len(oldMsgs), strings.Join(summaryParts, "\n")),
	}

	// 重新组装: system + compacted + recent
	result := make([]Message, 0, 2+len(recentMsgs))
	result = append(result, systemMsg)
	result = append(result, compactedMsg)
	result = append(result, recentMsgs...)
	return result
}

// ========================= 微信消息处理 =========================

// handleWechatMessage 处理微信消息：维护对话上下文 → processTask → 回复
func (b *Bridge) handleWechatMessage(fromAgent, wechatUser, content string) {
	log.Printf("[Wechat] from=%s user=%s content=%s", fromAgent, wechatUser, content)

	// 1. 检查是否为重置命令
	if isConversationResetCommand(content) {
		b.wechatConvMgr.Reset(wechatUser)
		b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
			Channel: "wechat",
			To:      wechatUser,
			Content: "已开始新对话。",
		})
		log.Printf("[Wechat] conversation reset for user=%s", wechatUser)
		return
	}

	// 2. 获取或创建对话
	conv, isNew := b.wechatConvMgr.GetOrCreate(wechatUser)

	// 序列化同一用户的消息处理（后到的消息等前一个完成）
	conv.processing.Lock()
	defer conv.processing.Unlock()

	// 3. 即时反馈：区分新/续会话
	var feedbackMsg string
	if isNew {
		feedbackMsg = "⏳ 收到消息，开始新对话..."
	} else {
		conv.mu.Lock()
		turnNum := conv.TurnCount + 1
		conv.mu.Unlock()
		feedbackMsg = fmt.Sprintf("⏳ 收到消息，继续对话（第%d轮）...", turnNum)
	}
	b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatUser,
		Content: feedbackMsg,
	})

	// 4. 构建/追加消息
	conv.mu.Lock()
	conv.LastActiveAt = time.Now()

	if isNew || len(conv.Messages) == 0 {
		// 新会话：构建 system prompt + 第一条 user 消息
		systemPrompt := b.buildAssistantSystemPrompt(b.cfg.DefaultAccount)
		conv.Messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}
		log.Printf("[Wechat] 新会话 sessionID=%s user=%s", conv.SessionID, wechatUser)
	} else {
		// 续接对话：追加 user 消息
		conv.Messages = append(conv.Messages, Message{Role: "user", Content: content})
		log.Printf("[Wechat] 续接会话 sessionID=%s user=%s turn=%d msgCount=%d",
			conv.SessionID, wechatUser, conv.TurnCount, len(conv.Messages))
	}

	// 5. 上下文压缩
	conv.Messages = compactWechatMessages(conv.Messages, b.wechatConvMgr.maxMessages)

	// 6. 复制消息快照（避免 processTask 执行期间被修改）
	messagesCopy := make([]Message, len(conv.Messages))
	copy(messagesCopy, conv.Messages)

	taskID := fmt.Sprintf("%s_%d", conv.SessionID, conv.TurnCount)
	conv.TurnCount++
	conv.mu.Unlock()

	// 7. 构建 TaskContext（传入完整对话历史）
	sink := &WechatSink{
		bridge:     b,
		fromAgent:  fromAgent,
		wechatUser: wechatUser,
	}

	ctx := &TaskContext{
		TaskID:   taskID,
		Account:  b.cfg.DefaultAccount,
		Query:    content,
		Source:   "wechat",
		Messages: messagesCopy, // 传入完整对话历史
		Sink:     sink,
	}

	taskStart := time.Now()
	result, _ := b.processTask(ctx)
	taskDuration := time.Since(taskStart)

	// 发送完成耗时事件
	sink.OnEvent("task_complete", fmt.Sprintf("处理完成，耗时 %s", fmtDuration(taskDuration)))

	if result == "" {
		result = "抱歉，未能生成回复。"
	}

	// 8. 将 assistant 回复追加到对话历史
	conv.mu.Lock()
	conv.Messages = append(conv.Messages, Message{Role: "assistant", Content: result})
	conv.mu.Unlock()

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

// StartWechatCleanupLoop 后台定时清理过期微信对话
func (b *Bridge) StartWechatCleanupLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			b.wechatConvMgr.CleanupExpired()
		}
	}()
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
		data, err := b.CallTool(toolName, args)
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout after %v", timeout)
	}
}
