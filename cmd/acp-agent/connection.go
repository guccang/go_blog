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
		Description: "项目代码分析、编码（支持 Claude ACP / Codex CLI）",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildACPToolDefs(),
		Meta: map[string]any{
			"projects":         projectNames(agent.ScanProjects()),
			"workspaces":       cfg.Workspaces,
			"coding_backend":   cfg.EffectiveCodingBackend(),
			"analysis_timeout": cfg.AnalysisTimeout,
			"coding_backends":  []string{BackendClaudeACP, BackendCodexExec},
		},
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		agent:     agent,
	}

	// 注册消息处理器
	c.RegisterToolCallHandler(c.handleToolCall)
	c.RegisterHandler(uap.MsgNotify, c.handleNotify)
	c.RegisterHandler(uap.MsgError, c.handleError)
	c.RegisterHandler(uap.MsgPermissionResponse, c.handlePermissionResponse)
	c.RegisterHandler(uap.MsgSetMode, c.handleSetMode)

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

// handleToolCancelCallback 处理工具取消回调（停止正在执行的 ACP 会话）
func (c *Connection) handleToolCancelCallback(toolName, msgID string) {
	// 停止最近的活跃会话（ACP 工具通常只有一个活跃会话）
	sessionID, rec := c.agent.GetLastSession()
	if rec != nil && rec.Active {
		log.Printf("[INFO] stopping active session: %s", sessionID)
		c.agent.StopTask(sessionID)
	}
}

// ========================= 协议层载荷构建 =========================

// SendMsg 发送消息给 blog-agent-agent
func (c *Connection) SendMsg(msgType string, payload interface{}) error {
	return c.Client.SendTo(c.cfg.GoBackendAgentID, msgType, payload)
}

func (c *Connection) buildRegisterPayload() interface{} {
	return RegisterPayload{
		AgentID:       c.agent.ID,
		Name:          c.cfg.AgentName,
		AgentType:     c.cfg.AgentType,
		Workspaces:    c.cfg.Workspaces,
		Projects:      projectNames(c.agent.ScanProjects()),
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
		Projects:       projectNames(c.agent.ScanProjects()),
	}
}

// projectNames 从 ProjectInfo 列表提取项目名
func projectNames(projects []ProjectInfo) []string {
	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name
	}
	return names
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
			Name:        "AcpCreateProject",
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
			Name:        "AcpStartSession",
			Description: "启动编码会话（后端由 acp-agent 配置决定，当前支持 Claude ACP / Codex CLI；同步等待完成，进度通过 stream_event 推送）。默认在本轮完成后自动关闭子进程；只有明确需要多轮续聊时才传 keep_session=true。重要：prompt 参数必须使用用户的原始输入原文，禁止修改、缩写、翻译或重新措辞。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project":         map[string]interface{}{"type": "string", "description": "项目名称"},
					"prompt":          map[string]interface{}{"type": "string", "description": "用户的原始编码需求，必须完整保留用户输入的原文，不得修改、缩写、翻译或重新措辞"},
					"extra_args":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "动态 CLI 参数（如 --dangerously-skip-permissions, --settings path）"},
					"interactive":     map[string]interface{}{"type": "boolean", "description": "是否启用交互式权限模式（默认 false）"},
					"caller_agent_id": map[string]interface{}{"type": "string", "description": "调用方 agent ID（交互模式下权限请求和流式事件发给该 agent）"},
					"keep_session":    map[string]interface{}{"type": "boolean", "description": "是否在本轮完成后保留 ACP 会话供后续继续对话；默认 false"},
				},
				"required": []string{"project"},
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
	case "AcpCreateProject":
		result = c.toolCreateProject(args)
	case "AcpStartSession":
		result = c.toolStartSession(msg.From, msg.ID, args)
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

	success, errText := interpretToolResult(result)
	if !success {
		log.Printf("[WARN] tool_call failed from=%s tool=%s err=%s", msg.From, payload.ToolName, errText)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Result:    result,
			Error:     errText,
		})
		return
	}

	c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    result,
	})
}

func interpretToolResult(raw string) (bool, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true, ""
	}

	var payload struct {
		Success *bool  `json:"success"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return true, ""
	}
	if payload.Success == nil || *payload.Success {
		return true, ""
	}
	if strings.TrimSpace(payload.Error) != "" {
		return false, strings.TrimSpace(payload.Error)
	}
	if strings.TrimSpace(payload.Message) != "" {
		return false, strings.TrimSpace(payload.Message)
	}
	return false, "tool execution failed"
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

func (c *Connection) toolCreateProject(args map[string]interface{}) string {
	name, _ := args["name"].(string)
	if name == "" {
		return `{"success":false,"error":"缺少项目名称(name参数)"}`
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return `{"success":false,"error":"无效的项目名称"}`
	}
	for _, project := range c.agent.ScanProjects() {
		if project.Name == name {
			return fmt.Sprintf(`{"success":false,"error":"项目 %s 已存在"}`, name)
		}
	}
	if len(c.agent.cfg.Workspaces) == 0 {
		return `{"success":false,"error":"未配置 workspace"}`
	}

	projectPath := filepath.Join(c.agent.cfg.Workspaces[0], name)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Sprintf(`{"success":false,"error":"创建失败: %v"}`, err)
	}
	log.Printf("[INFO] project created via tool_call: %s at %s", name, projectPath)
	tr := uap.BuildToolResult("", map[string]string{"name": name, "path": projectPath}, fmt.Sprintf("项目 %s 已创建", name))
	return tr.Result
}

func (c *Connection) toolAnalyzeProject(callerAgentID, requestID string, args map[string]interface{}) string {
	project, _ := args["project"].(string)
	prompt, _ := args["prompt"].(string)

	if project == "" || prompt == "" {
		return `{"success":false,"error":"缺少 project 或 prompt 参数"}`
	}
	if !c.agent.CanAccept() {
		return `{"success":false,"error":"agent 繁忙，无法接受新任务"}`
	}

	sessionID := fmt.Sprintf("acp_%d", time.Now().UnixNano())

	result, err := c.agent.ExecuteACP(c, sessionID, requestID, project, prompt, nil, false, callerAgentID, false)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, escapeJSON(err.Error()))
	}

	data := map[string]interface{}{
		"session_id":    sessionID,
		"project":       project,
		"report":        result.Summary,
		"files_written": result.FilesWritten,
		"files_edited":  result.FilesEdited,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("项目 %s 分析完成", project))
	return tr.Result
}

func (c *Connection) toolStartSession(callerAgentID, requestID string, args map[string]interface{}) string {
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
	overrideCallerAgentID, _ := args["caller_agent_id"].(string)
	keepSession, _ := args["keep_session"].(bool)
	if overrideCallerAgentID != "" {
		callerAgentID = overrideCallerAgentID
	}

	sessionID := fmt.Sprintf("acp_%d", time.Now().UnixNano())

	result, err := c.agent.ExecuteACP(c, sessionID, requestID, project, prompt, extraArgs, interactive, callerAgentID, keepSession)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, escapeJSON(err.Error()))
	}

	data := map[string]interface{}{
		"session_id":      result.SessionID,
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
	tr := uap.BuildToolResult("", data, fmt.Sprintf("%s 会话 %s 完成", c.cfg.BackendLabel(), sessionID))
	return tr.Result
}

func (c *Connection) toolSendMessage(callerAgentID, requestID string, args map[string]interface{}) string {
	prompt, _ := args["prompt"].(string)
	sessionID, _ := args["session_id"].(string)
	interactive, _ := args["interactive"].(bool)
	overrideCallerAgentID, _ := args["caller_agent_id"].(string)
	keepSession, _ := args["keep_session"].(bool)
	if overrideCallerAgentID != "" {
		callerAgentID = overrideCallerAgentID
	}

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

	result, err := c.agent.SendMessage(c, sessionID, requestID, prompt, interactive, callerAgentID, keepSession)
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
