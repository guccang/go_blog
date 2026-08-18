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

func readThemeScript(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "theme.js"))
	if err != nil {
		t.Fatalf("读取主题脚本失败: %v", err)
	}
	return string(content)
}

func TestAtlasThemePalettes(t *testing.T) {
	styles := strings.ToLower(readThemeStyles(t))
	script := readThemeScript(t)
	cases := []struct {
		key    string
		colors []string
	}{
		{key: "atlas-celadon", colors: []string{"#e7efea", "#b8d2c7", "#7fa99a", "#3f6f66", "#283c39"}},
	}
	for _, c := range cases {
		if !strings.Contains(styles, `data-theme="`+c.key+`"`) {
			t.Errorf("theme.css 缺少主题块 %q", c.key)
		}
		for _, color := range c.colors {
			if !strings.Contains(styles, color) {
				t.Errorf("主题 %s 缺少色值 %s", c.key, color)
			}
		}
		if !strings.Contains(script, `'`+c.key+`':`) {
			t.Errorf("theme.js 未注册主题 %q", c.key)
		}
	}
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
