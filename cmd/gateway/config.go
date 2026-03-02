package main

import (
	"encoding/json"
	"os"
)

// Config Gateway 配置
type Config struct {
	Port         int    `json:"port"`           // 网关监听端口
	GoBackendURL string `json:"go_backend_url"` // go_blog 后端地址（反向代理）
	AuthToken    string `json:"auth_token"`     // agent 认证 token
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Port:         9000,
		GoBackendURL: "http://127.0.0.1:8080",
		AuthToken:    "",
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
