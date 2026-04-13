package main

import "strings"

const (
	BackendClaudeACP = "claude_acp"
	BackendCodexExec = "codex_exec"
)

func normalizeCodingBackend(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "acp", "claude", "claude_acp", "claude-acp":
		return BackendClaudeACP
	case "codex", "codex_exec", "codex-exec":
		return BackendCodexExec
	default:
		return ""
	}
}

func (cfg *AgentConfig) EffectiveCodingBackend() string {
	backend := normalizeCodingBackend(cfg.CodingBackend)
	if backend == "" {
		return BackendClaudeACP
	}
	return backend
}

func (cfg *AgentConfig) BackendLabel() string {
	switch cfg.EffectiveCodingBackend() {
	case BackendCodexExec:
		return "Codex"
	default:
		return "Claude ACP"
	}
}

func (cfg *AgentConfig) BackendCommand() (string, []string) {
	switch cfg.EffectiveCodingBackend() {
	case BackendCodexExec:
		return cfg.CodexCmd, cfg.CodexArgs
	default:
		return cfg.ACPAgentCmd, cfg.ACPAgentArgs
	}
}

func (cfg *AgentConfig) SupportsInteractivePermissions() bool {
	return cfg.EffectiveCodingBackend() == BackendClaudeACP
}

func (cfg *AgentConfig) SupportsSessionModes() bool {
	return cfg.EffectiveCodingBackend() == BackendClaudeACP
}
