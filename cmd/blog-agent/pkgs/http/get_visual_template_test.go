package http

import (
	"os"
	"strings"
	"testing"
)

func TestArticleTemplatesUseUnifiedReadingWorkspace(t *testing.T) {
	tests := []struct {
		path     string
		page     string
		artifact string
	}{
		{"../../templates/get.template", `data-page="get"`, "ARTICLE DESK"},
		{"../../templates/get_public.template", `data-page="get-public"`, "PUBLIC READING"},
	}

	for _, tt := range tests {
		content, err := os.ReadFile(tt.path)
		if err != nil {
			t.Fatal(err)
		}
		template := string(content)
		for _, marker := range []string{
			tt.page,
			`class="reader-artifact"`,
			`data-symbol="article"`,
			tt.artifact,
			`class="container reader-workspace"`,
			`site-paper-panel`,
		} {
			if !strings.Contains(template, marker) {
				t.Fatalf("%s missing %q", tt.path, marker)
			}
		}
	}
}

func TestArticleWorkspaceStylesUseThemeTokens(t *testing.T) {
	for _, path := range []string{
		"../../statics/css/get_workspace.css",
		"../../statics/css/get_public.css",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		styles := string(content)
		for _, marker := range []string{"var(--ui-canvas)", "var(--ui-surface)", "var(--ui-coral)"} {
			if !strings.Contains(styles, marker) {
				t.Fatalf("%s missing %q", path, marker)
			}
		}
	}
}
