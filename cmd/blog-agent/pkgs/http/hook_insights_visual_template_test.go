package http

import (
	"os"
	"strings"
	"testing"
)

func TestHookInsightsUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/hook_insights.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="hook-insights"`, `insights-hero site-page-hero`, `data-symbol="search"`, `site-paper-panel`, `hook_insights_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("hook insights template missing %q", marker)
		}
	}
}
