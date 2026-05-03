// goal.js - 统一目标管理前端
const LEVEL_LABELS = { daily: '日', weekly: '周', monthly: '月', yearly: '年' };
const PRIORITY_MAP = { high: '重要', medium: '普通', low: '低优' };
const STATUS_MAP = { pending: '待办', in_progress: '进行中', completed: '已完成', cancelled: '已取消' };

const STATE = {
    level: 'daily',
    period: '',
    nav: null,       // { prev, current, next }
    goal: null,      // current goal object
    goals: [],       // list of goals for list view
    view: 'detail',  // 'detail' | 'list'
    editTaskId: null,// currently editing task ID (inline)
};

// ============================================================================
// Initialization
// ============================================================================
function init() {
    // Tabs
    document.querySelectorAll('.goal-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.goal-tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            STATE.level = tab.dataset.level;
            STATE.period = '';
            loadGoal();
            updateTodayLabel();
        });
    });

    // Period navigation
    document.getElementById('prevBtn').addEventListener('click', () => {
        if (STATE.nav && STATE.nav.prev) {
            STATE.period = STATE.nav.prev;
            loadGoal();
        }
    });
    document.getElementById('nextBtn').addEventListener('click', () => {
        if (STATE.nav && STATE.nav.next) {
            STATE.period = STATE.nav.next;
            loadGoal();
        }
    });
    document.getElementById('todayBtn').addEventListener('click', goToCurrentPeriod);

    // View toggle
    document.querySelectorAll('.view-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.view-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            STATE.view = btn.dataset.view;
            if (STATE.view === 'detail') {
                loadGoal();
            } else {
                loadGoalList();
            }
        });
    });

    // Close modals on overlay click
    document.getElementById('taskModal').addEventListener('click', function(e) {
        if (e.target === this) closeTaskModal();
    });
    document.getElementById('confirmModal').addEventListener('click', function(e) {
        if (e.target === this) closeConfirmModal();
    });

    updateTodayLabel();
    loadGoal();
}

function updateTodayLabel() {
    const btn = document.getElementById('todayBtn');
    if (!btn) return;
    const m = { daily: '今天', weekly: '本周', monthly: '本月', yearly: '本年' };
    btn.textContent = m[STATE.level] || '今天';
}

// ============================================================================
// Period Utilities
// ============================================================================
function getCurrentPeriod(level) {
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');

    switch (level) {
        case 'daily':   return `${year}-${month}-${day}`;
        case 'weekly':  return getISOWeek(now);
        case 'monthly': return `${year}-${month}`;
        case 'yearly':  return `${year}`;
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

function formatPeriodLabel(level, period) {
    if (!period) return '';
    switch (level) {
        case 'daily': {
            const t = parseDate(period);
            if (!t) return period;
            const days = ['周日','周一','周二','周三','周四','周五','周六'];
            return `${t.getFullYear()}年${t.getMonth()+1}月${t.getDate()}日 ${days[t.getDay()]}`;
        }
        case 'weekly': {
            const m = period.match(/^(\d{4})-W(\d{2})$/);
            return m ? `${m[1]}年第${parseInt(m[2])}周` : period;
        }
        case 'monthly': {
            const m = period.match(/^(\d{4})-(\d{2})$/);
            return m ? `${m[1]}年${parseInt(m[2])}月` : period;
        }
        case 'yearly':
            return `${period}年`;
        default:
            return period;
    }
}

function parseDate(s) {
    const parts = s.split('-');
    if (parts.length !== 3) return null;
    return new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]));
}

function goToCurrentPeriod() {
    STATE.period = getCurrentPeriod(STATE.level);
    loadGoal();
}

// ============================================================================
// Data Loading
// ============================================================================
async function loadGoal() {
    if (!STATE.period) {
        STATE.period = getCurrentPeriod(STATE.level);
    }

    try {
        const resp = await fetch(`/api/goal?level=${STATE.level}&period=${STATE.period}&nav=true`);
        const data = await resp.json();
        if (data.success) {
            STATE.goal = data.data;
            if (data.nav) {
                STATE.nav = data.nav;
                document.getElementById('periodLabel').textContent = formatPeriodLabel(STATE.level, STATE.period);
            }
        } else {
            STATE.goal = null;
        }
    } catch (e) {
        STATE.goal = null;
    }
    renderDetail();
}

async function loadGoalList() {
    try {
        const resp = await fetch(`/api/goals?level=${STATE.level}`);
        const data = await resp.json();
        if (data.success) {
            STATE.goals = data.data || [];
        } else {
            STATE.goals = [];
        }
    } catch (e) {
        STATE.goals = [];
    }
    renderList();
}

function render() {
    if (STATE.view === 'detail') {
        renderDetail();
    } else {
        loadGoalList();
    }
}

// ============================================================================
// Detail View Rendering
// ============================================================================
function renderDetail() {
    const el = document.getElementById('mainContent');
    if (!el) return;

    const goal = STATE.goal;
    const label = LEVEL_LABELS[STATE.level] || '';
    const periodLabel = formatPeriodLabel(STATE.level, STATE.period);

    let done = 0, total = 0;
    if (goal && goal.tasks) {
        total = goal.tasks.length;
        done = goal.tasks.filter(t => t.status === 'completed').length;
    }
    const progress = total > 0 ? Math.round(done * 100 / total) : 0;

    const statusLabel = goal && goal.status === 'completed' ? '已完成' : '进行中';

    let updatedHtml = '';
    if (goal && goal.updated_at) {
        updatedHtml = `<span class="update-hint">上次更新: ${goal.updated_at}</span>`;
    }

    el.innerHTML = `
        <div class="goal-card">
            <div class="goal-card-header">
                <span class="card-period">${label}目标 · ${periodLabel}</span>
                <div style="display:flex; gap:8px; align-items:center;">
                    <span class="goal-status ${goal ? (goal.status || 'active') : 'active'}">${statusLabel}</span>
                    <button class="btn btn-sm btn-outline" onclick="toggleStatus()">
                        ${(goal && goal.status === 'completed') ? '重新开始' : '标记完成'}
                    </button>
                </div>
            </div>

            <textarea class="goal-overview" id="overviewInput" placeholder="写写${label}目标的内容...">${goal ? esc(goal.overview || '') : ''}</textarea>
            <div class="goal-actions">
                <button class="btn btn-primary btn-sm" onclick="saveOverview()">保存目标</button>
                ${updatedHtml}
            </div>

            <div class="goal-progress">
                <div class="progress-header">
                    <span>任务进度</span>
                    <span>${done}/${total} 完成 (${progress}%)</span>
                </div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width:${progress}%"></div>
                </div>
            </div>

            <div class="delete-section">
                <button class="btn btn-sm btn-danger" onclick="deleteGoal()">删除此目标</button>
            </div>
        </div>

        <div class="goal-card">
            <h4 class="task-section-title">任务列表</h4>
            <ul class="task-list" id="taskList">
                ${goal && goal.tasks && goal.tasks.length > 0
                    ? goal.tasks.map(t => renderTask(t)).join('')
                    : '<div class="empty-state">还没有任务，添加一个吧</div>'
                }
            </ul>
            <div class="add-task-row">
                <input class="add-task-input" id="newTaskTitle" placeholder="新任务标题..." onkeydown="if(event.key==='Enter')addTask()" />
                <select class="priority-select" id="newTaskPriority">
                    <option value="medium">普通</option>
                    <option value="high">重要</option>
                    <option value="low">低优</option>
                </select>
                <button class="btn btn-primary btn-sm" onclick="addTask()">添加</button>
            </div>
        </div>
    `;
}

function renderTask(task) {
    const done = task.status === 'completed';
    const isEditing = STATE.editTaskId === task.id;
    const statusTag = (task.status === 'in_progress' || task.status === 'cancelled')
        ? `<span class="task-status-tag ${task.status}">${STATUS_MAP[task.status]}</span>`
        : '';

    return `
        <li class="task-item" data-id="${task.id}">
            <div class="task-checkbox ${done ? 'done' : ''}" onclick="toggleTask('${task.id}')">
                ${done ? '\u2713' : ''}
            </div>
            ${isEditing
                ? `<input class="task-title-edit" id="editInput_${task.id}" value="${esc(task.title)}"
                    onblur="saveEditTitle('${task.id}')"
                    onkeydown="if(event.key==='Enter')saveEditTitle('${task.id}');if(event.key==='Escape')cancelEditTitle('${task.id}')" />`
                : `<span class="task-title-inner ${done ? 'done' : ''}" onclick="startEditTitle('${task.id}')" title="点击编辑">${esc(task.title)}</span>`
            }
            <div class="task-meta">
                ${statusTag}
                <select class="inline-select task-priority ${task.priority}" onchange="changePriority('${task.id}', this.value)" title="优先级">
                    <option value="high" ${task.priority === 'high' ? 'selected' : ''}>重要</option>
                    <option value="medium" ${task.priority === 'medium' ? 'selected' : ''}>普通</option>
                    <option value="low" ${task.priority === 'low' ? 'selected' : ''}>低优</option>
                </select>
                <button class="btn btn-xs btn-secondary" onclick="openTaskModal('${task.id}')" title="更多设置">编辑</button>
                <button class="btn btn-xs btn-danger" onclick="deleteTask('${task.id}')">删除</button>
            </div>
        </li>
    `;
}

// ============================================================================
// List View Rendering
// ============================================================================
function renderList() {
    const el = document.getElementById('mainContent');
    if (!el) return;

    if (!STATE.goals || STATE.goals.length === 0) {
        el.innerHTML = '<div class="goal-card"><div class="empty-state">暂无已保存的目标</div></div>';
        return;
    }

    const cards = STATE.goals.map(g => {
        const done = g.done_tasks || 0;
        const total = g.total_tasks || 0;
        const progress = total > 0 ? Math.round(done * 100 / total) : (g.progress || 0);
        const overview = (g.overview || '').slice(0, 60) || '无概述';

        return `
            <div class="list-card" onclick="STATE.period='${g.period}'; STATE.view='detail'; loadGoal(); document.querySelectorAll('.view-btn').forEach(b=>{b.classList.toggle('active', b.dataset.view==='detail')})">
                <span class="card-period">${formatPeriodLabel(STATE.level, g.period)}</span>
                <div class="card-overview">${esc(overview)}</div>
                <div class="goal-progress" style="margin-bottom:4px;">
                    <div class="progress-bar">
                        <div class="progress-fill" style="width:${progress}%"></div>
                    </div>
                </div>
                <div class="card-stats">
                    <span>${done}/${total} 完成 (${progress}%)</span>
                    <span class="goal-status ${g.status || 'active'}">${g.status === 'completed' ? '已完成' : '进行中'}</span>
                </div>
            </div>
        `;
    }).join('');

    const label = LEVEL_LABELS[STATE.level] || '';
    el.innerHTML = `
        <div style="font-size:13px; color:#888; margin-bottom:10px;">${label}目标 · 共 ${STATE.goals.length} 条</div>
        <div class="list-grid">${cards}</div>
    `;
}

// ============================================================================
// Goal Actions
// ============================================================================
async function saveOverview() {
    const overview = document.getElementById('overviewInput').value;
    await fetch('/api/goal/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, overview }),
    });
    loadGoal();
}

async function toggleStatus() {
    if (!STATE.goal) return;
    const newStatus = STATE.goal.status === 'completed' ? 'active' : 'completed';
    await fetch('/api/goal/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, status: newStatus }),
    });
    loadGoal();
}

// ============================================================================
// Task Actions
// ============================================================================
async function toggleTask(taskId) {
    if (!STATE.goal) return;
    const task = STATE.goal.tasks.find(t => t.id === taskId);
    if (!task) return;
    const newStatus = task.status === 'completed' ? 'pending' : 'completed';
    await fetch('/api/goal/task/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, task_id: taskId, task: { ...task, status: newStatus } }),
    });
    loadGoal();
}

async function addTask() {
    const titleEl = document.getElementById('newTaskTitle');
    const priorityEl = document.getElementById('newTaskPriority');
    const title = titleEl.value.trim();
    if (!title) return;

    await fetch('/api/goal/task', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, task: { title, priority: priorityEl.value } }),
    });
    titleEl.value = '';
    loadGoal();
}

async function deleteTask(taskId) {
    if (!confirm('确定删除此任务？')) return;
    await fetch('/api/goal/task/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, task_id: taskId }),
    });
    loadGoal();
}

// Inline title editing
function startEditTitle(taskId) {
    STATE.editTaskId = taskId;
    renderDetail();
    setTimeout(() => {
        const input = document.getElementById('editInput_' + taskId);
        if (input) { input.focus(); input.select(); }
    }, 50);
}

async function saveEditTitle(taskId) {
    const input = document.getElementById('editInput_' + taskId);
    if (!input) { STATE.editTaskId = null; renderDetail(); return; }
    const newTitle = input.value.trim();
    STATE.editTaskId = null;
    if (!newTitle) { renderDetail(); return; }

    const task = STATE.goal && STATE.goal.tasks ? STATE.goal.tasks.find(t => t.id === taskId) : null;
    if (!task) { renderDetail(); return; }
    if (task.title === newTitle) { renderDetail(); return; }

    await fetch('/api/goal/task/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, task_id: taskId, task: { ...task, title: newTitle } }),
    });
    loadGoal();
}

function cancelEditTitle(taskId) {
    STATE.editTaskId = null;
    renderDetail();
}

// Priority inline change
async function changePriority(taskId, newPriority) {
    if (!STATE.goal) return;
    const task = STATE.goal.tasks.find(t => t.id === taskId);
    if (!task || task.priority === newPriority) return;
    await fetch('/api/goal/task/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, task_id: taskId, task: { ...task, priority: newPriority } }),
    });
    loadGoal();
}

// ============================================================================
// Task Edit Modal
// ============================================================================
function openTaskModal(taskId) {
    if (!STATE.goal) return;
    const task = STATE.goal.tasks.find(t => t.id === taskId);
    if (!task) return;

    STATE.editTaskId = taskId;
    document.getElementById('modalTaskTitle').value = task.title || '';
    document.getElementById('modalTaskPriority').value = task.priority || 'medium';
    document.getElementById('modalTaskStatus').value = task.status || 'pending';
    document.getElementById('taskModal').classList.add('show');
    setTimeout(() => document.getElementById('modalTaskTitle').focus(), 100);
}

function closeTaskModal() {
    document.getElementById('taskModal').classList.remove('show');
    STATE.editTaskId = null;
}

async function saveTaskEdit() {
    const taskId = STATE.editTaskId;
    if (!taskId || !STATE.goal) return;

    const task = STATE.goal.tasks.find(t => t.id === taskId);
    if (!task) { closeTaskModal(); return; }

    const updated = {
        ...task,
        title: document.getElementById('modalTaskTitle').value.trim(),
        priority: document.getElementById('modalTaskPriority').value,
        status: document.getElementById('modalTaskStatus').value,
    };
    if (!updated.title) { closeTaskModal(); return; }

    await fetch('/api/goal/task/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period, task_id: taskId, task: updated }),
    });
    closeTaskModal();
    loadGoal();
}

// ============================================================================
// Goal Deletion
// ============================================================================
function deleteGoal() {
    const info = document.getElementById('confirmGoalInfo');
    if (info) {
        info.textContent = `${LEVEL_LABELS[STATE.level]}目标 · ${formatPeriodLabel(STATE.level, STATE.period)}`;
    }
    document.getElementById('confirmModal').classList.add('show');
}

function closeConfirmModal() {
    document.getElementById('confirmModal').classList.remove('show');
}

async function confirmDeleteGoal() {
    await fetch('/api/goal/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ level: STATE.level, period: STATE.period }),
    });
    closeConfirmModal();
    STATE.period = getCurrentPeriod(STATE.level);
    loadGoal();
}

// ============================================================================
// Utilities
// ============================================================================
function esc(s) {
    const d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
}

document.addEventListener('DOMContentLoaded', init);
