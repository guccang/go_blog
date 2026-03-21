package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"uap"
)

// ========================= 命令解析 =========================

// ClaudeCommandOpts /claude 命令解析结果
type ClaudeCommandOpts struct {
	Project   string   // 项目名
	Prompt    string   // 初始 prompt
	Ask       bool     // --ask → 交互式权限
	Settings  string   // --settings <name>
	Model     string   // --model <name>
	MaxTurns  int      // --max-turns <n>
	ExtraArgs []string // 其他未识别的 flags，透传给 ACP 子进程
}

// parseClaudeCommand 解析 /claude 命令
// 格式: /claude [--ask] [--settings <name>] [--model <name>] [--max-turns <n>] <project> [prompt]
func parseClaudeCommand(content string) *ClaudeCommandOpts {
	// 移除 /claude 前缀
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "/claude") {
		return nil
	}
	content = strings.TrimPrefix(content, "/claude")
	content = strings.TrimSpace(content)

	if content == "" {
		return nil
	}

	opts := &ClaudeCommandOpts{}
	parts := strings.Fields(content)

	i := 0
	for i < len(parts) {
		arg := parts[i]
		switch {
		case arg == "--ask":
			opts.Ask = true
			i++
		case arg == "--settings" && i+1 < len(parts):
			opts.Settings = parts[i+1]
			i += 2
		case arg == "--model" && i+1 < len(parts):
			opts.Model = parts[i+1]
			i += 2
		case arg == "--max-turns" && i+1 < len(parts):
			if n, err := strconv.Atoi(parts[i+1]); err == nil {
				opts.MaxTurns = n
			}
			i += 2
		case strings.HasPrefix(arg, "--"):
			// 未识别的 flag: 带值的格式 --xxx value
			opts.ExtraArgs = append(opts.ExtraArgs, arg)
			if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "--") {
				opts.ExtraArgs = append(opts.ExtraArgs, parts[i+1])
				i += 2
			} else {
				i++
			}
		default:
			// 第一个非 flag 参数是 project
			if opts.Project == "" {
				opts.Project = arg
				i++
			} else {
				// 剩下的都是 prompt
				opts.Prompt = strings.Join(parts[i:], " ")
				i = len(parts)
			}
		}
	}

	if opts.Project == "" {
		return nil
	}

	return opts
}

// buildExtraArgs 根据解析结果构建传给 ACP 子进程的额外参数
func buildExtraArgs(opts *ClaudeCommandOpts) []string {
	var args []string

	// 默认加 --dangerously-skip-permissions（除非 --ask）
	if !opts.Ask {
		args = append(args, "--dangerously-skip-permissions")
	}

	// --settings 传短名称，由 acp-agent 解析为绝对路径
	if opts.Settings != "" {
		args = append(args, "--settings", opts.Settings)
	}

	// --model 透传
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// --max-turns 透传
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}

	// 其他未识别 flags 透传
	args = append(args, opts.ExtraArgs...)

	return args
}

// ========================= Claude Mode 入口 =========================

// handleClaudeCommand 处理 /claude 命令，进入 Claude Mode
func (b *Bridge) handleClaudeCommand(fromAgent, wechatUser, content string) {
	opts := parseClaudeCommand(content)
	if opts == nil {
		b.sendWechat(fromAgent, wechatUser, "用法: /claude [--ask] [--settings <name>] [--model <name>] <project> [prompt]")
		return
	}

	// 查找可用的 acp-agent
	acpAgentID, ok := b.getToolAgent("AcpStartSession")
	if !ok {
		b.sendWechat(fromAgent, wechatUser, "未找到可用的 ACP Agent，无法进入 Claude 模式。")
		return
	}

	// 获取或创建会话
	session, _ := b.sessionMgr.GetOrCreate("wechat", wechatUser, b.cfg.DefaultAccount)

	// 设置 Claude Mode 状态
	session.mu.Lock()
	session.ClaudeMode = true
	session.ClaudeProject = opts.Project
	session.ClaudeACPAgentID = acpAgentID
	session.ClaudeFromAgent = fromAgent
	session.ClaudeInteractive = opts.Ask
	session.LastActiveAt = time.Now()
	session.mu.Unlock()

	// 构建状态提示
	modeDesc := "自动"
	if opts.Ask {
		modeDesc = "交互"
	}
	statusMsg := fmt.Sprintf("🤖 进入 Claude 模式 | 项目: %s | 权限: %s", opts.Project, modeDesc)
	if opts.Settings != "" {
		statusMsg += fmt.Sprintf(" | settings: %s", opts.Settings)
	}
	if opts.Model != "" {
		statusMsg += fmt.Sprintf(" | model: %s", opts.Model)
	}
	statusMsg += "\n指令: cc exit(退出) cc stop(中断) cc plan/cc code(切换模式)"
	b.sendWechat(fromAgent, wechatUser, statusMsg)

	// 始终创建 ACP 会话（无 prompt 也创建，让 cc plan/code 可用）
	go b.handleClaudeModeFirstMessage(session, fromAgent, wechatUser, opts)
}

// handleClaudeModeFirstMessage 发送 Claude Mode 的第一条消息（创建新 ACP 会话）
func (b *Bridge) handleClaudeModeFirstMessage(session *ChatSession, fromAgent, wechatUser string, opts *ClaudeCommandOpts) {
	extraArgs := buildExtraArgs(opts)

	// 通过 tool_call 调用 AcpStartSession
	args := map[string]interface{}{
		"project":         opts.Project,
		"extra_args":      extraArgs,
		"interactive":     opts.Ask,
		"caller_agent_id": b.cfg.AgentID,
	}
	if opts.Prompt != "" {
		args["prompt"] = opts.Prompt
	}
	argsJSON, _ := json.Marshal(args)

	// 注册 claude stream sink
	sink := newClaudeStreamSink(b, fromAgent, wechatUser)
	go sink.run()

	// 记录 sink 到 bridge（供 handleMessage 路由）
	b.claudeSinksMu.Lock()
	b.claudeSinks[sessionKey("wechat", wechatUser)] = sink
	b.claudeSinksMu.Unlock()

	result, err := b.CallTool("AcpStartSession", argsJSON)
	sink.stop()

	// 清理 sink
	b.claudeSinksMu.Lock()
	delete(b.claudeSinks, sessionKey("wechat", wechatUser))
	b.claudeSinksMu.Unlock()

	if err != nil {
		b.sendWechat(fromAgent, wechatUser, fmt.Sprintf("Claude 会话启动失败: %v", err))
		return
	}

	// 提取 session_id
	if result != nil {
		var data struct {
			Data struct {
				SessionID string `json:"session_id"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(result.Result), &data) == nil && data.Data.SessionID != "" {
			session.mu.Lock()
			session.ClaudeSessionID = data.Data.SessionID
			session.mu.Unlock()
		}
	}

	if opts.Prompt != "" {
		b.sendWechat(fromAgent, wechatUser, "✅ Claude 完成，可以继续发消息或 cc exit 退出。")
	} else {
		b.sendWechat(fromAgent, wechatUser, "✅ Claude 会话就绪，发送消息开始对话。\n可用指令: cc plan/cc code(切换模式) cc exit(退出)")
	}
}

// handleClaudeModeMessage 处理 Claude Mode 中的后续消息（多轮对话）
func (b *Bridge) handleClaudeModeMessage(session *ChatSession, fromAgent, wechatUser, content string) {
	session.mu.Lock()
	sessionID := session.ClaudeSessionID
	interactive := session.ClaudeInteractive
	session.LastActiveAt = time.Now()
	session.mu.Unlock()

	// 注册 claude stream sink
	sink := newClaudeStreamSink(b, fromAgent, wechatUser)
	go sink.run()

	b.claudeSinksMu.Lock()
	b.claudeSinks[sessionKey("wechat", wechatUser)] = sink
	b.claudeSinksMu.Unlock()

	var result *ToolCallResult
	var err error

	if sessionID != "" {
		// 续接已有会话
		args := map[string]interface{}{
			"prompt":          content,
			"session_id":      sessionID,
			"interactive":     interactive,
			"caller_agent_id": b.cfg.AgentID,
		}
		argsJSON, _ := json.Marshal(args)
		result, err = b.CallTool("AcpSendMessage", argsJSON)
	} else {
		// 无 session_id，新建会话
		session.mu.Lock()
		project := session.ClaudeProject
		session.mu.Unlock()

		extraArgs := []string{}
		if !interactive {
			extraArgs = append(extraArgs, "--dangerously-skip-permissions")
		}

		args := map[string]interface{}{
			"project":         project,
			"prompt":          content,
			"extra_args":      extraArgs,
			"interactive":     interactive,
			"caller_agent_id": b.cfg.AgentID,
		}
		argsJSON, _ := json.Marshal(args)
		result, err = b.CallTool("AcpStartSession", argsJSON)

		// 提取 session_id
		if result != nil && err == nil {
			var data struct {
				Data struct {
					SessionID string `json:"session_id"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(result.Result), &data) == nil && data.Data.SessionID != "" {
				session.mu.Lock()
				session.ClaudeSessionID = data.Data.SessionID
				session.mu.Unlock()
			}
		}
	}

	sink.stop()

	// 清理 sink
	b.claudeSinksMu.Lock()
	delete(b.claudeSinks, sessionKey("wechat", wechatUser))
	b.claudeSinksMu.Unlock()

	if err != nil {
		b.sendWechat(fromAgent, wechatUser, fmt.Sprintf("Claude 执行失败: %v", err))
		return
	}

	b.sendWechat(fromAgent, wechatUser, "✅ Claude 完成，可以继续发消息或 cc exit 退出。")
}

// ========================= 权限回复处理 =========================

// handlePermissionReply 解析用户的权限回复并发送给 acp-agent
func (b *Bridge) handlePermissionReply(session *ChatSession, fromAgent, wechatUser, content string) {
	perm := session.GetPendingPermission()
	if perm == nil {
		b.sendWechat(fromAgent, wechatUser, "当前没有待处理的权限请求。")
		return
	}

	content = strings.TrimSpace(strings.ToLower(content))

	var optionID string
	var cancelled bool

	switch content {
	case "y", "yes", "允许", "同意":
		// 选择第一个选项（通常是 allow_once）
		if len(perm.Options) > 0 {
			optionID = perm.Options[0].OptionID
		}
	case "n", "no", "拒绝", "deny":
		// 查找 deny 选项，否则取消
		for _, opt := range perm.Options {
			if opt.Kind == "deny" {
				optionID = opt.OptionID
				break
			}
		}
		if optionID == "" {
			cancelled = true
		}
	default:
		// 数字选择
		if idx, err := strconv.Atoi(content); err == nil && idx >= 1 && idx <= len(perm.Options) {
			optionID = perm.Options[idx-1].OptionID
		} else {
			b.sendWechat(fromAgent, wechatUser, fmt.Sprintf("请输入 1-%d 的数字，或 y/n", len(perm.Options)))
			// 放回 pending
			session.SetPendingPermission(perm)
			return
		}
	}

	// 发送权限回复给 acp-agent
	payload := uap.PermissionResponsePayload{
		SessionID: perm.SessionID,
		RequestID: perm.RequestID,
		OptionID:  optionID,
		Cancelled: cancelled,
	}
	if err := b.client.SendTo(perm.ACPAgentID, uap.MsgPermissionResponse, payload); err != nil {
		log.Printf("[ClaudeMode] send permission response failed: %v", err)
		b.sendWechat(fromAgent, wechatUser, "发送权限回复失败。")
		return
	}

	if cancelled {
		b.sendWechat(fromAgent, wechatUser, "❌ 已拒绝，Claude 继续执行...")
	} else {
		b.sendWechat(fromAgent, wechatUser, "✅ 已批准，Claude 继续执行...")
	}
}

// ========================= 模式切换 =========================

// handleModeSwitch 发送模式切换请求给 acp-agent
func (b *Bridge) handleModeSwitch(session *ChatSession, fromAgent, wechatUser, modeID string) {
	session.mu.Lock()
	sessionID := session.ClaudeSessionID
	acpAgentID := session.ClaudeACPAgentID
	session.mu.Unlock()

	if sessionID == "" {
		b.sendWechat(fromAgent, wechatUser, "还没有活跃的 Claude 会话。")
		return
	}

	payload := uap.SetModePayload{
		SessionID: sessionID,
		ModeID:    modeID,
	}
	if err := b.client.SendTo(acpAgentID, uap.MsgSetMode, payload); err != nil {
		log.Printf("[ClaudeMode] send set_mode failed: %v", err)
		b.sendWechat(fromAgent, wechatUser, "模式切换失败。")
		return
	}

	b.sendWechat(fromAgent, wechatUser, fmt.Sprintf("🔄 已请求切换到 %s 模式", modeID))
}

// ========================= 退出/中断 =========================

// exitClaudeMode 退出 Claude Mode
func (b *Bridge) exitClaudeMode(session *ChatSession, fromAgent, wechatUser string) {
	// 停止当前活跃会话
	session.mu.Lock()
	sessionID := session.ClaudeSessionID
	acpAgentID := session.ClaudeACPAgentID
	session.mu.Unlock()

	if sessionID != "" && acpAgentID != "" {
		args := map[string]interface{}{
			"session_id": sessionID,
		}
		argsJSON, _ := json.Marshal(args)
		b.CallTool("AcpStopSession", argsJSON)
	}

	session.ResetClaudeMode()
	b.sendWechat(fromAgent, wechatUser, "👋 已退出 Claude 模式，回到普通对话。")
}

// stopClaudeSession 中断当前 Claude 任务（不退出模式）
func (b *Bridge) stopClaudeSession(session *ChatSession, fromAgent, wechatUser string) {
	session.CancelRunning()

	session.mu.Lock()
	sessionID := session.ClaudeSessionID
	session.ClaudeSessionID = "" // 清除会话 ID，下次消息会新建会话
	session.mu.Unlock()

	if sessionID != "" {
		args := map[string]interface{}{
			"session_id": sessionID,
		}
		argsJSON, _ := json.Marshal(args)
		go b.CallTool("AcpStopSession", argsJSON)
	}

	b.sendWechat(fromAgent, wechatUser, "🛑 已中断当前 Claude 任务，发消息可开始新任务。")
}

// ========================= 流式输出 Sink =========================

// claudeStreamSink 缓冲 + 节流的流式输出转发到微信
type claudeStreamSink struct {
	bridge     *Bridge
	fromAgent  string
	wechatUser string

	mu     sync.Mutex
	buf    strings.Builder
	done   chan struct{}
	closed bool
}

func newClaudeStreamSink(b *Bridge, fromAgent, wechatUser string) *claudeStreamSink {
	return &claudeStreamSink{
		bridge:     b,
		fromAgent:  fromAgent,
		wechatUser: wechatUser,
		done:       make(chan struct{}),
	}
}

// onStreamEvent 处理从 acp-agent 收到的流式事件
func (s *claudeStreamSink) onStreamEvent(evt StreamEventPayload) {
	switch evt.Event.Type {
	case "assistant":
		// 文本输出，缓冲
		s.mu.Lock()
		s.buf.WriteString(evt.Event.Text)
		s.mu.Unlock()
	case "tool":
		// 工具调用，立即转发
		s.bridge.sendWechat(s.fromAgent, s.wechatUser, evt.Event.Text)
	case "plan":
		// 计划更新，立即转发
		s.bridge.sendWechat(s.fromAgent, s.wechatUser, evt.Event.Text)
	case "mode":
		// 模式切换，立即转发
		s.bridge.sendWechat(s.fromAgent, s.wechatUser, evt.Event.Text)
	case "system":
		// 系统消息，立即转发
		s.bridge.sendWechat(s.fromAgent, s.wechatUser, evt.Event.Text)
	}
}

// run 定时刷新缓冲区到微信（3 秒/次）
func (s *claudeStreamSink) run() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flush()
		case <-s.done:
			s.flush() // 最终刷新
			return
		}
	}
}

// flush 将缓冲区内容发送到微信
func (s *claudeStreamSink) flush() {
	s.mu.Lock()
	if s.buf.Len() == 0 {
		s.mu.Unlock()
		return
	}
	text := s.buf.String()
	s.buf.Reset()
	s.mu.Unlock()

	// 截断过长文本
	if len(text) > 2000 {
		text = text[:2000] + "\n...(截断)"
	}

	s.bridge.sendWechat(s.fromAgent, s.wechatUser, "📝 "+text)
}

// stop 停止 sink
func (s *claudeStreamSink) stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	close(s.done)
}

// ========================= 工具函数 =========================

// sendWechat 发送微信消息
func (b *Bridge) sendWechat(fromAgent, wechatUser, content string) {
	b.client.SendTo(fromAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      wechatUser,
		Content: content,
	})
}

// isClaudeCommand 检查是否为 /claude 命令
func isClaudeCommand(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), "/claude")
}

// isClaudeModeCommand 检查是否为 Claude Mode 内置命令（cc xxx）
func isClaudeModeCommand(content string) (cmd string, ok bool) {
	content = strings.TrimSpace(strings.ToLower(content))
	switch content {
	case "cc exit", "cc 退出":
		return "exit", true
	case "cc stop", "cc 停止":
		return "stop", true
	case "cc plan":
		return "plan", true
	case "cc code":
		return "code", true
	}
	return "", false
}

// StreamEventPayload 流式事件（从 acp-agent notify 解析）
// 复用 acp-agent 的格式
type StreamEventPayload struct {
	SessionID string      `json:"session_id"`
	Event     StreamEvent `json:"event"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Done      bool   `json:"done,omitempty"`
}
