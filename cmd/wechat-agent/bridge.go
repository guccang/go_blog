package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"uap"
)

// Bridge 微信消息 ↔ UAP 消息桥接
type Bridge struct {
	cfg    *Config
	client *uap.Client

	// access_token 缓存
	cachedToken   string
	tokenExpireAt time.Time
	tokenMu       sync.Mutex

	// codegen 事件节流
	lastEventTime map[string]time.Time // session_id → 上次推送时间
	eventMu       sync.Mutex

	// session → wechat user 映射（从 payload.Account 学习）
	sessionUsers map[string]string // session_id → wechat user
	sessionMu    sync.Mutex
}

// NewBridge 创建桥接器
func NewBridge(cfg *Config) *Bridge {
	agentID := fmt.Sprintf("wechat-%s", cfg.AgentName)

	client := uap.NewClient(cfg.GatewayURL, agentID, "wechat", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Description = "企业微信消息发送"
	client.Tools = []uap.ToolDef{
		{
			Name:        "wechat.SendMessage",
			Description: "通过企业微信发送消息给指定用户",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"to_user":{"type":"string"},"content":{"type":"string"}}}`),
		},
		{
			Name:        "wechat.SendMarkdown",
			Description: "通过企业微信 Webhook 推送 Markdown 消息到群",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}}}`),
		},
	}
	client.Capacity = 10
	client.Meta = map[string]any{
		"http_port": cfg.HTTPPort,
	}

	b := &Bridge{
		cfg:           cfg,
		client:        client,
		lastEventTime: make(map[string]time.Time),
		sessionUsers:  make(map[string]string),
	}

	// 设置消息回调
	client.OnMessage = b.handleUAPMessage

	return b
}

// Run 启动 gateway 连接（阻塞）
func (b *Bridge) Run() {
	b.client.Run()
}

// Stop 停止
func (b *Bridge) Stop() {
	b.client.Stop()
}

// IsConnected 是否已连接 gateway
func (b *Bridge) IsConnected() bool {
	return b.client.IsConnected()
}

// HandleWechatMessage 处理来自微信的消息，转发到 gateway
func (b *Bridge) HandleWechatMessage(msg *WechatMessage) {
	if msg.MsgType != "text" {
		return
	}

	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}

	// 本地处理命令（支持 / 开头和旧格式）
	switch {
	case content == "/help" || content == "帮助" || content == "help":
		b.sendAppMessage(msg.FromUserName, getHelpText())
		return
	case content == "/status" || content == "状态" || content == "status":
		connStatus := "❌ 未连接"
		if b.IsConnected() {
			connStatus = "✅ 已连接"
		}
		b.sendAppMessage(msg.FromUserName, fmt.Sprintf("WeChat Agent 状态\nGateway: %s", connStatus))
		return
	}

	if !b.IsConnected() {
		log.Printf("[Bridge] not connected to gateway, dropping message from %s", msg.FromUserName)
		b.sendAppMessage(msg.FromUserName, "⚠️ Gateway 连接断开，请稍后重试")
		return
	}

	// 路由：结构化命令 → blog-agent，自然语言 → llm-agent
	targetAgent := b.cfg.LLMAgentID
	if isCmdCommand(content) && b.cfg.CmdAgentID != "" {
		targetAgent = b.cfg.CmdAgentID
	} else if isBackendCommand(content) && b.cfg.BackendAgentID != "" {
		targetAgent = b.cfg.BackendAgentID
	}
	if targetAgent == "" {
		log.Printf("[Bridge] no target agent configured, dropping message from %s", msg.FromUserName)
		b.sendAppMessage(msg.FromUserName, "⚠️ 消息路由未配置")
		return
	}

	// 发送 notify 消息到目标 agent
	err := b.client.SendTo(targetAgent, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      msg.FromUserName,
		Content: content,
	})
	if err != nil {
		log.Printf("[Bridge] send to %s failed: %v", targetAgent, err)
		b.sendAppMessage(msg.FromUserName, "⚠️ 消息发送失败，请稍后重试")
	}
}

// handleUAPMessage 处理来自 gateway 的 UAP 消息
func (b *Bridge) handleUAPMessage(msg *uap.Message) {
	switch msg.Type {
	case uap.MsgNotify:
		// 收到通知消息 → 发送给微信用户
		var payload uap.NotifyPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid notify payload: %v", err)
			return
		}
		if payload.Channel == "wechat" && payload.To != "" {
			b.sendNotification(payload.To, payload.Content)
		}

	case uap.MsgToolCall:
		// 收到工具调用请求
		var payload uap.ToolCallPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid tool_call payload: %v", err)
			return
		}
		b.handleToolCall(msg, &payload)

	case "stream_event":
		// [Phase 2] 收到 codegen 流式事件（从 blog-agent-agent 转发）
		b.handleCodegenStreamEvent(msg)

	case "task_complete":
		// [Phase 2] 收到 codegen 任务完成（从 blog-agent-agent 转发）
		b.handleCodegenTaskComplete(msg)

	case uap.MsgError:
		var payload uap.ErrorPayload
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			log.Printf("[Bridge] error from gateway: %s - %s", payload.Code, payload.Message)
		}

	default:
		log.Printf("[Bridge] unhandled message type: %s from %s", msg.Type, msg.From)
	}
}

// handleToolCall 处理工具调用
func (b *Bridge) handleToolCall(msg *uap.Message, payload *uap.ToolCallPayload) {
	var result uap.ToolResultPayload

	switch payload.ToolName {
	case "wechat.SendMessage":
		var args struct {
			ToUser  string `json:"to_user"`
			Content string `json:"content"`
		}
		json.Unmarshal(payload.Arguments, &args)
		if err := b.sendAppMessage(args.ToUser, args.Content); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("send failed: %v", err))
		} else {
			result = uap.BuildToolResult(msg.ID, nil, "消息已发送")
		}

	case "wechat.SendMarkdown":
		var args struct {
			Content string `json:"content"`
		}
		json.Unmarshal(payload.Arguments, &args)
		if err := b.sendWebhookMarkdown(args.Content); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("send failed: %v", err))
		} else {
			result = uap.BuildToolResult(msg.ID, nil, "Markdown消息已发送")
		}

	default:
		result = uap.BuildToolError(msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName))
	}

	// 发送结果
	b.client.SendTo(msg.From, uap.MsgToolResult, result)
}

// ========================= CodeGen 事件处理 =========================

// codegenStreamEvent codegen stream_event 的 payload 结构
type codegenStreamEvent struct {
	SessionID string `json:"session_id"`
	Account   string `json:"account,omitempty"`
	Event     struct {
		Type     string  `json:"type"`
		Text     string  `json:"text,omitempty"`
		ToolName string  `json:"tool_name,omitempty"`
		CostUSD  float64 `json:"cost_usd,omitempty"`
		Done     bool    `json:"done,omitempty"`
	} `json:"event"`
}

// codegenTaskComplete codegen task_complete 的 payload 结构
type codegenTaskComplete struct {
	SessionID string `json:"session_id"`
	Account   string `json:"account,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// handleCodegenStreamEvent 处理 codegen 流式事件，节流后推送微信
func (b *Bridge) handleCodegenStreamEvent(msg *uap.Message) {
	var payload codegenStreamEvent
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Bridge] invalid stream_event payload: %v", err)
		return
	}

	// 学习/查找 session → user 映射
	toUser := ""
	b.sessionMu.Lock()
	if payload.Account != "" {
		b.sessionUsers[payload.SessionID] = payload.Account
		toUser = payload.Account
	} else {
		toUser = b.sessionUsers[payload.SessionID]
	}
	b.sessionMu.Unlock()

	// 节流：同一 session 每 10 秒最多推送一次
	b.eventMu.Lock()
	lastTime := b.lastEventTime[payload.SessionID]
	now := time.Now()
	shouldSend := now.Sub(lastTime) >= 10*time.Second
	if shouldSend {
		b.lastEventTime[payload.SessionID] = now
	}
	b.eventMu.Unlock()

	if !shouldSend {
		return
	}

	// 格式化事件为微信消息
	text := formatEventForWeChat(&payload)
	if text == "" {
		return
	}

	b.sendNotification(toUser, text)
}

// handleCodegenTaskComplete 处理 codegen 任务完成
func (b *Bridge) handleCodegenTaskComplete(msg *uap.Message) {
	var payload codegenTaskComplete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Bridge] invalid task_complete payload: %v", err)
		return
	}

	// 清理节流状态
	b.eventMu.Lock()
	delete(b.lastEventTime, payload.SessionID)
	b.eventMu.Unlock()

	// 查找/学习 session → user 映射
	toUser := ""
	b.sessionMu.Lock()
	if payload.Account != "" {
		toUser = payload.Account
	} else {
		toUser = b.sessionUsers[payload.SessionID]
	}
	// 完成后清理映射
	delete(b.sessionUsers, payload.SessionID)
	b.sessionMu.Unlock()

	// 构建完成消息
	var text string
	if payload.Status == "error" {
		text = fmt.Sprintf("❌ 编码任务失败\n会话: %s\n错误: %s", payload.SessionID, payload.Error)
	} else {
		text = fmt.Sprintf("✅ 编码任务完成\n会话: %s", payload.SessionID)
	}

	b.sendNotification(toUser, text)
}

// formatEventForWeChat 将 stream_event 格式化为微信推送文本
func formatEventForWeChat(payload *codegenStreamEvent) string {
	sessionPrefix := payload.SessionID
	if len(sessionPrefix) > 8 {
		sessionPrefix = sessionPrefix[:8]
	}

	text := strings.TrimSpace(payload.Event.Text)
	switch payload.Event.Type {
	case "system":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("📦 [%s] %s", sessionPrefix, text)
	case "assistant":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("💬 [%s] %s", sessionPrefix, text)
	case "thought":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("🧠 [%s] %s", sessionPrefix, text)
	case "tool":
		if payload.Event.ToolName != "" {
			return fmt.Sprintf("🔧 [%s] 执行: %s", sessionPrefix, payload.Event.ToolName)
		}
		if text == "" {
			return ""
		}
		return fmt.Sprintf("🔧 [%s] %s", sessionPrefix, text)
	case "tool_detail":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("📝 [%s] %s", sessionPrefix, text)
	case "tool_update":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("✅ [%s] %s", sessionPrefix, text)
	case "plan":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("🗂 [%s]\n%s", sessionPrefix, text)
	case "mode":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("🔄 [%s] %s", sessionPrefix, text)
	case "error":
		if text == "" {
			return ""
		}
		return fmt.Sprintf("⚠️ [%s] %s", sessionPrefix, text)
	case "result":
		if text == "" {
			return ""
		}
		if payload.Event.CostUSD > 0 {
			return fmt.Sprintf("📊 [%s] %s (费用: $%.4f)", sessionPrefix, text, payload.Event.CostUSD)
		}
		return fmt.Sprintf("📊 [%s] %s", sessionPrefix, text)
	default:
		return ""
	}
}

// ========================= 统一推送 =========================

const maxAppMessageSize = 256 * 1024 // 256KB

// truncateForApp 截断过长内容
func truncateForApp(content string) string {
	if len(content) <= maxAppMessageSize {
		return content
	}
	return content[:maxAppMessageSize-20] + "\n...(内容已截断)"
}

// sendNotification 统一推送：应用优先 → webhook 兜底
func (b *Bridge) sendNotification(toUser, content string) {
	content = truncateForApp(content)
	if toUser != "" {
		if err := b.sendAppMessage(toUser, content); err == nil {
			return
		} else {
			log.Printf("[Bridge] app push failed for user=%s: %v, fallback to webhook", toUser, err)
		}
	}
	b.sendWebhookMarkdown(content)
}

// ========================= 微信 API =========================

// sendAppMessage 通过应用消息 API 发送消息
func (b *Bridge) sendAppMessage(toUser, content string) error {
	if !b.cfg.IsAppEnabled() {
		return fmt.Errorf("wechat app not configured")
	}

	token, err := b.getAccessToken()
	if err != nil {
		return fmt.Errorf("get token: %v", err)
	}

	// agentid 需要是整数类型（企业微信 API 要求）
	agentIDInt, _ := strconv.Atoi(b.cfg.AgentID)

	msg := map[string]any{
		"touser":   toUser,
		"msgtype":  "markdown",
		"agentid":  agentIDInt,
		"markdown": map[string]string{"content": content},
	}

	data, _ := json.Marshal(msg)
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && result.ErrCode != 0 {
		if result.ErrCode == 40014 || result.ErrCode == 42001 {
			b.tokenMu.Lock()
			b.cachedToken = ""
			b.tokenMu.Unlock()
			log.Println("[Bridge] token expired, retrying...")
			return b.sendAppMessage(toUser, content)
		}
		return fmt.Errorf("send error: %d %s", result.ErrCode, result.ErrMsg)
	}

	log.Printf("[Bridge] WeChat app message sent to %s", toUser)
	return nil
}

// getAccessToken 获取并缓存 access_token
func (b *Bridge) getAccessToken() (string, error) {
	b.tokenMu.Lock()
	defer b.tokenMu.Unlock()

	if b.cachedToken != "" && time.Now().Before(b.tokenExpireAt) {
		return b.cachedToken, nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		b.cfg.CorpID, b.cfg.Secret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("get access_token: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token: %v", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("token error: %d %s", result.ErrCode, result.ErrMsg)
	}

	b.cachedToken = result.AccessToken
	b.tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	log.Println("[Bridge] access_token refreshed")
	return b.cachedToken, nil
}

// sendWebhookMarkdown 通过 Webhook 推送 Markdown
func (b *Bridge) sendWebhookMarkdown(content string) error {
	if b.cfg.WebhookURL == "" {
		return fmt.Errorf("webhook not configured")
	}

	msg := map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": content},
	}
	data, _ := json.Marshal(msg)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(b.cfg.WebhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("webhook: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.ErrCode != 0 {
		return fmt.Errorf("webhook error: %d %s", result.ErrCode, result.ErrMsg)
	}

	log.Println("[Bridge] webhook sent")
	return nil
}

// ========================= 帮助文本 =========================

// isCmdCommand 判断消息是否应由 cmd-agent 处理。
func isCmdCommand(content string) bool {
	return strings.HasPrefix(content, "/cg") || strings.HasPrefix(content, "cg ") || content == "cg"
}

// isBackendCommand 判断消息是否为其他结构化命令（直接路由到 blog-agent，不经过 LLM）。
func isBackendCommand(content string) bool {
	return content == "刷新提示词" || strings.EqualFold(content, "reload prompts")
}

func getHelpText() string {
	return "📖 Go Blog 企业微信指令\n\n" +
		"💬 对话管理\n• /help — 显示此帮助\n• /reset — 开始新对话（清空上下文）\n• /status — 查看服务器状态\n\n" +
		"🤖 Claude Mode\n• /claude <项目> [提示词] — 进入 Claude 直连模式\n• /claude --ask <项目> — 交互式权限模式\n• /claude --model <名称> <项目> — 指定模型\n\n" +
		"Claude Mode 命令（进入后使用）:\n• cc exit / cc 退出 — 退出 Claude Mode\n• cc stop / cc 停止 — 停止当前任务\n• cc plan — 切换到 plan 模式\n• cc code — 切换到 code 模式\n• cc help / cc 帮助 — 查看详细帮助\n\n" +
		"📋 数据查询\n• 待办 / todo — 今日待办\n• 运动 / exercise — 运动统计\n• 阅读 / reading — 阅读进度\n\n" +
		"📊 报告\n• 日报 — 生成今日报告\n• 周报 — 生成本周报告\n\n" +
		"⏰ 提醒\n• 提醒列表 — 查看定时提醒\n\n" +
		"💻 编码\n• /cg list — 项目列表\n• /cg start <项目> <需求> — 启动编码\n• /cg status — 查看进度\n• /cg stop — 停止编码\n\n" +
		"🧠 AI — 其他任意问题直接发送"
}
