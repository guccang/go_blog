package http

import (
	"os"
	"strings"
	"testing"
)

func TestBookDetailUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/book_detail.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="book-detail"`,
		`class="book-detail-container site-shell"`,
		`class="book-detail-symbol"`,
		`data-symbol="reading"`,
		`site-paper-panel`,
		`book_detail_unified.css`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("book detail template missing %q", marker)
		}
	}
}
