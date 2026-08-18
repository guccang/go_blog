package http

import (
	"os"
	"strings"
	"testing"
)

func TestNotifyUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/notify.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="notify"`, `class="site-page-hero"`, `data-symbol="success"`, `class="site-ticket"`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("notify template missing %q", marker)
		}
	}
}
