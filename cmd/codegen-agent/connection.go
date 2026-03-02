package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"uap"
)

// Connection 通过 UAP gateway 的客户端连接管理
type Connection struct {
	cfg    *AgentConfig
	agent  *Agent
	client *uap.Client

	// 是否已向 go_blog-agent 注册成功
	backendRegistered bool
	regMu             sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *AgentConfig, agent *Agent) *Connection {
	// 构建 UAP 客户端
	client := uap.NewClient(cfg.ServerURL, agent.ID, "codegen", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Capacity = cfg.MaxConcurrent
	client.Meta = map[string]any{
		"workspaces": cfg.Workspaces,
	}

	c := &Connection{
		cfg:    cfg,
		agent:  agent,
		client: client,
	}

	// 设置消息回调
	client.OnMessage = c.handleUAPMessage

	return c
}

// Run 启动连接（阻塞，自动重连）
func (c *Connection) Run() {
	// uap.Client.Run() 内置自动重连和心跳
	c.client.Run()
}

// Stop 停止连接
func (c *Connection) Stop() {
	c.client.Stop()
}

// handleUAPMessage 处理来自 gateway 的 UAP 消息
func (c *Connection) handleUAPMessage(msg *uap.Message) {
	switch msg.Type {
	case MsgRegisterAck:
		// go_blog-agent 发来的 register_ack
		var payload RegisterAckPayload
		json.Unmarshal(msg.Payload, &payload)
		if payload.Success {
			c.regMu.Lock()
			c.backendRegistered = true
			c.regMu.Unlock()
			log.Printf("[INFO] registered with go_blog backend")
		} else {
			// go_blog 可能重启过，重置注册状态以便重试
			c.regMu.Lock()
			c.backendRegistered = false
			c.regMu.Unlock()
			log.Printf("[WARN] go_blog register: %s, will retry", payload.Error)
		}

	case MsgTaskAssign:
		var payload TaskAssignPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[INFO] received task: session=%s project=%s", payload.SessionID, payload.Project)

		if c.agent.CanAccept() {
			c.SendMsg(MsgTaskAccepted, TaskAcceptedPayload{SessionID: payload.SessionID})
			go c.agent.ExecuteTask(c, &payload)
		} else {
			c.SendMsg(MsgTaskRejected, TaskRejectedPayload{
				SessionID: payload.SessionID,
				Reason:    "agent at max capacity",
			})
		}

	case MsgTaskStop:
		var payload TaskStopPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[INFO] stop task: session=%s", payload.SessionID)
		c.agent.StopTask(payload.SessionID)

	case MsgFileRead:
		var payload FileReadPayload
		json.Unmarshal(msg.Payload, &payload)
		go c.agent.HandleFileRead(c, &payload)

	case MsgTreeRead:
		var payload TreeReadPayload
		json.Unmarshal(msg.Payload, &payload)
		go c.agent.HandleTreeRead(c, &payload)

	case MsgProjectCreate:
		var payload ProjectCreatePayload
		json.Unmarshal(msg.Payload, &payload)
		go c.agent.HandleProjectCreate(c, &payload)

	case MsgHeartbeatAck:
		// ok

	case uap.MsgError:
		// gateway 返回错误（如目标 agent 不在线）
		var payload uap.ErrorPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[WARN] gateway error: %s - %s", payload.Code, payload.Message)

	default:
		log.Printf("[WARN] unhandled message type: %s from %s", msg.Type, msg.From)
	}
}

// SendMsg 发送消息给 go_blog-agent（通过 gateway 路由）
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	targetAgent := c.cfg.GoBackendAgentID
	return c.client.SendTo(targetAgent, msgType, payload)
}

// onConnected UAP 客户端连接后发送注册消息给 go_blog-agent
// 注：uap.Client 会自动向 gateway 注册。这里额外向 go_blog-agent 发送 codegen 协议的 register 消息
func (c *Connection) sendCodegenRegister() {
	payload := RegisterPayload{
		AgentID:          c.agent.ID,
		Name:             c.cfg.AgentName,
		Workspaces:       c.cfg.Workspaces,
		Projects:         c.agent.ScanProjects(),
		Models:           c.agent.ScanSettings(),
		ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(),
		OpenCodeModels:   c.agent.ScanOpenCodeSettings(),
		Tools:            c.agent.ScanTools(),
		MaxConcurrent:    c.cfg.MaxConcurrent,
		AuthToken:        c.cfg.AuthToken,
	}
	c.SendMsg(MsgRegister, payload)
}

// SendHeartbeat 发送心跳给 go_blog-agent（由外部定时调用）
func (c *Connection) sendCodegenHeartbeat() {
	c.SendMsg(MsgHeartbeat, HeartbeatPayload{
		AgentID:          c.agent.ID,
		ActiveSessions:   c.agent.ActiveCount(),
		Load:             c.agent.LoadFactor(),
		Projects:         c.agent.ScanProjects(),
		Models:           c.agent.ScanSettings(),
		ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(),
		OpenCodeModels:   c.agent.ScanOpenCodeSettings(),
		Tools:            c.agent.ScanTools(),
	})
}

// StartCodegenProtocol 启动 codegen 协议层（注册 + 心跳）
// 在 uap.Client 连接成功后调用
func (c *Connection) StartCodegenProtocol() {
	// 等待 UAP 连接就绪
	for !c.client.IsConnected() {
		time.Sleep(100 * time.Millisecond)
	}

	// 发送 codegen 注册消息
	c.sendCodegenRegister()

	// 启动 codegen 层心跳（补充 UAP 心跳之外的业务心跳）
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !c.client.IsConnected() {
				// 断线后等待重连，重连后重新注册
				c.regMu.Lock()
				c.backendRegistered = false
				c.regMu.Unlock()
				for !c.client.IsConnected() {
					time.Sleep(1 * time.Second)
				}
				c.sendCodegenRegister()
			}

			// 如果尚未注册成功（go_blog 可能晚于本 agent 启动），重试注册
			c.regMu.Lock()
			registered := c.backendRegistered
			c.regMu.Unlock()
			if !registered {
				log.Printf("[INFO] go_blog backend not registered yet, retrying...")
				c.sendCodegenRegister()
				continue
			}

			c.sendCodegenHeartbeat()
		}
	}()
}
