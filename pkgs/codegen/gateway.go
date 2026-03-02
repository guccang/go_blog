package codegen

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"sync"
	"time"

	"uap"
)

// MCPToolInfo MCP 工具定义信息（用于依赖注入，避免直接依赖 mcp 包）
type MCPToolInfo struct {
	Name        string      // MCP 回调名（如 RawAllBlogName）
	Description string      // 工具描述
	Parameters  interface{} // JSON Schema 参数
}

// MCP 桥接函数（由 agent 包注入，避免 codegen 直接依赖 mcp 的重量级传递依赖链）
var (
	// MCPCallInnerTools 调用 MCP 内部工具
	MCPCallInnerTools func(name string, args map[string]interface{}) string
	// MCPGetToolInfos 获取 MCP 工具定义列表
	MCPGetToolInfos func() []MCPToolInfo
)

// GatewaySender 通过 gateway 路由发送消息
type GatewaySender struct {
	client      *uap.Client
	toAgentID   string // 目标 agent ID（codegen-agent / deploy-agent 的 UAP ID）
}

// SendAgentMsg 通过 gateway 路由发送 AgentMessage
func (s *GatewaySender) SendAgentMsg(msgType string, payload interface{}) error {
	return s.client.SendTo(s.toAgentID, msgType, payload)
}

// GatewayBridge go_blog 的 gateway 适配层
type GatewayBridge struct {
	client *uap.Client
	pool   *AgentPool

	// wechat notify 处理器
	wechatHandler func(wechatUser, message string) string

	// UAP tool_name → MCP callback name 映射
	toolMapping map[string]string

	mu sync.Mutex
}

// 全局 gateway bridge 实例
var gatewayBridge *GatewayBridge

// InitGatewayBridge 初始化 go_blog 到 gateway 的连接
func InitGatewayBridge(gatewayURL, authToken string) {
	// 构建工具定义和映射表
	toolDefs, toolMapping := buildToolDefs()

	client := uap.NewClient(gatewayURL, "go_blog", "go_blog", "Go Blog Server")
	client.AuthToken = authToken
	client.Capacity = 100
	client.Tools = toolDefs
	client.Meta = map[string]any{
		"role": "backend",
	}

	bridge := &GatewayBridge{
		client:      client,
		pool:        agentPool,
		toolMapping: toolMapping,
	}

	client.OnMessage = bridge.handleMessage

	gatewayBridge = bridge

	// 后台连接 gateway（非阻塞）
	go func() {
		log.MessageF(log.ModuleAgent, "CodeGen: connecting to gateway at %s (tools=%d)", gatewayURL, len(toolDefs))
		client.Run()
	}()
}

// SetWechatHandler 设置微信命令处理器
func SetWechatHandler(handler func(wechatUser, message string) string) {
	if gatewayBridge != nil {
		gatewayBridge.mu.Lock()
		gatewayBridge.wechatHandler = handler
		gatewayBridge.mu.Unlock()
	}
}

// GetGatewayClient 获取 gateway 客户端（供外部使用）
func GetGatewayClient() *uap.Client {
	if gatewayBridge != nil {
		return gatewayBridge.client
	}
	return nil
}

// handleMessage 处理从 gateway 收到的 UAP 消息
func (b *GatewayBridge) handleMessage(msg *uap.Message) {
	// 解析 AgentMessage payload（codegen/deploy agent 发来的原协议消息）
	switch msg.Type {
	case MsgRegister:
		b.handleRegister(msg)

	case MsgHeartbeat:
		b.handleHeartbeat(msg)

	case MsgTaskAccepted:
		var payload TaskAcceptedPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_accepted payload: %v", err)
			return
		}
		agent := b.getAgent(msg.From)
		if agent != nil {
			agent.mu.Lock()
			agent.ActiveSessions[payload.SessionID] = true
			agent.mu.Unlock()
		}
		log.MessageF(log.ModuleAgent, "CodeGen: gateway agent %s accepted task %s", msg.From, payload.SessionID)

	case MsgTaskRejected:
		var payload TaskRejectedPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_rejected payload: %v", err)
			return
		}
		log.WarnF(log.ModuleAgent, "CodeGen: gateway agent %s rejected task %s: %s",
			msg.From, payload.SessionID, payload.Reason)
		if session := GetSession(payload.SessionID); session != nil {
			session.mu.Lock()
			session.Status = StatusError
			session.Error = "agent rejected: " + payload.Reason
			session.EndTime = time.Now()
			session.mu.Unlock()
			session.broadcast(StreamEvent{
				Type: "error",
				Text: "❌ Agent 拒绝任务: " + payload.Reason,
				Done: true,
			})
		}

	case MsgStreamEvent:
		var payload StreamEventPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid stream_event payload: %v", err)
			return
		}
		b.pool.handleStreamEvent(&payload)

		// 转发 task_event 给所有 wechat-agent
		b.forwardToWechatAgents(msg)

	case MsgTaskComplete:
		var payload TaskCompletePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_complete payload: %v", err)
			return
		}
		agent := b.getAgent(msg.From)
		b.pool.handleTaskComplete(agent, &payload)

		// 转发 task_complete 给所有 wechat-agent
		b.forwardToWechatAgents(msg)

	case MsgFileReadResp, MsgTreeReadResp, MsgProjectCreateResp:
		var base struct {
			RequestID string `json:"request_id"`
		}
		json.Unmarshal(msg.Payload, &base)
		b.pool.pendMu.Lock()
		if ch, ok := b.pool.pending[base.RequestID]; ok {
			ch <- msg.Payload
			delete(b.pool.pending, base.RequestID)
		}
		b.pool.pendMu.Unlock()

	case uap.MsgNotify:
		b.handleNotify(msg)

	case uap.MsgToolCall:
		go b.handleToolCall(msg) // 异步处理，避免阻塞消息循环

	default:
		log.WarnF(log.ModuleAgent, "CodeGen gateway: unhandled message type=%s from=%s", msg.Type, msg.From)
	}
}

// handleRegister 处理 codegen/deploy agent 的注册消息
func (b *GatewayBridge) handleRegister(msg *uap.Message) {
	var payload RegisterPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid register payload: %v", err)
		return
	}

	// 验证 token
	if agentToken != "" && payload.AuthToken != agentToken {
		b.client.SendTo(msg.From, MsgRegisterAck, RegisterAckPayload{
			Success: false, Error: "invalid auth token",
		})
		return
	}

	// 检查同名 agent
	if existing := b.pool.findOnlineAgentByName(payload.Name); existing != nil {
		b.client.SendTo(msg.From, MsgRegisterAck, RegisterAckPayload{
			Success: false,
			Error:   fmt.Sprintf("agent '%s' already connected (id=%s), reject duplicate", payload.Name, existing.ID),
		})
		log.WarnF(log.ModuleAgent, "CodeGen gateway: reject duplicate agent name=%s, existing id=%s, new id=%s",
			payload.Name, existing.ID, payload.AgentID)
		return
	}

	// 使用 From 作为 agent ID（gateway 填充的 UAP agent ID）
	agentID := msg.From
	if agentID == "" {
		agentID = payload.AgentID
	}

	agent := &RemoteAgent{
		ID:   agentID,
		Name: payload.Name,
		Sender: &GatewaySender{
			client:    b.client,
			toAgentID: agentID,
		},
		Conn:             nil, // gateway 模式无直连
		Workspaces:       payload.Workspaces,
		Projects:         payload.Projects,
		Models:           payload.Models,
		ClaudeCodeModels: payload.ClaudeCodeModels,
		OpenCodeModels:   payload.OpenCodeModels,
		Tools:            payload.Tools,
		MaxConcurrent:    payload.MaxConcurrent,
		ActiveSessions:   make(map[string]bool),
		LastHeartbeat:    time.Now(),
		Status:           "online",
	}
	b.pool.addAgent(agent)

	b.client.SendTo(agentID, MsgRegisterAck, RegisterAckPayload{Success: true})
	log.MessageF(log.ModuleAgent, "CodeGen gateway: agent registered via gateway: %s (%s), projects=%v",
		agentID, agent.Name, agent.Projects)
}

// handleHeartbeat 处理心跳
func (b *GatewayBridge) handleHeartbeat(msg *uap.Message) {
	var payload HeartbeatPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return
	}

	agent := b.getAgent(msg.From)
	if agent == nil {
		// agent 不在 pool 中（可能 go_blog 重启过），通知它重新注册
		log.WarnF(log.ModuleAgent, "CodeGen gateway: heartbeat from unknown agent %s, asking to re-register", msg.From)
		b.client.SendTo(msg.From, MsgRegisterAck, RegisterAckPayload{
			Success: false,
			Error:   "not_registered",
		})
		return
	}

	agent.mu.Lock()
	agent.LastHeartbeat = time.Now()
	if len(payload.Projects) > 0 {
		agent.Projects = payload.Projects
	}
	if len(payload.Models) > 0 {
		agent.Models = payload.Models
	}
	if len(payload.ClaudeCodeModels) > 0 {
		agent.ClaudeCodeModels = payload.ClaudeCodeModels
	}
	if len(payload.OpenCodeModels) > 0 {
		agent.OpenCodeModels = payload.OpenCodeModels
	}
	if len(payload.Tools) > 0 {
		agent.Tools = payload.Tools
	}
	agent.mu.Unlock()
	b.client.SendTo(msg.From, MsgHeartbeatAck, struct{}{})
}

// handleNotify 处理通知消息（来自 wechat-agent）
func (b *GatewayBridge) handleNotify(msg *uap.Message) {
	var payload uap.NotifyPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid notify payload: %v", err)
		return
	}

	if payload.Channel != "wechat" {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: unsupported notify channel: %s", payload.Channel)
		return
	}

	b.mu.Lock()
	handler := b.wechatHandler
	b.mu.Unlock()

	if handler == nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: wechat handler not set, dropping message from %s", payload.To)
		return
	}

	// 调用 handleWechatCommand，获取结果
	result := handler(payload.To, payload.Content)

	// 回发结果给 wechat-agent
	b.client.SendTo(msg.From, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      payload.To,
		Content: result,
	})
}

// forwardToWechatAgents 转发消息给所有连接的 wechat-agent（通过 gateway 路由）
func (b *GatewayBridge) forwardToWechatAgents(msg *uap.Message) {
	// 使用约定的 wechat-agent ID 前缀
	// wechat-agent 注册时 ID 为 "wechat-<name>"
	// 这里直接广播给已知的 wechat-agent
	// 简单实现：发给 "wechat-wechat-agent"（默认 wechat-agent ID）
	wechatAgentID := "wechat-wechat-agent"
	b.client.Send(&uap.Message{
		Type:    msg.Type,
		ID:      uap.NewMsgID(),
		From:    "go_blog",
		To:      wechatAgentID,
		Payload: msg.Payload,
		Ts:      time.Now().UnixMilli(),
	})
}

// getAgent 从 pool 中获取 agent
func (b *GatewayBridge) getAgent(agentID string) *RemoteAgent {
	b.pool.mu.RLock()
	agent := b.pool.agents[agentID]
	b.pool.mu.RUnlock()
	return agent
}

// handleToolCall 处理跨 agent 工具调用请求
func (b *GatewayBridge) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid tool_call payload: %v", err)
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid tool_call payload",
		})
		return
	}

	// 查映射表：UAP tool_name → MCP callback name
	mcpName, ok := b.toolMapping[payload.ToolName]
	if !ok {
		// 兼容：直接用原名尝试（可能调用者已经使用 MCP 回调名）
		mcpName = payload.ToolName
	}

	// 解析 arguments
	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid tool_call arguments: %v", err)
			b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   false,
				Error:     "invalid arguments: " + err.Error(),
			})
			return
		}
	} else {
		args = make(map[string]interface{})
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: tool_call from=%s tool=%s (mcp=%s)", msg.From, payload.ToolName, mcpName)

	// 调用 MCP 内部工具（通过注入的函数）
	if MCPCallInnerTools == nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: MCPCallInnerTools not initialized")
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "MCP bridge not initialized",
		})
		return
	}
	result := MCPCallInnerTools(mcpName, args)

	// 发送结果
	b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    result,
	})
}

// buildToolDefs 构建 UAP 工具定义列表和映射表
// 从 MCP 已注册的 LLMTool 定义中提取，转换为 UAP ToolDef 格式
func buildToolDefs() ([]uap.ToolDef, map[string]string) {
	toolMapping := make(map[string]string) // UAP name → MCP callback name

	// 从注入的 MCP 工具定义获取完整参数信息
	mcpToolMap := make(map[string]MCPToolInfo)
	if MCPGetToolInfos != nil {
		for _, t := range MCPGetToolInfos() {
			mcpToolMap[t.Name] = t
		}
	}

	// 24 个核心工具的映射定义
	entries := []struct {
		uapName string
		mcpName string
		desc    string // 覆盖描述（空则使用 MCP 原始描述）
	}{
		// Blog
		{"blog.GetBlogs", "RawAllBlogName", "获取博客列表"},
		{"blog.GetBlog", "RawGetBlogData", "获取博客内容"},
		{"blog.CreateBlog", "RawCreateBlog", "创建博客"},
		{"blog.SearchBlog", "RawSearchBlogContent", "搜索博客内容"},
		// TodoList
		{"todolist.GetTodos", "RawGetTodosByDate", "获取指定日期的待办列表"},
		{"todolist.CreateTodo", "RawAddTodo", "创建待办事项"},
		{"todolist.ToggleTodo", "RawToggleTodo", "切换待办完成状态"},
		{"todolist.DeleteTodo", "RawDeleteTodo", "删除待办事项"},
		// Exercise
		{"exercise.GetRecords", "RawGetExerciseByDate", "获取指定日期运动记录"},
		{"exercise.AddRecord", "RawAddExercise", "添加运动记录"},
		{"exercise.GetStats", "RawGetExerciseStats", "获取运动统计数据"},
		// Reading
		{"reading.GetBooks", "RawGetAllBooks", "获取阅读书籍列表"},
		{"reading.UpdateProgress", "RawUpdateReadingProgress", "更新阅读进度"},
		// Reminder
		{"reminder.Create", "CreateReminder", "创建定时提醒"},
		{"reminder.List", "ListReminders", "列出所有提醒"},
		{"reminder.Delete", "DeleteReminder", "删除提醒"},
		// Notification
		{"notification.Send", "SendNotification", "发送通知"},
		// Report
		{"report.Generate", "GenerateReport", "生成报告(日报/周报/月报)"},
		// Model
		{"model.Switch", "SwitchModel", "切换LLM模型"},
		{"model.GetCurrent", "GetCurrentModel", "获取当前模型信息"},
		// CodeGen
		{"codegen.ListProjects", "CodegenListProjects", "列出编码项目"},
		{"codegen.StartSession", "CodegenStartSession", "启动编码会话"},
		{"codegen.GetStatus", "CodegenGetStatus", "查看编码状态"},
		{"codegen.StopSession", "CodegenStopSession", "停止编码会话"},
	}

	var toolDefs []uap.ToolDef

	for _, e := range entries {
		toolMapping[e.uapName] = e.mcpName

		// 从 MCP 工具定义获取参数 schema
		desc := e.desc
		var params interface{}
		if mcpTool, ok := mcpToolMap[e.mcpName]; ok {
			if desc == "" {
				desc = mcpTool.Description
			}
			params = mcpTool.Parameters
		} else {
			// MCP 中没有该工具的定义（如 codegen 工具由 agent 包动态注册）
			// 使用空参数 schema
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		// 序列化参数为 json.RawMessage
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: failed to marshal params for %s: %v", e.uapName, err)
			paramsJSON = []byte(`{"type":"object","properties":{}}`)
		}

		toolDefs = append(toolDefs, uap.ToolDef{
			Name:        e.uapName,
			Description: desc,
			Parameters:  json.RawMessage(paramsJSON),
		})
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: built %d tool definitions for UAP registration", len(toolDefs))
	return toolDefs, toolMapping
}
