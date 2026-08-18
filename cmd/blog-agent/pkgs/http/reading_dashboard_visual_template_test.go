package http

import (
	"os"
	"strings"
	"testing"
)

func TestReadingDashboardUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/reading_dashboard.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="reading-dashboard"`,
		`class="dashboard-hero site-page-hero"`,
		`data-symbol="reading"`,
		`site-paper-panel`,
		`reading_dashboard_unified.css`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("reading dashboard template missing %q", marker)
		}
	}
}
