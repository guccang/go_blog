package codegen

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"sync"
	"time"

	"uap"
)

// GatewaySender 通过 gateway 路由发送消息
type GatewaySender struct {
	client      *uap.Client
	toAgentID   string // 目标 agent ID（codegen-agent / deploy-agent 的 UAP ID）
}

// SendAgentMsg 通过 gateway 路由发送 AgentMessage
func (s *GatewaySender) SendAgentMsg(msgType string, payload interface{}) error {
	return s.client.SendTo(s.toAgentID, msgType, payload)
}

// GatewayBridge go_blog 的 gateway 适配层
type GatewayBridge struct {
	client *uap.Client
	pool   *AgentPool

	// wechat notify 处理器
	wechatHandler func(wechatUser, message string) string

	mu sync.Mutex
}

// 全局 gateway bridge 实例
var gatewayBridge *GatewayBridge

// InitGatewayBridge 初始化 go_blog 到 gateway 的连接
func InitGatewayBridge(gatewayURL, authToken string) {
	client := uap.NewClient(gatewayURL, "go_blog", "go_blog", "Go Blog Server")
	client.AuthToken = authToken
	client.Capacity = 100
	client.Meta = map[string]any{
		"role": "backend",
	}

	bridge := &GatewayBridge{
		client: client,
		pool:   agentPool,
	}

	client.OnMessage = bridge.handleMessage

	gatewayBridge = bridge

	// 后台连接 gateway（非阻塞）
	go func() {
		log.MessageF(log.ModuleAgent, "CodeGen: connecting to gateway at %s", gatewayURL)
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

// handleMessage 处理从 gateway 收到的 UAP 消息
func (b *GatewayBridge) handleMessage(msg *uap.Message) {
	// 解析 AgentMessage payload（codegen/deploy agent 发来的原协议消息）
	switch msg.Type {
	case MsgRegister:
		b.handleRegister(msg)

	case MsgHeartbeat:
		b.handleHeartbeat(msg)

	case MsgTaskAccepted:
		var payload TaskAcceptedPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_accepted payload: %v", err)
			return
		}
		agent := b.getAgent(msg.From)
		if agent != nil {
			agent.mu.Lock()
			agent.ActiveSessions[payload.SessionID] = true
			agent.mu.Unlock()
		}
		log.MessageF(log.ModuleAgent, "CodeGen: gateway agent %s accepted task %s", msg.From, payload.SessionID)

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

		// 转发 task_event 给所有 wechat-agent
		b.forwardToWechatAgents(msg)

	case MsgTaskComplete:
		var payload TaskCompletePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_complete payload: %v", err)
			return
		}
		agent := b.getAgent(msg.From)
		b.pool.handleTaskComplete(agent, &payload)

		// 转发 task_complete 给所有 wechat-agent
		b.forwardToWechatAgents(msg)

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
		b.handleNotify(msg)

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
		ID:   agentID,
		Name: payload.Name,
		Sender: &GatewaySender{
			client:    b.client,
			toAgentID: agentID,
		},
		Conn:             nil, // gateway 模式无直连
		Workspaces:       payload.Workspaces,
		Projects:         payload.Projects,
		Models:           payload.Models,
		ClaudeCodeModels: payload.ClaudeCodeModels,
		OpenCodeModels:   payload.OpenCodeModels,
		Tools:            payload.Tools,
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
		// agent 不在 pool 中（可能 go_blog 重启过），通知它重新注册
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
		agent.Projects = payload.Projects
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

// handleNotify 处理通知消息（来自 wechat-agent）
func (b *GatewayBridge) handleNotify(msg *uap.Message) {
	var payload uap.NotifyPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid notify payload: %v", err)
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

	// 回发结果给 wechat-agent
	b.client.SendTo(msg.From, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      payload.To,
		Content: result,
	})
}

// forwardToWechatAgents 转发消息给所有连接的 wechat-agent（通过 gateway 路由）
func (b *GatewayBridge) forwardToWechatAgents(msg *uap.Message) {
	// 使用约定的 wechat-agent ID 前缀
	// wechat-agent 注册时 ID 为 "wechat-<name>"
	// 这里直接广播给已知的 wechat-agent
	// 简单实现：发给 "wechat-wechat-agent"（默认 wechat-agent ID）
	wechatAgentID := "wechat-wechat-agent"
	b.client.Send(&uap.Message{
		Type:    msg.Type,
		ID:      uap.NewMsgID(),
		From:    "go_blog",
		To:      wechatAgentID,
		Payload: msg.Payload,
		Ts:      time.Now().UnixMilli(),
	})
}

// getAgent 从 pool 中获取 agent
func (b *GatewayBridge) getAgent(agentID string) *RemoteAgent {
	b.pool.mu.RLock()
	agent := b.pool.agents[agentID]
	b.pool.mu.RUnlock()
	return agent
}
