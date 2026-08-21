package http

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMainTemplateKeepsOnlyPrimaryJobs(t *testing.T) {
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", "main.template"))
	if err != nil {
		t.Fatalf("parse main.template: %v", err)
	}
	var output bytes.Buffer
	err = tmpl.Execute(&output, MainPageData{
		USER_ACCOUNT: "ztt",
		USER_AVATAR:  "Z",
		RECENT_LINKS: []LinkData{{
			URL: "/get?blogname=SQLite", DESC: "SQLite", ACCESS_TIME: "15:20",
			PREVIEW: "迁移过程与关键决定", IMAGE_URL: "/media/1234567890abcdef1234567890abcdef",
		}},
	})
	if err != nil {
		t.Fatalf("execute main.template: %v", err)
	}
	page := output.String()
	for _, expected := range []string{
		`id="workspaceQueryForm"`, `id="askPIButton"`, "继续阅读", "快速开始", `href="/link"`,
		`href="/products"`, "产品库",
		`data-page="main"`, `class="query-hero site-page-hero"`, `data-symbol="portal"`, `data-symbol-motion="portal"`,
		`/js/visual_symbol_motion.js?v=1`,
		`class="quick-section site-section"`, `class="continue-section site-section"`,
		`class="recent-grid"`, `class="recent-card has-media"`, `loading="lazy"`, "迁移过程与关键决定",
		`class="daily-quote"`, `id="dailyQuote"`, "今日格言",
		`id="celadonStage"`, `id="celadonCanvas"`, `data-celadon-mode="ambient"`,
		`/js/celadon_rain_lab.js?v=7`, `/css/main.css?v=celadon-rain-3`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("main page missing %q", expected)
		}
	}
	for _, removed := range []string{
		`class="sidebar"`, `id="blogContainer"`, "博客数量:", "search-command-select",
		"data-natural-motion", "natural_motion.js", "natural_motion.css", "main-theme-symbol--celadon",
	} {
		if strings.Contains(page, removed) {
			t.Fatalf("main page still contains removed module %q", removed)
		}
	}
	quickIndex := strings.Index(page, `class="quick-section site-section"`)
	queryIndex := strings.Index(page, `class="query-hero site-page-hero"`)
	continueIndex := strings.Index(page, `class="continue-section site-section"`)
	if quickIndex < 0 || queryIndex < 0 || continueIndex < 0 || !(queryIndex < quickIndex && quickIndex < continueIndex) {
		t.Fatalf("main sections are not ordered page portal, quick start, continue reading")
	}
}

func TestBuildMainBlogPreviewUsesLocalImageAndReadableLines(t *testing.T) {
	content := "# SQLite\n\n![架构](/media/1234567890abcdef1234567890abcdef)\n\n第一行介绍。\n- 第二行重点\n[第三行链接](/get?blogname=other)"
	preview, imageURL := buildMainBlogPreview("SQLite", content)
	if imageURL != "/media/1234567890abcdef1234567890abcdef" {
		t.Fatalf("image URL = %q", imageURL)
	}
	if preview != "第一行介绍。 第二行重点 第三行链接" {
		t.Fatalf("preview = %q", preview)
	}
}

func TestBuildMainBlogPreviewIgnoresExternalImageAndTruncatesRunes(t *testing.T) {
	content := "![外链](https://example.com/a.png)\n" + strings.Repeat("中文", 80)
	preview, imageURL := buildMainBlogPreview("长文", content)
	if imageURL != "" {
		t.Fatalf("external image should be ignored, got %q", imageURL)
	}
	if len([]rune(preview)) != 141 || !strings.HasSuffix(preview, "…") {
		t.Fatalf("preview should contain 140 runes and ellipsis, got %d: %q", len([]rune(preview)), preview)
	}
}

func TestMainStylesUseResponsiveReadingCards(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "main.css"))
	if err != nil {
		t.Fatalf("read main.css: %v", err)
	}
	styles := string(content)
	for _, expected := range []string{
		"var(--ui-canvas)",
		"var(--ui-coral)",
		"grid-template-columns: repeat(2, minmax(0, 1fr))",
		".recent-card-image",
		"object-fit: cover",
		".recent-grid { grid-template-columns: 1fr; }",
		`:root[data-theme="watercolor"] .quick-item.primary { color: var(--ui-text); }`,
		`:root[data-theme="watercolor"] .quick-item.primary small { color: var(--ui-text-muted); }`,
		`.main-celadon-stage`, `html[data-theme="atlas-celadon"] .main-celadon-stage`,
		`#celadonCanvas`, `.main-celadon-stage[data-render-state="error"]`,
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("main.css missing %q", expected)
		}
	}

	language, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "visual_language.css"))
	if err != nil {
		t.Fatalf("read visual_language.css: %v", err)
	}
	if !strings.Contains(string(language), ".site-page-hero__exhibit") {
		t.Fatal("visual_language.css missing \".site-page-hero__exhibit\"")
	}
}

func TestMainCeladonRainUsesSharedWebGL2AmbientMode(t *testing.T) {
	scriptContent, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "celadon_rain_lab.js"))
	if err != nil {
		t.Fatalf("read celadon renderer: %v", err)
	}
	script := string(scriptContent)
	for _, expected := range []string{
		`getContext('webgl2'`, `ambientMode`, `stage.dataset.celadonMode`,
		`this.ambient = ambientMode`, `bindAmbientMode`, `setAmbientTheme`,
		`guccang:themechange`, `theme === 'atlas-celadon'`, `window.cancelAnimationFrame`,
		`document.addEventListener('pointermove'`, `document.addEventListener('pointerdown'`,
		`point.inside && point.y <= self.waterline + 0.1`,
		`u_vessel_position`, `u_vessel_scale`, `this.assetReady`,
		`wind: this.ambient ? 0.72 : 1`, `rain: this.ambient ? 0.68 : 1`,
		`tyndallStrength: this.ambient ? 0.46 : 0.75`,
		`far_rain`, `middle_rain`, `near_rain`, `stepSimulation`, `refraction_shift`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("celadon ambient renderer missing %q", expected)
		}
	}
	for _, forbidden := range []string{`getContext('2d')`, "Canvas2DRenderer", "fallback"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("celadon ambient renderer must hard-stop instead of using %q", forbidden)
		}
	}

	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "main.template"))
	if err != nil {
		t.Fatalf("read main template: %v", err)
	}
	page := string(templateContent)
	for _, forbidden := range []string{`id="windControl"`, `id="rainControl"`, `id="lightInstrument"`, `id="pauseButton"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("main page must not expose lab control %q", forbidden)
		}
	}

	stylesContent, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "main.css"))
	if err != nil {
		t.Fatalf("read main styles: %v", err)
	}
	styles := string(stylesContent)
	for _, expected := range []string{
		`html[data-theme="atlas-celadon"] .query-hero::after`,
		`linear-gradient(90deg`, `pointer-events: none`,
		`.main-celadon-stage[data-render-state="ready"] ~ .site-page-hero__exhibit`,
		`height: 220px`, `@media (prefers-reduced-motion: reduce)`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("main celadon presentation missing %q", expected)
		}
	}
}

func TestQueryTemplatesKeepResultsOnSecondaryPages(t *testing.T) {
	for _, name := range []string{"search_results.template", "ask.template"} {
		tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", name))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		var output bytes.Buffer
		if err := tmpl.Execute(&output, QueryPageData{QUERY: "灵妖", USER_ACCOUNT: "ztt", USER_AVATAR: "Z"}); err != nil {
			t.Fatalf("execute %s: %v", name, err)
		}
		if !strings.Contains(output.String(), "灵妖") {
			t.Fatalf("%s did not retain query", name)
		}
	}
}

func TestLibraryTemplateContainsOnlyLibraryTools(t *testing.T) {
	tmpl, err := template.ParseFiles(filepath.Join("..", "..", "templates", "link.template"))
	if err != nil {
		t.Fatalf("parse link.template: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, LinkDatas{
		BLOGS_NUMBER: 1,
		USER_AVATAR:  "Z",
		LINKS: []LinkData{{
			URL: "/get?blogname=SQLite", DESC: "SQLite", ACCESS_TIME: "15:20",
			PREVIEW: "迁移过程与关键决定", IMAGE_URL: "/media/1234567890abcdef1234567890abcdef",
		}},
	}); err != nil {
		t.Fatalf("execute link.template: %v", err)
	}
	page := output.String()
	for _, expected := range []string{
		`id="blogContainer"`, `id="blogFilterBar"`, "搜索博客或输入高级命令",
		`class="blog-card has-media"`, `class="blog-card-image"`, `loading="lazy"`,
		"迁移过程与关键决定",
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("library page missing %q", expected)
		}
	}
	for _, removed := range []string{"问我的博客", "最近查看", "今日格言", `class="sidebar"`} {
		if strings.Contains(page, removed) {
			t.Fatalf("library page still contains %q", removed)
		}
	}
}

func TestRecentTimeLabel(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 0, time.Local)
	tests := []struct {
		access string
		modify string
		want   string
	}{
		{access: "2026-07-29 15:20:00", want: "15:20"},
		{access: "2026-07-28 22:00:00", want: "昨天"},
		{access: "2026-03-16 17:01:07", want: "03-16"},
		{access: "", modify: "2025-12-31 09:00:00", want: "2025-12-31"},
	}
	for _, test := range tests {
		if got := recentTimeLabel(test.access, test.modify, now); got != test.want {
			t.Fatalf("recentTimeLabel(%q, %q) = %q, want %q", test.access, test.modify, got, test.want)
		}
	}
}
