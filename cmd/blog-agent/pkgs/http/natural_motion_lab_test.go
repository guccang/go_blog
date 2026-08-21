package http

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestNaturalMotionLabOwnsCeladonRainSample(t *testing.T) {
	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "natural_motion_lab.template"))
	if err != nil {
		t.Fatalf("读取青瓷雨实验室模板失败: %v", err)
	}
	page := string(templateContent)
	for _, expected := range []string{
		`id="celadonStage"`, `id="celadonCanvas"`, `data-celadon-asset=`,
		`id="windControl"`, `id="rainControl"`, `id="lightControl"`,
		`id="windCompass"`, `id="windDirectionValue"`, `data-wind-direction="north"`,
		`data-wind-direction="northeast"`, `data-wind-direction="east"`, `data-wind-direction="southeast"`,
		`data-wind-direction="south"`, `data-wind-direction="southwest"`, `data-wind-direction="west"`,
		`data-wind-direction="northwest"`, `data-wind-angle="45"`, `data-wind-angle="135"`,
		`data-wind-angle="225"`, `data-wind-angle="315"`,
		`id="impactButton"`, `id="pauseButton"`, `id="resetButton"`,
		`/js/theme.js?v=7`, `/css/theme.css?v=7`,
		`/css/celadon_rain_lab.css?v=3`, `/js/celadon_rain_lab.js?v=3`,
		"北风 0°，N → S", "东风 90°，E → W", "南风 180°，S → N", "西风 270°，W → E",
		"东北风 45°，NE → SW", "东南风 135°，SE → NW",
		"西南风 225°，SW → NE", "西北风 315°，NW → SE",
		"风场", "三层", "水冠", "波面法线",
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("青瓷雨实验室缺少 %q", expected)
		}
	}
}

func TestCeladonRainRendererUsesPhysicalWebGL2Passes(t *testing.T) {
	staticDir := filepath.Join("..", "..", "statics")
	scriptContent, err := os.ReadFile(filepath.Join(staticDir, "js", "celadon_rain_lab.js"))
	if err != nil {
		t.Fatalf("读取青瓷雨渲染器失败: %v", err)
	}
	stylesContent, err := os.ReadFile(filepath.Join(staticDir, "css", "celadon_rain_lab.css"))
	if err != nil {
		t.Fatalf("读取青瓷雨样式失败: %v", err)
	}

	script := string(scriptContent)
	for _, expected := range []string{
		`getContext('webgl2'`, `EXT_color_buffer_float`, `gl.RGBA16F`,
		`createFramebuffer`, `laplacian`, `u_impulses`, `stepSimulation`,
		`wind_field`, `curl_value`, `u_wind_direction`, `u_wind_strength`,
		`read_wave_state`, `advection_offset`, `far_rain`, `middle_rain`, `near_rain`,
		`splash_shape`, `wave_normal`, `refraction_shift`, `paper_environment`,
		`setWindDirection`, `setWindAngle`, `updateWindDirection`, `meteorologicalFlow`,
		`normalizeWindAngle`, `shortestWindTurn`, `windDirections`,
		`north: { angle: 0`, `northeast: { angle: 45`, `east: { angle: 90`,
		`southeast: { angle: 135`, `south: { angle: 180`, `southwest: { angle: 225`,
		`west: { angle: 270`, `northwest: { angle: 315`,
		`webglcontextlost`, `prefers-reduced-motion`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("青瓷雨渲染器缺少 %q", expected)
		}
	}
	for _, forbidden := range []string{`getContext('2d')`, "Canvas2DRenderer", "registerEffect", "registerPreset", "float active"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("青瓷雨样板不应包含 %q", forbidden)
		}
	}

	literalSmoothstep := regexp.MustCompile(`smoothstep\(([0-9]+(?:\.[0-9]+)?),\s*([0-9]+(?:\.[0-9]+)?)`)
	for _, match := range literalSmoothstep.FindAllStringSubmatch(script, -1) {
		left, _ := strconv.ParseFloat(match[1], 64)
		right, _ := strconv.ParseFloat(match[2], 64)
		if left >= right {
			t.Errorf("着色器 smoothstep 边界必须递增: %s", match[0])
		}
	}

	styles := string(stylesContent)
	for _, expected := range []string{
		`.celadon-stage`, `min-height: clamp(620px, 78vh, 920px)`, `.lab-controls`,
		`.wind-compass`, `.wind-compass__dial`, `.wind-compass__needle`,
		`.wind-direction--northeast`, `.wind-direction--southeast`,
		`.wind-direction--southwest`, `.wind-direction--northwest`, `--wind-flow-angle`,
		`--paper: #f4eedf`, `--celadon: #7fa99a`, `--deep-water: #3f6f66`,
		`@media (max-width: 640px)`, `prefers-reduced-motion`, `:focus-visible`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("青瓷雨展示样式缺少 %q", expected)
		}
	}
}

func TestNaturalMotionLabIsWiredIntoToolsOnly(t *testing.T) {
	checks := []struct {
		path     string
		expected string
	}{
		{path: "http_core.go", expected: `h.HandleFunc("/tools/natural-motion-lab", HandleNaturalMotionLab)`},
		{path: "http_lifecycle.go", expected: `view.PageNaturalMotionLab(w)`},
		{path: "templates.go", expected: `natural_motion_lab.template`},
		{path: filepath.Join("..", "..", "templates", "tools.template"), expected: `href="/tools/natural-motion-lab"`},
	}
	for _, check := range checks {
		content, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", check.path, err)
		}
		if !strings.Contains(string(content), check.expected) {
			t.Errorf("%s 缺少实验室接入点 %q", check.path, check.expected)
		}
	}

	for _, pagePath := range []string{
		filepath.Join("..", "..", "templates", "main.template"),
		filepath.Join("..", "..", "templates", "visual_themes.template"),
	} {
		content, err := os.ReadFile(pagePath)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", pagePath, err)
		}
		if strings.Contains(string(content), "celadon_rain_lab") || strings.Contains(string(content), "natural_motion") {
			t.Errorf("未验收动效不应接入 %s", pagePath)
		}
	}
}
