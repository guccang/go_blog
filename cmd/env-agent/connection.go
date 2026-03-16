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

// Connection UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg         *Config
	toolCatalog *agentbase.ToolCatalog
	orch        *Orchestrator

	// 请求-响应关联（pending channel 模式）— 用于 tool_call
	pending map[string]chan *uap.ToolResultPayload
	pendMu  sync.Mutex

	// task_assign 响应 — 用于委托 llm-mcp-agent
	taskResults map[string]chan *uap.TaskCompletePayload
	taskMu      sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "env_agent",
		AgentName:   cfg.AgentName,
		Description: "环境编排中心：远程检测/安装软件环境，支持预置脚本和 LLM 生成脚本",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildEnvToolDefs(),
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		toolCatalog: agentbase.NewToolCatalog(cfg.GatewayHTTP),
		pending:     make(map[string]chan *uap.ToolResultPayload),
		taskResults: make(map[string]chan *uap.TaskCompletePayload),
	}

	c.orch = NewOrchestrator(c)

	// 注册消息处理器
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgToolResult, c.handleToolResult)
	c.RegisterHandler(uap.MsgTaskComplete, c.handleTaskComplete)
	c.RegisterHandler(uap.MsgError, c.handleError)

	return c
}

// ========================= 工具注册 =========================

func buildEnvToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "EnvCheck",
			Description: "远程检测目标 agent 上指定软件的安装状态和版本",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_agent": map[string]interface{}{"type": "string", "description": "目标 agent 名称（如 execute-code-agent）"},
					"software":     map[string]interface{}{"type": "string", "description": "软件名称（如 python, go, redis）"},
					"min_version":  map[string]interface{}{"type": "string", "description": "最低版本要求（可选，如 3.0）"},
				},
				"required": []string{"target_agent", "software"},
			}),
		},
		{
			Name:        "EnvInstall",
			Description: "远程安装指定软件到目标 agent（先检测，未安装则安装）",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_agent": map[string]interface{}{"type": "string", "description": "目标 agent 名称"},
					"software":     map[string]interface{}{"type": "string", "description": "软件名称"},
					"min_version":  map[string]interface{}{"type": "string", "description": "最低版本要求（可选）"},
				},
				"required": []string{"target_agent", "software"},
			}),
		},
		{
			Name:        "EnvCheckAll",
			Description: "远程检测目标 agent 上所有常用软件的安装状态",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_agent": map[string]interface{}{"type": "string", "description": "目标 agent 名称"},
				},
				"required": []string{"target_agent"},
			}),
		},
		{
			Name:        "EnvSetup",
			Description: "远程批量检测+安装软件环境（支持预置脚本和 LLM 生成脚本）",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_agent": map[string]interface{}{"type": "string", "description": "目标 agent 名称"},
					"requirements": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"software":    map[string]interface{}{"type": "string"},
								"min_version": map[string]interface{}{"type": "string"},
							},
							"required": []string{"software"},
						},
						"description": "软件需求列表",
					},
				},
				"required": []string{"target_agent", "requirements"},
			}),
		},
	}
}

// ========================= 消息处理 =========================

func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	go c.handleToolCall(msg)
}

func (c *Connection) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[EnvAgent] invalid tool_call payload: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid tool_call payload",
		})
		return
	}

	var args map[string]interface{}
	if err := json.Unmarshal(payload.Arguments, &args); err != nil {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid arguments: " + err.Error(),
		})
		return
	}

	log.Printf("[EnvAgent] tool_call from=%s tool=%s", msg.From, payload.ToolName)

	var resultJSON string
	var success bool

	switch payload.ToolName {
	case "EnvCheck":
		resultJSON, success = c.orch.handleEnvCheck(args)
	case "EnvInstall":
		resultJSON, success = c.orch.handleEnvInstall(args)
	case "EnvCheckAll":
		resultJSON, success = c.orch.handleEnvCheckAll(args)
	case "EnvSetup":
		resultJSON, success = c.orch.handleEnvSetup(args)
	default:
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
		})
		return
	}

	resp := uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   success,
		Result:    resultJSON,
	}
	if !success {
		// 从 result 中提取 error
		var res struct {
			Error string `json:"error"`
		}
		json.Unmarshal([]byte(resultJSON), &res)
		resp.Error = res.Error
	}
	c.Client.SendTo(msg.From, uap.MsgToolResult, resp)
}

// handleToolResult 处理远程工具调用结果
func (c *Connection) handleToolResult(msg *uap.Message) {
	var payload uap.ToolResultPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[EnvAgent] invalid tool_result payload: %v", err)
		return
	}
	c.pendMu.Lock()
	ch, ok := c.pending[payload.RequestID]
	c.pendMu.Unlock()
	if ok {
		ch <- &payload
	} else {
		log.Printf("[EnvAgent] tool_result requestID=%s has no pending channel", payload.RequestID)
	}
}

// handleTaskComplete 处理 task_complete 消息（llm-mcp-agent 返回）
func (c *Connection) handleTaskComplete(msg *uap.Message) {
	var payload uap.TaskCompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[EnvAgent] invalid task_complete payload: %v", err)
		return
	}
	log.Printf("[EnvAgent] task_complete taskID=%s status=%s", payload.TaskID, payload.Status)

	c.taskMu.Lock()
	ch, ok := c.taskResults[payload.TaskID]
	c.taskMu.Unlock()
	if ok {
		ch <- &payload
	} else {
		log.Printf("[EnvAgent] task_complete taskID=%s has no pending channel", payload.TaskID)
	}
}

// handleError 处理 gateway 错误消息
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[EnvAgent] invalid error payload: %v", err)
		return
	}
	log.Printf("[EnvAgent] error from=%s code=%s msg=%s", msg.From, payload.Code, payload.Message)

	// 释放 pending channel
	c.pendMu.Lock()
	ch, ok := c.pending[msg.ID]
	c.pendMu.Unlock()
	if ok {
		ch <- &uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     payload.Message,
		}
	}
}

// ========================= 远程调用 =========================

// callRemoteTool 调用远程 agent 的工具（通过 tool_call 消息）
func (c *Connection) callRemoteTool(toolName string, args map[string]interface{}, timeout time.Duration) (string, error) {
	// 从 toolCatalog 查找目标 agent
	agentID, ok := c.toolCatalog.GetAgentID(toolName)
	if !ok {
		return "", fmt.Errorf("tool %s not found in catalog", toolName)
	}

	return c.callRemoteToolDirect(agentID, toolName, args, timeout)
}

// callRemoteToolDirect 直接向指定 agent 发送 tool_call
func (c *Connection) callRemoteToolDirect(agentID, toolName string, args map[string]interface{}, timeout time.Duration) (string, error) {
	msgID := uap.NewMsgID()
	ch := make(chan *uap.ToolResultPayload, 1)

	c.pendMu.Lock()
	c.pending[msgID] = ch
	c.pendMu.Unlock()

	defer func() {
		c.pendMu.Lock()
		delete(c.pending, msgID)
		c.pendMu.Unlock()
	}()

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal args: %v", err)
	}

	log.Printf("[EnvAgent] tool_call → agent=%s tool=%s", agentID, toolName)

	err = c.Client.Send(&uap.Message{
		Type: uap.MsgToolCall,
		ID:   msgID,
		From: c.AgentID,
		To:   agentID,
		Payload: mustMarshalJSON(uap.ToolCallPayload{
			ToolName:  toolName,
			Arguments: json.RawMessage(argsJSON),
		}),
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("send tool_call: %v", err)
	}

	select {
	case result := <-ch:
		if !result.Success {
			return result.Result, fmt.Errorf("tool error: %s", result.Error)
		}
		return result.Result, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("tool %s timeout after %ds", toolName, int(timeout.Seconds()))
	}
}

// sendTaskAssign 发送 task_assign 到 llm-mcp-agent，等待 task_complete
func (c *Connection) sendTaskAssign(targetAgentID string, payload interface{}, timeout time.Duration) (*uap.TaskCompletePayload, error) {
	taskID := uap.NewMsgID()
	ch := make(chan *uap.TaskCompletePayload, 1)

	c.taskMu.Lock()
	c.taskResults[taskID] = ch
	c.taskMu.Unlock()

	defer func() {
		c.taskMu.Lock()
		delete(c.taskResults, taskID)
		c.taskMu.Unlock()
	}()

	taskPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal task payload: %v", err)
	}

	log.Printf("[EnvAgent] task_assign → agent=%s taskID=%s", targetAgentID, taskID)

	err = c.Client.Send(&uap.Message{
		Type: uap.MsgTaskAssign,
		ID:   uap.NewMsgID(),
		From: c.AgentID,
		To:   targetAgentID,
		Payload: mustMarshalJSON(uap.TaskAssignPayload{
			TaskID:  taskID,
			Payload: json.RawMessage(taskPayload),
		}),
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("send task_assign: %v", err)
	}

	select {
	case result := <-ch:
		return result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("task timeout after %ds", int(timeout.Seconds()))
	}
}

// ========================= 工具目录 =========================

// DiscoverTools 从 gateway 获取工具目录
func (c *Connection) DiscoverTools() error {
	return c.toolCatalog.Discover(c.AgentID)
}

// StartRefreshLoop 后台定时刷新工具目录
func (c *Connection) StartRefreshLoop() {
	c.toolCatalog.StartRefreshLoop(60*time.Second, c.AgentID)
}

// resolveAgentID 根据 agent name 从 toolCatalog 查找 agent ID
// 通过查找该 agent 注册的任意工具来获取 agent_id
func (c *Connection) resolveAgentID(agentName string) (string, bool) {
	all := c.toolCatalog.GetAll()
	for _, agentID := range all {
		if strings.Contains(agentID, agentName) {
			return agentID, true
		}
	}
	return "", false
}

// ========================= 工具函数 =========================

func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
