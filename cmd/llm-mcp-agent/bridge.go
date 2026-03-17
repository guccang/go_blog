package main

import (
	"context"
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

// queuedTask 缓冲队列中的待执行任务
type queuedTask struct {
	taskID    string
	taskType  string
	handler   func()
	createdAt time.Time
}

// 共享的 gateway HTTP 客户端
var gatewayHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// toolResultWithFrom 工具结果（含来源 agent ID）
type toolResultWithFrom struct {
	uap.ToolResultPayload
	FromID string // 返回结果的 agent ID
}

// AgentInfo agent 元数据（用于两级路由 + 能力描述注入）
type AgentInfo struct {
	ID               string
	Name             string
	Description      string
	ToolNames        []string
	Models           []string // 合并后的模型配置名列表（如 default, deepseek）
	ClaudeCodeModels []string // Claude Code 可用配置
	OpenCodeModels   []string // OpenCode 可用配置
	CodingTools      []string // 可用编码工具（claudecode, opencode）
}

// Bridge UAP 客户端 + 工具路由层
type Bridge struct {
	cfg    *Config
	client *uap.Client

	// 工具目录
	toolCatalog map[string]string // tool_name → agent_id
	llmTools    []LLMTool         // LLM function calling 工具列表
	catalogMu   sync.RWMutex

	// agent 感知存储（两级路由用）
	agentInfo  map[string]AgentInfo // agent_id → 元数据
	agentTools map[string][]LLMTool // agent_id → 该 agent 的工具列表

	// 请求-响应关联
	pending map[string]chan *toolResultWithFrom // request_id → result channel
	pendMu  sync.Mutex

	// 微信对话上下文管理
	wechatConvMgr *WechatConversationManager

	// Skill 管理器
	skillMgr *SkillManager

	// 任务生命周期 hook
	hooks *HookManager

	// 并发控制
	activeTasks  map[string]string // taskID → task_type
	activeTaskMu sync.Mutex

	// 任务缓冲队列
	taskQueue chan *queuedTask
	queueDone chan struct{}
}

// NewBridge 创建 Bridge
func NewBridge(cfg *Config) *Bridge {
	client := uap.NewClient(cfg.GatewayURL, cfg.AgentID, "llm_mcp", cfg.AgentName)
	client.AuthToken = cfg.AuthToken
	client.Capacity = cfg.MaxConcurrent
	client.Tools = nil // llm-mcp-agent 不对外注册工具

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
		toolCatalog:   make(map[string]string),
		agentInfo:     make(map[string]AgentInfo),
		agentTools:    make(map[string][]LLMTool),
		pending:       make(map[string]chan *toolResultWithFrom),
		wechatConvMgr: NewWechatConversationManager(timeout, maxMessages, maxTurns),
		activeTasks:   make(map[string]string),
		taskQueue:     make(chan *queuedTask, cfg.TaskQueueSize),
		queueDone:     make(chan struct{}),
	}

	client.OnMessage = b.handleMessage

	// 初始化 hook 管理器
	b.hooks = NewHookManager()
	b.hooks.Register(&WechatUsageSummaryHook{bridge: b})

	// 初始化 Skill 管理器
	if cfg.WorkspaceDir != "" {
		b.skillMgr = NewSkillManager(cfg.WorkspaceDir)
		if err := b.skillMgr.Load(); err != nil {
			log.Printf("[Bridge] load skills: %v", err)
		}
	}

	return b
}

// Run 启动连接（阻塞，自动重连）
func (b *Bridge) Run() {
	b.client.Run()
}

// fallbackCooldown 返回配置的降级冷却时长
func (b *Bridge) fallbackCooldown() time.Duration {
	sec := b.cfg.FallbackCooldownSec
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

// sendLLM 带降级链的同步 LLM 请求
func (b *Bridge) sendLLM(messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	if len(b.cfg.Fallbacks) == 0 {
		return SendLLMRequest(&b.cfg.LLM, messages, tools)
	}
	return SendLLMRequestWithFallback(&b.cfg.LLM, b.cfg.Fallbacks, b.fallbackCooldown(), messages, tools)
}

// sendStreamingLLM 带降级链的流式 LLM 请求
func (b *Bridge) sendStreamingLLM(messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	if len(b.cfg.Fallbacks) == 0 {
		return SendStreamingLLMRequest(&b.cfg.LLM, messages, tools, onChunk)
	}
	return SendStreamingLLMRequestWithFallback(&b.cfg.LLM, b.cfg.Fallbacks, b.fallbackCooldown(), messages, tools, onChunk)
}

// Stop 停止
func (b *Bridge) Stop() {
	close(b.queueDone)
	b.client.Stop()
}

// ========================= 并发控制 =========================

// canAccept 是否可以接受新任务
func (b *Bridge) canAccept() bool {
	b.activeTaskMu.Lock()
	defer b.activeTaskMu.Unlock()
	return len(b.activeTasks) < b.cfg.MaxConcurrent
}

// registerTask 注册活跃任务
func (b *Bridge) registerTask(taskID, taskType string) {
	b.activeTaskMu.Lock()
	defer b.activeTaskMu.Unlock()
	b.activeTasks[taskID] = taskType
	log.Printf("[Bridge] task registered: %s (type=%s, active=%d/%d)", taskID, taskType, len(b.activeTasks), b.cfg.MaxConcurrent)
}

// deregisterTask 注销活跃任务，并尝试从队列消费下一个
func (b *Bridge) deregisterTask(taskID string) {
	b.activeTaskMu.Lock()
	delete(b.activeTasks, taskID)
	active := len(b.activeTasks)
	b.activeTaskMu.Unlock()
	log.Printf("[Bridge] task deregistered: %s (active=%d/%d)", taskID, active, b.cfg.MaxConcurrent)
	b.drainQueue()
}

// activeCount 当前活跃任务数
func (b *Bridge) activeCount() int {
	b.activeTaskMu.Lock()
	defer b.activeTaskMu.Unlock()
	return len(b.activeTasks)
}

// loadFactor 负载因子 0.0~1.0
func (b *Bridge) loadFactor() float64 {
	if b.cfg.MaxConcurrent <= 0 {
		return 1.0
	}
	return float64(b.activeCount()) / float64(b.cfg.MaxConcurrent)
}

// enqueueOrReject 非阻塞入队，队列满时返回 false
func (b *Bridge) enqueueOrReject(qt *queuedTask) bool {
	select {
	case b.taskQueue <- qt:
		log.Printf("[Bridge] task enqueued: %s (type=%s, queueLen=%d/%d)", qt.taskID, qt.taskType, len(b.taskQueue), b.cfg.TaskQueueSize)
		return true
	default:
		log.Printf("[Bridge] task queue full, rejecting: %s (type=%s)", qt.taskID, qt.taskType)
		return false
	}
}

// drainQueue 从队列取出一个可执行任务并启动
func (b *Bridge) drainQueue() {
	if !b.canAccept() {
		return
	}
	select {
	case qt := <-b.taskQueue:
		log.Printf("[Bridge] task dequeued: %s (type=%s, queueLen=%d)", qt.taskID, qt.taskType, len(b.taskQueue))
		b.registerTask(qt.taskID, qt.taskType)
		go func() {
			defer b.deregisterTask(qt.taskID)
			qt.handler()
		}()
	default:
		// 队列为空
	}
}

// StartQueueConsumer 后台定时消费队列（兜底，正常流程靠 deregisterTask 触发 drainQueue）
func (b *Bridge) StartQueueConsumer() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-b.queueDone:
				return
			case <-ticker.C:
				b.drainQueue()
			}
		}
	}()
	log.Printf("[Bridge] queue consumer started (MaxConcurrent=%d TaskQueueSize=%d)", b.cfg.MaxConcurrent, b.cfg.TaskQueueSize)
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

	// 应用工具权限策略
	b.applyToolPolicy()

	return nil
}

// applyToolPolicy 根据配置的 allow/deny 列表过滤工具
func (b *Bridge) applyToolPolicy() {
	if b.cfg.ToolPolicy == nil {
		return
	}
	policy := b.cfg.ToolPolicy
	if len(policy.Allow) == 0 && len(policy.Deny) == 0 {
		return
	}

	denySet := make(map[string]bool, len(policy.Deny))
	for _, name := range policy.Deny {
		denySet[name] = true
		denySet[sanitizeToolName(name)] = true
	}
	allowSet := make(map[string]bool, len(policy.Allow))
	for _, name := range policy.Allow {
		allowSet[name] = true
		allowSet[sanitizeToolName(name)] = true
	}

	b.catalogMu.Lock()
	defer b.catalogMu.Unlock()

	var filtered []LLMTool
	var removed []string
	for _, tool := range b.llmTools {
		name := tool.Function.Name
		originalName := unsanitizeToolName(name)

		// deny 优先
		if denySet[name] || denySet[originalName] {
			removed = append(removed, originalName)
			delete(b.toolCatalog, originalName)
			continue
		}
		// allow 非空时，只保留白名单中的
		if len(allowSet) > 0 && !allowSet[name] && !allowSet[originalName] {
			removed = append(removed, originalName)
			delete(b.toolCatalog, originalName)
			continue
		}
		filtered = append(filtered, tool)
	}
	b.llmTools = filtered

	// 同步清理 agentTools
	for agentID, tools := range b.agentTools {
		var agentFiltered []LLMTool
		for _, tool := range tools {
			name := tool.Function.Name
			originalName := unsanitizeToolName(name)
			if denySet[name] || denySet[originalName] {
				continue
			}
			if len(allowSet) > 0 && !allowSet[name] && !allowSet[originalName] {
				continue
			}
			agentFiltered = append(agentFiltered, tool)
		}
		b.agentTools[agentID] = agentFiltered
	}

	if len(removed) > 0 {
		log.Printf("[Bridge] tool policy applied: removed %d tools: %v", len(removed), removed)
	}
}

// DiscoverAgents 从 gateway 获取所有在线 agent 的元数据（含 meta 扩展字段）
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
			AgentID     string         `json:"agent_id"`
			AgentType   string         `json:"agent_type"`
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Tools       []string       `json:"tools"`
			Meta        map[string]any `json:"meta"`
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
		info := AgentInfo{
			ID:          a.AgentID,
			Name:        a.Name,
			Description: a.Description,
			ToolNames:   a.Tools,
		}
		// 从 meta 提取动态能力信息
		if a.Meta != nil {
			info.Models = parseStringSlice(a.Meta["models"])
			info.ClaudeCodeModels = parseStringSlice(a.Meta["claudecode_models"])
			info.OpenCodeModels = parseStringSlice(a.Meta["opencode_models"])
			info.CodingTools = parseStringSlice(a.Meta["coding_tools"])
		}
		infoMap[a.AgentID] = info
		log.Printf("[Bridge] agent: %s (%s) tools=%v models=%v coding_tools=%v",
			a.Name, a.AgentID, a.Tools, info.Models, info.CodingTools)
	}

	b.catalogMu.Lock()
	b.agentInfo = infoMap
	b.catalogMu.Unlock()

	log.Printf("[Bridge] discovered %d agents", len(infoMap))
	return nil
}

// parseStringSlice 从 any (interface{}) 解析 []string，兼容 JSON 反序列化的 []interface{}
func parseStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// getAgentDescriptionBlock 构建 agent 描述文本用于注入系统提示（含可用模型和工具信息）
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
		// 注入可用模型和编码工具信息，让 LLM 知道合法参数值
		if len(info.CodingTools) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用编码工具(tool参数): %s\n", strings.Join(info.CodingTools, ", ")))
		}
		if len(info.Models) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用模型配置(model参数): %s\n", strings.Join(info.Models, ", ")))
		}
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

	routePrompt := fmt.Sprintf(`你是一个 Agent 路由器。根据用户问题，从以下 Agent 列表中选择与任务直接相关的 Agent。

用户问题: %s

Agent 列表:
%s
选择规则：
1. 根据 Agent 的 description 和 tools 判断相关性，只选直接相关的
2. 与任务无关的 Agent 不要选
3. 只返回 agent_id 的 JSON 数组，不要其他文字
4. 简单问答不需要任何 Agent，返回空数组 []

示例: ["go_blog", "codegen-xxx"]`, query, catalog.String())

	messages := []Message{
		{Role: "user", Content: routePrompt},
	}

	resp, _, err := b.sendLLM(messages, nil)
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
		log.Printf("[Agent路由] LLM 未选择任何 agent, 仅保留基础 agent")
		return alwaysInclude
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

// getSiblingTools 获取与指定工具同 agent 的所有兄弟工具
// 用于工具业务失败时扩展可选工具集，让 LLM 自行决策是修复参数重试还是切换替代工具
func (b *Bridge) getSiblingTools(toolName string) []LLMTool {
	agentID, ok := b.getToolAgent(toolName)
	if !ok {
		return nil
	}
	b.catalogMu.RLock()
	defer b.catalogMu.RUnlock()
	return b.agentTools[agentID]
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

// CapabilitySelection Pass 1 的 LLM 选择结果
type CapabilitySelection struct {
	Agents []string     // 选中的 agent_id 列表
	Skills []SkillEntry // 选中的 skill 列表（完整条目）
}

// routeCapabilities Pass 1 能力选择：用 LLM 从 agent + skill 列表中选择与用户问题相关的能力
// execute-code-agent 和文件工具 agent 作为基础能力始终保留，不参与 LLM 筛选
func (b *Bridge) routeCapabilities(query string) *CapabilitySelection {
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

	// 收集 skill 信息
	var allSkills []SkillEntry
	if b.skillMgr != nil {
		allSkills = b.skillMgr.GetAllSkills()
	}

	if len(agentInfoCopy) == 0 && len(allSkills) == 0 {
		return nil
	}

	// 分离 execute-code-agent 和文件工具 agent（始终保留）
	var alwaysIncludeAgents []string
	var catalog strings.Builder

	// Agent 列表
	var routeableAgentIDs []string
	for _, info := range agentInfoCopy {
		if isExecuteCodeAgent(info) || isFileToolAgent(info) {
			alwaysIncludeAgents = append(alwaysIncludeAgents, info.ID)
			continue
		}
		toolNames := make([]string, 0, len(agentToolsCopy[info.ID]))
		for _, t := range agentToolsCopy[info.ID] {
			toolNames = append(toolNames, t.Function.Name)
		}
		catalog.WriteString(fmt.Sprintf("- agent_id=%s name=%s description=%s tools=[%s]\n",
			info.ID, info.Name, info.Description, strings.Join(toolNames, ", ")))
		routeableAgentIDs = append(routeableAgentIDs, info.ID)
	}

	// Skill 列表
	var skillCatalog strings.Builder
	var skillNames []string
	for _, skill := range allSkills {
		skillCatalog.WriteString(fmt.Sprintf("- skill_name=%s description=%s tools=[%s]\n",
			skill.Name, skill.Description, strings.Join(skill.Tools, ", ")))
		skillNames = append(skillNames, skill.Name)
	}

	// 没有需要路由的 agent 和 skill → 直接返回始终保留的 agent
	if len(routeableAgentIDs) == 0 && len(allSkills) == 0 {
		return &CapabilitySelection{Agents: alwaysIncludeAgents}
	}

	routePrompt := fmt.Sprintf(`你是一个能力路由器。根据用户问题，从以下 Agent 和 Skill 列表中精确选择需要的能力。
注意：execute-code-agent（ExecuteCode）和文件工具 agent 已自动包含，不需要你选择。

用户问题: %s

Agent 列表（数据源和执行工具）:
%s
Skill 列表（任务指引和专业知识）:
%s
选择规则：
1. 重点查看每个 Agent 的 tools 列表，选择拥有任务所需工具的 Agent
   - 数据查询/分析任务：必须选择拥有数据工具（如 Raw 开头的工具）的 Agent
   - 编码任务：选择拥有 Codegen 工具的 Agent
   - 部署任务：选择拥有 Deploy 工具的 Agent
2. 根据 Skill 的 description 判断是否匹配用户意图
3. 与任务无关的 Agent 坚决不选
4. 混合任务可同时选多种能力
5. 简单问答（闲聊、常识问题）返回空
6. 只返回 JSON，不要其他文字

返回格式: {"agents": ["agent_id1"], "skills": ["skill_name1"]}
简单问答: {"agents": [], "skills": []}`, query, catalog.String(), skillCatalog.String())

	messages := []Message{
		{Role: "user", Content: routePrompt},
	}

	resp, _, err := b.sendLLM(messages, nil)
	if err != nil {
		log.Printf("[能力路由] LLM 调用失败: %v, 返回全部能力", err)
		return &CapabilitySelection{
			Agents: append(alwaysIncludeAgents, routeableAgentIDs...),
			Skills: allSkills,
		}
	}

	// 解析 JSON
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var selection struct {
		Agents []string `json:"agents"`
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal([]byte(resp), &selection); err != nil {
		log.Printf("[能力路由] 解析失败: %v, 返回全部能力", err)
		return &CapabilitySelection{
			Agents: append(alwaysIncludeAgents, routeableAgentIDs...),
			Skills: allSkills,
		}
	}

	// 合并 agent：始终保留 + LLM 选中
	resultAgents := append(alwaysIncludeAgents, selection.Agents...)

	// 匹配 skill：从 LLM 选中的 skill name 查找完整条目
	skillNameSet := make(map[string]bool, len(selection.Skills))
	for _, name := range selection.Skills {
		skillNameSet[name] = true
	}
	var resultSkills []SkillEntry
	for _, skill := range allSkills {
		if skillNameSet[skill.Name] {
			resultSkills = append(resultSkills, skill)
		}
	}

	log.Printf("[能力路由] 从 %d agent + %d skill 中选择了 %d agent + %d skill: agents=%v skills=%v",
		len(agentInfoCopy), len(allSkills), len(resultAgents), len(resultSkills), resultAgents, selection.Skills)

	return &CapabilitySelection{
		Agents: resultAgents,
		Skills: resultSkills,
	}
}

// collectSkillTools 从选中的 skill 收集关联工具
func collectSkillTools(selectedSkills []SkillEntry, allTools []LLMTool) []LLMTool {
	if len(selectedSkills) == 0 {
		return nil
	}

	// 收集所有选中 skill 的工具名
	needSet := make(map[string]bool)
	for _, skill := range selectedSkills {
		for _, t := range skill.Tools {
			needSet[t] = true
			needSet[sanitizeToolName(t)] = true // 同时支持 sanitized 格式
		}
	}

	var result []LLMTool
	var matchedNames []string
	for _, tool := range allTools {
		if needSet[tool.Function.Name] || needSet[unsanitizeToolName(tool.Function.Name)] {
			result = append(result, tool)
			matchedNames = append(matchedNames, unsanitizeToolName(tool.Function.Name))
		}
	}

	// 日志：skill 声明的工具 vs 实际匹配到的工具
	var skillSummary []string
	for _, skill := range selectedSkills {
		skillSummary = append(skillSummary, fmt.Sprintf("%s→[%s]", skill.Name, strings.Join(skill.Tools, ",")))
	}
	log.Printf("[collectSkillTools] skills: %s → matched %d tools: %v",
		strings.Join(skillSummary, ", "), len(result), matchedNames)

	return result
}

// mergeCapabilityTools 合并 agent 工具 + skill 工具 + 基础工具，去重
func (b *Bridge) mergeCapabilityTools(selection *CapabilitySelection, allTools []LLMTool) []LLMTool {
	seen := make(map[string]bool)
	var merged []LLMTool

	addTool := func(tool LLMTool) {
		if !seen[tool.Function.Name] {
			seen[tool.Function.Name] = true
			merged = append(merged, tool)
		}
	}

	// 1. agent 工具
	agentTools := b.getToolsForAgents(selection.Agents)
	agentCount := 0
	for _, t := range agentTools {
		before := len(merged)
		addTool(t)
		if len(merged) > before {
			agentCount++
		}
	}

	// 2. skill 工具
	skillTools := collectSkillTools(selection.Skills, allTools)
	skillCount := 0
	for _, t := range skillTools {
		before := len(merged)
		addTool(t)
		if len(merged) > before {
			skillCount++
		}
	}

	// 3. 基础工具始终保留（ExecuteCode + 文件工具）
	baseCount := 0
	for _, t := range allTools {
		if t.Function.Name == "ExecuteCode" || isFileToolName(t.Function.Name) {
			before := len(merged)
			addTool(t)
			if len(merged) > before {
				baseCount++
			}
		}
	}

	var mergedNames []string
	for _, t := range merged {
		mergedNames = append(mergedNames, unsanitizeToolName(t.Function.Name))
	}
	log.Printf("[mergeCapabilityTools] agent=%d skill=%d base=%d → total=%d tools=%v",
		agentCount, skillCount, baseCount, len(merged), mergedNames)

	return merged
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

	routePrompt := fmt.Sprintf(`你是一个工具路由器。根据用户的问题，从以下工具目录中选择需要用到的工具。

用户问题: %s

工具目录:
%s
选择规则：
1. 只选与任务直接相关的工具
2. 如果选了 ExecuteCode，数据查询类工具（Raw 开头的）可以不选，因为 ExecuteCode 内部可通过 call_tool 调用所有工具
3. 根据工具 description 判断相关性
4. 只返回 JSON 数组，不要其他文字

示例: ["ExecuteCode", "RawCurrentDate"]
如果不需要任何工具，返回 []`, query, catalog.String())

	messages := []Message{
		{Role: "user", Content: routePrompt},
	}

	// 无工具的 LLM 请求用于路由
	resp, _, err := b.sendLLM(messages, nil)
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
			return &ToolCallResult{Result: result.Result, AgentID: agentID, FromID: result.FromID},
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

// CallToolCtx context 感知的工具调用，支持级联取消
func (b *Bridge) CallToolCtx(ctx context.Context, toolName string, args json.RawMessage) (*ToolCallResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("cancelled before tool call %s: %v", toolName, err)
	}

	agentID, ok := b.getToolAgent(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %s not found in catalog", toolName)
	}

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

	log.Printf("[Bridge] tool_call(ctx) → agent=%s tool=%s msgID=%s", agentID, toolName, msgID)

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
			return &ToolCallResult{Result: result.Result, AgentID: agentID, FromID: result.FromID},
				fmt.Errorf("tool error: %s", result.Error)
		}
		log.Printf("[Bridge] tool_result(ctx) ← from=%s tool=%s msgID=%s", result.FromID, toolName, msgID)
		return &ToolCallResult{
			Result:  result.Result,
			AgentID: agentID,
			FromID:  result.FromID,
		}, nil
	case <-time.After(timeout):
		return &ToolCallResult{AgentID: agentID},
			fmt.Errorf("tool_call %s timeout after %v", toolName, timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("tool_call %s cancelled: %v", toolName, ctx.Err())
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

		// 构建 handler（根据 task_type 解析 payload）
		var handler func()
		switch taskType.TaskType {
		case "assistant_chat":
			var assistantPayload AssistantTaskPayload
			if err := json.Unmarshal(taskPayload.Payload, &assistantPayload); err != nil {
				log.Printf("[Bridge] invalid assistant task payload: %v", err)
				return
			}
			handler = func() { b.handleAssistantTask(taskPayload.TaskID, &assistantPayload) }
		case "llm_request":
			var llmPayload LLMRequestPayload
			if err := json.Unmarshal(taskPayload.Payload, &llmPayload); err != nil {
				log.Printf("[Bridge] invalid llm_request payload: %v", err)
				return
			}
			handler = func() { b.handleLLMRequestTask(taskPayload.TaskID, &llmPayload) }
		case "resume_task":
			var resumePayload ResumeTaskPayload
			if err := json.Unmarshal(taskPayload.Payload, &resumePayload); err != nil {
				log.Printf("[Bridge] invalid resume_task payload: %v", err)
				return
			}
			handler = func() { b.handleResumeTask(taskPayload.TaskID, &resumePayload) }
		default:
			log.Printf("[Bridge] unknown task_type: %s", taskType.TaskType)
			return
		}

		// 统一发送 task_accepted（无论直接执行还是入队，都告知 gateway 已收到）
		b.client.Send(&uap.Message{
			Type:    uap.MsgTaskAccepted,
			ID:      uap.NewMsgID(),
			From:    b.cfg.AgentID,
			To:      "go_blog",
			Payload: mustMarshal(uap.TaskAcceptedPayload{TaskID: taskPayload.TaskID}),
			Ts:      time.Now().UnixMilli(),
		})

		// 准入控制：直接执行 / 入队 / 拒绝
		if b.canAccept() {
			b.registerTask(taskPayload.TaskID, taskType.TaskType)
			go func() {
				defer b.deregisterTask(taskPayload.TaskID)
				handler()
			}()
		} else if b.enqueueOrReject(&queuedTask{
			taskID:    taskPayload.TaskID,
			taskType:  taskType.TaskType,
			handler:   handler,
			createdAt: time.Now(),
		}) {
			// 入队成功，等待 drainQueue 触发执行
		} else {
			// 队列也满了，发送 task_rejected
			b.client.Send(&uap.Message{
				Type: uap.MsgTaskRejected,
				ID:   uap.NewMsgID(),
				From: b.cfg.AgentID,
				To:   "go_blog",
				Payload: mustMarshal(uap.TaskRejectedPayload{
					TaskID: taskPayload.TaskID,
					Reason: fmt.Sprintf("agent at max capacity (active=%d/%d, queue=%d/%d)",
						b.activeCount(), b.cfg.MaxConcurrent, len(b.taskQueue), b.cfg.TaskQueueSize),
				}),
				Ts: time.Now().UnixMilli(),
			})
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

// RecoverInProgressTasks 启动时扫描并恢复中断的任务
func (b *Bridge) RecoverInProgressTasks() {
	store := NewSessionStore(b.cfg.SessionDir)
	runningIDs, err := store.ListRunningSessions()
	if err != nil {
		log.Printf("[Bridge] recover: scan failed: %v", err)
		return
	}
	if len(runningIDs) == 0 {
		log.Printf("[Bridge] recover: no interrupted tasks found")
		return
	}

	log.Printf("[Bridge] recover: found %d interrupted tasks: %v", len(runningIDs), runningIDs)
	for _, rootID := range runningIDs {
		rid := rootID
		if b.canAccept() {
			b.registerTask(rid, "resume_task")
			go func() {
				defer b.deregisterTask(rid)
				b.handleResumeTask(rid, &ResumeTaskPayload{RootSessionID: rid})
			}()
		} else if b.enqueueOrReject(&queuedTask{
			taskID:    rid,
			taskType:  "resume_task",
			handler:   func() { b.handleResumeTask(rid, &ResumeTaskPayload{RootSessionID: rid}) },
			createdAt: time.Now(),
		}) {
			log.Printf("[Bridge] recover: enqueued %s", rid)
		} else {
			log.Printf("[Bridge] recover: skipped %s (queue full)", rid)
		}
	}
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
