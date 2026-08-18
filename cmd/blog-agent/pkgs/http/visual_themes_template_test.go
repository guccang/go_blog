package http

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVisualThemeAtlasContainsOneHundredThemes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "visual_themes.js"))
	if err != nil {
		t.Fatalf("读取视觉主题数据失败: %v", err)
	}

	script := string(content)
	rowPattern := regexp.MustCompile(`(?m)^\s*\['([0-9]{3})',`)
	rows := rowPattern.FindAllStringSubmatch(script, -1)
	if len(rows) != 100 {
		t.Fatalf("视觉主题数量应为 100，实际为 %d", len(rows))
	}
	for index, row := range rows {
		expected := fmt.Sprintf("%03d", index+1)
		if row[1] != expected {
			t.Errorf("第 %d 个主题编号应为 %s，实际为 %s", index+1, expected, row[1])
		}
	}

	colors := regexp.MustCompile(`#[0-9A-F]{6}`).FindAllString(script, -1)
	if len(colors) != 500 {
		t.Errorf("100 款主题应各含 5 个色值，期望 500 个，实际为 %d", len(colors))
	}
	for _, expected := range []string{
		"oriental", "masters", "nature", "cinema", "material",
		"editorial", "digital", "subculture", "craft", "quiet",
		"copyColor", "renderGallery", "openTheme",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("视觉主题脚本缺少 %q", expected)
		}
	}
}

func TestVisualThemeAtlasPageIsWiredIntoTools(t *testing.T) {
	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "visual_themes.template"))
	if err != nil {
		t.Fatalf("读取视觉主题模板失败: %v", err)
	}
	page := string(templateContent)
	for _, expected := range []string{
		`CHROMA 100`,
		`id="theme-gallery"`,
		`/css/visual_themes.css?v=1`,
		`/js/visual_themes.js?v=1`,
		`/js/theme.js?v=3`,
		`/css/theme.css?v=3`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("视觉主题模板缺少 %q", expected)
		}
	}

	checks := []struct {
		path     string
		expected string
	}{
		{path: "http_core.go", expected: `h.HandleFunc("/tools/visual-themes", HandleVisualThemes)`},
		{path: "http_lifecycle.go", expected: `view.PageVisualThemes(w)`},
		{path: "templates.go", expected: `PageVisualThemes:       PageVisualThemes`},
		{path: filepath.Join("..", "..", "templates", "tools.template"), expected: `href="/tools/visual-themes"`},
	}
	for _, check := range checks {
		content, readErr := os.ReadFile(check.path)
		if readErr != nil {
			t.Fatalf("读取 %s 失败: %v", check.path, readErr)
		}
		if !strings.Contains(string(content), check.expected) {
			t.Errorf("%s 缺少接入点 %q", check.path, check.expected)
		}
	}
}
