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

            // 处理图表更新 - 只更新已打开的图表，不重新弹出
            if (notification.type && notification.type.startsWith('graph_') ||
                notification.type && notification.type.startsWith('node_')) {
                if (currentGraphTaskId === notification.task_id) {
                    updateGraphIfOpen(currentGraphTaskId);
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
        const taskProgress = task.progress || 0;
        const taskStatus = normalizeTaskStatus(task.status || 'pending', taskProgress);
        const taskCreatedAt = task.created_at;

        // 检查是否活跃运行中
        const isActive = currentActiveIds && currentActiveIds.includes(taskId) && !isTerminalStatus(taskStatus);

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
        const status = normalizeTaskStatus(task.status || 'pending', task.progress || 0);
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
let currentDetailGraphData = null; // 保存当前详情图数据

async function viewTaskDetail(taskId) {
    const summary = currentTasks.find(t => t.id === taskId);
    const modalTitle = document.getElementById('modalTitle');
    if (modalTitle) modalTitle.textContent = summary?.title || '任务详情';

    const modal = document.getElementById('taskModal');
    if (modal) modal.classList.add('show');

    const taskContent = document.getElementById('taskContent');
    if (taskContent) {
        taskContent.innerHTML = '<p style="color: var(--text-muted);">加载中...</p>';
    }

    try {
        const response = await fetch(`/api/agent/task/graph?id=${taskId}`);
        const data = await response.json();

        if (!data.success) {
            if (taskContent) {
                taskContent.innerHTML = `<p style="color: var(--danger);">加载失败: ${data.error || '未知错误'}</p>`;
            }
            return;
        }

        currentDetailGraphData = data;
        const graph = data.graph;
        const rootNode = graph.nodes && graph.nodes.length > 0 ? graph.nodes[0] : {};

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
                    <span class="meta-value">${getStatusText(rootNode.status)}</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">进度</span>
                    <span class="meta-value">${graph.stats?.progress?.toFixed(0) || 0}%</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">节点数</span>
                    <span class="meta-value">${graph.stats?.total_nodes || 0}</span>
                </div>
            `;
        }

        // 构建树结构
        const nodeMap = {};
        const nodes = graph.nodes || [];
        nodes.forEach(n => nodeMap[n.id] = { ...n, children: [] });

        let rootNodes = [];
        nodes.forEach(n => {
            if (n.parent_id && nodeMap[n.parent_id]) {
                nodeMap[n.parent_id].children.push(nodeMap[n.id]);
            } else if (!n.parent_id || n.id === taskId) {
                rootNodes.push(nodeMap[n.id]);
            }
        });

        // 渲染HTML
        let html = '<div class="task-detail-container">';

        // 任务树
        html += '<div class="task-tree-section">';
        html += '<h3>📊 任务结构</h3>';
        html += '<div class="task-tree">';
        rootNodes.forEach(node => {
            html += renderTaskTreeNode(node, 0);
        });
        html += '</div></div>';

        // 提示区
        html += '<div class="llm-hint-section">';
        html += '<p class="hint">💡 点击节点查看 LLM 交互详情（弹窗显示）</p>';
        html += '</div>';

        html += '</div>';

        // 添加样式
        html += `<style>
            .task-detail-container { display: flex; flex-direction: column; gap: 20px; }
            .task-tree-section, .llm-context-panel { background: rgba(0,0,0,0.3); border-radius: 8px; padding: 15px; }
            .task-tree-section h3, .llm-context-panel h3 { margin: 0 0 10px 0; font-size: 1rem; color: var(--primary); }
            .task-tree { font-family: monospace; font-size: 0.9rem; }
            .tree-node { margin: 4px 0; cursor: pointer; padding: 4px 8px; border-radius: 4px; transition: background 0.2s; }
            .tree-node:hover { background: rgba(99, 102, 241, 0.2); }
            .tree-node.selected { background: rgba(99, 102, 241, 0.4); }
            .tree-indent { display: inline-block; }
            .tree-connector { color: var(--text-muted); margin-right: 6px; }
            .mode-badge { font-size: 0.7rem; padding: 2px 6px; border-radius: 10px; margin-left: 6px; }
            .mode-parallel { background: rgba(168, 85, 247, 0.3); color: #a855f7; }
            .mode-sequential { background: rgba(34, 197, 94, 0.3); color: #22c55e; }
            .status-badge { font-size: 0.8rem; margin-right: 6px; }
            .llm-context-panel { max-height: 400px; overflow-y: auto; }
            .llm-context-panel .hint { color: var(--text-muted); font-style: italic; }
            .llm-item { margin: 10px 0; padding: 10px; background: rgba(0,0,0,0.2); border-radius: 6px; border-left: 3px solid var(--primary); }
            .llm-item-header { font-weight: bold; margin-bottom: 8px; display: flex; justify-content: space-between; }
            .llm-item-phase { padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; }
            .llm-item-phase.planning { background: rgba(59, 130, 246, 0.3); }
            .llm-item-phase.execution { background: rgba(245, 158, 11, 0.3); }
            .llm-code { background: rgba(0,0,0,0.3); padding: 8px; border-radius: 4px; white-space: pre-wrap; word-break: break-all; font-size: 0.8rem; max-height: 200px; overflow-y: auto; }
        </style>`;

        if (taskContent) {
            taskContent.innerHTML = html;
        }
    } catch (error) {
        console.error('获取任务详情失败:', error);
        if (taskContent) {
            taskContent.innerHTML = `<p style="color: var(--danger);">获取任务详情失败</p>`;
        }
    }
}

// 渲染任务树节点
function renderTaskTreeNode(node, depth) {
    const indent = '&nbsp;&nbsp;&nbsp;&nbsp;'.repeat(depth);
    const connector = depth > 0 ? (node.execution_mode === 'parallel' ? '├─' : '├─') : '';
    const icon = getStatusIcon(node.status);
    const modeText = node.has_children ?
        (node.execution_mode === 'parallel' ?
            '<span class="mode-badge mode-parallel">⇉ 并行</span>' :
            '<span class="mode-badge mode-sequential">→ 串行</span>') : '';

    let html = `<div class="tree-node" onclick="showNodeLLMContext('${node.id}')" data-nodeid="${node.id}">`;
    html += `<span class="tree-indent">${indent}</span>`;
    html += `<span class="tree-connector">${connector}</span>`;
    html += `<span class="status-badge">${icon}</span>`;
    html += `<span class="tree-title">${escapeHtml(node.title || node.id)}</span>`;
    html += modeText;
    if (node.duration) html += ` <span style="color: var(--text-muted); font-size: 0.8rem;">(${node.duration})</span>`;
    html += '</div>';

    if (node.children && node.children.length > 0) {
        node.children.forEach(child => {
            html += renderTaskTreeNode(child, depth + 1);
        });
    }
    return html;
}

// 显示节点的 LLM 上下文 (弹出窗口)
function showNodeLLMContext(nodeId) {
    if (!currentDetailGraphData) return;

    // 高亮选中节点
    document.querySelectorAll('.tree-node').forEach(el => el.classList.remove('selected'));
    const selectedNode = document.querySelector(`.tree-node[data-nodeid="${nodeId}"]`);
    if (selectedNode) selectedNode.classList.add('selected');

    // 查找节点
    const nodes = currentDetailGraphData.graph.nodes || [];
    const node = nodes.find(n => n.id === nodeId);

    if (!node) {
        alert('未找到节点');
        return;
    }

    // 创建弹窗
    let popup = document.getElementById('llmContextPopup');
    if (!popup) {
        popup = document.createElement('div');
        popup.id = 'llmContextPopup';
        popup.innerHTML = `
            <div class="llm-popup-overlay" onclick="closeLLMPopup()"></div>
            <div class="llm-popup-content">
                <div class="llm-popup-header">
                    <h3 id="llmPopupTitle">💬 LLM 交互记录</h3>
                    <button class="llm-popup-close" onclick="closeLLMPopup()">✕</button>
                </div>
                <div id="llmPopupBody" class="llm-popup-body"></div>
            </div>
        `;
        popup.innerHTML += `<style>
            #llmContextPopup { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 10000; display: flex; align-items: center; justify-content: center; }
            .llm-popup-overlay { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.7); }
            .llm-popup-content { position: relative; background: var(--card-bg, #1e1e2e); border-radius: 12px; width: 90%; max-width: 900px; max-height: 85vh; display: flex; flex-direction: column; box-shadow: 0 20px 60px rgba(0,0,0,0.5); }
            .llm-popup-header { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid rgba(255,255,255,0.1); }
            .llm-popup-header h3 { margin: 0; color: var(--primary, #6366f1); font-size: 1.1rem; }
            .llm-popup-close { background: none; border: none; color: var(--text-muted, #888); font-size: 1.5rem; cursor: pointer; padding: 0; line-height: 1; }
            .llm-popup-close:hover { color: var(--danger, #ef4444); }
            .llm-popup-body { padding: 20px; overflow-y: auto; flex: 1; }
            .llm-popup-item { margin-bottom: 20px; padding: 15px; background: rgba(0,0,0,0.3); border-radius: 8px; border-left: 4px solid var(--primary, #6366f1); }
            .llm-popup-item-header { display: flex; justify-content: space-between; margin-bottom: 12px; font-weight: bold; }
            .llm-popup-phase { padding: 4px 10px; border-radius: 4px; font-size: 0.85rem; }
            .llm-popup-phase.planning { background: rgba(59, 130, 246, 0.3); color: #60a5fa; }
            .llm-popup-phase.execution { background: rgba(245, 158, 11, 0.3); color: #fbbf24; }
            .llm-popup-section { margin: 10px 0; }
            .llm-popup-section summary { cursor: pointer; padding: 8px 12px; background: rgba(0,0,0,0.2); border-radius: 6px; font-weight: 500; }
            .llm-popup-section summary:hover { background: rgba(99, 102, 241, 0.2); }
            .llm-popup-code { background: rgba(0,0,0,0.4); padding: 12px; border-radius: 6px; white-space: pre-wrap; word-break: break-word; font-family: monospace; font-size: 0.85rem; margin-top: 8px; line-height: 1.5; }
            .llm-no-history { color: var(--text-muted, #888); font-style: italic; text-align: center; padding: 40px; }
            .llm-popup-result { background: rgba(34, 197, 94, 0.1); border: 1px solid rgba(34, 197, 94, 0.3); border-radius: 8px; padding: 15px; margin-bottom: 15px; }
            .llm-popup-result h4 { margin: 0 0 10px 0; color: #22c55e; }
            .result-status { font-size: 0.9rem; margin-bottom: 8px; }
            .result-status.success { color: #22c55e; }
            .result-status.failed { color: #ef4444; }
            .result-summary { margin: 10px 0; padding: 8px; background: rgba(0,0,0,0.2); border-radius: 4px; }
            .result-error { margin: 10px 0; padding: 8px; background: rgba(239, 68, 68, 0.2); border-radius: 4px; color: #ef4444; }
            .llm-popup-phase.synthesis { background: rgba(34, 197, 94, 0.3); color: #22c55e; }
            .llm-tool-calls { margin: 10px 0; padding: 10px; background: rgba(99, 102, 241, 0.1); border: 1px solid rgba(99, 102, 241, 0.3); border-radius: 6px; }
            .tool-calls-header { font-weight: bold; color: var(--primary, #6366f1); margin-bottom: 8px; }
            .tool-call-item { margin: 8px 0; padding: 8px; background: rgba(0,0,0,0.2); border-radius: 4px; }
            .tool-call-name { font-weight: 500; margin-bottom: 6px; }
            .tool-call-error { color: #ef4444; margin-top: 4px; font-size: 0.9rem; }
            .llm-popup-code { position: relative; padding-top: 24px; } /* Make room for button */
            .copy-btn {
                position: absolute;
                top: 4px;
                right: 4px;
                background: rgba(255, 255, 255, 0.1);
                border: 1px solid rgba(255, 255, 255, 0.2);
                border-radius: 4px;
                color: var(--text-muted, #ccc);
                cursor: pointer;
                padding: 2px 8px;
                font-size: 0.7rem;
                transition: all 0.2s;
                z-index: 10;
            }
            .copy-btn:hover {
                background: rgba(255, 255, 255, 0.2);
                color: var(--text-normal, #fff);
            }
        </style>`;
        document.body.appendChild(popup);
    }

    // 填充内容
    document.getElementById('llmPopupTitle').textContent = `💬 ${node.title || node.id}`;

    const body = document.getElementById('llmPopupBody');
    let html = '';

    // 显示任务结果（子节点汇总数据）
    if (node.result) {
        html += '<div class="llm-popup-result">';
        html += `<h4>📊 任务结果</h4>`;
        html += `<div class="result-status ${node.result.success ? 'success' : 'failed'}">${node.result.success ? '✅ 成功' : '❌ 失败'}</div>`;

        // 显示 LLM 整合后的摘要
        if (node.result.summary) {
            html += `<div class="result-section"><strong>🤖 LLM整合摘要:</strong><div class="result-summary">${escapeHtml(node.result.summary)}</div></div>`;
        }

        // 显示原始摘要（整合前）
        if (node.result.raw_summary && node.result.raw_summary !== node.result.summary) {
            html += `<details class="llm-popup-section"><summary>📝 原始摘要（整合前）</summary><div class="llm-popup-code"><button class="copy-btn" onclick="copyCode(this)">复制</button>${escapeHtml(node.result.raw_summary)}</div></details>`;
        }

        // 显示详细输出
        if (node.result.output) {
            html += `<details class="llm-popup-section"><summary>📋 详细输出 (${node.result.output.length} 字符)</summary><div class="llm-popup-code"><button class="copy-btn" onclick="copyCode(this)">复制</button>${escapeHtml(node.result.output)}</div></details>`;
        }
        if (node.result.error) {
            html += `<div class="result-error"><strong>错误:</strong> ${escapeHtml(node.result.error)}</div>`;
        }
        html += '</div>';
    }

    // LLM 交互历史
    if (!node.llm_history || node.llm_history.length === 0) {
        if (!node.result) {
            html += '<p class="llm-no-history">此节点无数据</p>';
        }
    } else {
        html += '<h4 style="margin-top: 20px;">💬 LLM 交互历史</h4>';
        node.llm_history.forEach((item, idx) => {
            const phaseClass = item.phase === 'planning' ? 'planning' : (item.phase === 'synthesis' ? 'synthesis' : 'execution');
            const phaseText = item.phase === 'planning' ? '📋 规划' : (item.phase === 'synthesis' ? '🔄 整合' : '⚡ 执行');
            const time = item.timestamp ? new Date(item.timestamp).toLocaleTimeString('zh-CN') : '';
            const duration = item.duration_ms ? `${item.duration_ms}ms` : '';

            html += `<div class="llm-popup-item">`;
            html += `<div class="llm-popup-item-header">`;
            html += `<span><span class="llm-popup-phase ${phaseClass}">${phaseText}</span> 交互 #${idx + 1}</span>`;
            html += `<span style="color: var(--text-muted);">${time} ${duration}</span>`;
            html += `</div>`;
            html += `</div>`;
            html += `<details class="llm-popup-section"><summary>📤 请求 (${(item.request || '').length} 字符)</summary><div class="llm-popup-code"><button class="copy-btn" onclick="copyCode(this)">复制</button>${escapeHtml(item.request || '')}</div></details>`;

            // 显示工具调用
            if (item.tool_calls && item.tool_calls.length > 0) {
                html += `<div class="llm-tool-calls">`;
                html += `<div class="tool-calls-header">🔧 工具调用 (${item.tool_calls.length})</div>`;
                item.tool_calls.forEach((tc, tcIdx) => {
                    const statusIcon = tc.success ? '✅' : '❌';
                    html += `<div class="tool-call-item">`;
                    html += `<div class="tool-call-name">${statusIcon} ${escapeHtml(tc.name || '未知工具')}</div>`;
                    if (tc.arguments) {
                        html += `<details class="llm-popup-section"><summary>参数</summary><div class="llm-popup-code"><button class="copy-btn" onclick="copyCode(this)">复制</button>${escapeHtml(JSON.stringify(tc.arguments, null, 2))}</div></details>`;
                    }
                    if (tc.result) {
                        html += `<details class="llm-popup-section"><summary>结果</summary><div class="llm-popup-code"><button class="copy-btn" onclick="copyCode(this)">复制</button>${escapeHtml(typeof tc.result === 'string' ? tc.result : JSON.stringify(tc.result, null, 2))}</div></details>`;
                    }
                    if (tc.error) {
                        html += `<div class="tool-call-error">错误: ${escapeHtml(tc.error)}</div>`;
                    }
                    html += `</div>`;
                });
                html += `</div>`;
            }

            html += `<details class="llm-popup-section"><summary>📥 响应 (${(item.response || '').length} 字符)</summary><div class="llm-popup-code"><button class="copy-btn" onclick="copyCode(this)">复制</button>${escapeHtml(item.response || '')}</div></details>`;
            html += `</div>`;
        });
    }

    body.innerHTML = html;

    popup.style.display = 'flex';
}

// 复制代码功能
function copyCode(btn) {
    const codeBlock = btn.parentElement;
    // 获取文本内容，排除按钮本身的文本
    // 这里我们假设按钮是第一个子元素，且文本内容紧随其后
    // 更稳健的方法是遍历子节点，提取文本节点
    let text = '';
    codeBlock.childNodes.forEach(node => {
        if (node.nodeType === Node.TEXT_NODE) {
            text += node.textContent;
        }
    });

    navigator.clipboard.writeText(text).then(() => {
        const originalText = btn.textContent;
        btn.textContent = '已复制!';
        btn.style.color = '#4ade80'; // green-400
        setTimeout(() => {
            btn.textContent = originalText;
            btn.style.color = '';
        }, 2000);
    }).catch(err => {
        console.error('复制失败:', err);
        btn.textContent = '失败';
        setTimeout(() => btn.textContent = '复制', 2000);
    });
}

function closeLLMPopup() {
    const popup = document.getElementById('llmContextPopup');
    if (popup) popup.style.display = 'none';
}

function closeModal() {
    const modal = document.getElementById('taskModal');
    if (modal) modal.classList.remove('show');
}

// ============================================================================
// Graph Visualization
// ============================================================================
// 仅更新已打开的图表，不重新弹出
async function updateGraphIfOpen(taskId) {
    const graphModal = document.getElementById('graphModal');
    // 只有弹窗已经显示时才更新
    if (!graphModal || !graphModal.classList.contains('show')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/agent/task/graph?id=${taskId}`);
        const data = await response.json();

        if (data.success) {
            currentGraphData = data.graph;
            currentLogs = data.logs || [];
            renderGraphModal(data.graph, data.logs);
            // 不调用 classList.add('show')，避免重复触发弹窗
        }
    } catch (error) {
        console.error('更新任务图失败:', error);
    }
}

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
    status = normalizeStatus(status);
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
    status = normalizeStatus(status);
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
    const status = normalizeStatus(task.status || '');

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

function normalizeStatus(status) {
    const statusMap = {
        'queued': 'pending',
        'in_progress': 'running',
        'async': 'running',
        'completed': 'done',
        'success': 'done',
        'error': 'failed',
        'cancelled': 'canceled',
        'stopped': 'canceled',
        'node_in_progress': 'node_running',
        'node_completed': 'node_done',
        'node_error': 'node_failed'
    };
    return statusMap[status] || status;
}

function normalizeTaskStatus(status, progress) {
    const normalized = normalizeStatus(status);
    if ((normalized === 'running' || normalized === 'node_running') && Number(progress || 0) >= 100) {
        return normalized === 'node_running' ? 'node_done' : 'done';
    }
    return normalized;
}

function isTerminalStatus(status) {
    return ['done', 'failed', 'canceled', 'node_done', 'node_failed', 'node_cancelled'].includes(normalizeStatus(status));
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
