package main

import (
	"os"
	"path/filepath"
	"strings"
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
	if !hasTool(conn.Client.Tools, "AnalyzeLog") {
		t.Fatalf("expected AnalyzeLog to be registered")
	}
}

func TestToolListLogSourcesIncludesFileStats(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.log"), []byte("test log\n"), 0644)

	cfg := DefaultConfig()
	cfg.LogSources = map[string]LogSource{
		"test": {Path: dir, Description: "test logs"},
	}
	conn := NewConnection(cfg, "log-agent-test")

	result := conn.toolListLogSources()
	if !strings.Contains(result, "file_count") {
		t.Fatalf("expected file_count in result, got: %s", result)
	}
	if !strings.Contains(result, "newest_file") {
		t.Fatalf("expected newest_file in result, got: %s", result)
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
