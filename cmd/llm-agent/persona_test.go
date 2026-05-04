package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadScopedPersonaProfileUsesAccountFilePath(t *testing.T) {
	workspace := t.TempDir()
	fallback := &PersonaProfile{
		Name:     "GlobalBot",
		Body:     "global body",
		FilePath: filepath.Join(workspace, "PERSONA.md"),
	}

	profile := loadScopedPersonaProfile(workspace, "alice", fallback)
	wantPath := filepath.Join(workspace, "users", "alice", "PERSONA.md")
	if profile.FilePath != wantPath {
		t.Fatalf("FilePath=%q want=%q", profile.FilePath, wantPath)
	}
	if profile.Body != "global body" {
		t.Fatalf("Body=%q want global fallback body", profile.Body)
	}
}

func TestSetPersonaWritesAccountPersonaOnly(t *testing.T) {
	workspace := t.TempDir()
	globalPath := filepath.Join(workspace, "PERSONA.md")
	globalContent := "---\nname: \"GlobalBot\"\n---\n\nglobal body\n"
	if err := os.WriteFile(globalPath, []byte(globalContent), 0644); err != nil {
		t.Fatalf("WriteFile global persona: %v", err)
	}

	bridge := &Bridge{
		cfg: &Config{WorkspaceDir: workspace},
		persona: &PersonaProfile{
			Name:     "GlobalBot",
			Body:     "global body",
			FilePath: globalPath,
		},
		toolHandlers: make(map[string]ToolHandler),
		toolNameMap:  make(map[string]string),
	}
	bridge.registerBuiltinTools()

	handler := bridge.toolHandlers["set_persona"]
	if handler == nil {
		t.Fatalf("set_persona handler not registered")
	}

	ctx := WithAuthenticatedUser(context.Background(), "alice")
	result, err := handler(ctx, []byte(`{"name":"AliceBot","owner_title":"主人"}`), nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result == nil || !strings.Contains(result.Result, "人设设置成功") {
		t.Fatalf("unexpected handler result: %#v", result)
	}

	accountPath := filepath.Join(workspace, "users", "alice", "PERSONA.md")
	accountData, err := os.ReadFile(accountPath)
	if err != nil {
		t.Fatalf("ReadFile account persona: %v", err)
	}
	if !strings.Contains(string(accountData), `name: "AliceBot"`) {
		t.Fatalf("account persona not updated: %s", string(accountData))
	}

	globalData, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile global persona: %v", err)
	}
	if string(globalData) != globalContent {
		t.Fatalf("global persona should remain unchanged, got: %s", string(globalData))
	}
}
