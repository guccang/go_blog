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
		Description: "项目代码分析、架构评审、优化建议（基于 ACP 协议）",
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

// toolListProjects 列出可分析的项目
func (c *Connection) toolListProjects() string {
	projects := c.agent.ScanProjects()
	data := map[string]interface{}{
		"projects": projects,
		"agent":    c.cfg.AgentName,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("列出%d个可分析项目", len(projects)))
	return tr.Result
}

// toolAnalyzeProject 分析项目（同步等待完成）
func (c *Connection) toolAnalyzeProject(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	prompt, _ := args["prompt"].(string)

	if project == "" || prompt == "" {
		return `{"success":false,"error":"缺少 project 或 prompt 参数"}`
	}
	if !c.agent.CanAccept() {
		return `{"success":false,"error":"agent 繁忙，无法接受新分析任务"}`
	}

	sessionID := fmt.Sprintf("acp_%d", time.Now().UnixNano())

	result, err := c.agent.ExecuteAnalysis(c, sessionID, project, prompt)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"session_id":"%s","error":"%s"}`, sessionID, escapeJSON(err.Error()))
	}

	data := map[string]string{
		"session_id": sessionID,
		"project":    project,
		"report":     result,
	}
	tr := uap.BuildToolResult("", data, fmt.Sprintf("项目 %s 分析完成", project))
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
