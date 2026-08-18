package http

import (
	"os"
	"strings"
	"testing"
)

func TestAskTemplateUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/ask.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="ask"`,
		`class="site-page-hero"`,
		`data-symbol="editor"`,
		`class="query-workspace site-section"`,
		`class="standalone-query site-paper-panel"`,
		`class="pi-answer site-paper-panel"`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("ask template missing %q", marker)
		}
	}
}

func TestAskStylesAreThemeScoped(t *testing.T) {
	content, err := os.ReadFile("../../statics/css/query_page.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(content)
	for _, marker := range []string{
		`html[data-page="ask"] .query-shell`,
		`html[data-page="ask"] .pi-answer`,
		`:root[data-theme="terminal"][data-page="ask"]`,
		`:root[data-theme="watercolor"][data-page="ask"]`,
	} {
		if !strings.Contains(styles, marker) {
			t.Fatalf("ask styles missing %q", marker)
		}
	}
}
