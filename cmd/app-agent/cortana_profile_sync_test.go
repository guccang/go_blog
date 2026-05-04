package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCortanaProfileToWorkspace(t *testing.T) {
	baseDir := t.TempDir()
	err := syncCortanaProfileToWorkspace(baseDir, "alice", CortanaSettings{
		PersonaName:        "Cortana X",
		OwnerTitle:         "主人",
		PersonaDescription: "冷静、连续、会接话",
		UpdatedAt:          123,
	})
	if err != nil {
		t.Fatalf("syncCortanaProfileToWorkspace() error: %v", err)
	}

	path := filepath.Join(baseDir, "users", "alice", "memory", "cortana_profile.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if got["name"] != "Cortana X" {
		t.Fatalf("name=%v want Cortana X", got["name"])
	}
	if got["owner_title"] != "主人" {
		t.Fatalf("owner_title=%v want 主人", got["owner_title"])
	}
	if got["description"] != "冷静、连续、会接话" {
		t.Fatalf("description=%v want expected text", got["description"])
	}
}
