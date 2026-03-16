package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// LogSource 命名日志源配置
type LogSource struct {
	Path        string `json:"path"`        // 日志目录绝对路径
	Description string `json:"description"` // 描述（注入工具描述，帮助 LLM 理解）
}

// Config log-agent 配置
type Config struct {
	ServerURL  string               `json:"server_url"`
	AuthToken  string               `json:"auth_token"`
	AgentName  string               `json:"agent_name"`
	LogSources map[string]LogSource `json:"log_sources"` // 源名 → 配置
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:  "ws://127.0.0.1:10086/ws/uap",
		AgentName:  "log-agent",
		LogSources: make(map[string]LogSource),
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
		cfg.AgentName = "log-agent"
	}

	return cfg, nil
}
