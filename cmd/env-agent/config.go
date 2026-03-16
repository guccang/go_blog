package main

import (
	"encoding/json"
	"log"
	"os"
)

// Config env-agent 配置
type Config struct {
	ServerURL      string `json:"server_url"`       // ws://127.0.0.1:10086/ws/uap
	GatewayHTTP    string `json:"gateway_http"`      // http://127.0.0.1:10086
	AuthToken      string `json:"auth_token"`
	AgentName      string `json:"agent_name"`        // "env-agent"
	MaxConcurrent  int    `json:"max_concurrent"`    // 默认 3
	InstallTimeout int    `json:"install_timeout"`   // 预置脚本安装超时秒数，默认 300
	LLMTaskTimeout int    `json:"llm_task_timeout"`  // 委托 LLM 任务超时秒数，默认 600
	LLMAgentID     string `json:"llm_agent_id"`      // llm-mcp-agent 的 agent name，默认 "llm-mcp-agent"
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:      "ws://127.0.0.1:10086/ws/uap",
		GatewayHTTP:    "http://127.0.0.1:10086",
		AgentName:      "env-agent",
		MaxConcurrent:  3,
		InstallTimeout: 300,
		LLMTaskTimeout: 600,
		LLMAgentID:     "llm-mcp-agent",
	}
}

// LoadConfig 从 JSON 文件加载配置
func LoadConfig(path string) *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[Config] 配置文件 %s 不存在，使用默认配置", path)
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		log.Printf("[Config] 解析配置文件失败: %v，使用默认配置", err)
		return DefaultConfig()
	}

	// 填充默认值
	if cfg.AgentName == "" {
		cfg.AgentName = "env-agent"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.InstallTimeout <= 0 {
		cfg.InstallTimeout = 300
	}
	if cfg.LLMTaskTimeout <= 0 {
		cfg.LLMTaskTimeout = 600
	}
	if cfg.LLMAgentID == "" {
		cfg.LLMAgentID = "llm-mcp-agent"
	}

	return cfg
}
