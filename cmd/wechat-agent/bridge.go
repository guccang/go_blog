package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
}

// NewBridge 创建桥接器
func NewBridge(cfg *Config) *Bridge {
	agentID := fmt.Sprintf("wechat-%s", cfg.AgentName)

	client := uap.NewClient(cfg.GatewayURL, agentID, "wechat", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
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
		cfg:    cfg,
		client: client,
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

	// 本地处理 help/status 等简单命令
	switch {
	case content == "帮助" || content == "help":
		b.sendAppMessage(msg.FromUserName, getHelpText())
		return
	case content == "状态" || content == "status":
		connStatus := "❌ 未连接"
		if b.IsConnected() {
			connStatus = "✅ 已连接"
		}
		b.sendAppMessage(msg.FromUserName, fmt.Sprintf("WeChat Agent 状态\nGateway: %s", connStatus))
		return
	}

	// 过渡期：将消息转发给 go_blog-agent 处理
	targetAgent := b.cfg.GoBackendAgentID
	if targetAgent == "" {
		log.Printf("[Bridge] no target agent configured, dropping message from %s", msg.FromUserName)
		b.sendAppMessage(msg.FromUserName, "⚠️ 消息路由未配置")
		return
	}

	if !b.IsConnected() {
		log.Printf("[Bridge] not connected to gateway, dropping message from %s", msg.FromUserName)
		b.sendAppMessage(msg.FromUserName, "⚠️ Gateway 连接断开，请稍后重试")
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
			b.sendAppMessage(payload.To, payload.Content)
		}

	case uap.MsgToolCall:
		// 收到工具调用请求
		var payload uap.ToolCallPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid tool_call payload: %v", err)
			return
		}
		b.handleToolCall(msg, &payload)

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
	var result string
	var success bool

	switch payload.ToolName {
	case "wechat.SendMessage":
		var args struct {
			ToUser  string `json:"to_user"`
			Content string `json:"content"`
		}
		json.Unmarshal(payload.Arguments, &args)
		if err := b.sendAppMessage(args.ToUser, args.Content); err != nil {
			result = fmt.Sprintf("send failed: %v", err)
		} else {
			result = "sent"
			success = true
		}

	case "wechat.SendMarkdown":
		var args struct {
			Content string `json:"content"`
		}
		json.Unmarshal(payload.Arguments, &args)
		if err := b.sendWebhookMarkdown(args.Content); err != nil {
			result = fmt.Sprintf("send failed: %v", err)
		} else {
			result = "sent"
			success = true
		}

	default:
		result = fmt.Sprintf("unknown tool: %s", payload.ToolName)
	}

	// 发送结果
	b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   success,
		Result:    result,
	})
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

	msg := map[string]any{
		"touser":   toUser,
		"msgtype":  "markdown",
		"agentid":  b.cfg.AgentID,
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

func getHelpText() string {
	return "📖 Go Blog 企业微信指令\n\n" +
		"📋 数据查询\n• 待办 / todo — 今日待办\n• 运动 / exercise — 运动统计\n• 阅读 / reading — 阅读进度\n\n" +
		"📊 报告\n• 日报 — 生成今日报告\n• 周报 — 生成本周报告\n\n" +
		"⏰ 提醒\n• 提醒列表 — 查看定时提醒\n• 状态 / status — 服务器状态\n\n" +
		"💻 编码\n• cg list — 项目列表\n• cg start <项目> <需求> — 启动编码\n• cg status — 查看进度\n• cg stop — 停止编码\n\n" +
		"🧠 AI — 其他任意问题直接发送"
}
