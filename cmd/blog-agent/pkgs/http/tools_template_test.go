package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolsPageUsesUnifiedVisualLanguage(t *testing.T) {
	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "tools.template"))
	if err != nil {
		t.Fatalf("读取工具页模板失败: %v", err)
	}
	page := string(templateContent)
	for _, expected := range []string{
		`data-page="tools"`, `class="tools-header site-page-hero"`,
		`class="site-page-symbol" data-symbol="tools"`,
		`class="tool-nav-grid site-section"`, `/css/tools.css?v=unified-1`,
		`data-tool="time"`, `data-tool="pi-usage"`, `href="/tools/visual-themes"`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("工具页缺少 %q", expected)
		}
	}

	styles, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "tools.css"))
	if err != nil {
		t.Fatalf("读取工具页样式失败: %v", err)
	}
	css := string(styles)
	for _, expected := range []string{
		`data-page="tools"`, `var(--ui-canvas)`, `var(--ui-coral)`,
		`.tool-nav-card`, `.tool-card`,
	} {
		if !strings.Contains(css, expected) {
			t.Errorf("工具页统一样式缺少 %q", expected)
		}
	}
}
