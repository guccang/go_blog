package http

import (
	"os"
	"strings"
	"testing"
)

func TestMediaViewerUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/media_viewer.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="media-viewer"`, `viewer-header site-hanging-nav`, `data-symbol="article"`, `viewer-stage site-paper-panel`, `media_viewer_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("media viewer template missing %q", marker)
		}
	}
}
