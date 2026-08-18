package http

import (
	"os"
	"strings"
	"testing"
)

func TestLoginUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/login.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="login"`, `class="login-container site-paper-panel"`, `class="login-artifact"`, `data-symbol="account"`, `login_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("login template missing %q", marker)
		}
	}
}
