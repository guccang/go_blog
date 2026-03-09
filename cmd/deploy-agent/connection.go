package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"agentbase"
	"uap"
)

// Connection 通过 UAP gateway 的客户端连接管理
type Connection struct {
	*agentbase.AgentBase // 组合基类

	cfg         *DeployConfig
	password    string
	activeTasks map[string]bool
	taskMu      sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *DeployConfig, password string, agentID string) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL: cfg.ServerURL,
		AgentID:   agentID,
		AgentType: "deploy",
		AgentName: cfg.AgentName,
		AuthToken: cfg.AuthToken,
		Capacity:  cfg.MaxConcurrent,
		Tools:     buildDeployToolDefs(cfg),
		Meta: map[string]any{
			"projects": cfg.ProjectNames(),
		},
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		password:    password,
		activeTasks: make(map[string]bool),
	}

	// 注册消息处理器
	c.RegisterHandler(MsgTaskAssign, c.handleTaskAssign)
	c.RegisterHandler(MsgTaskStop, c.handleTaskStop)
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgError, c.handleError)

	// 启用协议层
	c.EnableProtocolLayer(&agentbase.ProtocolLayerConfig{
		TargetAgentID:  cfg.GoBackendAgentID,
		BuildRegister:  c.buildRegisterPayload,
		BuildHeartbeat: c.buildHeartbeatPayload,
	})

	return c
}

// ========================= 消息处理器 =========================

// handleTaskAssign 处理任务分配
func (c *Connection) handleTaskAssign(msg *uap.Message) {
	var payload TaskAssignPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[INFO] received deploy task: session=%s project=%s pipeline=%s target=%s pack_only=%v",
		payload.SessionID, payload.Project, payload.Pipeline, payload.DeployTarget, payload.PackOnly)

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
}

// handleTaskStop 处理停止任务
func (c *Connection) handleTaskStop(msg *uap.Message) {
	var payload TaskStopPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[INFO] stop deploy task: session=%s (deploy not interruptible)", payload.SessionID)
}

// handleToolCallMsg 处理工具调用（包装器）
func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	go c.handleToolCall(msg)
}

// handleError 处理错误消息
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[WARN] gateway error: %s - %s", payload.Code, payload.Message)
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

	err = deployer.Run(packOnly, targetFilter)

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

		stepErr := deployer.Run(packOnly, targetFilter)

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
	return c.Client.SendTo(targetAgent, msgType, payload)
}

// ========================= Tool 自注册 =========================

// buildDeployToolDefs 构建 deploy-agent 的 UAP 工具定义列表
func buildDeployToolDefs(cfg *DeployConfig) []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "DeployListProjects",
			Description: "列出已配置的可部署项目（项目名、部署目标、构建平台）",
			Parameters:  mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "DeployProject",
			Description: "部署指定项目到目标服务器。支持两种模式：1) 已配置项目：直接按项目名部署；2) 未配置项目：提供 project_dir 参数，自动生成部署配置（打包脚本、发布脚本、项目配置文件）后部署",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project":       map[string]interface{}{"type": "string", "description": "项目名称（Go 项目的二进制名，即 go.mod module 路径最后一段）"},
					"deploy_target": map[string]interface{}{"type": "string", "description": "部署目标（如 local, ssh-prod），不填则使用默认"},
					"pack_only":     map[string]interface{}{"type": "boolean", "description": "仅打包不部署"},
					"project_dir":   map[string]interface{}{"type": "string", "description": "Go 项目目录绝对路径（项目未配置时必填，会自动检测 go.mod 并生成部署配置文件）"},
				},
				"required": []string{"project"},
			}),
		},
		{
			Name:        "DeployListPipelines",
			Description: "列出可用的部署编排 pipeline",
			Parameters:  mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "DeployPipeline",
			Description: "执行部署编排 pipeline（按步骤顺序部署多个项目）",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pipeline": map[string]interface{}{"type": "string", "description": "pipeline 名称"},
				},
				"required": []string{"pipeline"},
			}),
		},
	}
}

// handleToolCall 处理来自 gateway 的工具调用请求
func (c *Connection) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[WARN] invalid tool_call payload: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid tool_call payload",
		})
		return
	}

	// 解析 arguments
	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[WARN] invalid tool_call arguments: %v", err)
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   false,
				Error:     "invalid arguments: " + err.Error(),
			})
			return
		}
	} else {
		args = make(map[string]interface{})
	}

	log.Printf("[INFO] tool_call from=%s tool=%s", msg.From, payload.ToolName)

	var result string
	switch payload.ToolName {
	case "DeployListProjects":
		result = c.toolListProjects()
	case "DeployProject":
		result = c.toolDeployProject(args)
	case "DeployListPipelines":
		result = c.toolListPipelines()
	case "DeployPipeline":
		result = c.toolDeployPipeline(args)
	default:
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
		})
		return
	}

	c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    result,
	})
}

// toolListProjects 列出已配置的可部署项目
func (c *Connection) toolListProjects() string {
	type projectInfo struct {
		Name     string   `json:"name"`
		Targets  []string `json:"targets"`
		Platform string   `json:"platform"`
	}
	var projects []projectInfo
	for _, name := range c.cfg.ProjectOrder {
		proj := c.cfg.Projects[name]
		var targets []string
		for _, t := range proj.Targets {
			targets = append(targets, t.Name)
		}
		projects = append(projects, projectInfo{
			Name:     proj.Name,
			Targets:  targets,
			Platform: c.cfg.HostPlatform,
		})
	}
	return string(mustMarshalJSON(map[string]interface{}{
		"success":  true,
		"status":   "completed",
		"projects": projects,
	}))
}

// toolDeployProject 部署指定项目
func (c *Connection) toolDeployProject(args map[string]interface{}) string {
	projectName, _ := args["project"].(string)
	deployTarget, _ := args["deploy_target"].(string)
	packOnly, _ := args["pack_only"].(bool)
	projectDir, _ := args["project_dir"].(string)

	proj, err := c.resolveProject(projectName)
	if err != nil && projectDir != "" {
		// 项目配置不存在，自动初始化
		log.Printf("[INFO] project %q not found, auto-initializing from %s", projectName, projectDir)
		initOpts := &InitOptions{NonInteractive: true}
		if initErr := runInit(projectDir, c.cfg.ConfigPath, initOpts); initErr != nil {
			return fmt.Sprintf(`{"success":false,"status":"failed","error":"自动初始化失败: %s"}`, initErr.Error())
		}
		// 重新加载配置
		newCfg, reloadErr := LoadConfigForDaemon(c.cfg.ConfigPath)
		if reloadErr != nil {
			return fmt.Sprintf(`{"success":false,"status":"failed","error":"重新加载配置失败: %s"}`, reloadErr.Error())
		}
		c.cfg = newCfg
		proj = c.cfg.GetProject(projectName)
		if proj == nil {
			return fmt.Sprintf(`{"success":false,"status":"failed","error":"初始化完成但未找到项目 %q，请检查项目目录"}`, projectName)
		}
		log.Printf("[INFO] project %q auto-initialized successfully", projectName)
	} else if err != nil {
		return fmt.Sprintf(`{"success":false,"status":"failed","error":"%s"}`, err.Error())
	}

	// 浅拷贝 cfg
	deployCfg := *c.cfg

	deployer := NewDeployer(&deployCfg, proj, c.password)
	err = deployer.Run(packOnly, deployTarget)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"status":"failed","error":"部署失败: %s"}`, err.Error())
	}

	action := "部署"
	if packOnly {
		action = "打包"
	}
	return fmt.Sprintf(`{"success":true,"status":"completed","message":"%s项目 %s 完成"}`, action, proj.Name)
}

// toolListPipelines 列出可用 pipeline
func (c *Connection) toolListPipelines() string {
	if c.cfg.PipelinesDir == "" {
		return `{"success":true,"status":"completed","pipelines":[]}`
	}

	pipCfg, err := LoadPipelines(c.cfg.PipelinesDir)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"status":"failed","error":"加载 pipelines 失败: %s"}`, err.Error())
	}

	type pipInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Steps       int    `json:"steps"`
	}
	var pipelines []pipInfo
	for _, p := range pipCfg.Pipelines {
		pipelines = append(pipelines, pipInfo{
			Name:        p.Name,
			Description: p.Description,
			Steps:       len(p.Steps),
		})
	}
	return string(mustMarshalJSON(map[string]interface{}{
		"success":   true,
		"status":    "completed",
		"pipelines": pipelines,
	}))
}

// toolDeployPipeline 执行部署编排 pipeline
func (c *Connection) toolDeployPipeline(args map[string]interface{}) string {
	pipelineName, _ := args["pipeline"].(string)
	if pipelineName == "" {
		return `{"success":false,"status":"failed","error":"缺少 pipeline 参数"}`
	}

	if c.cfg.PipelinesDir == "" {
		return `{"success":false,"status":"failed","error":"未配置 pipelines 目录"}`
	}

	pipCfg, err := LoadPipelines(c.cfg.PipelinesDir)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"status":"failed","error":"加载 pipelines 失败: %s"}`, err.Error())
	}

	pip := pipCfg.Get(pipelineName)
	if pip == nil {
		return fmt.Sprintf(`{"success":false,"status":"failed","error":"pipeline %q 不存在，可用: %v"}`, pipelineName, pipCfg.Names())
	}

	if err := ValidatePipeline(pip, c.cfg); err != nil {
		return fmt.Sprintf(`{"success":false,"status":"failed","error":"%s"}`, err.Error())
	}

	// 逐步执行
	for i, step := range pip.Steps {
		proj := c.cfg.GetProject(step.Project)
		deployCfg := *c.cfg

		deployer := NewDeployer(&deployCfg, proj, c.password)
		stepErr := deployer.Run(step.PackOnly, step.Target)

		if stepErr != nil {
			return fmt.Sprintf(`{"success":false,"status":"failed","error":"Pipeline %q 在步骤 [%d/%d] %s 失败: %v"}`,
				pip.Name, i+1, len(pip.Steps), proj.Name, stepErr)
		}
	}

	return fmt.Sprintf(`{"success":true,"status":"completed","message":"Pipeline %q 全部完成 (%d 步)"}`, pip.Name, len(pip.Steps))
}

// mustMarshalJSON 将值序列化为 JSON，失败时返回空对象
func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}

// ========================= 协议层载荷构建 =========================

// buildRegisterPayload 构建注册消息载荷
func (c *Connection) buildRegisterPayload() interface{} {
	var pipelineNames []string
	if c.cfg.PipelinesDir != "" {
		if pipCfg, err := LoadPipelines(c.cfg.PipelinesDir); err == nil {
			pipelineNames = pipCfg.Names()
		}
	}

	return RegisterPayload{
		AgentID:       c.AgentID,
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
}

// buildHeartbeatPayload 构建心跳消息载荷
func (c *Connection) buildHeartbeatPayload() interface{} {
	return HeartbeatPayload{
		AgentID:        c.AgentID,
		ActiveSessions: c.activeCount(),
		Load:           float64(c.activeCount()) / float64(c.cfg.MaxConcurrent),
		Tools:          []string{"deploy"},
	}
}
