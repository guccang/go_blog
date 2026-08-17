(() => {
    const state = {
        catalog: null,
        profile: null,
        selectedMovement: 'pushup',
        plan: null,
    };
    const app = document.getElementById('professionalApp');
    const loading = document.getElementById('loadingState');
    const toast = document.getElementById('toast');
    let toastTimer;
    let saveTimer;

    class APIError extends Error {
        constructor(message, status, code) {
            super(message);
            this.status = status;
            this.code = code || '';
        }
    }

    function localDate(date = new Date()) {
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        return `${year}-${month}-${day}`;
    }

    function escapeHTML(value) {
        const element = document.createElement('span');
        element.textContent = value ?? '';
        return element.innerHTML;
    }

    function message(text) {
        toast.textContent = text;
        toast.classList.add('show');
        clearTimeout(toastTimer);
        toastTimer = setTimeout(() => toast.classList.remove('show'), 2600);
    }

    async function request(url, options = {}) {
        const response = await fetch(url, options);
        const type = response.headers.get('content-type') || '';
        const body = type.includes('application/json') ? await response.json() : { error: await response.text() };
        if (!response.ok) {
            throw new APIError(body.error || `请求失败 (${response.status})`, response.status, body.code);
        }
        return body;
    }

    function jsonRequest(url, method, body) {
        return request(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
    }

    function currentMovement() {
        return state.catalog.movements.find(item => item.id === state.selectedMovement);
    }

    function currentLevel(movement = currentMovement()) {
        return movement.levels[state.profile.levels[movement.id] - 1];
    }

    function targetText(item) {
        if (item.hold_seconds) return `${item.sets} 组 × ${item.hold_seconds} 秒`;
        return `${item.sets} 组 × ${item.reps} 次`;
    }

    function renderMovementNav() {
        document.getElementById('movementNav').innerHTML = state.catalog.movements.map(movement => `
            <button type="button" data-movement="${movement.id}" class="${movement.id === state.selectedMovement ? 'active' : ''}" aria-pressed="${movement.id === state.selectedMovement}">
                <span>${escapeHTML(movement.icon)}</span>
                <strong>${escapeHTML(movement.name)}</strong>
            </button>
        `).join('');
    }

    function renderMovement() {
        const movement = currentMovement();
        const level = currentLevel(movement);
        document.getElementById('movementIcon').textContent = movement.icon;
        document.getElementById('movementTarget').textContent = movement.target;
        document.getElementById('movementTitle').textContent = movement.name;
        document.getElementById('movementSummary').textContent = movement.summary;
        document.getElementById('movementEquipment').textContent = `器材：${movement.equipment}`;
        document.getElementById('currentLevelLabel').textContent = `第 ${level.level} 级 · ${level.name}`;
        document.getElementById('levelTrack').innerHTML = movement.levels.map(item => {
            const classes = [item.level <= level.level ? 'reached' : '', item.level === level.level ? 'active' : ''].filter(Boolean).join(' ');
            return `<button type="button" class="${classes}" data-level="${item.level}" aria-label="第 ${item.level} 级 ${escapeHTML(item.name)}" aria-pressed="${item.level === level.level}">
                <i>${item.level}</i><small>${escapeHTML(item.name)}</small>
            </button>`;
        }).join('');
        document.getElementById('levelDifficulty').textContent = `${level.difficulty} · 第 ${level.level} 级`;
        document.getElementById('levelName').textContent = level.name;
        document.getElementById('prescription').textContent = targetText(level);
        document.getElementById('levelTempo').textContent = level.tempo;
        document.getElementById('levelRest').textContent = `${level.rest_seconds} 秒`;
        document.getElementById('levelEquipment').textContent = level.equipment;
        document.getElementById('levelCues').innerHTML = level.cues.map(item => `<li>${escapeHTML(item)}</li>`).join('');
        document.getElementById('levelMistakes').innerHTML = level.mistakes.map(item => `<li>${escapeHTML(item)}</li>`).join('');
        document.getElementById('levelAdvance').textContent = level.advance;
    }

    function renderFrequency() {
        document.querySelectorAll('#frequencySwitch button').forEach(button => {
            const active = Number(button.dataset.days) === state.profile.days_per_week;
            button.classList.toggle('active', active);
            button.setAttribute('aria-pressed', String(active));
        });
        const hints = {
            2: '第 1、4 天训练；每次三类动作，恢复时间最多。',
            3: '第 1、3、5 天训练；每天两类动作，推荐起点。',
            4: '第 1、2、4、6 天训练；单次更短，避免连续刺激同一动作。',
        };
        document.getElementById('frequencyHint').textContent = hints[state.profile.days_per_week];
    }

    function renderPlan() {
        const preview = document.getElementById('planPreview');
        const applyButton = document.getElementById('applyButton');
        document.getElementById('conflictBox').hidden = true;
        if (!state.plan) {
            preview.hidden = true;
            applyButton.hidden = true;
            preview.innerHTML = '';
            return;
        }
        preview.innerHTML = state.plan.sessions.map(session => `
            <section class="plan-session">
                <time datetime="${session.date}">第 ${session.day} 天 · ${escapeHTML(formatDate(session.date))}</time>
                <ul>${session.items.map(item => `
                    <li><strong>${escapeHTML(item.name)}</strong><span>${escapeHTML(targetText(item))}</span></li>
                `).join('')}</ul>
            </section>
        `).join('');
        preview.hidden = false;
        applyButton.hidden = false;
        applyButton.disabled = false;
        applyButton.textContent = '加入训练';
    }

    function formatDate(value) {
        return new Date(`${value}T12:00:00`).toLocaleDateString('zh-CN', {
            month: 'long',
            day: 'numeric',
            weekday: 'short',
        });
    }

    function clearPlan() {
        state.plan = null;
        renderPlan();
    }

    function profilePayload() {
        return {
            version: state.catalog.version,
            levels: state.profile.levels,
            days_per_week: state.profile.days_per_week,
            start_date: document.getElementById('startDate').value,
        };
    }

    function scheduleProfileSave() {
        clearTimeout(saveTimer);
        saveTimer = setTimeout(async () => {
            try {
                state.profile = await jsonRequest('/api/exercise/pro/profile', 'PUT', profilePayload());
            } catch (error) {
                message(`训练设置保存失败：${error.message}`);
            }
        }, 350);
    }

    function planPayload(replace = false) {
        return {
            start_date: document.getElementById('startDate').value,
            days_per_week: state.profile.days_per_week,
            levels: state.profile.levels,
            replace,
        };
    }

    async function previewPlan() {
        const button = document.getElementById('previewButton');
        button.disabled = true;
        button.textContent = '正在安排…';
        try {
            state.plan = await jsonRequest('/api/exercise/pro/plan/preview', 'POST', planPayload());
            renderPlan();
            document.getElementById('planPreview').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        } catch (error) {
            message(`计划预览失败：${error.message}`);
        } finally {
            button.disabled = false;
            button.textContent = '预览一周计划';
        }
    }

    async function applyPlan(replace) {
        const button = replace ? document.getElementById('replaceButton') : document.getElementById('applyButton');
        button.disabled = true;
        const original = button.textContent;
        let applied = false;
        button.textContent = replace ? '正在替换…' : '正在加入…';
        try {
            const result = await jsonRequest('/api/exercise/pro/plan/apply', 'POST', planPayload(replace));
            applied = true;
            document.getElementById('conflictBox').hidden = true;
            document.getElementById('applyButton').textContent = '已加入训练';
            document.getElementById('applyButton').disabled = true;
            message(`计划已加入：新增 ${result.created} 项，保留 ${result.preserved} 项`);
        } catch (error) {
            if (error.status === 409 && error.code === 'professional_plan_conflict') {
                document.getElementById('conflictBox').hidden = false;
                message('这七天已有专业计划，请确认是否替换');
            } else {
                message(`加入计划失败：${error.message}`);
            }
        } finally {
            if (!applied || button.id === 'replaceButton') {
                button.disabled = false;
                button.textContent = original;
            }
        }
    }

    function bindEvents() {
        document.getElementById('movementNav').addEventListener('click', event => {
            const button = event.target.closest('button[data-movement]');
            if (!button || button.dataset.movement === state.selectedMovement) return;
            state.selectedMovement = button.dataset.movement;
            renderMovementNav();
            renderMovement();
        });
        document.getElementById('levelTrack').addEventListener('click', event => {
            const button = event.target.closest('button[data-level]');
            if (!button) return;
            const level = Number(button.dataset.level);
            if (state.profile.levels[state.selectedMovement] === level) return;
            state.profile.levels[state.selectedMovement] = level;
            renderMovement();
            clearPlan();
            scheduleProfileSave();
        });
        document.getElementById('frequencySwitch').addEventListener('click', event => {
            const button = event.target.closest('button[data-days]');
            if (!button) return;
            const days = Number(button.dataset.days);
            if (days === state.profile.days_per_week) return;
            state.profile.days_per_week = days;
            renderFrequency();
            clearPlan();
            scheduleProfileSave();
        });
        document.getElementById('startDate').addEventListener('change', () => {
            clearPlan();
            scheduleProfileSave();
        });
        document.getElementById('previewButton').addEventListener('click', previewPlan);
        document.getElementById('applyButton').addEventListener('click', () => applyPlan(false));
        document.getElementById('replaceButton').addEventListener('click', () => applyPlan(true));
    }

    async function init() {
        try {
            const [catalog, profile] = await Promise.all([
                request('/api/exercise/pro/catalog'),
                request('/api/exercise/pro/profile'),
            ]);
            state.catalog = catalog;
            state.profile = profile;
            state.catalog.movements.forEach(movement => {
                if (!state.profile.levels[movement.id]) state.profile.levels[movement.id] = 1;
            });
            const startDate = document.getElementById('startDate');
            startDate.value = state.profile.start_date || localDate();
            document.getElementById('catalogNotice').textContent = catalog.notice;
            renderMovementNav();
            renderMovement();
            renderFrequency();
            bindEvents();
            loading.hidden = true;
            app.hidden = false;
        } catch (error) {
            loading.textContent = `专业训练加载失败：${error.message}`;
        }
    }

    init();
})();
