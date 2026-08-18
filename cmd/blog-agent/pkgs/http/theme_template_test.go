package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllHTMLTemplatesLoadSharedTheme(t *testing.T) {
	templateDir := filepath.Join("..", "..", "templates")
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatalf("读取模板目录失败: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".template" {
			continue
		}

		path := filepath.Join(templateDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取模板 %s 失败: %v", entry.Name(), err)
		}
		page := string(content)
		if !strings.Contains(page, "<html") {
			continue
		}
		checked++

		for _, asset := range []string{`/js/theme.js?v=`, `/css/theme.css?v=`} {
			if count := strings.Count(page, asset); count != 1 {
				t.Errorf("模板 %s 应恰好加载一次 %s，实际为 %d 次", entry.Name(), asset, count)
			}
		}
		if strings.Index(page, `/js/theme.js`) > strings.Index(page, `/css/theme.css`) {
			t.Errorf("模板 %s 应先加载主题初始化脚本，避免首次绘制闪烁", entry.Name())
		}
	}

	if checked == 0 {
		t.Fatal("未发现可验证的 HTML 模板")
	}
}

func TestThemeRuntimeIncludesPersistenceAndAccessibility(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "theme.js"))
	if err != nil {
		t.Fatalf("读取主题脚本失败: %v", err)
	}
	script := string(content)
	for _, expected := range []string{
		"guccang-theme",
		"prefers-color-scheme: dark",
		"localStorage.setItem",
		"data-theme-picker",
		"option.dataset.themeOption = key",
		"renderOptions",
		"if (value === 'dark') return 'terminal'",
		"if (value === 'light') return 'watercolor'",
		"return savedTheme() || 'classic'",
		"root.dataset.theme = normalized",
		"墨纸经典",
		"夜间终端",
		"水彩小馆",
		"aria-label",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("主题脚本缺少关键能力 %q", expected)
		}
	}
}

func TestThemeStylesMatchAllVisualContracts(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "theme.css"))
	if err != nil {
		t.Fatalf("读取主题样式失败: %v", err)
	}
	styles := strings.ToLower(string(content))
	for _, expected := range []string{
		`@import url("/css/visual_language.css?v=1")`,
		`@import url("/css/visual_symbols.css?v=1")`,
		`data-theme]`,
		`#f4f1e8`,
		`#25211d`,
		`#c84f35`,
		`data-theme="terminal"`,
		`#14100c`,
		`#e1a82f`,
		`#f15a29`,
		`#62acd0`,
		`#d37aa2`,
		`data-theme="watercolor"`,
		`#fbf7ec`,
		`#4056b5`,
		`#dc5a3c`,
		`#d79a2b`,
		`#bfd8e8`,
		`/images/theme/watercolor-studio.png`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("主题样式缺少设计契约 %q", expected)
		}
	}
	assetPath := filepath.Join("..", "..", "statics", "images", "theme", "watercolor-studio.png")
	if info, err := os.Stat(assetPath); err != nil {
		t.Fatalf("水彩主题主视觉资产不可用: %v", err)
	} else if info.Size() == 0 {
		t.Fatal("水彩主题主视觉资产为空")
	}
}

func TestVisualLanguageDefinesSharedComponentsAndSemanticSymbols(t *testing.T) {
	cssDir := filepath.Join("..", "..", "statics", "css")
	visualLanguage, err := os.ReadFile(filepath.Join(cssDir, "visual_language.css"))
	if err != nil {
		t.Fatalf("读取全站视觉语言失败: %v", err)
	}
	visualSymbols, err := os.ReadFile(filepath.Join(cssDir, "visual_symbols.css"))
	if err != nil {
		t.Fatalf("读取视觉符号映射失败: %v", err)
	}

	language := string(visualLanguage)
	for _, expected := range []string{
		`.site-shell`, `.site-page-hero`, `.site-page-symbol`, `.site-paper-panel`,
		`.site-ticket`, `.site-seal-button`, `.site-empty-state`,
		`prefers-reduced-motion`,
	} {
		if !strings.Contains(language, expected) {
			t.Errorf("全站视觉语言缺少 %q", expected)
		}
	}

	symbols := string(visualSymbols)
	for _, expected := range []string{
		`data-symbol="portal"`, `data-symbol="tools"`, `data-symbol="products"`,
		`data-symbol="archive"`, `data-symbol="article"`, `data-symbol="search"`,
		`data-symbol="editor"`, `data-symbol="reading"`, `data-symbol="goal"`,
		`data-symbol="exercise"`, `data-symbol="account"`, `data-symbol="settings"`,
		`data-symbol="success"`, `data-symbol="warning"`, `data-symbol="error"`,
		`data-symbol="empty"`,
	} {
		if !strings.Contains(symbols, expected) {
			t.Errorf("视觉符号映射缺少 %q", expected)
		}
	}
}
