package http

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
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
		`/js/celadon_rain_lab.js?v=10`, `/css/main.css?v=dunhuang-1`,
		`id="mainCeladonControls"`, `data-celadon-wind-direction`,
		`data-celadon-setting="wind"`, `data-celadon-setting="rain"`,
		`data-celadon-setting="lightPosition"`, `data-celadon-setting="lightVertical"`,
		`data-celadon-setting="lightAngle"`, `data-celadon-setting="lightRange"`,
		`data-celadon-light-source="point"`, `data-celadon-light-source="spot"`,
		`data-celadon-light-source="directional"`, `data-celadon-light-source="area"`,
		`data-celadon-toggle="breathingEnabled"`, `data-celadon-toggle="tyndallEnabled"`,
		`data-celadon-toggle="beamEnabled"`, `data-celadon-setting="beamCount"`,
		`data-celadon-action="impact"`, `data-celadon-action="pause"`, `data-celadon-action="reset"`,
		`main-celadon-controls__compact-label`, `main-celadon-controls__full-label`,
		`id="dunhuangStage"`, `id="dunhuangCanvas"`, `data-dunhuang-mode="ambient"`,
		`/js/dunhuang_motion_lab.js?v=2`, `id="mainDunhuangControls"`,
		`data-dunhuang-wind-direction`, `data-dunhuang-setting="ribbonAmplitude"`,
		`data-dunhuang-setting="lightX"`, `data-dunhuang-toggle="breathingEnabled"`,
		`data-dunhuang-toggle="tyndallEnabled"`, `data-dunhuang-action="gust"`,
		`main-dunhuang-controls__compact-label`, `main-dunhuang-controls__full-label`,
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
		`.query-hero[data-celadon-ready="true"] > .site-page-hero__exhibit`,
		`.main-celadon-controls`, `.main-celadon-controls__body`,
		`.main-dunhuang-stage`, `html[data-theme="atlas-dunhuang"] .main-dunhuang-stage`,
		`#dunhuangCanvas`, `.main-dunhuang-stage[data-render-state="error"]`,
		`.query-hero[data-dunhuang-ready="true"] > .site-page-hero__exhibit`,
		`.main-dunhuang-controls`, `.main-dunhuang-controls__body`,
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
		`celadonSettingsPreset`, `celadonNumericSchema`, `celadonBooleanSettings`,
		`wind: { min: 0, max: 2, step: 0.05, initial: 1`,
		`mediumDensity: { min: 0.15, max: 1.25`,
		`celadonStorageKey`, `guccang-celadon-rain-settings`, `legacyAmbientStorageKey`,
		`restoreCeladonPreferences`, `saveCeladonPreferences`, `clearCeladonPreferences`,
		`labNumericBindings`, `applyCeladonSchema`, `window.localStorage.setItem`,
		`bindAmbientControls`, `syncAmbientControls`, `setAmbientWindDirection`,
		`setAmbientPaused`, `resetAmbientSettings`, `window.localStorage.removeItem`,
		`setAmbientReady(true)`, `setAmbientReady(false)`, `ambientPaused`,
		`syncAmbientInspectorLayout`, `ambientControls.addEventListener('toggle'`,
		`ambientHero.dataset.celadonInspector`, `self.resize()`,
		`ambientControls.contains(event.target)`,
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
	settings := regexp.MustCompile(`data-celadon-setting="([^"]+)"`).FindAllStringSubmatch(page, -1)
	if len(settings) != 15 {
		t.Errorf("main celadon controls should expose 15 numeric settings, got %d", len(settings))
	}
	for _, setting := range settings {
		if !strings.Contains(page, `data-celadon-output="`+setting[1]+`"`) {
			t.Errorf("main celadon setting %q is missing its output", setting[1])
		}
	}
	celadonStart := strings.Index(page, `id="mainCeladonControls"`)
	dunhuangStart := strings.Index(page, `id="mainDunhuangControls"`)
	if celadonStart < 0 || dunhuangStart <= celadonStart {
		t.Fatal("main celadon and dunhuang controls are not ordered")
	}
	celadonPage := page[celadonStart:dunhuangStart]
	if directions := len(regexp.MustCompile(`<option value="(?:north|northeast|east|southeast|south|southwest|west|northwest)"`).FindAllString(celadonPage, -1)); directions != 8 {
		t.Errorf("main celadon wind selector should expose 8 directions, got %d", directions)
	}
	if sources := len(regexp.MustCompile(`data-celadon-light-source="(?:point|spot|directional|area)"`).FindAllString(page, -1)); sources != 4 {
		t.Errorf("main celadon source picker should expose 4 light sources, got %d", sources)
	}
	if toggles := len(regexp.MustCompile(`data-celadon-toggle="(?:beamEnabled|breathingEnabled|tyndallEnabled)"`).FindAllString(page, -1)); toggles != 3 {
		t.Errorf("main celadon controls should expose 3 effect toggles, got %d", toggles)
	}

	labContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "natural_motion_lab.template"))
	if err != nil {
		t.Fatalf("read natural motion lab template: %v", err)
	}
	inputPattern := regexp.MustCompile(`<input[^>]*data-celadon-setting="([^"]+)"[^>]*min="([^"]+)"[^>]*max="([^"]+)"[^>]*value="([^"]+)"[^>]*step="([^"]+)"`)
	parseInputs := func(content string) map[string]string {
		result := make(map[string]string)
		for _, match := range inputPattern.FindAllStringSubmatch(content, -1) {
			result[match[1]] = strings.Join(match[2:], "|")
		}
		return result
	}
	mainInputs := parseInputs(page)
	labInputs := parseInputs(string(labContent))
	if len(mainInputs) != 15 || len(labInputs) != 15 {
		t.Fatalf("main and lab should each define 15 numeric inputs, got main=%d lab=%d", len(mainInputs), len(labInputs))
	}
	for key, mainDefinition := range mainInputs {
		if labDefinition := labInputs[key]; labDefinition != mainDefinition {
			t.Errorf("celadon setting %q differs: main=%q lab=%q", key, mainDefinition, labDefinition)
		}
	}
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
		`.query-hero[data-celadon-ready="true"] > .site-page-hero__exhibit`,
		`visibility: hidden`, `.main-celadon-controls`,
		`[data-celadon-inspector="open"]`, `grid-template-columns: minmax(0, 1fr) 292px`,
		`right: 312px`, `grid-template-rows: 280px auto`, `grid-row: 2`,
		`max-height: min(42vh, 260px)`, `height: 220px`,
		`.main-celadon-controls:not([open])`, `width: 128px`,
		`.main-celadon-controls__compact-label`, `.main-celadon-source-picker`,
		`@media (prefers-reduced-motion: reduce)`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("main celadon presentation missing %q", expected)
		}
	}
	if regexp.MustCompile(`(?s)\.main-celadon-controls\s*\{\s*position:\s*fixed`).MatchString(styles) {
		t.Error("main celadon controls must not use the viewport-covering fixed drawer")
	}
	for _, forbidden := range []string{"ambientSettingsPreset", "ambientNumericLimits", "restoreAmbientPreferences", "saveAmbientPreferences"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("celadon renderer still contains divergent ambient configuration %q", forbidden)
		}
	}
}

func TestMainDunhuangUsesSharedWebGL2AmbientMode(t *testing.T) {
	scriptContent, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "dunhuang_motion_lab.js"))
	if err != nil {
		t.Fatalf("read dunhuang renderer: %v", err)
	}
	script := string(scriptContent)
	for _, expected := range []string{
		`getContext('webgl2'`, `ambientMode`, `stage.dataset.dunhuangMode`,
		`this.ambient = ambientMode`, `bindAmbientMode`, `setAmbientTheme`,
		`guccang:themechange`, `theme === 'atlas-dunhuang'`, `window.cancelAnimationFrame`,
		`dunhuangSettingsPreset`, `dunhuangNumericSchema`, `dunhuangBooleanSettings`,
		`windStrength: { min: 0, max: 2, step: 0.05, initial: 0.9`,
		`beamSpread: { min: 0.08, max: 0.65`,
		`guccang-dunhuang-motion-settings`, `restoreDunhuangPreferences`,
		`saveDunhuangPreferences`, `clearDunhuangPreferences`, `labNumericBindings`,
		`bindAmbientControls`, `syncAmbientControls`, `setAmbientWindDirection`,
		`setAmbientPaused`, `resetAmbientSettings`, `syncAmbientInspectorLayout`,
		`ambientHero.dataset.dunhuangInspector`, `setAmbientReady(true)`, `setAmbientReady(false)`,
		`ambientControls.contains(event.target)`, `this.assetReady`,
		`dust_layer`, `far_dust`, `middle_dust`, `near_dust`, `ribbon_region`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("dunhuang ambient renderer missing %q", expected)
		}
	}
	for _, forbidden := range []string{`getContext('2d')`, "Canvas2DRenderer", "fallback"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("dunhuang ambient renderer must hard-stop instead of using %q", forbidden)
		}
	}

	mainContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "main.template"))
	if err != nil {
		t.Fatalf("read main template: %v", err)
	}
	labContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "dunhuang_motion_lab.template"))
	if err != nil {
		t.Fatalf("read dunhuang lab template: %v", err)
	}
	mainPage := string(mainContent)
	labPage := string(labContent)
	inputPattern := regexp.MustCompile(`<input[^>]*data-dunhuang-setting="([^"]+)"[^>]*min="([^"]+)"[^>]*max="([^"]+)"[^>]*value="([^"]+)"[^>]*step="([^"]+)"`)
	parseInputs := func(content string) map[string]string {
		result := make(map[string]string)
		for _, match := range inputPattern.FindAllStringSubmatch(content, -1) {
			result[match[1]] = strings.Join(match[2:], "|")
		}
		return result
	}
	mainInputs := parseInputs(mainPage)
	labInputs := parseInputs(labPage)
	if len(mainInputs) != 12 || len(labInputs) != 12 {
		t.Fatalf("main and dunhuang lab should each define 12 numeric inputs, got main=%d lab=%d", len(mainInputs), len(labInputs))
	}
	for key, mainDefinition := range mainInputs {
		if labDefinition := labInputs[key]; labDefinition != mainDefinition {
			t.Errorf("dunhuang setting %q differs: main=%q lab=%q", key, mainDefinition, labDefinition)
		}
		if !strings.Contains(mainPage, `data-dunhuang-output="`+key+`"`) {
			t.Errorf("main dunhuang setting %q is missing its output", key)
		}
	}
	dunhuangStart := strings.Index(mainPage, `id="mainDunhuangControls"`)
	if dunhuangStart < 0 {
		t.Fatal("main dunhuang controls missing")
	}
	dunhuangPage := mainPage[dunhuangStart:]
	if directions := len(regexp.MustCompile(`<option value="(?:north|northeast|east|southeast|south|southwest|west|northwest)"`).FindAllString(dunhuangPage, -1)); directions != 8 {
		t.Errorf("main dunhuang wind selector should expose 8 directions, got %d", directions)
	}
	if toggles := len(regexp.MustCompile(`data-dunhuang-toggle="(?:breathingEnabled|tyndallEnabled)"`).FindAllString(mainPage, -1)); toggles != 2 {
		t.Errorf("main dunhuang controls should expose 2 effect toggles, got %d", toggles)
	}

	stylesContent, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "main.css"))
	if err != nil {
		t.Fatalf("read main styles: %v", err)
	}
	styles := string(stylesContent)
	for _, expected := range []string{
		`html[data-theme="atlas-dunhuang"] .query-hero[data-dunhuang-ready="true"]::after`,
		`.query-hero[data-dunhuang-ready="true"] > .site-page-hero__exhibit`,
		`[data-dunhuang-inspector="open"]`, `grid-template-columns: minmax(0, 1fr) 292px`,
		`right: 312px`, `grid-template-rows: 280px auto`, `grid-row: 2`,
		`.main-dunhuang-controls:not([open])`, `width: 128px`,
		`.main-dunhuang-controls__compact-label`, `@media (prefers-reduced-motion: reduce)`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("main dunhuang presentation missing %q", expected)
		}
	}
	if regexp.MustCompile(`(?s)\.main-dunhuang-controls\s*\{\s*position:\s*fixed`).MatchString(styles) {
		t.Error("main dunhuang controls must not use the viewport-covering fixed drawer")
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
