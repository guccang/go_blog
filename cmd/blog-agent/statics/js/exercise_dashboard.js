(() => {
    const today = new Date().toISOString().slice(0, 10);
    const state = {
        date: today,
        days: 7,
        items: [],
        duration: 30,
        preset: { name: '慢跑', type: 'cardio', intensity: 'medium' },
        custom: false,
        editingId: '',
    };
    const typeLabels = { cardio: '有氧', strength: '力量', flexibility: '柔韧', sports: '运动项目', other: '其他' };
    const typeIcons = { cardio: '🏃', strength: '🏋️', flexibility: '🧘', sports: '🚴', other: '●' };
    const toast = document.getElementById('toast');
    const datePicker = document.getElementById('datePicker');
    let toastTimer;

    function escapeHTML(value) {
        const element = document.createElement('span');
        element.textContent = value ?? '';
        return element.innerHTML;
    }

    function message(text) {
        toast.textContent = text;
        toast.classList.add('show');
        clearTimeout(toastTimer);
        toastTimer = setTimeout(() => toast.classList.remove('show'), 2400);
    }

    function formatDate(date) {
        return new Date(`${date}T12:00:00`).toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' });
    }

    async function request(url, options = {}) {
        const response = await fetch(url, options);
        if (!response.ok) {
            const detail = (await response.text()).trim();
            throw new Error(detail || `请求失败 (${response.status})`);
        }
        return response.json();
    }

    function jsonRequest(url, method, body) {
        return request(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
    }

    function updateDateLabels() {
        datePicker.value = state.date;
        document.getElementById('selectedDateLabel').textContent = formatDate(state.date);
    }

    function renderDay() {
        const list = document.getElementById('exerciseItems');
        const empty = document.getElementById('exerciseEmpty');
        empty.hidden = state.items.length !== 0;
        const completedMinutes = state.items.filter(item => item.completed).reduce((sum, item) => sum + (item.duration || 0), 0);
        document.getElementById('dayTotal').textContent = `${completedMinutes} 分钟`;
        list.innerHTML = state.items.map(item => {
            if (state.editingId === item.id) {
                return `<div class="exercise-row">
                    <div class="edit-row" data-edit-row="${item.id}">
                        <input data-field="name" maxlength="30" value="${escapeHTML(item.name)}" aria-label="运动名称">
                        <select data-field="type" aria-label="运动类型">${Object.entries(typeLabels).map(([value, label]) => `<option value="${value}" ${item.type === value ? 'selected' : ''}>${label}</option>`).join('')}</select>
                        <input data-field="duration" type="number" min="1" max="1440" value="${item.duration}" aria-label="运动分钟">
                        <button type="button" data-save="${item.id}">保存</button>
                    </div>
                </div>`;
            }
            return `<div class="exercise-row">
                <span class="exercise-icon">${typeIcons[item.type] || '●'}</span>
                <div class="exercise-copy">
                    <strong>${escapeHTML(item.name)}</strong>
                    <span>${item.duration} 分钟 · ${typeLabels[item.type] || '运动'}${item.completed ? '' : ' · <em class="pending-mark">未完成</em>'}</span>
                </div>
                <div class="row-actions">
                    ${item.completed ? '' : `<button type="button" data-complete="${item.id}">完成</button>`}
                    <button type="button" data-edit="${item.id}" aria-label="编辑 ${escapeHTML(item.name)}">编辑</button>
                    <button type="button" data-delete="${item.id}" aria-label="删除 ${escapeHTML(item.name)}">删除</button>
                </div>
            </div>`;
        }).join('');
    }

    function chartLabel(item, index, total) {
        const date = new Date(`${item.date}T12:00:00`);
        if (total === 7) return date.toLocaleDateString('zh-CN', { weekday: 'short' }).replace('周', '');
        if (index === 0 || index === total - 1 || index % 5 === 0) return `${date.getMonth() + 1}/${date.getDate()}`;
        return '';
    }

    function renderOverview(data) {
        document.getElementById('exerciseDays').textContent = data.exercise_days || 0;
        document.getElementById('totalDuration').textContent = data.total_duration || 0;
        document.getElementById('currentStreak').textContent = data.current_streak || 0;
        const days = data.days || [];
        const max = Math.max(1, ...days.map(item => item.duration || 0));
        document.getElementById('pulseChart').innerHTML = days.map((item, index) => {
            const height = item.duration ? Math.max(9, item.duration / max * 100) : 3;
            const classes = [item.duration ? 'active' : '', item.date === state.date ? 'today' : ''].filter(Boolean).join(' ');
            return `<div class="pulse-day ${classes}" title="${item.date}：${item.duration} 分钟">
                <i style="height:${height}%"></i><span>${chartLabel(item, index, days.length)}</span>
            </div>`;
        }).join('');
        const note = data.exercise_days
            ? `这 ${state.days} 天完成 ${data.total_duration} 分钟运动，已有 ${data.exercise_days} 天留下记录。`
            : `这 ${state.days} 天还没有完成记录，从十分钟开始就很好。`;
        document.getElementById('rhythmNote').textContent = note;
    }

    async function loadDay() {
        updateDateLabels();
        try {
            const data = await request(`/api/exercises?date=${encodeURIComponent(state.date)}`);
            state.items = data.items || [];
            state.editingId = '';
            renderDay();
        } catch (error) {
            message(`当天记录加载失败：${error.message}`);
        }
    }

    async function loadOverview() {
        try {
            const data = await request(`/api/exercise-overview?end_date=${encodeURIComponent(state.date)}&days=${state.days}`);
            renderOverview(data);
        } catch (error) {
            message(`统计加载失败：${error.message}`);
        }
    }

    async function refresh() {
        await Promise.all([loadDay(), loadOverview()]);
    }

    function selectPreset(button) {
        document.querySelectorAll('#presetGrid button').forEach(item => item.classList.toggle('active', item === button));
        state.custom = button.dataset.custom === 'true';
        document.getElementById('customFields').hidden = !state.custom;
        if (state.custom) {
            document.getElementById('customName').focus();
            return;
        }
        state.preset = {
            name: button.dataset.name,
            type: button.dataset.type,
            intensity: button.dataset.intensity,
        };
    }

    document.getElementById('presetGrid').addEventListener('click', event => {
        const button = event.target.closest('button');
        if (button) selectPreset(button);
    });

    document.getElementById('durationChoices').addEventListener('click', event => {
        const button = event.target.closest('button');
        if (!button) return;
        state.duration = Number(button.dataset.duration);
        document.querySelectorAll('#durationChoices button').forEach(item => item.classList.toggle('active', item === button));
    });

    document.querySelector('.range-switch').addEventListener('click', event => {
        const button = event.target.closest('button[data-days]');
        if (!button || Number(button.dataset.days) === state.days) return;
        state.days = Number(button.dataset.days);
        document.querySelectorAll('.range-switch button').forEach(item => item.classList.toggle('active', item === button));
        loadOverview();
    });

    document.getElementById('recordButton').addEventListener('click', async event => {
        const button = event.currentTarget;
        const customName = document.getElementById('customName').value.trim();
        const payload = state.custom
            ? { name: customName, type: document.getElementById('customType').value, intensity: 'medium' }
            : state.preset;
        if (!payload.name) {
            message('请填写运动名称');
            document.getElementById('customName').focus();
            return;
        }
        button.disabled = true;
        button.textContent = '记录中…';
        try {
            const item = await jsonRequest('/api/exercises', 'POST', {
                date: state.date,
                name: payload.name,
                type: payload.type,
                duration: state.duration,
                intensity: payload.intensity,
                calories: 0,
                notes: '',
                weight: 0,
                body_parts: [],
                completed: true,
            });
            state.items.push(item);
            renderDay();
            await loadOverview();
            if (state.custom) document.getElementById('customName').value = '';
            message('锻炼已记录');
        } catch (error) {
            message(`记录失败：${error.message}`);
        } finally {
            button.disabled = false;
            button.textContent = '记录完成';
        }
    });

    document.getElementById('exerciseItems').addEventListener('click', async event => {
        const editId = event.target.dataset.edit;
        const saveId = event.target.dataset.save;
        const completeId = event.target.dataset.complete;
        const deleteId = event.target.dataset.delete;
        if (editId) {
            state.editingId = editId;
            renderDay();
            return;
        }
        try {
            if (saveId) {
                const item = state.items.find(value => value.id === saveId);
                const row = document.querySelector(`[data-edit-row="${CSS.escape(saveId)}"]`);
                const name = row.querySelector('[data-field="name"]').value.trim();
                const duration = Number(row.querySelector('[data-field="duration"]').value);
                if (!name || duration < 1) throw new Error('名称和时长不能为空');
                await jsonRequest('/api/exercises', 'PUT', {
                    date: state.date,
                    id: item.id,
                    name,
                    type: row.querySelector('[data-field="type"]').value,
                    duration,
                    intensity: item.intensity || 'medium',
                    calories: item.calories || 0,
                    notes: item.notes || '',
                    weight: item.weight || 0,
                    body_parts: item.body_parts || [],
                });
                item.name = name;
                item.duration = duration;
                item.type = row.querySelector('[data-field="type"]').value;
                state.editingId = '';
                renderDay();
                await loadOverview();
                message('记录已更新');
            }
            if (completeId) {
                await jsonRequest('/api/exercises/toggle', 'POST', { date: state.date, id: completeId });
                const item = state.items.find(value => value.id === completeId);
                if (item) item.completed = true;
                renderDay();
                await loadOverview();
                message('已计入完成');
            }
            if (deleteId && window.confirm('删除这条锻炼记录？')) {
                await jsonRequest('/api/exercises', 'DELETE', { date: state.date, id: deleteId });
                state.items = state.items.filter(value => value.id !== deleteId);
                renderDay();
                await loadOverview();
                message('记录已删除');
            }
        } catch (error) {
            message(`操作失败：${error.message}`);
        }
    });

    datePicker.max = today;
    datePicker.addEventListener('change', () => {
        if (!datePicker.value) return;
        state.date = datePicker.value;
        refresh();
    });
    document.getElementById('todayButton').addEventListener('click', () => {
        state.date = today;
        refresh();
    });

    refresh();
})();
