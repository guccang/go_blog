package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	client.Tools = buildCodegenToolDefs()
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

	case uap.MsgToolCall:
		go c.handleToolCall(msg)

	case uap.MsgNotify:
		// 来自其他 agent 的单向通知（如微信消息通知等）
		var payload uap.NotifyPayload
		json.Unmarshal(msg.Payload, &payload)
		log.Printf("[INFO] notify from %s: channel=%s to=%s content=%s", msg.From, payload.Channel, payload.To, payload.Content)

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
		AgentType:        c.cfg.AgentType,
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
		AgentType:        c.cfg.AgentType,
		ActiveSessions:   c.agent.ActiveCount(),
		Load:             c.agent.LoadFactor(),
		Projects:         c.agent.ScanProjects(),
		Models:           c.agent.ScanSettings(),
		ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(),
		OpenCodeModels:   c.agent.ScanOpenCodeSettings(),
		Tools:            c.agent.ScanTools(),
	})
}

// ========================= Tool 自注册 =========================

// buildCodegenToolDefs 构建 codegen-agent 的 UAP 工具定义列表
func buildCodegenToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "CodegenListProjects",
			Description: "列出本 agent 上的编码项目、可用工具和模型配置",
			Parameters:  mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "CodegenCreateProject",
			Description: "在本 agent 上创建新编码项目",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string", "description": "项目名称"},
				},
				"required": []string{"name"},
			}),
		},
		{
			Name:        "CodegenStartSession",
			Description: "启动 AI 编码会话（异步执行，进度通过 stream_event 推送）",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "项目名称"},
					"prompt":  map[string]interface{}{"type": "string", "description": "编码需求描述"},
					"model":   map[string]interface{}{"type": "string", "description": "模型配置名称（可选）"},
					"tool":    map[string]interface{}{"type": "string", "description": "编码工具（可选，claudecode/opencode）"},
				},
				"required": []string{"project", "prompt"},
			}),
		},
		{
			Name:        "CodegenSendMessage",
			Description: "向编码会话追加消息（基于上一次会话续接）",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt":     map[string]interface{}{"type": "string", "description": "追加的消息内容"},
					"session_id": map[string]interface{}{"type": "string", "description": "要续接的会话ID（可选，默认使用最近的会话）"},
				},
				"required": []string{"prompt"},
			}),
		},
		{
			Name:        "CodegenGetStatus",
			Description: "查看当前编码会话运行状态",
			Parameters: mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "CodegenStopSession",
			Description: "停止编码会话",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string", "description": "要停止的会话ID（可选，默认停止最近的活跃会话）"},
				},
			}),
		},
	}
}

// handleToolCall 处理来自 gateway 的工具调用请求
func (c *Connection) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[WARN] invalid tool_call payload: %v", err)
		c.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
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
			c.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
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
	case "CodegenListProjects":
		result = c.toolListProjects()
	case "CodegenCreateProject":
		result = c.toolCreateProject(args)
	case "CodegenStartSession":
		result = c.toolStartSession(args)
	case "CodegenSendMessage":
		result = c.toolSendMessage(args)
	case "CodegenGetStatus":
		result = c.toolGetStatus()
	case "CodegenStopSession":
		result = c.toolStopSession(args)
	default:
		c.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
		})
		return
	}

	c.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    result,
	})
}

// toolListProjects 列出本 agent 上的编码项目
func (c *Connection) toolListProjects() string {
	projects := c.agent.ScanProjects()
	tools := c.agent.ScanTools()
	models := c.agent.ScanSettings()
	return string(mustMarshalJSON(map[string]interface{}{
		"success":  true,
		"projects": projects,
		"tools":    tools,
		"models":   models,
		"agent":    c.cfg.AgentName,
	}))
}

// toolCreateProject 在本 agent 上创建编码项目
func (c *Connection) toolCreateProject(args map[string]interface{}) string {
	name, _ := args["name"].(string)
	if name == "" {
		return `{"success":false,"error":"缺少项目名称(name参数)"}`
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return `{"success":false,"error":"无效的项目名称"}`
	}
	if existing := c.agent.findProjectPath(name); existing != "" {
		return fmt.Sprintf(`{"success":false,"error":"项目 %s 已存在"}`, name)
	}
	if len(c.agent.cfg.Workspaces) == 0 {
		return `{"success":false,"error":"未配置 workspace"}`
	}

	projectPath := filepath.Join(c.agent.cfg.Workspaces[0], name)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Sprintf(`{"success":false,"error":"创建失败: %v"}`, err)
	}
	ensureGitInit(projectPath)
	log.Printf("[INFO] project created via tool_call: %s at %s", name, projectPath)
	return fmt.Sprintf(`{"success":true,"message":"项目 %s 已创建"}`, name)
}

// toolStartSession 启动编码会话
func (c *Connection) toolStartSession(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	prompt, _ := args["prompt"].(string)
	model, _ := args["model"].(string)
	tool, _ := args["tool"].(string)

	if project == "" || prompt == "" {
		return `{"success":false,"error":"缺少 project 或 prompt 参数"}`
	}
	if !c.agent.CanAccept() {
		return `{"success":false,"error":"agent 繁忙，无法接受新任务"}`
	}

	sessionID := fmt.Sprintf("tc_%d", time.Now().UnixNano())
	task := &TaskAssignPayload{
		SessionID: sessionID,
		Project:   project,
		Prompt:    prompt,
		Model:     model,
		Tool:      tool,
	}

	c.agent.RecordSession(sessionID, project, model, tool)
	go c.agent.ExecuteTask(c, task)

	return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"编码会话已启动"}`, sessionID)
}

// toolSendMessage 向编码会话追加消息（基于上一次会话续接）
func (c *Connection) toolSendMessage(args map[string]interface{}) string {
	prompt, _ := args["prompt"].(string)
	sessionID, _ := args["session_id"].(string)

	if prompt == "" {
		return `{"success":false,"error":"缺少 prompt 参数"}`
	}

	// 查找要续接的会话
	var rec *sessionRecord
	if sessionID != "" {
		rec = c.agent.GetSession(sessionID)
	} else {
		sessionID, rec = c.agent.GetLastSession()
	}
	if rec == nil {
		return `{"success":false,"error":"未找到可续接的会话"}`
	}
	if rec.Active {
		return `{"success":false,"error":"会话正在执行中，请等待完成后再发送消息"}`
	}
	if !c.agent.CanAccept() {
		return `{"success":false,"error":"agent 繁忙，无法接受新任务"}`
	}

	// 启动新会话续接上一次（通过 --resume）
	newSessionID := fmt.Sprintf("tc_%d", time.Now().UnixNano())
	task := &TaskAssignPayload{
		SessionID:     newSessionID,
		Project:       rec.Project,
		Prompt:        prompt,
		Model:         rec.Model,
		Tool:          rec.Tool,
		ClaudeSession: rec.ClaudeSession,
	}

	c.agent.RecordSession(newSessionID, rec.Project, rec.Model, rec.Tool)
	go c.agent.ExecuteTask(c, task)

	return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"消息已发送，基于会话 %s 续接"}`, newSessionID, sessionID)
}

// toolGetStatus 查看当前编码会话状态
func (c *Connection) toolGetStatus() string {
	c.agent.mu.Lock()
	activeCount := len(c.agent.activeTasks)
	var activeSessions []string
	for sid := range c.agent.activeTasks {
		activeSessions = append(activeSessions, sid)
	}
	c.agent.mu.Unlock()

	return string(mustMarshalJSON(map[string]interface{}{
		"success":         true,
		"active":          activeCount > 0,
		"active_count":    activeCount,
		"active_sessions": activeSessions,
		"max_concurrent":  c.agent.cfg.MaxConcurrent,
		"agent":           c.cfg.AgentName,
	}))
}

// toolStopSession 停止编码会话
func (c *Connection) toolStopSession(args map[string]interface{}) string {
	sessionID, _ := args["session_id"].(string)

	if sessionID == "" {
		// 停止最近的活跃会话
		c.agent.mu.Lock()
		for sid := range c.agent.activeTasks {
			sessionID = sid
		}
		c.agent.mu.Unlock()
	}

	if sessionID == "" {
		return `{"success":false,"error":"没有活跃的编码会话"}`
	}

	c.agent.StopTask(sessionID)
	return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"编码会话已停止"}`, sessionID)
}

// mustMarshalJSON 将值序列化为 JSON，失败时返回空对象
func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
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
