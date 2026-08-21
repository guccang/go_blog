(function () {
    'use strict';

    var stage = document.getElementById('dunhuangStage');
    var canvas = document.getElementById('dunhuangCanvas');
    if (!stage || !canvas) return;
    var ambientMode = stage.dataset.dunhuangMode === 'ambient';
    var ambientHero = ambientMode ? stage.closest('.query-hero') : null;
    var ambientControls = ambientMode ? document.getElementById('mainDunhuangControls') : null;
    var dunhuangStorageKey = 'guccang-dunhuang-motion-settings';

    var statusTitle = stage.querySelector('[data-dunhuang-status-title]');
    var statusDetail = stage.querySelector('[data-dunhuang-status-detail]');
    var windStrengthControl = document.getElementById('windStrengthControl');
    var windStrengthValue = document.getElementById('windStrengthValue');
    var dustDensityControl = document.getElementById('dustDensityControl');
    var dustDensityValue = document.getElementById('dustDensityValue');
    var ribbonAmplitudeControl = document.getElementById('ribbonAmplitudeControl');
    var ribbonAmplitudeValue = document.getElementById('ribbonAmplitudeValue');
    var ribbonTensionControl = document.getElementById('ribbonTensionControl');
    var ribbonTensionValue = document.getElementById('ribbonTensionValue');
    var ribbonResponseControl = document.getElementById('ribbonResponseControl');
    var ribbonResponseValue = document.getElementById('ribbonResponseValue');
    var lightStrengthControl = document.getElementById('lightStrengthControl');
    var lightStrengthValue = document.getElementById('lightStrengthValue');
    var lightXControl = document.getElementById('lightXControl');
    var lightXValue = document.getElementById('lightXValue');
    var lightYControl = document.getElementById('lightYControl');
    var lightYValue = document.getElementById('lightYValue');
    var lightAngleControl = document.getElementById('lightAngleControl');
    var lightAngleValue = document.getElementById('lightAngleValue');
    var lightSourceValue = document.getElementById('lightSourceValue');
    var lightSourceDetail = document.getElementById('lightSourceDetail');
    var lightSourceButtons = Array.prototype.slice.call(stage.querySelectorAll('[data-light-source]'));
    var breathingToggle = document.getElementById('breathingToggle');
    var breathStrengthControl = document.getElementById('breathStrengthControl');
    var breathStrengthValue = document.getElementById('breathStrengthValue');
    var breathPeriodControl = document.getElementById('breathPeriodControl');
    var breathPeriodValue = document.getElementById('breathPeriodValue');
    var tyndallToggle = document.getElementById('tyndallToggle');
    var tyndallStrengthControl = document.getElementById('tyndallStrengthControl');
    var tyndallStrengthValue = document.getElementById('tyndallStrengthValue');
    var beamSpreadControl = document.getElementById('beamSpreadControl');
    var beamSpreadValue = document.getElementById('beamSpreadValue');
    var windCompass = document.getElementById('windCompass');
    var windDirectionValue = document.getElementById('windDirectionValue');
    var windDirectionDetail = document.getElementById('windDirectionDetail');
    var windDirectionButtons = Array.prototype.slice.call(stage.querySelectorAll('[data-wind-direction]'));
    var lightInstrument = document.getElementById('lightInstrument');
    var gustButton = document.getElementById('gustButton');
    var pauseButton = document.getElementById('pauseButton');
    var resetButton = document.getElementById('resetButton');

    var windDirections = {
        north: { angle: 0, label: '北风 N → S', detail: '来自北方，向南方与近景流动。' },
        northeast: { angle: 45, label: '东北风 NE → SW', detail: '来自东北，向西南方向流动。' },
        east: { angle: 90, label: '东风 E → W', detail: '来自东方，向西方流动。' },
        southeast: { angle: 135, label: '东南风 SE → NW', detail: '来自东南，向西北方向流动。' },
        south: { angle: 180, label: '南风 S → N', detail: '来自南方，向北方与远景流动。' },
        southwest: { angle: 225, label: '西南风 SW → NE', detail: '来自西南，向东北方向流动。' },
        west: { angle: 270, label: '西风 W → E', detail: '来自西方，向东方流动。' },
        northwest: { angle: 315, label: '西北风 NW → SE', detail: '来自西北，向东南方向流动。' }
    };

    var lightSources = {
        point: { uniform: 0, label: '点光源', detail: '从月轮位置向四周柔和散射，角度参数不参与点光计算。' },
        spot: { uniform: 1, label: '聚光源', detail: '从月轮位置沿入射角展开聚光锥，让矿物尘显出成束光路。' },
        directional: { uniform: 2, label: '平行光', detail: '模拟远处恒定方向的月光，形成方向一致的平行光束。' },
        area: { uniform: 3, label: '面光源', detail: '以月轮为中心铺开宽阔柔光面，边缘更缓、覆盖范围更大。' }
    };

    var dunhuangNumericSchema = {
        windStrength: { min: 0, max: 2, step: 0.05, initial: 0.9, format: 'fixed2' },
        dustDensity: { min: 0.1, max: 1.4, step: 0.02, initial: 0.72, format: 'fixed2' },
        ribbonAmplitude: { min: 0, max: 1.4, step: 0.02, initial: 0.72, format: 'fixed2' },
        ribbonTension: { min: 0.08, max: 0.95, step: 0.01, initial: 0.58, format: 'fixed2' },
        ribbonResponse: { min: 0.1, max: 1.3, step: 0.02, initial: 0.8, format: 'fixed2' },
        lightStrength: { min: 0.15, max: 1.3, step: 0.01, initial: 0.78, format: 'fixed2' },
        lightX: { min: 0.12, max: 0.88, step: 0.01, initial: 0.34, format: 'percent' },
        lightY: { min: 0.3, max: 0.94, step: 0.01, initial: 0.72, format: 'percent' },
        lightAngle: { min: -60, max: 60, step: 1, initial: 20, format: 'angle' },
        breathStrength: { min: 0, max: 0.6, step: 0.01, initial: 0.24, format: 'fixed2' },
        breathPeriod: { min: 3, max: 16, step: 0.5, initial: 7, format: 'seconds' },
        tyndallStrength: { min: 0, max: 1.5, step: 0.02, initial: 0.82, format: 'fixed2' },
        beamSpread: { min: 0.08, max: 0.65, step: 0.01, initial: 0.34, format: 'fixed2' }
    };
    var dunhuangBooleanSettings = ['breathingEnabled', 'tyndallEnabled'];

    function dunhuangSettingsPreset() {
        var settings = {};
        Object.keys(dunhuangNumericSchema).forEach(function (key) {
            settings[key] = dunhuangNumericSchema[key].initial;
        });
        dunhuangBooleanSettings.forEach(function (key) { settings[key] = true; });
        return settings;
    }

    function readDunhuangPreferences() {
        try {
            var value = JSON.parse(window.localStorage.getItem(dunhuangStorageKey) || '{}');
            return value && typeof value === 'object' ? value : {};
        } catch (error) {
            return {};
        }
    }

    function clearDunhuangPreferences() {
        try {
            window.localStorage.removeItem(dunhuangStorageKey);
        } catch (error) {
            // 隐私模式无法写入存储时，当前会话仍可正常重置。
        }
    }

    function applyDunhuangSchema(input, key) {
        var definition = dunhuangNumericSchema[key];
        if (!input || !definition) return;
        input.min = String(definition.min);
        input.max = String(definition.max);
        input.step = String(definition.step);
    }

    function formatDunhuangValue(format, value) {
        if (format === 'percent') return Math.round(value * 100) + '%';
        if (format === 'seconds') return value.toFixed(1) + ' s';
        if (format === 'angle') {
            if (value < 0) return '左倾 ' + Math.abs(Math.round(value)) + '°';
            if (value > 0) return '右倾 ' + Math.round(value) + '°';
            return '正下 0°';
        }
        return value.toFixed(2);
    }

    function labNumericBindings() {
        return [
            { input: windStrengthControl, output: windStrengthValue, key: 'windStrength' },
            { input: dustDensityControl, output: dustDensityValue, key: 'dustDensity' },
            { input: ribbonAmplitudeControl, output: ribbonAmplitudeValue, key: 'ribbonAmplitude' },
            { input: ribbonTensionControl, output: ribbonTensionValue, key: 'ribbonTension' },
            { input: ribbonResponseControl, output: ribbonResponseValue, key: 'ribbonResponse' },
            { input: lightStrengthControl, output: lightStrengthValue, key: 'lightStrength' },
            { input: lightXControl, output: lightXValue, key: 'lightX' },
            { input: lightYControl, output: lightYValue, key: 'lightY' },
            { input: lightAngleControl, output: lightAngleValue, key: 'lightAngle' },
            { input: breathStrengthControl, output: breathStrengthValue, key: 'breathStrength' },
            { input: breathPeriodControl, output: breathPeriodValue, key: 'breathPeriod' },
            { input: tyndallStrengthControl, output: tyndallStrengthValue, key: 'tyndallStrength' },
            { input: beamSpreadControl, output: beamSpreadValue, key: 'beamSpread' }
        ];
    }

    function setAmbientReady(ready) {
        if (ambientHero) ambientHero.dataset.dunhuangReady = String(Boolean(ready));
    }

    function normalizeAngle(angle) {
        return ((Number(angle) % 360) + 360) % 360;
    }

    function shortestTurn(fromAngle, toAngle) {
        return normalizeAngle(toAngle - fromAngle + 180) - 180;
    }

    function meteorologicalFlow(angle) {
        var radians = normalizeAngle(angle) * Math.PI / 180;
        return [-Math.sin(radians), -Math.cos(radians)];
    }

    var vertexSource = [
        '#version 300 es',
        'precision highp float;',
        'out vec2 v_uv;',
        'void main() {',
        '    vec2 position_value;',
        '    if (gl_VertexID == 0) position_value = vec2(-1.0, -1.0);',
        '    else if (gl_VertexID == 1) position_value = vec2(3.0, -1.0);',
        '    else position_value = vec2(-1.0, 3.0);',
        '    v_uv = position_value * 0.5 + 0.5;',
        '    gl_Position = vec4(position_value, 0.0, 1.0);',
        '}'
    ].join('\n');

    var fragmentSource = [
        '#version 300 es',
        'precision highp float;',
        'in vec2 v_uv;',
        'out vec4 out_color;',
        'uniform sampler2D u_dunhuang_atlas;',
        'uniform vec2 u_resolution;',
        'uniform vec2 u_pointer;',
        'uniform float u_time;',
        'uniform vec2 u_wind_direction;',
        'uniform float u_wind_strength;',
        'uniform float u_gust;',
        'uniform float u_dust_density;',
        'uniform float u_ribbon_amplitude;',
        'uniform float u_ribbon_tension;',
        'uniform float u_ribbon_response;',
        'uniform float u_light_strength;',
        'uniform vec2 u_light_position;',
        'uniform int u_light_type;',
        'uniform float u_light_angle;',
        'uniform float u_breath_strength;',
        'uniform float u_breath_period;',
        'uniform float u_tyndall_strength;',
        'uniform float u_beam_spread;',
        'uniform vec2 u_figure_position;',
        'uniform float u_figure_scale;',
        '',
        'float hash21(vec2 point_value) {',
        '    point_value = fract(point_value * vec2(123.34, 456.21));',
        '    point_value += dot(point_value, point_value + 45.32);',
        '    return fract(point_value.x * point_value.y);',
        '}',
        '',
        'float value_noise(vec2 point_value) {',
        '    vec2 cell_value = floor(point_value);',
        '    vec2 local_value = fract(point_value);',
        '    local_value = local_value * local_value * (3.0 - 2.0 * local_value);',
        '    float corner_a = hash21(cell_value);',
        '    float corner_b = hash21(cell_value + vec2(1.0, 0.0));',
        '    float corner_c = hash21(cell_value + vec2(0.0, 1.0));',
        '    float corner_d = hash21(cell_value + vec2(1.0, 1.0));',
        '    return mix(mix(corner_a, corner_b, local_value.x), mix(corner_c, corner_d, local_value.x), local_value.y);',
        '}',
        '',
        'float fbm(vec2 point_value) {',
        '    float total_value = 0.0;',
        '    float amplitude_value = 0.5;',
        '    mat2 transform_value = mat2(1.63, 1.17, -1.17, 1.63);',
        '    for (int octave_index = 0; octave_index < 4; octave_index++) {',
        '        total_value += value_noise(point_value) * amplitude_value;',
        '        point_value = transform_value * point_value + 0.19;',
        '        amplitude_value *= 0.48;',
        '    }',
        '    return total_value;',
        '}',
        '',
        'float breathing_light() {',
        '    float cycle_value = fract(u_time / max(u_breath_period, 1.0));',
        '    float inhale_value = smoothstep(0.0, 0.4, cycle_value);',
        '    float exhale_value = 1.0 - smoothstep(0.4, 1.0, cycle_value);',
        '    float envelope_value = smoothstep(0.0, 1.0, min(inhale_value, exhale_value));',
        '    float mineral_flicker = sin(u_time * 1.37 + 0.8) * 0.025 * smoothstep(0.0, 0.02, u_breath_strength);',
        '    return max(0.58, 1.0 + u_breath_strength * mix(-0.42, 1.0, envelope_value) + mineral_flicker);',
        '}',
        '',
        'vec2 dust_layer(vec2 uv_value, float columns_value, float rows_value, float speed_value, float size_value, float seed_value, float depth_value) {',
        '    float wind_energy = u_wind_strength + u_gust * mix(0.7, 1.35, depth_value);',
        '    vec2 dust_point = uv_value * vec2(columns_value, rows_value);',
        '    dust_point -= u_wind_direction * u_time * speed_value * wind_energy * vec2(0.72, 0.38);',
        '    dust_point += vec2(seed_value, seed_value * 1.71);',
        '    vec2 cell_id = floor(dust_point);',
        '    vec2 cell_value = fract(dust_point) - 0.5;',
        '    float random_value = hash21(cell_id + seed_value);',
        '    vec2 particle_offset = vec2(hash21(cell_id + seed_value * 2.3), hash21(cell_id + seed_value * 4.7)) - 0.5;',
        '    particle_offset *= 0.72;',
        '    particle_offset.y += sin(u_time * (0.35 + random_value * 0.4) + random_value * 6.283) * 0.045;',
        '    float particle_distance = length(cell_value - particle_offset);',
        '    float visibility_value = smoothstep(0.48, 0.96, random_value);',
        '    float core_value = 1.0 - smoothstep(size_value * 0.42, size_value, particle_distance);',
        '    float halo_value = 1.0 - smoothstep(size_value, size_value * 3.4, particle_distance);',
        '    return vec2(core_value, halo_value) * visibility_value * u_dust_density;',
        '}',
        '',
        'vec2 light_direction() {',
        '    return normalize(vec2(sin(u_light_angle), -cos(u_light_angle)));',
        '}',
        '',
        'float direct_light_field(vec2 uv_value, float aspect_value) {',
        '    vec2 direction_value = light_direction();',
        '    vec2 perpendicular_value = vec2(-direction_value.y, direction_value.x);',
        '    vec2 relative_value = uv_value - u_light_position;',
        '    relative_value.x *= aspect_value;',
        '    float forward_value = dot(relative_value, direction_value);',
        '    float lateral_value = dot(relative_value, perpendicular_value);',
        '    float radial_distance = length(relative_value);',
        '    if (u_light_type == 0) {',
        '        return exp(-radial_distance * 3.1);',
        '    }',
        '    if (u_light_type == 1) {',
        '        float cone_width = 0.045 + max(forward_value, 0.0) * mix(0.16, 0.7, u_beam_spread);',
        '        float cone_value = 1.0 - smoothstep(cone_width * 0.55, cone_width, abs(lateral_value));',
        '        float travel_value = smoothstep(-0.015, 0.08, forward_value) * (1.0 - smoothstep(0.78, 1.42, forward_value));',
        '        return max(exp(-radial_distance * 6.2) * 0.42, cone_value * travel_value);',
        '    }',
        '    if (u_light_type == 2) {',
        '        float lane_width = mix(0.2, 0.66, u_beam_spread);',
        '        float lane_value = 1.0 - smoothstep(lane_width * 0.58, lane_width, abs(lateral_value));',
        '        return lane_value * mix(0.82, 1.0, smoothstep(-0.35, 0.7, forward_value));',
        '    }',
        '    float sheet_width = mix(0.34, 0.92, u_beam_spread);',
        '    float sheet_value = 1.0 - smoothstep(sheet_width * 0.55, sheet_width, abs(lateral_value));',
        '    float sheet_depth = exp(-abs(forward_value) * 0.82);',
        '    return sheet_value * mix(0.72, 1.0, sheet_depth);',
        '}',
        '',
        'vec2 path_to_light(vec2 uv_value, float aspect_value) {',
        '    if (u_light_type == 2) {',
        '        vec2 direction_value = light_direction();',
        '        return vec2(-direction_value.x / aspect_value, -direction_value.y) * 1.25;',
        '    }',
        '    return u_light_position - uv_value;',
        '}',
        '',
        'float beam_cluster(vec2 uv_value, float aspect_value) {',
        '    vec2 direction_value = light_direction();',
        '    vec2 perpendicular_value = vec2(-direction_value.y, direction_value.x);',
        '    vec2 relative_value = uv_value - u_light_position;',
        '    relative_value.x *= aspect_value;',
        '    float forward_value = dot(relative_value, direction_value);',
        '    float lateral_value = dot(relative_value, perpendicular_value);',
        '    if (u_light_type == 0) {',
        '        float radial_distance = length(relative_value);',
        '        float mineral_halo = exp(-radial_distance * 3.8);',
        '        float radial_texture = 0.78 + value_noise(vec2(atan(relative_value.y, relative_value.x) * 3.2, radial_distance * 13.0 - u_time * 0.08)) * 0.22;',
        '        return mineral_halo * radial_texture;',
        '    }',
        '    float travel_value = smoothstep(0.015, 0.11, forward_value) * (1.0 - smoothstep(0.78, 1.36, forward_value));',
        '    if (u_light_type == 3) {',
        '        float sheet_width = mix(0.32, 0.86, u_beam_spread);',
        '        float sheet_value = 1.0 - smoothstep(sheet_width * 0.62, sheet_width, abs(lateral_value));',
        '        float curtain_texture = mix(0.72, 1.0, value_noise(vec2(lateral_value * 11.0, forward_value * 3.2 - u_time * 0.035)));',
        '        return sheet_value * curtain_texture * mix(0.68, 1.0, travel_value);',
        '    }',
        '    float cluster_value = 0.0;',
        '    for (int beam_index = 0; beam_index < 5; beam_index++) {',
        '        float seed_value = hash21(vec2(float(beam_index) + 3.2, 9.7));',
        '        float index_value = float(beam_index) / 4.0 * 2.0 - 1.0;',
        '        float fan_value = u_light_type == 1 ? (0.1 + max(forward_value, 0.0) * 0.72) : 0.48;',
        '        float beam_center = (index_value + (seed_value - 0.5) * 0.2) * u_beam_spread * fan_value;',
        '        float width_value = mix(0.012, 0.027, seed_value) * (0.8 + max(forward_value, 0.0) * 0.48);',
        '        float beam_value = 1.0 - smoothstep(width_value * 0.38, width_value * 2.4, abs(lateral_value - beam_center));',
        '        cluster_value = max(cluster_value, beam_value * mix(0.7, 1.0, seed_value));',
        '    }',
        '    float source_field = direct_light_field(uv_value, aspect_value);',
        '    return mix(0.1, 1.0, cluster_value) * max(travel_value, source_field * 0.78);',
        '}',
        '',
        'float tyndall_scattering(vec2 uv_value, float aspect_value) {',
        '    vec2 light_path = path_to_light(uv_value, aspect_value);',
        '    float path_length = max(length(vec2(light_path.x * aspect_value, light_path.y)), 0.05);',
        '    vec2 dust_drift = u_wind_direction * u_time * 0.012 * (u_wind_strength + u_gust);',
        '    float volume_value = 0.0;',
        '    for (int volume_index = 0; volume_index < 6; volume_index++) {',
        '        float path_ratio = (float(volume_index) + 0.5) / 6.0;',
        '        vec2 sample_point = uv_value + light_path * path_ratio;',
        '        float coarse_value = value_noise(sample_point * vec2(5.1, 7.6) + dust_drift + vec2(1.7));',
        '        float fine_value = value_noise(sample_point * vec2(12.3, 9.2) - dust_drift * 1.4 + vec2(5.2));',
        '        float density_value = mix(0.28, 1.18, smoothstep(0.16, 0.9, mix(coarse_value, fine_value, 0.3)));',
        '        float transmittance = exp(-path_ratio * path_length * u_dust_density * 0.78);',
        '        volume_value += density_value * transmittance;',
        '    }',
        '    float source_field = direct_light_field(uv_value, aspect_value);',
        '    return volume_value / 6.0 * beam_cluster(uv_value, aspect_value) * mix(0.45, 1.0, source_field) * u_dust_density;',
        '}',
        '',
        'vec4 dunhuang_sample(vec2 uv_value, float aspect_value) {',
        '    vec2 local_value = uv_value - u_figure_position;',
        '    local_value.x *= aspect_value;',
        '    vec2 texture_uv = local_value / (vec2(0.82, 0.78) * u_figure_scale) + 0.5;',
        '    float ribbon_region = smoothstep(0.52, 0.7, texture_uv.x);',
        '    ribbon_region *= smoothstep(0.06, 0.2, texture_uv.y) * (1.0 - smoothstep(0.9, 0.99, texture_uv.y));',
        '    float frequency_value = mix(7.0, 18.0, u_ribbon_tension);',
        '    float compliance_value = mix(1.28, 0.42, u_ribbon_tension);',
        '    float wind_energy = (u_wind_strength + u_gust * 0.8) * u_ribbon_response;',
        '    float wave_value = sin(texture_uv.x * frequency_value - u_time * (0.72 + wind_energy * 0.64) + texture_uv.y * 2.7);',
        '    wave_value += sin(texture_uv.x * frequency_value * 0.47 - u_time * 1.13 + 1.8) * 0.36;',
        '    float pointer_value = exp(-distance(uv_value, u_pointer) * 5.2);',
        '    vec2 perpendicular_value = vec2(-u_wind_direction.y, u_wind_direction.x);',
        '    vec2 ribbon_shift = perpendicular_value * wave_value * 0.016 * u_ribbon_amplitude * compliance_value;',
        '    ribbon_shift += u_wind_direction * (0.006 + pointer_value * 0.008) * wind_energy * u_ribbon_amplitude;',
        '    vec2 flowing_uv = texture_uv - ribbon_shift * ribbon_region;',
        '    float inside_value = step(0.0, texture_uv.x) * step(texture_uv.x, 1.0) * step(0.0, texture_uv.y) * step(texture_uv.y, 1.0);',
        '    vec2 atlas_uv = vec2(0.6 + texture_uv.x * 0.2, 0.5 + texture_uv.y * 0.5);',
        '    vec4 anchored_value = texture(u_dunhuang_atlas, atlas_uv) * inside_value;',
        '    float flowing_inside = step(0.0, flowing_uv.x) * step(flowing_uv.x, 1.0) * step(0.0, flowing_uv.y) * step(flowing_uv.y, 1.0);',
        '    vec2 flowing_atlas_uv = vec2(0.6 + flowing_uv.x * 0.2, 0.5 + flowing_uv.y * 0.5);',
        '    vec4 flowing_value = texture(u_dunhuang_atlas, flowing_atlas_uv) * flowing_inside;',
        '    float painted_ribbon = smoothstep(0.02, 0.24, max(anchored_value.a, flowing_value.a));',
        '    return mix(anchored_value, flowing_value, ribbon_region * painted_ribbon);',
        '}',
        '',
        'vec3 fresco_background(vec2 uv_value, float aspect_value, float light_energy) {',
        '    vec3 night_color = vec3(0.169, 0.145, 0.2);',
        '    vec3 violet_color = vec3(0.4, 0.314, 0.42);',
        '    vec3 clay_color = vec3(0.71, 0.416, 0.298);',
        '    vec3 gold_color = vec3(0.851, 0.643, 0.255);',
        '    float vertical_value = smoothstep(0.0, 1.0, uv_value.y);',
        '    vec3 color_value = mix(night_color, violet_color, vertical_value * 0.72);',
        '    float horizon_value = exp(-pow((uv_value.y - 0.34) * 4.8, 2.0));',
        '    color_value = mix(color_value, clay_color, horizon_value * 0.2);',
        '    float light_pool = direct_light_field(uv_value, aspect_value);',
        '    color_value += gold_color * light_pool * light_energy * 0.22;',
        '    float plaster_value = fbm(uv_value * vec2(3.4, 2.1) + vec2(1.3, 4.1));',
        '    float mineral_value = value_noise(uv_value * vec2(24.0, 18.0) + vec2(8.7));',
        '    color_value *= 0.9 + plaster_value * 0.14;',
        '    color_value += (mineral_value - 0.5) * 0.022;',
        '    return color_value;',
        '}',
        '',
        'void main() {',
        '    vec2 uv_value = v_uv;',
        '    float aspect_value = u_resolution.x / max(u_resolution.y, 1.0);',
        '    float breath_value = breathing_light();',
        '    float light_energy = u_light_strength * breath_value;',
        '    vec3 color_value = fresco_background(uv_value, aspect_value, light_energy);',
        '',
        '    vec2 far_dust = dust_layer(uv_value, 31.0, 18.0, 0.028, 0.105, 3.7, 0.1);',
        '    vec2 middle_dust = dust_layer(uv_value, 20.0, 12.0, 0.052, 0.13, 11.4, 0.55);',
        '    color_value += vec3(0.4, 0.314, 0.42) * far_dust.y * 0.11;',
        '    color_value += vec3(0.71, 0.416, 0.298) * (middle_dust.x * 0.1 + middle_dust.y * 0.08);',
        '',
        '    float tyndall_value = 0.0;',
        '    if (u_tyndall_strength > 0.001) {',
        '        tyndall_value = tyndall_scattering(uv_value, aspect_value);',
        '        color_value += vec3(0.906, 0.824, 0.631) * tyndall_value * u_tyndall_strength * light_energy * 0.46;',
        '    }',
        '',
        '    vec4 symbol_value = dunhuang_sample(uv_value, aspect_value);',
        '    float symbol_light = direct_light_field(uv_value, aspect_value) * light_energy;',
        '    vec3 lit_symbol = symbol_value.rgb * mix(0.78, 1.16, clamp(symbol_light, 0.0, 1.0));',
        '    color_value = mix(color_value, lit_symbol, symbol_value.a);',
        '',
        '    vec2 near_dust = dust_layer(uv_value, 12.0, 8.0, 0.088, 0.17, 23.8, 1.0);',
        '    vec3 near_color = mix(vec3(0.71, 0.416, 0.298), vec3(0.851, 0.643, 0.255), breath_value * 0.48);',
        '    color_value += near_color * (near_dust.x * 0.32 + near_dust.y * 0.13) * (0.72 + tyndall_value * 0.7);',
        '',
        '    float vignette_value = 1.0 - smoothstep(0.28, 1.12, distance(uv_value, vec2(0.5)));',
        '    color_value *= mix(0.78, 1.04, vignette_value);',
        '    float grain_value = hash21(gl_FragCoord.xy + floor(u_time * 10.0)) - 0.5;',
        '    color_value += grain_value * 0.009;',
        '    out_color = vec4(clamp(color_value, 0.0, 1.0), 1.0);',
        '}'
    ].join('\n');

    function createShader(gl, type, source, label) {
        var shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            var message = gl.getShaderInfoLog(shader) || '未知着色器错误';
            gl.deleteShader(shader);
            throw new Error(label + '着色器编译失败: ' + message);
        }
        return shader;
    }

    function createProgram(gl) {
        var vertexShader = createShader(gl, gl.VERTEX_SHADER, vertexSource, '敦煌暮色顶点');
        var fragmentShader = createShader(gl, gl.FRAGMENT_SHADER, fragmentSource, '敦煌暮色片元');
        var program = gl.createProgram();
        gl.attachShader(program, vertexShader);
        gl.attachShader(program, fragmentShader);
        gl.linkProgram(program);
        gl.deleteShader(vertexShader);
        gl.deleteShader(fragmentShader);
        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            var message = gl.getProgramInfoLog(program) || '未知链接错误';
            gl.deleteProgram(program);
            throw new Error('敦煌暮色渲染程序链接失败: ' + message);
        }
        return program;
    }

    function uniformMap(gl, program, names) {
        return names.reduce(function (uniforms, name) {
            uniforms[name] = gl.getUniformLocation(program, name);
            return uniforms;
        }, {});
    }

    function DunhuangMotionLab() {
        this.ambient = ambientMode;
        this.gl = canvas.getContext('webgl2', {
            alpha: false,
            antialias: false,
            depth: false,
            stencil: false,
            premultipliedAlpha: false,
            powerPreference: 'high-performance'
        });
        if (!this.gl) throw new Error('当前环境不支持 WebGL2，敦煌暮色样板已停用。');

        this.settings = dunhuangSettingsPreset();
        this.windDirectionKey = 'west';
        this.lightSourceKey = 'spot';
        this.windAngle = 270;
        this.targetWindAngle = 270;
        this.ambientPaused = false;
        this.restoreDunhuangPreferences();
        this.figurePosition = this.ambient ? { x: 0.71, y: 0.51 } : { x: 0.51, y: 0.5 };
        this.figureScale = this.ambient ? 1.08 : 1;
        this.sceneAspectRatio = 0;
        this.sceneHeight = 0;
        this.pointer = { x: 0.5, y: 0.5, targetX: 0.5, targetY: 0.5 };
        this.elapsed = 0;
        this.gust = 0;
        this.lastTimestamp = 0;
        this.frameHandle = 0;
        this.destroyed = false;
        this.assetReady = false;
        this.reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        this.themeActive = !this.ambient || document.documentElement.dataset.theme === 'atlas-dunhuang';
        this.paused = this.reducedMotion || !this.themeActive || this.ambientPaused;

        var gl = this.gl;
        this.vertexArray = gl.createVertexArray();
        gl.bindVertexArray(this.vertexArray);
        this.program = createProgram(gl);
        this.uniforms = uniformMap(gl, this.program, [
            'u_dunhuang_atlas', 'u_resolution', 'u_pointer', 'u_time',
            'u_wind_direction', 'u_wind_strength', 'u_gust', 'u_dust_density',
            'u_ribbon_amplitude', 'u_ribbon_tension', 'u_ribbon_response',
            'u_light_strength', 'u_light_position', 'u_light_type', 'u_light_angle',
            'u_breath_strength', 'u_breath_period',
            'u_tyndall_strength', 'u_beam_spread', 'u_figure_position', 'u_figure_scale'
        ]);
        this.assetTexture = this.createAssetTexture();
        this.bindControls();
        this.resizeObserver = new ResizeObserver(this.resize.bind(this));
        this.resizeObserver.observe(stage);
        this.handleVisibility = this.handleVisibility.bind(this);
        document.addEventListener('visibilitychange', this.handleVisibility);
        canvas.addEventListener('webglcontextlost', this.handleContextLost.bind(this), { once: true });
    }

    DunhuangMotionLab.prototype.createAssetTexture = function () {
        var gl = this.gl;
        var texture = gl.createTexture();
        gl.bindTexture(gl.TEXTURE_2D, texture);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([43, 37, 51, 0]));
        return texture;
    };

    DunhuangMotionLab.prototype.loadAsset = function () {
        var self = this;
        return new Promise(function (resolve, reject) {
            var image = new Image();
            image.decoding = 'async';
            image.onload = function () {
                try {
                    var gl = self.gl;
                    gl.bindTexture(gl.TEXTURE_2D, self.assetTexture);
                    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
                    gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false);
                    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image);
                    gl.generateMipmap(gl.TEXTURE_2D);
                    resolve();
                } catch (error) {
                    reject(error);
                }
            };
            image.onerror = function () { reject(new Error('敦煌飞天符号资源加载失败。')); };
            image.src = stage.dataset.dunhuangAsset;
        });
    };

    DunhuangMotionLab.prototype.bindControls = function () {
        if (this.ambient) {
            this.bindAmbientMode();
            return;
        }
        var self = this;
        labNumericBindings().forEach(function (control) {
            applyDunhuangSchema(control.input, control.key);
            control.input.addEventListener('input', function () {
                var value = Number(control.input.value);
                self.settings[control.key] = value;
                control.output.value = formatDunhuangValue(dunhuangNumericSchema[control.key].format, value);
                self.saveDunhuangPreferences();
                if (self.paused) self.render();
            });
        });

        windDirectionButtons.forEach(function (button) {
            button.addEventListener('click', function () {
                self.setWindDirection(button.dataset.windDirection);
                self.saveDunhuangPreferences();
            });
        });
        lightSourceButtons.forEach(function (button) {
            button.addEventListener('click', function () {
                self.setLightSource(button.dataset.lightSource);
            });
        });
        [
            { button: breathingToggle, key: 'breathingEnabled' },
            { button: tyndallToggle, key: 'tyndallEnabled' }
        ].forEach(function (effect) {
            effect.button.addEventListener('click', function () {
                self.settings[effect.key] = !self.settings[effect.key];
                self.syncLightControls();
                self.saveDunhuangPreferences();
                if (self.paused) self.render();
            });
        });

        gustButton.addEventListener('click', function () { self.addGust(); });
        pauseButton.addEventListener('click', function () {
            self.paused = !self.paused;
            self.syncPauseButton();
            if (!self.paused) self.requestFrame(); else self.render();
        });
        resetButton.addEventListener('click', function () { self.reset(); });
        canvas.addEventListener('pointermove', function (event) {
            var point = self.eventPoint(event);
            self.pointer.targetX = point.x;
            self.pointer.targetY = point.y;
        });
        canvas.addEventListener('pointerleave', function () {
            self.pointer.targetX = 0.5;
            self.pointer.targetY = 0.5;
        });

        this.syncLabControls();
        this.setWindDirection(this.windDirectionKey, true);
        this.syncLightControls();
        this.syncPauseButton();
    };

    DunhuangMotionLab.prototype.syncLabControls = function () {
        var self = this;
        labNumericBindings().forEach(function (control) {
            applyDunhuangSchema(control.input, control.key);
            control.input.value = String(self.settings[control.key]);
            control.output.value = formatDunhuangValue(dunhuangNumericSchema[control.key].format, Number(self.settings[control.key]));
        });
    };

    DunhuangMotionLab.prototype.bindAmbientMode = function () {
        var self = this;
        stage.dataset.themeActive = String(this.themeActive);
        this.bindAmbientControls();
        document.addEventListener('pointermove', function (event) {
            if (!self.themeActive || (ambientControls && ambientControls.contains(event.target))) return;
            var point = self.eventPoint(event);
            if (point.inside) {
                self.pointer.targetX = point.x;
                self.pointer.targetY = point.y;
            } else {
                self.pointer.targetX = 0.5;
                self.pointer.targetY = 0.5;
            }
        }, { passive: true });
        window.addEventListener('guccang:themechange', function (event) {
            self.setAmbientTheme(event.detail && event.detail.theme);
        });
    };

    DunhuangMotionLab.prototype.restoreDunhuangPreferences = function () {
        var preferences = readDunhuangPreferences();
        var self = this;
        Object.keys(dunhuangNumericSchema).forEach(function (key) {
            if (typeof preferences[key] !== 'number' || !Number.isFinite(preferences[key])) return;
            var definition = dunhuangNumericSchema[key];
            self.settings[key] = Math.max(definition.min, Math.min(definition.max, preferences[key]));
        });
        dunhuangBooleanSettings.forEach(function (key) {
            if (typeof preferences[key] === 'boolean') self.settings[key] = preferences[key];
        });
        if (windDirections[preferences.windDirection]) {
            this.windDirectionKey = preferences.windDirection;
            this.windAngle = windDirections[preferences.windDirection].angle;
            this.targetWindAngle = this.windAngle;
        }
        if (lightSources[preferences.lightSource]) this.lightSourceKey = preferences.lightSource;
    };

    DunhuangMotionLab.prototype.saveDunhuangPreferences = function () {
        var preferences = { windDirection: this.windDirectionKey, lightSource: this.lightSourceKey };
        var self = this;
        Object.keys(dunhuangNumericSchema).forEach(function (key) { preferences[key] = self.settings[key]; });
        dunhuangBooleanSettings.forEach(function (key) { preferences[key] = self.settings[key]; });
        try {
            window.localStorage.setItem(dunhuangStorageKey, JSON.stringify(preferences));
        } catch (error) {
            // 隐私模式无法持久化时，当前会话内的参数仍保持有效。
        }
    };

    DunhuangMotionLab.prototype.syncAmbientControls = function () {
        if (!ambientControls) return;
        var self = this;
        ambientControls.querySelectorAll('[data-dunhuang-setting]').forEach(function (input) {
            var key = input.dataset.dunhuangSetting;
            applyDunhuangSchema(input, key);
            input.value = String(self.settings[key]);
            var output = ambientControls.querySelector('[data-dunhuang-output="' + key + '"]');
            if (output) output.value = formatDunhuangValue(input.dataset.dunhuangFormat || dunhuangNumericSchema[key].format, Number(self.settings[key]));
        });
        ambientControls.querySelectorAll('[data-dunhuang-toggle]').forEach(function (input) {
            input.checked = Boolean(self.settings[input.dataset.dunhuangToggle]);
        });
        var directionControl = ambientControls.querySelector('[data-dunhuang-wind-direction]');
        if (directionControl) directionControl.value = this.windDirectionKey;
        var sourceOutput = ambientControls.querySelector('[data-dunhuang-light-source-output]');
        if (sourceOutput) sourceOutput.value = lightSources[this.lightSourceKey].label;
        ambientControls.querySelectorAll('[data-dunhuang-light-source]').forEach(function (button) {
            button.setAttribute('aria-pressed', String(button.dataset.dunhuangLightSource === self.lightSourceKey));
        });
        var ambientAngleControl = ambientControls.querySelector('[data-dunhuang-setting="lightAngle"]');
        if (ambientAngleControl) ambientAngleControl.disabled = this.lightSourceKey === 'point';
        var pauseControl = ambientControls.querySelector('[data-dunhuang-action="pause"]');
        if (pauseControl) {
            pauseControl.disabled = this.reducedMotion;
            pauseControl.setAttribute('aria-pressed', String(this.ambientPaused));
            pauseControl.textContent = this.reducedMotion ? '系统已暂停' : (this.ambientPaused ? '继续' : '暂停');
        }
        ambientControls.dataset.breathing = this.settings.breathingEnabled ? 'on' : 'off';
        ambientControls.dataset.tyndall = this.settings.tyndallEnabled ? 'on' : 'off';
        ambientControls.dataset.source = this.lightSourceKey;
        ambientControls.dataset.paused = String(this.paused);
    };

    DunhuangMotionLab.prototype.bindAmbientControls = function () {
        if (!ambientControls) return;
        var self = this;
        ambientControls.addEventListener('toggle', function () { self.syncAmbientInspectorLayout(); });
        ambientControls.querySelectorAll('[data-dunhuang-setting]').forEach(function (input) {
            input.addEventListener('input', function () {
                self.settings[input.dataset.dunhuangSetting] = Number(input.value);
                self.syncAmbientControls();
                self.saveDunhuangPreferences();
                if (self.paused && self.assetReady) self.render();
            });
        });
        ambientControls.querySelectorAll('[data-dunhuang-toggle]').forEach(function (input) {
            input.addEventListener('change', function () {
                self.settings[input.dataset.dunhuangToggle] = input.checked;
                self.syncAmbientControls();
                self.saveDunhuangPreferences();
                if (self.paused && self.assetReady) self.render();
            });
        });
        var directionControl = ambientControls.querySelector('[data-dunhuang-wind-direction]');
        if (directionControl) {
            directionControl.addEventListener('change', function () {
                self.setAmbientWindDirection(directionControl.value, false);
            });
        }
        ambientControls.querySelectorAll('[data-dunhuang-light-source]').forEach(function (button) {
            button.addEventListener('click', function () {
                self.setLightSource(button.dataset.dunhuangLightSource);
            });
        });
        var gustControl = ambientControls.querySelector('[data-dunhuang-action="gust"]');
        if (gustControl) gustControl.addEventListener('click', function () { self.addGust(); });
        var pauseControl = ambientControls.querySelector('[data-dunhuang-action="pause"]');
        if (pauseControl) pauseControl.addEventListener('click', function () { self.setAmbientPaused(!self.ambientPaused); });
        var resetControl = ambientControls.querySelector('[data-dunhuang-action="reset"]');
        if (resetControl) resetControl.addEventListener('click', function () { self.resetAmbientSettings(); });
        this.syncAmbientInspectorLayout();
        this.syncAmbientControls();
    };

    DunhuangMotionLab.prototype.syncAmbientInspectorLayout = function () {
        if (!ambientHero || !ambientControls) return;
        var inspectorOpen = ambientControls.open;
        if (inspectorOpen) this.captureAmbientSceneGeometry(true);
        ambientHero.dataset.dunhuangInspector = inspectorOpen ? 'open' : 'closed';
        var self = this;
        window.requestAnimationFrame(function () {
            if (!self.themeActive) return;
            self.syncAmbientSceneGeometry();
            self.resize();
            if (self.assetReady) self.render();
        });
    };

    DunhuangMotionLab.prototype.captureAmbientSceneGeometry = function (force) {
        if (!this.ambient || !ambientHero || !ambientControls || (ambientControls.open && !force) || !this.themeActive) return;
        var bounds = stage.getBoundingClientRect();
        if (bounds.width <= 1 || bounds.height <= 1) return;
        this.sceneAspectRatio = bounds.width / bounds.height;
        this.sceneHeight = bounds.height;
        ambientHero.style.setProperty('--dunhuang-scene-height', bounds.height.toFixed(2) + 'px');
    };

    DunhuangMotionLab.prototype.syncAmbientSceneGeometry = function () {
        if (!this.ambient || !ambientHero || !ambientControls) return;
        if (!ambientControls.open) {
            this.captureAmbientSceneGeometry();
            return;
        }
        if (this.sceneAspectRatio <= 0) return;
        var stageWidth = stage.getBoundingClientRect().width;
        if (stageWidth <= 1) return;
        this.sceneHeight = stageWidth / this.sceneAspectRatio;
        ambientHero.style.setProperty('--dunhuang-scene-height', this.sceneHeight.toFixed(2) + 'px');
    };

    DunhuangMotionLab.prototype.setAmbientWindDirection = function (directionKey, immediate) {
        var direction = windDirections[directionKey];
        if (!direction) return;
        this.windDirectionKey = directionKey;
        this.targetWindAngle = direction.angle;
        if (immediate || this.paused) this.windAngle = direction.angle;
        this.syncAmbientControls();
        this.saveDunhuangPreferences();
        if (this.paused && this.assetReady) this.render();
    };

    DunhuangMotionLab.prototype.setAmbientPaused = function (paused) {
        if (!this.ambient || this.reducedMotion) return;
        this.ambientPaused = Boolean(paused);
        this.paused = this.ambientPaused || !this.themeActive;
        if (this.paused) {
            if (this.frameHandle) window.cancelAnimationFrame(this.frameHandle);
            this.frameHandle = 0;
            if (this.assetReady) this.render();
        } else {
            this.lastTimestamp = 0;
            this.requestFrame();
        }
        this.syncAmbientControls();
    };

    DunhuangMotionLab.prototype.setAmbientTheme = function (theme) {
        if (!this.ambient) return;
        this.themeActive = theme === 'atlas-dunhuang';
        stage.dataset.themeActive = String(this.themeActive);
        if (!this.themeActive || this.reducedMotion || this.ambientPaused) {
            this.paused = true;
            if (this.frameHandle) window.cancelAnimationFrame(this.frameHandle);
            this.frameHandle = 0;
            if (this.themeActive && this.assetReady) {
                this.resize();
                this.render();
            }
            this.syncAmbientControls();
            return;
        }
        this.paused = false;
        this.lastTimestamp = 0;
        this.resize();
        this.requestFrame();
        this.syncAmbientControls();
    };

    DunhuangMotionLab.prototype.setWindDirection = function (directionKey, immediate) {
        var direction = windDirections[directionKey];
        if (!direction) return;
        this.windDirectionKey = directionKey;
        this.targetWindAngle = direction.angle;
        if (immediate || this.paused) this.windAngle = direction.angle;
        windDirectionValue.value = direction.label + ' · ' + direction.angle + '°';
        windDirectionDetail.textContent = '气象角度 ' + direction.angle + '° · ' + direction.detail;
        windCompass.dataset.direction = directionKey;
        windDirectionButtons.forEach(function (button) {
            button.setAttribute('aria-pressed', String(button.dataset.windDirection === directionKey));
        });
        this.syncWindNeedle();
        if (this.paused) this.render();
    };

    DunhuangMotionLab.prototype.updateWind = function (dt) {
        var turn = shortestTurn(this.windAngle, this.targetWindAngle);
        this.windAngle = normalizeAngle(this.windAngle + turn * Math.min(1, dt * 3.8));
        this.syncWindNeedle();
    };

    DunhuangMotionLab.prototype.syncWindNeedle = function () {
        if (!windCompass) return;
        var flow = meteorologicalFlow(this.windAngle);
        var flowAngle = Math.atan2(flow[1], flow[0]) * 180 / Math.PI;
        windCompass.style.setProperty('--wind-flow-angle', flowAngle + 'deg');
    };

    DunhuangMotionLab.prototype.syncLightControls = function () {
        var source = lightSources[this.lightSourceKey];
        lightSourceButtons.forEach(function (button) {
            button.setAttribute('aria-pressed', String(button.dataset.lightSource === this.lightSourceKey));
        }, this);
        if (lightSourceValue) lightSourceValue.value = source.label;
        if (lightSourceDetail) lightSourceDetail.textContent = source.detail;
        if (lightAngleControl) lightAngleControl.disabled = this.lightSourceKey === 'point';
        breathingToggle.setAttribute('aria-checked', String(this.settings.breathingEnabled));
        breathingToggle.textContent = this.settings.breathingEnabled ? '已开启' : '已关闭';
        tyndallToggle.setAttribute('aria-checked', String(this.settings.tyndallEnabled));
        tyndallToggle.textContent = this.settings.tyndallEnabled ? '已开启' : '已关闭';
        lightInstrument.dataset.breathing = this.settings.breathingEnabled ? 'on' : 'off';
        lightInstrument.dataset.tyndall = this.settings.tyndallEnabled ? 'on' : 'off';
        lightInstrument.dataset.source = this.lightSourceKey;
    };

    DunhuangMotionLab.prototype.setLightSource = function (sourceKey) {
        if (!lightSources[sourceKey]) return;
        this.lightSourceKey = sourceKey;
        if (this.ambient) this.syncAmbientControls(); else this.syncLightControls();
        this.saveDunhuangPreferences();
        if (this.paused && this.assetReady) this.render();
    };

    DunhuangMotionLab.prototype.syncPauseButton = function () {
        pauseButton.setAttribute('aria-pressed', String(this.paused));
        pauseButton.textContent = this.paused ? '继续' : '暂停';
        stage.dataset.paused = String(this.paused);
    };

    DunhuangMotionLab.prototype.eventPoint = function (event) {
        var bounds = canvas.getBoundingClientRect();
        return {
            x: Math.max(0, Math.min(1, (event.clientX - bounds.left) / Math.max(bounds.width, 1))),
            y: Math.max(0, Math.min(1, 1 - (event.clientY - bounds.top) / Math.max(bounds.height, 1))),
            inside: event.clientX >= bounds.left && event.clientX <= bounds.right && event.clientY >= bounds.top && event.clientY <= bounds.bottom
        };
    };

    DunhuangMotionLab.prototype.resize = function () {
        if (this.ambient) this.syncAmbientSceneGeometry();
        var bounds = stage.getBoundingClientRect();
        var dpr = Math.min(window.devicePixelRatio || 1, window.matchMedia('(max-width: 720px)').matches ? 1.15 : 1.55);
        var pixelBudget = window.matchMedia('(max-width: 720px)').matches ? 850000 : 2100000;
        var desiredPixels = bounds.width * bounds.height * dpr * dpr;
        if (desiredPixels > pixelBudget) dpr *= Math.sqrt(pixelBudget / desiredPixels);
        var width = Math.max(1, Math.round(bounds.width * dpr));
        var height = Math.max(1, Math.round(bounds.height * dpr));
        if (canvas.width !== width || canvas.height !== height) {
            canvas.width = width;
            canvas.height = height;
            if (this.paused) this.render();
        }
    };

    DunhuangMotionLab.prototype.update = function (dt) {
        this.elapsed += dt;
        this.updateWind(dt);
        this.pointer.x += (this.pointer.targetX - this.pointer.x) * Math.min(1, dt * 2.8);
        this.pointer.y += (this.pointer.targetY - this.pointer.y) * Math.min(1, dt * 2.8);
        this.gust = Math.max(0, this.gust - dt * 0.58);
    };

    DunhuangMotionLab.prototype.addGust = function () {
        this.gust = Math.min(1.4, this.gust + 1);
        if (this.paused && this.assetReady) this.render();
    };

    DunhuangMotionLab.prototype.render = function () {
        if (!canvas.width || !canvas.height) return;
        var gl = this.gl;
        var uniforms = this.uniforms;
        var flow = meteorologicalFlow(this.windAngle);
        gl.bindVertexArray(this.vertexArray);
        gl.viewport(0, 0, canvas.width, canvas.height);
        gl.useProgram(this.program);
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, this.assetTexture);
        gl.uniform1i(uniforms.u_dunhuang_atlas, 0);
        gl.uniform2f(uniforms.u_resolution, canvas.width, canvas.height);
        gl.uniform2f(uniforms.u_pointer, this.pointer.x, this.pointer.y);
        gl.uniform1f(uniforms.u_time, this.elapsed);
        gl.uniform2f(uniforms.u_wind_direction, flow[0], flow[1]);
        gl.uniform1f(uniforms.u_wind_strength, this.settings.windStrength);
        gl.uniform1f(uniforms.u_gust, this.gust);
        gl.uniform1f(uniforms.u_dust_density, this.settings.dustDensity);
        gl.uniform1f(uniforms.u_ribbon_amplitude, this.settings.ribbonAmplitude);
        gl.uniform1f(uniforms.u_ribbon_tension, this.settings.ribbonTension);
        gl.uniform1f(uniforms.u_ribbon_response, this.settings.ribbonResponse);
        gl.uniform1f(uniforms.u_light_strength, this.settings.lightStrength);
        gl.uniform2f(uniforms.u_light_position, this.settings.lightX, this.settings.lightY);
        gl.uniform1i(uniforms.u_light_type, lightSources[this.lightSourceKey].uniform);
        gl.uniform1f(uniforms.u_light_angle, this.settings.lightAngle * Math.PI / 180);
        gl.uniform1f(uniforms.u_breath_strength, this.settings.breathingEnabled ? this.settings.breathStrength : 0);
        gl.uniform1f(uniforms.u_breath_period, this.settings.breathPeriod);
        gl.uniform1f(uniforms.u_tyndall_strength, this.settings.tyndallEnabled ? this.settings.tyndallStrength : 0);
        gl.uniform1f(uniforms.u_beam_spread, this.settings.beamSpread);
        gl.uniform2f(uniforms.u_figure_position, this.figurePosition.x, this.figurePosition.y);
        gl.uniform1f(uniforms.u_figure_scale, this.figureScale);
        gl.drawArrays(gl.TRIANGLES, 0, 3);
    };

    DunhuangMotionLab.prototype.frame = function (timestamp) {
        this.frameHandle = 0;
        if (this.destroyed || this.paused || document.hidden) return;
        var dt = this.lastTimestamp ? Math.min((timestamp - this.lastTimestamp) / 1000, 0.05) : 1 / 60;
        this.lastTimestamp = timestamp;
        this.update(dt);
        this.render();
        this.requestFrame();
    };

    DunhuangMotionLab.prototype.requestFrame = function () {
        if (this.frameHandle || this.destroyed || this.paused || !this.assetReady || document.hidden) return;
        this.frameHandle = window.requestAnimationFrame(this.frame.bind(this));
    };

    DunhuangMotionLab.prototype.reset = function () {
        this.settings = dunhuangSettingsPreset();
        this.lightSourceKey = 'spot';
        clearDunhuangPreferences();
        this.setWindDirection('west', true);
        this.syncLabControls();
        this.syncLightControls();
        this.pointer = { x: 0.5, y: 0.5, targetX: 0.5, targetY: 0.5 };
        this.elapsed = 0;
        this.gust = 0;
        this.render();
    };

    DunhuangMotionLab.prototype.resetAmbientSettings = function () {
        if (!this.ambient) return;
        this.settings = dunhuangSettingsPreset();
        this.windDirectionKey = 'west';
        this.lightSourceKey = 'spot';
        this.windAngle = 270;
        this.targetWindAngle = 270;
        this.ambientPaused = false;
        this.pointer = { x: 0.5, y: 0.5, targetX: 0.5, targetY: 0.5 };
        this.elapsed = 0;
        this.gust = 0;
        clearDunhuangPreferences();
        this.paused = this.reducedMotion || !this.themeActive;
        this.syncAmbientControls();
        if (this.assetReady) this.render();
        if (!this.paused) this.requestFrame();
    };

    DunhuangMotionLab.prototype.handleVisibility = function () {
        this.lastTimestamp = 0;
        if (!document.hidden && !this.paused) this.requestFrame();
    };

    DunhuangMotionLab.prototype.handleContextLost = function (event) {
        event.preventDefault();
        this.destroyed = true;
        showError('WebGL2 上下文已丢失，敦煌暮色样板已停用。', '请刷新页面重新创建图形上下文。');
    };

    DunhuangMotionLab.prototype.start = function () {
        var self = this;
        this.resize();
        return this.loadAsset().then(function () {
            if (self.destroyed) return;
            self.assetReady = true;
            self.render();
            stage.dataset.renderState = 'ready';
            setAmbientReady(true);
            self.syncAmbientControls();
            if (!self.paused) self.requestFrame();
        });
    };

    function showError(title, detail) {
        stage.dataset.renderState = 'error';
        setAmbientReady(false);
        if (statusTitle) statusTitle.textContent = title;
        if (statusDetail) statusDetail.textContent = detail || '当前样板不会回退到其他渲染方式。';
    }

    try {
        var lab = new DunhuangMotionLab();
        lab.start().catch(function (error) {
            lab.destroyed = true;
            showError('敦煌暮色资源加载失败。', error.message);
            if (window.console && window.console.error) window.console.error(error);
        });
    } catch (error) {
        showError('敦煌暮色需要 WebGL2，当前场景已停用。', error.message);
        if (window.console && window.console.error) window.console.error(error);
    }
})();
