package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config Gateway 配置
type Config struct {
	Port         int    `json:"port"`           // 网关监听端口
	GoBackendURL string `json:"go_backend_url"` // go_blog 后端地址（反向代理）
	AuthToken    string `json:"auth_token"`     // agent 认证 token

	// 事件追踪配置
	EventTracking   bool   `json:"event_tracking"`       // 启用追踪（默认 true）
	EventBufferSize int    `json:"event_buffer_size"`    // 缓冲区大小（默认 10000）
	EventLogDir     string `json:"event_log_dir"`        // JSONL 目录（默认 "logs"）
	EventLogStdout  bool   `json:"event_log_stdout"`     // 终端输出（默认 true）
	EventSkipHB     bool   `json:"event_skip_heartbeat"` // 跳过心跳事件（默认 false）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Port:            9000,
		GoBackendURL:    "http://127.0.0.1:8080",
		AuthToken:       "",
		EventTracking:   true,
		EventBufferSize: 10000,
		EventLogDir:     "logs",
		EventLogStdout:  true,
		EventSkipHB:     false,
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

// writeDefaultConfig 将默认配置序列化为 JSON 并写入指定路径
func writeDefaultConfig(path string, cfg interface{}) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("配置文件已存在: %s（不会覆盖）", path)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}
	fmt.Printf("已生成默认配置文件: %s\n", path)
	return nil
}
