package http

import (
	"os"
	"strings"
	"testing"
)

func TestClipboardUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/clipboard.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="clipboard"`, `clipboard-hero site-page-hero`, `data-symbol="tools"`, `clipboard-composer site-paper-panel`, `clipboard_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("clipboard template missing %q", marker)
		}
	}
}
