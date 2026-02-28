package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection WebSocket 客户端连接管理
type Connection struct {
	cfg        *AgentConfig
	conn       *websocket.Conn
	agent      *Agent
	mu         sync.Mutex
	connected  bool
	stopCh     chan struct{}
	backoffIdx int
}

// NewConnection 创建连接管理器
func NewConnection(cfg *AgentConfig, agent *Agent) *Connection {
	return &Connection{
		cfg:    cfg,
		agent:  agent,
		stopCh: make(chan struct{}),
	}
}

// Run 启动连接（阻塞，自动重连）
func (c *Connection) Run() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if err := c.connect(); err != nil {
			log.Printf("[WARN] connect failed: %v", err)
			c.backoffSleep()
			continue
		}

		c.register()
		c.runLoop()
	}
}

// connect 建立 WebSocket 连接
func (c *Connection) connect() error {
	url := c.cfg.ServerURL
	if c.cfg.AuthToken != "" {
		url += "?token=" + c.cfg.AuthToken
	}

	log.Printf("[INFO] connecting to %s ...", c.cfg.ServerURL)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial: %v", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.backoffIdx = 0 // 连接成功，重置退避
	c.mu.Unlock()

	log.Printf("[INFO] connected to server")
	return nil
}

// register 发送注册消息
func (c *Connection) register() {
	payload := RegisterPayload{
		AgentID:          c.agent.ID,
		Name:             c.cfg.AgentName,
		Workspaces:       c.cfg.Workspaces,
		Projects:         c.agent.ScanProjects(),
		Models:           c.agent.ScanSettings(),           // 兼容旧版
		ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(), // Claude Code 配置
		OpenCodeModels:   c.agent.ScanOpenCodeSettings(),   // OpenCode 配置
		Tools:            c.agent.ScanTools(),
		MaxConcurrent:    c.cfg.MaxConcurrent,
		AuthToken:        c.cfg.AuthToken,
	}
	c.SendMsg(MsgRegister, payload)
}

// runLoop 消息读取主循环
func (c *Connection) runLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	// 启动心跳
	go c.heartbeatLoop()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[WARN] ws read error: %v, reconnecting...", err)
			return
		}

		var msg AgentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[WARN] parse message error: %v", err)
			continue
		}

		c.handleMessage(&msg)
	}
}

// handleMessage 处理服务端消息
func (c *Connection) handleMessage(msg *AgentMessage) {
	switch msg.Type {
	case MsgRegisterAck:
		var payload RegisterAckPayload
		json.Unmarshal(msg.Payload, &payload)
		if payload.Success {
			log.Printf("[INFO] registered successfully")
		} else {
			log.Printf("[ERROR] register rejected: %s", payload.Error)
			// 同名 agent 已在线，关闭连接，退避后重试
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
			}
			c.mu.Unlock()
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
	}
}

// heartbeatLoop 定时发送心跳
func (c *Connection) heartbeatLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			connected := c.connected
			c.mu.Unlock()
			if !connected {
				return
			}
			c.SendMsg(MsgHeartbeat, HeartbeatPayload{
				AgentID:          c.agent.ID,
				ActiveSessions:   c.agent.ActiveCount(),
				Load:             c.agent.LoadFactor(),
				Projects:         c.agent.ScanProjects(),
				Models:           c.agent.ScanSettings(),           // 兼容旧版
				ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(), // Claude Code 配置
				OpenCodeModels:   c.agent.ScanOpenCodeSettings(),   // OpenCode 配置
				Tools:            c.agent.ScanTools(),
			})
		case <-c.stopCh:
			return
		}
	}
}

// SendMsg 发送消息给服务端
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := AgentMessage{
		Type:    msgType,
		Payload: json.RawMessage(data),
		Ts:      time.Now().UnixMilli(),
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.TextMessage, msgData)
}

// backoffSleep 指数退避重连等待（1s → 60s）
func (c *Connection) backoffSleep() {
	delays := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		60 * time.Second,
	}

	delay := delays[c.backoffIdx]
	if c.backoffIdx < len(delays)-1 {
		c.backoffIdx++
	}

	select {
	case <-c.stopCh:
		return
	case <-time.After(delay):
	}
}

// Stop 停止连接
func (c *Connection) Stop() {
	close(c.stopCh)
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()
}
