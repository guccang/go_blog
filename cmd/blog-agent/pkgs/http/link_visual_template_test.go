package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlogLibraryUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "link.template"))
	if err != nil {
		t.Fatalf("读取博客库模板失败: %v", err)
	}
	page := string(content)
	for _, expected := range []string{
		`data-page="link"`, `class="library-header site-shell site-hanging-nav"`,
		`class="library-intro site-page-hero"`, `data-symbol="archive"`,
		`class="library-toolbar site-paper-panel"`, `class="section site-section"`,
		`/css/link.css?v=unified-1`, `id="blogContainer"`, `id="blogFilterBar"`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("博客库缺少 %q", expected)
		}
	}

	styles, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "link.css"))
	if err != nil {
		t.Fatalf("读取博客库样式失败: %v", err)
	}
	css := string(styles)
	for _, expected := range []string{
		`--bg: var(--ui-canvas)`, `data-page="link"`,
		`.library-count`, `.blog-filter-chip`, `.blog-card`, `.load-more-btn`,
	} {
		if !strings.Contains(css, expected) {
			t.Errorf("博客库统一样式缺少 %q", expected)
		}
	}
}
