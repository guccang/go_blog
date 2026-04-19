package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type OBSStorageConfig struct {
	Endpoint         string `json:"endpoint,omitempty"`
	Bucket           string `json:"bucket,omitempty"`
	AK               string `json:"ak,omitempty"`
	SK               string `json:"sk,omitempty"`
	Region           string `json:"region,omitempty"`
	KeyPrefix        string `json:"key_prefix,omitempty"`
	PathStyle        bool   `json:"path_style,omitempty"`
	DisableSSLVerify bool   `json:"disable_ssl_verify,omitempty"`
}

// Config defines app-agent settings.
type Config struct {
	HTTPPort int `json:"http_port"`

	GatewayURL string `json:"gateway_url"`
	AuthToken  string `json:"auth_token"`
	AgentName  string `json:"agent_name"`

	ReceiveToken             string           `json:"receive_token,omitempty"`
	MaxPendingPerUser        int              `json:"max_pending_per_user,omitempty"`
	PendingMessageTTLHours   int              `json:"pending_message_ttl_hours,omitempty"`
	BlogAgentBaseURL         string           `json:"blog_agent_base_url,omitempty"`
	CmdAgentBaseURL          string           `json:"cmd_agent_base_url,omitempty"`
	AppSessionTTLMinutes     int              `json:"app_session_ttl_minutes,omitempty"`
	AppRefreshTokenTTLHours  int              `json:"app_refresh_token_ttl_hours,omitempty"`
	GroupStoreFile           string           `json:"group_store_file,omitempty"`
	AttachmentStoreDir       string           `json:"attachment_store_dir,omitempty"`
	ObsAgentBaseURL          string           `json:"obs_agent_base_url,omitempty"`
	ObsAgentToken            string           `json:"obs_agent_token,omitempty"`
	DownloadTicketSecret     string           `json:"download_ticket_secret,omitempty"`
	DownloadTicketTTLSeconds int              `json:"download_ticket_ttl_seconds,omitempty"`
	OBS                      OBSStorageConfig `json:"obs,omitempty"`

	LLMAgentID     string `json:"llm_agent_id"`
	CmdAgentID     string `json:"cmd_agent_id,omitempty"`
	BackendAgentID string `json:"backend_agent_id"`

	ProtectedFiles []string `json:"protected_files,omitempty"`

	// DelegationSecretKey 用于签发委托令牌的密钥（需与 blog-agent 配置一致）
	DelegationSecretKey string `json:"delegation_secret_key,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		HTTPPort:                 9002,
		GatewayURL:               "ws://127.0.0.1:9000/ws/uap",
		AgentName:                "app-agent",
		MaxPendingPerUser:        200,
		PendingMessageTTLHours:   24,
		BlogAgentBaseURL:         "http://127.0.0.1:8888",
		CmdAgentBaseURL:          "http://127.0.0.1:9013",
		AppSessionTTLMinutes:     2880,
		AppRefreshTokenTTLHours:  720,
		GroupStoreFile:           "app-groups.json",
		AttachmentStoreDir:       "app-attachments",
		DownloadTicketTTLSeconds: 300,
		LLMAgentID:               "llm-agent",
		CmdAgentID:               "cmd-agent",
		BackendAgentID:           "blog-agent",
		ProtectedFiles:           []string{"app-agent.json"},
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.MaxPendingPerUser <= 0 {
		cfg.MaxPendingPerUser = 200
	}
	if cfg.PendingMessageTTLHours <= 0 {
		cfg.PendingMessageTTLHours = 24
	}
	if cfg.AppSessionTTLMinutes <= 0 {
		cfg.AppSessionTTLMinutes = 2880
	}
	if cfg.AppRefreshTokenTTLHours <= 0 {
		cfg.AppRefreshTokenTTLHours = 720
	}
	if cfg.DownloadTicketTTLSeconds <= 0 {
		cfg.DownloadTicketTTLSeconds = 300
	}
	if cfg.CmdAgentID == "" {
		cfg.CmdAgentID = "cmd-agent"
	}
	if cfg.CmdAgentBaseURL == "" {
		cfg.CmdAgentBaseURL = "http://127.0.0.1:9013"
	}
	if cfg.GroupStoreFile == "" {
		cfg.GroupStoreFile = "app-groups.json"
	}
	if cfg.AttachmentStoreDir == "" {
		cfg.AttachmentStoreDir = "app-attachments"
	}
	return cfg, nil
}

func writeDefaultConfig(path string, cfg interface{}) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config failed: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write config failed: %v", err)
	}
	fmt.Printf("generated config file: %s\n", path)
	return nil
}
