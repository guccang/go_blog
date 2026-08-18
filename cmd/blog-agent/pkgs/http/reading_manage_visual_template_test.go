package http

import (
	"os"
	"strings"
	"testing"
)

func TestReadingManageUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/reading.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="reading-manage"`,
		`class="manage-heading site-page-hero"`,
		`data-symbol="reading"`,
		`class="top-bar site-paper-panel"`,
		`data-symbol="empty"`,
		`reading_manage_unified.css`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("reading manage template missing %q", marker)
		}
	}
}
