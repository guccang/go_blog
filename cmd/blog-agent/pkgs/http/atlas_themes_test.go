package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readThemeStyles(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "theme.css"))
	if err != nil {
		t.Fatalf("读取主题样式失败: %v", err)
	}
	return string(content)
}

func TestAtlasThemeBaseDerivesTokens(t *testing.T) {
	styles := readThemeStyles(t)
	for _, expected := range []string{
		`:root[data-theme^="atlas-"]`,
		`--ui-canvas: var(--c1)`,
		`--ui-text: var(--ink, var(--c5))`,
		`--ui-coral: var(--accent, var(--c4))`,
		`var(--ui-card-shadow, 4px 4px 0 var(--ui-shadow-soft))`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("图鉴基座缺少 %q", expected)
		}
	}
}
