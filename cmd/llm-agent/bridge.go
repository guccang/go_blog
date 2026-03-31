package main

import (
	"encoding/json"
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
	DetailDescription string            // AGENT.md 全文（来自 meta.agent_description）
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

	// 当前会话的 delegation token（从 app-agent 消息中提取）
	delegationToken string

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

	// 活跃 LLM 配置（运行时可切换）
	activeLLM *ActiveLLMState

	// 记忆系统
	memoryMgr       *MemoryManager          // 共享记忆管理器（用于 set_rule 等无账户上下文操作）
	memoryMgrs      map[string]*MemoryManager // 多账户支持：account → MemoryManager（按需创建）
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

	// LLM 调用间隔控制
	lastLLMCall time.Time
	llmCallMu   sync.Mutex
}

// NewBridge 创建 Bridge
func NewBridge(cfg *Config) *Bridge {
	// 设置全局 providers 配置，供 LLM 调用时的自动模型切换使用
	SetProviders(cfg.Providers)

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
		activeLLM:         NewActiveLLMState(cfg.LLM),
		activeTasks:       make(map[string]string),
		taskQueue:         make(chan *queuedTask, cfg.TaskQueueSize),
		queueDone:         make(chan struct{}),
		memoryMgrs:        make(map[string]*MemoryManager),
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

	// 初始化 Skill 管理器（使用共享的 skills 目录）
	if cfg.WorkspaceDir != "" {
		b.skillMgr = NewSkillManager(GetSharedSkillsDir(cfg.WorkspaceDir))
		if err := b.skillMgr.Load(); err != nil {
			log.Printf("[Bridge] load skills: %v", err)
		}
		// 注入 agent 在线检查：技能目录展示时过滤不可用技能
		// 同时检查 agentInfo（DiscoverAgents 填充）和 agentTools（DiscoverTools 填充）
		b.skillMgr.SetAgentOnlineChecker(func(prefix string) bool {
			b.catalogMu.RLock()
			defer b.catalogMu.RUnlock()
			for agentID := range b.agentInfo {
				if strings.HasPrefix(agentID, prefix) {
					return true
				}
			}
			// 回退：DiscoverTools 比 DiscoverAgents 先执行，agentTools 可能已有数据
			for agentID := range b.agentTools {
				if strings.HasPrefix(agentID, prefix) {
					return true
				}
			}
			return false
		})
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
	b.memoryMgrs = make(map[string]*MemoryManager)
	// 创建共享记忆管理器（用于 set_rule 等无账户上下文操作）
	b.memoryMgr = b.createMemoryManager(memoryDir, cfg, "")
	b.memoryCollector = NewMemoryCollector(b.memoryMgr, b, cfg.SkillIterationThreshold)

	// 注册内置工具到统一注册表
	b.registerBuiltinTools()

	return b
}

// Run 启动连接（阻塞，自动重连）
func (b *Bridge) Run() {
	b.client.Run()
}

// Stop 停止
func (b *Bridge) Stop() {
	close(b.queueDone)
	b.client.Stop()
}

// fallbackCooldown 返回配置的降级冷却时长
func (b *Bridge) fallbackCooldown() time.Duration {
	sec := b.cfg.FallbackCooldownSec
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

// ========================= 多账户 MemoryManager 支持 =========================

// createMemoryManager 为指定账户创建 MemoryManager
// account 为空时创建共享的记忆管理器
func (b *Bridge) createMemoryManager(baseMemoryDir string, cfg *Config, account string) *MemoryManager {
	var memoryDir string
	if account != "" {
		// 账户特定的 memory 目录: baseDir/users/{account}/memory
		memoryDir = filepath.Join(baseMemoryDir, "..", "users", account, "memory")
	} else {
		memoryDir = baseMemoryDir
	}

	mgr := NewMemoryManager(memoryDir, cfg.MemoryMaxChars)
	mgr.SetLimits(cfg.MemoryMaxFileChars, cfg.MemoryMaxEntries, cfg.MemoryExpiryDays)

	// 注入 LLM 压缩回调
	mgr.SetLLMCompactFunc(func(entries []MemoryEntry) ([]MemoryEntry, error) {
		return b.llmCompactMemory(entries)
	})

	// 注入 LLM 规则整理回调
	mgr.SetLLMCompactRulesFunc(func(content string) (string, error) {
		return b.llmCompactRules(content)
	})

	// 注入 toolName → skillName 映射回调
	if b.skillMgr != nil {
		mgr.SetSkillNameResolver(func(toolName string) string {
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

	if err := mgr.Load(); err != nil {
		log.Printf("[Bridge] load memory for account=%s: %v", account, err)
	}
	if err := mgr.LoadRules(); err != nil {
		log.Printf("[Bridge] load rules for account=%s: %v", account, err)
	}

	log.Printf("[Bridge] memory manager ready for account=%s dir=%s", account, memoryDir)
	return mgr
}

// GetMemoryManager 获取指定账户的 MemoryManager（按需创建）
func (b *Bridge) GetMemoryManager(account string) *MemoryManager {
	if account == "" {
		return b.memoryMgr
	}

	// 尝试从缓存获取
	if mgr, ok := b.memoryMgrs[account]; ok {
		return mgr
	}

	// 按需创建
	baseMemoryDir := b.cfg.MemoryDir
	if baseMemoryDir == "" {
		baseMemoryDir = "workspace/memory"
	}
	mgr := b.createMemoryManager(baseMemoryDir, b.cfg, account)
	b.memoryMgrs[account] = mgr
	return mgr
}

// ========================= 工具函数 =========================

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
