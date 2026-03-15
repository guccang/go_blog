package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"agentbase"
	"uap"
)

// Connection UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg        *Config
	mcpMgr     *MCPManager
	cfgPath    string // 配置文件路径（热加载用）
	logToolKit *agentbase.LogToolKit
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string, mcpMgr *MCPManager, cfgPath string) *Connection {
	logToolKit := agentbase.NewLogToolKit("Mcp", "mcp-agent.log")

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "mcp_bridge",
		AgentName:   cfg.AgentName,
		Description: "外部 MCP Server 桥接代理，将社区 MCP 工具接入 UAP gateway",
		AuthToken:   cfg.AuthToken,
		Capacity:    10,
		Tools:       logToolKit.ToolDefs(), // 启动后由 mcpMgr.BuildUAPTools() 追加
	}

	c := &Connection{
		AgentBase:  agentbase.NewAgentBase(baseCfg),
		cfg:        cfg,
		mcpMgr:     mcpMgr,
		cfgPath:    cfgPath,
		logToolKit: logToolKit,
	}

	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgError, c.handleError)

	return c
}

// handleToolCallMsg 处理工具调用请求
func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Connection] parse tool_call payload failed: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid payload"))
		return
	}

	log.Printf("[Connection] tool_call from=%s tool=%s", msg.From, payload.ToolName)

	// 解析参数
	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[Connection] parse arguments failed: %v", err)
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid arguments"))
			return
		}
	}

	// 先尝试 LogToolKit 处理
	if result, handled := c.logToolKit.HandleTool(payload.ToolName, args); handled {
		log.Printf("[Connection] tool %s handled by logToolKit", payload.ToolName)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolResult(msg.ID, result, ""))
		return
	}

	// 带超时的工具调用
	timeout := time.Duration(c.cfg.ToolCallTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := c.mcpMgr.CallTool(ctx, payload.ToolName, args)
	if err != nil {
		log.Printf("[Connection] tool %s failed: %v", payload.ToolName, err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, err.Error()))
		return
	}

	log.Printf("[Connection] tool %s success, result_len=%d", payload.ToolName, len(result))
	c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolResult(msg.ID, result, ""))
}

// handleError 处理错误消息
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Connection] parse error payload failed: %v", err)
		return
	}
	log.Printf("[Connection] error from=%s code=%s msg=%s", msg.From, payload.Code, payload.Message)
}

