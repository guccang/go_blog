package http

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestDunhuangMotionLabOwnsTheme004Sample(t *testing.T) {
	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "dunhuang_motion_lab.template"))
	if err != nil {
		t.Fatalf("读取敦煌暮色实验室模板失败: %v", err)
	}
	page := string(templateContent)
	for _, expected := range []string{
		`id="dunhuangStage"`, `id="dunhuangCanvas"`, `data-dunhuang-asset="/images/visual-symbols/visual-symbols-01.webp"`,
		`id="windStrengthControl"`, `id="dustDensityControl"`, `id="gustButton"`,
		`id="ribbonAmplitudeControl"`, `id="ribbonTensionControl"`, `id="ribbonResponseControl"`,
		`id="lightStrengthControl"`, `id="lightXControl"`, `id="lightYControl"`, `id="lightAngleControl"`,
		`id="lightSourceValue"`, `data-light-source="point"`, `data-light-source="spot"`,
		`data-light-source="directional"`, `data-light-source="area"`,
		`id="breathingToggle"`, `id="breathStrengthControl"`, `id="breathPeriodControl"`,
		`id="tyndallToggle"`, `id="tyndallStrengthControl"`, `id="beamSpreadControl"`,
		`id="pauseButton"`, `id="resetButton"`,
		`data-wind-direction="north"`, `data-wind-direction="northeast"`,
		`data-wind-direction="east"`, `data-wind-direction="southeast"`,
		`data-wind-direction="south"`, `data-wind-direction="southwest"`,
		`data-wind-direction="west"`, `data-wind-direction="northwest"`,
		`/css/dunhuang_motion_lab.css?v=2`, `/js/dunhuang_motion_lab.js?v=4`,
		"远、中、近三层", "飘带", "呼吸月光", "丁达尔", "WebGL2",
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("敦煌暮色实验室缺少 %q", expected)
		}
	}
}

func TestDunhuangRendererUsesWebGL2NaturalPasses(t *testing.T) {
	staticDir := filepath.Join("..", "..", "statics")
	scriptContent, err := os.ReadFile(filepath.Join(staticDir, "js", "dunhuang_motion_lab.js"))
	if err != nil {
		t.Fatalf("读取敦煌暮色渲染器失败: %v", err)
	}
	stylesContent, err := os.ReadFile(filepath.Join(staticDir, "css", "dunhuang_motion_lab.css"))
	if err != nil {
		t.Fatalf("读取敦煌暮色样式失败: %v", err)
	}

	script := string(scriptContent)
	for _, expected := range []string{
		`getContext('webgl2'`, `dust_layer`, `beam_cluster`, `tyndall_scattering`, `dunhuang_sample`,
		`0.6 + texture_uv.x * 0.2`, `far_dust`, `middle_dust`, `near_dust`,
		`ribbon_region`, `u_ribbon_amplitude`, `u_ribbon_tension`, `u_ribbon_response`,
		`anchored_value`, `flowing_value`, `mix(anchored_value, flowing_value, ribbon_region * painted_ribbon)`,
		`u_dust_density`, `u_light_position`, `u_light_strength`, `u_light_type`, `u_light_angle`,
		`lightSources`, `light_direction`, `direct_light_field`, `path_to_light`,
		`gl.uniform1i(uniforms.u_light_type`, `this.settings.lightAngle * Math.PI / 180`,
		`breathing_light`, `u_breath_strength`, `u_breath_period`,
		`u_tyndall_strength`, `u_beam_spread`, `this.settings.tyndallEnabled ? this.settings.tyndallStrength : 0`,
		`windDirections`, `meteorologicalFlow`, `webglcontextlost`, `prefers-reduced-motion`,
		`敦煌暮色需要 WebGL2`, `着色器编译失败`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("敦煌暮色渲染器缺少 %q", expected)
		}
	}
	for _, forbidden := range []string{
		`getContext('2d')`, "Canvas2D", "fallback", "float active", "repeating-linear-gradient",
		`vec2 target_value = vec2(0.56, 0.12)`,
	} {
		if strings.Contains(script, forbidden) {
			t.Errorf("敦煌暮色样板不应包含 %q", forbidden)
		}
	}

	tyndallComposite := strings.Index(script, `float tyndall_value`)
	symbolComposite := strings.Index(script, `vec4 symbol_value`)
	if tyndallComposite < 0 || symbolComposite < 0 || symbolComposite <= tyndallComposite {
		t.Error("丁达尔散射应先形成环境光，再由人物与月轮遮挡，避免光束穿透主体")
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
		`.dunhuang-stage`, `#dunhuangCanvas`, `.lab-controls`, `.wind-compass`,
		`.ribbon-controls`, `.light-controls`, `.light-source-picker`, `.effect-toggle`, `[aria-checked="false"]`,
		`--night: #2b2533`, `--violet: #66506b`, `--clay: #b56a4c`,
		`--gold: #d9a441`, `--sand: #e7d2a1`,
		`@media (max-width: 640px)`, `prefers-reduced-motion`, `:focus-visible`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("敦煌暮色展示样式缺少 %q", expected)
		}
	}
}

func TestDunhuangMotionLabIsWiredIntoAtlas(t *testing.T) {
	checks := []struct {
		path     string
		expected string
	}{
		{path: "http_core.go", expected: `h.HandleFunc("/tools/dunhuang-motion-lab", HandleDunhuangMotionLab)`},
		{path: "http_lifecycle.go", expected: `view.PageDunhuangMotionLab(w)`},
		{path: "templates.go", expected: `dunhuang_motion_lab.template`},
		{path: filepath.Join("..", "..", "statics", "js", "visual_themes.js"), expected: `url: '/tools/dunhuang-motion-lab'`},
	}
	for _, check := range checks {
		content, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", check.path, err)
		}
		if !strings.Contains(string(content), check.expected) {
			t.Errorf("%s 缺少敦煌实验室接入点 %q", check.path, check.expected)
		}
	}
}
