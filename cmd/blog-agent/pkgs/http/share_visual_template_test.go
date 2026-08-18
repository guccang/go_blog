package http

import (
	"os"
	"strings"
	"testing"
)

func TestShareTemplateUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/share.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="share"`,
		`class="site-page-hero"`,
		`data-symbol="portal"`,
		`class="share-search__panel site-paper-panel"`,
		`class="site-index-list site-paper-panel"`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("share template missing %q", marker)
		}
	}
	if !strings.Contains(template, `id="search"`) || !strings.Contains(template, "onSearch()") {
		t.Fatal("share template must preserve tag search behavior")
	}
}
