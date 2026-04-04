package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverInstructionFilesOrder(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "repo")
	subdir := filepath.Join(project, "cmd", "llm-agent")

	for _, dir := range []string{
		filepath.Join(project, ".claude", "rules"),
		subdir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	files := map[string]string{
		filepath.Join(project, "AGENTS.md"):                "root agents",
		filepath.Join(project, ".claude", "CLAUDE.md"):     "project claude",
		filepath.Join(project, ".claude", "rules", "a.md"): "rule a",
		filepath.Join(subdir, "CLAUDE.local.md"):           "local claude",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	discovered := discoverInstructionFiles(subdir)
	if len(discovered) != 4 {
		t.Fatalf("expected 4 files, got %d", len(discovered))
	}

	got := []string{
		filepath.Base(discovered[0].Path),
		filepath.Base(discovered[1].Path),
		filepath.Base(discovered[2].Path),
		filepath.Base(discovered[3].Path),
	}
	want := []string{"AGENTS.md", "CLAUDE.md", "a.md", "CLAUDE.local.md"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestBuildInstructionBlockIncludesContents(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("utf8 only"), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	block := buildInstructionBlock(root)
	if !strings.Contains(block, "项目指令") {
		t.Fatalf("expected header in block: %s", block)
	}
	if !strings.Contains(block, "utf8 only") {
		t.Fatalf("expected file content in block: %s", block)
	}
}
