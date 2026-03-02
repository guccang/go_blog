package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"uap"
)

// Connection 通过 UAP gateway 的客户端连接管理
type Connection struct {
	cfg         *DeployConfig
	password    string
	agentID     string
	client      *uap.Client
	activeTasks map[string]bool
	taskMu      sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *DeployConfig, password string, agentID string) *Connection {
	client := uap.NewClient(cfg.ServerURL, agentID, "deploy", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Capacity = cfg.MaxConcurrent
	client.Meta = map[string]any{
		"projects": cfg.ProjectNames(),
	}

	c := &Connection{
		cfg:         cfg,
		password:    password,
		agentID:     agentID,
		client:      client,
		activeTasks: make(map[string]bool),
	}

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
		var payload RegisterAckPayload
		json.Unmarshal(msg.Payload, &payload)
		if payload.Success {
			log.Printf("[INFO] registered with go_blog backend as deploy agent (projects: %v)", c.cfg.ProjectNames())
		} else {
			log.Printf("[ERROR] go_blog register rejected: %s", payload.Error)
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

	default:
		log.Printf("[WARN] unhandled message type: %s from %s", msg.Type, msg.From)
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

	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := httpClient.Get(proj.VerifyURL)
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

// SendMsg 发送消息给 go_blog-agent（通过 gateway 路由）
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	targetAgent := c.cfg.GoBackendAgentID
	return c.client.SendTo(targetAgent, msgType, payload)
}

// sendDeployRegister 发送 deploy 协议注册消息给 go_blog-agent
func (c *Connection) sendDeployRegister() {
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

// sendDeployHeartbeat 发送心跳给 go_blog-agent
func (c *Connection) sendDeployHeartbeat() {
	c.SendMsg(MsgHeartbeat, HeartbeatPayload{
		AgentID:        c.agentID,
		ActiveSessions: c.activeCount(),
		Load:           float64(c.activeCount()) / float64(c.cfg.MaxConcurrent),
		Tools:          []string{"deploy"},
	})
}

// StartDeployProtocol 启动 deploy 协议层（注册 + 心跳）
func (c *Connection) StartDeployProtocol() {
	// 等待 UAP 连接就绪
	for !c.client.IsConnected() {
		time.Sleep(100 * time.Millisecond)
	}

	c.sendDeployRegister()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !c.client.IsConnected() {
				for !c.client.IsConnected() {
					time.Sleep(1 * time.Second)
				}
				c.sendDeployRegister()
			}
			c.sendDeployHeartbeat()
		}
	}()
}
