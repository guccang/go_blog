// goal.js - 统一目标管理前端
const STATE = {
    level: 'daily',
    goal: null,
};

function init() {
    document.querySelectorAll('.goal-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.goal-tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            STATE.level = tab.dataset.level;
            loadGoal();
        });
    });
    loadGoal();
}

async function loadGoal() {
    const period = getCurrentPeriod(STATE.level);
    try {
        const resp = await fetch(`/api/goal?level=${STATE.level}&period=${period}`);
        const data = await resp.json();
        if (data.success) {
            STATE.goal = data.data;
        } else {
            STATE.goal = null;
        }
    } catch (e) {
        STATE.goal = null;
    }
    render();
}

function getCurrentPeriod(level) {
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');

    switch (level) {
        case 'daily':
            return `${year}-${month}-${day}`;
        case 'weekly':
            return getISOWeek(now);
        case 'monthly':
            return `${year}-${month}`;
        case 'yearly':
            return `${year}`;
    }
}

function getISOWeek(d) {
    const date = new Date(d);
    date.setHours(0, 0, 0, 0);
    date.setDate(date.getDate() + 3 - (date.getDay() + 6) % 7);
    const week1 = new Date(date.getFullYear(), 0, 4);
    const weekNum = 1 + Math.round(((date - week1) / 86400000 - 3 + (week1.getDay() + 6) % 7) / 7);
    return `${date.getFullYear()}-W${String(weekNum).padStart(2, '0')}`;
}

function render() {
    const el = document.getElementById('goalContent');
    if (!el) return;

    const goal = STATE.goal;
    const levelLabels = { daily: '今日', weekly: '本周', monthly: '本月', yearly: '本年' };
    const label = levelLabels[STATE.level] || '';

    let done = 0, total = 0;
    if (goal && goal.tasks) {
        total = goal.tasks.length;
        done = goal.tasks.filter(t => t.status === 'completed').length;
    }
    const progress = total > 0 ? Math.round(done * 100 / total) : 0;

    el.innerHTML = `
        <div class="goal-card">
            <div class="goal-meta">
                <span class="period-label">${label}目标 · ${getCurrentPeriod(STATE.level)}</span>
                ${goal ? `<span class="goal-status ${goal.status || 'active'}">${goal.status === 'completed' ? '已完成' : '进行中'}</span>` : ''}
                <div style="flex:1"></div>
                <button class="btn btn-sm btn-success" onclick="toggleStatus()">
                    ${(goal && goal.status === 'completed') ? '重新开始' : '标记完成'}
                </button>
            </div>
            <textarea class="goal-overview" id="overviewInput" placeholder="写写${label}的目标...">${goal ? (goal.overview || '') : ''}</textarea>
            <div style="display:flex; gap:8px; margin-bottom:16px;">
                <button class="btn btn-primary" onclick="saveOverview()">保存目标</button>
                <span style="font-size:12px;color:#666;line-height:32px;">
                    ${goal && goal.updated_at ? '上次更新: ' + goal.updated_at : ''}
                </span>
            </div>
            <div class="goal-progress">
                <div style="display:flex; justify-content:space-between;">
                    <span style="font-size:13px; color:#888;">任务进度</span>
                    <span class="progress-text">${done}/${total} 完成 (${progress}%)</span>
                </div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width:${progress}%"></div>
                </div>
            </div>
        </div>

        <div class="goal-card">
            <h4 style="margin:0 0 12px 0; color:#ccc;">任务列表</h4>
            <ul class="task-list" id="taskList">
                ${goal && goal.tasks && goal.tasks.length > 0
                    ? goal.tasks.map(t => renderTask(t)).join('')
                    : `<div class="empty-state">还没有任务，添加一个吧</div>`
                }
            </ul>
            <div class="add-task-row">
                <input class="add-task-input" id="newTaskTitle" placeholder="新任务标题..." />
                <select class="priority-select" id="newTaskPriority">
                    <option value="medium">普通</option>
                    <option value="high">重要</option>
                    <option value="low">低优</option>
                </select>
                <button class="btn btn-primary" onclick="addTask()">添加</button>
            </div>
        </div>
    `;
}

function renderTask(task) {
    const done = task.status === 'completed';
    return `
        <li class="task-item" data-id="${task.id}">
            <div class="task-checkbox ${done ? 'done' : ''}" onclick="toggleTask('${task.id}')">
                ${done ? '✓' : ''}
            </div>
            <span class="task-title ${done ? 'done' : ''}">${escapeHtml(task.title)}</span>
            <span class="task-priority ${task.priority}">${priorityLabel(task.priority)}</span>
            <div class="task-actions">
                <button class="btn btn-sm btn-danger" onclick="deleteTask('${task.id}')">删除</button>
            </div>
        </li>
    `;
}

function priorityLabel(p) {
    const m = { high: '重要', medium: '普通', low: '低优' };
    return m[p] || p;
}

function escapeHtml(s) {
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}

async function saveOverview() {
    const overview = document.getElementById('overviewInput').value;
    const period = getCurrentPeriod(STATE.level);
    await fetch('/api/goal/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period, overview }),
    });
    loadGoal();
}

async function toggleStatus() {
    if (!STATE.goal) return;
    const period = getCurrentPeriod(STATE.level);
    const newStatus = STATE.goal.status === 'completed' ? 'active' : 'completed';
    await fetch('/api/goal/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period, status: newStatus }),
    });
    loadGoal();
}

async function toggleTask(taskId) {
    const task = STATE.goal.tasks.find(t => t.id === taskId);
    if (!task) return;
    const newStatus = task.status === 'completed' ? 'pending' : 'completed';
    const period = getCurrentPeriod(STATE.level);
    await fetch('/api/goal/task/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period, task_id: taskId, task: { ...task, status: newStatus } }),
    });
    loadGoal();
}

async function deleteTask(taskId) {
    const period = getCurrentPeriod(STATE.level);
    await fetch('/api/goal/task/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period, task_id: taskId }),
    });
    loadGoal();
}

async function addTask() {
    const titleEl = document.getElementById('newTaskTitle');
    const priorityEl = document.getElementById('newTaskPriority');
    const title = titleEl.value.trim();
    if (!title) return;

    const period = getCurrentPeriod(STATE.level);
    await fetch('/api/goal/task', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period, task: { title, priority: priorityEl.value } }),
    });
    titleEl.value = '';
    loadGoal();
}

document.addEventListener('DOMContentLoaded', init);
