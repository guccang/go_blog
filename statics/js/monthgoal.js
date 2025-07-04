// 月度工作目标管理 JavaScript

// 全局变量
let currentYear = new Date().getFullYear();
let currentMonth = new Date().getMonth() + 1;
let currentWeek = 1;
let currentTaskId = null;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeTabSystem();
    loadMonthGoal();
    initializeWeekTabs();
});

// 初始化页签系统
function initializeTabSystem() {
    const tabBtns = document.querySelectorAll('.tab-btn');
    const tabContents = document.querySelectorAll('.tab-content');
    
    tabBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const targetTab = this.getAttribute('data-tab');
            
            // 移除所有活跃状态
            tabBtns.forEach(b => b.classList.remove('active'));
            tabContents.forEach(content => content.classList.remove('active'));
            
            // 添加活跃状态
            this.classList.add('active');
            document.getElementById(targetTab + '-content').classList.add('active');
            
            // 特殊处理
            if (targetTab === 'weeks') {
                loadWeekGoals();
            } else if (targetTab === 'tasks') {
                loadTasks();
            }
        });
    });
}

// 加载月度目标
async function loadMonthGoal() {
    const year = document.getElementById('yearInput').value;
    const month = document.getElementById('monthSelect').value;
    
    currentYear = parseInt(year);
    currentMonth = parseInt(month);
    
    // 更新页面标题
    const pageTitle = document.querySelector('.page-title');
    if (pageTitle) {
        pageTitle.textContent = `${currentYear}年${currentMonth}月工作目标`;
    }
    
    try {
        const response = await fetch(`/api/monthgoal?year=${year}&month=${month}`);
        const data = await response.json();
        
        if (response.ok && data.success) {
            document.getElementById('month-overview-content').value = data.data.content || '';
            showToast('月度目标加载成功', 'success');
        } else if (response.status === 404) {
            // 月度目标不存在，初始化为空
            document.getElementById('month-overview-content').value = '';
            showToast('暂无月度目标，请添加内容', 'info');
        } else {
            throw new Error(data.message || '加载失败');
        }
        
        // 重新初始化周页签，重新计算年度周数
        initializeWeekTabs();
        
    } catch (error) {
        console.error('加载月度目标失败:', error);
        showToast('加载月度目标失败', 'error');
        
        // 即使加载失败，也要重新初始化周页签
        initializeWeekTabs();
    }
}

// 保存月度总结
async function saveMonthOverview() {
    const content = document.getElementById('month-overview-content').value;
    
    try {
        const response = await fetch('/api/savemonthgoal', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                year: currentYear,
                month: currentMonth,
                content: content
            })
        });
        
        const data = await response.json();
        
        if (response.ok && data.status === 'success') {
            showToast('月度总结保存成功', 'success');
        } else {
            showToast('保存失败: ' + (data.message || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('保存月度总结失败:', error);
        showToast('保存失败', 'error');
    }
}

// 切换预览模式
function toggleOverviewPreview() {
    const editorArea = document.querySelector('#overview-content .editor-area');
    const previewArea = document.querySelector('#overview-content .preview-area');
    const content = document.getElementById('month-overview-content').value;
    
    if (editorArea.classList.contains('active')) {
        // 切换到预览模式
        editorArea.classList.remove('active');
        previewArea.classList.add('active');
        
        if (window.marked) {
            previewArea.innerHTML = window.marked.parse(content);
        } else {
            previewArea.innerHTML = '<p>Markdown渲染器未加载</p>';
        }
    } else {
        // 切换到编辑模式
        previewArea.classList.remove('active');
        editorArea.classList.add('active');
    }
}

// 获取月份的周数
function getWeeksInMonth(year, month) {
    const firstDay = new Date(year, month - 1, 1);
    const lastDay = new Date(year, month, 0);
    const daysInMonth = lastDay.getDate();
    
    return Math.ceil(daysInMonth / 7);
}

// 计算年度周数
function getWeekOfYear(date) {
    const firstDayOfYear = new Date(date.getFullYear(), 0, 1);
    const pastDaysOfYear = (date - firstDayOfYear) / 86400000;
    return Math.ceil((pastDaysOfYear + firstDayOfYear.getDay() + 1) / 7);
}

// 获取指定月份的年度周数范围
function getYearWeeksForMonth(year, month) {
    const firstDay = new Date(year, month - 1, 1);
    const lastDay = new Date(year, month, 0);
    
    const firstWeek = getWeekOfYear(firstDay);
    const lastWeek = getWeekOfYear(lastDay);
    
    const weeks = [];
    for (let week = firstWeek; week <= lastWeek; week++) {
        weeks.push(week);
    }
    
    return weeks;
}

// 初始化周页签
function initializeWeekTabs() {
    const weekTabsContainer = document.getElementById('week-tabs');
    const yearWeeks = getYearWeeksForMonth(currentYear, currentMonth);
    
    weekTabsContainer.innerHTML = '';
    
    yearWeeks.forEach((yearWeek, index) => {
        const weekTab = document.createElement('div');
        weekTab.className = `week-tab ${index === 0 ? 'active' : ''}`;
        weekTab.textContent = `第${yearWeek}周`;
        weekTab.setAttribute('data-week', yearWeek);
        weekTab.setAttribute('data-month-week', index + 1); // 保留月内周数用于后端API
        
        weekTab.addEventListener('click', function() {
            selectWeek(yearWeek, index + 1);
        });
        
        weekTabsContainer.appendChild(weekTab);
    });
    
    if (yearWeeks.length > 0) {
        selectWeek(yearWeeks[0], 1);
    }
}

// 选择周
function selectWeek(yearWeek, monthWeek) {
    currentWeek = monthWeek; // 保留月内周数用于后端API
    
    // 更新周页签状态
    document.querySelectorAll('.week-tab').forEach(tab => {
        tab.classList.remove('active');
    });
    document.querySelector(`[data-week="${yearWeek}"]`).classList.add('active');
    
    // 加载周目标，传递年度周数用于显示
    loadWeekGoal(monthWeek, yearWeek);
}

// 加载周目标详情
function loadWeekGoals() {
    const activeTab = document.querySelector('.week-tab.active');
    if (activeTab) {
        const yearWeek = parseInt(activeTab.getAttribute('data-week'));
        const monthWeek = parseInt(activeTab.getAttribute('data-month-week'));
        loadWeekGoal(monthWeek, yearWeek);
    }
}

// 加载周目标
async function loadWeekGoal(monthWeek, yearWeek) {
    const contentArea = document.getElementById('week-content-area');
    
    contentArea.innerHTML = `
        <div class="week-content">
            <div class="week-header">
                <h4 class="week-title">${currentYear}年第${yearWeek}周目标</h4>
                <div class="week-actions">
                    <button class="action-btn preview-btn" onclick="toggleWeekPreview(${monthWeek})">
                        <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z" fill="currentColor"/>
                        </svg>
                        预览
                    </button>
                    <button class="action-btn save-btn" onclick="saveWeekGoal(${monthWeek}, ${yearWeek})">
                        <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M17 3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2V7l-4-4zm-5 16c-1.66 0-3-1.34-3-3s1.34-3 3-3 3 1.34 3 3-1.34 3-3 3zm3-10H5V5h10v4z" fill="currentColor"/>
                        </svg>
                        保存
                    </button>
                </div>
            </div>
            <div class="week-editor active" id="week-editor-${monthWeek}">
                <textarea class="week-textarea" id="week-content-${monthWeek}" placeholder="在这里输入第${yearWeek}周的目标...支持Markdown格式"></textarea>
            </div>
            <div class="week-preview" id="week-preview-${monthWeek}">
                <!-- 预览内容将在这里显示 -->
            </div>
        </div>
    `;
    
    // 加载周目标内容
    try {
        const response = await fetch(`/api/weekgoal?year=${currentYear}&month=${currentMonth}&week=${monthWeek}`);
        const data = await response.json();
        
        if (response.ok && data.success) {
            document.getElementById(`week-content-${monthWeek}`).value = data.data.content || '';
        } else if (response.status === 404) {
            // 周目标不存在，初始化为空
            document.getElementById(`week-content-${monthWeek}`).value = '';
        } else {
            throw new Error(data.message || '加载失败');
        }
    } catch (error) {
        console.error('加载周目标失败:', error);
        showToast('加载周目标失败', 'error');
    }
}

// 切换周预览
function toggleWeekPreview(monthWeek) {
    const editorArea = document.getElementById(`week-editor-${monthWeek}`);
    const previewArea = document.getElementById(`week-preview-${monthWeek}`);
    const content = document.getElementById(`week-content-${monthWeek}`).value;
    
    if (editorArea.classList.contains('active')) {
        // 切换到预览模式
        editorArea.classList.remove('active');
        previewArea.classList.add('active');
        
        if (window.marked) {
            previewArea.innerHTML = window.marked.parse(content);
        } else {
            previewArea.innerHTML = '<p>Markdown渲染器未加载</p>';
        }
    } else {
        // 切换到编辑模式
        previewArea.classList.remove('active');
        editorArea.classList.add('active');
    }
}

// 保存周目标
async function saveWeekGoal(monthWeek, yearWeek) {
    const content = document.getElementById(`week-content-${monthWeek}`).value;
    
    try {
        const response = await fetch('/api/saveweekgoal', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                year: currentYear,
                month: currentMonth,
                week: monthWeek,
                content: content
            })
        });
        
        const data = await response.json();
        
        if (response.ok && data.status === 'success') {
            showToast(`第${yearWeek}周目标保存成功`, 'success');
        } else {
            showToast('保存失败: ' + (data.message || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('保存周目标失败:', error);
        showToast('保存失败', 'error');
    }
}

// 任务管理
async function loadTasks() {
    try {
        const response = await fetch(`/api/monthgoals?year=${currentYear}`);
        
        if (response.ok) {
            const data = await response.json();
            // 后端返回的是包含12个月数据的数组，按月份索引查找
            const currentMonthGoal = data.find(goal => goal.month === currentMonth);
            const tasks = (currentMonthGoal && currentMonthGoal.tasks) ? currentMonthGoal.tasks : [];
            renderTasks(tasks);
        } else {
            // 尝试使用单个月度目标API作为备用
            const monthResponse = await fetch(`/api/monthgoal?year=${currentYear}&month=${currentMonth}`);
            if (monthResponse.ok) {
                const monthData = await monthResponse.json();
                if (monthData.success && monthData.data.tasks) {
                    renderTasks(monthData.data.tasks);
                } else {
                    renderTasks([]);
                }
            } else {
                showToast('加载任务失败', 'error');
                renderTasks([]);
            }
        }
    } catch (error) {
        console.error('加载任务失败:', error);
        showToast('加载任务失败', 'error');
        renderTasks([]);
    }
}

// 渲染任务列表
function renderTasks(tasks) {
    const tasksContainer = document.getElementById('tasks-list');
    
    if (!tasks || tasks.length === 0) {
        tasksContainer.innerHTML = '<div class="no-tasks">暂无任务</div>';
        return;
    }
    
    tasksContainer.innerHTML = tasks.map(task => `
        <div class="task-item">
            <div class="task-content">
                <h4 class="task-title">${escapeHtml(task.title)}</h4>
                ${task.description ? `<p class="task-description">${escapeHtml(task.description)}</p>` : ''}
                <div class="task-meta">
                    <span class="priority-badge priority-${task.priority}">${getPriorityText(task.priority)}</span>
                    <span class="status-badge status-${task.status}">${getStatusText(task.status)}</span>
                    ${task.due_date ? `<span class="due-date">截止: ${task.due_date}</span>` : ''}
                </div>
            </div>
            <div class="task-actions">
                <button class="task-btn edit-btn" onclick="editTask('${task.id}')">编辑</button>
                <button class="task-btn delete-btn" onclick="deleteTask('${task.id}')">删除</button>
            </div>
        </div>
    `).join('');
}

// 显示添加任务模态框
function showAddTaskModal() {
    currentTaskId = null;
    document.getElementById('modal-title').textContent = '添加任务';
    document.getElementById('task-form').reset();
    document.getElementById('task-modal').style.display = 'block';
}

// 编辑任务 - 使用单个月度目标API来获取任务数据
async function editTask(taskId) {
    try {
        const response = await fetch(`/api/monthgoal?year=${currentYear}&month=${currentMonth}`);
        
        if (response.ok) {
            const data = await response.json();
            if (data.success && data.data.tasks) {
                const task = data.data.tasks.find(t => t.id === taskId);
                if (task) {
                    currentTaskId = taskId;
                    document.getElementById('modal-title').textContent = '编辑任务';
                    document.getElementById('task-title').value = task.title;
                    document.getElementById('task-description').value = task.description || '';
                    document.getElementById('task-priority').value = task.priority;
                    document.getElementById('task-status').value = task.status;
                    document.getElementById('task-due-date').value = task.due_date || '';
                    
                    document.getElementById('task-modal').style.display = 'block';
                } else {
                    showToast('任务不存在', 'error');
                }
            } else {
                showToast('任务不存在', 'error');
            }
        } else {
            showToast('加载任务详情失败', 'error');
        }
    } catch (error) {
        console.error('加载任务详情失败:', error);
        showToast('加载任务详情失败', 'error');
    }
}

// 保存任务
async function saveTask() {
    const title = document.getElementById('task-title').value.trim();
    if (!title) {
        showToast('请输入任务标题', 'warning');
        return;
    }
    
    const taskData = {
        title: title,
        description: document.getElementById('task-description').value.trim(),
        priority: document.getElementById('task-priority').value,
        status: document.getElementById('task-status').value,
        due_date: document.getElementById('task-due-date').value,
        id: currentTaskId || Date.now().toString() // 生成ID
    };
    
    try {
        const apiUrl = currentTaskId ? '/api/updatetask' : '/api/addtask';
        const requestData = {
            year: currentYear,
            month: currentMonth,
            task: taskData
        };
        
        if (currentTaskId) {
            requestData.task_id = currentTaskId;
        }
        
        const response = await fetch(apiUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestData)
        });
        
        const data = await response.json();
        
        if (response.ok && data.status === 'success') {
            closeTaskModal();
            loadTasks();
            showToast(currentTaskId ? '任务更新成功' : '任务添加成功', 'success');
        } else {
            showToast('保存失败: ' + (data.message || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('保存任务失败:', error);
        showToast('保存任务失败', 'error');
    }
}

// 删除任务
async function deleteTask(taskId) {
    if (!confirm('确定要删除这个任务吗？')) {
        return;
    }
    
    try {
        const response = await fetch('/api/deletetask', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                year: currentYear,
                month: currentMonth,
                task_id: taskId
            })
        });
        
        const data = await response.json();
        
        if (response.ok && data.status === 'success') {
            loadTasks();
            showToast('任务删除成功', 'success');
        } else {
            showToast('删除失败: ' + (data.message || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('删除任务失败:', error);
        showToast('删除任务失败', 'error');
    }
}

// 关闭任务模态框
function closeTaskModal() {
    document.getElementById('task-modal').style.display = 'none';
    currentTaskId = null;
}

// 工具函数
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function getPriorityText(priority) {
    const priorities = {
        'low': '低',
        'medium': '中',
        'high': '高',
        'urgent': '紧急'
    };
    return priorities[priority] || priority;
}

function getStatusText(status) {
    const statuses = {
        'pending': '待办',
        'in_progress': '进行中',
        'completed': '已完成',
        'cancelled': '已取消'
    };
    return statuses[status] || status;
}

// 显示提示信息
function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toast-container');
    
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    
    toastContainer.appendChild(toast);
    
    setTimeout(() => {
        toast.remove();
    }, 3000);
}

// 点击模态框外部关闭
window.onclick = function(event) {
    const modal = document.getElementById('task-modal');
    if (event.target === modal) {
        closeTaskModal();
    }
} 