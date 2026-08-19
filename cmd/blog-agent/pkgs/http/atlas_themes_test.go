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

func TestAtlasThemeBackgroundLayerVisible(t *testing.T) {
	styles := readThemeStyles(t)
	if !strings.Contains(styles, `html[data-theme^="atlas-"] body { background: transparent; }`) {
		t.Error("atlas 主题 body 背景应为透明，否则 z-index:-1 的 body::before 纹理层被不透明背景覆盖而不可见")
	}
}

func TestAtlasThemeDynamics(t *testing.T) {
	styles := readThemeStyles(t)
	for _, expected := range []string{
		"@keyframes atlas-celadon-rain",
		"@keyframes atlas-swiss-tick",
		"@keyframes atlas-chrome-breathe",
		`html[data-theme="atlas-celadon"][data-page="main"] :where(.quick-item, .recent-card):hover`,
		`html[data-theme="atlas-swiss"][data-page="main"] :where(.quick-item, .recent-card):hover`,
		`html[data-theme="atlas-chrome"][data-page="main"] :where(.quick-item, .recent-card):hover`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("atlas 主题动态效果缺少 %q", expected)
		}
	}

	reducedIdx := strings.LastIndex(styles, "@media (prefers-reduced-motion: reduce)")
	if reducedIdx < 0 {
		t.Fatal("theme.css 缺少 prefers-reduced-motion 块")
	}
	reduced := styles[reducedIdx:]
	for _, expected := range []string{
		`html[data-theme^="atlas-"] body::before`,
		"animation: none",
	} {
		if !strings.Contains(reduced, expected) {
			t.Errorf("reduced-motion 兜底缺少 %q", expected)
		}
	}
}

func TestAtlasThemePalettes(t *testing.T) {
	styles := strings.ToLower(readThemeStyles(t))
	script := readThemeScript(t)
	cases := []struct {
		key    string
		colors []string
	}{
		{key: "atlas-celadon", colors: []string{"#e7efea", "#b8d2c7", "#7fa99a", "#3f6f66", "#283c39"}},
		{key: "atlas-swiss", colors: []string{"#f5f4ef", "#1a1c1f", "#d9382a", "#b8bdc2", "#6c7178"}},
		{key: "atlas-chrome", colors: []string{"#080b10", "#222935", "#667589", "#b9d1e6", "#7b5cff"}},
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
