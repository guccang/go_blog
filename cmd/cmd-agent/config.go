package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	GatewayURL     string   `json:"gateway_url"`
	AuthToken      string   `json:"auth_token,omitempty"`
	AgentID        string   `json:"agent_id,omitempty"`
	AgentName      string   `json:"agent_name,omitempty"`
	WorkspaceDir   string   `json:"workspace_dir,omitempty"`
	ProtectedFiles []string `json:"protected_files,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		GatewayURL:     "ws://127.0.0.1:9000/ws/uap",
		AgentID:        "cmd-agent",
		AgentName:      "cmd-agent",
		WorkspaceDir:   "workspace",
		ProtectedFiles: []string{"cmd-agent.json"},
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
	if cfg.AgentID == "" {
		cfg.AgentID = "cmd-agent"
	}
	if cfg.AgentName == "" {
		cfg.AgentName = cfg.AgentID
	}
	if cfg.WorkspaceDir == "" {
		cfg.WorkspaceDir = "workspace"
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
