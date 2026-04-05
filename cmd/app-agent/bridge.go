package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"uap"
)

// AppMessage is the message pushed from the app side to app-agent.
type AppMessage struct {
	UserID          string         `json:"user_id"`
	Content         string         `json:"content"`
	MessageType     string         `json:"message_type,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	TraceID         string         `json:"trace_id,omitempty"`
	Meta            map[string]any `json:"meta,omitempty"`
	DelegationToken string         `json:"delegation_token,omitempty"` // delegation token for blog-agent API
}

// AppPushPayload is the payload pushed from app-agent back to the app side.
type AppPushPayload struct {
	MessageID   string         `json:"message_id,omitempty"`
	Sequence    int64          `json:"sequence"`
	UserID      string         `json:"user_id"`
	Content     string         `json:"content"`
	MessageType string         `json:"message_type"`
	Channel     string         `json:"channel"`
	Timestamp   int64          `json:"timestamp"`
	Meta        map[string]any `json:"meta,omitempty"`
}

type AppAttachment struct {
	MessageType  string         `json:"message_type"`
	FileID       string         `json:"file_id,omitempty"`
	FileName     string         `json:"file_name,omitempty"`
	FilePath     string         `json:"file_path,omitempty"`
	InlineBase64 string         `json:"inline_base64,omitempty"`
	FileSize     int            `json:"file_size,omitempty"`
	Format       string         `json:"format,omitempty"`
	MIMEType     string         `json:"mime_type,omitempty"`
	DurationMS   int            `json:"duration_ms,omitempty"`
	SpeechText   string         `json:"speech_text,omitempty"`
	InputMode    string         `json:"input_mode,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
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

	deliveryMu      sync.Mutex
	nextSequence    int64
	pendingMessages map[string]*pendingMessage
	pendingByUser   map[string][]string
	clients         map[string]map[*appClientConn]struct{}

	// delegation tokens by user
	delegationTokens map[string]string
	delegationMu     sync.Mutex
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
		{
			Name:        "app.SendRichMessage",
			Description: "Send a rich app message with text, image, audio, or file payload",
			Parameters: json.RawMessage(`{
				"type":"object",
				"properties":{
					"to_user":{"type":"string"},
					"content":{"type":"string"},
					"message_type":{"type":"string"},
					"meta":{"type":"object"}
				},
				"required":["to_user","message_type"]
			}`),
		},
	}
	client.Capacity = 20
	client.Meta = map[string]any{
		"http_port": cfg.HTTPPort,
	}

	b := &Bridge{
		cfg:              cfg,
		client:           client,
		groups:           newGroupManager(cfg.GroupStoreFile),
		lastEventTime:    make(map[string]time.Time),
		sessionUsers:     make(map[string]string),
		pendingMessages:  make(map[string]*pendingMessage),
		pendingByUser:    make(map[string][]string),
		clients:          make(map[string]map[*appClientConn]struct{}),
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
	total := 0
	for _, clients := range b.clients {
		total += len(clients)
	}
	return total
}

func (b *Bridge) PendingMessageCount() int {
	b.deliveryMu.Lock()
	defer b.deliveryMu.Unlock()

	total := 0
	for _, msg := range b.pendingMessages {
		for _, delivery := range msg.Deliveries {
			if delivery.AckedAt.IsZero() {
				total++
			}
		}
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
	msg.MessageType = normalizeAppMessageType(msg.MessageType, msg.Meta)
	content := strings.TrimSpace(msg.Content)
	if content == "" && msg.MessageType == "text" {
		return
	}
	log.Printf("[Bridge] inbound app message user=%s type=%s len=%d content=%q",
		msg.UserID, msg.MessageType, len(content), shortText(content))

	attachment, err := b.persistAttachment(msg)
	if err != nil {
		log.Printf("[Bridge] persist attachment failed user=%s type=%s err=%v", msg.UserID, msg.MessageType, err)
		_ = b.sendAppPush(msg.UserID, fmt.Sprintf("附件处理失败: %v", err), nil)
		return
	}

	if groupID := b.groupIDFromMeta(msg.Meta); groupID != "" {
		if err := b.handleGroupMessage(groupID, msg, attachment); err != nil {
			log.Printf("[Bridge] group broadcast failed user=%s group=%s: %v", msg.UserID, groupID, err)
			_ = b.sendAppPush(msg.UserID, fmt.Sprintf("Group message failed: %v", err), nil)
		}
		return
	}

	switch {
	case content == "/help" || content == "help" || content == "甯姪":
		if msg.MessageType != "text" {
			break
		}
		if err := b.sendAppPush(msg.UserID, getHelpText(), nil); err != nil {
			log.Printf("[Bridge] send help failed: %v", err)
		}
		return
	case content == "/status" || content == "status":
		if msg.MessageType != "text" {
			break
		}
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

	messageContent := b.buildAppContentForAgent(msg, attachment, "")
	if msg.DelegationToken != "" {
		messageContent = fmt.Sprintf("[delegation:%s]%s", msg.DelegationToken, messageContent)
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

func (b *Bridge) PushUploadedAPK(toUser, content, fileName string, src io.Reader) (*AppAttachment, error) {
	if strings.TrimSpace(toUser) == "" {
		return nil, fmt.Errorf("empty user")
	}
	attachment, err := b.storeUploadedAPK(toUser, fileName, src)
	if err != nil {
		return nil, err
	}
	if err := b.sendExistingAttachmentMessage(toUser, content, "file", nil, attachment); err != nil {
		return nil, err
	}
	return attachment, nil
}

func (b *Bridge) PushUploadedAPKToGroup(groupID, content, fileName string, src io.Reader) (*AppAttachment, []string, error) {
	groupID = normalizeGroupID(groupID)
	if groupID == "" {
		return nil, nil, fmt.Errorf("empty group")
	}
	robotAccount, ok := b.groups.RobotAccount(groupID)
	if !ok {
		return nil, nil, fmt.Errorf("group robot account not found")
	}
	humanMembers, err := b.groups.HumanMembers(groupID)
	if err != nil {
		return nil, nil, err
	}
	attachment, err := b.storeUploadedAPK(robotAccount, fileName, src)
	if err != nil {
		return nil, nil, err
	}
	pushMeta := buildPushMeta(nil, attachment)
	if attachment != nil && strings.TrimSpace(content) == "" {
		if attachment.FileName != "" {
			content = attachment.FileName
		} else {
			content = defaultAttachmentLabel(attachment)
		}
	}
	if err := b.broadcastGroupMessage(groupID, robotAccount, content, "file", pushMeta); err != nil {
		return nil, nil, err
	}
	return attachment, humanMembers, nil
}

func (b *Bridge) storeUploadedAPK(owner, fileName string, src io.Reader) (*AppAttachment, error) {
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("empty owner")
	}
	if strings.TrimSpace(fileName) == "" {
		return nil, fmt.Errorf("empty file name")
	}
	if src == nil {
		return nil, fmt.Errorf("empty file stream")
	}

	dir, err := b.ensureAttachmentDir(owner)
	if err != nil {
		return nil, err
	}
	safeName := sanitizeFileName(fileName)
	if !strings.HasSuffix(strings.ToLower(safeName), ".apk") {
		safeName += ".apk"
	}
	filePath := filepath.Join(dir, safeName)
	out, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create attachment failed: %w", err)
	}
	written, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("write attachment failed: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close attachment failed: %w", closeErr)
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve attachment abs path failed: %w", err)
	}
	fileID, err := buildAttachmentFileIDWithTimestamp(b.cfg.AttachmentStoreDir, filePath, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	attachment := &AppAttachment{
		MessageType: "file",
		FileID:      fileID,
		FileName:    safeName,
		FilePath:    absPath,
		FileSize:    int(written),
		Format:      "apk",
		MIMEType:    "application/vnd.android.package-archive",
	}
	return attachment, nil
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

func (b *Bridge) handleGroupMessage(groupID string, msg *AppMessage, attachment *AppAttachment) error {
	if msg == nil {
		return fmt.Errorf("empty message")
	}
	pushMeta := buildPushMeta(sanitizeAppMetaForPush(msg.Meta), attachment)
	if err := b.broadcastGroupMessage(groupID, msg.UserID, msg.Content, msg.MessageType, pushMeta); err != nil {
		return err
	}
	robotAccount, ok := b.groups.RobotAccount(groupID)
	if !ok {
		return fmt.Errorf("group robot account not found")
	}
	return b.forwardGroupMessageToLLM(groupID, msg.UserID, robotAccount, b.buildAppContentForAgent(msg, attachment, groupID))
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
	log.Printf("[Bridge] route group message to llm group=%s from=%s robot_account=%s len=%d content=%q",
		groupID, fromUser, robotAccount, len(llmContent), shortText(llmContent))
	if err := b.client.SendTo(b.cfg.LLMAgentID, uap.MsgNotify, payload); err != nil {
		return fmt.Errorf("send to %s failed: %w", b.cfg.LLMAgentID, err)
	}
	return nil
}

func normalizeAppMessageType(messageType string, meta map[string]any) string {
	mt := strings.TrimSpace(strings.ToLower(messageType))
	if mt != "" {
		switch mt {
		case "audio", "image", "text", "file", "zip", "archive", "video":
			return mt
		}
	}
	if meta == nil {
		return "text"
	}
	switch {
	case stringMeta(meta, "audio_base64") != "":
		return "audio"
	case stringMeta(meta, "image_base64") != "":
		return "image"
	case stringMeta(meta, "zip_base64") != "":
		return "zip"
	case stringMeta(meta, "file_base64") != "":
		if isZipFileName(stringMeta(meta, "file_name")) {
			return "zip"
		}
		return "file"
	default:
		return "text"
	}
}

func (b *Bridge) persistAttachment(msg *AppMessage) (*AppAttachment, error) {
	if msg == nil || msg.Meta == nil {
		return nil, nil
	}
	base64Text, fileName, format := attachmentPayload(msg.MessageType, msg.Meta)
	if base64Text == "" {
		mimeType := attachmentMimeType(msg.MessageType, fileName, format)
		return &AppAttachment{
			MessageType: msg.MessageType,
			FileName:    fileName,
			Format:      format,
			MIMEType:    mimeType,
			DurationMS:  intMeta(msg.Meta, "duration_ms"),
			SpeechText:  stringMeta(msg.Meta, "speech_text"),
			InputMode:   stringMeta(msg.Meta, "input_mode"),
			Meta:        sanitizeAppMetaForForward(msg.Meta),
		}, nil
	}

	data, err := base64.StdEncoding.DecodeString(base64Text)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	dir, err := b.ensureAttachmentDir(msg.UserID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = buildAttachmentFileName(msg.MessageType, format)
	}
	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("write attachment failed: %w", err)
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve attachment abs path failed: %w", err)
	}
	fileID, err := buildAttachmentFileIDWithTimestamp(b.cfg.AttachmentStoreDir, filePath, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	mimeType := attachmentMimeType(msg.MessageType, fileName, format)

	return &AppAttachment{
		MessageType: msg.MessageType,
		FileID:      fileID,
		FileName:    fileName,
		// 这里必须传绝对路径，避免 llm-agent 与 app-agent 工作目录不一致时读不到附件。
		FilePath:     absPath,
		InlineBase64: base64Text,
		FileSize:     len(data),
		Format:       format,
		MIMEType:     mimeType,
		DurationMS:   intMeta(msg.Meta, "duration_ms"),
		SpeechText:   stringMeta(msg.Meta, "speech_text"),
		InputMode:    stringMeta(msg.Meta, "input_mode"),
		Meta:         sanitizeAppMetaForForward(msg.Meta),
	}, nil
}

func (b *Bridge) ensureAttachmentDir(userID string) (string, error) {
	root := attachmentRootDir(b.cfg.AttachmentStoreDir)
	dir := filepath.Join(root, sanitizeFileName(userID), time.Now().Format("20060102"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir attachment dir failed: %w", err)
	}
	return dir, nil
}

func (b *Bridge) buildAppContentForAgent(msg *AppMessage, attachment *AppAttachment, groupID string) string {
	if msg == nil {
		return ""
	}
	payload := map[string]any{
		"kind":         "app_message",
		"user_id":      msg.UserID,
		"message_type": msg.MessageType,
		"content":      strings.TrimSpace(msg.Content),
	}
	if groupID != "" {
		payload["scope"] = "group"
		payload["group_id"] = groupID
	} else {
		payload["scope"] = "direct"
	}
	if attachment != nil {
		payload["attachment"] = attachment
	}
	if msg.Meta != nil {
		payload["meta"] = sanitizeAppMetaForForward(msg.Meta)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return strings.TrimSpace(msg.Content)
	}
	return "APP_MESSAGE_JSON:\n" + string(data)
}

func attachmentPayload(messageType string, meta map[string]any) (base64Text string, fileName string, format string) {
	if meta == nil {
		return "", "", ""
	}
	fileName = stringMeta(meta, "file_name")
	format = stringMeta(meta, "audio_format")
	switch messageType {
	case "audio":
		return stringMeta(meta, "audio_base64"), fileName, format
	case "image":
		return stringMeta(meta, "image_base64"), fileName, stringMeta(meta, "image_format")
	case "zip":
		return firstNonEmpty(stringMeta(meta, "zip_base64"), stringMeta(meta, "file_base64")), fileName, "zip"
	case "archive", "file", "video":
		return stringMeta(meta, "file_base64"), fileName, firstNonEmpty(stringMeta(meta, "file_format"), format)
	default:
		return firstNonEmpty(
			stringMeta(meta, "file_base64"),
			stringMeta(meta, "image_base64"),
			stringMeta(meta, "audio_base64"),
			stringMeta(meta, "zip_base64"),
		), fileName, firstNonEmpty(stringMeta(meta, "file_format"), format)
	}
}

func sanitizeAppMetaForForward(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		switch k {
		case "audio_base64", "image_base64", "file_base64", "zip_base64":
			out[k+"_present"] = true
		default:
			out[k] = v
		}
	}
	return out
}

func sanitizeAppMetaForPush(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		switch k {
		case "audio_base64", "image_base64", "file_base64", "zip_base64":
			continue
		default:
			out[k] = v
		}
	}
	return out
}

func buildAttachmentFileName(messageType, format string) string {
	ext := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(format)), ".")
	if ext == "" {
		switch strings.TrimSpace(strings.ToLower(messageType)) {
		case "audio":
			ext = "bin"
		case "image":
			ext = "png"
		case "zip", "archive":
			ext = "zip"
		default:
			ext = "bin"
		}
	}
	return fmt.Sprintf("%s_%d.%s", sanitizeFileName(messageType), time.Now().UnixMilli(), ext)
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return replacer.Replace(name)
}

func stringMeta(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, _ := meta[key].(string)
	return strings.TrimSpace(v)
}

func intMeta(meta map[string]any, key string) int {
	if meta == nil {
		return 0
	}
	switch v := meta[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func isZipFileName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".7z") || strings.HasSuffix(lower, ".rar") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".gz")
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
		if err := b.sendNotificationMessage(strings.TrimSpace(args.ToUser), strings.TrimSpace(args.Content), "text", nil); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("send failed: %v", err))
		} else {
			result = uap.BuildToolResult(msg.ID, nil, "message queued")
		}
	case "app.SendRichMessage":
		var args struct {
			ToUser      string         `json:"to_user"`
			Content     string         `json:"content"`
			MessageType string         `json:"message_type"`
			Meta        map[string]any `json:"meta"`
		}
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("invalid arguments: %v", err))
			break
		}
		if err := b.sendNotificationMessage(
			strings.TrimSpace(args.ToUser),
			strings.TrimSpace(args.Content),
			normalizeAppMessageType(args.MessageType, args.Meta),
			args.Meta,
		); err != nil {
			result = uap.BuildToolError(msg.ID, fmt.Sprintf("send failed: %v", err))
		} else {
			result = uap.BuildToolResult(msg.ID, nil, "rich message queued")
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
	if err := b.sendNotificationMessage(toUser, content, "text", nil); err != nil {
		log.Printf("[Bridge] app push failed for user=%s: %v", toUser, err)
	}
}

func (b *Bridge) sendNotificationMessage(toUser, content, messageType string, meta map[string]any) error {
	if toUser == "" {
		log.Printf("[Bridge] skip notification: empty user")
		return nil
	}
	messageType = normalizeAppMessageType(messageType, meta)
	pushMeta := cloneMeta(meta)
	var attachment *AppAttachment
	var err error
	if pushMeta != nil {
		attachment, err = b.persistAttachment(&AppMessage{
			UserID:      toUser,
			Content:     content,
			MessageType: messageType,
			Meta:        pushMeta,
		})
		if err != nil {
			return err
		}
		pushMeta = sanitizeAppMetaForPush(pushMeta)
	}
	return b.sendExistingAttachmentMessage(toUser, content, messageType, pushMeta, attachment)
}

func (b *Bridge) sendExistingAttachmentMessage(toUser, content, messageType string, meta map[string]any, attachment *AppAttachment) error {
	pushMeta := buildPushMeta(meta, attachment)
	if attachment != nil && strings.TrimSpace(content) == "" {
		if attachment.FileName != "" {
			content = attachment.FileName
		} else {
			content = defaultAttachmentLabel(attachment)
		}
	}
	if groupID, ok := b.groups.GroupIDByRobotAccount(toUser); ok {
		log.Printf("[Bridge] robot notification routed account=%s -> group=%s len=%d content=%q",
			toUser, groupID, len(content), shortText(content))
		groupMeta := map[string]any{
			"scope":     "group",
			"group_id":  groupID,
			"from_user": groupRobotDisplayName,
			"origin":    "llm-agent",
			"account":   toUser,
		}
		for k, v := range pushMeta {
			if _, exists := groupMeta[k]; !exists {
				groupMeta[k] = v
			}
		}
		if err := b.broadcastGroupMessage(groupID, toUser, content, messageType, groupMeta); err != nil {
			log.Printf("[Bridge] robot group broadcast failed group=%s account=%s: %v", groupID, toUser, err)
		}
		return nil
	}
	log.Printf("[Bridge] deliver notification user=%s len=%d content=%q", toUser, len(content), shortText(content))
	return b.sendAppPushWithType(toUser, content, messageType, pushMeta)
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
	recipients := make([]string, 0, len(humanMembers))
	for _, member := range humanMembers {
		if member == fromUser {
			continue
		}
		recipients = append(recipients, member)
	}
	if displayFrom == groupRobotDisplayName {
		recipients = humanMembers
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

	messageID := buildPushMessageID(groupID)
	for _, member := range recipients {
		log.Printf("[Bridge] queue group message group=%s to_member=%s from=%s type=%s message_id=%s",
			groupID, member, displayFrom, messageType, messageID)
	}
	if len(recipients) == 0 {
		log.Printf("[Bridge] skip group delivery group=%s from=%s: no recipient after sender exclusion",
			groupID, displayFrom)
		return nil
	}
	if err := b.enqueueAndDeliverMany(recipients, AppPushPayload{
		MessageID:   messageID,
		Content:     truncateForApp(content),
		MessageType: strings.TrimSpace(messageType),
		Channel:     "app",
		Timestamp:   time.Now().UnixMilli(),
		Meta:        pushMeta,
	}); err != nil {
		return err
	}
	log.Printf("[Bridge] group message broadcast group=%s from=%s members=%d type=%s len=%d",
		groupID, displayFrom, len(recipients), messageType, len(content))
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
		MessageID:   buildPushMessageID(toUser),
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

func buildPushMeta(baseMeta map[string]any, attachment *AppAttachment) map[string]any {
	out := cloneMeta(baseMeta)
	if attachment == nil {
		return out
	}
	if out == nil {
		out = make(map[string]any)
	}
	if attachment.FileID != "" {
		out["file_id"] = attachment.FileID
	}
	if attachment.FileName != "" {
		out["file_name"] = attachment.FileName
	}
	if attachment.FileSize > 0 {
		out["file_size"] = attachment.FileSize
	}
	if attachment.Format != "" {
		switch attachment.MessageType {
		case "audio":
			out["audio_format"] = attachment.Format
		case "image":
			out["image_format"] = attachment.Format
		default:
			out["file_format"] = attachment.Format
		}
	}
	if attachment.MIMEType != "" {
		out["mime_type"] = attachment.MIMEType
	}
	if attachment.DurationMS > 0 {
		out["duration_ms"] = attachment.DurationMS
	}
	if attachment.SpeechText != "" {
		out["speech_text"] = attachment.SpeechText
	}
	if attachment.InputMode != "" {
		out["input_mode"] = attachment.InputMode
	}
	return out
}

func defaultAttachmentLabel(attachment *AppAttachment) string {
	if attachment == nil {
		return ""
	}
	switch attachment.MessageType {
	case "image":
		return "[Image]"
	case "audio":
		if attachment.DurationMS > 0 {
			return fmt.Sprintf("[Voice %.1fs]", float64(attachment.DurationMS)/1000)
		}
		return "[Voice]"
	case "zip", "archive":
		return "[Archive]"
	case "file":
		return "[File]"
	default:
		if attachment.FileName != "" {
			return attachment.FileName
		}
		return "[Attachment]"
	}
}

func buildPushMessageID(userID string) string {
	return fmt.Sprintf("%s-%d", sanitizeFileName(userID), time.Now().UnixNano())
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
