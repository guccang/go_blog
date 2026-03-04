package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"uap"
)

// 共享的 gateway HTTP 客户端
var gatewayHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

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

	resp, err := gatewayHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %v", err)
	}

	var result struct {
		Success bool              `json:"success"`
		Tools   []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("gateway returned success=false")
	}

	catalog := make(map[string]string)

	// 用于去重，记录已添加的工具以及判断是否需要覆盖（优先保留有参数的）
	type toolEntry struct {
		AgentID   string
		Tool      LLMTool
		HasParams bool
	}
	dedupMap := make(map[string]toolEntry)

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

		// 构建 LLM 函数名
		llmFuncName := sanitizeToolName(tool.Name)

		params := tool.Parameters
		hasParams := len(params) > 0 && string(params) != `{"type":"object","properties":{}}`
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		newTool := LLMTool{
			Type: "function",
			Function: LLMFunction{
				Name:        llmFuncName,
				Description: tool.Description,
				Parameters:  params,
			},
		}

		// 去重逻辑：如果已经存在同名工具，优先保留有参数的那个
		existing, exists := dedupMap[llmFuncName]
		if !exists || (!existing.HasParams && hasParams) {
			dedupMap[llmFuncName] = toolEntry{
				AgentID:   tool.AgentID,
				Tool:      newTool,
				HasParams: hasParams,
			}
			catalog[tool.Name] = tool.AgentID // 更新 catalog 路由到正确的 Agent
		}
	}

	var llmTools []LLMTool
	var toolNames []string
	for name, entry := range dedupMap {
		llmTools = append(llmTools, entry.Tool)
		toolNames = append(toolNames, name)
	}

	b.catalogMu.Lock()
	b.toolCatalog = catalog
	b.llmTools = llmTools
	b.catalogMu.Unlock()

	log.Printf("[Bridge] discovered %d unique tools from %d entries. Tools: %v", len(llmTools), len(result.Tools), toolNames)
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

// filterToolsBySelection 根据用户选择过滤工具列表
// selectedTools 为空时返回全部工具
func (b *Bridge) filterToolsBySelection(selectedTools []string) []LLMTool {
	allTools := b.getLLMTools()
	if len(selectedTools) == 0 {
		return allTools
	}

	// 构建 O(1) 查找表，同时支持 sanitized 名称（下划线）和原始名称（点号）
	selectedMap := make(map[string]bool, len(selectedTools)*2)
	for _, name := range selectedTools {
		selectedMap[name] = true
		selectedMap[sanitizeToolName(name)] = true
	}

	var filtered []LLMTool
	for _, tool := range allTools {
		if selectedMap[tool.Function.Name] {
			filtered = append(filtered, tool)
		}
	}

	if len(filtered) == 0 {
		log.Printf("[Bridge] no tools matched selection %v, falling back to all %d tools", selectedTools, len(allTools))
		return allTools
	}

	log.Printf("[Bridge] filtered %d tools from %d by user selection", len(filtered), len(allTools))
	return filtered
}

// routeTools 智能工具路由：用 LLM 从工具目录中筛选与用户问题相关的工具
// 当可用工具数 > maxToolsBeforeRoute 时自动启用
func (b *Bridge) routeTools(query string, tools []LLMTool) []LLMTool {
	// 构建工具目录（仅 name + description，不含参数 schema，节省 token）
	var catalog strings.Builder
	toolMap := make(map[string]LLMTool, len(tools))
	for i, tool := range tools {
		catalog.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, tool.Function.Name, tool.Function.Description))
		toolMap[tool.Function.Name] = tool
	}

	routePrompt := fmt.Sprintf(`你是一个工具路由器。根据用户的问题，从以下工具目录中选择所有可能需要用到的工具。

用户问题: %s

工具目录:
%s
选择规则：
1. 宁多勿少，把所有可能相关的工具都选上
2. 如果任务需要日期信息，必须包含 RawCurrentDate
3. 如果涉及查询数据，同时选择获取数据的工具和可能需要的辅助工具
4. 只返回JSON数组，不要其他文字

示例: ["RawCurrentDate", "RawGetExerciseByDateRange"]
如果不需要任何工具，返回 []`, query, catalog.String())

	messages := []Message{
		{Role: "user", Content: routePrompt},
	}

	// 无工具的 LLM 请求用于路由
	resp, _, err := SendLLMRequest(&b.cfg.LLM, messages, nil)
	if err != nil {
		log.Printf("[工具路由] LLM 调用失败: %v, 使用全部工具", err)
		return tools // fallback 到全部工具
	}

	// 解析 JSON 数组
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var toolNames []string
	if err := json.Unmarshal([]byte(resp), &toolNames); err != nil {
		log.Printf("[工具路由] 解析失败: %v, 原始响应: %s, 使用全部工具", err, resp)
		return tools // fallback 到全部工具
	}

	if len(toolNames) == 0 {
		log.Printf("[工具路由] LLM 判断无需工具")
		return []LLMTool{} // 返回空，让 LLM 直接回答
	}

	// 筛选出对应的完整工具定义
	var selected []LLMTool
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			selected = append(selected, tool)
		}
	}

	if len(selected) == 0 {
		log.Printf("[工具路由] 未匹配到任何工具，使用全部工具")
		return tools
	}

	log.Printf("[工具路由] 从 %d 个工具中筛选出 %d 个: %v", len(tools), len(selected), toolNames)
	return selected
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
		// 先探测 task_type 字段
		var taskType struct {
			TaskType string `json:"task_type"`
		}
		json.Unmarshal(taskPayload.Payload, &taskType)

		switch taskType.TaskType {
		case "assistant_chat":
			var assistantPayload AssistantTaskPayload
			if err := json.Unmarshal(taskPayload.Payload, &assistantPayload); err != nil {
				log.Printf("[Bridge] invalid assistant task payload: %v", err)
				return
			}
			go b.handleAssistantTask(taskPayload.TaskID, &assistantPayload)
		case "llm_request":
			var llmPayload LLMRequestPayload
			if err := json.Unmarshal(taskPayload.Payload, &llmPayload); err != nil {
				log.Printf("[Bridge] invalid llm_request payload: %v", err)
				return
			}
			go b.handleLLMRequestTask(taskPayload.TaskID, &llmPayload)
		case "resume_task":
			var resumePayload ResumeTaskPayload
			if err := json.Unmarshal(taskPayload.Payload, &resumePayload); err != nil {
				log.Printf("[Bridge] invalid resume_task payload: %v", err)
				return
			}
			go b.handleResumeTask(taskPayload.TaskID, &resumePayload)
		default:
			log.Printf("[Bridge] unknown task_type: %s", taskType.TaskType)
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

// WarmupLLM 预热 LLM 连接，提前建立 TCP+TLS 连接，避免首次请求 EOF
func WarmupLLM(cfg *LLMConfig) {
	url := fmt.Sprintf("%s/models", cfg.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[LLM-MCP] warmup: create request failed: %v", err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		log.Printf("[LLM-MCP] warmup: request failed (non-critical): %v", err)
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // 消费 body 以确保连接可被复用
	log.Printf("[LLM-MCP] warmup: LLM connection established (status=%d)", resp.StatusCode)
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
