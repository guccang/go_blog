package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentbase"
	"uap"
)

// Connection 通过 UAP gateway 的客户端连接管理
type Connection struct {
	*agentbase.AgentBase // 组合基类

	cfg         *AgentConfig
	agent       *Agent
	fileToolKit *agentbase.FileToolKit
}

// NewConnection 创建连接管理器
func NewConnection(cfg *AgentConfig, agent *Agent) *Connection {
	fileToolKit := agentbase.NewFileToolKit("Codegen", agent.findProjectPath)

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agent.ID,
		AgentType:   "codegen",
		AgentName:   cfg.AgentName,
		Description: "代码编写、项目管理、编码会话",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildCodegenToolDefs(fileToolKit),
		Meta: map[string]any{
			"workspaces":       cfg.Workspaces,
			"models":           agent.ScanSettings(),
			"claudecode_models": agent.ScanClaudeCodeSettings(),
			"opencode_models":  agent.ScanOpenCodeSettings(),
			"coding_tools":     agent.ScanTools(),
		},
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		agent:       agent,
		fileToolKit: fileToolKit,
	}

	// 注册消息处理器
	c.RegisterHandler(MsgTaskAssign, c.handleTaskAssign)
	c.RegisterHandler(MsgTaskStop, c.handleTaskStop)
	c.RegisterHandler(MsgFileRead, c.handleFileRead)
	c.RegisterHandler(MsgTreeRead, c.handleTreeRead)
	c.RegisterHandler(MsgProjectCreate, c.handleProjectCreate)
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgNotify, c.handleNotify)
	c.RegisterHandler(uap.MsgError, c.handleError)

	// 启用协议层
	c.EnableProtocolLayer(&agentbase.ProtocolLayerConfig{
		TargetAgentID: cfg.GoBackendAgentID,
		BuildRegister: c.buildRegisterPayload,
		BuildHeartbeat: c.buildHeartbeatPayload,
	})

	return c
}

// ========================= 消息处理器 =========================

// handleTaskAssign 处理任务分配
func (c *Connection) handleTaskAssign(msg *uap.Message) {
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
}

// handleTaskStop 处理停止任务
func (c *Connection) handleTaskStop(msg *uap.Message) {
	var payload TaskStopPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[INFO] stop task: session=%s", payload.SessionID)
	c.agent.StopTask(payload.SessionID)
}

// handleFileRead 处理文件读取请求
func (c *Connection) handleFileRead(msg *uap.Message) {
	var payload FileReadPayload
	json.Unmarshal(msg.Payload, &payload)
	go c.agent.HandleFileRead(c, &payload)
}

// handleTreeRead 处理目录树读取请求
func (c *Connection) handleTreeRead(msg *uap.Message) {
	var payload TreeReadPayload
	json.Unmarshal(msg.Payload, &payload)
	go c.agent.HandleTreeRead(c, &payload)
}

// handleProjectCreate 处理项目创建请求
func (c *Connection) handleProjectCreate(msg *uap.Message) {
	var payload ProjectCreatePayload
	json.Unmarshal(msg.Payload, &payload)
	go c.agent.HandleProjectCreate(c, &payload)
}

// handleToolCallMsg 处理工具调用（包装器）
func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	go c.handleToolCall(msg)
}

// handleNotify 处理通知消息
func (c *Connection) handleNotify(msg *uap.Message) {
	var payload uap.NotifyPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[INFO] notify from %s: channel=%s to=%s content=%s", msg.From, payload.Channel, payload.To, payload.Content)
}

// handleError 处理错误消息
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[WARN] gateway error: %s - %s", payload.Code, payload.Message)
}

// ========================= 协议层载荷构建 =========================

// SendMsg 发送消息给 go_blog-agent（通过 gateway 路由）
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	targetAgent := c.cfg.GoBackendAgentID
	return c.Client.SendTo(targetAgent, msgType, payload)
}

// buildRegisterPayload 构建注册消息载荷
func (c *Connection) buildRegisterPayload() interface{} {
	return RegisterPayload{
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
}

// buildHeartbeatPayload 构建心跳消息载荷
func (c *Connection) buildHeartbeatPayload() interface{} {
	return HeartbeatPayload{
		AgentID:          c.agent.ID,
		AgentType:        c.cfg.AgentType,
		ActiveSessions:   c.agent.ActiveCount(),
		Load:             c.agent.LoadFactor(),
		Projects:         c.agent.ScanProjects(),
		Models:           c.agent.ScanSettings(),
		ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(),
		OpenCodeModels:   c.agent.ScanOpenCodeSettings(),
		Tools:            c.agent.ScanTools(),
	}
}

// ========================= Tool 自注册 =========================

// buildCodegenToolDefs 构建 codegen-agent 的 UAP 工具定义列表
func buildCodegenToolDefs(ftk *agentbase.FileToolKit) []uap.ToolDef {
	defs := []uap.ToolDef{
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
			Description: "启动 AI 编码会话（同步等待完成，进度通过 stream_event 推送）。重要：prompt 参数必须使用用户的原始输入原文，禁止修改、缩写、翻译或重新措辞。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "项目名称"},
					"prompt":  map[string]interface{}{"type": "string", "description": "用户的原始编码需求，必须完整保留用户输入的原文，不得修改、缩写、翻译或重新措辞"},
					"model":   map[string]interface{}{"type": "string", "description": "模型配置名称（可选）"},
					"tool":    map[string]interface{}{"type": "string", "description": "编码工具（可选，claudecode/opencode）"},
				},
				"required": []string{"project", "prompt"},
			}),
		},
		{
			Name:        "CodegenSendMessage",
			Description: "向编码会话追加消息并等待完成（基于上一次会话续接）。重要：prompt 参数必须使用用户的原始输入原文，禁止修改或重新措辞。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt":     map[string]interface{}{"type": "string", "description": "用户的原始消息内容，必须完整保留用户输入的原文，不得修改或重新措辞"},
					"session_id": map[string]interface{}{"type": "string", "description": "要续接的会话ID（可选，默认使用最近的会话）"},
				},
				"required": []string{"prompt"},
			}),
		},
		{
			Name:        "CodegenGetStatus",
			Description: "查看编码会话状态。传入 session_id 查询指定会话的状态（in_progress/completed/failed），不传则返回全局概览",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string", "description": "要查询的会话ID（可选，不传返回全局状态）"},
				},
			}),
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
	// 追加 CodegenExecEnvBash（供 env-agent 远程执行环境检测命令）
	for _, td := range ftk.ToolDefs() {
		if strings.HasSuffix(td.Name, "ExecEnvBash") {
			defs = append(defs, td)
		}
	}
	return defs
}
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
	case "CodegenListProjects":
		result = c.toolListProjects()
	case "CodegenCreateProject":
		result = c.toolCreateProject(args)
	case "CodegenStartSession":
		result = c.toolStartSession(args)
	case "CodegenSendMessage":
		result = c.toolSendMessage(args)
	case "CodegenGetStatus":
		result = c.toolGetStatus(args)
	case "CodegenStopSession":
		result = c.toolStopSession(args)
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

// toolListProjects 列出本 agent 上的编码项目
func (c *Connection) toolListProjects() string {
	projects := c.agent.ScanProjects()
	tools := c.agent.ScanTools()
	models := c.agent.ScanSettings()
	data := map[string]interface{}{
		"projects": projects,
		"tools":    tools,
		"models":   models,
		"agent":    c.cfg.AgentName,
	}
	result := uap.BuildToolResult("", data, fmt.Sprintf("列出%d个项目", len(projects)))
	return result.Result
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
	result := uap.BuildToolResult("", map[string]string{"name": name, "path": projectPath}, fmt.Sprintf("项目 %s 已创建", name))
	return result.Result
}

// toolStartSession 启动编码会话（同步等待完成）
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

	// 注册完成通知，同步等待任务完成
	completionCh := c.agent.RegisterCompletion(sessionID)
	c.agent.RecordSession(sessionID, project, model, tool)
	go c.agent.ExecuteTask(c, task)

	result := <-completionCh
	if result.Status != "done" {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, result.Error)
	}

	data := map[string]interface{}{
		"session_id":    sessionID,
		"project_dir":   result.ProjectDir,
		"summary":       result.Summary,
		"files_written": result.FilesWritten,
		"files_edited":  result.FilesEdited,
	}
	// 编码完成但无任何文件产出，附加警告
	if result.FilesWritten == 0 && result.FilesEdited == 0 {
		data["warning"] = "编码会话完成但未产生任何文件变更，项目目录可能为空"
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("编码会话 %s 完成", sessionID))
	return tr.Result
}

// toolSendMessage 向编码会话追加消息（同步等待完成）
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

	// 注册完成通知，同步等待任务完成
	completionCh := c.agent.RegisterCompletion(newSessionID)
	c.agent.RecordSession(newSessionID, rec.Project, rec.Model, rec.Tool)
	go c.agent.ExecuteTask(c, task)

	result := <-completionCh
	if result.Status != "done" {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, newSessionID, result.Error)
	}

	data := map[string]interface{}{
		"session_id":    newSessionID,
		"project_dir":   result.ProjectDir,
		"summary":       result.Summary,
		"files_written": result.FilesWritten,
		"files_edited":  result.FilesEdited,
	}
	// 编码完成但无任何文件产出，附加警告
	if result.FilesWritten == 0 && result.FilesEdited == 0 {
		data["warning"] = "编码会话完成但未产生任何文件变更，项目目录可能为空"
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("编码会话 %s 完成", newSessionID))
	return tr.Result
}

// toolGetStatus 查看编码会话状态（支持 per-session 查询）
func (c *Connection) toolGetStatus(args map[string]interface{}) string {
	sessionID, _ := args["session_id"].(string)

	// 指定 session_id → 返回该会话状态
	if sessionID != "" {
		rec := c.agent.GetSession(sessionID)
		if rec == nil {
			return fmt.Sprintf(`{"success":false,"error":"会话 %s 不存在"}`, sessionID)
		}
		data := map[string]interface{}{
			"session_id": sessionID,
			"project":    rec.Project,
			"status":     rec.Status,
			"active":     rec.Active,
		}
		if rec.Summary != "" {
			data["summary"] = rec.Summary
		}
		tr := uap.BuildToolResult("", data, fmt.Sprintf("会话 %s 状态: %s", sessionID, rec.Status))
		return tr.Result
	}

	// 无 session_id → 返回全局概览（原行为）
	c.agent.mu.Lock()
	activeCount := len(c.agent.activeTasks)
	var activeSessions []string
	for sid := range c.agent.activeTasks {
		activeSessions = append(activeSessions, sid)
	}
	c.agent.mu.Unlock()

	data := map[string]interface{}{
		"active":          activeCount > 0,
		"active_count":    activeCount,
		"active_sessions": activeSessions,
		"max_concurrent":  c.agent.cfg.MaxConcurrent,
		"agent":           c.cfg.AgentName,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("活跃会话 %d 个", activeCount))
	return tr.Result
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
	tr := uap.BuildToolResult("", map[string]string{"session_id": sessionID}, "编码会话已停止")
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

// escapeJSON 转义字符串中的特殊字符以嵌入 JSON 字符串值
func escapeJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	// json.Marshal 返回带引号的字符串，去掉首尾引号
	return string(b[1 : len(b)-1])
}

