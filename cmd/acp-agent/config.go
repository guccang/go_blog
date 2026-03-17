package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Port           int               `json:"port"`
	AgentBaseConfig map[string]interface{} `json:"agent_base_config"`
	MaxClients     int               `json:"max_clients"`
	LogLevel       string            `json:"log_level"`
	CloudCode      CloudCodeConfig   `json:"cloud_code"`
	ClientCodeX    ClientCodeXConfig `json:"client_code_x"`
}

type CloudCodeConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	APIKey  string `json:"api_key,omitempty"`
	Timeout int    `json:"timeout"`
}

type ClientCodeXConfig struct {
	Enabled   bool   `json:"enabled"`
	Protocol  string `json:"protocol"`
	Endpoint  string `json:"endpoint"`
	AuthToken string `json:"auth_token,omitempty"`
}

func loadConfig(configPath string) (*Config, error) {
	if !filepath.IsAbs(configPath) {
		exeDir, err := os.Executable()
		if err != nil {
			return nil, err
		}
		configPath = filepath.Join(filepath.Dir(exeDir), configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	if config.Port == 0 {
		config.Port = 8888
	}
	if config.MaxClients == 0 {
		config.MaxClients = 100
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}
	if config.CloudCode.Timeout == 0 {
		config.CloudCode.Timeout = 30
	}
	if config.CloudCode.Host == "" {
		config.CloudCode.Host = "localhost"
	}
	if config.CloudCode.Port == 0 {
		config.CloudCode.Port = 8080
	}
	if config.ClientCodeX.Protocol == "" {
		config.ClientCodeX.Protocol = "ws"
	}

	return &config, nil
}
