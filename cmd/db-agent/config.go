package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DBConfig holds the db-agent configuration.
type DBConfig struct {
	ServerURL     string `json:"server_url"`
	AgentName     string `json:"agent_name"`
	AuthToken     string `json:"auth_token"`
	MaxConcurrent int    `json:"max_concurrent"`

	Driver  string `json:"driver"`   // "sqlite" (default), "mongodb", "redis"
	DSN     string `json:"dsn"`      // SQLite file path / MongoDB URI / Redis addr
	DataDir string `json:"data_dir"` // Data directory for database files
}

// LoadConfig reads and parses the JSON config file.
func LoadConfig(path string) (*DBConfig, error) {
	cfg := &DBConfig{
		MaxConcurrent: 5,
		Driver:        "sqlite",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.AgentName == "" {
		host, _ := os.Hostname()
		if host != "" {
			cfg.AgentName = host + "-db"
		} else {
			cfg.AgentName = "db-agent"
		}
	}
	if cfg.Driver == "" {
		cfg.Driver = "sqlite"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Dir(path)
	}

	// Resolve relative DSN
	if cfg.DSN != "" && !filepath.IsAbs(cfg.DSN) {
		cfg.DSN = filepath.Join(filepath.Dir(path), cfg.DSN)
	}

	return cfg, nil
}

// generateDefaultConfig creates a default db-agent.json config file.
func generateDefaultConfig(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s (will not overwrite)", configPath)
	}

	dir := filepath.Dir(configPath)
	dataDir := filepath.Join(dir, "data")

	defaultCfg := &DBConfig{
		Driver:        "sqlite",
		MaxConcurrent: 5,
		DataDir:       dataDir,
	}

	data, err := json.MarshalIndent(defaultCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("generated default config: %s\n", configPath)
	return nil
}
