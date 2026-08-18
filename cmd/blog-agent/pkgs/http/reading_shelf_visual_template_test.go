package http

import (
	"os"
	"strings"
	"testing"
)

func TestReadingShelfUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/reading_shelf.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="reading-shelf"`,
		`class="reading-hero site-page-hero"`,
		`data-symbol="reading"`,
		`site-paper-panel`,
		`data-symbol="empty"`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("reading shelf template missing %q", marker)
		}
	}
}
