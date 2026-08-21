(function () {
    'use strict';

    var stage = document.getElementById('dunhuangStage');
    var canvas = document.getElementById('dunhuangCanvas');
    if (!stage || !canvas) return;

    var statusTitle = document.getElementById('stageStatusTitle');
    var statusDetail = document.getElementById('stageStatusDetail');
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
    var windDirectionButtons = Array.prototype.slice.call(document.querySelectorAll('[data-wind-direction]'));
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
        'uniform float u_breath_strength;',
        'uniform float u_breath_period;',
        'uniform float u_tyndall_strength;',
        'uniform float u_beam_spread;',
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
        'float beam_cluster(vec2 uv_value, float aspect_value) {',
        '    vec2 target_value = vec2(0.56, 0.12);',
        '    vec2 direction_value = target_value - u_light_position;',
        '    direction_value.x *= aspect_value;',
        '    direction_value = normalize(direction_value);',
        '    vec2 perpendicular_value = vec2(-direction_value.y, direction_value.x);',
        '    vec2 relative_value = uv_value - u_light_position;',
        '    relative_value.x *= aspect_value;',
        '    float forward_value = dot(relative_value, direction_value);',
        '    float lateral_value = dot(relative_value, perpendicular_value);',
        '    float travel_value = smoothstep(0.015, 0.11, forward_value) * (1.0 - smoothstep(0.74, 1.28, forward_value));',
        '    float cluster_value = 0.0;',
        '    for (int beam_index = 0; beam_index < 5; beam_index++) {',
        '        float seed_value = hash21(vec2(float(beam_index) + 3.2, 9.7));',
        '        float index_value = float(beam_index) / 4.0 * 2.0 - 1.0;',
        '        float beam_center = (index_value + (seed_value - 0.5) * 0.2) * u_beam_spread * (0.1 + max(forward_value, 0.0) * 0.72);',
        '        float width_value = mix(0.012, 0.027, seed_value) * (0.8 + max(forward_value, 0.0) * 0.48);',
        '        float beam_value = 1.0 - smoothstep(width_value * 0.38, width_value * 2.4, abs(lateral_value - beam_center));',
        '        cluster_value = max(cluster_value, beam_value * mix(0.7, 1.0, seed_value));',
        '    }',
        '    return mix(0.13, 1.0, cluster_value) * travel_value;',
        '}',
        '',
        'float tyndall_scattering(vec2 uv_value, float aspect_value) {',
        '    vec2 light_path = u_light_position - uv_value;',
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
        '    return volume_value / 6.0 * beam_cluster(uv_value, aspect_value) * u_dust_density;',
        '}',
        '',
        'vec4 dunhuang_sample(vec2 uv_value, float aspect_value) {',
        '    vec2 local_value = uv_value - vec2(0.51, 0.5);',
        '    local_value.x *= aspect_value;',
        '    vec2 texture_uv = local_value / vec2(0.82, 0.78) + 0.5;',
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
        'vec3 fresco_background(vec2 uv_value, float light_energy) {',
        '    vec3 night_color = vec3(0.169, 0.145, 0.2);',
        '    vec3 violet_color = vec3(0.4, 0.314, 0.42);',
        '    vec3 clay_color = vec3(0.71, 0.416, 0.298);',
        '    vec3 gold_color = vec3(0.851, 0.643, 0.255);',
        '    float vertical_value = smoothstep(0.0, 1.0, uv_value.y);',
        '    vec3 color_value = mix(night_color, violet_color, vertical_value * 0.72);',
        '    float horizon_value = exp(-pow((uv_value.y - 0.34) * 4.8, 2.0));',
        '    color_value = mix(color_value, clay_color, horizon_value * 0.2);',
        '    float light_pool = exp(-distance(uv_value, u_light_position) * 3.4);',
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
        '    vec3 color_value = fresco_background(uv_value, light_energy);',
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
        '    float symbol_light = exp(-distance(uv_value, u_light_position) * 2.6) * light_energy;',
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
        this.gl = canvas.getContext('webgl2', {
            alpha: false,
            antialias: false,
            depth: false,
            stencil: false,
            premultipliedAlpha: false,
            powerPreference: 'high-performance'
        });
        if (!this.gl) throw new Error('当前环境不支持 WebGL2，敦煌暮色样板已停用。');

        this.settings = {
            windStrength: 0.9,
            dustDensity: 0.72,
            ribbonAmplitude: 0.72,
            ribbonTension: 0.58,
            ribbonResponse: 0.8,
            lightStrength: 0.78,
            lightX: 0.34,
            lightY: 0.72,
            breathingEnabled: true,
            breathStrength: 0.24,
            breathPeriod: 7,
            tyndallEnabled: true,
            tyndallStrength: 0.82,
            beamSpread: 0.34
        };
        this.windDirectionKey = 'west';
        this.windAngle = 270;
        this.targetWindAngle = 270;
        this.pointer = { x: 0.5, y: 0.5, targetX: 0.5, targetY: 0.5 };
        this.elapsed = 0;
        this.gust = 0;
        this.lastTimestamp = 0;
        this.frameHandle = 0;
        this.destroyed = false;
        this.paused = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

        var gl = this.gl;
        this.vertexArray = gl.createVertexArray();
        gl.bindVertexArray(this.vertexArray);
        this.program = createProgram(gl);
        this.uniforms = uniformMap(gl, this.program, [
            'u_dunhuang_atlas', 'u_resolution', 'u_pointer', 'u_time',
            'u_wind_direction', 'u_wind_strength', 'u_gust', 'u_dust_density',
            'u_ribbon_amplitude', 'u_ribbon_tension', 'u_ribbon_response',
            'u_light_strength', 'u_light_position', 'u_breath_strength', 'u_breath_period',
            'u_tyndall_strength', 'u_beam_spread'
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
        var self = this;
        [
            { input: windStrengthControl, output: windStrengthValue, key: 'windStrength', format: fixedTwo },
            { input: dustDensityControl, output: dustDensityValue, key: 'dustDensity', format: fixedTwo },
            { input: ribbonAmplitudeControl, output: ribbonAmplitudeValue, key: 'ribbonAmplitude', format: fixedTwo },
            { input: ribbonTensionControl, output: ribbonTensionValue, key: 'ribbonTension', format: fixedTwo },
            { input: ribbonResponseControl, output: ribbonResponseValue, key: 'ribbonResponse', format: fixedTwo },
            { input: lightStrengthControl, output: lightStrengthValue, key: 'lightStrength', format: fixedTwo },
            { input: lightXControl, output: lightXValue, key: 'lightX', format: percentValue },
            { input: lightYControl, output: lightYValue, key: 'lightY', format: percentValue },
            { input: breathStrengthControl, output: breathStrengthValue, key: 'breathStrength', format: fixedTwo },
            { input: breathPeriodControl, output: breathPeriodValue, key: 'breathPeriod', format: function (value) { return value.toFixed(1) + ' s'; } },
            { input: tyndallStrengthControl, output: tyndallStrengthValue, key: 'tyndallStrength', format: fixedTwo },
            { input: beamSpreadControl, output: beamSpreadValue, key: 'beamSpread', format: fixedTwo }
        ].forEach(function (control) {
            control.input.addEventListener('input', function () {
                var value = Number(control.input.value);
                self.settings[control.key] = value;
                control.output.value = control.format(value);
                if (self.paused) self.render();
            });
        });

        windDirectionButtons.forEach(function (button) {
            button.addEventListener('click', function () { self.setWindDirection(button.dataset.windDirection); });
        });
        [
            { button: breathingToggle, key: 'breathingEnabled' },
            { button: tyndallToggle, key: 'tyndallEnabled' }
        ].forEach(function (effect) {
            effect.button.addEventListener('click', function () {
                self.settings[effect.key] = !self.settings[effect.key];
                self.syncLightControls();
                if (self.paused) self.render();
            });
        });

        gustButton.addEventListener('click', function () {
            self.gust = Math.min(1.4, self.gust + 1);
            if (self.paused) self.render();
        });
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

        this.setWindDirection('west', true);
        this.syncLightControls();
        this.syncPauseButton();
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
        var flow = meteorologicalFlow(this.windAngle);
        var flowAngle = Math.atan2(flow[1], flow[0]) * 180 / Math.PI;
        windCompass.style.setProperty('--wind-flow-angle', flowAngle + 'deg');
    };

    DunhuangMotionLab.prototype.syncLightControls = function () {
        breathingToggle.setAttribute('aria-checked', String(this.settings.breathingEnabled));
        breathingToggle.textContent = this.settings.breathingEnabled ? '已开启' : '已关闭';
        tyndallToggle.setAttribute('aria-checked', String(this.settings.tyndallEnabled));
        tyndallToggle.textContent = this.settings.tyndallEnabled ? '已开启' : '已关闭';
        lightInstrument.dataset.breathing = this.settings.breathingEnabled ? 'on' : 'off';
        lightInstrument.dataset.tyndall = this.settings.tyndallEnabled ? 'on' : 'off';
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
            y: Math.max(0, Math.min(1, 1 - (event.clientY - bounds.top) / Math.max(bounds.height, 1)))
        };
    };

    DunhuangMotionLab.prototype.resize = function () {
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
        gl.uniform1f(uniforms.u_breath_strength, this.settings.breathingEnabled ? this.settings.breathStrength : 0);
        gl.uniform1f(uniforms.u_breath_period, this.settings.breathPeriod);
        gl.uniform1f(uniforms.u_tyndall_strength, this.settings.tyndallEnabled ? this.settings.tyndallStrength : 0);
        gl.uniform1f(uniforms.u_beam_spread, this.settings.beamSpread);
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
        if (this.frameHandle || this.destroyed || this.paused || document.hidden) return;
        this.frameHandle = window.requestAnimationFrame(this.frame.bind(this));
    };

    DunhuangMotionLab.prototype.reset = function () {
        this.settings = {
            windStrength: 0.9,
            dustDensity: 0.72,
            ribbonAmplitude: 0.72,
            ribbonTension: 0.58,
            ribbonResponse: 0.8,
            lightStrength: 0.78,
            lightX: 0.34,
            lightY: 0.72,
            breathingEnabled: true,
            breathStrength: 0.24,
            breathPeriod: 7,
            tyndallEnabled: true,
            tyndallStrength: 0.82,
            beamSpread: 0.34
        };
        var resetValues = [
            [windStrengthControl, windStrengthValue, '0.9', '0.90'],
            [dustDensityControl, dustDensityValue, '0.72', '0.72'],
            [ribbonAmplitudeControl, ribbonAmplitudeValue, '0.72', '0.72'],
            [ribbonTensionControl, ribbonTensionValue, '0.58', '0.58'],
            [ribbonResponseControl, ribbonResponseValue, '0.8', '0.80'],
            [lightStrengthControl, lightStrengthValue, '0.78', '0.78'],
            [lightXControl, lightXValue, '0.34', '34%'],
            [lightYControl, lightYValue, '0.72', '72%'],
            [breathStrengthControl, breathStrengthValue, '0.24', '0.24'],
            [breathPeriodControl, breathPeriodValue, '7', '7.0 s'],
            [tyndallStrengthControl, tyndallStrengthValue, '0.82', '0.82'],
            [beamSpreadControl, beamSpreadValue, '0.34', '0.34']
        ];
        resetValues.forEach(function (item) { item[0].value = item[2]; item[1].value = item[3]; });
        this.setWindDirection('west', true);
        this.syncLightControls();
        this.pointer = { x: 0.5, y: 0.5, targetX: 0.5, targetY: 0.5 };
        this.elapsed = 0;
        this.gust = 0;
        this.render();
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

    function fixedTwo(value) { return value.toFixed(2); }
    function percentValue(value) { return Math.round(value * 100) + '%'; }

    function showError(title, detail) {
        stage.dataset.renderState = 'error';
        statusTitle.textContent = title;
        statusDetail.textContent = detail || '当前样板不会回退到其他渲染方式。';
    }

    try {
        var lab = new DunhuangMotionLab();
        lab.resize();
        lab.loadAsset().then(function () {
            if (lab.destroyed) return;
            lab.render();
            stage.dataset.renderState = 'ready';
            lab.requestFrame();
        }).catch(function (error) {
            lab.destroyed = true;
            showError('敦煌暮色资源加载失败。', error.message);
            window.console.error(error);
        });
    } catch (error) {
        showError('敦煌暮色需要 WebGL2，当前场景已停用。', error.message);
        window.console.error(error);
    }
})();
