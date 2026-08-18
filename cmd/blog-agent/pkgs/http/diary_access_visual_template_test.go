package http

import (
	"os"
	"strings"
	"testing"
)

func TestDiaryAccessPagesUseUnifiedVisualLanguage(t *testing.T) {
	tests := []struct{ path, page, symbol string }{
		{"../../templates/diary_password.template", `data-page="diary-password"`, `data-symbol="warning"`},
		{"../../templates/diary_password_error.template", `data-page="diary-password-error"`, `data-symbol="error"`},
	}
	for _, tt := range tests {
		content, err := os.ReadFile(tt.path)
		if err != nil {
			t.Fatal(err)
		}
		template := string(content)
		for _, marker := range []string{tt.page, tt.symbol, `diary-access-card site-paper-panel`, `diary_access_unified.css`} {
			if !strings.Contains(template, marker) {
				t.Fatalf("%s missing %q", tt.path, marker)
			}
		}
	}
}
