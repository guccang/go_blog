(function () {
    'use strict';

    var stage = document.getElementById('celadonStage');
    var canvas = document.getElementById('celadonCanvas');
    if (!stage || !canvas) return;

    var statusTitle = document.getElementById('stageStatusTitle');
    var statusDetail = document.getElementById('stageStatusDetail');
    var windControl = document.getElementById('windControl');
    var rainControl = document.getElementById('rainControl');
    var lightControl = document.getElementById('lightControl');
    var windValue = document.getElementById('windValue');
    var rainValue = document.getElementById('rainValue');
    var lightValue = document.getElementById('lightValue');
    var impactButton = document.getElementById('impactButton');
    var pauseButton = document.getElementById('pauseButton');
    var resetButton = document.getElementById('resetButton');

    var vertexSource = [
        '#version 300 es',
        'precision highp float;',
        'out vec2 v_uv;',
        'void main() {',
        '    vec2 position;',
        '    if (gl_VertexID == 0) position = vec2(-1.0, -1.0);',
        '    else if (gl_VertexID == 1) position = vec2(3.0, -1.0);',
        '    else position = vec2(-1.0, 3.0);',
        '    v_uv = position * 0.5 + 0.5;',
        '    gl_Position = vec4(position, 0.0, 1.0);',
        '}'
    ].join('\n');

    var simulationFragmentSource = [
        '#version 300 es',
        'precision highp float;',
        'in vec2 v_uv;',
        'out vec4 out_state;',
        'uniform sampler2D u_state;',
        'uniform vec2 u_texel;',
        'uniform float u_dt;',
        'uniform vec4 u_impulses[12];',
        'uniform int u_impulse_count;',
        'void main() {',
        '    vec4 center_state = texture(u_state, v_uv);',
        '    float height_value = center_state.r;',
        '    float velocity_value = center_state.g;',
        '    float left_height = texture(u_state, v_uv - vec2(u_texel.x, 0.0)).r;',
        '    float right_height = texture(u_state, v_uv + vec2(u_texel.x, 0.0)).r;',
        '    float down_height = texture(u_state, v_uv - vec2(0.0, u_texel.y)).r;',
        '    float up_height = texture(u_state, v_uv + vec2(0.0, u_texel.y)).r;',
        '    float laplacian = left_height + right_height + down_height + up_height - 4.0 * height_value;',
        '    velocity_value += laplacian * 34.0 * u_dt;',
        '    for (int index = 0; index < 12; index++) {',
        '        if (index >= u_impulse_count) break;',
        '        vec4 impulse = u_impulses[index];',
        '        vec2 offset_value = (v_uv - impulse.xy) / vec2(max(impulse.w, 0.001), max(impulse.w * 0.7, 0.001));',
        '        float energy = exp(-dot(offset_value, offset_value) * 3.4) * impulse.z;',
        '        velocity_value -= energy * 1.8;',
        '    }',
        '    float edge_fade = smoothstep(0.0, 0.055, v_uv.x) * smoothstep(0.0, 0.055, 1.0 - v_uv.x);',
        '    edge_fade *= smoothstep(0.0, 0.07, v_uv.y) * smoothstep(0.0, 0.07, 1.0 - v_uv.y);',
        '    velocity_value *= pow(0.986, u_dt * 60.0);',
        '    height_value += velocity_value * u_dt;',
        '    height_value *= mix(0.94, 1.0, edge_fade);',
        '    out_state = vec4(clamp(height_value, -1.0, 1.0), clamp(velocity_value, -4.0, 4.0), 0.0, 1.0);',
        '}'
    ].join('\n');

    var renderFragmentSource = [
        '#version 300 es',
        'precision highp float;',
        'in vec2 v_uv;',
        'out vec4 out_color;',
        'uniform sampler2D u_wave_state;',
        'uniform sampler2D u_celadon_atlas;',
        'uniform vec2 u_resolution;',
        'uniform vec2 u_wave_texel;',
        'uniform vec2 u_pointer;',
        'uniform float u_time;',
        'uniform float u_wind;',
        'uniform float u_rain;',
        'uniform float u_light;',
        'uniform float u_waterline;',
        'uniform vec4 u_impacts[12];',
        'uniform int u_impact_count;',
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
        '    mat2 transform_value = mat2(1.62, 1.18, -1.18, 1.62);',
        '    for (int octave = 0; octave < 4; octave++) {',
        '        total_value += value_noise(point_value) * amplitude_value;',
        '        point_value = transform_value * point_value + 0.17;',
        '        amplitude_value *= 0.48;',
        '    }',
        '    return total_value;',
        '}',
        '',
        'mat2 rotation(float angle_value) {',
        '    float cosine_value = cos(angle_value);',
        '    float sine_value = sin(angle_value);',
        '    return mat2(cosine_value, -sine_value, sine_value, cosine_value);',
        '}',
        '',
        'vec2 wind_field(vec2 uv_value, float depth_value) {',
        '    float epsilon_value = 0.018;',
        '    vec2 field_point = uv_value * vec2(2.1, 1.45) + vec2(u_time * 0.045, -u_time * 0.022);',
        '    float noise_x_a = fbm(field_point + vec2(epsilon_value, 0.0));',
        '    float noise_x_b = fbm(field_point - vec2(epsilon_value, 0.0));',
        '    float noise_y_a = fbm(field_point + vec2(0.0, epsilon_value));',
        '    float noise_y_b = fbm(field_point - vec2(0.0, epsilon_value));',
        '    vec2 curl_value = vec2((noise_y_a - noise_y_b), -(noise_x_a - noise_x_b)) / (2.0 * epsilon_value);',
        '    float gust_value = sin(u_time * 0.47 + uv_value.y * 2.3) * 0.55;',
        '    gust_value += sin(u_time * 1.13 + uv_value.x * 4.1) * 0.26;',
        '    gust_value += sin(u_time * 2.37 + uv_value.y * 7.0) * 0.11;',
        '    float pointer_influence = exp(-distance(uv_value, u_pointer) * 3.8);',
        '    vec2 pointer_flow = vec2((u_pointer.x - 0.5) * pointer_influence, (u_pointer.y - uv_value.y) * pointer_influence * 0.12);',
        '    return (vec2(0.48 + gust_value * 0.2, -0.035) + curl_value * 0.12 + pointer_flow) * u_wind * mix(0.7, 1.18, depth_value);',
        '}',
        '',
        'vec2 rain_layer(vec2 uv_value, vec2 shared_flow, float columns_value, float rows_value, float speed_value, float width_value, float density_value, float seed_value, float depth_value) {',
        '    vec2 flow_value = shared_flow * mix(0.72, 1.2, depth_value);',
        '    vec2 rain_point = uv_value;',
        '    rain_point.x -= (1.0 - rain_point.y) * flow_value.x * (0.16 + depth_value * 0.08);',
        '    rain_point.y -= flow_value.y * 0.04;',
        '    rain_point *= vec2(columns_value, rows_value);',
        '    rain_point.y += u_time * speed_value;',
        '    vec2 cell_id = floor(rain_point);',
        '    vec2 cell_value = fract(rain_point) - 0.5;',
        '    float random_value = hash21(cell_id + seed_value);',
        '    float visibility_value = smoothstep(1.0 - density_value, 1.0, random_value);',
        '    float x_offset = (hash21(cell_id + seed_value * 2.73) - 0.5) * 0.74;',
        '    float slant_value = flow_value.x * cell_value.y * 0.16;',
        '    float horizontal_distance = abs(cell_value.x - x_offset - slant_value);',
        '    float vertical_mask = 1.0 - smoothstep(0.12, 0.49, abs(cell_value.y));',
        '    vertical_mask *= smoothstep(0.0, 0.1, cell_value.y + 0.5);',
        '    float streak_core = (1.0 - smoothstep(width_value, width_value * 2.2, horizontal_distance)) * vertical_mask;',
        '    float streak_halo = (1.0 - smoothstep(width_value * 2.0, width_value * 6.0, horizontal_distance)) * vertical_mask;',
        '    return vec2(streak_core, streak_halo) * visibility_value;',
        '}',
        '',
        'vec4 celadon_sample(vec2 uv_value, float aspect_value, float angle_value, vec2 optical_shift) {',
        '    vec2 center_value = vec2(0.51, 0.615);',
        '    vec2 local_value = uv_value + optical_shift - center_value;',
        '    local_value.x *= aspect_value;',
        '    vec2 pivot_value = vec2(0.0, 0.235);',
        '    local_value = rotation(-angle_value) * (local_value - pivot_value) + pivot_value;',
        '    vec2 texture_uv = local_value / vec2(0.39, 0.53) + 0.5;',
        '    float inside_value = step(0.0, texture_uv.x) * step(texture_uv.x, 1.0) * step(0.0, texture_uv.y) * step(texture_uv.y, 1.0);',
        '    vec2 atlas_uv = vec2(texture_uv.x * 0.2, 0.5 + texture_uv.y * 0.5);',
        '    vec4 texture_value = texture(u_celadon_atlas, atlas_uv);',
        '    return texture_value * inside_value;',
        '}',
        '',
        'vec3 paper_environment(vec2 uv_value) {',
        '    vec3 paper_color = vec3(0.957, 0.933, 0.875);',
        '    vec3 mist_color = vec3(0.906, 0.937, 0.918);',
        '    float vertical_mist = smoothstep(0.04, 0.96, uv_value.y);',
        '    float cloud_value = fbm(uv_value * vec2(2.4, 1.5) + vec2(u_time * 0.018, 1.7));',
        '    float light_pool = exp(-distance(uv_value, vec2(0.73, 0.78)) * 2.8);',
        '    vec3 environment_color = mix(paper_color, mist_color, vertical_mist * 0.72 + cloud_value * 0.1);',
        '    environment_color *= mix(0.91, 1.075, light_pool * u_light);',
        '    environment_color *= 0.97 + (cloud_value - 0.5) * 0.055;',
        '    return environment_color;',
        '}',
        '',
        'float splash_shape(vec2 uv_value, float aspect_value, vec4 impact_value) {',
        '    float age_value = impact_value.y;',
        '    float strength_value = impact_value.z;',
        '    vec2 local_value = uv_value - vec2(impact_value.x, u_waterline);',
        '    local_value.x *= aspect_value;',
        '    float ring_radius = age_value * (0.026 + strength_value * 0.012);',
        '    float ring_distance = abs(length(vec2(local_value.x, local_value.y * 1.9)) - ring_radius);',
        '    float crown_value = (1.0 - smoothstep(0.002, 0.006, ring_distance));',
        '    crown_value *= 1.0 - smoothstep(0.0, 0.7, age_value);',
        '    float droplets_value = 0.0;',
        '    for (int drop_index = 0; drop_index < 5; drop_index++) {',
        '        float drop_seed = hash21(vec2(float(drop_index), impact_value.w));',
        '        float direction_value = mix(-1.0, 1.0, drop_seed);',
        '        float launch_value = mix(0.018, 0.04, hash21(vec2(impact_value.w, float(drop_index) + 4.0)));',
        '        vec2 drop_position = vec2(direction_value * age_value * launch_value, age_value * launch_value * 1.7 - age_value * age_value * 0.055);',
        '        float drop_radius = mix(0.0012, 0.0025, drop_seed);',
        '        droplets_value += 1.0 - smoothstep(drop_radius, drop_radius * 2.4, length(local_value - drop_position));',
        '    }',
        '    return (crown_value * 0.74 + droplets_value) * strength_value * (1.0 - smoothstep(0.0, 1.0, age_value));',
        '}',
        '',
        'void main() {',
        '    vec2 uv_value = v_uv;',
        '    float aspect_value = u_resolution.x / max(u_resolution.y, 1.0);',
        '    float slow_gust = sin(u_time * 0.39) * 0.018 + sin(u_time * 0.83 + 1.4) * 0.009;',
        '    float vessel_angle = slow_gust * u_wind;',
        '    vec3 color_value = paper_environment(uv_value);',
        '',
        '    float thread_x = 0.51 + sin(vessel_angle) * 0.035;',
        '    float thread_mask = (1.0 - smoothstep(0.00035, 0.0012, abs(uv_value.x - thread_x))) * smoothstep(0.77, 0.82, uv_value.y);',
        '    color_value = mix(color_value, vec3(0.17, 0.23, 0.22), thread_mask * 0.72);',
        '',
        '    vec4 vessel_value = celadon_sample(uv_value, aspect_value, vessel_angle, vec2(0.0));',
        '    vec3 lit_vessel = vessel_value.rgb * mix(0.86, 1.08, u_light);',
        '    color_value = mix(color_value, lit_vessel, vessel_value.a);',
        '',
        '    if (uv_value.y < u_waterline) {',
        '        vec2 simulation_uv = vec2(uv_value.x, clamp(uv_value.y / u_waterline, 0.0, 1.0));',
        '        float wave_height = texture(u_wave_state, simulation_uv).r;',
        '        float wave_left = texture(u_wave_state, simulation_uv - vec2(u_wave_texel.x, 0.0)).r;',
        '        float wave_right = texture(u_wave_state, simulation_uv + vec2(u_wave_texel.x, 0.0)).r;',
        '        float wave_down = texture(u_wave_state, simulation_uv - vec2(0.0, u_wave_texel.y)).r;',
        '        float wave_up = texture(u_wave_state, simulation_uv + vec2(0.0, u_wave_texel.y)).r;',
        '        vec2 wave_normal = vec2(wave_left - wave_right, wave_down - wave_up);',
        '        float water_depth = 1.0 - uv_value.y / u_waterline;',
        '        vec2 refraction_shift = wave_normal * (0.17 + water_depth * 0.08);',
        '        vec2 reflected_uv = vec2(uv_value.x, u_waterline + (u_waterline - uv_value.y) * 0.78);',
        '        vec4 reflection_value = celadon_sample(reflected_uv, aspect_value, -vessel_angle, refraction_shift);',
        '        vec3 shallow_color = vec3(0.51, 0.68, 0.62);',
        '        vec3 deep_color = vec3(0.11, 0.25, 0.23);',
        '        vec3 refracted_environment = paper_environment(uv_value + refraction_shift * 0.45);',
        '        vec3 water_color = mix(shallow_color, deep_color, pow(water_depth, 0.72));',
        '        water_color = mix(water_color, refracted_environment, 0.16 + (1.0 - water_depth) * 0.16);',
        '        float caustic_value = pow(0.5 + 0.5 * sin((uv_value.x + wave_height * 0.08) * 185.0 + u_time * 0.55), 12.0);',
        '        water_color += vec3(0.38, 0.56, 0.5) * caustic_value * (1.0 - water_depth) * 0.12;',
        '        float reflection_fade = (1.0 - smoothstep(0.0, 1.0, water_depth)) * reflection_value.a * 0.56;',
        '        water_color = mix(water_color, reflection_value.rgb * vec3(0.64, 0.79, 0.74), reflection_fade);',
        '        float surface_glint = exp(-abs(uv_value.y - u_waterline) * 420.0);',
        '        surface_glint += clamp(abs(wave_normal.x) + abs(wave_normal.y), 0.0, 1.0) * (1.0 - water_depth) * 1.8;',
        '        water_color += vec3(0.72, 0.84, 0.79) * surface_glint * 0.24 * u_light;',
        '        color_value = water_color;',
        '    }',
        '',
        '    float rain_amount = clamp(u_rain, 0.0, 1.8);',
        '    vec2 shared_flow = wind_field(uv_value, 0.55);',
        '    vec2 far_rain = rain_layer(uv_value, shared_flow, 42.0, 7.0, 2.45, 0.016, min(0.66, 0.25 * rain_amount), 4.7, 0.12);',
        '    vec2 middle_rain = rain_layer(uv_value, shared_flow, 27.0, 5.4, 3.45, 0.022, min(0.78, 0.34 * rain_amount), 11.2, 0.55);',
        '    vec2 near_rain = rain_layer(uv_value, shared_flow, 14.0, 3.8, 4.9, 0.032, min(0.88, 0.42 * rain_amount), 23.9, 1.0);',
        '    vec3 rain_far_color = vec3(0.66, 0.78, 0.73);',
        '    vec3 rain_mid_color = vec3(0.48, 0.66, 0.6);',
        '    vec3 rain_near_color = vec3(0.82, 0.9, 0.86);',
        '    color_value = mix(color_value, rain_far_color, far_rain.y * 0.12 + far_rain.x * 0.16);',
        '    color_value = mix(color_value, rain_mid_color, middle_rain.y * 0.12 + middle_rain.x * 0.36);',
        '    color_value = mix(color_value, rain_near_color, near_rain.y * 0.19 + near_rain.x * 0.5);',
        '',
        '    float splash_value = 0.0;',
        '    for (int impact_index = 0; impact_index < 12; impact_index++) {',
        '        if (impact_index >= u_impact_count) break;',
        '        splash_value += splash_shape(uv_value, aspect_value, u_impacts[impact_index]);',
        '    }',
        '    color_value += vec3(0.64, 0.82, 0.75) * clamp(splash_value, 0.0, 1.0) * 0.72;',
        '',
        '    float vignette_value = 1.0 - smoothstep(0.28, 1.1, distance(uv_value, vec2(0.5, 0.5)));',
        '    color_value *= mix(0.94, 1.025, vignette_value);',
        '    float grain_value = hash21(gl_FragCoord.xy + floor(u_time * 12.0)) - 0.5;',
        '    color_value += grain_value * 0.008;',
        '    out_color = vec4(clamp(color_value, 0.0, 1.0), 1.0);',
        '}'
    ].join('\n');

    function createShader(gl, shaderType, source, label) {
        var shader = gl.createShader(shaderType);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            var message = gl.getShaderInfoLog(shader);
            gl.deleteShader(shader);
            throw new Error(label + '\u7740\u8272\u5668\u7f16\u8bd1\u5931\u8d25: ' + message);
        }
        return shader;
    }

    function createProgram(gl, fragmentSource, label) {
        var program = gl.createProgram();
        var vertexShader = createShader(gl, gl.VERTEX_SHADER, vertexSource, label + '\u9876\u70b9');
        var fragmentShader = createShader(gl, gl.FRAGMENT_SHADER, fragmentSource, label + '\u7247\u5143');
        gl.attachShader(program, vertexShader);
        gl.attachShader(program, fragmentShader);
        gl.linkProgram(program);
        gl.deleteShader(vertexShader);
        gl.deleteShader(fragmentShader);
        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            var message = gl.getProgramInfoLog(program);
            gl.deleteProgram(program);
            throw new Error(label + '\u7740\u8272\u5668\u94fe\u63a5\u5931\u8d25: ' + message);
        }
        return program;
    }

    function uniformMap(gl, program, names) {
        var result = {};
        names.forEach(function (name) {
            var lookupName = name === 'u_impacts' || name === 'u_impulses' ? name + '[0]' : name;
            result[name] = gl.getUniformLocation(program, lookupName);
        });
        return result;
    }

    function seededRandom(seed) {
        var stateValue = seed >>> 0;
        return function () {
            stateValue += 0x6D2B79F5;
            var randomValue = stateValue;
            randomValue = Math.imul(randomValue ^ randomValue >>> 15, randomValue | 1);
            randomValue ^= randomValue + Math.imul(randomValue ^ randomValue >>> 7, randomValue | 61);
            return ((randomValue ^ randomValue >>> 14) >>> 0) / 4294967296;
        };
    }

    function CeladonRainLab() {
        this.gl = canvas.getContext('webgl2', {
            alpha: false,
            antialias: false,
            depth: false,
            stencil: false,
            premultipliedAlpha: false,
            powerPreference: 'high-performance'
        });
        if (!this.gl) throw new Error('\u5f53\u524d\u73af\u5883\u4e0d\u652f\u6301 WebGL2\uff0c\u6837\u677f\u5df2\u505c\u7528\u3002');
        if (!this.gl.getExtension('EXT_color_buffer_float')) {
            throw new Error('\u5f53\u524d WebGL2 \u7f3a\u5c11 EXT_color_buffer_float\uff0c\u65e0\u6cd5\u8fd0\u884c\u6c34\u9762\u9ad8\u5ea6\u573a\u3002');
        }

        this.simulationWidth = window.matchMedia('(max-width: 720px)').matches ? 192 : 320;
        this.simulationHeight = window.matchMedia('(max-width: 720px)').matches ? 80 : 128;
        this.waterline = 0.285;
        this.random = seededRandom(1001);
        this.settings = { wind: 1, rain: 1, light: 1 };
        this.pointer = { x: 0.5, y: 0.62, targetX: 0.5, targetY: 0.62 };
        this.impacts = [];
        this.pendingImpulses = [];
        this.elapsed = 0;
        this.lastTimestamp = 0;
        this.nextImpact = 0.18;
        this.frameHandle = 0;
        this.destroyed = false;
        this.paused = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

        var gl = this.gl;
        this.vertexArray = gl.createVertexArray();
        gl.bindVertexArray(this.vertexArray);
        this.simulationProgram = createProgram(gl, simulationFragmentSource, '\u6c34\u9762\u6a21\u62df');
        this.renderProgram = createProgram(gl, renderFragmentSource, '\u9752\u74f7\u96e8\u6e32\u67d3');
        this.simulationUniforms = uniformMap(gl, this.simulationProgram, [
            'u_state', 'u_texel', 'u_dt', 'u_impulses', 'u_impulse_count'
        ]);
        this.renderUniforms = uniformMap(gl, this.renderProgram, [
            'u_wave_state', 'u_celadon_atlas', 'u_resolution', 'u_wave_texel', 'u_pointer',
            'u_time', 'u_wind', 'u_rain', 'u_light', 'u_waterline', 'u_impacts', 'u_impact_count'
        ]);
        this.createSimulationTargets();
        this.assetTexture = this.createAssetTexture();
        this.bindControls();
        this.resizeObserver = new ResizeObserver(this.resize.bind(this));
        this.resizeObserver.observe(stage);
        this.handleVisibility = this.handleVisibility.bind(this);
        document.addEventListener('visibilitychange', this.handleVisibility);
        canvas.addEventListener('webglcontextlost', this.handleContextLost.bind(this), { once: true });
    }

    CeladonRainLab.prototype.createSimulationTargets = function () {
        var gl = this.gl;
        this.simulationTextures = [];
        this.framebuffers = [];
        for (var index = 0; index < 2; index++) {
            var texture = gl.createTexture();
            gl.bindTexture(gl.TEXTURE_2D, texture);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F, this.simulationWidth, this.simulationHeight, 0, gl.RGBA, gl.HALF_FLOAT, null);
            var framebuffer = gl.createFramebuffer();
            gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
            gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, texture, 0);
            if (gl.checkFramebufferStatus(gl.FRAMEBUFFER) !== gl.FRAMEBUFFER_COMPLETE) {
                throw new Error('\u6c34\u9762\u9ad8\u5ea6\u573a\u5e27\u7f13\u51b2\u521b\u5efa\u5931\u8d25\u3002');
            }
            this.simulationTextures.push(texture);
            this.framebuffers.push(framebuffer);
        }
        this.readTarget = 0;
        this.writeTarget = 1;
        this.clearSimulation();
    };

    CeladonRainLab.prototype.createAssetTexture = function () {
        var gl = this.gl;
        var texture = gl.createTexture();
        gl.bindTexture(gl.TEXTURE_2D, texture);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([231, 239, 234, 0]));
        return texture;
    };

    CeladonRainLab.prototype.loadAsset = function () {
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
            image.onerror = function () { reject(new Error('\u9752\u74f7\u7b26\u53f7\u8d44\u6e90\u52a0\u8f7d\u5931\u8d25\u3002')); };
            image.src = stage.dataset.celadonAsset;
        });
    };

    CeladonRainLab.prototype.clearSimulation = function () {
        var gl = this.gl;
        gl.clearColor(0, 0, 0, 1);
        this.framebuffers.forEach(function (framebuffer) {
            gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
            gl.viewport(0, 0, this.simulationWidth, this.simulationHeight);
            gl.clear(gl.COLOR_BUFFER_BIT);
        }, this);
        gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    };

    CeladonRainLab.prototype.bindControls = function () {
        var self = this;
        [
            { input: windControl, output: windValue, key: 'wind' },
            { input: rainControl, output: rainValue, key: 'rain' },
            { input: lightControl, output: lightValue, key: 'light' }
        ].forEach(function (control) {
            control.input.addEventListener('input', function () {
                self.settings[control.key] = Number(control.input.value);
                control.output.value = Number(control.input.value).toFixed(2);
                if (self.paused) self.render();
            });
        });

        impactButton.addEventListener('click', function () { self.addImpact(0.5, 1.0); });
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
            self.pointer.targetY = 0.62;
        });
        canvas.addEventListener('pointerdown', function (event) {
            var point = self.eventPoint(event);
            if (point.y <= self.waterline + 0.035) self.addImpact(point.x, 1.05);
        });
        this.syncPauseButton();
    };

    CeladonRainLab.prototype.eventPoint = function (event) {
        var bounds = canvas.getBoundingClientRect();
        return {
            x: Math.max(0, Math.min(1, (event.clientX - bounds.left) / Math.max(bounds.width, 1))),
            y: Math.max(0, Math.min(1, 1 - (event.clientY - bounds.top) / Math.max(bounds.height, 1)))
        };
    };

    CeladonRainLab.prototype.syncPauseButton = function () {
        pauseButton.setAttribute('aria-pressed', String(this.paused));
        pauseButton.textContent = this.paused ? '\u7ee7\u7eed' : '\u6682\u505c';
        stage.dataset.paused = String(this.paused);
    };

    CeladonRainLab.prototype.resize = function () {
        var bounds = stage.getBoundingClientRect();
        var dpr = Math.min(window.devicePixelRatio || 1, window.matchMedia('(max-width: 720px)').matches ? 1.2 : 1.6);
        var pixelBudget = window.matchMedia('(max-width: 720px)').matches ? 900000 : 2200000;
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

    CeladonRainLab.prototype.addImpact = function (x, strength) {
        var impact = {
            x: Math.max(0.04, Math.min(0.96, x)),
            age: 0,
            strength: strength,
            seed: this.random() * 100
        };
        this.impacts.push(impact);
        if (this.impacts.length > 12) this.impacts.shift();
        this.pendingImpulses.push({
            x: impact.x,
            y: 0.92,
            strength: strength,
            radius: 0.022 + this.random() * 0.014
        });
        if (this.paused) {
            this.stepSimulation(1 / 60);
            this.render();
        }
    };

    CeladonRainLab.prototype.update = function (dt) {
        this.elapsed += dt;
        this.pointer.x += (this.pointer.targetX - this.pointer.x) * Math.min(1, dt * 2.4);
        this.pointer.y += (this.pointer.targetY - this.pointer.y) * Math.min(1, dt * 2.4);
        this.nextImpact -= dt * this.settings.rain;
        if (this.nextImpact <= 0) {
            this.addImpact(0.08 + this.random() * 0.84, 0.46 + this.random() * 0.52);
            this.nextImpact = 0.22 + this.random() * 0.46;
        }
        this.impacts.forEach(function (impact) { impact.age += dt; });
        this.impacts = this.impacts.filter(function (impact) { return impact.age < 1.45; });
        this.stepSimulation(dt);
    };

    CeladonRainLab.prototype.stepSimulation = function (dt) {
        var gl = this.gl;
        var uniforms = this.simulationUniforms;
        var impulseData = new Float32Array(48);
        this.pendingImpulses.slice(0, 12).forEach(function (impulse, index) {
            impulseData[index * 4] = impulse.x;
            impulseData[index * 4 + 1] = impulse.y;
            impulseData[index * 4 + 2] = impulse.strength;
            impulseData[index * 4 + 3] = impulse.radius;
        });

        gl.bindVertexArray(this.vertexArray);
        gl.useProgram(this.simulationProgram);
        gl.bindFramebuffer(gl.FRAMEBUFFER, this.framebuffers[this.writeTarget]);
        gl.viewport(0, 0, this.simulationWidth, this.simulationHeight);
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, this.simulationTextures[this.readTarget]);
        gl.uniform1i(uniforms.u_state, 0);
        gl.uniform2f(uniforms.u_texel, 1 / this.simulationWidth, 1 / this.simulationHeight);
        gl.uniform1f(uniforms.u_dt, Math.min(dt, 1 / 30));
        gl.uniform4fv(uniforms.u_impulses, impulseData);
        gl.uniform1i(uniforms.u_impulse_count, Math.min(this.pendingImpulses.length, 12));
        gl.drawArrays(gl.TRIANGLES, 0, 3);
        var previousRead = this.readTarget;
        this.readTarget = this.writeTarget;
        this.writeTarget = previousRead;
        this.pendingImpulses = [];
    };

    CeladonRainLab.prototype.render = function () {
        if (!canvas.width || !canvas.height) return;
        var gl = this.gl;
        var uniforms = this.renderUniforms;
        var impactData = new Float32Array(48);
        this.impacts.slice(0, 12).forEach(function (impact, index) {
            impactData[index * 4] = impact.x;
            impactData[index * 4 + 1] = impact.age;
            impactData[index * 4 + 2] = impact.strength;
            impactData[index * 4 + 3] = impact.seed;
        });

        gl.bindVertexArray(this.vertexArray);
        gl.bindFramebuffer(gl.FRAMEBUFFER, null);
        gl.viewport(0, 0, canvas.width, canvas.height);
        gl.useProgram(this.renderProgram);
        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, this.simulationTextures[this.readTarget]);
        gl.uniform1i(uniforms.u_wave_state, 0);
        gl.activeTexture(gl.TEXTURE1);
        gl.bindTexture(gl.TEXTURE_2D, this.assetTexture);
        gl.uniform1i(uniforms.u_celadon_atlas, 1);
        gl.uniform2f(uniforms.u_resolution, canvas.width, canvas.height);
        gl.uniform2f(uniforms.u_wave_texel, 1 / this.simulationWidth, 1 / this.simulationHeight);
        gl.uniform2f(uniforms.u_pointer, this.pointer.x, this.pointer.y);
        gl.uniform1f(uniforms.u_time, this.elapsed);
        gl.uniform1f(uniforms.u_wind, this.settings.wind);
        gl.uniform1f(uniforms.u_rain, this.settings.rain);
        gl.uniform1f(uniforms.u_light, this.settings.light);
        gl.uniform1f(uniforms.u_waterline, this.waterline);
        gl.uniform4fv(uniforms.u_impacts, impactData);
        gl.uniform1i(uniforms.u_impact_count, Math.min(this.impacts.length, 12));
        gl.drawArrays(gl.TRIANGLES, 0, 3);
    };

    CeladonRainLab.prototype.frame = function (timestamp) {
        this.frameHandle = 0;
        if (this.destroyed || this.paused || document.hidden) return;
        var dt = this.lastTimestamp ? Math.min((timestamp - this.lastTimestamp) / 1000, 0.05) : 1 / 60;
        this.lastTimestamp = timestamp;
        this.update(dt);
        this.render();
        this.requestFrame();
    };

    CeladonRainLab.prototype.requestFrame = function () {
        if (this.frameHandle || this.destroyed || this.paused || document.hidden) return;
        this.frameHandle = window.requestAnimationFrame(this.frame.bind(this));
    };

    CeladonRainLab.prototype.reset = function () {
        this.settings = { wind: 1, rain: 1, light: 1 };
        windControl.value = '1';
        rainControl.value = '1';
        lightControl.value = '1';
        windValue.value = '1.00';
        rainValue.value = '1.00';
        lightValue.value = '1.00';
        this.pointer = { x: 0.5, y: 0.62, targetX: 0.5, targetY: 0.62 };
        this.impacts = [];
        this.pendingImpulses = [];
        this.elapsed = 0;
        this.nextImpact = 0.18;
        this.clearSimulation();
        this.render();
    };

    CeladonRainLab.prototype.handleVisibility = function () {
        this.lastTimestamp = 0;
        if (!document.hidden && !this.paused) this.requestFrame();
    };

    CeladonRainLab.prototype.handleContextLost = function (event) {
        event.preventDefault();
        this.destroyed = true;
        showError('WebGL2 \u4e0a\u4e0b\u6587\u5df2\u4e22\u5931\uff0c\u6837\u677f\u5df2\u505c\u7528\u3002', '\u8bf7\u5237\u65b0\u9875\u9762\u91cd\u65b0\u521b\u5efa\u56fe\u5f62\u4e0a\u4e0b\u6587\u3002');
    };

    CeladonRainLab.prototype.start = function () {
        var self = this;
        this.resize();
        return this.loadAsset().then(function () {
            self.addImpact(0.42, 0.82);
            self.addImpact(0.64, 0.68);
            self.stepSimulation(1 / 60);
            self.render();
            stage.dataset.renderState = 'ready';
            if (!self.paused) self.requestFrame();
        });
    };

    function showError(title, detail) {
        stage.dataset.renderState = 'error';
        statusTitle.textContent = title;
        statusDetail.textContent = detail || '\u4e0d\u4f1a\u542f\u7528 Canvas 2D \u6216\u9759\u6001\u8fd1\u4f3c\u6548\u679c\u3002';
    }

    try {
        var lab = new CeladonRainLab();
        lab.start().catch(function (error) {
            showError('\u9752\u74f7\u96e8\u521d\u59cb\u5316\u5931\u8d25', error.message);
            if (window.console && window.console.error) window.console.error(error);
        });
    } catch (error) {
        showError('\u9752\u74f7\u96e8\u65e0\u6cd5\u542f\u52a8', error.message);
        if (window.console && window.console.error) window.console.error(error);
    }
}());
