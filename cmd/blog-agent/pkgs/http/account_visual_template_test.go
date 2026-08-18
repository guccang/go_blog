package http

import (
	"os"
	"strings"
	"testing"
)

func TestAccountUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile("../../templates/account.template")
	if err != nil {
		t.Fatal(err)
	}
	template := string(content)
	for _, marker := range []string{`data-page="account"`, `class="account-header site-page-hero"`, `data-symbol="account"`, `class="account-stats site-section"`, `account_unified.css`} {
		if !strings.Contains(template, marker) {
			t.Fatalf("account template missing %q", marker)
		}
	}
}
