package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"uap"
)

// Bridge UAP 客户端 + 工具路由层
type Bridge struct {
	cfg    *Config
	client *uap.Client

	// 工具目录
	toolCatalog map[string]string // tool_name → agent_id
	llmTools    []LLMTool         // LLM function calling 工具列表
	catalogMu   sync.RWMutex

	// 请求-响应关联
	pending map[string]chan *uap.ToolResultPayload // request_id → result channel
	pendMu  sync.Mutex
}

// NewBridge 创建 Bridge
func NewBridge(cfg *Config) *Bridge {
	client := uap.NewClient(cfg.GatewayURL, cfg.AgentID, "llm_mcp", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Tools = nil // llm-mcp-agent 不对外注册工具
	client.Capacity = 10

	b := &Bridge{
		cfg:         cfg,
		client:      client,
		toolCatalog: make(map[string]string),
		pending:     make(map[string]chan *uap.ToolResultPayload),
	}

	client.OnMessage = b.handleMessage
	return b
}

// Run 启动连接（阻塞，自动重连）
func (b *Bridge) Run() {
	b.client.Run()
}

// Stop 停止
func (b *Bridge) Stop() {
	b.client.Stop()
}

// ========================= 工具发现 =========================

// DiscoverTools 从 gateway 获取所有在线 agent 的工具定义
func (b *Bridge) DiscoverTools() error {
	url := fmt.Sprintf("%s/api/gateway/tools", b.cfg.GatewayHTTP)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %v", err)
	}

	var result struct {
		Success bool             `json:"success"`
		Tools   []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("gateway returned success=false")
	}

	catalog := make(map[string]string)
	var llmTools []LLMTool

	for _, raw := range result.Tools {
		var tool struct {
			AgentID     string          `json:"agent_id"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal(raw, &tool); err != nil {
			log.Printf("[Bridge] skip invalid tool: %v", err)
			continue
		}

		// 跳过自身的工具（如果有）
		if tool.AgentID == b.cfg.AgentID {
			continue
		}

		catalog[tool.Name] = tool.AgentID

		// 构建 LLM 函数名：使用原始名称（如 "todolist.GetTodos"），替换 . 为 _
		llmFuncName := sanitizeToolName(tool.Name)

		params := tool.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		llmTools = append(llmTools, LLMTool{
			Type: "function",
			Function: LLMFunction{
				Name:        llmFuncName,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}

	b.catalogMu.Lock()
	b.toolCatalog = catalog
	b.llmTools = llmTools
	b.catalogMu.Unlock()

	log.Printf("[Bridge] discovered %d tools from %d entries", len(llmTools), len(result.Tools))
	return nil
}

// sanitizeToolName 将工具名转为 LLM 兼容格式（. → _）
func sanitizeToolName(name string) string {
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			result[i] = '_'
		} else {
			result[i] = name[i]
		}
	}
	return string(result)
}

// unsanitizeToolName 将 LLM 函数名还原为原始工具名（_ → .）
// 只替换第一个 _（命名空间分隔符），其余保留
func unsanitizeToolName(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == '_' {
			return name[:i] + "." + name[i+1:]
		}
	}
	return name
}

// getToolAgent 查找工具所属的 agent
func (b *Bridge) getToolAgent(toolName string) (string, bool) {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	agentID, ok := b.toolCatalog[toolName]
	return agentID, ok
}

// getLLMTools 获取 LLM 工具列表
func (b *Bridge) getLLMTools() []LLMTool {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	return b.llmTools
}

// ========================= 跨 Agent 工具调用 =========================

// CallTool 发送 MsgToolCall 到目标 agent 并等待 MsgToolResult
func (b *Bridge) CallTool(toolName string, args json.RawMessage) (string, error) {
	// 查找目标 agent
	agentID, ok := b.getToolAgent(toolName)
	if !ok {
		return "", fmt.Errorf("tool %s not found in catalog", toolName)
	}

	// 创建 pending channel
	msgID := uap.NewMsgID()
	ch := make(chan *uap.ToolResultPayload, 1)

	b.pendMu.Lock()
	b.pending[msgID] = ch
	b.pendMu.Unlock()

	defer func() {
		b.pendMu.Lock()
		delete(b.pending, msgID)
		b.pendMu.Unlock()
	}()

	// 发送 tool_call
	err := b.client.Send(&uap.Message{
		Type: uap.MsgToolCall,
		ID:   msgID,
		From: b.cfg.AgentID,
		To:   agentID,
		Payload: mustMarshal(uap.ToolCallPayload{
			ToolName:  toolName,
			Arguments: args,
		}),
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("send tool_call: %v", err)
	}

	// 等待结果
	timeout := time.Duration(b.cfg.ToolCallTimeoutSec) * time.Second
	select {
	case result := <-ch:
		if !result.Success {
			return "", fmt.Errorf("tool error: %s", result.Error)
		}
		return result.Result, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("tool_call %s timeout after %v", toolName, timeout)
	}
}

// ========================= UAP 消息处理 =========================

// handleMessage 处理来自 gateway 的消息
func (b *Bridge) handleMessage(msg *uap.Message) {
	switch msg.Type {
	case uap.MsgNotify:
		var payload uap.NotifyPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid notify payload: %v", err)
			return
		}
		if payload.Channel == "wechat" {
			go b.handleChat(msg.From, payload.To, payload.Content)
		} else {
			log.Printf("[Bridge] unhandled notify channel: %s", payload.Channel)
		}

	case uap.MsgToolResult:
		var payload uap.ToolResultPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid tool_result payload: %v", err)
			return
		}
		b.pendMu.Lock()
		ch, ok := b.pending[payload.RequestID]
		b.pendMu.Unlock()
		if ok {
			ch <- &payload
		} else {
			log.Printf("[Bridge] no pending request for %s", payload.RequestID)
		}

	case uap.MsgError:
		var payload uap.ErrorPayload
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			log.Printf("[Bridge] error: %s - %s (msg_id=%s)", payload.Code, payload.Message, msg.ID)
			// 如果是 agent_offline 错误，也需要释放 pending
			b.pendMu.Lock()
			ch, ok := b.pending[msg.ID]
			b.pendMu.Unlock()
			if ok {
				ch <- &uap.ToolResultPayload{
					RequestID: msg.ID,
					Success:   false,
					Error:     payload.Message,
				}
			}
		}

	case uap.MsgTaskAssign:
		var taskPayload uap.TaskAssignPayload
		if err := json.Unmarshal(msg.Payload, &taskPayload); err != nil {
			log.Printf("[Bridge] invalid task_assign payload: %v", err)
			return
		}
		// 解析内部 payload
		var assistantPayload AssistantTaskPayload
		if err := json.Unmarshal(taskPayload.Payload, &assistantPayload); err != nil {
			log.Printf("[Bridge] invalid assistant task payload: %v", err)
			return
		}
		if assistantPayload.TaskType == "assistant_chat" {
			go b.handleAssistantTask(taskPayload.TaskID, &assistantPayload)
		} else {
			log.Printf("[Bridge] unknown task_type: %s", assistantPayload.TaskType)
		}

	default:
		log.Printf("[Bridge] unhandled message type: %s from %s", msg.Type, msg.From)
	}
}

// ========================= 后台刷新 =========================

// StartRefreshLoop 后台定时刷新工具目录
func (b *Bridge) StartRefreshLoop() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := b.DiscoverTools(); err != nil {
				log.Printf("[Bridge] refresh tools failed: %v", err)
			}
		}
	}()
}

// ========================= 工具函数 =========================

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
