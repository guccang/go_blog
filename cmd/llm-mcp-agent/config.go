package main

import (
	"encoding/json"
	"os"
)

// LLMConfig LLM API 配置
type LLMConfig struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// Config llm-mcp-agent 配置
type Config struct {
	GatewayURL  string `json:"gateway_url"`  // ws://127.0.0.1:9000/ws/uap
	GatewayHTTP string `json:"gateway_http"` // http://127.0.0.1:9000
	AuthToken   string `json:"auth_token"`
	AgentID     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`

	LLM LLMConfig `json:"llm"`

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

	// 微信对话上下文配置
	WechatSessionTimeoutMin int `json:"wechat_session_timeout_min"` // 会话超时分钟数（默认 30）
	WechatMaxMessages       int `json:"wechat_max_messages"`        // 单会话最大消息数（默认 40）
	WechatMaxTurns          int `json:"wechat_max_turns"`           // 单会话最大对话轮次（默认 15）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		GatewayURL:  "ws://127.0.0.1:9000/ws/uap",
		GatewayHTTP: "http://127.0.0.1:9000",
		AgentID:     "llm-mcp",
		AgentName:   "LLM MCP Agent",
		LLM: LLMConfig{
			BaseURL:     "https://api.deepseek.com/v1",
			Model:       "deepseek-chat",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
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

		WechatSessionTimeoutMin: 30,
		WechatMaxMessages:       40,
		WechatMaxTurns:          15,
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
	return cfg, nil
}
