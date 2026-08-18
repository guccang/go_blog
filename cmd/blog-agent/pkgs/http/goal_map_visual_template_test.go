package http

import (
	"os"
	"strings"
	"testing"
)

func TestGoalMapUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/goal_map.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="goal-map"`, `class="goal-hero site-page-hero"`, `data-symbol="goal"`, `site-paper-panel`, `goal_map_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("goal map template missing %q", marker)
		}
	}
}
