package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"agentbase"
	"uap"
)

// Connection UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase // 组合基类

	cfg          *Config
	executor     *Executor
	toolCatalog  *agentbase.ToolCatalog
	fileToolKit  *agentbase.FileToolKit
	remoteCaller *agentbase.RemoteCaller
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	// FileToolKit 用于 ExecEnvBash（不需要 project resolver）
	ftk := agentbase.NewFileToolKit("Exec", nil)

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "execute_code",
		AgentName:   cfg.AgentName,
		Description: "Python代码执行、MCP工具批量调用",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildToolDefs(ftk),
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		toolCatalog: agentbase.NewToolCatalog(cfg.GatewayHTTP),
		fileToolKit: ftk,
	}

	// 创建 RemoteCaller
	c.remoteCaller = agentbase.NewRemoteCaller(c.AgentBase, c.toolCatalog)

	// 创建 executor
	c.executor = NewExecutor(cfg)
	// 注入工具调用桥接函数
	c.executor.callTool = c.remoteCallToolBridge

	// 注册消息处理器
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgToolResult, c.handleToolResult)
	c.RegisterHandler(uap.MsgError, c.handleError)

	return c
}

// ========================= 工具注册 =========================

// buildToolDefs 构建 execute-code-agent 注册的 UAP 工具
func buildToolDefs(ftk *agentbase.FileToolKit) []uap.ToolDef {
	tools := []uap.ToolDef{
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
	// 追加 ExecEnvBash（只取这一个工具）
	for _, td := range ftk.ToolDefs() {
		if strings.HasSuffix(td.Name, "ExecEnvBash") {
			tools = append(tools, td)
		}
	}
	return tools
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
	// 尝试分发到 RemoteCaller
	if c.remoteCaller.DispatchToolResult(&payload) {
		return
	}
	log.Printf("[Connection] tool_result requestID=%s has no pending channel (from=%s success=%v)",
		payload.RequestID, msg.From, payload.Success)
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

	// 尝试分发到 RemoteCaller，让工具调用快速失败而非等待超时
	c.remoteCaller.DispatchError(msg.ID, payload.Message)
}

// handleToolCall 处理 ExecuteCode / ExecEnvBash 工具调用
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

	// 尝试 FileToolKit 处理（ExecEnvBash）
	var ftkArgs map[string]interface{}
	if err := json.Unmarshal(payload.Arguments, &ftkArgs); err == nil {
		if result, handled := c.fileToolKit.HandleTool(payload.ToolName, ftkArgs); handled {
			log.Printf("[ExecEnvBash] from=%s command=%v", msg.From, ftkArgs["command"])
			// 解析 result JSON 判断 success
			var res struct {
				Success bool `json:"success"`
			}
			json.Unmarshal([]byte(result), &res)
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   res.Success,
				Result:    result,
			})
			return
		}
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
	if !execResult.Success {
		log.Printf("[ExecuteCode] error_type=%s stderr=%s", execResult.ErrorType, truncate(execResult.Stderr, 500))
	}

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

// remoteCallToolBridge 桥接 executor 的 callTool 到 RemoteCaller
func (c *Connection) remoteCallToolBridge(toolName string, args json.RawMessage) (string, string, error) {
	return c.remoteCaller.CallToolWithRetry(toolName, args, 120*time.Second)
}
