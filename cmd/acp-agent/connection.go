package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"agentbase"
	"uap"
)

// Connection 通过 UAP gateway 的客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg   *AgentConfig
	agent *Agent
}

// NewConnection 创建连接管理器
func NewConnection(cfg *AgentConfig, agent *Agent) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agent.ID,
		AgentType:   "acp",
		AgentName:   cfg.AgentName,
		Description: "项目代码分析、编码（基于 ACP 协议）",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildACPToolDefs(),
		Meta: map[string]any{
			"workspaces":       cfg.Workspaces,
			"acp_agent_cmd":    cfg.ACPAgentCmd,
			"analysis_timeout": cfg.AnalysisTimeout,
		},
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		agent:     agent,
	}

	// 注册消息处理器
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgNotify, c.handleNotify)
	c.RegisterHandler(uap.MsgError, c.handleError)
	c.RegisterHandler(uap.MsgPermissionResponse, c.handlePermissionResponse)
	c.RegisterHandler(uap.MsgSetMode, c.handleSetMode)

	// 启用协议层
	c.EnableProtocolLayer(&agentbase.ProtocolLayerConfig{
		TargetAgentID:  cfg.GoBackendAgentID,
		BuildRegister:  c.buildRegisterPayload,
		BuildHeartbeat: c.buildHeartbeatPayload,
	})

	return c
}

// ========================= 消息处理器 =========================

func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	go c.handleToolCall(msg)
}

func (c *Connection) handleNotify(msg *uap.Message) {
	var payload uap.NotifyPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[INFO] notify from %s: channel=%s to=%s content=%s", msg.From, payload.Channel, payload.To, payload.Content)
}

func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	json.Unmarshal(msg.Payload, &payload)
	log.Printf("[WARN] gateway error: %s - %s", payload.Code, payload.Message)
}

// handlePermissionResponse 处理 llm-agent 发来的权限回复
func (c *Connection) handlePermissionResponse(msg *uap.Message) {
	var payload uap.PermissionResponsePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[WARN] invalid permission_response payload: %v", err)
		return
	}
	log.Printf("[INFO] permission_response: session=%s option=%s cancelled=%v", payload.SessionID, payload.OptionID, payload.Cancelled)
	if err := c.agent.deliverPermissionResponse(payload.SessionID, payload.OptionID, payload.Cancelled); err != nil {
		log.Printf("[WARN] deliver permission response: %v", err)
	}
}

// handleSetMode 处理 llm-agent 发来的模式切换请求
func (c *Connection) handleSetMode(msg *uap.Message) {
	var payload uap.SetModePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[WARN] invalid set_mode payload: %v", err)
		return
	}
	log.Printf("[INFO] set_mode: session=%s mode=%s", payload.SessionID, payload.ModeID)
	if err := c.agent.setSessionMode(payload.SessionID, payload.ModeID); err != nil {
		log.Printf("[WARN] set session mode: %v", err)
	}
}

// ========================= 协议层载荷构建 =========================

// SendMsg 发送消息给 go_blog-agent
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	return c.Client.SendTo(c.cfg.GoBackendAgentID, msgType, payload)
}

func (c *Connection) buildRegisterPayload() interface{} {
	return RegisterPayload{
		AgentID:       c.agent.ID,
		Name:          c.cfg.AgentName,
		AgentType:     c.cfg.AgentType,
		Workspaces:    c.cfg.Workspaces,
		Projects:      c.agent.ScanProjects(),
		MaxConcurrent: c.cfg.MaxConcurrent,
		AuthToken:     c.cfg.AuthToken,
	}
}

func (c *Connection) buildHeartbeatPayload() interface{} {
	return HeartbeatPayload{
		AgentID:        c.agent.ID,
		AgentType:      c.cfg.AgentType,
		ActiveSessions: c.agent.ActiveCount(),
		Load:           c.agent.LoadFactor(),
		Projects:       c.agent.ScanProjects(),
	}
}

// ========================= Tool 定义 =========================

func buildACPToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "AcpListProjects",
			Description: "列出可分析的项目列表",
			Parameters:  mustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "AcpAnalyzeProject",
			Description: "通过 ACP 协议调用 Claude Code 分析项目代码，给出优化建议。同步等待完成，进度通过 stream_event 推送。重要：prompt 参数必须使用用户的原始输入原文，禁止修改、缩写、翻译或重新措辞。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "项目名称"},
					"prompt":  map[string]interface{}{"type": "string", "description": "用户的原始分析需求，必须完整保留用户输入的原文，不得修改、缩写、翻译或重新措辞"},
				},
				"required": []string{"project", "prompt"},
			}),
		},
		{
			Name:        "AcpStartSession",
			Description: "启动 ACP 编码会话（同步等待完成，进度通过 stream_event 推送）。重要：prompt 参数必须使用用户的原始输入原文，禁止修改、缩写、翻译或重新措辞。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project":         map[string]interface{}{"type": "string", "description": "项目名称"},
					"prompt":          map[string]interface{}{"type": "string", "description": "用户的原始编码需求，必须完整保留用户输入的原文，不得修改、缩写、翻译或重新措辞"},
					"extra_args":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "动态 CLI 参数（如 --dangerously-skip-permissions, --settings path）"},
					"interactive":     map[string]interface{}{"type": "boolean", "description": "是否启用交互式权限模式（默认 false）"},
					"caller_agent_id": map[string]interface{}{"type": "string", "description": "调用方 agent ID（交互模式下权限请求和流式事件发给该 agent）"},
				},
				"required": []string{"project"},
			}),
		},
		{
			Name:        "AcpSendMessage",
			Description: "向 ACP 会话追加消息并等待完成（复用已有 ACP 会话多轮对话）。重要：prompt 参数必须使用用户的原始输入原文，禁止修改或重新措辞。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt":          map[string]interface{}{"type": "string", "description": "用户的原始消息内容，必须完整保留用户输入的原文，不得修改或重新措辞"},
					"session_id":      map[string]interface{}{"type": "string", "description": "要续接的会话ID（可选，默认使用最近的会话）"},
					"interactive":     map[string]interface{}{"type": "boolean", "description": "是否启用交互式权限模式"},
					"caller_agent_id": map[string]interface{}{"type": "string", "description": "调用方 agent ID"},
				},
				"required": []string{"prompt"},
			}),
		},
		{
			Name:        "AcpGetStatus",
			Description: "查看会话状态。传入 session_id 查询指定会话，不传则返回全局概览",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string", "description": "要查询的会话ID（可选，不传返回全局状态）"},
				},
			}),
		},
		{
			Name:        "AcpStopSession",
			Description: "停止 ACP 会话",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string", "description": "要停止的会话ID（可选，默认停止最近的活跃会话）"},
				},
			}),
		},
	}
}

// ========================= Tool 调用处理 =========================

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

	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
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
	case "AcpListProjects":
		result = c.toolListProjects()
	case "AcpAnalyzeProject":
		result = c.toolAnalyzeProject(args)
	case "AcpStartSession":
		result = c.toolStartSession(args)
	case "AcpSendMessage":
		result = c.toolSendMessage(args)
	case "AcpGetStatus":
		result = c.toolGetStatus(args)
	case "AcpStopSession":
		result = c.toolStopSession(args)
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

// ========================= Tool 实现 =========================

func (c *Connection) toolListProjects() string {
	projects := c.agent.ScanProjects()
	data := map[string]interface{}{
		"projects": projects,
		"agent":    c.cfg.AgentName,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("列出%d个可分析项目", len(projects)))
	return tr.Result
}

func (c *Connection) toolAnalyzeProject(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	prompt, _ := args["prompt"].(string)

	if project == "" || prompt == "" {
		return `{"success":false,"error":"缺少 project 或 prompt 参数"}`
	}
	if !c.agent.CanAccept() {
		return `{"success":false,"error":"agent 繁忙，无法接受新任务"}`
	}

	sessionID := fmt.Sprintf("acp_%d", time.Now().UnixNano())

	result, err := c.agent.ExecuteACP(c, sessionID, project, prompt, nil, false, "")
	if err != nil {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, escapeJSON(err.Error()))
	}

	data := map[string]interface{}{
		"session_id":    sessionID,
		"project":      project,
		"report":       result.Summary,
		"files_written": result.FilesWritten,
		"files_edited":  result.FilesEdited,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("项目 %s 分析完成", project))
	return tr.Result
}

func (c *Connection) toolStartSession(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	prompt, _ := args["prompt"].(string)

	if project == "" {
		return `{"success":false,"error":"缺少 project 参数"}`
	}
	if !c.agent.CanAccept() {
		return `{"success":false,"error":"agent 繁忙，无法接受新任务"}`
	}

	// 解析新参数
	var extraArgs []string
	if rawArgs, ok := args["extra_args"].([]interface{}); ok {
		for _, a := range rawArgs {
			if s, ok := a.(string); ok {
				extraArgs = append(extraArgs, s)
			}
		}
	}
	interactive, _ := args["interactive"].(bool)
	callerAgentID, _ := args["caller_agent_id"].(string)

	sessionID := fmt.Sprintf("acp_%d", time.Now().UnixNano())

	result, err := c.agent.ExecuteACP(c, sessionID, project, prompt, extraArgs, interactive, callerAgentID)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, escapeJSON(err.Error()))
	}

	data := map[string]interface{}{
		"session_id":      sessionID,
		"project_dir":     result.ProjectDir,
		"summary":         result.Summary,
		"files_written":   result.FilesWritten,
		"files_edited":    result.FilesEdited,
		"model":           result.Model,
		"current_mode":    result.CurrentMode,
		"available_modes": result.AvailableModes,
	}
	if result.FilesWritten == 0 && result.FilesEdited == 0 {
		data["warning"] = "会话完成但未产生任何文件变更"
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("ACP 会话 %s 完成", sessionID))
	return tr.Result
}

func (c *Connection) toolSendMessage(args map[string]interface{}) string {
	prompt, _ := args["prompt"].(string)
	sessionID, _ := args["session_id"].(string)
	interactive, _ := args["interactive"].(bool)
	callerAgentID, _ := args["caller_agent_id"].(string)

	if prompt == "" {
		return `{"success":false,"error":"缺少 prompt 参数"}`
	}

	// 查找目标会话
	if sessionID == "" {
		sessionID, _ = c.agent.GetLastSession()
	}
	if sessionID == "" {
		return `{"success":false,"error":"未找到可续接的会话"}`
	}

	rec := c.agent.GetSession(sessionID)
	if rec == nil {
		return `{"success":false,"error":"未找到可续接的会话"}`
	}
	if rec.Active {
		return `{"success":false,"error":"会话正在执行中，请等待完成后再发送消息"}`
	}

	result, err := c.agent.SendMessage(c, sessionID, prompt, interactive, callerAgentID)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, escapeJSON(err.Error()))
	}

	data := map[string]interface{}{
		"session_id":    sessionID,
		"project_dir":   result.ProjectDir,
		"summary":       result.Summary,
		"files_written": result.FilesWritten,
		"files_edited":  result.FilesEdited,
		"model":         result.Model,
		"current_mode":  result.CurrentMode,
	}
	if result.FilesWritten == 0 && result.FilesEdited == 0 {
		data["warning"] = "会话完成但未产生任何文件变更"
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("ACP 会话 %s 对话完成", sessionID))
	return tr.Result
}

func (c *Connection) toolGetStatus(args map[string]interface{}) string {
	sessionID, _ := args["session_id"].(string)

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
		// 从 ACPClient 获取 model/mode 信息
		c.agent.sessionsMu.Lock()
		fullRec, ok := c.agent.sessions[sessionID]
		c.agent.sessionsMu.Unlock()
		if ok && fullRec.ACPClient != nil {
			data["model"] = fullRec.ACPClient.GetModelID()
			data["current_mode"] = fullRec.ACPClient.GetCurrentModeID()
		}
		tr := uap.BuildToolResult("", data, fmt.Sprintf("会话 %s 状态: %s", sessionID, rec.Status))
		return tr.Result
	}

	// 全局概览
	activeCount := c.agent.ActiveCount()
	c.agent.sessionsMu.Lock()
	var activeSessions []string
	for sid, rec := range c.agent.sessions {
		if rec.Active {
			activeSessions = append(activeSessions, sid)
		}
	}
	c.agent.sessionsMu.Unlock()

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

func (c *Connection) toolStopSession(args map[string]interface{}) string {
	sessionID, _ := args["session_id"].(string)

	if sessionID == "" {
		c.agent.sessionsMu.Lock()
		for sid, rec := range c.agent.sessions {
			if rec.Active {
				sessionID = sid
				break
			}
		}
		c.agent.sessionsMu.Unlock()
	}

	if sessionID == "" {
		return `{"success":false,"error":"没有活跃的会话"}`
	}

	c.agent.StopTask(sessionID)
	tr := uap.BuildToolResult("", map[string]string{"session_id": sessionID}, "ACP 会话已停止")
	return tr.Result
}

// ========================= 工具函数 =========================

func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}

func escapeJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	return string(b[1 : len(b)-1])
}
