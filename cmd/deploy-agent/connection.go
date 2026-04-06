package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
	fileToolKit *agentbase.FileToolKit
}

// NewConnection 创建连接管理器
func NewConnection(cfg *DeployConfig, password string, agentID string) *Connection {
	// deploy-agent 的 ProjectResolver
	resolver := func(project string) string {
		proj := cfg.GetProject(project)
		if proj == nil {
			return ""
		}
		return proj.ProjectDir
	}
	fileToolKit := agentbase.NewFileToolKit("Deploy", "deploy-agent", resolver)

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "deploy",
		AgentName:   cfg.AgentName,
		Description: "项目部署、流水线管理、服务器操作",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildDeployToolDefs(cfg, fileToolKit),
		Meta: map[string]any{
			"projects":       cfg.ProjectNames(),
			"ssh_hosts":      cfg.SSHHosts,
			"deploy_targets": cfg.TargetNames,
			"host_platform":  cfg.HostPlatform,
			"target_hosts":   buildTargetHostMap(cfg),
			"pipelines":      scanPipelineNames(cfg),
		},
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		password:    password,
		activeTasks: make(map[string]bool),
		fileToolKit: fileToolKit,
	}

	// 注册消息处理器
	c.RegisterHandler(MsgTaskAssign, c.handleTaskAssign)
	c.RegisterHandler(MsgTaskStop, c.handleTaskStop)
	c.RegisterToolCallHandler(c.handleToolCall)
	c.RegisterHandler(uap.MsgError, c.handleError)

	// 注册 tool_cancel 回调（使用 agentbase 统一处理）
	c.OnToolCancel = c.handleToolCancelCallback

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

// handleToolCancelCallback 处理工具取消回调
func (c *Connection) handleToolCancelCallback(toolName, msgID string) {
	log.Printf("[INFO] tool_cancel: tool=%s msgID=%s (deploy operations not interruptible)", toolName, msgID)
	// deploy 操作通常不可中断（SSH 命令已发送），仅记录日志
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

	// adhoc 模式：ssh_host 存在时走一次性部署
	if task.SSHHost != "" {
		if task.ProjectDir == "" {
			sendEvent("error", "❌ adhoc 模式需要 project_dir 参数")
			c.SendMsg(MsgTaskComplete, TaskCompletePayload{
				SessionID: sessionID,
				Status:    "error",
				Error:     "adhoc mode requires project_dir",
			})
			return
		}

		adhoc := &AdhocConfig{
			ProjectDir: task.ProjectDir,
			SSHHost:    task.SSHHost,
			SSHPort:    task.SSHPort,
			RemoteDir:  task.RemoteDir,
			StartArgs:  task.StartArgs,
		}

		sendEvent("system", fmt.Sprintf("🚀 开始 adhoc 部署到 %s...", task.SSHHost))

		deployCfg := *c.cfg
		err := adhocDeploy(&deployCfg, adhoc, c.password, func(level, message string) {
			evtType := "system"
			prefix := "📦 "
			if level == "error" {
				evtType = "error"
				prefix = "⚠️ "
			}
			sendEvent(evtType, prefix+message)
		})

		status := SessionStatus("done")
		errMsg := ""
		if err != nil {
			status = "error"
			errMsg = err.Error()
			sendEvent("error", fmt.Sprintf("❌ adhoc 部署失败: %v", err))
		} else {
			sendEvent("system", "✅ adhoc 部署完成")
		}

		c.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    status,
			Error:     errMsg,
		})
		log.Printf("[INFO] adhoc deploy task %s completed, status=%s", sessionID, status)
		return
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
	deployer.DeployMode = DeployMode(task.DeployMode)
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

// SendMsg 发送消息给 blog-agent-agent（通过 gateway 路由）
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	targetAgent := c.cfg.GoBackendAgentID
	return c.Client.SendTo(targetAgent, msgType, payload)
}

// ========================= Tool 自注册 =========================

// buildTargetHostMap 构建 target 名→SSH host 的映射（如 ssh-prod → root@114.115.214.86）
func buildTargetHostMap(cfg *DeployConfig) map[string]string {
	m := make(map[string]string)
	for _, proj := range cfg.Projects {
		for _, t := range proj.Targets {
			if t.Name != "" && t.Host != "" && t.Host != "local" {
				m[t.Name] = t.Host
			}
		}
	}
	return m
}

// scanPipelineNames 扫描 pipelines 目录，返回可用 pipeline 名称列表
func scanPipelineNames(cfg *DeployConfig) []string {
	if cfg.PipelinesDir == "" {
		return nil
	}
	pipCfg, err := LoadPipelines(cfg.PipelinesDir)
	if err != nil {
		return nil
	}
	return pipCfg.Names()
}

// buildSSHHostDesc 动态生成 ssh_host 参数描述
func buildSSHHostDesc(cfg *DeployConfig) string {
	if len(cfg.SSHHosts) > 0 {
		return fmt.Sprintf("SSH 目标 user@host（可用服务器: %s）", strings.Join(cfg.SSHHosts, ", "))
	}
	return "SSH 目标 user@host"
}

// buildDeployToolDefs 构建 deploy-agent 的 UAP 工具定义列表
func buildDeployToolDefs(cfg *DeployConfig, ftk *agentbase.FileToolKit) []uap.ToolDef {
	defs := []uap.ToolDef{
		{
			Name:        "DeployListProjects",
			Description: "列出 deploy-agent 已知项目及其 configured/targets 信息；仅做发现，不执行部署。configured=true 用 DeployProject，configured=false 用 DeployAdhoc",
			Parameters:  mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "AgentShutdown",
			Description: "关闭指定 Agent。默认优雅退出；force=true 时立即强制退出。会修改目标 Agent 运行状态",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{"type": "string", "description": "目标 Agent ID"},
					"reason":   map[string]interface{}{"type": "string", "description": "关闭原因"},
					"force":    map[string]interface{}{"type": "boolean", "description": "是否强制立即退出（跳过 drain）"},
				},
				"required": []string{"agent_id"},
			}),
		},
		{
			Name:        "AgentStatus",
			Description: "查询指定 Agent 的当前运行状态和最近心跳；只读，不会修改目标 Agent",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{"type": "string", "description": "目标 Agent ID"},
				},
				"required": []string{"agent_id"},
			}),
		},
		{
			Name:        "DeployProject",
			Description: "部署 settings 中已配置的项目。调用前先用 DeployListProjects 确认 project 存在且 configured=true；不要传 project_dir 或 ssh_host。未配置项目改用 DeployAdhoc",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project":       map[string]interface{}{"type": "string", "description": "项目名称（必须是 DeployListProjects 中 configured=true 的项目）"},
					"deploy_target": map[string]interface{}{"type": "string", "description": "部署目标名称（如 local, ssh-prod），来自 DeployListProjects 返回的 targets 列表，不填则使用默认目标"},
					"deploy_mode":   map[string]interface{}{"type": "string", "enum": []string{"auto", "full", "increment"}, "description": "部署模式: auto=自动检测（默认）, full=完整部署覆盖所有文件, increment=增量部署保护配置文件"},
					"pack_only":     map[string]interface{}{"type": "boolean", "description": "仅打包不部署"},
				},
				"required": []string{"project"},
			}),
		},
		{
			Name:        "DeployAdhoc",
			Description: "一次性部署未在 settings 中配置的项目。必须提供 project_dir 和 ssh_host；如果项目已 configured=true，应改用 DeployProject 而不是此接口",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project":     map[string]interface{}{"type": "string", "description": "项目名称（Go 项目的二进制名，即 go.mod module 路径最后一段）"},
					"project_dir": map[string]interface{}{"type": "string", "description": "Go 项目目录绝对路径（包含 go.mod 的目录）"},
					"ssh_host":    map[string]interface{}{"type": "string", "description": buildSSHHostDesc(cfg)},
					"ssh_port":    map[string]interface{}{"type": "integer", "description": "SSH 端口（默认 22）"},
					"remote_dir":  map[string]interface{}{"type": "string", "description": "远程部署目录（默认 /data/program/<项目名>）"},
					"start_args":  map[string]interface{}{"type": "string", "description": "启动参数"},
					"port":        map[string]interface{}{"type": "integer", "description": "服务监听端口。部署前自动 kill 占用该端口的进程，防止 address already in use。如果端口已在 start_args 中指定（如 -port 8080），可不填"},
					"deploy_mode": map[string]interface{}{"type": "string", "enum": []string{"auto", "full", "increment"}, "description": "部署模式: auto=自动检测（默认）, full=完整部署覆盖所有文件, increment=增量部署保护配置文件"},
				},
				"required": []string{"project", "project_dir", "ssh_host"},
			}),
		},
		{
			Name:        "DeployListPipelines",
			Description: "列出可用的部署 pipeline；仅做发现，不执行任何部署步骤",
			Parameters:  mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "DeployPipeline",
			Description: "执行预配置部署 pipeline，按定义顺序运行多个步骤；调用前可先用 DeployListPipelines 确认名称",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pipeline": map[string]interface{}{"type": "string", "description": "pipeline 名称"},
				},
				"required": []string{"pipeline"},
			}),
		},
	}
	// 追加 DeployExecEnvBash（供 env-agent 远程执行环境检测命令）
	for _, td := range ftk.ToolDefs() {
		if strings.HasSuffix(td.Name, "ExecEnvBash") {
			defs = append(defs, td)
		}
	}
	return defs
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

	// 构建进度推送回调：通过 MsgNotify 将进度发送给调用方
	sendProgress := func(text string) {
		c.Client.SendTo(msg.From, uap.MsgNotify, uap.NotifyPayload{
			Channel: "tool_progress",
			To:      msg.ID, // 用工具调用的 msgID 做关联
			Content: text,
		})
	}

	var result string
	switch payload.ToolName {
	case "DeployListProjects":
		result = c.toolListProjects()
	case "DeployProject":
		result = c.toolDeployProject(args, sendProgress)
	case "DeployAdhoc":
		result = c.toolDeployAdhoc(args, sendProgress)
	case "DeployListPipelines":
		result = c.toolListPipelines()
	case "DeployPipeline":
		result = c.toolDeployPipeline(args, sendProgress)
	case "AgentShutdown":
		c.toolAgentShutdown(msg, args)
		return
	case "AgentStatus":
		c.toolAgentStatus(msg, args)
		return
	default:
		if result, handled := c.fileToolKit.HandleTool(payload.ToolName, args); handled {
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   true,
				Result:    result,
			})
			return
		}
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

// toolListProjects 列出所有可部署项目（含 workspace 发现的未配置项目）
func (c *Connection) toolListProjects() string {
	type projectInfo struct {
		Name       string   `json:"name"`
		Configured bool     `json:"configured"`
		ProjectDir string   `json:"project_dir"`
		Targets    []string `json:"targets"`
		Platform   string   `json:"platform"`
	}
	var projects []projectInfo
	for _, name := range c.cfg.ProjectOrder {
		proj := c.cfg.Projects[name]
		var targets []string
		for _, t := range proj.Targets {
			targets = append(targets, t.Name)
		}
		projects = append(projects, projectInfo{
			Name:       proj.Name,
			Configured: proj.Configured,
			ProjectDir: proj.ProjectDir,
			Targets:    targets,
			Platform:   c.cfg.HostPlatform,
		})
	}
	data := map[string]interface{}{
		"projects":  projects,
		"ssh_hosts": c.cfg.SSHHosts,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("列出%d个项目", len(projects)))
	return tr.Result
}

// toolDeployProject 部署已配置的项目（使用 settings 中的预配置）
func (c *Connection) toolDeployProject(args map[string]interface{}, sendProgress func(string)) string {
	projectName, _ := args["project"].(string)
	deployTarget, _ := args["deploy_target"].(string)
	packOnly, _ := args["pack_only"].(bool)
	projectDir, _ := args["project_dir"].(string)
	deployModeStr, _ := args["deploy_mode"].(string)

	proj, err := c.resolveProject(projectName)
	if err != nil && projectDir != "" {
		// 项目配置不存在，自动初始化
		log.Printf("[INFO] project %q not found, auto-initializing from %s", projectName, projectDir)
		initOpts := &InitOptions{NonInteractive: true}
		if initErr := runInit(projectDir, c.cfg.ConfigPath, initOpts); initErr != nil {
			return fmt.Sprintf(`{"success":false,"error":"自动初始化失败: %s"}`, initErr.Error())
		}
		// 重新加载配置
		newCfg, reloadErr := LoadConfigForDaemon(c.cfg.ConfigPath)
		if reloadErr != nil {
			return fmt.Sprintf(`{"success":false,"error":"重新加载配置失败: %s"}`, reloadErr.Error())
		}
		c.cfg = newCfg
		proj = c.cfg.GetProject(projectName)
		if proj == nil {
			return fmt.Sprintf(`{"success":false,"error":"初始化完成但未找到项目 %q，请检查项目目录"}`, projectName)
		}
		log.Printf("[INFO] project %q auto-initialized successfully", projectName)
	} else if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"%s。如果是未配置项目，请使用 DeployAdhoc 接口"}`, err.Error())
	}

	// 浅拷贝 cfg
	deployCfg := *c.cfg

	deployer := NewDeployer(&deployCfg, proj, c.password)
	deployer.DeployMode = DeployMode(deployModeStr)
	deployer.OnProgress = func(level, message string) {
		prefix := "📦 "
		if level == "error" {
			prefix = "⚠️ "
		}
		sendProgress(prefix + message)
	}

	action := "部署"
	if packOnly {
		action = "打包"
	}
	sendProgress(fmt.Sprintf("🚀 开始%s项目 [%s]...", action, proj.Name))

	err = deployer.Run(packOnly, deployTarget)
	if err != nil {
		sendProgress(fmt.Sprintf("❌ %s失败: %s", action, err.Error()))
		return fmt.Sprintf(`{"success":false,"error":"部署失败: %s"}`, err.Error())
	}

	sendProgress(fmt.Sprintf("✅ %s项目 %s 完成", action, proj.Name))
	tr := uap.BuildToolResult("", nil, fmt.Sprintf("%s项目 %s 完成", action, proj.Name))
	return tr.Result
}

// toolDeployAdhoc 一次性部署未配置的项目到指定服务器
func (c *Connection) toolDeployAdhoc(args map[string]interface{}, sendProgress func(string)) string {
	projectName, _ := args["project"].(string)
	projectDir, _ := args["project_dir"].(string)
	sshHost, _ := args["ssh_host"].(string)

	if projectDir == "" {
		return `{"success":false,"error":"project_dir 参数不能为空"}`
	}
	if sshHost == "" {
		return `{"success":false,"error":"ssh_host 参数不能为空"}`
	}

	sshPort := 22
	if p, ok := args["ssh_port"].(float64); ok && p > 0 {
		sshPort = int(p)
	}
	remoteDir, _ := args["remote_dir"].(string)
	startArgs, _ := args["start_args"].(string)
	servicePort := 0
	if p, ok := args["port"].(float64); ok && p > 0 {
		servicePort = int(p)
	}

	adhoc := &AdhocConfig{
		ProjectDir:  projectDir,
		SSHHost:     sshHost,
		SSHPort:     sshPort,
		RemoteDir:   remoteDir,
		StartArgs:   startArgs,
		ServicePort: servicePort,
	}

	deployCfg := *c.cfg
	sendProgress(fmt.Sprintf("🚀 开始 adhoc 部署项目 [%s]...", projectName))
	err := adhocDeploy(&deployCfg, adhoc, c.password, func(level, message string) {
		prefix := "📦 "
		if level == "error" {
			prefix = "⚠️ "
		}
		sendProgress(prefix + message)
	})
	if err != nil {
		sendProgress(fmt.Sprintf("❌ adhoc 部署失败: %s", err.Error()))
		return fmt.Sprintf(`{"success":false,"error":"adhoc 部署失败: %s"}`, err.Error())
	}
	sendProgress(fmt.Sprintf("✅ adhoc 部署项目 %s 完成", projectName))
	tr := uap.BuildToolResult("", nil, fmt.Sprintf("adhoc 部署项目 %s 完成", projectName))
	return tr.Result
}

// toolListPipelines 列出可用 pipeline
func (c *Connection) toolListPipelines() string {
	if c.cfg.PipelinesDir == "" {
		tr := uap.BuildToolResult("", []interface{}{}, "无可用 pipeline")
		return tr.Result
	}

	pipCfg, err := LoadPipelines(c.cfg.PipelinesDir)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"加载 pipelines 失败: %s"}`, err.Error())
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
	tr := uap.BuildToolResult("", pipelines, fmt.Sprintf("列出%d个 pipeline", len(pipelines)))
	return tr.Result
}

// toolDeployPipeline 执行部署编排 pipeline
func (c *Connection) toolDeployPipeline(args map[string]interface{}, sendProgress func(string)) string {
	pipelineName, _ := args["pipeline"].(string)
	if pipelineName == "" {
		return `{"success":false,"error":"缺少 pipeline 参数"}`
	}

	if c.cfg.PipelinesDir == "" {
		return `{"success":false,"error":"未配置 pipelines 目录"}`
	}

	pipCfg, err := LoadPipelines(c.cfg.PipelinesDir)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"加载 pipelines 失败: %s"}`, err.Error())
	}

	pip := pipCfg.Get(pipelineName)
	if pip == nil {
		return fmt.Sprintf(`{"success":false,"error":"pipeline %q 不存在，可用: %v"}`, pipelineName, pipCfg.Names())
	}

	if err := ValidatePipeline(pip, c.cfg); err != nil {
		return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
	}

	sendProgress(fmt.Sprintf("🔄 开始执行 Pipeline: %s (%d 步)", pip.Name, len(pip.Steps)))

	// 逐步执行
	for i, step := range pip.Steps {
		proj := c.cfg.GetProject(step.Project)
		deployCfg := *c.cfg

		sendProgress(fmt.Sprintf("🚀 [%d/%d] 部署项目 [%s]...", i+1, len(pip.Steps), proj.Name))

		deployer := NewDeployer(&deployCfg, proj, c.password)
		deployer.OnProgress = func(level, message string) {
			prefix := "📦 "
			if level == "error" {
				prefix = "⚠️ "
			}
			sendProgress(prefix + message)
		}
		stepErr := deployer.Run(step.PackOnly, step.Target)

		if stepErr != nil {
			sendProgress(fmt.Sprintf("❌ [%d/%d] %s 失败: %v", i+1, len(pip.Steps), proj.Name, stepErr))
			return fmt.Sprintf(`{"success":false,"error":"Pipeline %q 在步骤 [%d/%d] %s 失败: %v"}`,
				pip.Name, i+1, len(pip.Steps), proj.Name, stepErr)
		}

		sendProgress(fmt.Sprintf("✅ [%d/%d] %s 完成", i+1, len(pip.Steps), proj.Name))
	}

	sendProgress(fmt.Sprintf("✅ Pipeline %q 全部完成 (%d 步)", pip.Name, len(pip.Steps)))

	tr := uap.BuildToolResult("", nil, fmt.Sprintf("Pipeline %q 全部完成 (%d 步)", pip.Name, len(pip.Steps)))
	return tr.Result
}

// mustMarshalJSON 将值序列化为 JSON，失败时返回空对象
func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}

// ========================= 控制协议 Tool =========================

// toolAgentShutdown 通过 ctrl_shutdown 远程关闭指定 Agent
func (c *Connection) toolAgentShutdown(msg *uap.Message, args map[string]interface{}) {
	agentID, _ := args["agent_id"].(string)
	if agentID == "" {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "缺少 agent_id 参数"))
		return
	}
	agentID = c.cfg.ResolveAgentID(agentID)
	reason, _ := args["reason"].(string)
	if reason == "" {
		reason = "tool_call"
	}
	force, _ := args["force"].(bool)

	// 注册临时响应处理器
	responseCh := make(chan *uap.Message, 1)
	c.RegisterHandler(uap.MsgCtrlShutdownAck, func(resp *uap.Message) {
		responseCh <- resp
	})
	defer c.RegisterHandler(uap.MsgCtrlShutdownAck, nil)

	log.Printf("[INFO] sending ctrl_shutdown to %s (force=%v reason=%s)", agentID, force, reason)
	c.Client.SendTo(agentID, uap.MsgCtrlShutdown, uap.CtrlShutdownPayload{
		Reason: reason,
		Force:  force,
	})

	// 等待 ack
	select {
	case resp := <-responseCh:
		var ack uap.CtrlShutdownAckPayload
		json.Unmarshal(resp.Payload, &ack)
		result := uap.BuildToolResult(msg.ID, ack, fmt.Sprintf("shutdown %s: accepted=%v state=%s tasks=%d",
			ack.AgentID, ack.Accepted, ack.CurrentState, ack.ActiveTasks))
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   true,
			Result:    result.Result,
		})
	case <-time.After(10 * time.Second):
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID,
			fmt.Sprintf("等待 %s 的 shutdown ack 超时（10s），目标 agent 可能不在线", agentID)))
	}
}

// toolAgentStatus 通过 ctrl_status 查询指定 Agent 状态
func (c *Connection) toolAgentStatus(msg *uap.Message, args map[string]interface{}) {
	agentID, _ := args["agent_id"].(string)
	if agentID == "" {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "缺少 agent_id 参数"))
		return
	}
	agentID = c.cfg.ResolveAgentID(agentID)

	// 注册临时响应处理器
	responseCh := make(chan *uap.Message, 1)
	c.RegisterHandler(uap.MsgCtrlStatusReport, func(resp *uap.Message) {
		responseCh <- resp
	})
	defer c.RegisterHandler(uap.MsgCtrlStatusReport, nil)

	log.Printf("[INFO] sending ctrl_status to %s", agentID)
	c.Client.SendTo(agentID, uap.MsgCtrlStatus, uap.CtrlStatusPayload{})

	// 等待 report
	select {
	case resp := <-responseCh:
		var report uap.CtrlStatusReportPayload
		json.Unmarshal(resp.Payload, &report)
		result := uap.BuildToolResult(msg.ID, report, fmt.Sprintf("agent %s: state=%s tasks=%d/%d uptime=%ds",
			report.AgentName, report.State, report.ActiveTasks, report.Capacity, report.Uptime))
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   true,
			Result:    result.Result,
		})
	case <-time.After(10 * time.Second):
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID,
			fmt.Sprintf("等待 %s 的 status report 超时（10s），目标 agent 可能不在线", agentID)))
	}
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

	workspaces := c.cfg.Workspaces
	if workspaces == nil {
		workspaces = []string{}
	}

	return RegisterPayload{
		AgentID:       c.AgentID,
		Name:          c.cfg.AgentName,
		Workspaces:    workspaces,
		Projects:      c.cfg.ProjectNames(),
		Tools:         []string{"deploy"},
		MaxConcurrent: c.cfg.MaxConcurrent,
		AuthToken:     c.cfg.AuthToken,
		DeployTargets: c.cfg.TargetNames,
		SSHHosts:      c.cfg.SSHHosts,
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
