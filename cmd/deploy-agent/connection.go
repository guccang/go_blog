package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection WebSocket 客户端连接管理
type Connection struct {
	cfg         *DeployConfig
	password    string
	conn        *websocket.Conn
	agentID     string
	mu          sync.Mutex
	connected   bool
	stopCh      chan struct{}
	backoffIdx  int
	activeTasks map[string]bool
	taskMu      sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *DeployConfig, password string, agentID string) *Connection {
	return &Connection{
		cfg:         cfg,
		password:    password,
		agentID:     agentID,
		stopCh:      make(chan struct{}),
		activeTasks: make(map[string]bool),
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
	c.backoffIdx = 0
	c.mu.Unlock()

	log.Printf("[INFO] connected to server")
	return nil
}

// register 发送注册消息（包含支持的项目列表）
func (c *Connection) register() {
	payload := RegisterPayload{
		AgentID:       c.agentID,
		Name:          c.cfg.AgentName,
		Workspaces:    []string{},
		Projects:      c.cfg.ProjectNames(),
		Tools:         []string{"deploy"},
		MaxConcurrent: c.cfg.MaxConcurrent,
		AuthToken:     c.cfg.AuthToken,
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
			log.Printf("[INFO] registered successfully as deploy agent (projects: %v)", c.cfg.ProjectNames())
		} else {
			log.Printf("[ERROR] register rejected: %s", payload.Error)
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
			}
			c.mu.Unlock()
		}

	case MsgTaskAssign:
		var payload TaskAssignPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[INFO] received deploy task: session=%s project=%s", payload.SessionID, payload.Project)

		if c.canAccept() {
			c.SendMsg(MsgTaskAccepted, TaskAcceptedPayload{SessionID: payload.SessionID})
			go c.executeDeploy(payload.SessionID, payload.Project)
		} else {
			c.SendMsg(MsgTaskRejected, TaskRejectedPayload{
				SessionID: payload.SessionID,
				Reason:    "deploy agent busy",
			})
		}

	case MsgTaskStop:
		var payload TaskStopPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[INFO] stop deploy task: session=%s (deploy not interruptible)", payload.SessionID)

	case MsgHeartbeatAck:
		// ok
	}
}

// resolveProject 根据项目名称查找配置，支持空名称时使用默认项目
func (c *Connection) resolveProject(projectName string) (*ProjectConfig, error) {
	if projectName != "" {
		proj := c.cfg.GetProject(projectName)
		if proj != nil {
			return proj, nil
		}
		return nil, fmt.Errorf("project %q not found, available: %v", projectName, c.cfg.ProjectNames())
	}

	// 未指定项目名：仅一个项目时自动选择
	proj := c.cfg.DefaultProject()
	if proj != nil {
		return proj, nil
	}
	return nil, fmt.Errorf("project name required, available: %v", c.cfg.ProjectNames())
}

// executeDeploy 执行部署任务
func (c *Connection) executeDeploy(sessionID string, projectName string) {
	c.taskMu.Lock()
	c.activeTasks[sessionID] = true
	c.taskMu.Unlock()

	defer func() {
		c.taskMu.Lock()
		delete(c.activeTasks, sessionID)
		c.taskMu.Unlock()
	}()

	sendEvent := func(evtType, text string) {
		c.SendMsg(MsgStreamEvent, StreamEventPayload{
			SessionID: sessionID,
			Event:     StreamEvent{Type: evtType, Text: text},
		})
	}

	// 解析目标项目
	proj, err := c.resolveProject(projectName)
	if err != nil {
		sendEvent("error", fmt.Sprintf("❌ %v", err))
		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     err.Error(),
		})
		return
	}

	sendEvent("system", fmt.Sprintf("🚀 开始部署项目 [%s]...", proj.Name))

	// 创建新的 Deployer（避免并发冲突）
	deployer := NewDeployer(c.cfg, proj, c.password)
	deployer.OnProgress = func(level, message string) {
		evtType := "system"
		prefix := "📦 "
		if level == "error" {
			evtType = "error"
			prefix = "⚠️ "
		}
		sendEvent(evtType, prefix+message)
	}

	err = deployer.Run(false, "")

	// 部署后验证
	if err == nil && proj.VerifyURL != "" {
		sendEvent("system", "⏳ 等待服务启动 (5s)...")
		time.Sleep(5 * time.Second)

		if verifyErr := c.verify(proj); verifyErr != nil {
			err = fmt.Errorf("部署验证失败: %v", verifyErr)
		} else {
			sendEvent("system", "✅ 部署验证通过（HTTP 200）")
		}
	}

	status := SessionStatus("done")
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
		sendEvent("error", fmt.Sprintf("❌ 部署失败: %v", err))
	} else {
		sendEvent("system", fmt.Sprintf("✅ 项目 [%s] 部署完成", proj.Name))
	}

	c.SendMsg(MsgTaskComplete, TaskCompletePayload{
		SessionID: sessionID,
		Status:    status,
		Error:     errMsg,
	})

	log.Printf("[INFO] deploy task %s (project=%s) completed, status=%s", sessionID, proj.Name, status)
}

// verify HTTP GET 验证部署结果
func (c *Connection) verify(proj *ProjectConfig) error {
	timeout := proj.VerifyTimeout
	if timeout <= 0 {
		timeout = 10
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Get(proj.VerifyURL)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

// canAccept 是否可以接受新任务
func (c *Connection) canAccept() bool {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()
	return len(c.activeTasks) < c.cfg.MaxConcurrent
}

// activeCount 当前活跃任务数
func (c *Connection) activeCount() int {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()
	return len(c.activeTasks)
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
				AgentID:        c.agentID,
				ActiveSessions: c.activeCount(),
				Load:           float64(c.activeCount()) / float64(c.cfg.MaxConcurrent),
				Tools:          []string{"deploy"},
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

// backoffSleep 指数退避重连等待
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
