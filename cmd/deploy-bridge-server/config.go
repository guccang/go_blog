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
}

// LoadConfig 从 JSON 文件加载配置，未设置的字段使用默认值
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Listen:         ":9090",
		UploadDir:      "./uploads",
		MaxUploadSizeMB: 200,
		DeployTimeout:  120,
		LogRetainCount: 50,
	}

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
