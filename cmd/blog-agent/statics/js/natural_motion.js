(function () {
    'use strict';

    var root = document.documentElement;
    var effectFactories = Object.create(null);
    var presets = Object.create(null);
    var scenes = [];
    var activeScenes = [];
    var frameHandle = 0;
    var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');

    function clamp(value, minimum, maximum) {
        return Math.max(minimum, Math.min(maximum, value));
    }

    function copy(value) {
        return JSON.parse(JSON.stringify(value));
    }

    function seededRandom(seed) {
        var state = seed >>> 0;
        return function () {
            state += 0x6D2B79F5;
            var value = state;
            value = Math.imul(value ^ value >>> 15, value | 1);
            value ^= value + Math.imul(value ^ value >>> 7, value | 61);
            return ((value ^ value >>> 14) >>> 0) / 4294967296;
        };
    }

    function colorChannels(hex) {
        var value = String(hex || '#7FA99A').replace('#', '');
        if (value.length === 3) {
            value = value.split('').map(function (part) { return part + part; }).join('');
        }
        var parsed = parseInt(value, 16);
        return [
            ((parsed >> 16) & 255) / 255,
            ((parsed >> 8) & 255) / 255,
            (parsed & 255) / 255
        ];
    }

    function registerEffect(name, factory) {
        effectFactories[name] = factory;
    }

    function registerPreset(name, config) {
        presets[name] = copy(config);
    }

    function addActiveScene(scene) {
        if (activeScenes.indexOf(scene) === -1) activeScenes.push(scene);
        if (!frameHandle) frameHandle = window.requestAnimationFrame(runFrame);
    }

    function removeActiveScene(scene) {
        var index = activeScenes.indexOf(scene);
        if (index !== -1) activeScenes.splice(index, 1);
    }

    function runFrame(timestamp) {
        frameHandle = 0;
        activeScenes.slice().forEach(function (scene) { scene.frame(timestamp); });
        if (activeScenes.length) frameHandle = window.requestAnimationFrame(runFrame);
    }

    function compileShader(gl, type, source) {
        var shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            var message = gl.getShaderInfoLog(shader);
            gl.deleteShader(shader);
            throw new Error('自然动效着色器编译失败: ' + message);
        }
        return shader;
    }

    function createProgram(gl, vertexSource, fragmentSource) {
        var program = gl.createProgram();
        var vertexShader = compileShader(gl, gl.VERTEX_SHADER, vertexSource);
        var fragmentShader = compileShader(gl, gl.FRAGMENT_SHADER, fragmentSource);
        gl.attachShader(program, vertexShader);
        gl.attachShader(program, fragmentShader);
        gl.linkProgram(program);
        gl.deleteShader(vertexShader);
        gl.deleteShader(fragmentShader);
        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            var message = gl.getProgramInfoLog(program);
            gl.deleteProgram(program);
            throw new Error('自然动效着色器链接失败: ' + message);
        }
        return program;
    }

    var vertexShaderSource = [
        '#version 300 es',
        'in vec2 a_position;',
        'out vec2 v_uv;',
        'void main() {',
        '    v_uv = a_position * 0.5 + 0.5;',
        '    gl_Position = vec4(a_position, 0.0, 1.0);',
        '}'
    ].join('\n');

    var fragmentShaderSource = [
        '#version 300 es',
        'precision highp float;',
        'in vec2 v_uv;',
        'out vec4 out_color;',
        'uniform vec2 u_resolution;',
        'uniform float u_time;',
        'uniform float u_wind;',
        'uniform float u_rain_density;',
        'uniform float u_rain_speed;',
        'uniform float u_light;',
        'uniform vec3 u_rain_color;',
        'uniform vec3 u_ripple_color;',
        'uniform vec4 u_ripples[8];',
        'uniform int u_ripple_count;',
        '',
        'float hash21(vec2 point) {',
        '    point = fract(point * vec2(123.34, 456.21));',
        '    point += dot(point, point + 45.32);',
        '    return fract(point.x * point.y);',
        '}',
        '',
        'float rain_layer(vec2 uv, float scale, float speed, float seed) {',
        '    vec2 point = uv;',
        '    point.x -= (1.0 - point.y) * u_wind * (0.11 + scale * 0.025);',
        '    point *= vec2(21.0, 5.5) * scale;',
        '    point.y += u_time * speed * u_rain_speed;',
        '    vec2 cell_id = floor(point);',
        '    vec2 cell = fract(point) - 0.5;',
        '    float random_value = hash21(cell_id + seed);',
        '    float active = smoothstep(1.0 - u_rain_density, 1.0, random_value);',
        '    float offset = (hash21(cell_id + seed * 2.7) - 0.5) * 0.72;',
        '    float width = mix(0.012, 0.026, scale - 0.65);',
        '    float streak = 1.0 - smoothstep(width, width * 2.5, abs(cell.x - offset));',
        '    float tapered = 1.0 - smoothstep(0.16, 0.5, abs(cell.y + 0.06));',
        '    return streak * tapered * active;',
        '}',
        '',
        'void main() {',
        '    vec2 uv = v_uv;',
        '    float rain = rain_layer(uv, 0.72, 0.72, 2.3) * 0.24;',
        '    rain += rain_layer(uv, 1.0, 1.0, 7.1) * 0.42;',
        '    rain += rain_layer(uv, 1.28, 1.24, 13.7) * 0.27;',
        '',
        '    float aspect = u_resolution.x / max(u_resolution.y, 1.0);',
        '    float ripple = 0.0;',
        '    for (int index = 0; index < 8; index++) {',
        '        if (index >= u_ripple_count) break;',
        '        vec4 wave = u_ripples[index];',
        '        vec2 delta = vec2((uv.x - wave.x) * aspect, uv.y - wave.y);',
        '        float distance_to_center = length(delta);',
        '        float radius = wave.z * (0.052 + wave.w * 0.009);',
        '        float width = 0.008 + wave.z * 0.0025;',
        '        float primary = exp(-pow((distance_to_center - radius) / width, 2.0));',
        '        float echo = exp(-pow((distance_to_center - radius * 0.68) / (width * 1.35), 2.0)) * 0.34;',
        '        ripple += (primary + echo) * exp(-wave.z * 0.82) * wave.w;',
        '    }',
        '',
        '    float light_distance = distance(uv, vec2(0.76, 0.48));',
        '    float breathing_light = exp(-light_distance * 3.7) * (0.018 + u_light * 0.024);',
        '    float alpha = clamp(rain + ripple * 0.68 + breathing_light, 0.0, 0.62);',
        '    vec3 color = mix(u_rain_color, u_ripple_color, clamp(ripple * 1.4, 0.0, 1.0));',
        '    color = mix(color, vec3(0.91, 0.96, 0.94), breathing_light * 7.0);',
        '    out_color = vec4(color, alpha);',
        '}'
    ].join('\n');

    function WebGL2Renderer(scene) {
        this.scene = scene;
        this.canvas = scene.canvas;
        this.gl = this.canvas.getContext('webgl2', {
            alpha: true,
            antialias: false,
            depth: false,
            stencil: false,
            premultipliedAlpha: true,
            powerPreference: 'high-performance'
        });
        if (!this.gl) throw new Error('WebGL2 unavailable');

        var gl = this.gl;
        this.program = createProgram(gl, vertexShaderSource, fragmentShaderSource);
        this.buffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, this.buffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
        gl.useProgram(this.program);
        var position = gl.getAttribLocation(this.program, 'a_position');
        gl.enableVertexAttribArray(position);
        gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);
        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

        this.uniforms = {};
        [
            'u_resolution', 'u_time', 'u_wind', 'u_rain_density', 'u_rain_speed',
            'u_light', 'u_rain_color', 'u_ripple_color', 'u_ripples', 'u_ripple_count'
        ].forEach(function (name) {
            this.uniforms[name] = gl.getUniformLocation(this.program, name === 'u_ripples' ? 'u_ripples[0]' : name);
        }, this);

        this.rainColor = colorChannels(scene.config.renderer.rainColor);
        this.rippleColor = colorChannels(scene.config.renderer.rippleColor);
        this.handleContextLost = function (event) {
            event.preventDefault();
            scene.disable('webgl2-context-lost');
        };
        this.canvas.addEventListener('webglcontextlost', this.handleContextLost, { once: true });
        this.canvas.dataset.motionRenderer = 'webgl2';
    }

    WebGL2Renderer.prototype.resize = function (width, height, dpr) {
        this.canvas.width = Math.max(1, Math.round(width * dpr));
        this.canvas.height = Math.max(1, Math.round(height * dpr));
        this.gl.viewport(0, 0, this.canvas.width, this.canvas.height);
    };

    WebGL2Renderer.prototype.render = function (state) {
        var gl = this.gl;
        var uniforms = this.uniforms;
        var rippleData = new Float32Array(32);
        var ripples = state.ripples.slice(0, 8);
        ripples.forEach(function (ripple, index) {
            rippleData[index * 4] = ripple.x;
            rippleData[index * 4 + 1] = ripple.y;
            rippleData[index * 4 + 2] = ripple.age;
            rippleData[index * 4 + 3] = ripple.strength;
        });

        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.useProgram(this.program);
        gl.uniform2f(uniforms.u_resolution, this.canvas.width, this.canvas.height);
        gl.uniform1f(uniforms.u_time, state.time);
        gl.uniform1f(uniforms.u_wind, state.wind);
        gl.uniform1f(uniforms.u_rain_density, state.rainDensity);
        gl.uniform1f(uniforms.u_rain_speed, state.rainSpeed);
        gl.uniform1f(uniforms.u_light, state.light);
        gl.uniform3fv(uniforms.u_rain_color, this.rainColor);
        gl.uniform3fv(uniforms.u_ripple_color, this.rippleColor);
        gl.uniform4fv(uniforms.u_ripples, rippleData);
        gl.uniform1i(uniforms.u_ripple_count, ripples.length);
        gl.drawArrays(gl.TRIANGLES, 0, 3);
    };

    WebGL2Renderer.prototype.destroy = function () {
        this.canvas.removeEventListener('webglcontextlost', this.handleContextLost);
        if (!this.gl.isContextLost()) {
            this.gl.deleteBuffer(this.buffer);
            this.gl.deleteProgram(this.program);
        }
    };

    function createRenderer(scene) {
        try {
            return new WebGL2Renderer(scene);
        } catch (error) {
            scene.host.dataset.naturalMotionUnavailable = 'webgl2';
            scene.host.classList.add('is-natural-motion-unavailable');
            scene.disabled = true;
            scene.canvas.remove();
            if (window.console && window.console.warn) {
                window.console.warn('自然动效需要 WebGL2，当前场景已停用。', error);
            }
            return null;
        }
    }

    function WindEffect(config) {
        this.base = Number(config.base || 0);
        this.gust = Number(config.gust || 0.22);
        this.pointerInfluence = Number(config.pointerInfluence || 0.08);
    }

    WindEffect.prototype.update = function (state) {
        var slowCurrent = Math.sin(state.time * 0.43 + 0.7) * 0.58;
        var middleCurrent = Math.sin(state.time * 0.91 + 2.2) * 0.29;
        var fineCurrent = Math.sin(state.time * 1.77 + 4.1) * 0.13;
        var target = this.base + (slowCurrent + middleCurrent + fineCurrent) * this.gust;
        target += state.pointerX * this.pointerInfluence;
        state.wind += (target - state.wind) * Math.min(1, state.dt * 1.8);
    };

    function RainEffect(config, scene) {
        this.scene = scene;
        this.density = Number(config.density || 0.3);
        this.speed = Number(config.speed || 1);
        this.surfaceY = Number(config.surfaceY || 0.76);
        this.untilImpact = 0.18;
    }

    RainEffect.prototype.update = function (state) {
        state.rainDensity = this.density * this.scene.quality.density;
        state.rainSpeed = this.speed;
        this.untilImpact -= state.dt;
        if (this.untilImpact <= 0) {
            this.scene.emit('surface-impact', {
                x: 0.08 + this.scene.random() * 0.84,
                y: this.surfaceY + (this.scene.random() - 0.5) * 0.025,
                strength: 0.45 + this.scene.random() * 0.55
            });
            this.untilImpact = (0.34 + this.scene.random() * 0.62) / Math.max(this.density, 0.12);
        }
    };

    function RippleEffect(config, scene) {
        this.duration = Number(config.duration || 3.2);
        scene.on('surface-impact', function (impact) {
            scene.state.ripples.push({
                x: impact.x,
                y: impact.y,
                age: 0,
                strength: impact.strength
            });
            if (scene.state.ripples.length > 8) scene.state.ripples.shift();
        });
    }

    RippleEffect.prototype.update = function (state) {
        var duration = this.duration;
        state.ripples.forEach(function (ripple) { ripple.age += state.dt; });
        state.ripples = state.ripples.filter(function (ripple) { return ripple.age < duration; });
    };

    function LightBreathEffect(config) {
        this.speed = Number(config.speed || 0.42);
        this.intensity = Number(config.intensity || 0.18);
    }

    LightBreathEffect.prototype.update = function (state) {
        var primary = Math.sin(state.time * this.speed) * 0.68;
        var secondary = Math.sin(state.time * this.speed * 0.47 + 1.9) * 0.32;
        state.light = 0.72 + (primary + secondary + 1) * 0.5 * this.intensity;
    };

    function SwayEffect(config) {
        this.strength = Number(config.strength || 1);
        this.value = 0;
    }

    SwayEffect.prototype.update = function (state) {
        var target = state.wind * this.strength;
        this.value += (target - this.value) * Math.min(1, state.dt * 2.4);
        state.sway = this.value;
    };

    registerEffect('wind', function (config) { return new WindEffect(config); });
    registerEffect('rain', function (config, scene) { return new RainEffect(config, scene); });
    registerEffect('ripple', function (config, scene) { return new RippleEffect(config, scene); });
    registerEffect('light-breathe', function (config) { return new LightBreathEffect(config); });
    registerEffect('sway', function (config) { return new SwayEffect(config); });

    registerPreset('celadon-rain', {
        seed: 1001,
        theme: 'atlas-celadon',
        effects: [
            { name: 'wind', base: 0.08, gust: 0.26, pointerInfluence: 0.06 },
            { name: 'rain', density: 0.34, speed: 0.92, surfaceY: 0.77 },
            { name: 'ripple', duration: 3.2 },
            { name: 'light-breathe', speed: 0.48, intensity: 0.16 },
            { name: 'sway', strength: 1.15 }
        ],
        renderer: {
            rainColor: '#7FA99A',
            rippleColor: '#3F6F66'
        }
    });

    function MotionScene(host, presetName, overrides) {
        var preset = presets[presetName];
        if (!preset) throw new Error('未知自然动效预设: ' + presetName);

        this.host = host;
        this.presetName = presetName;
        this.config = copy(preset);
        this.theme = overrides && Object.prototype.hasOwnProperty.call(overrides, 'theme')
            ? overrides.theme
            : host.dataset.motionTheme || this.config.theme || '';
        this.random = seededRandom(Number(this.config.seed || 1));
        this.listeners = Object.create(null);
        this.effects = [];
        this.visible = true;
        this.started = false;
        this.disabled = false;
        this.lastTimestamp = 0;
        this.pointerTarget = 0;
        this.quality = {
            mobile: window.matchMedia('(max-width: 720px)').matches,
            density: window.matchMedia('(max-width: 720px)').matches ? 0.62 : 1,
            dpr: Math.min(window.devicePixelRatio || 1, window.matchMedia('(max-width: 720px)').matches ? 1.25 : 1.75)
        };
        this.state = {
            time: 0,
            dt: 0,
            wind: 0,
            light: 0.8,
            sway: 0,
            rainDensity: 0,
            rainSpeed: 1,
            pointerX: 0,
            ripples: []
        };

        if (overrides && overrides.renderer) {
            Object.keys(overrides.renderer).forEach(function (key) {
                this.config.renderer[key] = overrides.renderer[key];
            }, this);
        }

        this.canvas = document.createElement('canvas');
        this.canvas.className = 'natural-motion__canvas';
        this.canvas.setAttribute('aria-hidden', 'true');
        host.insertBefore(this.canvas, host.firstChild);
        host.classList.add('natural-motion-scene');
        host.dataset.naturalMotionReady = 'true';

        this.config.effects.forEach(function (effectConfig) {
            var factory = effectFactories[effectConfig.name];
            if (factory) this.effects.push(factory(effectConfig, this));
        }, this);

        this.renderer = createRenderer(this);
        if (!this.renderer) return;
        this.bindEnvironment();
        this.resize();
        this.refreshActivity();
    }

    MotionScene.prototype.on = function (name, callback) {
        if (!this.listeners[name]) this.listeners[name] = [];
        this.listeners[name].push(callback);
    };

    MotionScene.prototype.emit = function (name, payload) {
        (this.listeners[name] || []).forEach(function (callback) { callback(payload); });
    };

    MotionScene.prototype.matchesTheme = function () {
        return !this.theme || root.dataset.theme === this.theme;
    };

    MotionScene.prototype.bindEnvironment = function () {
        var scene = this;
        this.handlePointerMove = function (event) {
            var bounds = scene.host.getBoundingClientRect();
            if (!bounds.width) return;
            scene.pointerTarget = clamp(((event.clientX - bounds.left) / bounds.width - 0.5) * 2, -1, 1);
        };
        this.handlePointerLeave = function () { scene.pointerTarget = 0; };
        this.handleThemeChange = function () { scene.refreshActivity(); };
        this.handleVisibility = function () { scene.refreshActivity(); };
        this.handleReducedMotion = function () { scene.refreshActivity(); };

        this.host.addEventListener('pointermove', this.handlePointerMove, { passive: true });
        this.host.addEventListener('pointerleave', this.handlePointerLeave, { passive: true });
        window.addEventListener('guccang:themechange', this.handleThemeChange);
        document.addEventListener('visibilitychange', this.handleVisibility);
        if (reducedMotion.addEventListener) reducedMotion.addEventListener('change', this.handleReducedMotion);

        if ('IntersectionObserver' in window) {
            this.intersectionObserver = new IntersectionObserver(function (entries) {
                entries.forEach(function (entry) {
                    if (entry.target !== scene.host) return;
                    scene.visible = entry.isIntersecting;
                    scene.refreshActivity();
                });
            }, { rootMargin: '120px' });
            this.intersectionObserver.observe(this.host);
        }

        if ('ResizeObserver' in window) {
            this.resizeObserver = new ResizeObserver(function () { scene.resize(); });
            this.resizeObserver.observe(this.host);
        } else {
            this.handleResize = function () { scene.resize(); };
            window.addEventListener('resize', this.handleResize, { passive: true });
        }
    };

    MotionScene.prototype.resize = function () {
        if (!this.renderer) return;
        var bounds = this.host.getBoundingClientRect();
        if (!bounds.width || !bounds.height) return;
        this.renderer.resize(bounds.width, bounds.height, this.quality.dpr);
    };

    MotionScene.prototype.refreshActivity = function () {
        if (!this.renderer || this.disabled) return;
        var themeActive = this.matchesTheme();
        var shouldRun = themeActive && this.visible && !document.hidden && !reducedMotion.matches;
        this.host.classList.toggle('is-natural-motion-theme-active', themeActive);
        this.host.classList.toggle('is-natural-motion-reduced', reducedMotion.matches);
        if (shouldRun) this.start(); else this.stop();
        if (themeActive && reducedMotion.matches) {
            this.state.time = 0;
            this.state.dt = 0;
            this.renderer.render(this.state);
        }
    };

    MotionScene.prototype.start = function () {
        if (this.started || this.disabled) return;
        this.started = true;
        this.lastTimestamp = 0;
        addActiveScene(this);
        root.classList.add('has-natural-motion');
    };

    MotionScene.prototype.stop = function () {
        if (!this.started) return;
        this.started = false;
        removeActiveScene(this);
    };

    MotionScene.prototype.frame = function (timestamp) {
        if (!this.started || this.disabled) return;
        if (!this.lastTimestamp) this.lastTimestamp = timestamp;
        this.state.dt = clamp((timestamp - this.lastTimestamp) / 1000, 0, 0.034);
        this.state.time += this.state.dt;
        this.lastTimestamp = timestamp;
        this.state.pointerX += (this.pointerTarget - this.state.pointerX) * Math.min(1, this.state.dt * 2.6);
        this.effects.forEach(function (effect) { effect.update(this.state); }, this);
        this.host.style.setProperty('--natural-wind', this.state.sway.toFixed(4));
        this.host.style.setProperty('--natural-light', this.state.light.toFixed(4));
        this.renderer.render(this.state);
    };

    MotionScene.prototype.disable = function (reason) {
        if (this.disabled) return;
        this.disabled = true;
        this.stop();
        this.host.dataset.naturalMotionUnavailable = reason;
        this.host.classList.add('is-natural-motion-unavailable');
        this.canvas.hidden = true;
    };

    MotionScene.prototype.destroy = function () {
        if (!this.renderer) {
            delete this.host.dataset.naturalMotionReady;
            delete this.host.dataset.naturalMotionUnavailable;
            this.host.classList.remove('natural-motion-scene', 'is-natural-motion-unavailable');
            var unavailableIndex = scenes.indexOf(this);
            if (unavailableIndex !== -1) scenes.splice(unavailableIndex, 1);
            return;
        }
        this.stop();
        if (this.intersectionObserver) this.intersectionObserver.disconnect();
        if (this.resizeObserver) this.resizeObserver.disconnect();
        if (this.handleResize) window.removeEventListener('resize', this.handleResize);
        this.host.removeEventListener('pointermove', this.handlePointerMove);
        this.host.removeEventListener('pointerleave', this.handlePointerLeave);
        window.removeEventListener('guccang:themechange', this.handleThemeChange);
        document.removeEventListener('visibilitychange', this.handleVisibility);
        if (reducedMotion.removeEventListener) reducedMotion.removeEventListener('change', this.handleReducedMotion);
        this.renderer.destroy();
        this.canvas.remove();
        delete this.host.dataset.naturalMotionReady;
        delete this.host.dataset.naturalMotionUnavailable;
        this.host.classList.remove(
            'natural-motion-scene',
            'is-natural-motion-theme-active',
            'is-natural-motion-reduced',
            'is-natural-motion-unavailable'
        );
        this.host.style.removeProperty('--natural-wind');
        this.host.style.removeProperty('--natural-light');
        var index = scenes.indexOf(this);
        if (index !== -1) scenes.splice(index, 1);
    };

    function mount(host, presetName, overrides) {
        if (!host || host.dataset.naturalMotionReady === 'true') return null;
        var scene = new MotionScene(host, presetName || host.dataset.naturalMotion, overrides || {});
        scenes.push(scene);
        return scene;
    }

    function init() {
        document.querySelectorAll('[data-natural-motion]').forEach(function (host) {
            mount(host, host.dataset.naturalMotion);
        });
    }

    window.GuCcangNaturalMotion = {
        registerEffect: registerEffect,
        registerPreset: registerPreset,
        mount: mount,
        effects: effectFactories,
        presets: presets,
        scenes: scenes
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init, { once: true });
    } else {
        init();
    }
})();
