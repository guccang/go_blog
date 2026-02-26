package wechat

import (
	"bytes"
	"config"
	"encoding/json"
	"fmt"
	"io"
	log "mylog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// WechatConfig 企业微信配置
type WechatConfig struct {
	// 群机器人 Webhook（推送通知）
	WebhookURL string

	// 自建应用（接收指令 + 应用消息回复）
	CorpID         string
	AgentID        string
	Secret         string
	Token          string
	EncodingAESKey string

	Enabled         bool // Webhook 推送是否可用
	CallbackEnabled bool // 回调接收是否可用
}

var (
	globalConfig *WechatConfig
	initOnce     sync.Once

	// 指令处理器
	commandHandler func(account, message string) string

	// access_token 缓存
	cachedToken   string
	tokenExpireAt time.Time
	tokenMu       sync.Mutex
)

// InitWechatConfig 从 sys_conf.md 初始化企业微信配置
func InitWechatConfig() {
	initOnce.Do(func() {
		adminAccount := config.GetAdminAccount()
		webhookURL := config.GetConfigWithAccount(adminAccount, "wechat_webhook")
		corpID := config.GetConfigWithAccount(adminAccount, "wechat_corp_id")
		agentID := config.GetConfigWithAccount(adminAccount, "wechat_agent_id")
		secret := config.GetConfigWithAccount(adminAccount, "wechat_secret")
		token := config.GetConfigWithAccount(adminAccount, "wechat_token")
		encodingAESKey := config.GetConfigWithAccount(adminAccount, "wechat_encoding_aes_key")

		webhookEnabled := webhookURL != ""
		callbackEnabled := corpID != "" && token != "" && encodingAESKey != ""
		appEnabled := corpID != "" && secret != "" && agentID != ""

		globalConfig = &WechatConfig{
			WebhookURL:      webhookURL,
			CorpID:          corpID,
			AgentID:         agentID,
			Secret:          secret,
			Token:           token,
			EncodingAESKey:  encodingAESKey,
			Enabled:         webhookEnabled,
			CallbackEnabled: callbackEnabled,
		}

		if appEnabled {
			log.MessageF(log.ModuleAgent, "WeChat app message initialized (corpID=%s, agentID=%s)", corpID, agentID)
		} else {
			log.WarnF(log.ModuleAgent, "WeChat app NOT enabled: corpID=%v, secret=%v, agentID=%v",
				corpID != "", secret != "", agentID != "")
		}
		if webhookEnabled {
			log.Message(log.ModuleAgent, "WeChat webhook initialized")
		}
		if callbackEnabled {
			log.Message(log.ModuleAgent, "WeChat callback initialized")
		}
	})
}

// IsEnabled Webhook 推送是否可用
func IsEnabled() bool {
	return globalConfig != nil && globalConfig.Enabled
}

// IsAppEnabled 应用消息是否可用（corpID + secret + agentID）
func IsAppEnabled() bool {
	return globalConfig != nil && globalConfig.CorpID != "" && globalConfig.Secret != "" && globalConfig.AgentID != ""
}

// IsCallbackEnabled 回调接收是否可用
func IsCallbackEnabled() bool {
	return globalConfig != nil && globalConfig.CallbackEnabled
}

// SetCommandHandler 设置指令处理回调（由 agent 模块注入）
func SetCommandHandler(handler func(account, message string) string) {
	commandHandler = handler
}

// ========================= Access Token =========================

// getAccessToken 获取并缓存 access_token（2小时有效）
func getAccessToken() (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	if cachedToken != "" && time.Now().Before(tokenExpireAt) {
		return cachedToken, nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		globalConfig.CorpID, globalConfig.Secret)

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

	cachedToken = result.AccessToken
	tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	log.Message(log.ModuleAgent, "WeChat access_token refreshed")
	return cachedToken, nil
}

// ========================= 应用消息 API =========================

// SendAppMessage 通过自建应用发送消息给指定用户
func SendAppMessage(toUser, content string) error {
	if !IsAppEnabled() {
		return fmt.Errorf("wechat app not configured (need corp_id + secret + agent_id)")
	}

	token, err := getAccessToken()
	if err != nil {
		return fmt.Errorf("get token: %v", err)
	}

	msg := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": globalConfig.AgentID,
		"text":    map[string]string{"content": content},
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
		// token 过期，清除缓存重试
		if result.ErrCode == 40014 || result.ErrCode == 42001 {
			tokenMu.Lock()
			cachedToken = ""
			tokenMu.Unlock()
			log.Warn(log.ModuleAgent, "WeChat token expired, retrying...")
			return SendAppMessage(toUser, content)
		}
		return fmt.Errorf("send error: %d %s", result.ErrCode, result.ErrMsg)
	}

	log.MessageF(log.ModuleAgent, "WeChat app message sent to %s", toUser)
	return nil
}

// SendAppMessageToAll 发送应用消息给所有人
func SendAppMessageToAll(content string) error {
	return SendAppMessage("@all", content)
}

// ========================= Webhook 推送 =========================

type webhookMessage struct {
	MsgType  string           `json:"msgtype"`
	Text     *webhookText     `json:"text,omitempty"`
	Markdown *webhookMarkdown `json:"markdown,omitempty"`
}
type webhookText struct {
	Content string `json:"content"`
}
type webhookMarkdown struct {
	Content string `json:"content"`
}

// SendText 推送文本消息到企业微信群
func SendText(content string) error {
	if !IsEnabled() {
		return fmt.Errorf("wechat webhook not configured")
	}
	return sendWebhook(webhookMessage{MsgType: "text", Text: &webhookText{Content: content}})
}

// SendMarkdown 推送 Markdown 格式消息到企业微信群
func SendMarkdown(content string) error {
	if !IsEnabled() {
		return fmt.Errorf("wechat webhook not configured")
	}
	return sendWebhook(webhookMessage{MsgType: "markdown", Markdown: &webhookMarkdown{Content: content}})
}

func sendWebhook(msg webhookMessage) error {
	data, _ := json.Marshal(msg)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(globalConfig.WebhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		log.WarnF(log.ModuleAgent, "WeChat webhook failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.ErrCode != 0 {
		log.WarnF(log.ModuleAgent, "WeChat webhook error: %d %s", result.ErrCode, result.ErrMsg)
		return fmt.Errorf("wechat error: %d %s", result.ErrCode, result.ErrMsg)
	}
	log.Message(log.ModuleAgent, "WeChat webhook sent successfully")
	return nil
}

// ========================= 回调处理 =========================

func HandleCallback(w http.ResponseWriter, r *http.Request) {
	if !IsCallbackEnabled() {
		http.Error(w, "WeChat callback not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		handleVerify(w, r)
	case http.MethodPost:
		handleMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleVerify(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echoStr := r.URL.Query().Get("echostr")

	log.MessageF(log.ModuleAgent, "WeChat verify: timestamp=%s nonce=%s", timestamp, nonce)

	decrypted, err := VerifyURL(globalConfig.Token, globalConfig.EncodingAESKey,
		globalConfig.CorpID, msgSignature, timestamp, nonce, echoStr)
	if err != nil {
		log.WarnF(log.ModuleAgent, "WeChat verify failed: %v", err)
		http.Error(w, "Verification failed", http.StatusForbidden)
		return
	}
	log.Message(log.ModuleAgent, "WeChat URL verification successful")
	w.Write([]byte(decrypted))
}

func handleMessage(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	decrypted, err := DecryptMessage(globalConfig.Token, globalConfig.EncodingAESKey,
		globalConfig.CorpID, msgSignature, timestamp, nonce, string(body))
	if err != nil {
		log.WarnF(log.ModuleAgent, "WeChat decrypt failed: %v", err)
		http.Error(w, "Decrypt failed", http.StatusForbidden)
		return
	}

	msg, err := ParseMessage(decrypted)
	if err != nil {
		log.WarnF(log.ModuleAgent, "WeChat parse failed: %v", err)
		w.Write([]byte("success"))
		return
	}

	log.MessageF(log.ModuleAgent, "WeChat message from %s: %s", msg.FromUserName, msg.Content)
	go processUserMessage(msg)
	w.Write([]byte("success"))
}

// WechatMessage 企业微信消息
type WechatMessage struct {
	ToUserName   string `json:"ToUserName"`
	FromUserName string `json:"FromUserName"`
	CreateTime   int64  `json:"CreateTime"`
	MsgType      string `json:"MsgType"`
	Content      string `json:"Content"`
	MsgId        string `json:"MsgId"`
	AgentID      string `json:"AgentID"`
}

// processUserMessage 处理用户消息（异步）
func processUserMessage(msg *WechatMessage) {
	if msg.MsgType != "text" {
		return
	}

	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}

	log.MessageF(log.ModuleAgent, "WeChat command: %s (from: %s)", content, msg.FromUserName)

	var reply string
	switch {
	case content == "帮助" || content == "help":
		reply = getHelpText()
	case content == "状态" || content == "status":
		reply = "✅ Go Blog 服务运行中"
	default:
		if commandHandler != nil {
			reply = commandHandler(msg.FromUserName, content)
		} else {
			reply = "⚠️ AI 处理器未初始化"
		}
	}

	if reply == "" {
		return
	}

	// 优先通过应用消息 API 直接回复给用户
	if IsAppEnabled() {
		log.MessageF(log.ModuleAgent, "WeChat reply via APP API to %s (%d chars)", msg.FromUserName, len(reply))
		if err := SendAppMessage(msg.FromUserName, reply); err != nil {
			log.WarnF(log.ModuleAgent, "WeChat app reply failed: %v, falling back to webhook", err)
			// 降级到 Webhook
			if IsEnabled() {
				SendText(fmt.Sprintf("@%s\n%s", msg.FromUserName, reply))
			}
		}
		return
	}

	// 降级到 Webhook
	log.WarnF(log.ModuleAgent, "WeChat reply via WEBHOOK (app not enabled, corpID=%v secret=%v agentID=%v)",
		globalConfig.CorpID != "", globalConfig.Secret != "", globalConfig.AgentID != "")
	if IsEnabled() {
		if err := SendText(fmt.Sprintf("@%s\n%s", msg.FromUserName, reply)); err != nil {
			log.WarnF(log.ModuleAgent, "WeChat reply failed: %v", err)
		}
	}
}

func getHelpText() string {
	return "📖 Go Blog 企业微信指令\n\n" +
		"📋 数据查询\n• 待办 / todo — 今日待办\n• 运动 / exercise — 运动统计\n• 阅读 / reading — 阅读进度\n\n" +
		"📊 报告\n• 日报 — 生成今日报告\n• 周报 — 生成本周报告\n\n" +
		"⏰ 提醒\n• 提醒列表 — 查看定时提醒\n• 状态 / status — 服务器状态\n\n" +
		"🧠 AI — 其他任意问题直接发送"
}
