package http

import (
	"os"
	"strings"
	"testing"
)

func TestSearchTemplateUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/search_results.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="search"`,
		`class="site-page-hero"`,
		`data-symbol="search"`,
		`class="query-workspace site-section"`,
		`class="standalone-query site-paper-panel"`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("search template missing %q", marker)
		}
	}
}

func TestSearchStylesAreScopedForProgressiveMigration(t *testing.T) {
	content, err := os.ReadFile("../../statics/css/query_page.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(content)
	for _, marker := range []string{
		`html[data-page="search"] .query-shell`,
		`html[data-page="search"] .standalone-query`,
		`:root[data-theme="watercolor"][data-page="search"]`,
	} {
		if !strings.Contains(styles, marker) {
			t.Fatalf("search styles missing %q", marker)
		}
	}
}
