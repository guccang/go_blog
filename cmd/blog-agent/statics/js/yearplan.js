// 全局变量
let currentYear = new Date().getFullYear(); // 默认当前年份
let monthTasks = {}; // 存储每个月的任务

// DOM加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 设置默认年份
    document.getElementById('current-year').textContent = currentYear;
    document.getElementById('search-year').value = currentYear;
    
    // 加载当前年份的计划
    loadYearPlan();
    
    // 设置模态框事件
    setupModalEvents();
    
    // 设置任务表单提交事件
    setupTaskFormSubmit();

});

// 切换年度计划总览的编辑和预览模式
function toggleOverviewPreview() {
    const editor = document.getElementById('overview-editor');
    const preview = document.getElementById('overview-preview');
    const toggleBtn = document.getElementById('toggle-preview-btn');
    
    if (editor.classList.contains('active')) {
        // 切换到预览模式
        const content = document.getElementById('year-overview-content').value;
        preview.innerHTML = markdownToHtml(content);
        editor.classList.remove('active');
        preview.classList.add('active');
        toggleBtn.classList.add('in-preview-mode');
        toggleBtn.innerHTML = `
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M3 17.25V21H6.75L17.81 9.94L14.06 6.19L3 17.25Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
                <path d="M14.06 6.19L17.81 9.94L20.71 7.04C21.1 6.65 21.1 6.02 20.71 5.63L18.37 3.29C17.98 2.9 17.35 2.9 16.96 3.29L14.06 6.19Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
            </svg>
            编辑
        `;
    } else {
        // 切换到编辑模式
        editor.classList.add('active');
        preview.classList.remove('active');
        toggleBtn.classList.remove('in-preview-mode');
        toggleBtn.innerHTML = `
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 6C7.6 6 3.8 8.8 2 12C3.8 15.2 7.6 18 12 18C16.4 18 20.2 15.2 22 12C20.2 8.8 16.4 6 12 6Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
                <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
            </svg>
            预览
        `;
        
        // 调整文本框高度
        setTimeout(() => {
            const textarea = document.getElementById('year-overview-content');
            if (window.adjustAllTextareas) {
                window.adjustAllTextareas();
            }
        }, 100);
    }
}

// 将Markdown转换为HTML的简单实现
function markdownToHtml(markdown) {
    if (!markdown) return '';

    return mdToHtml(markdown);
}

// 切换月度计划的编辑和预览模式
function toggleMonthPreview(month) {
    const editor = document.getElementById(`month-${month}-editor`);
    const preview = document.getElementById(`month-${month}-preview`);
    const toggleBtn = document.querySelector(`.month-card[data-month="${month}"] .month-preview-btn`);
    
    if (editor.classList.contains('active')) {
        // 切换到预览模式
        const content = document.getElementById(`month-${month}-content`).value;
        preview.innerHTML = markdownToHtml(content);
        editor.classList.remove('active');
        preview.classList.add('active');
        toggleBtn.classList.add('in-preview-mode');
        toggleBtn.innerHTML = `
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M3 17.25V21H6.75L17.81 9.94L14.06 6.19L3 17.25Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
                <path d="M14.06 6.19L17.81 9.94L20.71 7.04C21.1 6.65 21.1 6.02 20.71 5.63L18.37 3.29C17.98 2.9 17.35 2.9 16.96 3.29L14.06 6.19Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
            </svg>
            编辑
        `;
    } else {
        // 切换到编辑模式
        editor.classList.add('active');
        preview.classList.remove('active');
        toggleBtn.classList.remove('in-preview-mode');
        toggleBtn.innerHTML = `
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 6C7.6 6 3.8 8.8 2 12C3.8 15.2 7.6 18 12 18C16.4 18 20.2 15.2 22 12C20.2 8.8 16.4 6 12 6Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
                <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
            </svg>
            预览
        `;
        
        // 调整文本框高度
        setTimeout(() => {
            if (window.adjustAllTextareas) {
                window.adjustAllTextareas();
            }
        }, 100);
    }
}

// 设置模态框事件
function setupModalEvents() {
    const modal = document.getElementById('task-modal');
    const closeBtn = document.querySelector('.close-modal');
    const deleteBtn = document.getElementById('delete-task-btn');
    
    // 关闭模态框
    closeBtn.addEventListener('click', function() {
        modal.style.display = 'none';
    });
    
    // 点击模态框外部关闭
    window.addEventListener('click', function(event) {
        if (event.target === modal) {
            modal.style.display = 'none';
        }
    });
    
    // 删除任务按钮
    deleteBtn.addEventListener('click', function() {
        const taskId = document.getElementById('task-id').value;
        const month = document.getElementById('task-month').value;
        
        if (taskId && month) {
            deleteTask(month, taskId);
            modal.style.display = 'none';
        }
    });
}

// 设置任务表单提交
function setupTaskFormSubmit() {
    const form = document.getElementById('task-form');
    
    form.addEventListener('submit', function(e) {
        e.preventDefault();
        
        const taskId = document.getElementById('task-id').value;
        const month = document.getElementById('task-month').value;
        const name = document.getElementById('task-name').value;
        const description = document.getElementById('task-description').value;
        const status = document.getElementById('task-status').value;
        const priority = document.getElementById('task-priority').value;
        
        // 任务标题不能为空
        if (!name.trim()) {
            showToast('任务名称不能为空', 'error');
            return;
        }
        
        const task = {
            id: taskId || generateTaskId(),
            name: name,
            description: description,
            status: status || 'pending', // 默认为未开始
            priority: priority || 'medium', // 默认为中优先级
            createdAt: new Date().toISOString()
        };
        
        saveTask(month, task);
        document.getElementById('task-modal').style.display = 'none';
    });
}

// 显示提示消息
function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span class="toast-message">${message}</span>`;
    toastContainer.appendChild(toast);
    
    // 4秒后移除提示
    setTimeout(() => {
        toast.remove();
    }, 4000);
}

// 加载年度计划
function loadYearPlan() {
    const yearInput = document.getElementById('search-year');
    const year = parseInt(yearInput.value);
    
    if (isNaN(year) || year < 2020 || year > 2100) {
        showToast('请输入有效的年份（2020-2100）', 'error');
        return;
    }
    
    currentYear = year;
    document.getElementById('current-year').textContent = currentYear;
    
    // 显示加载中提示
    showToast('正在加载计划数据...', 'info');
    
    // 构建blog标题，格式为 "年计划_2023"
    const planTitle = `年计划_${currentYear}`;
    
    // 在加载新数据前清空现有数据
    document.getElementById('year-overview-content').value = '';
    for (let i = 1; i <= 12; i++) {
        document.getElementById(`month-${i}-content`).value = '';
    }
    
    // 重置任务数据
    monthTasks = {};
    for (let i = 1; i <= 12; i++) {
        monthTasks[i.toString()] = [];
    }
    renderAllTasks();
    
    // 从服务器获取数据
    fetch(`/api/getplan?title=${planTitle}`)
        .then(response => {
            if (!response.ok) {
                if (response.status === 404) {
                    // 如果计划不存在，初始化空计划
                    return { yearOverview: '', monthPlans: Array(12).fill(''), tasks: {} };
                }
                throw new Error('获取计划失败');
            }
            return response.json();
        })
        .then(data => {
            // 填充年度总览
            document.getElementById('year-overview-content').value = data.yearOverview || '';
            
            // 填充月度计划
            const monthPlans = data.monthPlans || Array(12).fill('');
            for (let i = 1; i <= 12; i++) {
                document.getElementById(`month-${i}-content`).value = monthPlans[i-1] || '';
            }
            
            // 加载每月任务
            if (data.tasks) {
                console.log('从服务器加载任务数据:', data.tasks);
                monthTasks = data.tasks;
                
                // 确保每个月份都有有效的数组
                for (let i = 1; i <= 12; i++) {
                    const monthKey = i.toString();
                    if (!monthTasks[monthKey]) {
                        monthTasks[monthKey] = [];
                    }
                }
            } else {
                console.log('服务器无任务数据，初始化空数据');
                monthTasks = {};
                for (let i = 1; i <= 12; i++) {
                    monthTasks[i.toString()] = [];
                }
            }
            
            renderAllTasks();
            
            // 确保任务数量显示正确，额外再次调用更新
            setTimeout(() => {
                for (let i = 1; i <= 12; i++) {
                    updateTaskCountBadge(i);
                }
            }, 300);
            
            // 切换年度计划总览的编辑和预览模式
            // 如果是编辑模式，切换到预览模式   
            if (document.getElementById('overview-editor').classList.contains('active')) {
                toggleOverviewPreview();
            }
            for (let i = 1; i <= 12; i++) {
                // 如果是编辑模式，切换到预览模式
                if (document.getElementById(`month-${i}-editor`).classList.contains('active')) {
                    toggleMonthPreview(i);
                }
            }
            showToast('计划数据加载完成', 'success');
            
            // 调整所有文本框的高度
            setTimeout(() => {
                if (window.adjustAllTextareas) {
                    window.adjustAllTextareas();
                }
            }, 300);
        })
        .catch(error => {
            console.error('加载计划失败:', error);
            showToast('加载计划失败，请稍后重试', 'error');
        });
}

// 渲染所有月份的任务
function renderAllTasks() {
    for (let i = 1; i <= 12; i++) {
        renderMonthTasks(i);
    }
    
    // 在所有任务渲染完成后，再次更新任务数量显示
    for (let i = 1; i <= 12; i++) {
        updateTaskCountBadge(i);
    }
    
    // 更新底部任务统计
    updateTaskStats();
}

// 渲染指定月份的任务
function renderMonthTasks(month) {
    const tasksContainer = document.getElementById(`month-${month}-tasks`);
    tasksContainer.innerHTML = '';
    
    // 确保month是字符串类型键
    const monthKey = month.toString();
    const monthTasksArray = monthTasks[monthKey] || [];
    
    console.log(`渲染月份 ${monthKey} 任务，任务数: ${monthTasksArray.length}`);
    
    if (monthTasksArray.length === 0) {
        tasksContainer.innerHTML = '<div class="empty-tasks">暂无任务</div>';
        return;
    }
    
    monthTasksArray.forEach(task => {
        const taskEl = createTaskElement(task, month);
        tasksContainer.appendChild(taskEl);
    });
    
    // 更新任务数量显示
    updateTaskCountBadge(month);
}

// 切换月份任务列表的显示/隐藏
function toggleMonthTasks(month) {
    const tasksContainer = document.getElementById(`month-${month}-tasks`);
    const toggleBtn = document.querySelector(`.month-card[data-month="${month}"] .toggle-tasks-btn`);
    const toggleText = toggleBtn.querySelector('.toggle-tasks-text');
    
    if (tasksContainer.classList.contains('collapsed')) {
        // 展开任务列表
        tasksContainer.classList.remove('collapsed');
        tasksContainer.style.maxHeight = tasksContainer.scrollHeight + 'px';
        toggleBtn.classList.add('expanded');
        
        // 如果是第一次展开，可能需要渲染任务
        if (tasksContainer.childElementCount === 0 || tasksContainer.innerHTML.trim() === '') {
            renderMonthTasks(month);
        }
        
        // 更新按钮文本为"隐藏任务"
        const monthKey = month.toString();
        const monthTasksArray = monthTasks[monthKey] || [];
        if (monthTasksArray.length > 0) {
            toggleText.textContent = '隐藏任务';
        } else {
            toggleText.textContent = '隐藏任务';
        }
    } else {
        // 收起任务列表
        tasksContainer.classList.add('collapsed');
        tasksContainer.style.maxHeight = '0';
        toggleBtn.classList.remove('expanded');
        
        // 恢复显示任务数量
        updateTaskCountBadge(month);
    }
}

// 更新月份任务数量标识
function updateTaskCountBadge(month) {
    try {
        const monthKey = month.toString();
        const monthTasksArray = monthTasks[monthKey] || [];
        const toggleBtn = document.querySelector(`.month-card[data-month="${month}"] .toggle-tasks-btn`);
        
        if (!toggleBtn) {
            console.warn(`找不到月份 ${month} 的切换按钮`);
            return;
        }
        
        const toggleText = toggleBtn.querySelector('.toggle-tasks-text');
        if (!toggleText) {
            console.warn(`找不到月份 ${month} 的切换按钮文本`);
            return;
        }
        
        if (monthTasksArray.length > 0) {
            toggleText.textContent = `显示任务 (${monthTasksArray.length})`;
        } else {
            toggleText.textContent = '显示任务';
        }
        
        console.log(`已更新月份 ${monthKey} 任务数量标识为 ${monthTasksArray.length}`);
    } catch (error) {
        console.error(`更新月份 ${month} 任务数量时出错:`, error);
    }
}

// 创建任务元素
function createTaskElement(task, month) {
    const taskEl = document.createElement('div');
    taskEl.className = 'task-item';
    taskEl.setAttribute('data-id', task.id);
    taskEl.setAttribute('data-month', month);
    
    // 添加优先级指示器
    const priorityIndicator = document.createElement('div');
    priorityIndicator.className = `priority-indicator priority-${task.priority}`;
    taskEl.style.position = 'relative';
    taskEl.appendChild(priorityIndicator);
    
    // 添加状态图标（替换原来的复选框）
    const statusIcon = document.createElement('div');
    statusIcon.className = `status-icon status-${task.status}`;
    
    // 根据状态设置图标内容 - 使用SVG替代原来的Emoji
    switch(task.status) {
        case 'pending':
            // 未开始 - 空心圆
            statusIcon.innerHTML = `<svg class="icon" viewBox="0 0 24 24" width="24" height="24">
                <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>
            </svg>`;
            break;
        case 'in-progress':
            // 进行中 - 半圆环
            statusIcon.innerHTML = `<svg class="icon" viewBox="0 0 24 24" width="24" height="24">
                <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>
                <path d="M12 3 A 9 9 0 0 1 21 12" stroke="currentColor" stroke-width="2" fill="none"/>
            </svg>`;
            break;
        case 'completed':
            // 已完成 - 带勾的圆
            statusIcon.innerHTML = `<svg class="icon" viewBox="0 0 24 24" width="24" height="24">
                <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>
                <path d="M8 12 L11 15 L16 9" stroke="currentColor" stroke-width="2" fill="none"/>
            </svg>`;
            break;
    }
    
    // 添加点击事件切换状态
    statusIcon.addEventListener('click', function(e) {
        e.stopPropagation(); // 阻止冒泡
        toggleTaskStatus(month, task.id);
    });
    
    taskEl.appendChild(statusIcon);
    
    // 任务内容
    const contentDiv = document.createElement('div');
    contentDiv.className = 'task-content';
    
    const taskName = document.createElement('p');
    taskName.className = 'task-name';
    taskName.textContent = task.name;
    if (task.status === 'completed') {
        taskName.style.textDecoration = 'line-through';
        taskName.style.color = '#999';
    }
    contentDiv.appendChild(taskName);
    
    if (task.description) {
        const taskDesc = document.createElement('p');
        taskDesc.className = 'task-description';
        taskDesc.textContent = task.description;
        contentDiv.appendChild(taskDesc);
    }
    
    taskEl.appendChild(contentDiv);
    
    // 状态标签
    const statusBadge = document.createElement('span');
    statusBadge.className = `task-status-badge status-${task.status}`;
    statusBadge.textContent = getStatusText(task.status);
    taskEl.appendChild(statusBadge);
    
    // 点击编辑任务
    taskEl.addEventListener('click', function() {
        editTask(month, task.id);
    });
    
    return taskEl;
}

// 获取状态文本
function getStatusText(status) {
    switch (status) {
        case 'pending': return '未开始';
        case 'in-progress': return '进行中';
        case 'completed': return '已完成';
        default: return '未知';
    }
}

// 切换任务状态
function toggleTaskStatus(month, taskId) {
    // 确保month是字符串类型键
    const monthKey = month.toString();
    const tasks = monthTasks[monthKey] || [];
    const taskIndex = tasks.findIndex(t => t.id === taskId);
    
    if (taskIndex !== -1) {
        const task = tasks[taskIndex];
        
        // 循环切换三种状态：未开始 -> 进行中 -> 已完成 -> 未开始
        switch(task.status) {
            case 'pending':
                task.status = 'in-progress';
                break;
            case 'in-progress':
                task.status = 'completed';
                break;
            case 'completed':
                task.status = 'pending';
                break;
            default:
                task.status = 'pending';
        }
        
        console.log(`切换任务状态，月份: ${monthKey}，任务: ${task.name}，状态: ${task.status}`);
        
        renderMonthTasks(month);
        updateTaskStats(); // 更新任务统计
        savePlan();
    }
}

// 显示添加任务模态框
function showAddTaskModal(month) {
    const modal = document.getElementById('task-modal');
    const form = document.getElementById('task-form');
    const modalTitle = document.getElementById('modal-title');
    const deleteBtn = document.getElementById('delete-task-btn');
    
    modalTitle.textContent = `添加任务 - ${getMonthName(month)}`;
    form.reset();
    document.getElementById('task-month').value = month;
    document.getElementById('task-id').value = '';
    deleteBtn.style.display = 'none';
    
    modal.style.display = 'block';
}

// 获取月份名称
function getMonthName(month) {
    const monthNames = ['一月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '十二月'];
    return monthNames[month - 1];
}

// 编辑任务
function editTask(month, taskId) {
    const tasks = monthTasks[month] || [];
    const task = tasks.find(t => t.id === taskId);
    
    if (!task) return;
    
    const modal = document.getElementById('task-modal');
    const form = document.getElementById('task-form');
    const modalTitle = document.getElementById('modal-title');
    const deleteBtn = document.getElementById('delete-task-btn');
    
    modalTitle.textContent = `编辑任务 - ${getMonthName(month)}`;
    document.getElementById('task-month').value = month;
    document.getElementById('task-id').value = taskId;
    document.getElementById('task-name').value = task.name;
    document.getElementById('task-description').value = task.description || '';
    document.getElementById('task-status').value = task.status;
    document.getElementById('task-priority').value = task.priority;
    deleteBtn.style.display = 'block';
    
    modal.style.display = 'block';
}

// 保存任务
function saveTask(month, task) {
    if (!monthTasks[month]) {
        monthTasks[month] = [];
    }
    
    // 确保month是字符串类型键
    const monthKey = month.toString();
    if (!monthTasks[monthKey]) {
        monthTasks[monthKey] = [];
    }
    
    const taskIndex = monthTasks[monthKey].findIndex(t => t.id === task.id);
    
    if (taskIndex !== -1) {
        // 更新现有任务
        monthTasks[monthKey][taskIndex] = task;
    } else {
        // 添加新任务
        monthTasks[monthKey].push(task);
    }
    
    // 输出调试信息
    console.log(`保存任务至月份 ${monthKey}，任务ID: ${task.id}，当前任务数: ${monthTasks[monthKey].length}`);
    
    renderMonthTasks(month);
    updateTaskStats(); // 更新任务统计
    savePlan();
    
    showToast('任务已保存', 'success');
}

// 删除任务
function deleteTask(month, taskId) {
    // 确保month是字符串类型键
    const monthKey = month.toString();
    const tasks = monthTasks[monthKey] || [];
    
    const taskIndex = tasks.findIndex(t => t.id === taskId);
    
    if (taskIndex !== -1) {
        tasks.splice(taskIndex, 1);
        
        renderMonthTasks(month);
        updateTaskStats(); // 更新任务统计
        savePlan();
        
        showToast('任务已删除', 'warning');
    }
}

// 生成任务ID
function generateTaskId() {
    return Date.now().toString(36) + Math.random().toString(36).substring(2);
}

// 编辑月度计划
function editMonth(month) {
    const textarea = document.getElementById(`month-${month}-content`);
    textarea.focus();
    
    // 滚动到视图
    const monthCard = document.querySelector(`.month-card[data-month="${month}"]`);
    monthCard.scrollIntoView({ behavior: 'smooth', block: 'center' });
    
    // 高亮效果
    monthCard.style.boxShadow = '0 0 0 2px #409eff';
    setTimeout(() => {
        monthCard.style.boxShadow = '';
    }, 2000);
}

// 保存年度计划
function savePlan() {
    // 收集所有数据
    const yearOverview = document.getElementById('year-overview-content').value;
    const monthPlans = [];
    
    for (let i = 1; i <= 12; i++) {
        monthPlans.push(document.getElementById(`month-${i}-content`).value);
    }
    
    // 构建保存的数据
    const planData = {
        title: `年计划_${currentYear}`,
        yearOverview: yearOverview,
        monthPlans: monthPlans,
        year: currentYear,
        tasks: monthTasks
    };
    
    // 显示保存中提示
    showToast('正在保存计划...', 'info');
    
    // 输出日志以便调试
    console.log('保存的计划数据:', JSON.stringify(planData));
    
    // 发送到服务器
    fetch('/api/saveplan', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(planData)
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('保存计划失败');
        }
        return response.json();
    })
    .then(data => {
        showToast('计划保存成功', 'success');
        console.log('服务器响应:', data);
    })
    .catch(error => {
        console.error('保存计划失败:', error);
        showToast('保存计划失败，请稍后重试', 'error');
    });
}

// 更新底部任务统计
function updateTaskStats() {
    let totalTasks = 0;
    let completedTasks = 0;
    
    // 统计总任务数和已完成任务数
    Object.keys(monthTasks).forEach(month => {
        const tasks = monthTasks[month] || [];
        totalTasks += tasks.length;
        completedTasks += tasks.filter(task => task.status === 'completed').length;
    });
    
    // 更新显示
    document.getElementById('total-tasks').textContent = `总任务数: ${totalTasks}`;
    document.getElementById('completed-tasks').textContent = `已完成: ${completedTasks}`;
} 