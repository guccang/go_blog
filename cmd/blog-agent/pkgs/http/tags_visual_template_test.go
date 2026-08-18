package http

import (
	"os"
	"strings"
	"testing"
)

func TestTagsTemplateUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/tags.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="tags"`,
		`class="site-page-hero"`,
		`data-symbol="search"`,
		`class="site-index-list site-paper-panel"`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("tags template missing %q", marker)
		}
	}
	if strings.Contains(template, "@import url(/css/link.css)") {
		t.Fatal("tags template still imports legacy link page styles")
	}
}
