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
	*agentbase.AgentBase // 组合基类

	cfg         *Config
	executor    *Executor
	toolCatalog *agentbase.ToolCatalog

	// 请求-响应关联（pending channel 模式）
	pending map[string]chan *uap.ToolResultPayload
	pendMu  sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "execute_code",
		AgentName:   cfg.AgentName,
		Description: "Python代码执行、MCP工具批量调用",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildToolDefs(),
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		toolCatalog: agentbase.NewToolCatalog(cfg.GatewayHTTP),
		pending:     make(map[string]chan *uap.ToolResultPayload),
	}

	// 创建 executor
	c.executor = NewExecutor(cfg)
	// 注入工具调用桥接函数
	c.executor.callTool = c.callToolWithRetry

	// 注册消息处理器
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgToolResult, c.handleToolResult)
	c.RegisterHandler(uap.MsgError, c.handleError)

	return c
}

// ========================= 工具注册 =========================

// buildToolDefs 构建 execute-code-agent 注册的 UAP 工具
func buildToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "ExecuteCode",
			Description: "在 Python 沙箱中执行代码。代码可通过 call_tool(name, args) 调用其他 MCP 工具。只有 print() 的输出会返回。用于：多工具编排、数据过滤/转换/聚合、循环批量操作。safe_call_tool(name, args, default) 失败时返回 default 而不抛异常。",
			Parameters: mustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Python 代码（可使用 call_tool/safe_call_tool 调用 MCP 工具）",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "代码用途说明（可选）",
					},
					"tools_hint": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "可能用到的工具名列表（可选，用于预加载提示）",
					},
				},
				"required": []string{"code"},
			}),
		},
	}
}

// ========================= 消息处理 =========================

// handleToolCallMsg 处理 tool_call 消息（包装器）
func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	go c.handleToolCall(msg)
}

// handleToolResult 处理 tool_result 消息
func (c *Connection) handleToolResult(msg *uap.Message) {
	var payload uap.ToolResultPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Connection] invalid tool_result payload: %v", err)
		return
	}
	c.pendMu.Lock()
	ch, ok := c.pending[payload.RequestID]
	c.pendMu.Unlock()
	if ok {
		ch <- &payload
	} else {
		log.Printf("[Connection] tool_result requestID=%s has no pending channel (from=%s success=%v)",
			payload.RequestID, msg.From, payload.Success)
	}
}

// handleError 处理 gateway 错误消息（如 agent_offline）
// gateway 使用原始 msg.ID 作为错误消息的 ID，用于匹配 pending 请求
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Connection] invalid error payload: %v", err)
		return
	}

	log.Printf("[Connection] error from=%s code=%s msg=%s (id=%s)", msg.From, payload.Code, payload.Message, msg.ID)

	// 释放 pending channel，让工具调用快速失败而非等待超时
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

// handleToolCall 处理 ExecuteCode 工具调用
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

	if payload.ToolName != "ExecuteCode" {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
		})
		return
	}

	// 解析参数
	var args struct {
		Code        string   `json:"code"`
		Description string   `json:"description"`
		ToolsHint   []string `json:"tools_hint"`
	}
	if err := json.Unmarshal(payload.Arguments, &args); err != nil {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid arguments: " + err.Error(),
		})
		return
	}

	if args.Code == "" {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "code parameter is required",
		})
		return
	}

	desc := args.Description
	if desc == "" {
		desc = "(no description)"
	}
	log.Printf("[ExecuteCode] from=%s desc=%q code_len=%d", msg.From, desc, len(args.Code))

	// 执行代码
	execResult := c.executor.Execute(args.Code)

	log.Printf("[ExecuteCode] done success=%v exit_code=%d duration=%dms tool_calls=%d",
		execResult.Success, execResult.ExitCode, execResult.DurationMs, len(execResult.ToolCalls))

	// 构建返回值 — 统一返回结构化 JSON（含 tool_calls 调用链）
	stdout := execResult.Stdout
	if stdout == "" && execResult.Success {
		stdout = "(no output)"
	}
	if execResult.Truncated {
		stdout += "\n[输出已截断，原始输出超过限制]"
	}

	execData := map[string]interface{}{
		"stdout":      stdout,
		"duration_ms": execResult.DurationMs,
	}
	if len(execResult.ToolCalls) > 0 {
		execData["tool_calls"] = execResult.ToolCalls
	}
	if !execResult.Success {
		execData["error_type"] = execResult.ErrorType
		execData["stderr"] = truncate(execResult.Stderr, 2000)
	}
	if execResult.Truncated {
		execData["truncated"] = true
	}

	message := "执行完成"
	if !execResult.Success {
		message = "执行失败"
	}
	tr := uap.BuildToolResult(msg.ID, execData, message)

	c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   execResult.Success,
		Result:    tr.Result,
		Error:     conditionalError(execResult),
	})
}

// conditionalError 失败时返回简洁错误信息
func conditionalError(r *ExecutionResult) string {
	if r.Success {
		return ""
	}
	switch r.ErrorType {
	case "syntax":
		return "Python syntax error"
	case "timeout":
		return fmt.Sprintf("execution timeout (%ds)", r.DurationMs/1000)
	case "runtime":
		// 提取最后一行 stderr 作为简洁错误
		trimmed := strings.TrimSpace(r.Stderr)
		if trimmed != "" {
			lines := strings.Split(trimmed, "\n")
			return lines[len(lines)-1]
		}
		// stderr 为空（如命令不存在 exit_code=9009）
		if r.ExitCode == 9009 {
			return fmt.Sprintf("python command not found (exit_code=%d), check python_path config", r.ExitCode)
		}
		return fmt.Sprintf("runtime error (exit_code=%d)", r.ExitCode)
	default:
		return "execution failed"
	}
}

// ========================= 工具目录发现 =========================

// DiscoverTools 从 gateway HTTP API 获取所有在线 agent 的工具目录
func (c *Connection) DiscoverTools() error {
	return c.toolCatalog.Discover(c.AgentID)
}

// StartRefreshLoop 后台定时刷新工具目录
func (c *Connection) StartRefreshLoop() {
	c.toolCatalog.StartRefreshLoop(60*time.Second, c.AgentID)
}

// ========================= 工具调用桥接 =========================

// callToolWithRetry 带瞬态错误重试的工具调用
func (c *Connection) callToolWithRetry(toolName string, args json.RawMessage) (string, string, error) {
	result, agentID, err := c.callToolOnce(toolName, args)
	if err != nil && isTransientError(err) {
		log.Printf("[ExecuteCode] tool %s transient error, retrying: %v", toolName, err)
		time.Sleep(1 * time.Second)
		result, agentID, err = c.callToolOnce(toolName, args)
	}
	return result, agentID, err
}

// callToolOnce 单次工具调用
func (c *Connection) callToolOnce(toolName string, args json.RawMessage) (string, string, error) {
	agentID, ok := c.toolCatalog.GetAgentID(toolName)
	if !ok {
		return "", "", fmt.Errorf("tool %s not found in catalog", toolName)
	}

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

	log.Printf("[ExecuteCode] tool_call → agent=%s tool=%s", agentID, toolName)

	// 发送 tool_call
	err := c.Client.Send(&uap.Message{
		Type: uap.MsgToolCall,
		ID:   msgID,
		From: c.AgentID,
		To:   agentID,
		Payload: mustMarshalJSON(uap.ToolCallPayload{
			ToolName:  toolName,
			Arguments: args,
		}),
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return "", agentID, fmt.Errorf("send tool_call: %v", err)
	}

	// 等待结果
	select {
	case result := <-ch:
		if !result.Success {
			return "", agentID, fmt.Errorf("tool error: %s", result.Error)
		}
		log.Printf("[ExecuteCode] tool_result ← agent=%s tool=%s resultLen=%d", agentID, toolName, len(result.Result))
		return result.Result, agentID, nil
	case <-time.After(120 * time.Second):
		return "", agentID, fmt.Errorf("tool %s timeout after 120s", toolName)
	}
}

// isTransientError 判断是否是瞬态网络错误（值得重试）
func isTransientError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "not connected")
}
