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
		"copyColor", "renderGallery", "openTheme", "symbolVariables", "symbol-image",
		"visual-symbols-", "sheet: Math.floor(index / 10) + 1", "cell: index % 10",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("视觉主题脚本缺少 %q", expected)
		}
	}
	if strings.Contains(script, "var compositions") || strings.Contains(script, "data-composition") {
		t.Error("视觉主题脚本不应继续使用重复几何图块组合")
	}

	assets, err := filepath.Glob(filepath.Join("..", "..", "statics", "images", "visual-symbols", "visual-symbols-*.webp"))
	if err != nil {
		t.Fatalf("读取视觉符号资源失败: %v", err)
	}
	if len(assets) != 10 {
		t.Fatalf("100 枚视觉符号应由 10 张 image-2 精灵图承载，实际为 %d 张", len(assets))
	}
	for _, asset := range assets {
		info, statErr := os.Stat(asset)
		if statErr != nil {
			t.Fatalf("读取视觉符号资源 %s 失败: %v", asset, statErr)
		}
		if info.Size() < 100_000 {
			t.Errorf("视觉符号资源 %s 文件过小，可能未正确生成", asset)
		}
	}
}

func TestVisualThemeAtlasAppliesImplementedThemes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "visual_themes.js"))
	if err != nil {
		t.Fatalf("读取视觉主题脚本失败: %v", err)
	}
	script := string(content)
	for _, expected := range []string{
		"implementedThemes",
		"'001': 'atlas-celadon'",
		"'051': 'atlas-swiss'",
		"'061': 'atlas-chrome'",
		"dialog-apply",
		"guccang-theme",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("图鉴应用能力缺少 %q", expected)
		}
	}

	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "visual_themes.template"))
	if err != nil {
		t.Fatalf("读取视觉主题模板失败: %v", err)
	}
	if !strings.Contains(string(templateContent), `id="dialog-apply"`) {
		t.Error("视觉主题模板缺少应用按钮")
	}
}

func TestVisualThemeAtlasKeepsUnapprovedMotionOutOfDialog(t *testing.T) {
	scriptContent, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "visual_themes.js"))
	if err != nil {
		t.Fatalf("读取视觉主题脚本失败: %v", err)
	}
	for _, removed := range []string{
		"dialogMotionScene", "destroyDialogMotion", "GuCcangNaturalMotion", "natural_motion.js",
	} {
		if strings.Contains(string(scriptContent), removed) {
			t.Errorf("主题弹窗不应运行未验收动效 %q", removed)
		}
	}

	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "visual_themes.template"))
	if err != nil {
		t.Fatalf("读取视觉主题模板失败: %v", err)
	}
	page := string(templateContent)
	if !strings.Contains(page, `/js/visual_themes.js?v=6`) {
		t.Error("视觉主题模板未更新脚本缓存版本")
	}
	for _, removed := range []string{"natural_motion.js", "natural_motion.css"} {
		if strings.Contains(page, removed) {
			t.Errorf("视觉主题模板不应加载未验收动效 %q", removed)
		}
	}
}

func TestVisualThemeAtlasOwnsMotionLabDirectory(t *testing.T) {
	scriptContent, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "visual_themes.js"))
	if err != nil {
		t.Fatalf("读取视觉主题脚本失败: %v", err)
	}
	script := string(scriptContent)
	for _, expected := range []string{
		"motionLabs",
		"'001': { url: '/tools/natural-motion-lab'",
		"'004': { url: '/tools/dunhuang-motion-lab'",
		"labFilterRoot", "activeLabFilter", "renderLabFilters",
		"data-lab-filter", "specimen-lab-mark", "dialog-lab", "theme.lab",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("视觉主题图鉴缺少实验室目录能力 %q", expected)
		}
	}
	if count := strings.Count(script, "url: '/tools/"); count != 2 {
		t.Errorf("当前应登记 2 个动效实验室，实际为 %d", count)
	}

	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "visual_themes.template"))
	if err != nil {
		t.Fatalf("读取视觉主题模板失败: %v", err)
	}
	page := string(templateContent)
	for _, expected := range []string{`id="lab-filters"`, `id="dialog-lab"`} {
		if !strings.Contains(page, expected) {
			t.Errorf("视觉主题模板缺少 %q", expected)
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
		`百象悬廊`,
		`THE HANGING ARCHIVE`,
		`id="theme-gallery"`,
		`data-hero-theme="006"`,
		`/css/visual_themes.css?v=5`,
		`/js/visual_themes.js?v=6`,
		`/js/theme.js?v=7`,
		`/css/theme.css?v=7`,
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
