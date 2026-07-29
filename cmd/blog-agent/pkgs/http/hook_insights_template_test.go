package http

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookInsightsTemplateContainsMVPAreas(t *testing.T) {
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", "hook_insights.template"))
	if err != nil {
		t.Fatalf("parse hook insights template: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, nil); err != nil {
		t.Fatalf("execute hook insights template: %v", err)
	}
	page := output.String()
	for _, expected := range []string{"hookHeatmap", "hookTimeline", "featureRanking", "commonPaths", "/js/hook_insights.js"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("hook insights template missing %q", expected)
		}
	}
}
