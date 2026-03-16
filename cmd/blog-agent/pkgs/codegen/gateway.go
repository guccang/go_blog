package codegen

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"net/http"
	"strings"
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

// TaskEvent assistant 任务事件（投递给监听者）
type TaskEvent struct {
	Event    string // "chunk" | "tool_info" | "complete" | "error"
	Text     string
	TaskID   string
	Complete bool   // 是否为最终事件
	Error    string // 仅 complete 时有效
}

// GatewayBridge go_blog 的 gateway 适配层
type GatewayBridge struct {
	client     *uap.Client
	pool       *AgentPool
	gatewayHTTP string // gateway HTTP 地址（如 http://127.0.0.1:9000）

	// wechat notify 处理器
	wechatHandler func(wechatUser, message string) string

	// UAP tool_name → MCP callback name 映射
	toolMapping map[string]string

	// assistant 任务事件通道（taskID → event channel）
	taskEventChannels map[string]chan TaskEvent
	taskEventMu       sync.Mutex

	mu sync.Mutex
}

// 全局 gateway bridge 实例
var gatewayBridge *GatewayBridge

// InitGatewayBridge 初始化 go_blog 到 gateway 的连接
func InitGatewayBridge(gatewayURL, authToken string) {
	// 统一 token：gateway_token 同时作为 agent 认证 token
	if authToken != "" {
		agentToken = authToken
	}

	// 构建工具定义和映射表
	toolDefs, toolMapping := buildToolDefs()

	client := uap.NewClient(gatewayURL, "go_blog", "go_blog", "Go Blog Server")
	client.AuthToken = authToken
	client.Description = "博客CRUD、待办清单、运动记录、阅读管理、年度计划、星座占卜、实用工具、游戏"
	client.Capacity = 100
	client.Tools = toolDefs
	client.Meta = map[string]any{
		"role": "backend",
	}

	// 从 WebSocket URL 推导 HTTP URL（ws://host:port/ws/uap → http://host:port）
	httpURL := gatewayURL
	httpURL = strings.Replace(httpURL, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)
	if idx := strings.Index(httpURL, "/ws/"); idx > 0 {
		httpURL = httpURL[:idx]
	}

	bridge := &GatewayBridge{
		client:            client,
		pool:              agentPool,
		gatewayHTTP:       httpURL,
		toolMapping:       toolMapping,
		taskEventChannels: make(map[string]chan TaskEvent),
	}

	client.OnMessage = bridge.handleMessage

	gatewayBridge = bridge

	// 初始化 WeChat Bridge（通过 gateway 路由发送微信消息）
	// 使 cg start 等命令可以通过 StartSessionForWeChat 启动编码会话并推送进度
	InitWeChatBridge(func(toUser, content string) error {
		return bridge.sendWechatViaGateway(toUser, content)
	})

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
		// 兼容 codegen 协议（SessionID）和 UAP 协议（TaskID）
		var raw struct {
			SessionID string `json:"session_id"`
			TaskID    string `json:"task_id"`
		}
		if err := json.Unmarshal(msg.Payload, &raw); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_accepted payload: %v", err)
			return
		}
		if raw.SessionID != "" {
			// codegen agent
			agent := b.getAgent(msg.From)
			if agent != nil {
				agent.mu.Lock()
				agent.ActiveSessions[raw.SessionID] = true
				agent.mu.Unlock()
			}
			log.MessageF(log.ModuleAgent, "CodeGen: gateway agent %s accepted task %s", msg.From, raw.SessionID)
		} else if raw.TaskID != "" {
			// llm-mcp-agent assistant 任务
			log.MessageF(log.ModuleAgent, "CodeGen gateway: llm-mcp-agent accepted task %s", raw.TaskID)
		}

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
		// 兼容 codegen 协议（SessionID）和 UAP 协议（TaskID）
		var raw struct {
			SessionID string `json:"session_id"`
			TaskID    string `json:"task_id"`
			Status    string `json:"status"`
			Error     string `json:"error"`
			Result    string `json:"result"`
		}
		if err := json.Unmarshal(msg.Payload, &raw); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid task_complete payload: %v", err)
			return
		}
		if raw.TaskID != "" {
			// llm-mcp-agent assistant 任务完成 → 投递到 taskEventChannels
			b.taskEventMu.Lock()
			ch, ok := b.taskEventChannels[raw.TaskID]
			b.taskEventMu.Unlock()
			if ok {
				select {
				case ch <- TaskEvent{
					Event:    "complete",
					TaskID:   raw.TaskID,
					Text:     raw.Result,
					Complete: true,
					Error:    raw.Error,
				}:
				default:
				}
			}
		}
		if raw.SessionID != "" {
			// codegen agent 任务完成
			var payload TaskCompletePayload
			json.Unmarshal(msg.Payload, &payload)
			agent := b.getAgent(msg.From)
			b.pool.handleTaskComplete(agent, &payload)
			b.forwardToWechatAgents(msg)
		}

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
		go b.handleNotify(msg) // 异步处理，避免 LLM 处理阻塞消息循环

	case uap.MsgToolCall:
		go b.handleToolCall(msg) // 异步处理，避免阻塞消息循环

	case uap.MsgError:
		var payload uap.ErrorPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid error payload: %v", err)
			return
		}
		log.WarnF(log.ModuleAgent, "CodeGen gateway: error from %s: [%s] %s", msg.From, payload.Code, payload.Message)

		// agent_offline: 释放对应的 pending 请求
		if payload.Code == "agent_offline" && msg.ID != "" {
			b.pool.pendMu.Lock()
			if ch, ok := b.pool.pending[msg.ID]; ok {
				close(ch)
				delete(b.pool.pending, msg.ID)
			}
			b.pool.pendMu.Unlock()
		}

	case uap.MsgTaskEvent:
		// llm-mcp-agent 发来的 assistant 任务进度事件
		var taskEventPayload uap.TaskEventPayload
		if err := json.Unmarshal(msg.Payload, &taskEventPayload); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid uap task_event payload: %v", err)
			return
		}
		// 解析内部事件
		var evt struct {
			Event string `json:"event"`
			Text  string `json:"text"`
		}
		if err := json.Unmarshal(taskEventPayload.Event, &evt); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen gateway: invalid assistant event data: %v", err)
			return
		}
		// 投递到 pending channel
		b.taskEventMu.Lock()
		ch, ok := b.taskEventChannels[taskEventPayload.TaskID]
		b.taskEventMu.Unlock()
		if ok {
			select {
			case ch <- TaskEvent{Event: evt.Event, Text: evt.Text, TaskID: taskEventPayload.TaskID}:
			default:
				// channel 满了，丢弃（不阻塞消息循环）
				log.WarnF(log.ModuleAgent, "CodeGen gateway: task event channel full, dropping event for task %s", taskEventPayload.TaskID)
			}
		}

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
		DeployTargets:    payload.DeployTargets,
		HostPlatform:     payload.HostPlatform,
		Pipelines:        payload.Pipelines,
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

// handleNotify 处理通知消息（来自 gateway 或 wechat-agent）
func (b *GatewayBridge) handleNotify(msg *uap.Message) {
	// 先尝试解析为通用事件（gateway 广播的 agent_offline 等）
	var event struct {
		Event     string `json:"event"`
		AgentID   string `json:"agent_id"`
		AgentType string `json:"agent_type"`
		AgentName string `json:"agent_name"`
	}
	if err := json.Unmarshal(msg.Payload, &event); err == nil && event.Event == "agent_offline" {
		log.MessageF(log.ModuleAgent, "CodeGen gateway: agent offline notification: %s (%s)", event.AgentID, event.AgentName)
		b.pool.removeAgent(event.AgentID)
		return
	}

	// 解析为 NotifyPayload（wechat 等）
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

// sendWechatViaGateway 通过 gateway 路由发送微信消息给用户
// 发送 notify 消息到 wechat-agent，由其调用企业微信 API
func (b *GatewayBridge) sendWechatViaGateway(toUser, content string) error {
	wechatAgentID := "wechat-wechat-agent"
	return b.client.SendTo(wechatAgentID, uap.MsgNotify, uap.NotifyPayload{
		Channel: "wechat",
		To:      toUser,
		Content: content,
	})
}

// forwardToWechatAgents 转发消息给所有连接的 wechat-agent（通过 gateway 路由）
// 尝试从 payload 中提取 session_id，注入关联的 account 字段
func (b *GatewayBridge) forwardToWechatAgents(msg *uap.Message) {
	wechatAgentID := "wechat-wechat-agent"
	payload := enrichPayloadWithAccount(msg.Payload)
	b.client.Send(&uap.Message{
		Type:    msg.Type,
		ID:      uap.NewMsgID(),
		From:    "go_blog",
		To:      wechatAgentID,
		Payload: payload,
		Ts:      time.Now().UnixMilli(),
	})
}

// enrichPayloadWithAccount 尝试从 payload 提取 session_id，注入关联的 account
func enrichPayloadWithAccount(raw json.RawMessage) json.RawMessage {
	var base struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(raw, &base) != nil || base.SessionID == "" {
		return raw
	}
	account := GetSessionUser(base.SessionID)
	if account == "" {
		return raw
	}
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	m["account"] = account
	enriched, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return enriched
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

	log.MessageF(log.ModuleAgent, "CodeGen gateway: tool_call from=%s tool=%s (mcp=%s) msgID=%s", msg.From, payload.ToolName, mcpName, msg.ID)

	// 调用 MCP 内部工具（通过注入的函数，带超时保护）
	if MCPCallInnerTools == nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: MCPCallInnerTools not initialized")
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "MCP bridge not initialized",
		})
		return
	}

	// 超时保护：MCP 工具调用最多 90 秒
	type mcpResult struct {
		result string
	}
	resultCh := make(chan mcpResult, 1)
	go func() {
		r := MCPCallInnerTools(mcpName, args)
		resultCh <- mcpResult{result: r}
	}()

	var result string
	select {
	case r := <-resultCh:
		result = r.result
	case <-time.After(90 * time.Second):
		log.WarnF(log.ModuleAgent, "CodeGen gateway: tool_call timeout (90s) tool=%s msgID=%s", payload.ToolName, msg.ID)
		b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("tool %s timeout after 90s", payload.ToolName),
		})
		return
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: tool_result to=%s tool=%s msgID=%s resultLen=%d",
		msg.From, payload.ToolName, msg.ID, len(result))

	// 发送结果
	if err := b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    result,
	}); err != nil {
		log.WarnF(log.ModuleAgent, "CodeGen gateway: send tool_result failed to=%s msgID=%s: %v", msg.From, msg.ID, err)
	}
}

// buildToolDefs 构建 UAP 工具定义列表和映射表
// 自动注册所有 MCP 工具到 gateway，使用 MCP 回调名作为 UAP 工具名
func buildToolDefs() ([]uap.ToolDef, map[string]string) {
	toolMapping := make(map[string]string) // UAP name → MCP callback name（identity mapping）

	// 从注入的 MCP 工具定义获取全部工具
	var mcpTools []MCPToolInfo
	if MCPGetToolInfos != nil {
		mcpTools = MCPGetToolInfos()
	}

	var toolDefs []uap.ToolDef

	for _, t := range mcpTools {
		toolMapping[t.Name] = t.Name

		desc := t.Description
		if desc == "" {
			desc = t.Name
		}
		params := t.Parameters
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		paramsJSON, err := json.Marshal(params)
		if err != nil {
			paramsJSON = []byte(`{"type":"object","properties":{}}`)
		}

		toolDefs = append(toolDefs, uap.ToolDef{
			Name:        t.Name,
			Description: desc,
			Parameters:  json.RawMessage(paramsJSON),
		})
	}

	log.MessageF(log.ModuleAgent, "CodeGen gateway: registered %d MCP tools for UAP", len(toolDefs))
	return toolDefs, toolMapping
}

// ========================= Assistant 任务桥接 =========================

// SendTaskToLLMAgent 发送 MsgTaskAssign 给 llm-mcp-agent
func SendTaskToLLMAgent(taskID string, payload interface{}) error {
	if gatewayBridge == nil || gatewayBridge.client == nil {
		return fmt.Errorf("gateway bridge not initialized")
	}

	// 动态查找 llm_mcp 类型的 agent ID
	agentID := findLLMAgentID()
	if agentID == "" {
		return fmt.Errorf("llm-mcp-agent not found")
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %v", err)
	}

	return gatewayBridge.client.SendTo(agentID, uap.MsgTaskAssign, uap.TaskAssignPayload{
		TaskID:  taskID,
		Payload: json.RawMessage(payloadJSON),
	})
}

// RegisterTaskListener 注册任务事件监听器，返回事件 channel
func RegisterTaskListener(taskID string) chan TaskEvent {
	if gatewayBridge == nil {
		return nil
	}
	ch := make(chan TaskEvent, 1024) // 足够大的 buffer 避免丢事件
	gatewayBridge.taskEventMu.Lock()
	gatewayBridge.taskEventChannels[taskID] = ch
	gatewayBridge.taskEventMu.Unlock()
	return ch
}

// UnregisterTaskListener 注销任务事件监听器
func UnregisterTaskListener(taskID string) {
	if gatewayBridge == nil {
		return
	}
	gatewayBridge.taskEventMu.Lock()
	delete(gatewayBridge.taskEventChannels, taskID)
	gatewayBridge.taskEventMu.Unlock()
}

// SendWechatNotify 通过 gateway → wechat-agent 推送微信通知
// toUser 为微信用户ID，"@all" 表示发送给所有人
func SendWechatNotify(toUser, content string) error {
	if gatewayBridge == nil || gatewayBridge.client == nil {
		return fmt.Errorf("gateway bridge not initialized")
	}
	if !gatewayBridge.client.IsConnected() {
		return fmt.Errorf("gateway not connected")
	}
	return gatewayBridge.sendWechatViaGateway(toUser, content)
}

// IsGatewayConnected 检查 gateway 是否已连接
func IsGatewayConnected() bool {
	return gatewayBridge != nil && gatewayBridge.client != nil && gatewayBridge.client.IsConnected()
}

// IsLLMAgentOnline 检查 llm-mcp-agent 是否在线
func IsLLMAgentOnline() bool {
	return findLLMAgentID() != ""
}

// findLLMAgentID 通过 gateway HTTP API 查找 llm_mcp 类型的 agent ID
func findLLMAgentID() string {
	if gatewayBridge == nil || gatewayBridge.gatewayHTTP == "" {
		return ""
	}
	if !gatewayBridge.client.IsConnected() {
		return ""
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(gatewayBridge.gatewayHTTP + "/api/gateway/agents")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Agents []struct {
			AgentID   string `json:"agent_id"`
			AgentType string `json:"agent_type"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if !result.Success {
		return ""
	}

	for _, a := range result.Agents {
		if a.AgentType == "llm_mcp" {
			return a.AgentID
		}
	}
	return ""
}

// ========================= 同步 LLM 请求桥接 =========================

// LLMProgressCallback 同步 LLM 请求的进度回调（nil 表示不需要进度通知）
type LLMProgressCallback func(event, text string)

// SendSyncLLMTask 发送同步 LLM 请求给 llm-mcp-agent，等待结果返回（不含进度回调）
// messages 可以是任意可 JSON 序列化的消息列表
func SendSyncLLMTask(messages interface{}, account string, selectedTools []string, noTools bool, timeout time.Duration) (string, error) {
	return SendSyncLLMTaskWithProgress(messages, account, selectedTools, noTools, timeout, nil)
}

// SendSyncLLMTaskWithProgress 发送同步 LLM 请求给 llm-mcp-agent，等待结果返回（支持进度回调）
func SendSyncLLMTaskWithProgress(messages interface{}, account string, selectedTools []string, noTools bool, timeout time.Duration, progressCb LLMProgressCallback) (string, error) {
	if gatewayBridge == nil || gatewayBridge.client == nil {
		return "", fmt.Errorf("gateway bridge not initialized")
	}
	if !gatewayBridge.client.IsConnected() {
		return "", fmt.Errorf("gateway not connected")
	}

	// 生成 taskID
	taskID := fmt.Sprintf("llm_%d", time.Now().UnixNano())

	// 注册事件监听
	eventCh := RegisterTaskListener(taskID)
	if eventCh == nil {
		return "", fmt.Errorf("failed to register task listener")
	}
	defer UnregisterTaskListener(taskID)

	// 构建 payload
	taskPayload := map[string]interface{}{
		"task_type":      "llm_request",
		"messages":       messages,
		"account":        account,
		"selected_tools": selectedTools,
		"no_tools":       noTools,
	}

	// 发送 MsgTaskAssign
	if err := SendTaskToLLMAgent(taskID, taskPayload); err != nil {
		return "", fmt.Errorf("send task failed: %v", err)
	}

	log.MessageF(log.ModuleAgent, "SendSyncLLMTask: task=%s account=%s noTools=%v timeout=%v", taskID, account, noTools, timeout)

	// 等待完成事件
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				return "", fmt.Errorf("event channel closed")
			}
			if evt.Complete {
				if evt.Error != "" {
					return "", fmt.Errorf("llm task failed: %s", evt.Error)
				}
				return evt.Text, nil
			}
			// 非 complete 事件 → 转发给 callback
			if progressCb != nil {
				progressCb(evt.Event, evt.Text)
			}
		case <-timer.C:
			return "", fmt.Errorf("llm task timeout after %v", timeout)
		}
	}
}
