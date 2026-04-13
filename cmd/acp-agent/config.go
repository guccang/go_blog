package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// AgentConfig agent 配置
type AgentConfig struct {
	ServerURL             string   `json:"server_url"`              // gateway WebSocket 地址
	AgentName             string   `json:"agent_name"`              // agent 名称
	AgentType             string   `json:"agent_type"`              // agent 类型，默认 "acp"
	AuthToken             string   `json:"auth_token"`              // 认证令牌
	GoBackendAgentID      string   `json:"go_backend_agent_id"`     // blog-agent-agent 在 gateway 中的 ID
	CodingBackend         string   `json:"coding_backend"`          // 编码后端：claude_acp / codex_exec
	ACPAgentCmd           string   `json:"acp_agent_cmd"`           // ACP agent 命令，默认 "npx"
	ACPAgentArgs          []string `json:"acp_agent_args"`          // ACP agent 参数，默认 ["-y", "@zed-industries/claude-agent-acp@latest"]
	CodexCmd              string   `json:"codex_cmd"`               // Codex CLI 命令，默认 "codex"
	CodexArgs             []string `json:"codex_args"`              // Codex CLI 参数，默认 ["exec", "--json", "--skip-git-repo-check"]
	Workspaces            []string `json:"workspaces"`              // 项目工作区目录列表
	MaxConcurrent         int      `json:"max_concurrent"`          // 最大并发数，默认 2
	AnalysisTimeout       int      `json:"analysis_timeout"`        // ACP 分析超时（秒），默认 3600
	ClaudeCodeSettingsDir string   `json:"claudecode_settings_dir"` // Claude Code settings 目录（默认 settings/claudecode/）
	CodexSettingsDir      string   `json:"codex_settings_dir"`      // Codex settings 目录（默认 settings/codex/）
	DefaultSettings       string   `json:"default_settings"`        // 默认 --settings 名称（如 "default"），extraArgs 未指定时自动使用

	// 部署保护文件（deploy-agent 增量部署时跳过这些文件）
	ProtectedFiles []string `json:"protected_files,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *AgentConfig {
	return &AgentConfig{
		CodingBackend:    BackendClaudeACP,
		AgentType:        "acp",
		ACPAgentCmd:      "npx",
		ACPAgentArgs:     []string{"-y", "@zed-industries/claude-agent-acp@latest"},
		CodexCmd:         "codex",
		CodexArgs:        []string{"exec", "--json", "--skip-git-repo-check"},
		MaxConcurrent:    2,
		AnalysisTimeout:  3600,
		GoBackendAgentID: "blog-agent",

		ProtectedFiles: []string{"acp-agent.json", "settings/"},
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

	// 默认值填充
	if cfg.AgentName == "" {
		cfg.AgentName, _ = os.Hostname()
	}
	if cfg.AgentType == "" {
		cfg.AgentType = "acp"
	}
	if strings.TrimSpace(cfg.CodingBackend) == "" {
		cfg.CodingBackend = BackendClaudeACP
	}
	if normalized := normalizeCodingBackend(cfg.CodingBackend); normalized == "" {
		return nil, fmt.Errorf("unsupported coding_backend: %s", cfg.CodingBackend)
	} else {
		cfg.CodingBackend = normalized
	}
	if cfg.ACPAgentCmd == "" {
		cfg.ACPAgentCmd = "npx"
	}
	if len(cfg.ACPAgentArgs) == 0 {
		cfg.ACPAgentArgs = []string{"-y", "@zed-industries/claude-agent-acp@latest"}
	}
	if cfg.CodexCmd == "" {
		cfg.CodexCmd = "codex"
	}
	if len(cfg.CodexArgs) == 0 {
		cfg.CodexArgs = []string{"exec", "--json", "--skip-git-repo-check"}
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 2
	}
	if cfg.AnalysisTimeout <= 0 {
		cfg.AnalysisTimeout = 3600
	}
	if cfg.GoBackendAgentID == "" {
		cfg.GoBackendAgentID = "blog-agent"
	}

	configDir := filepath.Dir(path)
	defaultWorkspace, err := ensureDefaultWorkspaceDir(configDir)
	if err != nil {
		return nil, err
	}
	cfg.Workspaces = normalizeWorkspaceList(cfg.Workspaces, configDir, defaultWorkspace)

	// 默认 claudecode_settings_dir 为配置文件同目录下的 settings/claudecode/
	if cfg.ClaudeCodeSettingsDir == "" {
		cfg.ClaudeCodeSettingsDir = filepath.Join(configDir, "settings", "claudecode")
	}
	if !filepath.IsAbs(cfg.ClaudeCodeSettingsDir) {
		if abs, err := filepath.Abs(cfg.ClaudeCodeSettingsDir); err == nil {
			cfg.ClaudeCodeSettingsDir = abs
		}
	}

	if cfg.CodexSettingsDir == "" {
		cfg.CodexSettingsDir = filepath.Join(configDir, "settings", "codex")
	}
	if !filepath.IsAbs(cfg.CodexSettingsDir) {
		if abs, err := filepath.Abs(cfg.CodexSettingsDir); err == nil {
			cfg.CodexSettingsDir = abs
		}
	}

	return cfg, nil
}

var windowsDrivePathPattern = regexp.MustCompile(`^[A-Za-z]:[\\/].*`)

func ensureDefaultWorkspaceDir(configDir string) (string, error) {
	if strings.TrimSpace(configDir) == "" {
		configDir = "."
	}
	defaultWorkspace := filepath.Join(configDir, "workspace")
	defaultWorkspace = filepath.Clean(defaultWorkspace)
	if !filepath.IsAbs(defaultWorkspace) {
		abs, err := filepath.Abs(defaultWorkspace)
		if err != nil {
			return "", fmt.Errorf("resolve default workspace: %v", err)
		}
		defaultWorkspace = abs
	}
	if err := os.MkdirAll(defaultWorkspace, 0755); err != nil {
		return "", fmt.Errorf("create default workspace %s: %v", defaultWorkspace, err)
	}
	return defaultWorkspace, nil
}

func normalizeWorkspaceList(workspaces []string, configDir, defaultWorkspace string) []string {
	if len(workspaces) == 0 {
		return []string{defaultWorkspace}
	}

	normalized := make([]string, 0, len(workspaces))
	seen := make(map[string]bool, len(workspaces))
	for _, raw := range workspaces {
		ws, usedDefault := normalizeWorkspacePath(raw, configDir, defaultWorkspace)
		if usedDefault {
			fmt.Fprintf(os.Stderr, "[WARN] invalid workspace path %q, fallback to %s\n", raw, defaultWorkspace)
		}
		if seen[ws] {
			continue
		}
		seen[ws] = true
		normalized = append(normalized, ws)
	}
	if len(normalized) == 0 {
		return []string{defaultWorkspace}
	}
	return normalized
}

func normalizeWorkspacePath(raw, configDir, defaultWorkspace string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || containsNullByte(raw) || isIllegalWorkspacePath(raw) {
		return defaultWorkspace, true
	}

	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return defaultWorkspace, true
		}
		path = abs
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return defaultWorkspace, true
	}
	return path, false
}

func containsNullByte(s string) bool {
	return strings.ContainsRune(s, '\x00')
}

func isIllegalWorkspacePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return true
	}

	if runtime.GOOS != "windows" {
		if windowsDrivePathPattern.MatchString(path) {
			return true
		}
		if strings.HasPrefix(path, `\\`) {
			return true
		}
	}

	return false
}
