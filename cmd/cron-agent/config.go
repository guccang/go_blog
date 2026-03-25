package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config cron-agent 配置
type Config struct {
	ServerURL       string   `json:"server_url"`        // Gateway WebSocket URL
	AuthToken       string   `json:"auth_token"`        // 认证令牌
	AgentName       string   `json:"agent_name"`        // Agent 显示名称
	LLMAgentID      string   `json:"llm_agent_id"`      // 目标 llm-agent ID
	TaskFile        string   `json:"task_file"`         // 任务持久化文件路径
	ProtectedFiles  []string `json:"protected_files,omitempty"` // 部署保护文件
	QuietHoursStart string   `json:"quiet_hours_start"` // 免打扰开始时间，如 "23:00"（空=不启用）
	QuietHoursEnd   string   `json:"quiet_hours_end"`   // 免打扰结束时间，如 "07:00"
	Provider        string   `json:"provider,omitempty"` // LLM provider（如 openai, deepseek）
	Model           string   `json:"model,omitempty"`    // LLM model（如 gpt-4, deepseek-chat）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:      "ws://127.0.0.1:10086/ws/uap",
		AgentName:      "cron-agent",
		LLMAgentID:     "llm-agent",
		TaskFile:       "./cron-tasks.json",
		ProtectedFiles: []string{"cron-agent.json", "cron-tasks.json"},
	}
}

// LoadConfig 从 JSON 文件加载配置
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if cfg.AgentName == "" {
		cfg.AgentName = "cron-agent"
	}
	if cfg.LLMAgentID == "" {
		cfg.LLMAgentID = "llm-agent"
	}
	if cfg.TaskFile == "" {
		cfg.TaskFile = "./cron-tasks.json"
	}

	return cfg, nil
}
