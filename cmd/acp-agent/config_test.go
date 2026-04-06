package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadConfigFallsBackToDefaultWorkspaceWhenMissing(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "acp-agent.json")
	writeACPConfigForTest(t, configPath, map[string]any{
		"server_url": "ws://127.0.0.1:10086/ws/uap",
	})

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	want := filepath.Join(configDir, "workspace")
	if len(cfg.Workspaces) != 1 || cfg.Workspaces[0] != want {
		t.Fatalf("unexpected workspaces: %#v, want [%q]", cfg.Workspaces, want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("expected default workspace dir to exist, err=%v", err)
	}
}

func TestLoadConfigFallsBackForIllegalWorkspacePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows drive path is legal on Windows")
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "acp-agent.json")
	writeACPConfigForTest(t, configPath, map[string]any{
		"server_url": "ws://127.0.0.1:10086/ws/uap",
		"workspaces": []string{`E:\githubdesktop`},
	})

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	want := filepath.Join(configDir, "workspace")
	if len(cfg.Workspaces) != 1 || cfg.Workspaces[0] != want {
		t.Fatalf("unexpected workspaces: %#v, want [%q]", cfg.Workspaces, want)
	}
	if _, err := os.Stat(filepath.Join(configDir, `E:\githubdesktop`)); !os.IsNotExist(err) {
		t.Fatalf("unexpected illegal workspace directory created: err=%v", err)
	}
}

func writeACPConfigForTest(t *testing.T, path string, payload map[string]any) {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}
