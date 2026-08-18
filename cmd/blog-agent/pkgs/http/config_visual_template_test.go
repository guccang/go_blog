package http

import (
	"os"
	"strings"
	"testing"
)

func TestConfigUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/config.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="config"`, `class="header site-page-hero"`, `data-symbol="settings"`, `class="content site-section"`, `config_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("config template missing %q", marker)
		}
	}
}
