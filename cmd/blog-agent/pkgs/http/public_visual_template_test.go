package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicBlogUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "public.template"))
	if err != nil {
		t.Fatalf("读取公开博客模板失败: %v", err)
	}
	page := string(content)
	for _, expected := range []string{
		`data-page="public"`, `class="top-nav-inner site-shell site-hanging-nav"`,
		`class="hero site-shell site-page-hero"`, `data-symbol="article"`,
		`class="blog-main site-shell site-section"`, `class="empty-state site-empty-state"`,
		`/css/public.css?v=unified-1`, `id="blog-container"`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("公开博客缺少 %q", expected)
		}
	}

	styles, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "public.css"))
	if err != nil {
		t.Fatalf("读取公开博客样式失败: %v", err)
	}
	css := string(styles)
	for _, expected := range []string{`--bg: var(--ui-canvas)`, `data-page="public"`, `.hero-stat`, `.blog-card`, `.page-footer`} {
		if !strings.Contains(css, expected) {
			t.Errorf("公开博客统一样式缺少 %q", expected)
		}
	}
}
