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

	// 是否已向 go_blog-agent 注册成功
	backendRegistered bool
	regMu             sync.Mutex
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
			c.regMu.Lock()
			c.backendRegistered = true
			c.regMu.Unlock()
			log.Printf("[INFO] registered with go_blog backend as deploy agent (projects: %v)", c.cfg.ProjectNames())
		} else {
			c.regMu.Lock()
			c.backendRegistered = false
			c.regMu.Unlock()
			log.Printf("[WARN] go_blog register: %s, will retry", payload.Error)
		}

	case MsgTaskAssign:
		var payload TaskAssignPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[INFO] received deploy task: session=%s project=%s pipeline=%s target=%s platform=%s pack_only=%v",
			payload.SessionID, payload.Project, payload.Pipeline, payload.DeployTarget, payload.BuildPlatform, payload.PackOnly)

		if c.canAccept() {
			c.SendMsg(MsgTaskAccepted, TaskAcceptedPayload{SessionID: payload.SessionID})
			if payload.Pipeline != "" {
				go c.executePipeline(payload)
			} else {
				go c.executeDeploy(payload)
			}
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

	case uap.MsgNotify:
		// gateway 广播通知（如 agent_offline），deploy-agent 无需处理

	case uap.MsgError:
		var payload uap.ErrorPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[WARN] gateway error: %s - %s", payload.Code, payload.Message)

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
func (c *Connection) executeDeploy(task TaskAssignPayload) {
	sessionID := task.SessionID

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

	proj, err := c.resolveProject(task.Project)
	if err != nil {
		sendEvent("error", fmt.Sprintf("❌ %v", err))
		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     err.Error(),
		})
		return
	}

	packOnly := task.PackOnly
	targetFilter := task.DeployTarget
	buildPlatform := task.BuildPlatform

	// 浅拷贝 cfg
	deployCfg := *c.cfg

	if packOnly {
		sendEvent("system", fmt.Sprintf("📦 开始打包项目 [%s]...", proj.Name))
	} else {
		targetLabel := targetFilter
		if targetLabel == "" {
			targetLabel = "默认"
		}
		sendEvent("system", fmt.Sprintf("🚀 开始部署项目 [%s] (目标: %s)...", proj.Name, targetLabel))
	}

	deployer := NewDeployer(&deployCfg, proj, c.password)
	deployer.OnProgress = func(level, message string) {
		evtType := "system"
		prefix := "📦 "
		if level == "error" {
			evtType = "error"
			prefix = "⚠️ "
		}
		sendEvent(evtType, prefix+message)
	}

	err = deployer.Run(packOnly, targetFilter, buildPlatform)

	// 验证：从实际部署的 target 中获取 VerifyURL
	if err == nil && !packOnly {
		verifyURL := ""
		verifyTimeout := 10
		// 只检查本次实际部署的 target（按 targetFilter 过滤）
		for _, t := range proj.Targets {
			if targetFilter != "" && t.Name != targetFilter && t.Host != targetFilter {
				continue
			}
			if t.VerifyURL != "" {
				verifyURL = t.VerifyURL
				verifyTimeout = t.VerifyTimeout
				break
			}
		}
		if verifyURL == "" && proj.VerifyURL != "" {
			verifyURL = proj.VerifyURL
		}
		if verifyURL != "" {
			sendEvent("system", "⏳ 等待服务启动 (5s)...")
			time.Sleep(5 * time.Second)
			if verifyErr := c.verifyURL(verifyURL, verifyTimeout); verifyErr != nil {
				err = fmt.Errorf("部署验证失败: %v", verifyErr)
			} else {
				sendEvent("system", "✅ 部署验证通过（HTTP 200）")
			}
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

// executePipeline 执行 pipeline 编排任务（顺序执行，失败即停）
func (c *Connection) executePipeline(task TaskAssignPayload) {
	sessionID := task.SessionID

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

	// 加载 pipelines 目录
	if c.cfg.PipelinesDir == "" {
		sendEvent("error", "❌ 未配置 pipelines/ 目录")
		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     "pipelines directory not found",
		})
		return
	}

	pipCfg, err := LoadPipelines(c.cfg.PipelinesDir)
	if err != nil {
		sendEvent("error", fmt.Sprintf("❌ 加载 pipelines 失败: %v", err))
		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     err.Error(),
		})
		return
	}

	pip := pipCfg.Get(task.Pipeline)
	if pip == nil {
		errMsg := fmt.Sprintf("pipeline %q 不存在，可用: %v", task.Pipeline, pipCfg.Names())
		sendEvent("error", "❌ "+errMsg)
		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     errMsg,
		})
		return
	}

	if err := ValidatePipeline(pip, c.cfg); err != nil {
		sendEvent("error", fmt.Sprintf("❌ %v", err))
		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     err.Error(),
		})
		return
	}

	desc := ""
	if pip.Description != "" {
		desc = " — " + pip.Description
	}
	sendEvent("system", fmt.Sprintf("🔄 开始执行 Pipeline: %s%s (%d 步)", pip.Name, desc, len(pip.Steps)))

	for i, step := range pip.Steps {
		proj := c.cfg.GetProject(step.Project)

		// 浅拷贝 cfg
		deployCfg := *c.cfg

		packOnly := step.PackOnly
		targetFilter := step.Target
		buildPlatform := step.BuildPlatform

		if packOnly {
			sendEvent("system", fmt.Sprintf("📦 [%d/%d] 打包项目 [%s]...",
				i+1, len(pip.Steps), proj.Name))
		} else {
			targetLabel := targetFilter
			if targetLabel == "" {
				targetLabel = "默认"
			}
			sendEvent("system", fmt.Sprintf("🚀 [%d/%d] 部署项目 [%s] (目标: %s)...",
				i+1, len(pip.Steps), proj.Name, targetLabel))
		}

		deployer := NewDeployer(&deployCfg, proj, c.password)
		deployer.OnProgress = func(level, message string) {
			evtType := "system"
			prefix := "📦 "
			if level == "error" {
				evtType = "error"
				prefix = "⚠️ "
			}
			sendEvent(evtType, prefix+message)
		}

		stepErr := deployer.Run(packOnly, targetFilter, buildPlatform)

		// 验证
		if stepErr == nil && !packOnly {
			verifyURL := ""
			verifyTimeout := 10
			for _, t := range proj.Targets {
				if targetFilter != "" && t.Name != targetFilter && t.Host != targetFilter {
					continue
				}
				if t.VerifyURL != "" {
					verifyURL = t.VerifyURL
					verifyTimeout = t.VerifyTimeout
					break
				}
			}
			if verifyURL == "" && proj.VerifyURL != "" {
				verifyURL = proj.VerifyURL
			}
			if verifyURL != "" {
				sendEvent("system", "⏳ 等待服务启动 (5s)...")
				time.Sleep(5 * time.Second)
				if verifyErr := c.verifyURL(verifyURL, verifyTimeout); verifyErr != nil {
					stepErr = fmt.Errorf("部署验证失败: %v", verifyErr)
				} else {
					sendEvent("system", "✅ 部署验证通过（HTTP 200）")
				}
			}
		}

		if stepErr != nil {
			errMsg := fmt.Sprintf("Pipeline %q 在步骤 [%d/%d] %s 失败: %v",
				pip.Name, i+1, len(pip.Steps), proj.Name, stepErr)
			sendEvent("error", "❌ "+errMsg)
			c.SendMsg(MsgTaskComplete, TaskCompletePayload{
				SessionID: sessionID,
				Status:    "error",
				Error:     errMsg,
			})
			log.Printf("[INFO] pipeline task %s failed at step %d/%d, status=error", sessionID, i+1, len(pip.Steps))
			return
		}

		sendEvent("system", fmt.Sprintf("✅ [%d/%d] %s 完成", i+1, len(pip.Steps), proj.Name))
	}

	sendEvent("system", fmt.Sprintf("✅ Pipeline %q 全部完成 (%d 步)", pip.Name, len(pip.Steps)))
	c.SendMsg(MsgTaskComplete, TaskCompletePayload{
		SessionID: sessionID,
		Status:    "done",
	})
	log.Printf("[INFO] pipeline task %s completed, all %d steps done", sessionID, len(pip.Steps))
}

// verifyURL HTTP GET 验证部署结果
func (c *Connection) verifyURL(url string, timeout int) error {
	if timeout <= 0 {
		timeout = 10
	}

	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := httpClient.Get(url)
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
	// 加载 pipeline 名称
	var pipelineNames []string
	if c.cfg.PipelinesDir != "" {
		if pipCfg, err := LoadPipelines(c.cfg.PipelinesDir); err == nil {
			pipelineNames = pipCfg.Names()
		}
	}

	payload := RegisterPayload{
		AgentID:       c.agentID,
		Name:          c.cfg.AgentName,
		Workspaces:    []string{},
		Projects:      c.cfg.ProjectNames(),
		Tools:         []string{"deploy"},
		MaxConcurrent: c.cfg.MaxConcurrent,
		AuthToken:     c.cfg.AuthToken,
		DeployTargets: c.cfg.TargetNames,
		HostPlatform:  c.cfg.HostPlatform,
		Pipelines:     pipelineNames,
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
				c.regMu.Lock()
				c.backendRegistered = false
				c.regMu.Unlock()
				for !c.client.IsConnected() {
					time.Sleep(1 * time.Second)
				}
				c.sendDeployRegister()
			}

			// 如果尚未注册成功（go_blog 可能晚于本 agent 启动），重试注册
			c.regMu.Lock()
			registered := c.backendRegistered
			c.regMu.Unlock()
			if !registered {
				log.Printf("[INFO] go_blog backend not registered yet, retrying...")
				c.sendDeployRegister()
				continue
			}

			c.sendDeployHeartbeat()
		}
	}()
}
