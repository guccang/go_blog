package http

import (
	"os"
	"strings"
	"testing"
)

func TestMarkdownEditorUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/markdown_editor.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="markdown-editor"`, `article-meta site-page-hero`, `data-symbol="editor"`, `editor-content site-paper-panel`, `editor_workspace_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("markdown editor template missing %q", marker)
		}
	}
}
