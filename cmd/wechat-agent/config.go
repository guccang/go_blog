package main

import (
	"encoding/json"
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

	// 过渡期：直接转发给 go_blog-agent 的 ID
	GoBackendAgentID string `json:"go_backend_agent_id"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		HTTPPort:         9001,
		GatewayURL:       "ws://127.0.0.1:9000/ws/uap",
		AgentName:        "wechat-agent",
		GoBackendAgentID: "go_blog",
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

// IsCallbackEnabled 回调接收是否可用
func (c *Config) IsCallbackEnabled() bool {
	return c.CorpID != "" && c.Token != "" && c.EncodingAESKey != ""
}

// IsAppEnabled 应用消息是否可用
func (c *Config) IsAppEnabled() bool {
	return c.CorpID != "" && c.Secret != "" && c.AgentID != ""
}
