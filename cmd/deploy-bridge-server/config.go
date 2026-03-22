package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config bridge-server 配置
type Config struct {
	Listen         string `json:"listen"`
	AuthToken      string `json:"auth_token"`
	UploadDir      string `json:"upload_dir"`
	MaxUploadSizeMB int   `json:"max_upload_size_mb"`
	DeployTimeout  int    `json:"deploy_timeout_sec"`
	LogRetainCount int    `json:"log_retain_count"`

	// 部署保护文件（deploy-agent 增量部署时跳过这些文件）
	ProtectedFiles []string `json:"protected_files,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Listen:          ":9090",
		UploadDir:       "./uploads",
		MaxUploadSizeMB: 200,
		DeployTimeout:   120,
		LogRetainCount:  50,

		ProtectedFiles: []string{"bridge-server.json", "uploads/"},
	}
}

// LoadConfig 从 JSON 文件加载配置，未设置的字段使用默认值
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %v", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}

	if cfg.AuthToken == "" {
		return nil, fmt.Errorf("auth_token is required (empty token is not allowed)")
	}

	// 确保 upload_dir 存在
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload_dir: %v", err)
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
