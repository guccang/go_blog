package main

import (
	"path/filepath"
	"testing"

	"uap"
)

func TestNewConnectionRegistersCronTools(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TaskFile = filepath.Join(t.TempDir(), "cron-tasks.json")

	conn := NewConnection(cfg, "cron-agent-test")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	defer conn.engine.Stop()

	if conn.AgentType != "cron_agent" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(conn.Client.Tools))
	}
	if !hasTool(conn.Client.Tools, "cronCreateTask") {
		t.Fatalf("expected cronCreateTask to be registered")
	}
	if !hasTool(conn.Client.Tools, "cronListPending") {
		t.Fatalf("expected cronListPending to be registered")
	}
}

func hasTool(tools []uap.ToolDef, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
