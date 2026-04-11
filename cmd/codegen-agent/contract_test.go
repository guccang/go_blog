package main

import (
	"os"
	"path/filepath"
	"testing"

	"uap"
)

func TestNewConnectionBuildsCodegenContractSurface(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "demo"), 0755); err != nil {
		t.Fatalf("mkdir demo project: %v", err)
	}

	cfg := &AgentConfig{
		ServerURL:             "ws://127.0.0.1:10086/ws/uap",
		AgentName:             "codegen-agent",
		AgentType:             "codegen",
		Workspaces:            []string{workspace},
		MaxConcurrent:         2,
		MaxTurns:              10,
		ClaudePath:            "claude",
		OpenCodePath:          "opencode",
		ClaudeCodeSettingsDir: t.TempDir(),
		OpenCodeSettingsDir:   t.TempDir(),
		GoBackendAgentID:      "blog-agent",
	}

	agent := NewAgent("codegen-test", cfg)
	conn := NewConnection(cfg, agent)
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "codegen" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) == 0 {
		t.Fatalf("expected codegen tools to be registered")
	}
	if !containsToolName(conn.Client.Tools, "CodegenStartSession") {
		t.Fatalf("expected CodegenStartSession to be registered")
	}
	if !containsToolName(conn.Client.Tools, "CodegenListProjects") {
		t.Fatalf("expected CodegenListProjects to be registered")
	}
}

func containsToolName(tools []uap.ToolDef, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
