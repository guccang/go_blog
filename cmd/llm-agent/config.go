package main

import (
	"encoding/json"
	"os"
)

// ToolPolicy 工具权限控制策略
type ToolPolicy struct {
	Allow []string `json:"allow,omitempty"` // 白名单（为空=全部允许）
	Deny  []string `json:"deny,omitempty"`  // 黑名单（优先于白名单）
}

// ToolPolicyPipeline 多层工具策略管道
type ToolPolicyPipeline struct {
	Global    *ToolPolicy `json:"global,omitempty"`     // Layer 1: 全局 allow/deny
	BaseTools []string    `json:"base_tools,omitempty"` // 始终保留的基础工具名
}

// LLMConfig LLM API 配置
type LLMConfig struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// Config llm-agent 配置
type Config struct {
	GatewayURL  string `json:"gateway_url"`  // ws://127.0.0.1:9000/ws/uap
	GatewayHTTP string `json:"gateway_http"` // http://127.0.0.1:9000
	AuthToken   string `json:"auth_token"`
	AgentID     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`

	LLM                 LLMConfig   `json:"llm"`
	Fallbacks           []LLMConfig `json:"fallbacks,omitempty"`
	FallbackCooldownSec int         `json:"fallback_cooldown_sec"` // 模型降级冷却秒数（默认 60）
	MaxPlanRevisions    int         `json:"max_plan_revisions"`    // 最大计划修订次数（默认 3）

	DefaultAccount     string `json:"default_account"`
	ToolCallTimeoutSec int    `json:"tool_call_timeout_sec"`
	LongToolTimeoutSec int    `json:"long_tool_timeout_sec"` // 长时间工具超时秒数（默认 600）
	MaxToolIterations  int    `json:"max_tool_iterations"`
	SystemPromptPrefix string `json:"system_prompt_prefix"`

	// 任务拆解与编排配置
	MaxSubTasks          int    `json:"max_sub_tasks"`           // 最大子任务数（默认 10）
	SubTaskMaxIterations int    `json:"sub_task_max_iterations"` // 子任务最大 agentic loop 轮次（默认 10）
	SubTaskTimeoutSec    int    `json:"sub_task_timeout_sec"`    // 子任务超时秒数（默认 120）
	SessionDir           string `json:"session_dir"`             // 会话持久化目录（默认 agent_sessions）
	WorkspaceDir         string `json:"workspace_dir"`           // 工作区提示文件目录（默认 workspace）

	// 并发控制配置
	MaxConcurrent       int `json:"max_concurrent"`        // 最大并发任务数（默认 3）
	TaskQueueSize       int `json:"task_queue_size"`       // 任务缓冲队列容量（默认 10）
	MaxParallelSubtasks int `json:"max_parallel_subtasks"` // DAG 同层最大并行子任务数（默认 3）

	// 通用会话上下文配置（替代微信专用配置）
	WechatSessionTimeoutMin int `json:"wechat_session_timeout_min"` // 会话超时分钟数（默认 30）
	WechatMaxMessages       int `json:"wechat_max_messages"`        // 单会话最大消息数（默认 40）
	WechatMaxTurns          int `json:"wechat_max_turns"`           // 单会话最大对话轮次（默认 15）
	ChatSessionDir          string `json:"chat_session_dir"`        // 会话持久化目录（默认 chat_sessions）

	// 来源渠道 LLM 配置
	SourceLLMs []SourceLLMConfig `json:"source_llms,omitempty"` // 按来源渠道的 LLM 配置

	// 记忆系统配置
	MemoryDir               string `json:"memory_dir"`                // 记忆目录（默认 workspace/memory）
	SkillIterationThreshold int    `json:"skill_iteration_threshold"` // 同类错误触发 skill 迭代的阈值（默认 3）
	MemoryMaxChars          int    `json:"memory_max_chars"`          // 记忆注入 prompt 的最大字符数（默认 8000）
	MemoryMaxFileChars      int    `json:"memory_max_file_chars"`     // MEMORY.md 文件最大字符数，超过触发压缩（默认 50000）
	MemoryMaxEntries        int    `json:"memory_max_entries"`        // 最大记忆条目数（默认 200）
	MemoryExpiryDays        int    `json:"memory_expiry_days"`        // 记忆过期天数（默认 30）

	// 内置 Bash 工具配置
	BashTimeoutSec     int `json:"bash_timeout_sec"`      // Bash 命令超时秒数（默认 30）
	BashMaxOutputBytes int `json:"bash_max_output_bytes"` // Bash 输出截断字节数（默认 102400）

	// 工具权限控制
	ToolPolicy *ToolPolicy         `json:"tool_policy,omitempty"`
	Pipeline   *ToolPolicyPipeline `json:"tool_pipeline,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		GatewayURL:  "ws://127.0.0.1:9000/ws/uap",
		GatewayHTTP: "http://127.0.0.1:9000",
		AgentID:     "llm-agent",
		AgentName:   "LLM MCP Agent",
		LLM: LLMConfig{
			BaseURL:     "https://api.deepseek.com/v1",
			Model:       "deepseek-chat",
			MaxTokens:   8192,
			Temperature: 0.7,
		},
		FallbackCooldownSec: 60,
		MaxPlanRevisions:    3,

		DefaultAccount:       "ztj",
		ToolCallTimeoutSec:   120,
		LongToolTimeoutSec:   600,
		MaxToolIterations:    32,
		SystemPromptPrefix:   "你是 Go Blog 智能助手，通过企业微信与用户对话。重要规则：1. 收到指令后直接执行，不要反问确认、不要列出方案让用户选择，自行决定最合理的参数并立即调用工具。2. 回复必须精简，控制在500字以内，只输出执行结果和关键数据。适合手机屏幕阅读。",
		MaxSubTasks:          10,
		SubTaskMaxIterations: 10,
		SubTaskTimeoutSec:    120,
		SessionDir:           "agent_sessions",
		WorkspaceDir:         "workspace",

		MaxConcurrent:       3,
		TaskQueueSize:       10,
		MaxParallelSubtasks: 3,

		WechatSessionTimeoutMin: 30,
		WechatMaxMessages:       40,
		WechatMaxTurns:          15,
		ChatSessionDir:          "chat_sessions",

		MemoryDir:               "workspace/memory",
		SkillIterationThreshold: 3,
		MemoryMaxChars:          8000,
		MemoryMaxFileChars:      50000,
		MemoryMaxEntries:        200,
		MemoryExpiryDays:        30,

		BashTimeoutSec:     30,
		BashMaxOutputBytes: 102400,
	}
}

// LoadConfig 从 JSON 文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 迁移旧 ToolPolicy 到 Pipeline
	if cfg.Pipeline == nil && cfg.ToolPolicy != nil {
		cfg.Pipeline = &ToolPolicyPipeline{Global: cfg.ToolPolicy}
	}
	if cfg.Pipeline != nil && len(cfg.Pipeline.BaseTools) == 0 {
		cfg.Pipeline.BaseTools = []string{"ExecuteCode", "Bash"}
	}

	return cfg, nil
}
