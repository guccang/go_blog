package http

import (
	"os"
	"strings"
	"testing"
)

func TestImagePasteDemoUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/image_paste_demo.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="image-paste-demo"`, `intro site-page-hero`, `data-symbol="editor"`, `workspace site-paper-panel`, `image_paste_demo_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("image paste demo template missing %q", marker)
		}
	}
}
