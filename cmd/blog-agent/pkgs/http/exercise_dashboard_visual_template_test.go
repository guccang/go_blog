package http

import (
	"os"
	"strings"
	"testing"
)

func TestExerciseDashboardUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/exercise_dashboard.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="exercise-dashboard"`, `class="site-page-hero"`, `data-symbol="exercise"`, `site-paper-panel`, `data-symbol="empty"`, `exercise_dashboard_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("exercise dashboard template missing %q", marker)
		}
	}
}
