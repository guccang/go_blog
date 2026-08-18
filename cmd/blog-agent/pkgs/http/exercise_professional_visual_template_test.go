package http

import (
	"os"
	"strings"
	"testing"
)

func TestExerciseProfessionalUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/exercise_professional.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="exercise-professional"`, `class="pro-hero site-page-hero"`, `data-symbol="exercise"`, `site-paper-panel`, `exercise_professional_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("exercise professional template missing %q", marker)
		}
	}
}
