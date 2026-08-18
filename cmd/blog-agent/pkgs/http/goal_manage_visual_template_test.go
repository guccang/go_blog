package http

import (
	"os"
	"strings"
	"testing"
)

func TestGoalManageUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/goal.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{
		`data-page="goal-manage"`,
		`class="goal-manage-shell site-shell"`,
		`class="site-page-hero"`,
		`data-symbol="goal"`,
		`goal_manage_unified.css`,
	} {
		if !strings.Contains(template, marker) {
			t.Fatalf("goal manage template missing %q", marker)
		}
	}
}
