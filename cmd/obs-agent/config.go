package main

import (
	"encoding/json"
	"fmt"
	"os"

	"obsstore"
)

type Config struct {
	HTTPPort             int             `json:"http_port"`
	ReceiveToken         string          `json:"receive_token,omitempty"`
	DownloadTicketSecret string          `json:"download_ticket_secret,omitempty"`
	SignedURLTTLSeconds  int             `json:"signed_url_ttl_seconds,omitempty"`
	OBS                  obsstore.Config `json:"obs,omitempty"`
	ProtectedFiles       []string        `json:"protected_files,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		HTTPPort:            9004,
		SignedURLTTLSeconds: 300,
		ProtectedFiles:      []string{"obs-agent.json"},
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
	if cfg.HTTPPort <= 0 {
		cfg.HTTPPort = 9004
	}
	if cfg.SignedURLTTLSeconds <= 0 {
		cfg.SignedURLTTLSeconds = 300
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
