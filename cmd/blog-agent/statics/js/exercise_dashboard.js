(() => {
    const state = { date: new Date().toISOString().slice(0, 10), items: [] };
    const datePicker = document.getElementById('datePicker');
    const toast = document.getElementById('toast');
    const typeLabel = { cardio:'有氧', strength:'力量', flexibility:'柔韧', sports:'运动项目', other:'其他' };
    let toastTimer;

    function message(text) { toast.textContent = text; toast.classList.add('show'); clearTimeout(toastTimer); toastTimer = setTimeout(() => toast.classList.remove('show'), 2600); }
    function formatDate(date) { return new Date(`${date}T12:00:00`).toLocaleDateString('zh-CN', { month:'long', day:'numeric', weekday:'long' }); }
    function escapeHTML(value) { const span = document.createElement('span'); span.textContent = value; return span.innerHTML; }
    function updateDate() { document.getElementById('dateLabel').textContent = formatDate(state.date); datePicker.value = state.date; }

    function renderToday() {
        const done = state.items.filter((item) => item.completed);
        const duration = done.reduce((sum, item) => sum + item.duration, 0);
        const calories = done.reduce((sum, item) => sum + (item.calories || 0), 0);
        const progress = state.items.length ? Math.round(done.length / state.items.length * 100) : 0;
        document.getElementById('todayDuration').textContent = duration;
        document.getElementById('todayCalories').textContent = calories;
        document.getElementById('todayCount').textContent = done.length;
        document.getElementById('todayTotal').textContent = `/ ${state.items.length} 项`;
        document.getElementById('completionValue').textContent = `${progress}%`;
        document.getElementById('completionRing').style.setProperty('--progress', `${progress}%`);
        const list = document.getElementById('exerciseItems');
        document.getElementById('exerciseEmpty').hidden = state.items.length !== 0;
        list.innerHTML = state.items.map((item) => `<div class="exercise-row ${item.completed ? 'completed' : ''}"><button class="complete-button" data-toggle="${item.id}" title="切换完成">✓</button><div class="exercise-copy"><strong>${escapeHTML(item.name)}</strong><span>${item.duration} 分钟 · ${typeLabel[item.type] || '运动'} · ${item.intensity === 'high' ? '高强度' : item.intensity === 'low' ? '低强度' : '中等强度'}</span></div><button class="delete-button" data-delete="${item.id}" title="删除">×</button></div>`).join('');
    }

    async function loadToday() {
        updateDate();
        try {
            const response = await fetch(`/api/exercises?date=${encodeURIComponent(state.date)}`);
            if (!response.ok) throw new Error('加载失败');
            state.items = (await response.json()).items || [];
            renderToday();
        } catch { message('今日锻炼加载失败'); }
    }

    function lastDays() { const date = new Date(`${state.date}T12:00:00`); return Array.from({ length:7 }, (_, index) => { const item = new Date(date); item.setDate(date.getDate() - 6 + index); return item.toISOString().slice(0, 10); }); }
    async function loadWeek() {
        const days = lastDays();
        try {
            const results = await Promise.all(days.map(async (date) => { const response = await fetch(`/api/exercises?date=${date}`); const data = response.ok ? await response.json() : { items:[] }; return { date, duration:(data.items || []).filter((item) => item.completed).reduce((sum, item) => sum + item.duration, 0), active:(data.items || []).some((item) => item.completed) }; }));
            const max = Math.max(1, ...results.map((item) => item.duration));
            document.getElementById('weekChart').innerHTML = results.map((item) => `<div class="day-bar ${item.date === state.date ? 'today' : ''}" title="${item.date}：${item.duration} 分钟"><i style="height:${Math.max(item.duration ? 10 : 3, item.duration / max * 100)}%"></i><span>${new Date(`${item.date}T12:00:00`).toLocaleDateString('zh-CN',{weekday:'short'}).replace('周','')}</span></div>`).join('');
            const activeDays = results.filter((item) => item.active).length;
            const total = results.reduce((sum, item) => sum + item.duration, 0);
            document.getElementById('weekDays').textContent = `${activeDays}/7`;
            document.getElementById('weekNote').textContent = activeDays ? `过去 7 天完成 ${total} 分钟运动，保持这个节奏。` : '过去 7 天还没有完成记录，今天从一项小运动开始。';
        } catch { document.getElementById('weekNote').textContent = '本周趋势暂时无法加载。'; }
    }

    async function refresh() { await Promise.all([loadToday(), loadWeek()]); }
    async function post(url, method, payload) { const response = await fetch(url, { method, headers:{'Content-Type':'application/json'}, body:JSON.stringify(payload) }); if (!response.ok) throw new Error(await response.text()); return response.json(); }
    document.getElementById('todayButton').addEventListener('click', () => { state.date = new Date().toISOString().slice(0,10); refresh(); });
    datePicker.addEventListener('change', () => { state.date = datePicker.value; refresh(); });
    document.getElementById('showAdd').addEventListener('click', () => { const form = document.getElementById('quickForm'); form.hidden = !form.hidden; if (!form.hidden) document.getElementById('exerciseName').focus(); });
    document.getElementById('quickForm').addEventListener('submit', async (event) => { const form = event.currentTarget; event.preventDefault(); try { await post('/api/exercises','POST',{ date:state.date, name:document.getElementById('exerciseName').value.trim(), duration:Number(document.getElementById('exerciseDuration').value), type:document.getElementById('exerciseType').value, intensity:document.getElementById('exerciseIntensity').value, calories:0, notes:'', weight:0, body_parts:[] }); form.reset(); form.hidden = true; message('已添加到今天'); refresh(); } catch (error) { message(`添加失败：${error.message}`); } });
    document.getElementById('exerciseItems').addEventListener('click', async (event) => { const toggle = event.target.dataset.toggle; const remove = event.target.dataset.delete; try { if (toggle) await post('/api/exercises/toggle','POST',{date:state.date,id:toggle}); if (remove && confirm('删除这项锻炼？')) await post('/api/exercises','DELETE',{date:state.date,id:remove}); if (toggle || remove) refresh(); } catch (error) { message(`操作失败：${error.message}`); } });
    refresh();
})();
