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

// AppMessage is the message pushed from the app side to app-agent.
type AppMessage struct {
	UserID            string         `json:"user_id"`
	Content           string         `json:"content"`
	MessageType       string         `json:"message_type,omitempty"`
	SessionID         string         `json:"session_id,omitempty"`
	TraceID           string         `json:"trace_id,omitempty"`
	Meta              map[string]any `json:"meta,omitempty"`
	DelegationToken   string         `json:"delegation_token,omitempty"` // delegation token for blog-agent API
}

// AppPushPayload is the payload pushed from app-agent back to the app side.
type AppPushPayload struct {
	Sequence    int64          `json:"sequence"`
	UserID      string         `json:"user_id"`
	Content     string         `json:"content"`
	MessageType string         `json:"message_type"`
	Channel     string         `json:"channel"`
	Timestamp   int64          `json:"timestamp"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// Bridge bridges app messages and UAP messages.
type Bridge struct {
	cfg    *Config
	client *uap.Client
	groups *groupManager

	lastEventTime map[string]time.Time
	eventMu       sync.Mutex

	sessionUsers map[string]string
	sessionMu    sync.Mutex

	deliveryMu   sync.Mutex
	nextSequence int64
	pending      map[string][]AppPushPayload
	clients      map[string]*appClientConn

	// delegation tokens by user
	delegationTokens map[string]string
	delegationMu    sync.Mutex
}

func NewBridge(cfg *Config) *Bridge {
	agentID := fmt.Sprintf("app-%s", cfg.AgentName)

	client := uap.NewClient(cfg.GatewayURL, agentID, "app", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Description = "App message forwarding agent"
	client.Tools = []uap.ToolDef{
		{
			Name:        "app.SendMessage",
			Description: "Send a text message to an app user",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"to_user":{"type":"string"},
					"content":{"type":"string"}
				},
				"required":["to_user","content"]
			}`),
		},
	}
	client.Capacity = 20
	client.Meta = map[string]any{
		"http_port": cfg.HTTPPort,
	}

	b := &Bridge{
		cfg:           cfg,
		client:        client,
		groups:        newGroupManager(cfg.GroupStoreFile),
		lastEventTime: make(map[string]time.Time),
		sessionUsers:  make(map[string]string),
		pending:       make(map[string][]AppPushPayload),
		clients:       make(map[string]*appClientConn),
		delegationTokens: make(map[string]string),
	}
	client.OnMessage = b.handleUAPMessage
	return b
}

func (b *Bridge) Run() {
	b.client.Run()
}

func (b *Bridge) Stop() {
	b.client.Stop()
	b.closeAllClients()
}

func (b *Bridge) IsConnected() bool {
	return b.client.IsConnected()
}

func (b *Bridge) OnlineClientCount() int {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()
	return len(b.clients)
}

func (b *Bridge) PendingMessageCount() int {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	total := 0
	for _, queue := range b.pending {
		total += len(queue)
	}
	return total
}

// SetDelegationToken 设置用户的 delegation token
func (b *Bridge) SetDelegationToken(userID, token string) {
	b.delegationMu.Lock()
	defer b.delegationMu.Unlock()
	b.delegationTokens[userID] = token
}

// GetDelegationToken 获取用户的 delegation token
func (b *Bridge) GetDelegationToken(userID string) string {
	b.delegationMu.Lock()
	defer b.delegationMu.Unlock()
	return b.delegationTokens[userID]
}

func (b *Bridge) HandleAppMessage(msg *AppMessage) {
	content := strings.TrimSpace(msg.Content)
	if content == "" && msg.MessageType != "audio" {
		return
	}
	log.Printf("[Bridge] inbound app message user=%s type=%s len=%d content=%q",
		msg.UserID, msg.MessageType, len(content), shortText(content))

	if msg.MessageType == "audio" {
		if groupID := b.groupIDFromMeta(msg.Meta); groupID != "" {
			if err := b.handleGroupMessage(groupID, msg.UserID, content, msg.MessageType, msg.Meta); err != nil {
				log.Printf("[Bridge] group audio broadcast failed user=%s group=%s: %v", msg.UserID, groupID, err)
				_ = b.sendAppPush(msg.UserID, fmt.Sprintf("Group message failed: %v", err), nil)
			}
			return
		}
		audioBytes := 0
		if msg.Meta != nil {
			if base64Text, ok := msg.Meta["audio_base64"].(string); ok {
				audioBytes = len(base64Text)
			}
		}
		log.Printf("[Bridge] received audio message user=%s base64_len=%d meta_keys=%d",
			msg.UserID, audioBytes, len(msg.Meta))
		if err := b.sendAppPush(msg.UserID, "Voice message received and sent to app-agent. Swipe up-right to convert speech to text.", map[string]any{
			"kind":         "audio_ack",
			"message_type": "audio",
		}); err != nil {
			log.Printf("[Bridge] send audio ack failed: %v", err)
		}
		return
	}

	if groupID := b.groupIDFromMeta(msg.Meta); groupID != "" {
		if err := b.handleGroupMessage(groupID, msg.UserID, content, msg.MessageType, msg.Meta); err != nil {
			log.Printf("[Bridge] group broadcast failed user=%s group=%s: %v", msg.UserID, groupID, err)
			_ = b.sendAppPush(msg.UserID, fmt.Sprintf("Group message failed: %v", err), nil)
		}
		return
	}

	switch {
	case content == "/help" || content == "help" || content == "甯姪":
		if err := b.sendAppPush(msg.UserID, getHelpText(), nil); err != nil {
			log.Printf("[Bridge] send help failed: %v", err)
		}
		return
	case content == "/status" || content == "status":
		connStatus := "not connected"
		if b.IsConnected() {
			connStatus = "connected"
		}
		statusText := fmt.Sprintf(
			"App Agent status\nGateway: %s\nOnline clients: %d\nPending messages: %d",
			connStatus,
			b.OnlineClientCount(),
			b.PendingMessageCount(),
		)
		if err := b.sendAppPush(msg.UserID, statusText, nil); err != nil {
			log.Printf("[Bridge] send status failed: %v", err)
		}
		return
	}

	if !b.IsConnected() {
		log.Printf("[Bridge] not connected to gateway, dropping message from %s", msg.UserID)
		_ = b.sendAppPush(msg.UserID, "Gateway disconnected, please retry later.", nil)
		return
	}

	targetAgent := b.cfg.LLMAgentID
	if isBackendCommand(content) && b.cfg.BackendAgentID != "" {
		targetAgent = b.cfg.BackendAgentID
	}
	if targetAgent == "" {
		log.Printf("[Bridge] no target agent configured, dropping message from %s", msg.UserID)
		_ = b.sendAppPush(msg.UserID, "Message routing is not configured.", nil)
		return
	}

	// 如果有 delegation token，添加到内容前面
	messageContent := content
	if msg.DelegationToken != "" {
		messageContent = fmt.Sprintf("[delegation:%s]%s", msg.DelegationToken, content)
	}

	payload := uap.NotifyPayload{
		Channel: "app",
		To:      msg.UserID,
		Content: messageContent,
	}
	log.Printf("[Bridge] route app notify user=%s target=%s channel=%s len=%d content=%q",
		msg.UserID, targetAgent, payload.Channel, len(messageContent), shortText(messageContent))
	if err := b.client.SendTo(targetAgent, uap.MsgNotify, payload); err != nil {
		log.Printf("[Bridge] send to %s failed: %v", targetAgent, err)
		_ = b.sendAppPush(msg.UserID, "Message forwarding failed, please retry later.", nil)
	} else {
		log.Printf("[Bridge] routed app notify user=%s target=%s", msg.UserID, targetAgent)
	}
}

func (b *Bridge) groupIDFromMeta(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if groupID, ok := meta["group_id"].(string); ok {
		return normalizeGroupID(groupID)
	}
	return ""
}

func (b *Bridge) handleGroupMessage(groupID, fromUser, content, messageType string, meta map[string]any) error {
	if err := b.broadcastGroupMessage(groupID, fromUser, content, messageType, meta); err != nil {
		return err
	}
	if messageType != "text" {
		return nil
	}

	robotContent, ok := extractRobotMentionContent(content)
	if !ok {
		return nil
	}
	robotAccount, ok := b.groups.RobotAccount(groupID)
	if !ok {
		return fmt.Errorf("group robot account not found")
	}
	return b.forwardGroupMessageToLLM(groupID, fromUser, robotAccount, robotContent)
}

func extractRobotMentionContent(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", false
	}
	if !strings.Contains(strings.ToLower(trimmed), "@robot") {
		return "", false
	}
	replacer := strings.NewReplacer("@robot", "", "@Robot", "", "@ROBOT", "")
	cleaned := strings.TrimSpace(replacer.Replace(trimmed))
	if cleaned == "" {
		return "", false
	}
	return cleaned, true
}

func (b *Bridge) forwardGroupMessageToLLM(groupID, fromUser, robotAccount, content string) error {
	if !b.IsConnected() {
		return fmt.Errorf("gateway disconnected")
	}
	if strings.TrimSpace(b.cfg.LLMAgentID) == "" {
		return fmt.Errorf("llm routing is not configured")
	}

	llmContent := strings.TrimSpace(content)
	payload := uap.NotifyPayload{
		Channel: "app",
		To:      robotAccount,
		Content: llmContent,
	}
	log.Printf("[Bridge] route group robot message group=%s from=%s robot_account=%s len=%d content=%q",
		groupID, fromUser, robotAccount, len(llmContent), shortText(llmContent))
	if err := b.client.SendTo(b.cfg.LLMAgentID, uap.MsgNotify, payload); err != nil {
		return fmt.Errorf("send to %s failed: %w", b.cfg.LLMAgentID, err)
	}
	return nil
}

func (b *Bridge) handleUAPMessage(msg *uap.Message) {
	log.Printf("[Bridge] inbound UAP message type=%s from=%s to=%s payload_len=%d",
		msg.Type, msg.From, msg.To, len(msg.Payload))
	switch msg.Type {
	case uap.MsgNotify:
		var payload uap.NotifyPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid notify payload: %v", err)
			return
		}
		log.Printf("[Bridge] notify payload from=%s channel=%s to=%s len=%d content=%q",
			msg.From, payload.Channel, payload.To, len(payload.Content), shortText(payload.Content))
		if payload.Channel == "app" && payload.To != "" {
			b.sendNotification(payload.To, payload.Content)
		}

	case uap.MsgToolCall:
		var payload uap.ToolCallPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid tool_call payload: %v", err)
			return
		}
		b.handleToolCall(msg, &payload)

	case "stream_event":
		b.handleCodegenStreamEvent(msg)

	case "task_complete":
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

func (b *Bridge) handleToolCall(msg *uap.Message, payload *uap.ToolCallPayload) {
	var result uap.ToolResultPayload

	switch payload.ToolName {
	case "app.SendMessage":
		var args struct {
			ToUser  string `json:"to_user"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("invalid arguments: %v", err))
			break
		}
		if err := b.sendAppPush(strings.TrimSpace(args.ToUser), strings.TrimSpace(args.Content), nil); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("send failed: %v", err))
		} else {
			result = uap.BuildToolResult(msg.ID, nil, "message queued")
		}

	default:
		result = uap.BuildToolError(msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName))
	}

	if err := b.client.SendTo(msg.From, uap.MsgToolResult, result); err != nil {
		log.Printf("[Bridge] send tool result failed: %v", err)
	}
}

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

type codegenTaskComplete struct {
	SessionID string `json:"session_id"`
	Account   string `json:"account,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func (b *Bridge) handleCodegenStreamEvent(msg *uap.Message) {
	var payload codegenStreamEvent
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Bridge] invalid stream_event payload: %v", err)
		return
	}

	toUser := ""
	b.sessionMu.Lock()
	if payload.Account != "" {
		b.sessionUsers[payload.SessionID] = payload.Account
		toUser = payload.Account
	} else {
		toUser = b.sessionUsers[payload.SessionID]
	}
	b.sessionMu.Unlock()

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

	text := formatEventForApp(&payload)
	if text == "" {
		return
	}

	b.sendNotification(toUser, text)
}

func (b *Bridge) handleCodegenTaskComplete(msg *uap.Message) {
	var payload codegenTaskComplete
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Bridge] invalid task_complete payload: %v", err)
		return
	}

	b.eventMu.Lock()
	delete(b.lastEventTime, payload.SessionID)
	b.eventMu.Unlock()

	toUser := ""
	b.sessionMu.Lock()
	if payload.Account != "" {
		toUser = payload.Account
	} else {
		toUser = b.sessionUsers[payload.SessionID]
	}
	delete(b.sessionUsers, payload.SessionID)
	b.sessionMu.Unlock()

	var text string
	if payload.Status == "error" {
		text = fmt.Sprintf("Codegen task failed\nSession: %s\nError: %s", payload.SessionID, payload.Error)
	} else {
		text = fmt.Sprintf("Codegen task completed\nSession: %s", payload.SessionID)
	}

	b.sendNotification(toUser, text)
}

func formatEventForApp(payload *codegenStreamEvent) string {
	sessionPrefix := payload.SessionID
	if len(sessionPrefix) > 8 {
		sessionPrefix = sessionPrefix[:8]
	}

	switch payload.Event.Type {
	case "system":
		return fmt.Sprintf("[system][%s] %s", sessionPrefix, payload.Event.Text)
	case "tool":
		if payload.Event.ToolName != "" {
			return fmt.Sprintf("[tool][%s] %s", sessionPrefix, payload.Event.ToolName)
		}
		return ""
	case "error":
		return fmt.Sprintf("[error][%s] %s", sessionPrefix, payload.Event.Text)
	case "result":
		if payload.Event.CostUSD > 0 {
			return fmt.Sprintf("[result][%s] %s (cost: $%.4f)", sessionPrefix, payload.Event.Text, payload.Event.CostUSD)
		}
		return fmt.Sprintf("[result][%s] %s", sessionPrefix, payload.Event.Text)
	default:
		return ""
	}
}

const maxAppMessageSize = 256 * 1024

func truncateForApp(content string) string {
	if len(content) <= maxAppMessageSize {
		return content
	}
	return content[:maxAppMessageSize-20] + "\n...(truncated)"
}

func (b *Bridge) sendNotification(toUser, content string) {
	if toUser == "" {
		log.Printf("[Bridge] skip notification: empty user")
		return
	}
	if groupID, ok := b.groups.GroupIDByRobotAccount(toUser); ok {
		log.Printf("[Bridge] robot notification routed account=%s -> group=%s len=%d content=%q",
			toUser, groupID, len(content), shortText(content))
		meta := map[string]any{
			"scope":     "group",
			"group_id":  groupID,
			"from_user": groupRobotDisplayName,
			"origin":    "llm-agent",
			"account":   toUser,
		}
		if err := b.broadcastGroupMessage(groupID, toUser, content, "text", meta); err != nil {
			log.Printf("[Bridge] robot group broadcast failed group=%s account=%s: %v", groupID, toUser, err)
		}
		return
	}
	log.Printf("[Bridge] deliver notification user=%s len=%d content=%q", toUser, len(content), shortText(content))
	if err := b.sendAppPush(toUser, content, nil); err != nil {
		log.Printf("[Bridge] app push failed for user=%s: %v", toUser, err)
	}
}

func (b *Bridge) broadcastGroupMessage(groupID, fromUser, content, messageType string, meta map[string]any) error {
	groupID = normalizeGroupID(groupID)
	fromUser = strings.TrimSpace(fromUser)
	if groupID == "" || fromUser == "" {
		return fmt.Errorf("group_id and user_id are required")
	}
	if !b.groups.HasMember(groupID, fromUser) {
		return fmt.Errorf("you are not a member of group %s", groupID)
	}
	humanMembers, err := b.groups.HumanMembers(groupID)
	if err != nil {
		return err
	}
	visibleMembers, err := b.groups.VisibleMembers(groupID)
	if err != nil {
		return err
	}

	displayFrom := fromUser
	if robotAccount, ok := b.groups.RobotAccount(groupID); ok && robotAccount == fromUser {
		displayFrom = groupRobotDisplayName
	}
	log.Printf("[Bridge] prepare group broadcast group=%s from=%s display_from=%s type=%s human_members=%v visible_members=%v",
		groupID, fromUser, displayFrom, messageType, humanMembers, visibleMembers)

	pushMeta := map[string]any{
		"scope":      "group",
		"group_id":   groupID,
		"from_user":  displayFrom,
		"members":    visibleMembers,
		"origin":     "app-agent",
		"local_only": true,
	}
	for k, v := range meta {
		if _, exists := pushMeta[k]; !exists {
			pushMeta[k] = v
		}
	}

	for _, member := range humanMembers {
		log.Printf("[Bridge] push group message group=%s to_member=%s from=%s type=%s",
			groupID, member, displayFrom, messageType)
		if err := b.sendAppPushWithType(member, content, messageType, pushMeta); err != nil {
			log.Printf("[Bridge] group push failed group=%s member=%s: %v", groupID, member, err)
		}
	}
	log.Printf("[Bridge] group message broadcast group=%s from=%s members=%d type=%s len=%d",
		groupID, displayFrom, len(humanMembers), messageType, len(content))
	return nil
}

func (b *Bridge) sendAppPush(toUser, content string, meta map[string]any) error {
	return b.sendAppPushWithType(toUser, content, "text", meta)
}

func (b *Bridge) sendAppPushWithType(toUser, content, messageType string, meta map[string]any) error {
	if strings.TrimSpace(toUser) == "" {
		return fmt.Errorf("empty user")
	}
	log.Printf("[Bridge] enqueue app push user=%s len=%d meta_keys=%d content=%q",
		toUser, len(content), len(meta), shortText(content))

	payload := AppPushPayload{
		UserID:      toUser,
		Content:     truncateForApp(content),
		MessageType: strings.TrimSpace(messageType),
		Channel:     "app",
		Timestamp:   time.Now().UnixMilli(),
		Meta:        meta,
	}
	if payload.MessageType == "" {
		payload.MessageType = "text"
	}
	return b.enqueueAndDeliver(payload)
}

func isBackendCommand(content string) bool {
	return strings.HasPrefix(content, "/cg") || strings.HasPrefix(content, "cg ") || content == "cg" ||
		strings.EqualFold(content, "reload prompts")
}

func getHelpText() string {
	return "Go Blog App commands\n\n" +
		"/help show help\n" +
		"/reset start a new conversation\n" +
		"/status show service status\n\n" +
		"/cg list list projects\n" +
		"/cg start <project> <request> start codegen\n" +
		"/cg status show progress\n" +
		"/cg stop stop codegen\n\n" +
		"Other messages will be forwarded to llm-agent."
}

func shortText(text string) string {
	const limit = 120
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}
