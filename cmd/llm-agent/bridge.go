package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	HostPlatform     string            // 运行平台（macOS/Linux/Windows）
	HostIP           string            // 主机 IP 地址
	Workspace        string            // 工作目录
	Models           []string          // 合并后的模型配置名列表（如 default, deepseek）
	ClaudeCodeModels []string          // Claude Code 可用配置
	OpenCodeModels   []string          // OpenCode 可用配置
	CodingTools      []string          // 可用编码工具（claudecode, opencode）
	SSHHosts         []string          // 可用 SSH 主机
	DeployTargets    []string          // 部署目标
	TargetHosts      map[string]string // target 名→SSH host 映射（如 ssh-prod → root@114.115.214.86）
	Pipelines        []string          // 可用 pipeline
	PythonVersion    string            // Python 版本（execute-code-agent）
	MaxExecTime      int               // 最大执行时间秒（execute-code-agent）
	LogSources       map[string]string // 日志源名→描述（log-agent）
	SupportedSoftware []string         // 支持检测/安装的软件列表（env-agent）
	HostStats         map[string]any   // 主机资源信息（cpu_cores, mem_total_gb, disk_total_gb, disk_free_gb）
}

// Bridge UAP 客户端 + 工具路由层
type Bridge struct {
	cfg    *Config
	client *uap.Client

	// 统一工具注册表（Phase 4 替代 toolCatalog）
	toolHandlers map[string]ToolHandler // canonicalName → handler
	toolNameMap  map[string]string      // 任意名称变体 → canonicalName

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

	// 工具调用进度转发（deploy-agent 等发送的 tool_progress 事件）
	toolProgressSinks map[string]EventSink // msgID → sink
	toolProgressMu    sync.Mutex

	// Claude Mode: 流式输出 sink（sessionKey → sink）
	claudeSinks   map[string]*claudeStreamSink
	claudeSinksMu sync.Mutex

	// 通用会话上下文管理（替代微信专用）
	sessionMgr *ChatSessionManager

	// 来源渠道 LLM 配置
	sourceLLMs map[string]*SourceLLMConfig // source → config

	// 记忆系统
	memoryMgr       *MemoryManager
	memoryCollector *MemoryCollector

	// 人设配置
	persona *PersonaProfile

	// Skill 管理器
	skillMgr *SkillManager

	// 内置工具
	bashManager *BashToolManager

	// 任务生命周期 hook
	hooks *HookManager

	// Token 用量统计
	tokenStats *TokenStats

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
	client.Tools = nil // llm-agent 不对外注册工具
	client.HostPlatform = detectPlatform()
	client.HostIP = getLocalIP()
	if cfg.WorkspaceDir != "" {
		client.Workspace = cfg.WorkspaceDir
	} else if wd, err := os.Getwd(); err == nil {
		client.Workspace = wd
	}

	// 采集主机资源信息写入 Meta
	if client.Meta == nil {
		client.Meta = make(map[string]any)
	}
	client.Meta["host_stats"] = collectHostStats(client.Workspace)

	// 初始化通用会话管理器
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
	chatSessionDir := cfg.ChatSessionDir
	if chatSessionDir == "" {
		chatSessionDir = "chat_sessions"
	}

	// 构建 source → LLM 配置映射
	sourceLLMs := make(map[string]*SourceLLMConfig, len(cfg.SourceLLMs))
	for i := range cfg.SourceLLMs {
		sourceLLMs[cfg.SourceLLMs[i].Source] = &cfg.SourceLLMs[i]
	}

	b := &Bridge{
		cfg:               cfg,
		client:            client,
		toolHandlers:      make(map[string]ToolHandler),
		toolNameMap:       make(map[string]string),
		toolCatalog:       make(map[string]string),
		agentInfo:         make(map[string]AgentInfo),
		agentTools:        make(map[string][]LLMTool),
		pending:           make(map[string]chan *toolResultWithFrom),
		toolProgressSinks: make(map[string]EventSink),
		claudeSinks:       make(map[string]*claudeStreamSink),
		sessionMgr:        NewChatSessionManager(timeout, maxMessages, maxTurns, chatSessionDir),
		sourceLLMs:        sourceLLMs,
		activeTasks:       make(map[string]string),
		taskQueue:         make(chan *queuedTask, cfg.TaskQueueSize),
		queueDone:         make(chan struct{}),
	}

	client.OnMessage = b.handleMessage

	// 初始化 token 用量统计
	tokenStatsPath := filepath.Join(cfg.SessionDir, "token_stats.json")
	b.tokenStats = NewTokenStats(tokenStatsPath)
	b.tokenStats.Load()
	SetTokenStats(b.tokenStats)

	// 初始化 hook 管理器
	b.hooks = NewHookManager()
	b.hooks.Register(&WechatUsageSummaryHook{bridge: b})

	// 初始化内置 Bash 工具
	bashTimeout := time.Duration(cfg.BashTimeoutSec) * time.Second
	if bashTimeout <= 0 {
		bashTimeout = 30 * time.Second
	}
	bashMaxOutput := cfg.BashMaxOutputBytes
	if bashMaxOutput <= 0 {
		bashMaxOutput = 102400
	}
	b.bashManager = &BashToolManager{
		Timeout:   bashTimeout,
		MaxOutput: bashMaxOutput,
		AgentID:   cfg.AgentID,
		Platform:  detectPlatform(),
	}

	// 初始化 Skill 管理器
	if cfg.WorkspaceDir != "" {
		b.skillMgr = NewSkillManager(cfg.WorkspaceDir)
		if err := b.skillMgr.Load(); err != nil {
			log.Printf("[Bridge] load skills: %v", err)
		}
	}

	// 初始化人设配置
	if cfg.WorkspaceDir != "" {
		personaContent := loadWorkspaceFile(cfg.WorkspaceDir, "PERSONA.md", cfg.SystemPromptPrefix)
		b.persona = ParsePersonaFile(personaContent)
		b.persona.FilePath = filepath.Join(cfg.WorkspaceDir, "PERSONA.md")
		log.Printf("[Bridge] persona loaded: configured=%v name=%s", b.persona.IsConfigured(), b.persona.Name)
	}

	// 初始化记忆系统
	memoryDir := cfg.MemoryDir
	if memoryDir == "" {
		memoryDir = "workspace/memory"
	}
	b.memoryMgr = NewMemoryManager(memoryDir, cfg.MemoryMaxChars)
	b.memoryMgr.SetLimits(cfg.MemoryMaxFileChars, cfg.MemoryMaxEntries, cfg.MemoryExpiryDays)

	// 注入 LLM 压缩回调：超限时用 LLM 整理记忆，保留摘要和重要内容
	b.memoryMgr.SetLLMCompactFunc(func(entries []MemoryEntry) ([]MemoryEntry, error) {
		return b.llmCompactMemory(entries)
	})

	// 注入 LLM 规则整理回调：去重合并用户规则
	b.memoryMgr.SetLLMCompactRulesFunc(func(content string) (string, error) {
		return b.llmCompactRules(content)
	})

	// 注入 toolName → skillName 映射回调（用于 auto_skill 分流）
	if b.skillMgr != nil {
		b.skillMgr.SetMemoryDir(memoryDir)
		b.memoryMgr.SetSkillNameResolver(func(toolName string) string {
			for _, skill := range b.skillMgr.GetAllSkills() {
				for _, t := range skill.Tools {
					if t == toolName || strings.Contains(t, toolName) {
						return skill.Name
					}
				}
			}
			return ""
		})
	}

	if err := b.memoryMgr.Load(); err != nil {
		log.Printf("[Bridge] load memory: %v", err)
	}
	if err := b.memoryMgr.LoadRules(); err != nil {
		log.Printf("[Bridge] load rules: %v", err)
	}
	b.memoryCollector = NewMemoryCollector(b.memoryMgr, b, cfg.SkillIterationThreshold)

	// 注册内置工具到统一注册表
	b.registerBuiltinTools()

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

// GetLLMConfigForSource 返回指定来源渠道的 LLM 配置（primary + fallbacks）
// 无配置则返回全局默认
func (b *Bridge) GetLLMConfigForSource(source string) (*LLMConfig, []LLMConfig) {
	if sc, ok := b.sourceLLMs[source]; ok {
		return &sc.LLM, sc.Fallbacks
	}
	return &b.cfg.LLM, b.cfg.Fallbacks
}

// sendLLMWithConfig 使用指定配置的同步 LLM 请求
func (b *Bridge) sendLLMWithConfig(cfg *LLMConfig, fallbacks []LLMConfig, messages []Message, tools []LLMTool) (string, []ToolCall, error) {
	if len(fallbacks) == 0 {
		return SendLLMRequest(cfg, messages, tools)
	}
	return SendLLMRequestWithFallback(cfg, fallbacks, b.fallbackCooldown(), messages, tools)
}

// sendStreamingLLMWithConfig 使用指定配置的流式 LLM 请求
func (b *Bridge) sendStreamingLLMWithConfig(cfg *LLMConfig, fallbacks []LLMConfig, messages []Message, tools []LLMTool, onChunk func(string)) (string, []ToolCall, error) {
	if len(fallbacks) == 0 {
		return SendStreamingLLMRequest(cfg, messages, tools, onChunk)
	}
	return SendStreamingLLMRequestWithFallback(cfg, fallbacks, b.fallbackCooldown(), messages, tools, onChunk)
}

// llmCompactMemory 使用 LLM 整理记忆：合并重复、提取模式、保留重要摘要
func (b *Bridge) llmCompactMemory(entries []MemoryEntry) ([]MemoryEntry, error) {
	// 构建当前记忆文本
	var memoryText strings.Builder
	for _, entry := range entries {
		memoryText.WriteString(fmt.Sprintf("[%s][%s] %s: %s\n", entry.Date, entry.Category, entry.Source, entry.Content))
	}

	prompt := fmt.Sprintf(`你是一个记忆整理助手。以下是 AI Agent 积累的 %d 条工作记忆，需要压缩整理。

规则：
1. 合并重复的错误记录，只保留一条并注明出现次数
2. 将多条相关错误提炼为一条 [pattern] 类型的经验总结
3. [solution] [pattern] [preference] 类型的记忆优先保留完整内容
4. [error] 类型只保留有代表性的，删除重复的
5. [auto_skill] 类型全部保留
6. 目标：压缩到 %d 条以内

输出格式（每条一行，严格遵循）：
[日期][类别] 来源: 内容

类别只能是: error, solution, pattern, preference, auto_skill
日期格式: 2006-01-02

当前记忆：
%s`, len(entries), len(entries)*2/3, memoryText.String())

	messages := []Message{
		{Role: "system", Content: "你是记忆整理助手，负责压缩和整理 AI Agent 的工作记忆。只输出整理后的记忆条目，不要输出其他内容。"},
		{Role: "user", Content: prompt},
	}

	text, _, err := b.sendLLM(messages, nil)
	if err != nil {
		return nil, fmt.Errorf("LLM compact: %v", err)
	}

	// 解析 LLM 输出为 MemoryEntry
	compacted := parseLLMCompactOutput(text)
	if len(compacted) == 0 {
		return nil, fmt.Errorf("LLM compact returned empty result")
	}

	log.Printf("[Memory] LLM 整理: %d → %d 条", len(entries), len(compacted))
	return compacted, nil
}

// llmCompactRules 使用 LLM 整理用户规则：去重、合并、精简
func (b *Bridge) llmCompactRules(content string) (string, error) {
	prompt := fmt.Sprintf(`你是一个规则整理助手。以下是用户给 AI 助手设定的规则和提醒，其中可能有重复或相似的内容。

请整理这些规则：
1. 合并含义相同或相似的规则
2. 删除完全重复的
3. 保持原始意图不变
4. 语言精简清晰

只输出整理后的规则内容，不要输出其他说明。

当前规则：
%s`, content)

	messages := []Message{
		{Role: "system", Content: "你是规则整理助手，负责去重合并用户设定的 AI 助手行为规则。只输出整理后的规则。"},
		{Role: "user", Content: prompt},
	}

	text, _, err := b.sendLLM(messages, nil)
	if err != nil {
		return "", fmt.Errorf("LLM compact rules: %v", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("LLM compact rules returned empty")
	}

	log.Printf("[Memory] LLM 规则整理: %d → %d 字符", len(content), len(text))
	return text, nil
}

// parseLLMCompactOutput 解析 LLM 压缩输出
// 格式: [2026-03-19][pattern] tool_call: 内容
func parseLLMCompactOutput(text string) []MemoryEntry {
	var entries []MemoryEntry
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "[") {
			continue
		}

		// 解析 [date][category] source: content
		closeDateBracket := strings.Index(line[1:], "]")
		if closeDateBracket < 0 {
			continue
		}
		date := line[1 : closeDateBracket+1]

		rest := line[closeDateBracket+2:]
		if !strings.HasPrefix(rest, "[") {
			continue
		}

		closeCatBracket := strings.Index(rest[1:], "]")
		if closeCatBracket < 0 {
			continue
		}
		category := rest[1 : closeCatBracket+1]

		afterCat := strings.TrimSpace(rest[closeCatBracket+2:])
		source := "unknown"
		content := afterCat
		if colonIdx := strings.Index(afterCat, ":"); colonIdx > 0 {
			source = strings.TrimSpace(afterCat[:colonIdx])
			content = strings.TrimSpace(afterCat[colonIdx+1:])
		}

		if content != "" {
			entries = append(entries, MemoryEntry{
				Date:     date,
				Category: category,
				Source:   source,
				Content:  content,
			})
		}
	}

	return entries
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

	// 注册远程工具到统一注册表
	for toolName, agentID := range catalog {
		b.registerRemoteToolLocked(toolName, agentID)
	}

	b.catalogMu.Unlock()

	if len(llmTools) != prevCount {
		log.Printf("[Bridge] discovered %d unique tools from %d entries (was %d). Tools: %v", len(llmTools), len(result.Tools), prevCount, toolNames)
	}

	// 应用工具权限策略
	b.applyToolPolicy()

	// 合并内置工具（Bash）到 llmTools（用于 LLM function calling）
	// 注意：handler 已在 registerBuiltinTools 中注册到统一注册表
	if b.bashManager != nil {
		b.catalogMu.Lock()
		for _, tool := range b.bashManager.ToolDefs() {
			exists := false
			for _, t := range b.llmTools {
				if t.Function.Name == tool.Function.Name {
					exists = true
					break
				}
			}
			if !exists {
				b.llmTools = append(b.llmTools, tool)
			}
		}
		b.catalogMu.Unlock()
	}

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
		originalName := name
		if cn, ok := b.toolNameMap[name]; ok {
			originalName = cn
		}

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
			originalName := name
			if cn, ok := b.toolNameMap[name]; ok {
				originalName = cn
			}
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
			AgentID      string         `json:"agent_id"`
			AgentType    string         `json:"agent_type"`
			Name         string         `json:"name"`
			Description  string         `json:"description"`
			HostPlatform string         `json:"host_platform"`
			HostIP       string         `json:"host_ip"`
			Workspace    string         `json:"workspace"`
			Tools        []string       `json:"tools"`
			Meta         map[string]any `json:"meta"`
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
			ID:           a.AgentID,
			Name:         a.Name,
			Description:  a.Description,
			ToolNames:    a.Tools,
			HostPlatform: a.HostPlatform,
			HostIP:       a.HostIP,
			Workspace:    a.Workspace,
		}
		// 从 meta 提取动态能力信息
		if a.Meta != nil {
			info.Models = parseStringSlice(a.Meta["models"])
			info.ClaudeCodeModels = parseStringSlice(a.Meta["claudecode_models"])
			info.OpenCodeModels = parseStringSlice(a.Meta["opencode_models"])
			info.CodingTools = parseStringSlice(a.Meta["coding_tools"])
			// 兼容旧 agent：base 字段为空时从 meta 回退
			if info.HostPlatform == "" {
				if hp, ok := a.Meta["host_platform"].(string); ok {
					info.HostPlatform = hp
				}
			}
			info.SSHHosts = parseStringSlice(a.Meta["ssh_hosts"])
			info.DeployTargets = parseStringSlice(a.Meta["deploy_targets"])
			info.TargetHosts = parseStringMap(a.Meta["target_hosts"])
			info.Pipelines = parseStringSlice(a.Meta["pipelines"])
			if pv, ok := a.Meta["python_version"].(string); ok {
				info.PythonVersion = pv
			}
			if met, ok := a.Meta["max_exec_time"].(float64); ok {
				info.MaxExecTime = int(met)
			}
			info.LogSources = parseStringMap(a.Meta["log_sources"])
			info.SupportedSoftware = parseStringSlice(a.Meta["supported_software"])
			if hs, ok := a.Meta["host_stats"].(map[string]interface{}); ok {
				info.HostStats = make(map[string]any, len(hs))
				for k, v := range hs {
					info.HostStats[k] = v
				}
			}
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

// parseStringMap 从 any (interface{}) 解析 map[string]string，兼容 JSON 反序列化的 map[string]interface{}
func parseStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			result[k] = s
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

	// 注入 llm-agent 自身信息
	sb.WriteString(fmt.Sprintf("- **%s** (%s): LLM 编排中枢\n", b.cfg.AgentName, b.cfg.AgentID))
	if b.client.HostPlatform != "" {
		sb.WriteString(fmt.Sprintf("  - 运行平台: %s\n", b.client.HostPlatform))
	}
	if b.client.HostIP != "" {
		sb.WriteString(fmt.Sprintf("  - 主机IP: %s\n", b.client.HostIP))
	}
	if b.client.Workspace != "" {
		sb.WriteString(fmt.Sprintf("  - 工作目录: %s\n", b.client.Workspace))
	}

	for _, info := range b.agentInfo {
		// 标题行：有 description 就显示，没有就只显示名称
		if info.Description != "" {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", info.Name, info.ID, info.Description))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", info.Name, info.ID))
		}
		// 注入基础信息
		if info.HostPlatform != "" {
			sb.WriteString(fmt.Sprintf("  - 运行平台: %s\n", info.HostPlatform))
		}
		if info.HostIP != "" {
			sb.WriteString(fmt.Sprintf("  - 主机IP: %s\n", info.HostIP))
		}
		if info.Workspace != "" {
			sb.WriteString(fmt.Sprintf("  - 工作目录: %s\n", info.Workspace))
		}
		// 注入可用模型和编码工具信息，让 LLM 知道合法参数值
		if len(info.CodingTools) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用编码工具(tool参数): %s\n", strings.Join(info.CodingTools, ", ")))
		}
		if len(info.Models) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用模型配置(model参数): %s\n", strings.Join(info.Models, ", ")))
		}
		if len(info.SSHHosts) > 0 {
			sb.WriteString(fmt.Sprintf("  - SSH主机: %s\n", strings.Join(info.SSHHosts, ", ")))
		}
		if len(info.DeployTargets) > 0 {
			sb.WriteString(fmt.Sprintf("  - 部署目标(deploy_target参数): %s\n", strings.Join(info.DeployTargets, ", ")))
		}
		if len(info.TargetHosts) > 0 {
			sb.WriteString("  - 部署目标对应SSH地址(ssh_host参数):\n")
			for target, host := range info.TargetHosts {
				sb.WriteString(fmt.Sprintf("    - %s → %s\n", target, host))
			}
		}
		if len(info.Pipelines) > 0 {
			sb.WriteString(fmt.Sprintf("  - Pipeline: %s\n", strings.Join(info.Pipelines, ", ")))
		}
		if info.PythonVersion != "" {
			sb.WriteString(fmt.Sprintf("  - Python版本: %s", info.PythonVersion))
			if info.MaxExecTime > 0 {
				sb.WriteString(fmt.Sprintf(", 执行超时: %ds", info.MaxExecTime))
			}
			sb.WriteString("\n")
		}
		if len(info.LogSources) > 0 {
			sb.WriteString("  - 可查日志源(source参数):\n")
			for name, desc := range info.LogSources {
				sb.WriteString(fmt.Sprintf("    - %s: %s\n", name, desc))
			}
		}
		if len(info.SupportedSoftware) > 0 {
			sb.WriteString(fmt.Sprintf("  - 支持检测/安装的软件(software参数): %s\n", strings.Join(info.SupportedSoftware, ", ")))
		}
		if len(info.HostStats) > 0 {
			var parts []string
			if v, ok := info.HostStats["cpu_cores"]; ok {
				parts = append(parts, fmt.Sprintf("CPU %v核", v))
			}
			if v, ok := info.HostStats["mem_total_gb"]; ok {
				parts = append(parts, fmt.Sprintf("内存 %sGB", v))
			}
			if total, ok := info.HostStats["disk_total_gb"]; ok {
				if free, ok2 := info.HostStats["disk_free_gb"]; ok2 {
					parts = append(parts, fmt.Sprintf("磁盘 %sGB/可用 %sGB", total, free))
				} else {
					parts = append(parts, fmt.Sprintf("磁盘 %sGB", total))
				}
			}
			if len(parts) > 0 {
				sb.WriteString(fmt.Sprintf("  - 主机资源: %s\n", strings.Join(parts, ", ")))
			}
		}
	}
	return sb.String()
}

// getFilteredAgentDescriptionBlock 按当前工具集过滤 agent 描述（仅描述涉及的 agent）
// skillAgents: agent_id → skill_name，被 skill 接管的 agent 仍然显示，但标注通过哪个 skill 访问
// 过滤后为空则回退全量
func (b *Bridge) getFilteredAgentDescriptionBlock(tools []LLMTool, skillAgents map[string]string) string {
	b.catalogMu.RLock()
	toolCatalogCopy := make(map[string]string, len(b.toolCatalog))
	for k, v := range b.toolCatalog {
		toolCatalogCopy[k] = v
	}
	agentInfoCopy := make(map[string]AgentInfo, len(b.agentInfo))
	for k, v := range b.agentInfo {
		agentInfoCopy[k] = v
	}
	b.catalogMu.RUnlock()

	if len(agentInfoCopy) == 0 {
		return ""
	}

	// 从当前 tools 找出涉及的 agent ID
	involvedAgents := make(map[string]bool)
	for _, tool := range tools {
		originalName := b.resolveToolName(tool.Function.Name)
		if agentID, ok := toolCatalogCopy[originalName]; ok {
			involvedAgents[agentID] = true
		} else if agentID, ok := toolCatalogCopy[tool.Function.Name]; ok {
			involvedAgents[agentID] = true
		}
	}

	// 过滤后为空且无 skillAgents 则回退全量
	if len(involvedAgents) == 0 && len(skillAgents) == 0 {
		return b.getAgentDescriptionBlock()
	}

	var sb strings.Builder
	sb.WriteString("\n## 可用 Agent 能力\n")
	count := 0

	// 先显示工具直接可见的 agent（involvedAgents）
	for _, info := range agentInfoCopy {
		if !involvedAgents[info.ID] {
			continue
		}
		if info.Description == "" {
			continue
		}
		count++
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", info.Name, info.ID, info.Description))
		if len(info.CodingTools) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用编码工具(tool参数): %s\n", strings.Join(info.CodingTools, ", ")))
		}
		if len(info.Models) > 0 {
			sb.WriteString(fmt.Sprintf("  - 可用模型配置(model参数): %s\n", strings.Join(info.Models, ", ")))
		}
		if info.HostPlatform != "" {
			sb.WriteString(fmt.Sprintf("  - 运行平台: %s\n", info.HostPlatform))
		}
		if len(info.SSHHosts) > 0 {
			sb.WriteString(fmt.Sprintf("  - SSH主机: %s\n", strings.Join(info.SSHHosts, ", ")))
		}
		if len(info.DeployTargets) > 0 {
			sb.WriteString(fmt.Sprintf("  - 部署目标(deploy_target参数): %s\n", strings.Join(info.DeployTargets, ", ")))
		}
		if len(info.TargetHosts) > 0 {
			sb.WriteString("  - 部署目标对应SSH地址(ssh_host参数):\n")
			for target, host := range info.TargetHosts {
				sb.WriteString(fmt.Sprintf("    - %s → %s\n", target, host))
			}
		}
		if len(info.Pipelines) > 0 {
			sb.WriteString(fmt.Sprintf("  - Pipeline: %s\n", strings.Join(info.Pipelines, ", ")))
		}
		if info.PythonVersion != "" {
			sb.WriteString(fmt.Sprintf("  - Python版本: %s", info.PythonVersion))
			if info.MaxExecTime > 0 {
				sb.WriteString(fmt.Sprintf(", 执行超时: %ds", info.MaxExecTime))
			}
			sb.WriteString("\n")
		}
		if len(info.LogSources) > 0 {
			sb.WriteString("  - 可查日志源(source参数):\n")
			for name, desc := range info.LogSources {
				sb.WriteString(fmt.Sprintf("    - %s: %s\n", name, desc))
			}
		}
		if len(info.SupportedSoftware) > 0 {
			sb.WriteString(fmt.Sprintf("  - 支持检测/安装的软件(software参数): %s\n", strings.Join(info.SupportedSoftware, ", ")))
		}
	}

	// 再显示被 skill 接管、但不在 involvedAgents 里的 agent（简要信息 + skill 指针）
	for agentID, skillName := range skillAgents {
		if involvedAgents[agentID] {
			continue // 已经在上面显示过了
		}
		info, ok := agentInfoCopy[agentID]
		if !ok || info.Description == "" {
			continue
		}
		count++
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s → 通过 execute_skill(\"%s\") 使用\n", info.Name, info.ID, info.Description, skillName))
		if info.HostPlatform != "" {
			sb.WriteString(fmt.Sprintf("  - 运行平台: %s\n", info.HostPlatform))
		}
	}

	// 过滤后无有效 agent，回退全量
	if count == 0 {
		return b.getAgentDescriptionBlock()
	}

	return sb.String()
}

// executeCodeAgentType execute-code-agent 的类型标识（元工具，始终保留不参与路由筛选）
const executeCodeAgentType = "execute_code"

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

// ========================= 跨 Agent 工具调用 =========================

// longRunningTools 需要长超时的工具（编码、部署等耗时操作）
var longRunningTools = map[string]bool{
	"CodegenStartSession": true,
	"CodegenSendMessage":  true,
	"AcpStartSession":     true,
	"AcpSendMessage":      true,
	"AcpAnalyzeProject":   true,
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

// CallTool 统一工具调用入口（无 context）
func (b *Bridge) CallTool(toolName string, args json.RawMessage) (*ToolCallResult, error) {
	return b.DispatchTool(context.Background(), toolName, args, nil)
}

// CallToolCtx context 感知的工具调用，支持级联取消
func (b *Bridge) CallToolCtx(ctx context.Context, toolName string, args json.RawMessage) (*ToolCallResult, error) {
	return b.DispatchTool(ctx, toolName, args, nil)
}

// CallToolCtxWithProgress context 感知的工具调用，支持进度回调转发
func (b *Bridge) CallToolCtxWithProgress(ctx context.Context, toolName string, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	return b.DispatchTool(ctx, toolName, args, sink)
}

// callRemoteAgent 发送 tool_call 到远程 agent 并等待 MsgToolResult
// 从原 callToolCtxWithSink 提取，纯 UAP 消息收发
func (b *Bridge) callRemoteAgent(ctx context.Context, toolName, agentID string, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("cancelled before tool call %s: %v", toolName, err)
		}
	} else {
		ctx = context.Background()
	}

	msgID := uap.NewMsgID()
	ch := make(chan *toolResultWithFrom, 1)

	b.pendMu.Lock()
	b.pending[msgID] = ch
	b.pendMu.Unlock()

	// 注册进度回调 sink（deploy-agent 的 tool_progress 会通过 msgID 关联）
	if sink != nil {
		b.toolProgressMu.Lock()
		b.toolProgressSinks[msgID] = sink
		b.toolProgressMu.Unlock()
	}

	defer func() {
		b.pendMu.Lock()
		delete(b.pending, msgID)
		b.pendMu.Unlock()
		if sink != nil {
			b.toolProgressMu.Lock()
			delete(b.toolProgressSinks, msgID)
			b.toolProgressMu.Unlock()
		}
	}()

	log.Printf("[Bridge] tool_call → agent=%s tool=%s msgID=%s", agentID, toolName, msgID)

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
		} else if payload.Channel == "acp_stream" {
			// Claude Mode: acp-agent 发来的流式事件
			b.handleACPStreamEvent(payload)
		} else if payload.Channel == "tool_progress" {
			// deploy-agent 等发送的工具执行进度，payload.To 是工具调用 msgID
			b.toolProgressMu.Lock()
			sink, ok := b.toolProgressSinks[payload.To]
			b.toolProgressMu.Unlock()
			if ok {
				sink.OnEvent("tool_progress", payload.Content)
			} else {
				log.Printf("[Bridge] tool_progress for unknown msgID=%s: %s", payload.To, payload.Content)
			}
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

	case uap.MsgPermissionRequest:
		// Claude Mode: acp-agent 发来的权限请求
		var payload uap.PermissionRequestPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("[Bridge] invalid permission_request payload: %v", err)
			return
		}
		b.handlePermissionRequest(msg.From, payload)

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
		case "cron_reminder":
			var wrapper struct {
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(taskPayload.Payload, &wrapper); err != nil {
				log.Printf("[Bridge] invalid cron_reminder payload: %v", err)
				return
			}
			var reminderPayload CronReminderPayload
			if err := json.Unmarshal(wrapper.Payload, &reminderPayload); err != nil {
				log.Printf("[Bridge] invalid cron_reminder inner payload: %v", err)
				return
			}
			sourceAgent := msg.From
			handler = func() { b.handleCronReminder(taskPayload.TaskID, sourceAgent, &reminderPayload) }
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

// StartSessionCleanupLoop 后台定时清理过期会话（替代 StartWechatCleanupLoop）
func (b *Bridge) StartSessionCleanupLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			b.sessionMgr.CleanupExpired()
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

// ========================= Claude Mode 事件处理 =========================

// handleACPStreamEvent 处理 acp-agent 发来的流式事件（Claude Mode）
func (b *Bridge) handleACPStreamEvent(payload uap.NotifyPayload) {
	// payload.Content 是 JSON 序列化的 StreamEventPayload
	var evt StreamEventPayload
	if err := json.Unmarshal([]byte(payload.Content), &evt); err != nil {
		log.Printf("[Bridge] invalid acp_stream payload: %v", err)
		return
	}

	// 通过 ClaudeSessionID 反查对应的 wechat user session key
	var sinkKey string
	b.sessionMgr.mu.RLock()
	for key, session := range b.sessionMgr.sessions {
		if session.ClaudeMode && session.ClaudeSessionID == evt.SessionID {
			sinkKey = key
			break
		}
	}
	b.sessionMgr.mu.RUnlock()

	// 查找对应的 claude stream sink
	b.claudeSinksMu.Lock()
	sink, ok := b.claudeSinks[sinkKey]
	if !ok {
		// fallback: 尝试任意一个 sink
		for _, s := range b.claudeSinks {
			sink = s
			break
		}
	}
	b.claudeSinksMu.Unlock()

	if sink == nil {
		log.Printf("[Bridge] no claude sink for acp_stream event session=%s", evt.SessionID)
		return
	}

	sink.onStreamEvent(evt)
}

// handlePermissionRequest 处理 acp-agent 发来的权限请求（Claude Mode 交互模式）
func (b *Bridge) handlePermissionRequest(acpAgentID string, payload uap.PermissionRequestPayload) {
	log.Printf("[Bridge] permission_request: session=%s title=%s options=%d", payload.SessionID, payload.Title, len(payload.Options))

	// 通过 sessionID 反查 wechat user
	var targetSession *ChatSession
	var fromAgent, wechatUser string

	b.sessionMgr.mu.RLock()
	for _, session := range b.sessionMgr.sessions {
		if session.ClaudeMode && session.ClaudeSessionID == payload.SessionID {
			targetSession = session
			fromAgent = session.ClaudeFromAgent
			wechatUser = session.UserID
			break
		}
	}
	b.sessionMgr.mu.RUnlock()

	if targetSession == nil {
		log.Printf("[Bridge] no session found for permission request session=%s", payload.SessionID)
		return
	}

	// 构建权限选项信息
	options := make([]PermOptionInfo, len(payload.Options))
	for i, opt := range payload.Options {
		options[i] = PermOptionInfo{
			Index:    opt.Index,
			OptionID: opt.OptionID,
			Name:     opt.Name,
			Kind:     opt.Kind,
		}
	}

	// 设置 pending permission
	targetSession.SetPendingPermission(&PendingPermission{
		RequestID:  payload.RequestID,
		SessionID:  payload.SessionID,
		ACPAgentID: acpAgentID,
		Options:    options,
	})

	// 构建可读消息发给微信用户
	var sb strings.Builder
	sb.WriteString("🔒 请求授权\n")
	sb.WriteString(fmt.Sprintf("操作: %s\n", payload.Title))
	if payload.Content != "" {
		content := payload.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(content + "\n")
	}
	sb.WriteString("\n")
	for _, opt := range payload.Options {
		sb.WriteString(fmt.Sprintf("%d. %s\n", opt.Index, opt.Name))
	}
	sb.WriteString("\n回复数字或 y/n")

	b.sendWechat(fromAgent, wechatUser, sb.String())
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

// detectPlatform 检测当前运行平台
func detectPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

// getLocalIP 获取本机局域网 IP 地址
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}
