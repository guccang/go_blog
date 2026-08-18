package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductsPageUsesUnifiedVisualLanguage(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "products.template"))
	if err != nil {
		t.Fatalf("读取产品库模板失败: %v", err)
	}
	page := string(content)
	for _, expected := range []string{
		`data-page="products"`, `class="product-header site-shell site-hanging-nav"`,
		`class="product-hero site-page-hero"`, `data-symbol="products"`,
		`class="library site-section"`, `/css/products.css?v=unified-1`,
		`id="scanForm"`, `id="productGrid"`, `id="productDialog"`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("产品库缺少 %q", expected)
		}
	}

	styles, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "products.css"))
	if err != nil {
		t.Fatalf("读取产品库样式失败: %v", err)
	}
	css := string(styles)
	for _, expected := range []string{
		`--canvas: var(--ui-canvas)`, `data-page="products"`,
		`.product-symbol`, `.scan-console`, `.product-card`, `.product-dialog`,
		`.scan-jobs-heading strong { color: var(--ui-text-soft); }`,
		`.scan-job div strong { color: var(--ui-text); }`,
		`.scan-job-status { color: var(--ui-text-muted); }`,
	} {
		if !strings.Contains(css, expected) {
			t.Errorf("产品库统一样式缺少 %q", expected)
		}
	}
}
