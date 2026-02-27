package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AgentConfig agent 配置
type AgentConfig struct {
	ServerURL     string
	AgentName     string
	AuthToken     string
	Workspaces    []string
	ClaudePath    string
	MaxConcurrent int
	MaxTurns      int
	SettingsDir   string // Claude CLI --settings 配置文件目录
}

// LoadConfig 从配置文件加载配置
func LoadConfig(path string) (*AgentConfig, error) {
	cfg := &AgentConfig{
		ClaudePath:    "claude",
		MaxConcurrent: 3,
		MaxTurns:      20,
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "server_url":
			cfg.ServerURL = val
		case "agent_name":
			cfg.AgentName = val
		case "auth_token":
			cfg.AuthToken = val
		case "workspaces":
			for _, ws := range strings.Split(val, ",") {
				ws = strings.TrimSpace(ws)
				if ws != "" {
					cfg.Workspaces = append(cfg.Workspaces, ws)
				}
			}
		case "claude_path":
			cfg.ClaudePath = val
		case "max_concurrent":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxConcurrent = n
			}
		case "max_turns":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxTurns = n
			}
		case "settings_dir":
			cfg.SettingsDir = val
		}
	}

	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if cfg.AgentName == "" {
		cfg.AgentName, _ = os.Hostname()
	}
	if len(cfg.Workspaces) == 0 {
		return nil, fmt.Errorf("workspaces is required")
	}

	// 默认 settings_dir 为配置文件同目录下的 settings/
	if cfg.SettingsDir == "" {
		cfg.SettingsDir = filepath.Join(filepath.Dir(path), "settings")
	}

	// 将相对路径转为绝对路径（避免 Claude CLI 在项目目录下找不到）
	if !filepath.IsAbs(cfg.SettingsDir) {
		abs, err := filepath.Abs(cfg.SettingsDir)
		if err == nil {
			cfg.SettingsDir = abs
		}
	}

	return cfg, nil
}
