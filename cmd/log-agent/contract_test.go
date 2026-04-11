package main

import (
	"testing"

	"uap"
)

func TestNewConnectionIncludesLogQueryTools(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogSources = map[string]LogSource{
		"blog": {Path: t.TempDir(), Description: "blog logs"},
	}
	conn := NewConnection(cfg, "log-agent-test")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "log_query" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if !hasTool(conn.Client.Tools, "ListLogSources") {
		t.Fatalf("expected ListLogSources to be registered")
	}
	if !hasTool(conn.Client.Tools, "ReadLog") {
		t.Fatalf("expected ReadLog to be registered")
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
