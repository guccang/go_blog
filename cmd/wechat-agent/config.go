package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config wechat-agent 配置
type Config struct {
	// HTTP 服务
	HTTPPort int `json:"http_port"` // 微信回调监听端口

	// Gateway 连接
	GatewayURL string `json:"gateway_url"` // ws://host:port/ws/uap
	AuthToken  string `json:"auth_token"`
	AgentName  string `json:"agent_name"`

	// 微信自建应用凭证
	CorpID         string `json:"corp_id"`
	AgentID        string `json:"agent_id"`
	Secret         string `json:"secret"`
	Token          string `json:"token"`
	EncodingAESKey string `json:"encoding_aes_key"`

	// 微信群机器人 Webhook（可选，降级通知用）
	WebhookURL string `json:"webhook_url"`

	// 消息路由目标
	LLMAgentID     string `json:"llm_agent_id"`     // llm-agent 的 ID（自然语言）
	BackendAgentID string `json:"backend_agent_id"` // blog-agent 的 ID（结构化命令）

	// 部署保护文件（deploy-agent 增量部署时跳过这些文件）
	ProtectedFiles []string `json:"protected_files,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		HTTPPort:       9001,
		GatewayURL:     "ws://127.0.0.1:9000/ws/uap",
		AgentName:      "wechat-agent",
		LLMAgentID:     "llm-agent",
		BackendAgentID: "blog-agent",

		ProtectedFiles: []string{"wechat-agent.json"},
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

// IsCallbackEnabled 回调接收是否可用
func (c *Config) IsCallbackEnabled() bool {
	return c.CorpID != "" && c.Token != "" && c.EncodingAESKey != ""
}

// IsAppEnabled 应用消息是否可用
func (c *Config) IsAppEnabled() bool {
	return c.CorpID != "" && c.Secret != "" && c.AgentID != ""
}
