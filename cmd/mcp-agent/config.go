package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
)

// MCPServerConfig 单个 MCP Server 配置
type MCPServerConfig struct {
	Transport string            `json:"transport"` // "stdio" | "http"
	Command   string            `json:"command"`   // stdio: 启动命令
	Args      []string          `json:"args"`      // stdio: 命令参数
	Env       map[string]string `json:"env"`       // stdio: 环境变量
	URL       string            `json:"url"`       // http: 服务器 URL
	Headers   map[string]string `json:"headers"`   // http: 请求头
	Enabled   bool              `json:"enabled"`
}

// Config mcp-agent 配置
type Config struct {
	ServerURL          string                     `json:"server_url"`
	GatewayHTTP        string                     `json:"gateway_http"`
	AuthToken          string                     `json:"auth_token"`
	AgentName          string                     `json:"agent_name"`
	ToolPrefix         string                     `json:"tool_prefix"`
	ToolCallTimeoutSec int                        `json:"tool_call_timeout_sec"`
	MCPServers         map[string]MCPServerConfig `json:"mcp_servers"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServerURL:          "ws://127.0.0.1:10086/ws/uap",
		GatewayHTTP:        "http://127.0.0.1:10086",
		AgentName:          "mcp-agent",
		ToolPrefix:         "mcp",
		ToolCallTimeoutSec: 30,
		MCPServers:         make(map[string]MCPServerConfig),
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

	// 填充默认值
	if cfg.AgentName == "" {
		cfg.AgentName = "mcp-agent"
	}
	if cfg.ToolPrefix == "" {
		cfg.ToolPrefix = "mcp"
	}
	if cfg.ToolCallTimeoutSec <= 0 {
		cfg.ToolCallTimeoutSec = 30
	}

	return cfg, nil
}

// DiffServers 比较新旧配置，返回需要 added/removed/changed 的 server 名
func DiffServers(oldServers, newServers map[string]MCPServerConfig) (added, removed, changed []string) {
	for name := range newServers {
		if _, exists := oldServers[name]; !exists {
			added = append(added, name)
		}
	}
	for name := range oldServers {
		if _, exists := newServers[name]; !exists {
			removed = append(removed, name)
		}
	}
	for name, newCfg := range newServers {
		if oldCfg, exists := oldServers[name]; exists {
			if !reflect.DeepEqual(oldCfg, newCfg) {
				changed = append(changed, name)
			}
		}
	}

	log.Printf("[Config] diff: added=%v removed=%v changed=%v", added, removed, changed)
	return
}
