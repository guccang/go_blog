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
	ServerURL             string
	AgentName             string
	AuthToken             string
	Workspaces            []string
	ClaudePath            string
	OpenCodePath          string
	MaxConcurrent         int
	MaxTurns              int
	ClaudeCodeSettingsDir string // Claude Code --settings 配置目录
	OpenCodeSettingsDir   string // OpenCode 模型映射配置目录
	DeployAgentPath       string // deploy-agent.exe 路径（留空则不启用 auto_deploy）
	DeployAgentConfig     string // deploy.conf 路径
	VerifyURL             string // 部署验证 URL（HTTP GET）
	VerifyTimeout         int    // 验证超时秒数，默认 10
}

// LoadConfig 从配置文件加载配置
func LoadConfig(path string) (*AgentConfig, error) {
	cfg := &AgentConfig{
		ClaudePath:    "claude",
		OpenCodePath:  "opencode",
		MaxConcurrent: 3,
		MaxTurns:      20,
		VerifyTimeout: 10,
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
		case "opencode_path":
			cfg.OpenCodePath = val
		case "max_concurrent":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxConcurrent = n
			}
		case "max_turns":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxTurns = n
			}
		case "claudecode_settings_dir":
			cfg.ClaudeCodeSettingsDir = val
		case "opencode_settings_dir":
			cfg.OpenCodeSettingsDir = val
		case "deploy_agent_path":
			cfg.DeployAgentPath = val
		case "deploy_agent_config":
			cfg.DeployAgentConfig = val
		case "verify_url":
			cfg.VerifyURL = val
		case "verify_timeout":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.VerifyTimeout = n
			}
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

	configDir := filepath.Dir(path)

	// 默认 claudecode_settings_dir 为 settings/claudecode/
	if cfg.ClaudeCodeSettingsDir == "" {
		cfg.ClaudeCodeSettingsDir = filepath.Join(configDir, "settings", "claudecode")
	}
	// 默认 opencode_settings_dir 为 settings/opencode/
	if cfg.OpenCodeSettingsDir == "" {
		cfg.OpenCodeSettingsDir = filepath.Join(configDir, "settings", "opencode")
	}

	// 将相对路径转为绝对路径
	if !filepath.IsAbs(cfg.ClaudeCodeSettingsDir) {
		abs, err := filepath.Abs(cfg.ClaudeCodeSettingsDir)
		if err == nil {
			cfg.ClaudeCodeSettingsDir = abs
		}
	}
	if !filepath.IsAbs(cfg.OpenCodeSettingsDir) {
		abs, err := filepath.Abs(cfg.OpenCodeSettingsDir)
		if err == nil {
			cfg.OpenCodeSettingsDir = abs
		}
	}

	// deploy-agent 路径转绝对
	if cfg.DeployAgentPath != "" && !filepath.IsAbs(cfg.DeployAgentPath) {
		abs, err := filepath.Abs(cfg.DeployAgentPath)
		if err == nil {
			cfg.DeployAgentPath = abs
		}
	}
	if cfg.DeployAgentConfig != "" && !filepath.IsAbs(cfg.DeployAgentConfig) {
		abs, err := filepath.Abs(cfg.DeployAgentConfig)
		if err == nil {
			cfg.DeployAgentConfig = abs
		}
	}

	return cfg, nil
}
