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

	"agentbase"
	"uap"
)

// 共享的 gateway HTTP 客户端
var gatewayHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// toolResultWithFrom 工具结果（含来源 agent ID）
type toolResultWithFrom struct {
	uap.ToolResultPayload
	FromID string // 返回结果的 agent ID
}

// AgentInfo agent 元数据（用于两级路由）
type AgentInfo struct {
	ID          string
	Name        string
	Description string
	ToolNames   []string
}

// Bridge UAP 客户端 + 工具路由层
type Bridge struct {
	cfg    *Config
	client *uap.Client

	// 日志查询工具
	logToolKit *agentbase.LogToolKit

	// 工具目录
	toolCatalog map[string]string // tool_name → agent_id
	llmTools    []LLMTool         // LLM function calling 工具列表
	catalogMu   sync.RWMutex

	// agent 感知存储（两级路由用）
	agentInfo  map[string]AgentInfo   // agent_id → 元数据
	agentTools map[string][]LLMTool   // agent_id → 该 agent 的工具列表

	// 请求-响应关联
	pending map[string]chan *toolResultWithFrom // request_id → result channel
	pendMu  sync.Mutex

	// 微信对话上下文管理
	wechatConvMgr *WechatConversationManager

	// 任务生命周期 hook
	hooks *HookManager
}

// NewBridge 创建 Bridge
func NewBridge(cfg *Config) *Bridge {
	client := uap.NewClient(cfg.GatewayURL, cfg.AgentID, "llm_mcp", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Capacity = 10

	logToolKit := agentbase.NewLogToolKit("LlmMcp", "llm-mcp-agent.log")
	client.Tools = logToolKit.ToolDefs() // 注册自身日志查询工具

	// 初始化微信对话管理器
	timeout := time.Duration(cfg.WechatSessionTimeoutMin) * time.Minute
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	maxMessages := cfg.WechatMaxMessages
	if maxMessages <= 0 {
		maxMessages = 40
	}
	maxTurns := cfg.WechatMaxTurns
	if maxTurns <= 0 {
		maxTurns = 15
	}

	b := &Bridge{
		cfg:           cfg,
		client:        client,
		logToolKit:    logToolKit,
		toolCatalog:   make(map[string]string),
		agentInfo:     make(map[string]AgentInfo),
		agentTools:    make(map[string][]LLMTool),
		pending:       make(map[string]chan *toolResultWithFrom),
		wechatConvMgr: NewWechatConversationManager(timeout, maxMessages, maxTurns),
	}

	client.OnMessage = b.handleMessage

	// 初始化 hook 管理器
	b.hooks = NewHookManager()
	b.hooks.Register(&WechatUsageSummaryHook{bridge: b})

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
	agentToolsMap := make(map[string][]LLMTool)
	for name, entry := range dedupMap {
		llmTools = append(llmTools, entry.Tool)
		toolNames = append(toolNames, name)
		agentToolsMap[entry.AgentID] = append(agentToolsMap[entry.AgentID], entry.Tool)
	}

	b.catalogMu.Lock()
	prevCount := len(b.llmTools)
	b.toolCatalog = catalog
	b.llmTools = llmTools
	b.agentTools = agentToolsMap
	b.catalogMu.Unlock()

	if len(llmTools) != prevCount {
		log.Printf("[Bridge] discovered %d unique tools from %d entries (was %d). Tools: %v", len(llmTools), len(result.Tools), prevCount, toolNames)
	}
	return nil
}

// DiscoverAgents 从 gateway 获取所有在线 agent 的元数据
func (b *Bridge) DiscoverAgents() error {
	url := fmt.Sprintf("%s/api/gateway/agents", b.cfg.GatewayHTTP)

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
		Success bool `json:"success"`
		Agents  []struct {
			AgentID     string   `json:"agent_id"`
			AgentType   string   `json:"agent_type"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Tools       []string `json:"tools"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("gateway returned success=false")
	}

	infoMap := make(map[string]AgentInfo, len(result.Agents))
	for _, a := range result.Agents {
		if a.AgentID == b.cfg.AgentID {
			continue // 跳过自身
		}
		infoMap[a.AgentID] = AgentInfo{
			ID:          a.AgentID,
			Name:        a.Name,
			Description: a.Description,
			ToolNames:   a.Tools,
		}
	}

	b.catalogMu.Lock()
	b.agentInfo = infoMap
	b.catalogMu.Unlock()

	log.Printf("[Bridge] discovered %d agents", len(infoMap))
	return nil
}

// getAgentDescriptionBlock 构建 agent 描述文本用于注入系统提示
func (b *Bridge) getAgentDescriptionBlock() string {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	if len(b.agentInfo) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用 Agent 能力\n")
	for _, info := range b.agentInfo {
		if info.Description == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", info.Name, info.ID, info.Description))
	}
	return sb.String()
}

// executeCodeAgentType execute-code-agent 的类型标识（元工具，始终保留不参与路由筛选）
const executeCodeAgentType = "execute_code"

// routeAgents Level 1 agent 选择：用 LLM 从 agent 列表中筛选与用户问题相关的 agent
// execute-code-agent 和文件工具 agent 作为基础能力始终保留，不参与 LLM 筛选
func (b *Bridge) routeAgents(query string) []string {
	b.catalogMu.RLock()
	agentInfoCopy := make(map[string]AgentInfo, len(b.agentInfo))
	for k, v := range b.agentInfo {
		agentInfoCopy[k] = v
	}
	agentToolsCopy := make(map[string][]LLMTool, len(b.agentTools))
	for k, v := range b.agentTools {
		agentToolsCopy[k] = v
	}
	b.catalogMu.RUnlock()

	if len(agentInfoCopy) == 0 {
		return nil
	}

	// 分离 execute-code-agent（元工具，始终保留）和待路由 agent
	var alwaysInclude []string
	var catalog strings.Builder
	var routeableIDs []string

	for _, info := range agentInfoCopy {
		// execute-code-agent 和文件工具 agent 始终保留，不参与路由
		if isExecuteCodeAgent(info) || isFileToolAgent(info) {
			alwaysInclude = append(alwaysInclude, info.ID)
			continue
		}
		toolNames := make([]string, 0, len(agentToolsCopy[info.ID]))
		for _, t := range agentToolsCopy[info.ID] {
			toolNames = append(toolNames, t.Function.Name)
		}
		catalog.WriteString(fmt.Sprintf("- agent_id=%s name=%s description=%s tools=[%s]\n",
			info.ID, info.Name, info.Description, strings.Join(toolNames, ", ")))
		routeableIDs = append(routeableIDs, info.ID)
	}

	// 没有需要路由的 agent → 直接返回始终保留的
	if len(routeableIDs) == 0 {
		return alwaysInclude
	}

	routePrompt := fmt.Sprintf(`你是一个 Agent 路由器。根据用户问题，从以下 Agent 列表中选择所有可能需要的 Agent。

用户问题: %s

Agent 列表:
%s
选择规则：
1. 宁多勿少，把所有可能相关的 Agent 都选上
2. 只返回 agent_id 的 JSON 数组，不要其他文字
3. 如果不确定，返回所有 agent_id

示例: ["go_blog", "codegen-xxx"]`, query, catalog.String())

	messages := []Message{
		{Role: "user", Content: routePrompt},
	}

	resp, _, err := SendLLMRequest(&b.cfg.LLM, messages, nil)
	if err != nil {
		log.Printf("[Agent路由] LLM 调用失败: %v, 返回全部 agent", err)
		return append(alwaysInclude, routeableIDs...) // 降级兜底
	}

	// 解析 JSON 数组
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var selectedIDs []string
	if err := json.Unmarshal([]byte(resp), &selectedIDs); err != nil {
		log.Printf("[Agent路由] 解析失败: %v, 返回全部 agent", err)
		return append(alwaysInclude, routeableIDs...) // 降级兜底
	}

	if len(selectedIDs) == 0 {
		log.Printf("[Agent路由] LLM 未选择任何 agent, 返回全部")
		return append(alwaysInclude, routeableIDs...)
	}

	// 合并：始终保留 + LLM 选中
	result := append(alwaysInclude, selectedIDs...)
	log.Printf("[Agent路由] 从 %d 个 agent 中选择了 %d 个 (always=%d routed=%d): %v",
		len(agentInfoCopy), len(result), len(alwaysInclude), len(selectedIDs), result)
	return result
}

// isExecuteCodeAgent 判断是否为 execute-code-agent（元工具）
func isExecuteCodeAgent(info AgentInfo) bool {
	// 通过工具名判断（更可靠，不依赖 agent_id 命名）
	for _, name := range info.ToolNames {
		if name == "ExecuteCode" {
			return true
		}
	}
	return false
}

// isFileToolName 判断工具名是否为文件操作工具（始终保留）
func isFileToolName(name string) bool {
	return strings.HasSuffix(name, "ReadFile") ||
		strings.HasSuffix(name, "WriteFile") ||
		strings.HasSuffix(name, "ExecBash")
}

// isFileToolAgent 判断 agent 是否提供文件操作工具
func isFileToolAgent(info AgentInfo) bool {
	for _, name := range info.ToolNames {
		if isFileToolName(name) {
			return true
		}
	}
	return false
}

// getToolsForAgents 从 agentTools 收集指定 agent 的工具
func (b *Bridge) getToolsForAgents(agentIDs []string) []LLMTool {
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()

	idSet := make(map[string]bool, len(agentIDs))
	for _, id := range agentIDs {
		idSet[id] = true
	}

	var tools []LLMTool
	seen := make(map[string]bool)
	for agentID, agentToolList := range b.agentTools {
		if !idSet[agentID] {
			continue
		}
		for _, tool := range agentToolList {
			if !seen[tool.Function.Name] {
				tools = append(tools, tool)
				seen[tool.Function.Name] = true
			}
		}
	}
	return tools
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
		log.Printf("[Bridge] no tools matched selection %v, not using tools", selectedTools)
		return nil
	}

	log.Printf("[Bridge] filtered %d tools from %d by user selection", len(filtered), len(allTools))
	return filtered
}

// routeTools 智能工具路由：用 LLM 从工具目录中筛选与用户问题相关的工具
// ExecuteCode 和文件工具（ReadFile/WriteFile/ExecBash）作为基础能力始终保留，不参与 LLM 筛选
func (b *Bridge) routeTools(query string, tools []LLMTool) []LLMTool {
	// 分离 ExecuteCode 和文件工具（元工具，始终保留）和待路由工具
	var alwaysKeep []LLMTool
	var routable []LLMTool
	for _, tool := range tools {
		if tool.Function.Name == "ExecuteCode" || isFileToolName(tool.Function.Name) {
			alwaysKeep = append(alwaysKeep, tool)
		} else {
			routable = append(routable, tool)
		}
	}

	if len(routable) == 0 {
		return alwaysKeep
	}

	// 构建工具目录（仅 name + description，不含参数 schema，节省 token）
	var catalog strings.Builder
	toolMap := make(map[string]LLMTool, len(routable))
	for i, tool := range routable {
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
		log.Printf("[工具路由] LLM 调用失败: %v, 保留元工具", err)
		return alwaysKeep // 降级：至少保留 ExecuteCode
	}

	// 解析 JSON 数组
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var toolNames []string
	if err := json.Unmarshal([]byte(resp), &toolNames); err != nil {
		log.Printf("[工具路由] 解析失败: %v, 原始响应: %s, 保留元工具", err, resp)
		return alwaysKeep
	}

	if len(toolNames) == 0 {
		log.Printf("[工具路由] LLM 判断无需业务工具，保留元工具")
		return alwaysKeep // ExecuteCode 始终可用
	}

	// 筛选出对应的完整工具定义
	var selected []LLMTool
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			selected = append(selected, tool)
		}
	}

	// 合并：始终保留 + LLM 选中
	result := append(alwaysKeep, selected...)
	log.Printf("[工具路由] 从 %d 个工具中筛选出 %d 个 (always=%d routed=%d): %v",
		len(tools), len(result), len(alwaysKeep), len(selected), toolNames)
	return result
}

// ========================= 跨 Agent 工具调用 =========================

// longRunningTools 需要长超时的工具（编码、部署等耗时操作）
var longRunningTools = map[string]bool{
	"CodegenStartSession": true,
	"CodegenSendMessage":  true,
	"DeployProject":       true,
	"DeployPipeline":      true,
	"ExecuteCode":         true,
}

// isLongRunningTool 判断是否为长时间运行的工具
func isLongRunningTool(toolName string) bool {
	return longRunningTools[toolName]
}

// ToolCallResult 工具调用结果（含路由信息）
type ToolCallResult struct {
	Result  string // 工具返回内容
	AgentID string // 目标 agent ID（发送方）
	FromID  string // 结果来源 agent ID（响应方）
}

// CallTool 发送 MsgToolCall 到目标 agent 并等待 MsgToolResult
func (b *Bridge) CallTool(toolName string, args json.RawMessage) (*ToolCallResult, error) {
	// 查找目标 agent
	agentID, ok := b.getToolAgent(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %s not found in catalog", toolName)
	}

	// 创建 pending channel
	msgID := uap.NewMsgID()
	ch := make(chan *toolResultWithFrom, 1)

	b.pendMu.Lock()
	b.pending[msgID] = ch
	b.pendMu.Unlock()

	defer func() {
		b.pendMu.Lock()
		delete(b.pending, msgID)
		b.pendMu.Unlock()
	}()

	log.Printf("[Bridge] tool_call → agent=%s tool=%s msgID=%s", agentID, toolName, msgID)

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
		return nil, fmt.Errorf("send tool_call: %v", err)
	}

	// 等待结果（长时间工具使用更长超时）
	timeout := time.Duration(b.cfg.ToolCallTimeoutSec) * time.Second
	if isLongRunningTool(toolName) {
		longTimeout := time.Duration(b.cfg.LongToolTimeoutSec) * time.Second
		if longTimeout <= 0 {
			longTimeout = 600 * time.Second
		}
		timeout = longTimeout
	}
	select {
	case result := <-ch:
		if !result.Success {
			return &ToolCallResult{AgentID: agentID, FromID: result.FromID},
				fmt.Errorf("tool error: %s", result.Error)
		}
		log.Printf("[Bridge] tool_result ← from=%s tool=%s msgID=%s", result.FromID, toolName, msgID)
		return &ToolCallResult{
			Result:  result.Result,
			AgentID: agentID,
			FromID:  result.FromID,
		}, nil
	case <-time.After(timeout):
		return &ToolCallResult{AgentID: agentID},
			fmt.Errorf("tool_call %s timeout after %v", toolName, timeout)
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
			go b.handleWechatMessage(msg.From, payload.To, payload.Content)
		} else {
			log.Printf("[Bridge] unhandled notify channel: %s", payload.Channel)
		}

	case uap.MsgToolCall:
		var payload uap.ToolCallPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid tool_call payload: %v", err)
			return
		}
		var args map[string]interface{}
		if len(payload.Arguments) > 0 {
			json.Unmarshal(payload.Arguments, &args)
		}
		if result, handled := b.logToolKit.HandleTool(payload.ToolName, args); handled {
			b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   true,
				Result:    result,
			})
		} else {
			b.client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   false,
				Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
			})
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
			ch <- &toolResultWithFrom{ToolResultPayload: payload, FromID: msg.From}
		} else {
			log.Printf("[Bridge] no pending request for %s (from=%s)", payload.RequestID, msg.From)
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
				ch <- &toolResultWithFrom{
					ToolResultPayload: uap.ToolResultPayload{
						RequestID: msg.ID,
						Success:   false,
						Error:     payload.Message,
					},
					FromID: msg.From,
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

// StartRefreshLoop 后台定时刷新工具目录和 agent 信息
func (b *Bridge) StartRefreshLoop() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := b.DiscoverTools(); err != nil {
				log.Printf("[Bridge] refresh tools failed: %v", err)
			}
			if err := b.DiscoverAgents(); err != nil {
				log.Printf("[Bridge] refresh agents failed: %v", err)
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
