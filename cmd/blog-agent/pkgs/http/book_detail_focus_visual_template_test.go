package http

import (
	"os"
	"strings"
	"testing"
)

func TestBookDetailFocusUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/book_detail_focus.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="book-detail-focus"`,
		`class="book-topbar site-hanging-nav"`,
		`class="book-symbol-exhibit"`,
		`data-symbol="reading"`,
		`site-paper-panel`,
		`book_detail_focus_unified.css`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("book detail focus template missing %q", marker)
		}
	}
}
