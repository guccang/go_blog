/**
 * Agent Task Center - JavaScript
 * 后台任务中心核心逻辑
 */

// ============================================================================
// Global State
// ============================================================================
let ws = null;
let currentTasks = [];
let currentReminders = {};
let currentActiveIds = [];
let currentGraphTaskId = null;
let currentGraphData = null;
let currentLogs = [];
let logFilter = 'all';
let currentInputRequest = null;
let inputValue = null;

// ============================================================================
// Initialization
// ============================================================================
document.addEventListener('DOMContentLoaded', function () {
    loadTasks();

    // 节流控制：避免频繁刷新
    let loadTasksThrottled = throttle(loadTasks, 2000); // 最多每2秒刷新一次
    let pendingRefresh = false;

    // 使用 AgentNotifier 监听更新
    if (window.AgentNotifier) {
        window.AgentNotifier.addListener(function (notification) {
            // 只在特定类型通知时刷新任务列表
            const refreshTypes = ['submitted', 'completed', 'failed', 'canceled', 'retrying'];
            if (refreshTypes.includes(notification.type)) {
                loadTasksThrottled();
            } else if (notification.type && (notification.type.startsWith('node_') || notification.type.startsWith('graph_'))) {
                // 节点更新：标记需要刷新，但不立即刷新
                if (!pendingRefresh) {
                    pendingRefresh = true;
                    setTimeout(() => {
                        pendingRefresh = false;
                        loadTasksThrottled();
                    }, 3000); // 延迟3秒合并多个节点更新
                }
            }

            // 处理图表更新
            if (notification.type && notification.type.startsWith('graph_') ||
                notification.type && notification.type.startsWith('node_')) {
                if (currentGraphTaskId === notification.task_id) {
                    viewTaskGraph(currentGraphTaskId);
                }
            }

            // 处理输入请求
            if (notification.type === 'input_required') {
                handleInputNotification(notification);
            }
        });

        // 更新状态显示
        window.AgentNotifier.updateStatus(
            window.AgentNotifier.ws && window.AgentNotifier.ws.readyState === WebSocket.OPEN
        );
    }

    // 回车提交任务
    const taskInput = document.getElementById('taskInput');
    if (taskInput) {
        taskInput.addEventListener('keypress', function (e) {
            if (e.key === 'Enter') createTask();
        });
    }

    // 点击背景关闭弹窗
    const taskModal = document.getElementById('taskModal');
    if (taskModal) {
        taskModal.addEventListener('click', function (e) {
            if (e.target === this) closeModal();
        });
    }
});

// ============================================================================
// Task CRUD
// ============================================================================
async function createTask() {
    const input = document.getElementById('taskInput');
    const description = input.value.trim();
    if (!description) return;

    try {
        const response = await fetch('/api/agent/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                title: description.substring(0, 50),
                description: description,
                priority: 5
            })
        });

        const data = await response.json();
        if (data.success) {
            input.value = '';
            loadTasks();
        } else {
            alert('创建失败: ' + data.error);
        }
    } catch (error) {
        console.error('创建任务失败:', error);
        alert('创建任务失败');
    }
}

async function loadTasks() {
    try {
        const response = await fetch('/api/agent/tasks');
        const data = await response.json();

        if (data.success) {
            currentTasks = data.tasks || [];
            currentReminders = data.reminders || {};
            currentActiveIds = data.activeIds || [];
            renderTasks(currentTasks);
            updateStats(currentTasks);
        }
    } catch (error) {
        console.error('加载任务失败:', error);
    }
}

async function taskAction(taskId, action) {
    try {
        const response = await fetch('/api/agent/task/action', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ task_id: taskId, action: action })
        });

        const data = await response.json();
        if (data.success) {
            loadTasks();
        } else {
            alert('操作失败: ' + data.error);
        }
    } catch (error) {
        console.error('操作失败:', error);
    }
}

function pauseTask(id) { taskAction(id, 'pause'); }
function resumeTask(id) { taskAction(id, 'resume'); }
function cancelTask(id) { taskAction(id, 'cancel'); }
function retryTask(id) { taskAction(id, 'retry'); }

function deleteTask(id) {
    if (!confirm('确定要删除这个任务吗？')) return;

    fetch(`/api/agent/task?id=${id}`, { method: 'DELETE' })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                loadTasks();
            } else {
                alert('删除失败: ' + data.error);
            }
        })
        .catch(err => console.error('删除失败:', err));
}

// ============================================================================
// Task Rendering
// ============================================================================
function renderTasks(tasks) {
    const container = document.getElementById('taskList');
    if (!container) return;

    if (!tasks || tasks.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <i class="fas fa-inbox"></i>
                <p>暂无任务，创建一个新任务开始吧！</p>
            </div>
        `;
        return;
    }

    container.innerHTML = tasks.map(task => {
        // 简化后的 TaskSummary 格式
        const taskId = task.id;
        const taskTitle = task.title || '未命名任务';
        const taskStatus = task.status || 'pending';
        const taskProgress = task.progress || 0;
        const taskCreatedAt = task.created_at;

        // 检查是否活跃运行中
        const isActive = currentActiveIds && currentActiveIds.includes(taskId);

        let statusHtml = isActive
            ? `<span class="task-status running" style="animation: pulse 1.5s infinite;"><i class="fas fa-sync fa-spin"></i> 执行中</span>`
            : `<span class="task-status ${taskStatus}">${getStatusText(taskStatus)}</span>`;
        let progressHtml = `
                <div class="progress-bar">
                    <div class="progress-fill${isActive ? ' active' : ''}" style="width: ${taskProgress}%"></div>
                </div>
                <div class="progress-text">
                    <span>${taskProgress.toFixed ? taskProgress.toFixed(0) : taskProgress}% 完成</span>
                    <span>${formatTime(taskCreatedAt)}</span>
                </div>`;

        return `
        <div class="task-item" data-id="${taskId}" onclick="viewTaskDetail('${taskId}')">
            <div class="task-header">
                <span class="task-title">${escapeHtml(taskTitle.substring ? taskTitle.substring(0, 50) : taskTitle)}</span>
                ${statusHtml}
            </div>
            <div class="task-progress" onclick="event.stopPropagation(); viewTaskGraph('${taskId}')" style="cursor: pointer;" title="点击查看任务图">
                ${progressHtml}
            </div>
            <div class="task-actions" onclick="event.stopPropagation()">
                <button class="action-btn view" onclick="viewTaskDetail('${taskId}')">
                    <i class="fas fa-eye"></i> 详情
                </button>
                <button class="action-btn" onclick="viewTaskGraph('${taskId}')" style="background: rgba(168, 85, 247, 0.2); color: #a855f7;">
                    <i class="fas fa-project-diagram"></i> 图表
                </button>
                ${getActionButtons({ id: taskId, status: taskStatus })}
            </div>
        </div>
        `}).join('');
}

function updateStats(tasks) {
    const stats = { pending: 0, running: 0, done: 0, failed: 0 };
    tasks.forEach(task => {
        const status = task.status || 'pending';
        if (status === 'pending' || status === 'node_pending') stats.pending++;
        else if (status === 'running' || status === 'paused' || status === 'node_running') stats.running++;
        else if (status === 'done' || status === 'node_done') stats.done++;
        else stats.failed++;
    });

    const elements = {
        pending: document.getElementById('pendingCount'),
        running: document.getElementById('runningCount'),
        done: document.getElementById('doneCount'),
        failed: document.getElementById('failedCount')
    };

    if (elements.pending) elements.pending.textContent = stats.pending;
    if (elements.running) elements.running.textContent = stats.running;
    if (elements.done) elements.done.textContent = stats.done;
    if (elements.failed) elements.failed.textContent = stats.failed;
}

// ============================================================================
// Task Detail Modal
// ============================================================================
async function viewTaskDetail(taskId) {
    // 先从摘要显示基本信息
    const summary = currentTasks.find(t => t.id === taskId);

    const modalTitle = document.getElementById('modalTitle');
    if (modalTitle) modalTitle.textContent = summary?.title || '任务详情';

    // 显示弹窗
    const modal = document.getElementById('taskModal');
    if (modal) modal.classList.add('show');

    // 显示加载中
    const taskContent = document.getElementById('taskContent');
    if (taskContent) {
        taskContent.innerHTML = '<p style="color: var(--text-muted);">加载中...</p>';
    }

    try {
        // 获取完整任务数据
        const response = await fetch(`/api/agent/task/graph?id=${taskId}`);
        const data = await response.json();

        if (!data.success) {
            if (taskContent) {
                taskContent.innerHTML = `<p style="color: var(--danger);">加载失败: ${data.error || '未知错误'}</p>`;
            }
            return;
        }

        const graph = data.graph;
        const task = graph.nodes && graph.nodes.length > 0 ? graph.nodes[0] : {};

        // 显示元数据
        const taskMeta = document.getElementById('taskMeta');
        if (taskMeta) {
            taskMeta.innerHTML = `
                <div class="meta-item">
                    <span class="meta-label">任务ID</span>
                    <span class="meta-value">${taskId}</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">状态</span>
                    <span class="meta-value">${getStatusText(task.status)}</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">进度</span>
                    <span class="meta-value">${(task.progress || 0).toFixed(0)}%</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">创建时间</span>
                    <span class="meta-value">${formatTime(summary?.created_at)}</span>
                </div>
            `;
        }

        // 构建内容
        let content = `## 任务描述\n\n${task.title || ''}\n\n`;

        // 子节点
        if (graph.nodes && graph.nodes.length > 1) {
            content += `## 子节点(${graph.nodes.length - 1})\n\n`;
            graph.nodes.slice(1).forEach((node, i) => {
                const icon = getStatusIcon(node.status);
                content += `${i + 1}. ${icon} **${node.title || ''}**\n`;
            });
        }

        // 日志
        const logs = data.logs || [];
        if (logs.length > 0) {
            content += `\n## 执行日志\n\n`;
            content += `| 时间 | 消息 |\n|------|------|\n`;
            logs.slice(-10).forEach(log => {
                const time = log.time ? new Date(log.time).toLocaleTimeString('zh-CN') : '';
                content += `| ${time} | ${escapeHtml(log.message)} |\n`;
            });
            if (logs.length > 10) {
                content += `\n*... 共 ${logs.length} 条日志*`;
            }
        }

        // 渲染 markdown
        if (taskContent) {
            try {
                if (typeof marked !== 'undefined') {
                    taskContent.innerHTML = marked.parse(content);
                } else {
                    taskContent.innerHTML = `<pre>${escapeHtml(content)}</pre>`;
                }
            } catch (error) {
                console.error('渲染失败:', error);
                taskContent.innerHTML = `<pre>${escapeHtml(content)}</pre>`;
            }
        }
    } catch (error) {
        console.error('获取任务详情失败:', error);
        if (taskContent) {
            taskContent.innerHTML = `<p style="color: var(--danger);">获取任务详情失败</p>`;
        }
    }
}

function closeModal() {
    const modal = document.getElementById('taskModal');
    if (modal) modal.classList.remove('show');
}

// ============================================================================
// Graph Visualization
// ============================================================================
async function viewTaskGraph(taskId) {
    currentGraphTaskId = taskId;
    try {
        const response = await fetch(`/api/agent/task/graph?id=${taskId}`);
        const data = await response.json();

        if (data.success) {
            currentGraphData = data.graph;
            currentLogs = data.logs || [];
            renderGraphModal(data.graph, data.logs);
            document.getElementById('graphModal').classList.add('show');
        } else {
            alert('获取任务图失败: ' + (data.error || '未知错误'));
        }
    } catch (error) {
        console.error('获取任务图失败:', error);
        alert('获取任务图失败');
    }
}

function renderGraphModal(graph, logs) {
    const graphTitle = document.getElementById('graphTitle');
    const graphStats = document.getElementById('graphStats');

    if (graphTitle) {
        graphTitle.textContent = graph.nodes[0]?.title || '任务执行图';
    }
    if (graphStats && graph.stats) {
        graphStats.innerHTML = `
            <span class="stat-badge done">${graph.stats.done_nodes} /${graph.stats.total_nodes} 完成</span>
            <span class="stat-badge">${graph.stats.progress.toFixed(0)}%</span>
        `;
    }

    renderMermaidGraph(graph);
    renderLogs(logs);

    if (typeof mermaid !== 'undefined') {
        mermaid.init(undefined, '.mermaid');
    }
}

function renderMermaidGraph(graph) {
    const container = document.getElementById('graphDiagram');
    if (!container) return;

    // 构建节点树结构
    const nodeMap = {};
    graph.nodes.forEach(node => nodeMap[node.id] = { ...node, children: [] });

    // 找出根节点和建立父子关系
    let rootId = null;
    graph.edges.forEach(edge => {
        if (edge.type === 'parent_child' && nodeMap[edge.from] && nodeMap[edge.to]) {
            nodeMap[edge.from].children.push(nodeMap[edge.to]);
            nodeMap[edge.to].parentId = edge.from;
        }
    });

    // 找根节点
    for (const id in nodeMap) {
        if (!nodeMap[id].parentId) {
            rootId = id;
            break;
        }
    }

    if (!rootId && graph.nodes.length > 0) {
        rootId = graph.nodes[0].id;
    }

    // 渲染树形视图
    const renderNode = (node, depth = 0) => {
        const hasChildren = node.children && node.children.length > 0;
        const isExpanded = depth < 2; // 默认展开前2层
        const icon = getStatusIcon(node.status);
        const statusClass = node.status || 'pending';
        const indent = depth * 20;

        let html = `
            <div class="tree-node" data-id="${node.id}" data-depth="${depth}">
                <div class="tree-node-header ${statusClass}" style="padding-left: ${indent + 12}px">
                    ${hasChildren ? `<span class="tree-toggle ${isExpanded ? 'expanded' : ''}" onclick="toggleTreeNode(event, '${node.id}')">
                        <i class="fas fa-chevron-right"></i>
                    </span>` : '<span class="tree-toggle-placeholder"></span>'}
                    <span class="tree-icon">${icon}</span>
                    <span class="tree-title" onclick="showNodeDetail('${node.id}')">${escapeHtml(node.title || '未命名')}</span>
                    <span class="tree-progress">${(node.progress || 0).toFixed(0)}%</span>
                    <span class="tree-status ${statusClass}">${getStatusText(node.status)}</span>
                </div>
                ${hasChildren ? `<div class="tree-children ${isExpanded ? 'show' : ''}" data-parent="${node.id}">
                    ${node.children.map(child => renderNode(child, depth + 1)).join('')}
                </div>` : ''}
            </div>
        `;
        return html;
    };

    // 统计信息
    const stats = graph.stats || { total_nodes: graph.nodes.length, done_nodes: 0, progress: 0 };

    container.innerHTML = `
        <div class="tree-view-container">
            <div class="tree-toolbar">
                <button class="tree-btn" onclick="expandAllNodes()"><i class="fas fa-expand-alt"></i> 展开全部</button>
                <button class="tree-btn" onclick="collapseAllNodes()"><i class="fas fa-compress-alt"></i> 收起全部</button>
                <span class="tree-stats">${stats.done_nodes || 0}/${stats.total_nodes || graph.nodes.length} 完成</span>
            </div>
            <div class="tree-content">
                ${rootId ? renderNode(nodeMap[rootId]) : '<p class="empty-logs">无节点数据</p>'}
            </div>
        </div>
    `;
}

// 树节点展开/收起
function toggleTreeNode(event, nodeId) {
    event.stopPropagation();
    const toggle = event.currentTarget;
    const children = document.querySelector(`.tree-children[data-parent="${nodeId}"]`);

    if (children) {
        toggle.classList.toggle('expanded');
        children.classList.toggle('show');
    }
}

// 展开所有节点
function expandAllNodes() {
    document.querySelectorAll('.tree-toggle').forEach(t => t.classList.add('expanded'));
    document.querySelectorAll('.tree-children').forEach(c => c.classList.add('show'));
}

// 收起所有节点
function collapseAllNodes() {
    document.querySelectorAll('.tree-toggle').forEach(t => t.classList.remove('expanded'));
    document.querySelectorAll('.tree-children').forEach(c => c.classList.remove('show'));
}

function renderLogs(logs) {
    const container = document.getElementById('graphLogs');
    if (!container) return;

    const filtered = logFilter === 'all'
        ? logs
        : logs.filter(l => l.level === logFilter);

    if (!filtered || filtered.length === 0) {
        container.innerHTML = '<p class="empty-logs">暂无执行日志</p>';
        return;
    }

    container.innerHTML = filtered.map(log => `
            <div class="log-entry ${log.level}" onclick = "showNodeDetail('${log.node_id}')" >
                <div class="log-header">
                    <span class="log-time">${formatLogTime(log.time)}</span>
                    <span class="log-phase">${log.phase || ''}</span>
                </div>
                <div class="log-message">${escapeHtml(log.message)}</div>
            </div>
            `).join('');
}

function filterLogs(level) {
    logFilter = level;
    document.querySelectorAll('.log-filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.level === level);
    });
    renderLogs(currentLogs);
}

function showNodeDetail(nodeId) {
    if (!currentGraphData) return;
    const node = currentGraphData.nodes.find(n => n.id === nodeId);
    if (!node) return;

    const detailPanel = document.getElementById('nodeDetailPanel');
    if (!detailPanel) return;

    detailPanel.innerHTML = `
            <div class="node-detail-header">
            <h4>${getStatusIcon(node.status)} ${escapeHtml(node.title)}</h4>
            <button class="close-detail" onclick="hideNodeDetail()">×</button>
        </div >
            <div class="node-detail-body">
                <div class="detail-item">
                    <label>状态</label>
                    <span class="task-status ${node.status}">${getStatusText(node.status)}</span>
                </div>
                <div class="detail-item">
                    <label>进度</label>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${node.progress || 0}%"></div>
                    </div>
                    <span>${(node.progress || 0).toFixed(0)}%</span>
                </div>
                <div class="detail-item">
                    <label>深度</label>
                    <span>第 ${(node.depth || 0) + 1} 层</span>
                </div>
                <div class="detail-item">
                    <label>执行模式</label>
                    <span>${node.execution_mode === 'parallel' ? '🔀 并行' : '➡️ 串行'}</span>
                </div>
                ${node.duration ? `
            <div class="detail-item">
                <label>耗时</label>
                <span>${node.duration}</span>
            </div>
            ` : ''}
            </div>
        `;
    detailPanel.classList.add('show');
}

function hideNodeDetail() {
    const panel = document.getElementById('nodeDetailPanel');
    if (panel) panel.classList.remove('show');
}

function closeGraphModal() {
    const modal = document.getElementById('graphModal');
    if (modal) modal.classList.remove('show');
    currentGraphTaskId = null;
    currentGraphData = null;
    hideNodeDetail();
}

// ============================================================================
// User Input Modal
// ============================================================================
function showInputModal(request) {
    currentInputRequest = request;
    inputValue = request.default || null;

    const inputTitle = document.getElementById('inputTitle');
    const inputMessage = document.getElementById('inputMessage');
    const formGroup = document.getElementById('inputFormGroup');
    const footer = document.getElementById('inputFooter');

    if (inputTitle) inputTitle.textContent = request.title || '请输入';
    if (inputMessage) inputMessage.textContent = request.message || '';

    if (!formGroup || !footer) return;

    switch (request.input_type) {
        case 'text':
        case 'password':
        case 'number':
            formGroup.innerHTML = `
            < input type = "${request.input_type}"
        class="input-text"
        id = "inputField"
        placeholder = "${request.placeholder || ''}"
        value = "${request.default || ''}"
        onchange = "inputValue = this.value" >
            `;
            footer.style.display = 'flex';
            break;

        case 'textarea':
            formGroup.innerHTML = `
            < textarea class="input-textarea"
        id = "inputField"
        placeholder = "${request.placeholder || ''}"
        onchange = "inputValue = this.value" > ${request.default || ''}</textarea >
            `;
            footer.style.display = 'flex';
            break;

        case 'select':
            formGroup.innerHTML = `
            < div class="input-options" >
                ${(request.options || []).map(opt => `
                        <label class="input-option ${opt.value === request.default ? 'selected' : ''}" onclick="selectOption(this, '${opt.value}')">
                            <div class="radio"></div>
                            <span>${opt.label}</span>
                        </label>
                    `).join('')
                }
                </div >
            `;
            footer.style.display = 'flex';
            break;

        case 'confirm':
            formGroup.innerHTML = `
            < div class="confirm-buttons" >
                    <button class="confirm-btn" onclick="submitConfirm(false)">否</button>
                    <button class="confirm-btn yes" onclick="submitConfirm(true)">是</button>
                </div >
            `;
            footer.style.display = 'none';
            break;

        default:
            formGroup.innerHTML = `
            < input type = "text"
        class="input-text"
        id = "inputField"
        placeholder = "${request.placeholder || ''}"
        value = "${request.default || ''}"
        onchange = "inputValue = this.value" >
            `;
            footer.style.display = 'flex';
    }

    const modal = document.getElementById('inputModal');
    if (modal) modal.classList.add('active');
}

function selectOption(el, value) {
    document.querySelectorAll('.input-option').forEach(opt => opt.classList.remove('selected'));
    el.classList.add('selected');
    inputValue = value;
}

function submitConfirm(value) {
    inputValue = value;
    submitInput();
}

async function submitInput() {
    if (!currentInputRequest) return;

    const inputField = document.getElementById('inputField');
    if (inputField) {
        inputValue = inputField.value;
    }

    try {
        const response = await fetch('/api/agent/task/input', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                request_id: currentInputRequest.id,
                task_id: currentInputRequest.task_id,
                node_id: currentInputRequest.node_id,
                value: inputValue,
                cancelled: false
            })
        });

        const data = await response.json();
        if (data.success) {
            closeInputModal();
            showToast('输入已提交', 'success');
        } else {
            showToast('提交失败: ' + (data.error || '未知错误'), 'error');
        }
    } catch (err) {
        showToast('提交失败: ' + err.message, 'error');
    }
}

async function cancelInput() {
    if (!currentInputRequest) {
        closeInputModal();
        return;
    }

    try {
        await fetch('/api/agent/task/input', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                request_id: currentInputRequest.id,
                task_id: currentInputRequest.task_id,
                node_id: currentInputRequest.node_id,
                value: null,
                cancelled: true
            })
        });
    } catch (err) {
        console.error('Cancel input error:', err);
    }

    closeInputModal();
    showToast('已跳过输入', 'info');
}

function closeInputModal() {
    const modal = document.getElementById('inputModal');
    if (modal) modal.classList.remove('active');
    currentInputRequest = null;
    inputValue = null;
}

function handleInputNotification(data) {
    if (data.type === 'input_required' && data.input) {
        showInputModal(data.input);
    }
}

// ============================================================================
// Utilities
// ============================================================================
function getStatusText(status) {
    const statusMap = {
        'pending': '待执行',
        'running': '执行中',
        'paused': '已暂停',
        'done': '已完成',
        'failed': '失败',
        'canceled': '已取消',
        'node_pending': '待执行',
        'node_running': '执行中',
        'node_done': '已完成',
        'node_failed': '失败',
        'node_paused': '已暂停',
        'node_skipped': '已跳过',
        'node_cancelled': '已取消',
        'node_waiting_input': '等待输入'
    };
    return statusMap[status] || status;
}

function getStatusIcon(status) {
    const icons = {
        'pending': '⏳',
        'running': '🔄',
        'paused': '⏸️',
        'done': '✅',
        'failed': '❌',
        'canceled': '🚫',
        'skipped': '⏭️',
        'node_pending': '⏳',
        'node_running': '🔄',
        'node_done': '✅',
        'node_failed': '❌',
        'node_paused': '⏸️',
        'node_skipped': '⏭️',
        'node_waiting_input': '❓'
    };
    return icons[status] || '❓';
}

function getActionButtons(task) {
    let buttons = '';
    const status = task.status || '';

    if (status === 'running' || status === 'node_running') {
        buttons = `
            <button class="action-btn pause" onclick="pauseTask('${task.id}')">
                <i class="fas fa-pause"></i> 暂停
            </button>
            <button class="action-btn cancel" onclick="cancelTask('${task.id}')">
                <i class="fas fa-times"></i> 取消
            </button>
        `;
    } else if (status === 'paused' || status === 'node_paused') {
        buttons = `
            <button class="action-btn resume" onclick="resumeTask('${task.id}')">
                <i class="fas fa-play"></i> 恢复
            </button>
            <button class="action-btn cancel" onclick="cancelTask('${task.id}')">
                <i class="fas fa-times"></i> 取消
            </button>
        `;
    } else if (status === 'pending' || status === 'node_pending') {
        buttons = `
            <button class="action-btn cancel" onclick="cancelTask('${task.id}')">
                <i class="fas fa-times"></i> 取消
            </button>
        `;
    }

    // 已完成的任务添加重试和删除按钮
    if (['failed', 'canceled', 'node_failed', 'node_cancelled'].includes(status)) {
        buttons += `
            <button class="action-btn retry" onclick="retryTask('${task.id}')" style="background: rgba(34, 197, 94, 0.2); color: #22c55e;">
                <i class="fas fa-redo"></i> 重试
            </button>
        `;
    }
    if (['done', 'failed', 'canceled', 'node_done', 'node_failed', 'node_cancelled'].includes(status)) {
        buttons += `
            <button class="action-btn delete" onclick="deleteTask('${task.id}')" style="background: rgba(239, 68, 68, 0.2); color: var(--danger);">
                <i class="fas fa-trash"></i> 删除
            </button>
        `;
    }
    return buttons;
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatTime(timeStr) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    return date.toLocaleString('zh-CN');
}

function formatLogTime(timeStr) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    return date.toLocaleTimeString('zh-CN');
}

function showToast(message, type) {
    if (window.AgentNotifier && window.AgentNotifier.showToast) {
        window.AgentNotifier.showToast(message);
    } else {
        console.log(`[${type}] ${message} `);
    }
}

// 节流函数：限制函数调用频率
function throttle(fn, delay) {
    let lastCall = 0;
    let timeout = null;
    return function (...args) {
        const now = Date.now();
        if (now - lastCall >= delay) {
            lastCall = now;
            fn.apply(this, args);
        } else if (!timeout) {
            // 确保最后一次调用会被执行
            timeout = setTimeout(() => {
                lastCall = Date.now();
                timeout = null;
                fn.apply(this, args);
            }, delay - (now - lastCall));
        }
    };
}
