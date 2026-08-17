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

		for _, asset := range []string{`/js/theme.js?v=2`, `/css/theme.css?v=2`} {
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
		"data-theme-option=\"terminal\"",
		"data-theme-option=\"watercolor\"",
		"if (value === 'dark') return 'terminal'",
		"if (value === 'light') return 'watercolor'",
		"夜间终端",
		"水彩小馆",
		"aria-label",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("主题脚本缺少关键能力 %q", expected)
		}
	}
}

func TestThemeStylesMatchBothVisualContracts(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "theme.css"))
	if err != nil {
		t.Fatalf("读取主题样式失败: %v", err)
	}
	styles := strings.ToLower(string(content))
	for _, expected := range []string{
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
