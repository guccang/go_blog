package http

import (
	"os"
	"strings"
	"testing"
)

func TestListTemplateUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/list.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="list"`,
		`class="site-page-hero"`,
		`data-symbol="archive"`,
		`class="site-index-list site-paper-panel"`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("list template missing %q", marker)
		}
	}
}
