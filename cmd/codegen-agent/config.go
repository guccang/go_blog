package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AgentConfig agent 配置
type AgentConfig struct {
	ServerURL             string   `json:"server_url"`
	AgentName             string   `json:"agent_name"`
	AgentType             string   `json:"agent_type"` // agent 类型: codegen(编码) / deploy(发布)，默认 codegen
	AuthToken             string   `json:"auth_token"`
	Workspaces            []string `json:"workspaces"`
	ClaudePath            string   `json:"claude_path"`
	OpenCodePath          string   `json:"opencode_path"`
	MaxConcurrent         int      `json:"max_concurrent"`
	MaxTurns              int      `json:"max_turns"`
	ClaudeCodeSettingsDir string   `json:"claudecode_settings_dir"` // Claude Code --settings 配置目录
	OpenCodeSettingsDir   string   `json:"opencode_settings_dir"`   // OpenCode 模型映射配置目录
	ResumeModels          []string `json:"resume_models,omitempty"` // 支持 --resume 的模型名列表（空字符串代表默认模型）
	GoBackendAgentID      string   `json:"go_backend_agent_id"`     // blog-agent-agent 在 gateway 中的 ID，默认 "blog-agent"

	// 部署保护文件（deploy-agent 增量部署时跳过这些文件）
	ProtectedFiles []string `json:"protected_files,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *AgentConfig {
	return &AgentConfig{
		AgentType:        "codegen",
		ClaudePath:       "claude",
		OpenCodePath:     "opencode",
		MaxConcurrent:    3,
		MaxTurns:         20,
		GoBackendAgentID: "blog-agent",

		ProtectedFiles: []string{"codegen-agent.json", "settings/"},
	}
}

// LoadConfig 从 JSON 配置文件加载配置
func LoadConfig(path string) (*AgentConfig, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}

	// 必填字段校验
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if len(cfg.Workspaces) == 0 {
		return nil, fmt.Errorf("workspaces is required")
	}

	// 默认值填充
	if cfg.AgentName == "" {
		cfg.AgentName, _ = os.Hostname()
	}
	if cfg.AgentType == "" {
		cfg.AgentType = "codegen"
	}
	if cfg.ClaudePath == "" {
		cfg.ClaudePath = "claude"
	}
	if cfg.OpenCodePath == "" {
		cfg.OpenCodePath = "opencode"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 20
	}
	if cfg.GoBackendAgentID == "" {
		cfg.GoBackendAgentID = "blog-agent"
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

	return cfg, nil
}
