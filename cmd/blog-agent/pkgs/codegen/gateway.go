package codegen

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"net/http"
	"strings"
	"sync"
	"time"

	"agentbase"
	"uap"
)

// MCPToolInfo MCP 工具定义信息（用于依赖注入，避免直接依赖 mcp 包）
type MCPToolInfo struct {
	Name        string      // MCP 回调名（如 RawAllBlogName）
	Description string      // 工具描述
	Parameters  interface{} // JSON Schema 参数
}

// MCP 桥接函数（由 agent 包注入，避免 codegen 直接依赖 mcp 的重量级传递依赖链）
var (
	// MCPCallInnerTools 调用 MCP 内部工具
	MCPCallInnerTools func(name string, args map[string]interface{}) string
	// MCPGetToolInfos 获取 MCP 工具定义列表
	MCPGetToolInfos func() []MCPToolInfo

	// Delegation Token 相关的函数（由 agent 包注入）
	// ParseDelegationTokenFromHeader 解析 delegation token
	ParseDelegationTokenFromHeader func(header string) (DelegationTokenHolder, error)
	// SetDelegationToken 设置 delegation token 到上下文（本地存储）
	SetDelegationToken func(key string, token DelegationTokenHolder)
	// GetDelegationToken 从上下文获取 delegation token（本地存储）
	GetDelegationToken func(key string) DelegationTokenHolder
	// VerifyDelegationToken 验证 delegation token
	VerifyDelegationToken func(token DelegationTokenHolder) (string, error)

	// PrepareMCPContext 准备 MCP 调用上下文（由 agent 包注入）
	// 在调用 tool 前调用，设置 currentRequestID 和 delegation token 到 mcp 包
	PrepareMCPContext func(requestID string, account string)
)

// DelegationTokenHolder delegation token 接口（用于避免直接依赖 mcp 包）
type DelegationTokenHolder interface {
	GetTargetAccount() string
}

// delegationTokenStore 本地 token 存储（已废弃，仅保留兼容）
// 注意：现在使用 currentDelegationToken 存储当前有效的 token
var delegationTokenStore = make(map[string]DelegationTokenHolder)

// currentDelegationToken 当前有效的 delegation token
// 在 app-agent 发送消息时设置，每个 tool call 都必须验证此 token
var currentDelegationToken DelegationTokenHolder

// initDelegationTokenStore 初始化 delegation token 存储
func initDelegationTokenStore() {
	// 设置本地存储函数
	SetDelegationToken = func(key string, token DelegationTokenHolder) {
		// 存储到 map（兼容旧代码）
		delegationTokenStore[key] = token
		// 同时设置为当前有效 token
		currentDelegationToken = token
	}
	GetDelegationToken = func(key string) DelegationTokenHolder {
		// 优先返回当前有效 token
		if currentDelegationToken != nil {
			return currentDelegationToken
		}
		return delegationTokenStore[key]
	}
}

// GatewaySender 通过 gateway 路由发送消息
type GatewaySender struct {
	client    *uap.Client
	toAgentID string // 目标 agent ID（codegen-agent / deploy-agent 的 UAP ID）
}

// SendAgentMsg 通过 gateway 路由发送 AgentMessage
func (s *GatewaySender) SendAgentMsg(msgType string, payload interface{}) error {
	return s.client.SendTo(s.toAgentID, msgType, payload)
}

// TaskEvent assistant 任务事件（投递给监听者）
type TaskEvent struct {
	Event    string // "chunk" | "tool_info" | "complete" | "error"
	Text     string
	TaskID   string
	Complete bool   // 是否为最终事件
	Error    string // 仅 complete 时有效
}

type SessionRoute struct {
	AgentID string
	Channel string
	Account string
}

type GatewayBridgeOptions struct {
	AgentID      string
	AgentType    string
	AgentName    string
	Description  string
	WorkspaceDir string
}

// GatewayBridge blog-agent 的 gateway 适配层
type GatewayBridge struct {
	client      *uap.Client
	pool        *AgentPool
	gatewayHTTP string // gateway HTTP 地址（如 http://127.0.0.1:9000）

	// wechat notify 处理器
	wechatHandler func(wechatUser, message string) string

	// UAP tool_name → MCP callback name 映射
	toolMapping map[string]string

	// assistant 任务事件通道（taskID → event channel）
	taskEventChannels map[string]chan TaskEvent
	taskEventMu       sync.Mutex

	sessionRoutes map[string]SessionRoute
	routeMu       sync.RWMutex

	mu sync.Mutex
}

// 全局 gateway bridge 实例
var gatewayBridge *GatewayBridge

// InitGatewayBridge 初始化 blog-agent 到 gateway 的连接
func InitGatewayBridge(gatewayURL, authToken, workspaceDir string) {
	InitGatewayBridgeWithOptions(gatewayURL, authToken, GatewayBridgeOptions{
		AgentID:      "blog-agent",
		AgentType:    "blog-agent",
		AgentName:    "Go Blog Server",
		Description:  "博客CRUD、待办清单、运动记录、阅读管理、年度计划、星座占卜、实用工具、游戏",
		WorkspaceDir: workspaceDir,
	})
}

// InitGatewayBridgeWithOptions 初始化自定义 agent 身份的 gateway 连接。
func InitGatewayBridgeWithOptions(gatewayURL, authToken string, opts GatewayBridgeOptions) {
	// 统一 token：gateway_token 同时作为 agent 认证 token
	if authToken != "" {
		agentToken = authToken
	}

	// 构建工具定义和映射表
	toolDefs, toolMapping := buildToolDefs()

	agentID := strings.TrimSpace(opts.AgentID)
	if agentID == "" {
		agentID = "blog-agent"
	}
	agentType := strings.TrimSpace(opts.AgentType)
	if agentType == "" {
		agentType = agentID
	}
	agentName := strings.TrimSpace(opts.AgentName)
	if agentName == "" {
		agentName = agentID
	}
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		description = agentName
	}

	client := uap.NewClient(gatewayURL, agentID, agentType, agentName)
	client.AuthToken = authToken
	client.Description = description
	client.Capacity = 100
	client.Tools = toolDefs
	client.Meta = map[string]any{
		"role": "backend",
	}

	// 加载 workspace 描述
	if opts.WorkspaceDir != "" {
		ws := agentbase.LoadWorkspace(opts.WorkspaceDir)
		if ws.Summary != "" {
			client.Description = ws.Summary
		}
		if client.Meta == nil {
			client.Meta = make(map[string]any)
		}
		if ws.Detail != "" {
			client.Meta["agent_description"] = ws.Detail
		}
	}

	// 从 WebSocket URL 推导 HTTP URL（ws://host:port/ws/uap → http://host:port）
	httpURL := gatewayURL
	httpURL = strings.Replace(httpURL, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)
	if idx := strings.Index(httpURL, "/ws/"); idx > 0 {
		httpURL = httpURL[:idx]
	}

	bridge := &GatewayBridge{
		client:            client,
		pool:              agentPool,
		gatewayHTTP:       httpURL,
		toolMapping:       toolMapping,
		taskEventChannels: make(map[string]chan TaskEvent),
		sessionRoutes:     make(map[string]SessionRoute),
	}

	client.OnMessage = bridge.handleMessage

	gatewayBridge = bridge

	// 初始化 WeChat Bridge（通过 gateway 路由发送微信消息）
	// 使 cg start 等命令可以通过 StartSessionForWeChat 启动编码会话并推送进度
	InitWeChatBridge(func(toUser, content string) error {
		return bridge.sendWechatViaGateway(toUser, content)
	})

	// 后台连接 gateway（非阻塞）
	go func() {
		log.MessageF(log.ModuleAgent, "CodeGen: connecting to gateway at %s (tools=%d)", gatewayURL, len(toolDefs))
		client.Run()
	}()
}

// SetWechatHandler 设置微信命令处理器
func SetWechatHandler(handler func(wechatUser, message string) string) {
	if gatewayBridge != nil {
		gatewayBridge.mu.Lock()
		gatewayBridge.wechatHandler = handler
		gatewayBridge.mu.Unlock()
	}
}

// GetGatewayClient 获取 gateway 客户端（供外部使用）
func GetGatewayClient() *uap.Client {
	if gatewayBridge != nil {
		return gatewayBridge.client
	}
	return nil
}

func (b *GatewayBridge) setSessionRoute(sessionID string, route SessionRoute) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || strings.TrimSpace(route.AgentID) == "" {
		return
	}
	route.Account = strings.TrimSpace(route.Account)
	route.Channel = strings.TrimSpace(route.Channel)
	b.routeMu.Lock()
	b.sessionRoutes[sessionID] = route
	b.routeMu.Unlock()
}

func (b *GatewayBridge) sessionRoute(sessionID string) (SessionRoute, bool) {
	b.routeMu.RLock()
	defer b.routeMu.RUnlock()
	route, ok := b.sessionRoutes[sessionID]
	return route, ok
}

func (b *GatewayBridge) clearSessionRoute(sessionID string) {
	b.routeMu.Lock()
	delete(b.sessionRoutes, sessionID)
	b.routeMu.Unlock()
}

func (b *GatewayBridge) updateUserSessionRoute(userID, channel, sourceAgentID string) {
	if userID == "" || sourceAgentID == "" {
		return
	}
	sessionID := GetUserSessionID(userID)
	if sessionID == "" {
		return
	}
	b.setSessionRoute(sessionID, SessionRoute{
		AgentID: sourceAgentID,
		Channel: channel,
		Account: userID,
	})
}

// handleMessage 处理从 gateway 收到的 UAP 消息
func (b *GatewayBridge) handleMessage(msg *uap.Message) {
	// 解析 AgentMessage payload（codegen/deploy agent 发来的原协议消息）
	switch msg.Type {
	case MsgRegister:
		b.handleRegister(msg)

	case MsgHeartbeat:
		b.handleHeartbeat(msg)

	case MsgTaskAccepted:
		// 兼容 codegen 协议（SessionID）和 UAP 协议（TaskID）
		var raw struct {
			SessionID string `json:"session_id"`
			TaskID    string `json:"task_id"`
		}
		if err := json.Unmarshal(msg.Payload, &raw); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_accepted payload: %v", err)
			return
		}
		if raw.SessionID != "" {
			// codegen agent
			agent := b.getAgent(msg.From)
			if agent != nil {
				agent.mu.Lock()
				agent.ActiveSessions[raw.SessionID] = true
				agent.mu.Unlock()
			}
			log.MessageF(log.ModuleAgent, "CodeGen: gateway agent %s accepted task %s", msg.From, raw.SessionID)
		} else if raw.TaskID != "" {
			// llm-agent assistant 任务
			log.MessageF(log.ModuleAgent, "CodeGen gateway: llm-agent accepted task %s", raw.TaskID)
		}

	case MsgTaskRejected:
		var payload TaskRejectedPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_rejected payload: %v", err)
			return
		}
		log.WarnF(log.ModuleAgent, "CodeGen: gateway agent %s rejected task %s: %s",
			msg.From, payload.SessionID, payload.Reason)
		if session := GetSession(payload.SessionID); session != nil {
			session.mu.Lock()
			session.Status = StatusError
			session.Error = "agent rejected: " + payload.Reason
			session.EndTime = time.Now()
			session.mu.Unlock()
			session.broadcast(StreamEvent{
				Type: "error",
				Text: "❌ Agent 拒绝任务: " + payload.Reason,
				Done: true,
			})
		}

	case MsgStreamEvent:
		var payload StreamEventPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid stream_event payload: %v", err)
			return
		}
		b.pool.handleStreamEvent(&payload)
		b.forwardTaskUpdate(msg.Type, payload.SessionID, msg.Payload)

	case MsgTaskComplete:
		// 兼容 codegen 协议（SessionID）和 UAP 协议（TaskID）
		var raw struct {
			SessionID string `json:"session_id"`
			TaskID    string `json:"task_id"`
			Status    string `json:"status"`
			Error     string `json:"error"`
			Result    string `json:"result"`
		}
		if err := json.Unmarshal(msg.Payload, &raw); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_complete payload: %v", err)
			return
		}
		if raw.TaskID != "" {
			// llm-agent assistant 任务完成 → 投递到 taskEventChannels
			b.taskEventMu.Lock()
			ch, ok := b.taskEventChannels[raw.TaskID]
			b.taskEventMu.Unlock()
			if ok {
				select {
				case ch <- TaskEvent{
					Event:    "complete",
					TaskID:   raw.TaskID,
					Text:     raw.Result,
					Complete: true,
					Error:    raw.Error,
				}:
				default:
				}
			}
		}
		if raw.SessionID != "" {
			// codegen agent 任务完成
			var payload TaskCompletePayload
			json.Unmarshal(msg.Payload, &payload)
			agent := b.getAgent(msg.From)
			b.pool.handleTaskComplete(agent, &payload)
			b.forwardTaskUpdate(msg.Type, payload.SessionID, msg.Payload)
			b.clearSessionRoute(payload.SessionID)
		}

	case MsgFileReadResp, MsgTreeReadResp, MsgProjectCreateResp:
		var base struct {
			RequestID string `json:"request_id"`
		}
		json.Unmarshal(msg.Payload, &base)
		b.pool.pendMu.Lock()
		if ch, ok := b.pool.pending[base.RequestID]; ok {
			ch <- msg.Payload
			delete(b.pool.pending, base.RequestID)
		}
		b.pool.pendMu.Unlock()

	case uap.MsgNotify:
		go b.handleNotify(msg) // 异步处理，避免 LLM 处理阻塞消息循环

	case uap.MsgToolCall:
		go b.handleToolCall(msg) // 异步处理，避免阻塞消息循环

	case uap.MsgError:
		var payload uap.ErrorPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid error payload: %v", err)
			return
		}
		log.WarnF(log.ModuleAgent, "CodeGen gateway: error from %s: [%s] %s", msg.From, payload.Code, payload.Message)

		// agent_offline: 释放对应的 pending 请求
		if payload.Code == "agent_offline" && msg.ID != "" {
			b.pool.pendMu.Lock()
			if ch, ok := b.pool.pending[msg.ID]; ok {
				close(ch)
				delete(b.pool.pending, msg.ID)
			}
			b.pool.pendMu.Unlock()
		}

	case uap.MsgTaskEvent:
		// llm-agent 发来的 assistant 任务进度事件
		var taskEventPayload uap.TaskEventPayload
		if err := json.Unmarshal(msg.Payload, &taskEventPayload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid uap task_event payload: %v", err)
			return
		}
		// 解析内部事件
		var evt struct {
			Event string `json:"event"`
			Text  string `json:"text"`
		}
		if err := json.Unmarshal(taskEventPayload.Event, &evt); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid assistant event data: %v", err)
			return
		}
		// 投递到 pending channel
		b.taskEventMu.Lock()
		ch, ok := b.taskEventChannels[taskEventPayload.TaskID]
		b.taskEventMu.Unlock()
		if ok {
			select {
			case ch <- TaskEvent{Event: evt.Event, Text: evt.Text, TaskID: taskEventPayload.TaskID}:
			default:
				// channel 满了，丢弃（不阻塞消息循环）
				log.WarnF(log.ModuleAgent, "CodeGen gateway: task event channel full, dropping event for task %s", taskEventPayload.TaskID)
			}
		}

	default:
		log.WarnF(log.ModuleAgent, "CodeGen gateway: unhandled message type=%s from=%s", msg.Type, msg.From)
	}
}

// handleRegister 处理 codegen/deploy agent 的注册消息
func (b *GatewayBridge) handleRegister(msg *uap.Message) {
	var payload RegisterPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid register payload: %v", err)
		return
	}

	// 验证 token
	if agentToken != "" && payload.AuthToken != agentToken {
		b.client.SendTo(msg.From, MsgRegisterAck, RegisterAckPayload{
			Success: false, Error: "invalid auth token",
		})
		return
	}

	// 检查同名 agent
	if existing := b.pool.findOnlineAgentByName(payload.Name); existing != nil {
		b.client.SendTo(msg.From, MsgRegisterAck, RegisterAckPayload{
			Success: false,
			Error:   fmt.Sprintf("agent '%s' already connected (id=%s), reject duplicate", payload.Name, existing.ID),
		})
		log.WarnF(log.ModuleAgent, "CodeGen gateway: reject duplicate agent name=%s, existing id=%s, new id=%s",
			payload.Name, existing.ID, payload.AgentID)
		return
	}

	// 使用 From 作为 agent ID（gateway 填充的 UAP agent ID）
	agentID := msg.From
	if agentID == "" {
		agentID = payload.AgentID
	}

	agent := &RemoteAgent{
		ID:        agentID,
		Name:      payload.Name,
		AgentType: payload.AgentType,
		Sender: &GatewaySender{
			client:    b.client,
			toAgentID: agentID,
		},
		Conn:             nil, // gateway 模式无直连
		Workspaces:       payload.Workspaces,
		Projects:         []string(payload.Projects),
		Models:           payload.Models,
		ClaudeCodeModels: payload.ClaudeCodeModels,
		OpenCodeModels:   payload.OpenCodeModels,
		Tools:            payload.Tools,
		DeployTargets:    payload.DeployTargets,
		HostPlatform:     payload.HostPlatform,
		Pipelines:        payload.Pipelines,
		MaxConcurrent:    payload.MaxConcurrent,
		ActiveSessions:   make(map[string]bool),
		LastHeartbeat:    time.Now(),
		Status:           "online",
	}
	b.pool.addAgent(agent)

	b.client.SendTo(agentID, MsgRegisterAck, RegisterAckPayload{Success: true})
	log.MessageF(log.ModuleAgent, "CodeGen gateway: agent registered via gateway: %s (%s), projects=%v",
		agentID, agent.Name, agent.Projects)
}

// handleHeartbeat 处理心跳
func (b *GatewayBridge) handleHeartbeat(msg *uap.Message) {
	var payload HeartbeatPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return
	}

	agent := b.getAgent(msg.From)
	if agent == nil {
		// agent 不在 pool 中（可能 blog-agent 重启过），通知它重新注册
		log.WarnF(log.ModuleAgent, "CodeGen gateway: heartbeat from unknown agent %s, asking to re-register", msg.From)
		b.client.SendTo(msg.From, MsgRegisterAck, RegisterAckPayload{
			Success: false,
			Error:   "not_registered",
		})
		return
	}

	agent.mu.Lock()
	agent.LastHeartbeat = time.Now()
	if len(payload.Projects) > 0 {
		agent.Projects = []string(payload.Projects)
	}
	if len(payload.Models) > 0 {
		agent.Models = payload.Models
	}
	if len(payload.ClaudeCodeModels) > 0 {
		agent.ClaudeCodeModels = payload.ClaudeCodeModels
	}
	if len(payload.OpenCodeModels) > 0 {
		agent.OpenCodeModels = payload.OpenCodeModels
	}
	if len(payload.Tools) > 0 {
		agent.Tools = payload.Tools
	}
	agent.mu.Unlock()
	b.client.SendTo(msg.From, MsgHeartbeatAck, struct{}{})
}

// handleNotify 处理通知消息（来自 gateway 或 wechat-agent）
func (b *GatewayBridge) handleNotify(msg *uap.Message) {
	// 先尝试解析为通用事件（gateway 广播的 agent_offline 等）
	var event struct {
		Event     string `json:"event"`
		AgentID   string `json:"agent_id"`
		AgentType string `json:"agent_type"`
		AgentName string `json:"agent_name"`
	}
	if err := json.Unmarshal(msg.Payload, &event); err == nil && event.Event == "agent_offline" {
		log.MessageF(log.ModuleAgent, "CodeGen gateway: agent offline notification: %s (%s)", event.AgentID, event.AgentName)
		b.pool.removeAgent(event.AgentID)
		return
	}

	// 解析为 NotifyPayload（wechat 等）
	var payload uap.NotifyPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid notify payload: %v", err)
		return
	}

	// ====== 处理 app channel 的消息（来自 app-agent）======
	if payload.Channel == "app" {
		b.handleAppNotify(msg, &payload)
		return
	}

	if payload.Channel != "wechat" {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: unsupported notify channel: %s", payload.Channel)
		return
	}

	b.mu.Lock()
	handler := b.wechatHandler
	b.mu.Unlock()

	if handler == nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: wechat handler not set, dropping message from %s", payload.To)
		return
	}

	// 调用 handleWechatCommand，获取结果
	result := handler(payload.To, payload.Content)
	b.updateUserSessionRoute(payload.To, payload.Channel, msg.From)

	// 回发结果给 wechat-agent
	b.client.SendTo(msg.From, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      payload.To,
		Content: result,
	})
}

// handleAppNotify 处理来自 app-agent 的通知消息
// 提取并缓存 delegation token，以便后续 tool call 使用
func (b *GatewayBridge) handleAppNotify(msg *uap.Message, payload *uap.NotifyPayload) {
	// 从消息内容中提取 delegation token（格式：[delegation:xxx]actual content）
	delegationToken := ""
	content := payload.Content

	log.MessageF(log.ModuleAgent, "CodeGen gateway: handleAppNotify from=%s to=%s channel=%s",
		msg.From, payload.To, payload.Channel)

	if strings.HasPrefix(content, "[delegation:") {
		// 查找匹配的 ]
		endIdx := strings.Index(content, "]")
		if endIdx > 13 { // "[delegation:" 长度为 13
			delegationToken = content[13:endIdx]
			// 实际的聊天内容
			content = content[endIdx+1:]
		}
	}

	// 设置 delegation token 到 MCP 包的上下文中
	// 设置到 currentDelegationToken，后续 tool call 必须验证 account 匹配
	if delegationToken != "" {
		if tokenObj, err := ParseDelegationTokenFromHeader(delegationToken); err == nil {
			// 使用 token 中的 target account 作为 key
			targetAccount := tokenObj.GetTargetAccount()
			SetDelegationToken(targetAccount, tokenObj)
			log.MessageF(log.ModuleAgent, "CodeGen gateway: cached token: payload.To=%s token.TargetAccount=%s",
				payload.To, targetAccount)
		} else {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: failed to parse delegation token: %v", err)
		}
	} else {
		log.MessageF(log.ModuleAgent, "CodeGen gateway: no delegation token in message from %s", payload.To)
	}

	// 注意：这里只是缓存 token，实际的 tool call 验证在 handleToolCall 中进行
	log.MessageF(log.ModuleAgent, "CodeGen gateway: app notify from=%s content_len=%d", msg.From, len(content))

	reply, ok := b.buildAppNotifyReply(payload.To, content)
	if !ok {
		return
	}
	b.updateUserSessionRoute(payload.To, payload.Channel, msg.From)

	if err := b.client.SendTo(msg.From, uap.MsgNotify, uap.NotifyPayload{
		Channel: "app",
		To:      payload.To,
		Content: reply,
	}); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: send app notify reply failed: %v", err)
	}
}

func (b *GatewayBridge) buildAppNotifyReply(appUser, content string) (string, bool) {
	content = normalizeCodegenCommand(content)
	if strings.TrimSpace(content) == "" {
		return "", false
	}

	b.mu.Lock()
	handler := b.wechatHandler
	b.mu.Unlock()

	if handler == nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: wechat handler not set, dropping app message for %s", appUser)
		return "", false
	}

	result := strings.TrimSpace(handler(appUser, content))
	if result == "" {
		result = "⚠️ 后端未返回结果"
	}
	return result, true
}

// sendWechatViaGateway 通过 gateway 路由发送微信消息给用户
// 发送 notify 消息到 wechat-agent，由其调用企业微信 API
func (b *GatewayBridge) sendWechatViaGateway(toUser, content string) error {
	wechatAgentID := "wechat-wechat-agent"
	return b.client.SendTo(wechatAgentID, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      toUser,
		Content: content,
	})
}

func (b *GatewayBridge) forwardTaskUpdate(msgType, sessionID string, raw json.RawMessage) {
	route, ok := b.sessionRoute(sessionID)
	if !ok || route.AgentID == "" {
		return
	}
	payload := enrichPayloadWithRoute(raw, route)
	if err := b.client.Send(&uap.Message{
		Type:    msgType,
		ID:      uap.NewMsgID(),
		From:    b.client.AgentID,
		To:      route.AgentID,
		Payload: payload,
		Ts:      time.Now().UnixMilli(),
	}); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: forward %s failed session=%s to=%s: %v", msgType, sessionID, route.AgentID, err)
	}
}

// enrichPayloadWithRoute 尝试给流式任务 payload 注入 account/channel，便于客户端回路由。
func enrichPayloadWithRoute(raw json.RawMessage, route SessionRoute) json.RawMessage {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	if route.Account != "" {
		m["account"] = route.Account
	}
	if route.Channel != "" {
		m["channel"] = route.Channel
	}
	enriched, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return enriched
}

// getAgent 从 pool 中获取 agent
func (b *GatewayBridge) getAgent(agentID string) *RemoteAgent {
	b.pool.mu.RLock()
	agent := b.pool.agents[agentID]
	b.pool.mu.RUnlock()
	return agent
}

// handleToolCall 处理跨 agent 工具调用请求
func (b *GatewayBridge) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid tool_call payload: %v", err)
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid tool_call payload",
		})
		return
	}

	// 查映射表：UAP tool_name → MCP callback name
	mcpName, ok := b.toolMapping[payload.ToolName]
	if !ok {
		// 兼容：直接用原名尝试（可能调用者已经使用 MCP 回调名）
		mcpName = payload.ToolName
	}

	// 解析 arguments
	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid tool_call arguments: %v", err)
			b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   false,
				Error:     "invalid arguments: " + err.Error(),
			})
			return
		}
	} else {
		args = make(map[string]interface{})
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: tool_call from=%s tool=%s (mcp=%s) msgID=%s", msg.From, payload.ToolName, mcpName, msg.ID)

	// ====== 权限验证：AuthenticatedUser 方案 ======
	// 核心逻辑：如果 tool_call 携带 AuthenticatedUser，则必须与 args["account"] 匹配
	// 这确保了 app-agent 用户只能访问自己的账户数据
	var accountStr string
	if accountArg, hasAccount := args["account"]; hasAccount {
		accountStr, _ = accountArg.(string)
	}

	if payload.AuthenticatedUser != "" && accountStr != "" {
		if payload.AuthenticatedUser != accountStr {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: account mismatch: authenticated_user=%s requested_account=%s",
				payload.AuthenticatedUser, accountStr)
			b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   false,
				Error:     fmt.Sprintf("权限拒绝：用户 %s 无权访问账户 %s 的数据", payload.AuthenticatedUser, accountStr),
			})
			return
		}
		log.MessageF(log.ModuleAgent, "CodeGen gateway: access granted for authenticated_user=%s account=%s", payload.AuthenticatedUser, accountStr)
	}

	// 准备 MCP 调用上下文（设置 requestID 和 delegation token 到 mcp 包）
	if PrepareMCPContext != nil && accountStr != "" {
		PrepareMCPContext(msg.ID, accountStr)
	}

	// 调用 MCP 内部工具（通过注入的函数，带超时保护）
	if MCPCallInnerTools == nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: MCPCallInnerTools not initialized")
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "MCP bridge not initialized",
		})
		return
	}

	// 超时保护：MCP 工具调用最多 90 秒
	type mcpResult struct {
		result string
	}
	resultCh := make(chan mcpResult, 1)
	go func() {
		r := MCPCallInnerTools(mcpName, args)
		resultCh <- mcpResult{result: r}
	}()

	var result string
	select {
	case r := <-resultCh:
		result = r.result
	case <-time.After(90 * time.Second):
		log.WarnF(log.ModuleAgent, "CodeGen gateway: tool_call timeout (90s) tool=%s msgID=%s", payload.ToolName, msg.ID)
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("tool %s timeout after 90s", payload.ToolName),
		})
		return
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: tool_result to=%s tool=%s msgID=%s resultLen=%d",
		msg.From, payload.ToolName, msg.ID, len(result))

	// 解析统一信封格式
	var envelope struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data,omitempty"`
		Error string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal([]byte(result), &envelope); err != nil {
		// 信封解析失败 → 标记为错误，返回原始内容作为错误信息
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("invalid tool response format: %s", result),
		})
		return
	}
	if !envelope.OK {
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     envelope.Error,
		})
		return
	}
	// 成功：提取 data 字段作为 Result
	if err := b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    string(envelope.Data),
	}); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: send tool_result failed to=%s msgID=%s: %v", msg.From, msg.ID, err)
	}
}

// buildToolDefs 构建 UAP 工具定义列表和映射表
// 自动注册所有 MCP 工具到 gateway，使用 MCP 回调名作为 UAP 工具名
func buildToolDefs() ([]uap.ToolDef, map[string]string) {
	toolMapping := make(map[string]string) // UAP name → MCP callback name（identity mapping）

	// 从注入的 MCP 工具定义获取全部工具
	var mcpTools []MCPToolInfo
	if MCPGetToolInfos != nil {
		mcpTools = MCPGetToolInfos()
	}

	var toolDefs []uap.ToolDef

	for _, t := range mcpTools {
		toolMapping[t.Name] = t.Name

		desc := t.Description
		if desc == "" {
			desc = t.Name
		}
		params := t.Parameters
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		paramsJSON, err := json.Marshal(params)
		if err != nil {
			paramsJSON = []byte(`{"type":"object","properties":{}}`)
		}

		toolDefs = append(toolDefs, uap.ToolDef{
			Name:        t.Name,
			Description: desc,
			Parameters:  json.RawMessage(paramsJSON),
		})
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: registered %d MCP tools for UAP", len(toolDefs))
	return toolDefs, toolMapping
}

// ========================= Assistant 任务桥接 =========================

// SendTaskToLLMAgent 发送 MsgTaskAssign 给 llm-agent
func SendTaskToLLMAgent(taskID string, payload interface{}) error {
	if gatewayBridge == nil || gatewayBridge.client == nil {
		return fmt.Errorf("gateway bridge not initialized")
	}

	// 动态查找 llm_mcp 类型的 agent ID
	agentID := findLLMAgentID()
	if agentID == "" {
		return fmt.Errorf("llm-agent not found")
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %v", err)
	}

	return gatewayBridge.client.SendTo(agentID, uap.MsgTaskAssign, uap.TaskAssignPayload{
		TaskID:  taskID,
		Payload: json.RawMessage(payloadJSON),
	})
}

// RegisterTaskListener 注册任务事件监听器，返回事件 channel
func RegisterTaskListener(taskID string) chan TaskEvent {
	if gatewayBridge == nil {
		return nil
	}
	ch := make(chan TaskEvent, 1024) // 足够大的 buffer 避免丢事件
	gatewayBridge.taskEventMu.Lock()
	gatewayBridge.taskEventChannels[taskID] = ch
	gatewayBridge.taskEventMu.Unlock()
	return ch
}

// UnregisterTaskListener 注销任务事件监听器
func UnregisterTaskListener(taskID string) {
	if gatewayBridge == nil {
		return
	}
	gatewayBridge.taskEventMu.Lock()
	delete(gatewayBridge.taskEventChannels, taskID)
	gatewayBridge.taskEventMu.Unlock()
}

// SendWechatNotify 通过 gateway → wechat-agent 推送微信通知
// toUser 为微信用户ID，"@all" 表示发送给所有人
func SendWechatNotify(toUser, content string) error {
	if gatewayBridge == nil || gatewayBridge.client == nil {
		return fmt.Errorf("gateway bridge not initialized")
	}
	if !gatewayBridge.client.IsConnected() {
		return fmt.Errorf("gateway not connected")
	}
	return gatewayBridge.sendWechatViaGateway(toUser, content)
}

// IsGatewayConnected 检查 gateway 是否已连接
func IsGatewayConnected() bool {
	return gatewayBridge != nil && gatewayBridge.client != nil && gatewayBridge.client.IsConnected()
}

// IsLLMAgentOnline 检查 llm-agent 是否在线
func IsLLMAgentOnline() bool {
	return findLLMAgentID() != ""
}

// findLLMAgentID 通过 gateway HTTP API 查找 llm_mcp 类型的 agent ID
func findLLMAgentID() string {
	if gatewayBridge == nil || gatewayBridge.gatewayHTTP == "" {
		return ""
	}
	if !gatewayBridge.client.IsConnected() {
		return ""
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(gatewayBridge.gatewayHTTP + "/api/gateway/agents")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Agents  []struct {
			AgentID   string `json:"agent_id"`
			AgentType string `json:"agent_type"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if !result.Success {
		return ""
	}

	for _, a := range result.Agents {
		if a.AgentType == "llm_mcp" {
			return a.AgentID
		}
	}
	return ""
}

// ========================= 同步 LLM 请求桥接 =========================

// LLMProgressCallback 同步 LLM 请求的进度回调（nil 表示不需要进度通知）
type LLMProgressCallback func(event, text string)

// SendSyncLLMTask 发送同步 LLM 请求给 llm-agent，等待结果返回（不含进度回调）
// messages 可以是任意可 JSON 序列化的消息列表
func SendSyncLLMTask(messages interface{}, account string, selectedTools []string, noTools bool, timeout time.Duration) (string, error) {
	return SendSyncLLMTaskWithProgress(messages, account, selectedTools, noTools, timeout, nil)
}

// SendSyncLLMTaskWithProgress 发送同步 LLM 请求给 llm-agent，等待结果返回（支持进度回调）
func SendSyncLLMTaskWithProgress(messages interface{}, account string, selectedTools []string, noTools bool, timeout time.Duration, progressCb LLMProgressCallback) (string, error) {
	if gatewayBridge == nil || gatewayBridge.client == nil {
		return "", fmt.Errorf("gateway bridge not initialized")
	}
	if !gatewayBridge.client.IsConnected() {
		return "", fmt.Errorf("gateway not connected")
	}

	// 生成 taskID
	taskID := fmt.Sprintf("llm_%d", time.Now().UnixNano())

	// 注册事件监听
	eventCh := RegisterTaskListener(taskID)
	if eventCh == nil {
		return "", fmt.Errorf("failed to register task listener")
	}
	defer UnregisterTaskListener(taskID)

	// 构建 payload
	taskPayload := map[string]interface{}{
		"task_type":      "llm_request",
		"messages":       messages,
		"account":        account,
		"selected_tools": selectedTools,
		"no_tools":       noTools,
	}

	// 发送 MsgTaskAssign
	if err := SendTaskToLLMAgent(taskID, taskPayload); err != nil {
		return "", fmt.Errorf("send task failed: %v", err)
	}

	log.MessageF(log.ModuleAgent, "SendSyncLLMTask: task=%s account=%s noTools=%v timeout=%v", taskID, account, noTools, timeout)

	// 等待完成事件
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				return "", fmt.Errorf("event channel closed")
			}
			if evt.Complete {
				if evt.Error != "" {
					return "", fmt.Errorf("llm task failed: %s", evt.Error)
				}
				return evt.Text, nil
			}
			// 非 complete 事件 → 转发给 callback
			if progressCb != nil {
				progressCb(evt.Event, evt.Text)
			}
		case <-timer.C:
			return "", fmt.Errorf("llm task timeout after %v", timeout)
		}
	}
}
