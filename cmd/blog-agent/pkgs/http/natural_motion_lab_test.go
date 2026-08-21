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
		`id="lightInstrument"`, `data-source="point"`, `data-beams="on"`, `data-breathing="on"`, `data-tyndall="on"`,
		`id="lightSourceValue"`, `id="lightSourceDetail"`, `data-light-source="point"`,
		`data-light-source="spot"`, `data-light-source="directional"`, `data-light-source="area"`,
		`id="sourceIntensityControl"`, `id="lightPositionControl"`, `id="lightVerticalControl"`,
		`id="lightAngleControl"`, `id="lightRangeControl"`,
		`id="beamToggle"`, `id="beamCountControl"`, `id="beamSpreadControl"`, `id="beamSoftnessControl"`,
		`id="breathingToggle"`, `id="breathStrengthControl"`, `id="breathPeriodControl"`,
		`id="tyndallToggle"`, `id="tyndallStrengthControl"`, `id="mediumDensityControl"`,
		`id="impactButton"`, `id="pauseButton"`, `id="resetButton"`,
		`/js/theme.js?v=7`, `/css/theme.css?v=7`,
		`/css/celadon_rain_lab.css?v=6`, `/js/celadon_rain_lab.js?v=7`,
		"北风 0°，N → S", "东风 90°，E → W", "南风 180°，S → N", "西风 270°，W → E",
		"东北风 45°，NE → SW", "东南风 135°，SE → NW",
		"西南风 225°，SW → NE", "西北风 315°，NW → SE",
		"点光源", "聚光", "平行", "面光", "光源强度", "垂直位置", "光束簇", "光束数量",
		"丁达尔开启后显现多束体积光", "呼吸光", "介质散射", "丁达尔现象独立于光源", "右上 28°", "已开启",
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
		`lightSources`, `setLightSource`, `lightSource`,
		`point: { uniform: 0`, `spot: { uniform: 1`, `directional: { uniform: 2`, `area: { uniform: 3`,
		`u_light_type`, `u_light_position`, `u_light_angle`, `u_light_range`, `u_source_intensity`,
		`direct_light_field`, `u_light_type == 0`, `u_light_type == 1`, `u_light_type == 2`,
		`area_axes`, `path_to_light`,
		`u_breath_strength`, `u_breath_period`, `breathing_light`, `cycle_value`,
		`inhale_value`, `exhale_value`, `envelope_value`, `direct_light = u_source_intensity * breath_value`,
		`u_beam_enabled`, `u_beam_count`, `u_beam_spread`, `u_beam_softness`,
		`beam_cluster_field`, `beam_index < 7`, `cluster_contrast`, `beam_geometry`,
		`u_tyndall_strength`, `u_medium_density`, `tyndall_scattering`, `volume_density`,
		`volume_index < 8`, `vessel_shadow`, `shadow_index < 4`, `rain_in_light`,
		`formatLightAngle`, `syncLightControls`, `aria-checked`, `sourceIntensity`, `lightVertical`,
		`beamEnabled`, `beamCount`, `beamSpread`, `beamSoftness`, `breathingEnabled`, `tyndallEnabled`,
		`webglcontextlost`, `prefers-reduced-motion`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("青瓷雨渲染器缺少 %q", expected)
		}
	}
	for _, forbidden := range []string{
		`getContext('2d')`, "Canvas2DRenderer", "registerEffect", "registerPreset", "float active",
		"broad_shafts", "fine_shafts", "shaft_pattern", "u_tyndall_angle", "formatTyndallAngle",
	} {
		if strings.Contains(script, forbidden) {
			t.Errorf("青瓷雨样板不应包含 %q", forbidden)
		}
	}
	waterComposite := strings.Index(script, `float water_depth`)
	scatteringComposite := strings.Index(script, `float tyndall_value`)
	if waterComposite < 0 || scatteringComposite < 0 || scatteringComposite <= waterComposite {
		t.Error("丁达尔散射必须在器物与水面之后参与最终合成，避免被后续颜色覆盖")
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
		`.celadon-stage`, `min-height: clamp(960px, 96vh, 1120px)`, `.lab-controls`,
		`.wind-compass`, `.wind-compass__dial`, `.wind-compass__needle`,
		`.wind-direction--northeast`, `.wind-direction--southeast`,
		`.wind-direction--southwest`, `.wind-direction--northwest`, `--wind-flow-angle`,
		`.stage-instruments`, `.light-controls`, `.light-control-body`, `.light-source`,
		`.light-source-picker`, `.source-sliders`, `.light-effect`, `.effect-toggle`,
		`[aria-checked="false"]`, `[data-beams="off"]`, `[data-breathing="off"]`, `[data-tyndall="off"]`,
		`min-height: 1320px`,
		`--paper: #f4eedf`, `--celadon: #7fa99a`, `--deep-water: #3f6f66`,
		`@media (max-width: 640px)`, `prefers-reduced-motion`, `:focus-visible`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("青瓷雨展示样式缺少 %q", expected)
		}
	}
}

func TestNaturalMotionLabIsWiredIntoAtlasOnly(t *testing.T) {
	checks := []struct {
		path     string
		expected string
	}{
		{path: "http_core.go", expected: `h.HandleFunc("/tools/natural-motion-lab", HandleNaturalMotionLab)`},
		{path: "http_lifecycle.go", expected: `view.PageNaturalMotionLab(w)`},
		{path: "templates.go", expected: `natural_motion_lab.template`},
		{path: filepath.Join("..", "..", "statics", "js", "visual_themes.js"), expected: `url: '/tools/natural-motion-lab'`},
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

	toolsContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "tools.template"))
	if err != nil {
		t.Fatalf("读取工具页模板失败: %v", err)
	}
	if strings.Contains(string(toolsContent), `href="/tools/natural-motion-lab"`) {
		t.Error("青瓷雨实验室入口应从工具页迁移到视觉主题图鉴")
	}
}
