/**
 * 任务拆解追踪页面JavaScript
 */

class TaskBreakdownApp {
    constructor() {
        this.currentTask = null;
        this.tasks = [];
        this.charts = {};

        // 获取根任务ID（从URL参数或data属性）
        this.rootTaskID = this.getRootTaskID();

        this.init();
    }

    init() {
        // DOM元素
        this.elements = {
            taskTree: document.getElementById('taskTree'),
            taskDetailsCard: document.getElementById('taskDetailsCard'),
            taskTitle: document.getElementById('taskTitle'),
            taskStatus: document.getElementById('taskStatus'),
            taskPriority: document.getElementById('taskPriority'),
            taskProgress: document.getElementById('taskProgress'),
            taskDescription: document.getElementById('taskDescription'),
            taskStartDate: document.getElementById('taskStartDate'),
            taskEndDate: document.getElementById('taskEndDate'),
            taskEstimatedTime: document.getElementById('taskEstimatedTime'),
            taskActualTime: document.getElementById('taskActualTime'),
            taskTags: document.getElementById('taskTags'),
            addRootTask: document.getElementById('addRootTask'),
            editTask: document.getElementById('editTask'),
            deleteTask: document.getElementById('deleteTask'),
            addSubtask: document.getElementById('addSubtask'),
            refreshBtn: document.getElementById('refreshBtn'),
            tabLinks: document.querySelectorAll('.tab-link'),
            taskModal: document.getElementById('taskModal'),
            modalTitle: document.getElementById('modalTitle'),
            taskForm: document.getElementById('taskForm'),
            taskId: document.getElementById('taskId'),
            parentId: document.getElementById('parentId'),
            title: document.getElementById('title'),
            description: document.getElementById('description'),
            status: document.getElementById('status'),
            priority: document.getElementById('priority'),
            startDate: document.getElementById('startDate'),
            endDate: document.getElementById('endDate'),
            estimatedTime: document.getElementById('estimatedTime'),
            progress: document.getElementById('progress'),
            progressValue: document.getElementById('progressValue'),
            tags: document.getElementById('tags'),
            saveTask: document.getElementById('saveTask'),
            closeButtons: document.querySelectorAll('.close'),
            totalTasks: document.getElementById('totalTasks'),
            completedTasks: document.getElementById('completedTasks'),
            inProgressTasks: document.getElementById('inProgressTasks'),
            blockedTasks: document.getElementById('blockedTasks'),
            totalTime: document.getElementById('totalTime'),
            timelineChart: document.getElementById('timelineChart'),
            statusChart: document.getElementById('statusChart'),
            priorityChart: document.getElementById('priorityChart'),
            timeRangeSelect: document.getElementById('timeRangeSelect'),
            refreshTrendsBtn: document.getElementById('refreshTrendsBtn'),
            creationTrendChart: document.getElementById('creationTrendChart'),
            completionTrendChart: document.getElementById('completionTrendChart'),
            progressTrendChart: document.getElementById('progressTrendChart'),
            rootTaskBreadcrumb: document.getElementById('rootTaskBreadcrumb'),
            rootTaskTitle: document.getElementById('rootTaskTitle'),
            statusFilter: document.getElementById('statusFilter'),
            rootTasksPanel: document.getElementById('rootTasksPanel'),
            rootTasksList: document.getElementById('rootTasksList')
        };

        // 调试：检查关键元素是否存在
        console.log('TaskBreakdownApp初始化...');
        console.log('addRootTask元素:', this.elements.addRootTask);
        console.log('saveTask元素:', this.elements.saveTask);
        console.log('taskModal元素:', this.elements.taskModal);

        // 事件监听
        this.bindEvents();

        // 初始化日期为今天
        const today = new Date().toISOString().split('T')[0];
        this.elements.startDate.value = today;
        this.elements.endDate.value = today;

        // 进度滑块事件
        this.elements.progress.addEventListener('input', (e) => {
            this.elements.progressValue.textContent = `${e.target.value}%`;
        });

        // 加载数据
        this.loadData();

        // 加载趋势数据（延迟加载确保图表容器已准备好）
        this.loadInitialTrendsData();
    }

    bindEvents() {
        console.log('绑定事件...');

        // 按钮事件
        if (this.elements.addRootTask) {
            this.elements.addRootTask.addEventListener('click', () => {
                console.log('点击添加根任务按钮');
                this.openModal('add');
            });
        } else {
            console.error('addRootTask元素未找到');
        }

        if (this.elements.saveTask) {
            this.elements.saveTask.addEventListener('click', () => {
                console.log('点击保存按钮');
                this.saveTask();
            });
        } else {
            console.error('saveTask元素未找到');
        }

        if (this.elements.editTask) {
            this.elements.editTask.addEventListener('click', () => this.openModal('edit'));
        }

        if (this.elements.deleteTask) {
            this.elements.deleteTask.addEventListener('click', () => this.deleteCurrentTask());
        }

        if (this.elements.addSubtask) {
            this.elements.addSubtask.addEventListener('click', () => this.openModal('addSubtask'));
        }

        // 选项卡切换事件
        if (this.elements.tabLinks && this.elements.tabLinks.length > 0) {
            this.elements.tabLinks.forEach(tabLink => {
                tabLink.addEventListener('click', (e) => this.switchTab(e));
            });
        }

        if (this.elements.refreshBtn) {
            this.elements.refreshBtn.addEventListener('click', () => this.loadData());
        }

        if (this.elements.refreshTrendsBtn) {
            this.elements.refreshTrendsBtn.addEventListener('click', () => this.loadTrendsData());
        }

        if (this.elements.timeRangeSelect) {
            this.elements.timeRangeSelect.addEventListener('change', () => this.loadTrendsData());
        }

        // 状态过滤事件
        if (this.elements.statusFilter) {
            this.elements.statusFilter.addEventListener('change', () => this.applyStatusFilter());
        }

        // 模态框关闭
        this.elements.closeButtons.forEach(btn => {
            btn.addEventListener('click', () => this.closeModal());
        });

        // 点击模态框外部关闭
        window.addEventListener('click', (e) => {
            if (e.target === this.elements.taskModal) {
                this.closeModal();
            }
        });
    }

    switchTab(event) {
        const tabLink = event.currentTarget;
        const tabId = tabLink.getAttribute('data-tab');

        // 移除所有选项卡和窗格的激活状态
        this.elements.tabLinks.forEach(link => link.classList.remove('active'));
        document.querySelectorAll('.tab-pane').forEach(pane => {
            pane.classList.remove('active');
        });

        // 为点击的选项卡和对应的窗格添加激活状态
        tabLink.classList.add('active');
        const tabPane = document.getElementById(tabId);
        if (tabPane) {
            tabPane.classList.add('active');
        }

        // 如果需要，加载选项卡数据
        if (tabId === 'statsTab') {
            // 如果统计图表尚未渲染，重新加载数据以渲染图表
            if (!this.charts.status) {
                this.loadData(); // 这会加载统计数据并渲染图表
            }
        } else if (tabId === 'trendsTab') {
            // 加载时间趋势数据
            this.loadTrendsData();
        } else if (tabId === 'timelineTab') {
            // 如果时间线数据尚未加载，重新加载数据
            if (!this.timelineTasks || this.timelineTasks.length === 0) {
                this.loadData(); // 这会加载时间线数据并渲染
            }
        }
        // 任务树选项卡不需要特殊处理，数据已在loadData中加载
    }

    async loadData() {
        try {
            console.log('开始加载数据...');
            // 显示加载状态
            this.showLoading();

            // 并行加载所有数据
            const [tasks, stats, timeline] = await Promise.all([
                this.fetchTasks(),
                this.fetchStatistics(),
                this.fetchTimeline()
            ]);

            console.log('任务数据加载完成，数量:', tasks.length);
            console.log('任务数据示例（完整）:', tasks.length > 0 ? tasks[0] : '无任务');
            console.log('所有任务ID:', tasks.map(t => t.id || t.ID || t.Id || '无ID'));
            console.log('统计数据:', stats);

            this.tasks = tasks;
            console.log('设置this.tasks完成，长度:', this.tasks.length);

            // 渲染任务树
            console.log('开始渲染任务树...');
            this.renderTaskTree();
            console.log('任务树渲染完成');

            // 渲染根任务列表
            console.log('开始渲染根任务列表...');
            this.renderRootTasksList(tasks);
            console.log('根任务列表渲染完成');

            // 如果有根任务ID，自动选中该任务
            if (this.rootTaskID) {
                console.log(`尝试选中根任务: ${this.rootTaskID}`);
                const rootTask = this.findTaskById(this.rootTaskID);
                if (rootTask) {
                    console.log('找到根任务，自动选中:', rootTask);
                    this.selectTask(rootTask);

                    // 更新面包屑导航
                    if (this.elements.rootTaskBreadcrumb && this.elements.rootTaskTitle) {
                        this.elements.rootTaskBreadcrumb.style.display = 'flex';
                        const taskTitle = rootTask.title || rootTask.Title || '未命名任务';
                        this.elements.rootTaskTitle.textContent = taskTitle;
                    }
                } else {
                    console.log('未找到根任务，可能ID无效或数据未加载');
                    // 仍然显示面包屑，但显示未知任务
                    if (this.elements.rootTaskBreadcrumb && this.elements.rootTaskTitle) {
                        this.elements.rootTaskBreadcrumb.style.display = 'flex';
                        this.elements.rootTaskTitle.textContent = '未知任务';
                    }
                }
            }

            console.log('更新统计数据...');
            this.updateStatistics(stats);

            console.log('渲染图表...');
            this.renderCharts(stats);

            console.log('渲染时间线...');
            this.renderTimeline(timeline);

            // 如果有当前选中的任务，更新详情
            if (this.currentTask) {
                const task = this.findTaskById(this.currentTask.id);
                if (task) {
                    this.showTaskDetails(task);
                }
            }

            console.log('数据加载和渲染完成');

        } catch (error) {
            console.error('加载数据失败:', error);
            this.showError('加载数据失败，请刷新重试');
        } finally {
            this.hideLoading();
        }
    }

    async fetchTasks() {
        console.log('开始获取任务数据...');

        let url = '/api/tasks';
        if (this.rootTaskID) {
            console.log(`根任务ID: ${this.rootTaskID}, 获取子树`);
            url = `/api/tasks/subtasks?parent_id=${encodeURIComponent(this.rootTaskID)}`;
        }

        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        console.log('原始API响应:', data);
        let tasks = data.data || [];

        // 如果返回的是树形结构（单个任务对象），将其扁平化
        if (tasks && !Array.isArray(tasks)) {
            tasks = this.flattenTaskTree(tasks);
        }

        console.log('解析后的任务数据:', tasks);
        if (tasks.length > 0) {
            console.log('第一个任务的字段:', Object.keys(tasks[0]));
        }
        return tasks;
    }

    flattenTaskTree(taskTree, parentId = '') {
        const flatTasks = [];

        // 复制任务对象，添加parent_id（如果提供了）
        const taskCopy = { ...taskTree };
        if (parentId) {
            taskCopy.parent_id = parentId;
        }

        flatTasks.push(taskCopy);

        // 递归处理子任务
        if (taskTree.subtasks && Array.isArray(taskTree.subtasks)) {
            for (const subtask of taskTree.subtasks) {
                const subtaskFlat = this.flattenTaskTree(subtask, taskTree.id || taskTree.ID || taskTree.Id || '');
                flatTasks.push(...subtaskFlat);
            }
        }

        // 也处理 Subtasks 字段（大写）
        if (taskTree.Subtasks && Array.isArray(taskTree.Subtasks)) {
            for (const subtask of taskTree.Subtasks) {
                const subtaskFlat = this.flattenTaskTree(subtask, taskTree.id || taskTree.ID || taskTree.Id || '');
                flatTasks.push(...subtaskFlat);
            }
        }

        return flatTasks;
    }

    // 渲染根任务列表
    renderRootTasksList(tasks) {
        if (!this.elements.rootTasksList || !this.elements.rootTasksPanel) {
            return;
        }

        // 获取根任务（没有parent_id或parent_id为空）
        const rootTasks = tasks.filter(task => {
            const parentId = task.parent_id || task.parentId || task.parentID || '';
            return !parentId || parentId === '';
        });

        console.log('根任务数量:', rootTasks.length);

        // 如果没有根任务，隐藏面板
        if (rootTasks.length === 0) {
            this.elements.rootTasksPanel.style.display = 'none';
            return;
        }

        // 显示面板
        this.elements.rootTasksPanel.style.display = 'block';
        this.elements.rootTasksList.innerHTML = '';

        // 添加"所有任务"链接
        const allTasksItem = document.createElement('a');
        allTasksItem.href = '/taskbreakdown';
        allTasksItem.className = `root-task-item ${this.rootTaskID === '' ? 'active' : ''}`;
        allTasksItem.innerHTML = `
            <span class="root-task-status all"></span>
            <span>所有任务</span>
        `;
        allTasksItem.addEventListener('click', (e) => {
            e.preventDefault();
            window.location.href = '/taskbreakdown';
        });
        this.elements.rootTasksList.appendChild(allTasksItem);

        // 添加每个根任务
        rootTasks.forEach(task => {
            const taskId = task.id || task.ID || task.Id || '';
            const taskTitle = task.title || task.Title || '未命名任务';
            const taskStatus = task.status || task.Status || 'planning';

            const taskItem = document.createElement('a');
            taskItem.href = `/taskbreakdown?root=${encodeURIComponent(taskId)}`;
            taskItem.className = `root-task-item ${this.rootTaskID === taskId ? 'active' : ''}`;
            taskItem.innerHTML = `
                <span class="root-task-status ${taskStatus}"></span>
                <span>${taskTitle}</span>
            `;
            taskItem.addEventListener('click', (e) => {
                e.preventDefault();
                window.location.href = `/taskbreakdown?root=${encodeURIComponent(taskId)}`;
            });
            this.elements.rootTasksList.appendChild(taskItem);
        });
    }

    // 应用状态过滤
    applyStatusFilter() {
        if (!this.elements.statusFilter) {
            return;
        }

        const filterValue = this.elements.statusFilter.value;
        console.log('应用状态过滤:', filterValue);

        if (!filterValue) {
            // 重置过滤，显示所有任务
            this.renderTaskTree();
            return;
        }

        // 获取过滤后的任务
        const filteredTasks = this.tasks.filter(task => {
            const taskStatus = (task.status || task.Status || 'planning').toLowerCase();

            if (filterValue.includes(',')) {
                // 多个状态值（如"planning,in-progress,blocked"表示未完成）
                const allowedStatuses = filterValue.split(',').map(s => s.trim());
                return allowedStatuses.includes(taskStatus);
            } else {
                // 单个状态值
                return taskStatus === filterValue;
            }
        });

        // 使用过滤后的任务重新渲染任务树
        this.renderTaskTree(filteredTasks);
    }

    async fetchStatistics() {
        let url = '/api/tasks/statistics';
        if (this.rootTaskID) {
            url += `?root=${encodeURIComponent(this.rootTaskID)}`;
        }
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        return data.data || {};
    }

    async fetchTimeline() {
        let url = '/api/tasks/timeline';
        if (this.rootTaskID) {
            url += `?root=${encodeURIComponent(this.rootTaskID)}`;
        }
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        return data.data || { tasks: [] };
    }

    renderTaskTree(tasks = null) {
        const tasksToRender = tasks || this.tasks;
        console.log('=== 开始渲染任务树 ===');
        console.log('渲染任务总数:', tasksToRender.length);

        // 检查任务数据结构
        if (tasksToRender.length > 0) {
            const firstTask = tasksToRender[0];
            console.log('第一个任务的所有字段:', Object.keys(firstTask));
            console.log('第一个任务的parent_id字段值:', firstTask.parent_id);
            console.log('第一个任务的parentId字段值:', firstTask.parentId);
            console.log('第一个任务的parentID字段值:', firstTask.parentID);
            console.log('第一个任务的完整对象:', firstTask);
        }

        // 清空任务树
        this.elements.taskTree.innerHTML = '';

        // 重置已渲染任务记录
        this.renderedTasks = new Set();

        // 获取根任务 - 支持多种可能的字段名
        const rootTasks = tasksToRender.filter(task => {
            const taskId = task.id || task.ID || task.Id || '';
            const parentId = task.parent_id || task.parentId || task.parentID || '';
            console.log(`任务 ${taskId} 的parentId: "${parentId}"`);

            // 根任务的条件：
            // 1. parent_id 为空
            // 2. parent_id 等于自己的id（数据错误情况）
            // 3. parent_id 对应的任务不存在（孤立任务）
            if (!parentId || parentId === '') {
                console.log(`   -> 是根任务（parent_id为空）`);
                return true;
            }

            if (parentId === taskId) {
                console.log(`   -> 是根任务（parent_id等于自身id，数据错误）`);
                return true;
            }

            // 检查parent_id对应的任务是否存在
            const parentTask = tasksToRender.find(t => {
                const tId = t.id || t.ID || t.Id || '';
                return tId === parentId;
            });

            if (!parentTask) {
                console.log(`   -> 是根任务（父任务不存在，孤立任务）`);
                return true;
            }

            console.log(`   -> 不是根任务`);
            return false;
        });
        console.log('根任务数量:', rootTasks.length);
        console.log('根任务详情:', rootTasks);

        if (rootTasks.length === 0) {
            console.log('没有根任务，显示所有任务作为平铺列表');
            // 显示所有任务作为平铺列表
            tasksToRender.forEach((task, index) => {
                const taskTitle = task.title || task.Title || '无标题';
                console.log(`平铺渲染任务 ${index + 1}/${tasksToRender.length}: ${taskTitle}`);
                const rendered = this.renderTaskNode(task, this.elements.taskTree, 0);
                if (!rendered) {
                    console.log(`任务 ${taskTitle} 渲染失败或已跳过`);
                }
            });
            return;
        }

        // 按顺序排序 - 支持多种可能的字段名
        rootTasks.sort((a, b) => {
            const orderA = a.order || a.Order || 0;
            const orderB = b.order || b.Order || 0;
            console.log(`排序: 任务A order=${orderA}, 任务B order=${orderB}`);
            return orderA - orderB;
        });

        console.log('开始渲染根任务...');
        // 渲染根任务
        rootTasks.forEach((task, index) => {
            const taskTitle = task.title || task.Title || '无标题';
            console.log(`渲染根任务 ${index + 1}/${rootTasks.length}: ${taskTitle}`);
            const rendered = this.renderTaskNode(task, this.elements.taskTree, 0);
            if (!rendered) {
                console.log(`根任务 ${taskTitle} 渲染失败或已跳过`);
            }
        });

        // 初始化可排序
        this.initSortable();
        console.log('=== 任务树渲染完成 ===');
    }

    renderTaskNode(task, container, level, visited = new Set()) {
        // 支持多种可能的字段名
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || '未命名任务';
        const taskStatus = task.status || task.Status || 'planning';
        const taskPriority = task.priority || task.Priority || 3;
        const taskProgress = task.progress || task.Progress || 0;

        // 检查循环引用
        if (visited.has(taskId)) {
            console.error(`检测到循环引用！任务 ${taskId} (${taskTitle}) 已经在渲染路径中`);
            console.error('已访问的任务:', Array.from(visited));
            return null;
        }

        // 检查是否已经渲染过（避免重复渲染）
        if (this.renderedTasks && this.renderedTasks.has(taskId)) {
            console.log(`任务 ${taskId} (${taskTitle}) 已经渲染过，跳过`);
            return null;
        }

        console.log(`渲染任务: ${taskId} (${taskTitle}), 层级: ${level}`);

        const taskElement = document.createElement('div');
        taskElement.className = 'task-node';
        taskElement.dataset.taskId = taskId;
        taskElement.style.paddingLeft = `${level * 20 + 10}px`;

        // 获取子任务 - 支持多种可能的字段名
        const subtasks = this.tasks.filter(t => {
            const tParentId = t.parent_id || t.parentId || t.parentID || '';
            return tParentId === taskId && t !== task; // 避免将自己作为子任务
        });

        // 按顺序排序 - 支持多种可能的字段名
        subtasks.sort((a, b) => {
            const orderA = a.order || a.Order || 0;
            const orderB = b.order || b.Order || 0;
            return orderA - orderB;
        });

        // 状态和优先级样式
        const statusClass = `status-${taskStatus.replace('-', '')}`;
        const priorityClass = `priority-${taskPriority}`;

        // 构建HTML
        taskElement.innerHTML = `
            <div class="task-node-header">
                <div class="task-node-title">${this.escapeHtml(taskTitle)}</div>
                <div class="task-node-meta">
                    <span class="task-status-badge ${statusClass}">${this.getStatusText(taskStatus)}</span>
                    <span class="task-priority-badge ${priorityClass}">${this.getPriorityText(taskPriority)}</span>
                    <span class="task-progress-text">${taskProgress}%</span>
                </div>
            </div>
            <div class="task-progress-bar">
                <div class="task-progress-fill" style="width: ${taskProgress}%"></div>
            </div>
        `;

        // 点击事件
        taskElement.addEventListener('click', (e) => {
            // 阻止事件冒泡，避免父任务也处理这个点击事件
            e.stopPropagation();

            if (!e.target.closest('.task-node-meta')) {
                console.log(`点击任务元素，任务ID: ${taskId}, 任务标题: ${taskTitle}`);
                console.log('点击的任务对象:', task);

                // 从任务列表中重新查找任务，确保使用最新的数据
                const freshTask = this.findTaskById(taskId);
                if (freshTask) {
                    console.log('找到最新任务数据:', freshTask);
                    this.selectTask(freshTask);
                } else {
                    console.error(`未找到任务ID: ${taskId}，尝试从服务器获取`);
                    // 不再使用可能错误的原始task对象
                    // 而是尝试从服务器获取该任务
                    this.fetchTaskById(taskId).then(fetchedTask => {
                        if (fetchedTask) {
                            console.log('从服务器获取到任务:', fetchedTask);
                            this.selectTask(fetchedTask);
                        } else {
                            console.error(`无法获取任务ID: ${taskId}`);
                            this.showError(`无法找到任务: ${taskTitle}`);
                        }
                    }).catch(err => {
                        console.error(`获取任务失败: ${err}`);
                        this.showError(`获取任务失败: ${taskTitle}`);
                    });
                }
            }
        });

        container.appendChild(taskElement);

        // 标记为已渲染
        if (this.renderedTasks) {
            this.renderedTasks.add(taskId);
        }

        // 如果有子任务，递归渲染
        if (subtasks.length > 0) {
            console.log(`任务 ${taskId} 有 ${subtasks.length} 个子任务`);
            const subtasksContainer = document.createElement('div');
            subtasksContainer.className = 'task-subtasks';
            taskElement.appendChild(subtasksContainer);

            // 创建新的已访问集合，包含当前任务
            const newVisited = new Set(visited);
            newVisited.add(taskId);

            subtasks.forEach((subtask, index) => {
                const subtaskId = subtask.id || subtask.ID || subtask.Id || '';
                const subtaskTitle = subtask.title || subtask.Title || '无标题';
                console.log(`  渲染子任务 ${index + 1}/${subtasks.length}: ${subtaskId} (${subtaskTitle})`);

                const rendered = this.renderTaskNode(subtask, subtasksContainer, level + 1, newVisited);
                if (!rendered) {
                    console.log(`  子任务 ${subtaskId} 渲染失败或已跳过`);
                }
            });
        } else {
            console.log(`任务 ${taskId} 没有子任务`);
        }

        return taskElement;
    }

    initSortable() {
        // 初始化可排序功能
        const taskTree = this.elements.taskTree;
        Sortable.create(taskTree, {
            group: 'tasks',
            animation: 150,
            handle: '.task-node-header',
            onEnd: (evt) => {
                this.updateTaskOrder(evt.item.dataset.taskId, evt.newIndex);
            }
        });
    }

    async updateTaskOrder(taskId, newOrder) {
        try {
            console.log(`更新任务顺序: taskId=${taskId}, newOrder=${newOrder}`);

            const requestBody = { task_id: taskId, order: newOrder };
            console.log('请求体:', requestBody);

            const response = await fetch(`/api/tasks/order`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody)
            });

            console.log('响应状态:', response.status, response.statusText);

            if (!response.ok) {
                let errorMessage = `HTTP错误 ${response.status}: ${response.statusText}`;
                try {
                    const errorData = await response.json();
                    console.log('错误响应数据:', errorData);
                    errorMessage = errorData.error || errorData.message || errorMessage;
                } catch (e) {
                    console.log('无法解析错误响应:', e);
                }
                throw new Error(errorMessage);
            }

            const result = await response.json();
            console.log('更新成功:', result);

            // 重新加载数据
            await this.loadData();
        } catch (error) {
            console.error('更新任务顺序失败:', error);
            this.showError(`更新顺序失败: ${error.message}`);
        }
    }

    selectTask(task) {
        console.log('选择任务:', task);

        // 支持多种字段名
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || '无标题';
        console.log(`任务ID: ${taskId}, 任务标题: ${taskTitle}`);

        // 移除之前选中的样式
        document.querySelectorAll('.task-node.selected').forEach(el => {
            el.classList.remove('selected');
        });

        // 添加选中样式
        const taskElement = document.querySelector(`[data-task-id="${taskId}"]`);
        if (taskElement) {
            taskElement.classList.add('selected');
            console.log('找到任务元素，添加选中样式');
        } else {
            console.error(`未找到任务元素 data-task-id="${taskId}"`);
            // 尝试查找所有可能的task-id
            const allTaskElements = document.querySelectorAll('[data-task-id]');
            console.log('所有任务元素:', Array.from(allTaskElements).map(el => el.dataset.taskId));
        }

        // 显示任务详情
        this.showTaskDetails(task);
        this.currentTask = task;
        console.log('当前任务设置为:', this.currentTask);
    }

    showTaskDetails(task) {
        console.log('显示任务详情，传入的任务对象:', task);
        console.log('任务字段:', {
            id: task.id,
            title: task.title,
            status: task.status,
            priority: task.priority,
            progress: task.progress,
            description: task.description,
            start_date: task.start_date,
            end_date: task.end_date,
            estimated_time: task.estimated_time,
            actual_time: task.actual_time,
            parent_id: task.parent_id
        });

        this.elements.taskDetailsCard.style.display = 'block';

        // 支持多种字段名
        const taskTitle = task.title || task.Title || '无标题';
        const taskStatus = task.status || task.Status || 'planning';
        const taskPriority = task.priority || task.Priority || 3;
        const taskProgress = task.progress || task.Progress || 0;
        const taskDescription = task.description || task.Description || '暂无描述';
        const taskStartDate = task.start_date || task.startDate || task.StartDate || '-';
        const taskEndDate = task.end_date || task.endDate || task.EndDate || '-';
        const taskEstimatedTime = task.estimated_time || task.estimatedTime || task.EstimatedTime || 0;
        const taskActualTime = task.actual_time || task.actualTime || task.ActualTime || 0;

        // 更新任务详情
        this.elements.taskTitle.textContent = taskTitle;
        this.elements.taskStatus.textContent = this.getStatusText(taskStatus);
        this.elements.taskStatus.className = `task-status status-${taskStatus.replace('-', '')}`;

        this.elements.taskPriority.textContent = this.getPriorityText(taskPriority);
        this.elements.taskPriority.className = `task-priority priority-${taskPriority}`;

        this.elements.taskProgress.textContent = `${taskProgress}%`;
        this.elements.taskDescription.textContent = taskDescription;
        this.elements.taskStartDate.textContent = taskStartDate;
        this.elements.taskEndDate.textContent = taskEndDate;
        this.elements.taskEstimatedTime.textContent = `${taskEstimatedTime}分钟`;
        this.elements.taskActualTime.textContent = `${taskActualTime}分钟`;

        // 更新标签
        this.elements.taskTags.innerHTML = '';
        if (task.tags && task.tags.length > 0) {
            task.tags.forEach(tag => {
                const tagElement = document.createElement('span');
                tagElement.className = 'tag';
                tagElement.textContent = tag;
                this.elements.taskTags.appendChild(tagElement);
            });
        }
    }

    openModal(mode, parentTask = null) {
        console.log(`打开模态框，模式: ${mode}`);

        if (!this.elements.taskModal) {
            console.error('taskModal元素未找到');
            return;
        }

        this.elements.taskModal.style.display = 'block';
        console.log('模态框显示状态设置为block');

        this.elements.taskForm.reset();

        // 设置默认日期
        const today = new Date().toISOString().split('T')[0];
        this.elements.startDate.value = today;
        this.elements.endDate.value = today;
        this.elements.progress.value = 0;
        this.elements.progressValue.textContent = '0%';

        switch (mode) {
            case 'add':
                this.elements.modalTitle.textContent = '添加根任务';
                this.elements.parentId.value = '';
                break;

            case 'edit':
                console.log('编辑模式，当前任务:', this.currentTask);
                if (!this.currentTask) {
                    console.error('无法编辑：当前任务为空');
                    this.showError('请先选择一个任务进行编辑');
                    this.closeModal();
                    return;
                }
                console.log('编辑任务ID:', this.currentTask.id || this.currentTask.ID || this.currentTask.Id);
                console.log('编辑任务标题:', this.currentTask.title || this.currentTask.Title);
                this.elements.modalTitle.textContent = '编辑任务';
                this.fillFormWithTask(this.currentTask);
                break;

            case 'addSubtask':
                console.log('添加子任务模式，当前任务:', this.currentTask);
                if (!this.currentTask) {
                    console.error('无法添加子任务：当前任务为空');
                    this.showError('请先选择一个任务来添加子任务');
                    this.closeModal();
                    return;
                }
                console.log('父任务ID:', this.currentTask.id || this.currentTask.ID || this.currentTask.Id);
                this.elements.modalTitle.textContent = '添加子任务';
                this.elements.parentId.value = this.currentTask.id || this.currentTask.ID || this.currentTask.Id;
                break;
        }
    }

    fillFormWithTask(task) {
        console.log('填充表单数据:', task);

        // 支持多种字段名
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || '';
        const taskDescription = task.description || task.Description || '';
        const taskStatus = task.status || task.Status || 'planning';
        const taskPriority = task.priority || task.Priority || 3;
        const taskStartDate = task.start_date || task.startDate || task.StartDate || '';
        const taskEndDate = task.end_date || task.endDate || task.EndDate || '';
        const taskEstimatedTime = task.estimated_time || task.estimatedTime || task.EstimatedTime || 0;
        const taskProgress = task.progress || task.Progress || 0;
        const taskTags = task.tags || task.Tags || [];

        this.elements.taskId.value = taskId;
        this.elements.title.value = taskTitle;
        this.elements.description.value = taskDescription;
        this.elements.status.value = taskStatus;
        this.elements.priority.value = taskPriority.toString();
        this.elements.startDate.value = taskStartDate;
        this.elements.endDate.value = taskEndDate;
        this.elements.estimatedTime.value = taskEstimatedTime;
        this.elements.progress.value = taskProgress;
        this.elements.progressValue.textContent = `${taskProgress}%`;
        this.elements.tags.value = taskTags.join(', ');
    }

    closeModal() {
        this.elements.taskModal.style.display = 'none';
        this.elements.taskForm.reset();
    }

    async saveTask() {
        // 验证表单
        if (!this.elements.title.value.trim()) {
            this.showError('请填写任务标题');
            return;
        }

        const taskData = {
            title: this.elements.title.value.trim(),
            description: this.elements.description.value.trim(),
            status: this.elements.status.value,
            priority: parseInt(this.elements.priority.value),
            start_date: this.elements.startDate.value || null,
            end_date: this.elements.endDate.value || null,
            estimated_time: parseInt(this.elements.estimatedTime.value) || 0,
            progress: parseInt(this.elements.progress.value) || 0,
            tags: this.elements.tags.value ? this.elements.tags.value.split(',').map(tag => tag.trim()).filter(tag => tag) : []
        };

        // 如果有父任务ID，添加到数据中
        if (this.elements.parentId.value) {
            taskData.parent_id = this.elements.parentId.value;
        }

        try {
            let response;
            const taskId = this.elements.taskId.value;

            if (taskId) {
                // 更新任务
                response = await fetch(`/api/tasks/${taskId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(taskData)
                });
            } else {
                // 创建任务
                response = await fetch('/api/tasks', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(taskData)
                });
            }

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || '保存失败');
            }

            console.log('任务保存成功，开始重新加载数据...');
            this.closeModal();

            // 保存当前选中的任务ID，以便重新加载后能重新选中
            const currentTaskId = this.currentTask ? (this.currentTask.id || this.currentTask.ID || this.currentTask.Id) : null;
            console.log('当前选中的任务ID:', currentTaskId);

            // 清除当前任务引用，因为数据即将刷新
            this.currentTask = null;
            this.elements.taskDetailsCard.style.display = 'none';

            await this.loadData();

            // 如果之前有选中的任务，尝试重新选中
            if (currentTaskId) {
                console.log('尝试重新选中任务:', currentTaskId);
                const task = this.findTaskById(currentTaskId);
                if (task) {
                    console.log('找到任务，重新选中:', task);
                    this.selectTask(task);
                } else {
                    console.log('未找到任务，可能ID已变化或任务结构已更新');
                    // 尝试通过标题或其他方式查找
                    const savedResponse = await response.json();
                    if (savedResponse.data && savedResponse.data.id) {
                        const newTask = this.findTaskById(savedResponse.data.id);
                        if (newTask) {
                            console.log('通过API响应找到新任务:', newTask);
                            this.selectTask(newTask);
                        }
                    }
                }
            }

            this.showSuccess('保存成功');

        } catch (error) {
            console.error('保存任务失败:', error);
            this.showError(`保存失败: ${error.message}`);
        }
    }

    async deleteCurrentTask() {
        if (!this.currentTask || !confirm('确定要删除这个任务吗？')) {
            return;
        }

        try {
            const response = await fetch(`/api/tasks/${this.currentTask.id}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                throw new Error('删除失败');
            }

            this.currentTask = null;
            this.elements.taskDetailsCard.style.display = 'none';
            await this.loadData();
            this.showSuccess('删除成功');

        } catch (error) {
            console.error('删除任务失败:', error);
            this.showError('删除失败');
        }
    }

    updateStatistics(stats) {
        console.log('更新统计数据，接收到的数据:', stats);
        console.log('所有字段:', Object.keys(stats));
        console.log('字段详情:', {
            total_tasks: stats.total_tasks,
            completed_tasks: stats.completed_tasks,
            in_progress_tasks: stats.in_progress_tasks,
            blocked_tasks: stats.blocked_tasks,
            total_time: stats.total_time,
            status_distribution: stats.status_distribution
        });

        // 调试：检查 completed_tasks 的值
        console.log('completed_tasks 值:', stats.completed_tasks);
        console.log('completed_tasks 类型:', typeof stats.completed_tasks);

        this.elements.totalTasks.textContent = stats.total_tasks || 0;
        this.elements.completedTasks.textContent = stats.completed_tasks || 0;
        this.elements.inProgressTasks.textContent = stats.in_progress_tasks || 0;
        this.elements.blockedTasks.textContent = stats.blocked_tasks || 0;

        // 转换分钟为小时
        const totalHours = Math.round((stats.total_time || 0) / 60);
        this.elements.totalTime.textContent = `${totalHours}h`;
    }

    renderCharts(stats) {
        // 销毁之前的图表
        Object.values(this.charts).forEach(chart => {
            if (chart) chart.destroy();
        });

        // 状态分布图表
        const statusCtx = this.elements.statusChart.getContext('2d');
        this.charts.status = new Chart(statusCtx, {
            type: 'doughnut',
            data: {
                labels: Object.keys(stats.status_distribution || {}).map(key => this.getStatusText(key)),
                datasets: [{
                    data: Object.values(stats.status_distribution || {}),
                    backgroundColor: [
                        '#f0f0f0', // planning
                        '#e3f2fd', // in-progress
                        '#e8f5e9', // completed
                        '#ffebee', // blocked
                        '#f5f5f5'  // cancelled
                    ]
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        position: 'bottom'
                    }
                }
            }
        });

        // 优先级分布图表
        const priorityCtx = this.elements.priorityChart.getContext('2d');
        this.charts.priority = new Chart(priorityCtx, {
            type: 'bar',
            data: {
                labels: Object.keys(stats.priority_distribution || {}).map(key => this.getPriorityText(parseInt(key))),
                datasets: [{
                    label: '任务数量',
                    data: Object.values(stats.priority_distribution || {}),
                    backgroundColor: [
                        '#ffebee', // 最高
                        '#fff3e0', // 高
                        '#e8f5e9', // 中等
                        '#e3f2fd', // 低
                        '#f3e5f5'  // 最低
                    ]
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            stepSize: 1
                        }
                    }
                }
            }
        });
    }

    getStatusIcon(status) {
        const iconMap = {
            'planning': 'fas fa-calendar-alt',
            'in-progress': 'fas fa-spinner fa-spin',
            'completed': 'fas fa-check-circle',
            'blocked': 'fas fa-exclamation-circle',
            'cancelled': 'fas fa-times-circle'
        };
        return iconMap[status] || 'fas fa-calendar-alt';
    }

    formatDateForDisplay(dateString) {
        if (!dateString || dateString === '-') return '-';

        try {
            const date = new Date(dateString);
            if (isNaN(date.getTime())) return dateString;

            // 格式化为 YYYY-MM-DD
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            return `${year}-${month}-${day}`;
        } catch (error) {
            console.error('日期格式化错误:', error);
            return dateString;
        }
    }

    calculateDuration(startDate, endDate) {
        if (!startDate || !endDate || startDate === '-' || endDate === '-') return '-';

        try {
            const start = new Date(startDate);
            const end = new Date(endDate);

            if (isNaN(start.getTime()) || isNaN(end.getTime())) return '-';

            const diffTime = Math.abs(end - start);
            const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

            if (diffDays === 0) return '当天';
            if (diffDays === 1) return '1天';
            return `${diffDays}天`;
        } catch (error) {
            console.error('计算持续时间错误:', error);
            return '-';
        }
    }

    addTimelineEventListeners() {
        // 为查看任务按钮添加事件监听
        const viewButtons = document.querySelectorAll('.view-task-btn');
        viewButtons.forEach(button => {
            button.addEventListener('click', (e) => {
                e.stopPropagation();
                const taskId = button.getAttribute('data-task-id');
                console.log('查看时间线任务:', taskId);

                const task = this.findTaskById(taskId);
                if (task) {
                    this.selectTask(task);
                    // 切换到任务树选项卡
                    const taskTabLink = document.querySelector('.tab-link[data-tab="taskTab"]');
                    if (taskTabLink) {
                        taskTabLink.click();
                    }
                } else {
                    this.showError('无法找到该任务');
                }
            });
        });

        // 为时间线标记添加点击事件 - 切换显示内容
        const timelineMarkers = document.querySelectorAll('.timeline-marker');
        timelineMarkers.forEach(marker => {
            marker.addEventListener('click', (e) => {
                e.stopPropagation();
                const taskId = marker.getAttribute('data-task-id');
                const index = parseInt(marker.getAttribute('data-index'));
                console.log('点击时间线标记:', taskId, '索引:', index);

                // 先显示对应的任务卡片
                this.showTimelineTaskCard(index);

                // 然后更新激活状态（卡片已经渲染）
                this.updateTimelineActiveState(index);
            });
        });
    }

    updateTimelineActiveState(activeIndex) {
        // 移除所有激活状态
        const markers = document.querySelectorAll('.timeline-marker');
        const cards = document.querySelectorAll('.timeline-card-item');

        markers.forEach(marker => {
            marker.classList.remove('active');
        });

        cards.forEach(card => {
            card.classList.remove('active');
        });

        // 添加当前激活状态
        const activeMarker = document.querySelector(`.timeline-marker[data-index="${activeIndex}"]`);
        const activeCard = document.querySelector(`.timeline-card-item[data-index="${activeIndex}"]`);

        if (activeMarker) {
            activeMarker.classList.add('active');
        }

        if (activeCard) {
            activeCard.classList.add('active');
        }
    }

    showTimelineTaskCard(index) {
        if (!this.timelineTasks || !this.timelineTasks[index]) {
            console.error('找不到时间线任务:', index);
            return;
        }

        const task = this.timelineTasks[index];
        const placeholder = document.getElementById('timelineContentPlaceholder');

        if (!placeholder) {
            console.error('找不到时间线内容占位符');
            return;
        }

        // 清空并添加新的任务卡片
        placeholder.innerHTML = this.renderTimelineTaskCard(task, index, true);

        // 重新绑定查看任务按钮事件
        const viewButton = placeholder.querySelector('.view-task-btn');
        if (viewButton) {
            viewButton.addEventListener('click', (e) => {
                e.stopPropagation();
                const taskId = viewButton.getAttribute('data-task-id');
                console.log('查看时间线任务:', taskId);

                const task = this.findTaskById(taskId);
                if (task) {
                    this.selectTask(task);
                    // 切换到任务树选项卡
                    const taskTabLink = document.querySelector('.tab-link[data-tab="taskTab"]');
                    if (taskTabLink) {
                        taskTabLink.click();
                    }
                } else {
                    this.showError('无法找到该任务');
                }
            });
        }

        console.log('显示时间线任务卡片:', index, task.title || task.Title);
    }

    renderTimelineTaskCard(task, index, isActive = false) {
        // 支持多种字段名
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || '无标题';
        const taskStatus = task.status || task.Status || 'planning';
        const taskStartDate = task.start_date || task.StartDate || task.startDate || '';
        const taskEndDate = task.end_date || task.EndDate || task.endDate || '';
        const taskProgress = task.progress || task.Progress || 0;
        const taskDescription = task.description || task.Description || '';
        const taskPriority = task.priority || task.Priority || 3;
        const taskEstimatedTime = task.estimated_time || task.EstimatedTime || task.estimatedTime || 0;

        const statusClass = `status-${taskStatus.replace('-', '')}`;
        const priorityClass = `priority-${taskPriority}`;

        // 格式化日期
        const formattedStartDate = this.formatDateForDisplay(taskStartDate);
        const formattedEndDate = this.formatDateForDisplay(taskEndDate);

        // 计算持续时间
        const durationText = this.calculateDuration(taskStartDate, taskEndDate);

        const activeClass = isActive ? 'active' : '';

        return `
            <div class="timeline-card-item ${activeClass}" data-task-id="${taskId}" data-index="${index}">
                <div class="timeline-card-header">
                    <h3 class="timeline-card-title">${this.escapeHtml(taskTitle)}</h3>
                    <div class="timeline-card-meta">
                        <span class="task-status-badge ${statusClass}">${this.getStatusText(taskStatus)}</span>
                        <span class="task-priority-badge ${priorityClass}">${this.getPriorityText(taskPriority)}</span>
                    </div>
                </div>
                <div class="timeline-card-dates">
                    <div class="date-range">
                        <div class="date-item">
                            <i class="fas fa-play-circle"></i>
                            <span class="date-label">开始</span>
                            <span class="date-value highlight-date">${formattedStartDate}</span>
                        </div>
                        <div class="date-item">
                            <i class="fas fa-flag-checkered"></i>
                            <span class="date-label">结束</span>
                            <span class="date-value highlight-date">${formattedEndDate}</span>
                        </div>
                        <div class="date-item">
                            <i class="fas fa-clock"></i>
                            <span class="date-label">时长</span>
                            <span class="date-value">${durationText}</span>
                        </div>
                    </div>
                </div>
                <div class="timeline-card-progress">
                    <div class="progress-info">
                        <span class="progress-label">进度</span>
                        <span class="progress-value">${taskProgress}%</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${taskProgress}%"></div>
                    </div>
                </div>
                ${taskDescription ? `<div class="timeline-card-description">${this.escapeHtml(taskDescription)}</div>` : ''}
                <div class="timeline-card-footer">
                    <span class="estimated-time"><i class="fas fa-hourglass-half"></i> ${taskEstimatedTime}分钟</span>
                    <button class="btn btn-small btn-outline view-task-btn" data-task-id="${taskId}">
                        <i class="fas fa-eye"></i> 查看任务
                    </button>
                </div>
            </div>
        `;
    }

    renderTimeline(timelineData) {
        console.log('渲染时间线数据:', timelineData);
        // 简化的时间线渲染
        let tasks = timelineData.tasks || [];

        if (tasks.length === 0) {
            this.elements.timelineChart.innerHTML = `
                <div class="empty-state">
                    <i class="fas fa-calendar-alt"></i>
                    <p>没有时间线数据</p>
                </div>
            `;
            return;
        }

        // 在前端也进行排序，确保一致性
        tasks.sort((a, b) => {
            // 支持多种字段名
            const aStartDateStr = a.start_date || a.StartDate || a.startDate || '';
            const bStartDateStr = b.start_date || b.StartDate || b.startDate || '';
            const aEndDateStr = a.end_date || a.EndDate || a.endDate || '';
            const bEndDateStr = b.end_date || b.EndDate || b.endDate || '';
            const aParentID = a.parent_id || a.ParentID || a.parentId || '';
            const bParentID = b.parent_id || b.ParentID || b.parentId || '';

            // 将日期字符串转换为Date对象进行比较
            const parseDate = (dateStr) => {
                if (!dateStr || dateStr === '-') return new Date(0); // 空日期设为最小日期
                const date = new Date(dateStr);
                return isNaN(date.getTime()) ? new Date(0) : date;
            };

            const aStartDate = parseDate(aStartDateStr);
            const bStartDate = parseDate(bStartDateStr);
            const aEndDate = parseDate(aEndDateStr);
            const bEndDate = parseDate(bEndDateStr);

            // 1. 首先，父任务优先于子任务
            // 如果a是父任务（parent_id为空）而b是子任务，a应该排在前面
            if (aParentID === '' && bParentID !== '') {
                return -1;
            }
            // 如果a是子任务而b是父任务，b应该排在前面
            if (aParentID !== '' && bParentID === '') {
                return 1;
            }

            // 2. 都是父任务或都是子任务，按开始时间排序（最早的在前）
            if (aStartDate.getTime() !== bStartDate.getTime()) {
                return aStartDate.getTime() - bStartDate.getTime();
            }

            // 3. 开始时间相同，按结束时间排序
            return aEndDate.getTime() - bEndDate.getTime();
        });

        console.log('排序后的时间线任务:', tasks);

        // 调试：显示每个任务的详细信息
        console.log('任务详细信息:');
        tasks.forEach((task, index) => {
            const taskId = task.id || task.ID || task.Id || '';
            const taskTitle = task.title || task.Title || '无标题';
            const startDate = task.start_date || task.StartDate || task.startDate || '';
            const endDate = task.end_date || task.EndDate || task.endDate || '';
            const parentID = task.parent_id || task.ParentID || task.parentId || '';
            console.log(`[${index}] ID: ${taskId}, 标题: "${taskTitle}", 开始: ${startDate}, 结束: ${endDate}, 父ID: "${parentID}"`);
        });

        // 创建视觉化时间线
        let html = `
            <div class="timeline-container">
                <div class="timeline-axis">
                    <div class="timeline-line"></div>
        `;

        tasks.forEach((task, index) => {
            // 支持多种字段名
            const taskId = task.id || task.ID || task.Id || '';
            const taskTitle = task.title || task.Title || '无标题';
            const taskStatus = task.status || task.Status || 'planning';
            const taskStartDate = task.start_date || task.StartDate || task.startDate || '';
            const taskEndDate = task.end_date || task.EndDate || task.endDate || '';
            const taskProgress = task.progress || task.Progress || 0;
            const taskDescription = task.description || task.Description || '';
            const taskPriority = task.priority || task.Priority || 3;

            const statusClass = `status-${taskStatus.replace('-', '')}`;
            const priorityClass = `priority-${taskPriority}`;

            // 获取状态图标
            const statusIcon = this.getStatusIcon(taskStatus);

            // 格式化日期
            const formattedStartDate = this.formatDateForDisplay(taskStartDate);
            const formattedEndDate = this.formatDateForDisplay(taskEndDate);

            // 第一个任务默认激活
            const activeClass = index === 0 ? 'active' : '';

            html += `
                    <div class="timeline-marker ${activeClass}" data-task-id="${taskId}" data-index="${index}">
                        <div class="timeline-dot ${statusClass}">
                            <i class="${statusIcon}"></i>
                        </div>
                        <div class="timeline-info">
                            <div class="timeline-title">${this.escapeHtml(taskTitle)}</div>
                            <div class="timeline-date">
                                <i class="fas fa-calendar-alt"></i>
                                ${formattedStartDate}
                            </div>
                        </div>
                    </div>
            `;
        });

        html += `
                </div>
                <div class="timeline-content">
                    <div class="timeline-content-placeholder" id="timelineContentPlaceholder">
                        <!-- 内容将根据点击的时间点动态加载 -->
        `;

        // 默认显示第一个任务
        if (tasks.length > 0) {
            const firstTask = tasks[0];
            html += this.renderTimelineTaskCard(firstTask, 0, true);
        }

        html += `
                    </div>
                </div>
        `;

        this.elements.timelineChart.innerHTML = html;

        // 存储任务数据供点击时使用
        this.timelineTasks = tasks;

        // 添加时间线交互事件监听
        this.addTimelineEventListeners();

        console.log('时间线渲染完成');
    }


    // 工具方法
    getStatusText(status) {
        const statusMap = {
            'planning': '规划中',
            'in-progress': '进行中',
            'completed': '已完成',
            'blocked': '阻塞中',
            'cancelled': '已取消'
        };
        return statusMap[status] || status;
    }

    getPriorityText(priority) {
        const priorityMap = {
            1: '最高',
            2: '高',
            3: '中等',
            4: '低',
            5: '最低'
        };
        return priorityMap[priority] || `优先级 ${priority}`;
    }

    findTaskById(taskId) {
        if (!taskId) return null;

        // 首先在扁平任务列表中查找
        const flatResult = this.tasks.find(task => {
            // 支持多种可能的字段名
            const taskIdToCompare = task.id || task.ID || task.Id || '';
            return taskIdToCompare === taskId;
        });

        if (flatResult) {
            return flatResult;
        }

        // 如果在扁平列表中没找到，递归在任务树中查找
        return this.findTaskInTree(taskId, this.tasks);
    }

    findTaskInTree(taskId, tasks) {
        if (!taskId || !tasks || !Array.isArray(tasks)) return null;

        for (const task of tasks) {
            // 检查当前任务
            const taskIdToCompare = task.id || task.ID || task.Id || '';
            if (taskIdToCompare === taskId) {
                return task;
            }

            // 递归检查子任务
            if (task.subtasks && Array.isArray(task.subtasks) && task.subtasks.length > 0) {
                const foundInSubtasks = this.findTaskInTree(taskId, task.subtasks);
                if (foundInSubtasks) {
                    return foundInSubtasks;
                }
            }

            // 也检查 Subtasks 字段（大写）
            if (task.Subtasks && Array.isArray(task.Subtasks) && task.Subtasks.length > 0) {
                const foundInSubtasks = this.findTaskInTree(taskId, task.Subtasks);
                if (foundInSubtasks) {
                    return foundInSubtasks;
                }
            }
        }

        return null;
    }

    async fetchTaskById(taskId) {
        if (!taskId) return null;

        try {
            console.log(`从服务器获取任务ID: ${taskId}`);
            const response = await fetch(`/api/tasks/${taskId}`);
            if (!response.ok) {
                console.error(`获取任务失败，状态码: ${response.status}`);
                return null;
            }
            const data = await response.json();
            console.log('从服务器获取的任务数据:', data);
            return data.data || null;
        } catch (error) {
            console.error(`获取任务异常: ${error}`);
            return null;
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    getRootTaskID() {
        // 优先从body的data属性获取
        const body = document.body;
        if (body && body.dataset.rootTaskId) {
            return body.dataset.rootTaskId;
        }

        // 其次从URL参数获取
        const urlParams = new URLSearchParams(window.location.search);
        const rootParam = urlParams.get('root');
        if (rootParam) {
            return rootParam;
        }

        // 无根任务ID
        return '';
    }

    showLoading() {
        // 可以添加加载指示器
        this.elements.refreshBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
    }

    hideLoading() {
        this.elements.refreshBtn.innerHTML = '<i class="fas fa-sync-alt"></i>';
    }

    showError(message) {
        alert(`错误: ${message}`);
    }

    showSuccess(message) {
        // 可以添加更优雅的成功提示
        console.log(`成功: ${message}`);
    }


    // ==================== 时间趋势相关方法 ====================

    async loadTrendsData() {
        try {
            console.log('开始加载时间趋势数据...');

            // 显示加载状态
            if (this.elements.refreshTrendsBtn) {
                this.elements.refreshTrendsBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
            }

            // 构建请求URL
            let url = '/api/tasks/trends';
            const params = new URLSearchParams();

            // 添加根任务ID（如果有）
            if (this.rootTaskID) {
                params.append('root', this.rootTaskID);
            }

            // 添加时间范围
            const timeRange = this.elements.timeRangeSelect ? this.elements.timeRangeSelect.value : '30d';
            params.append('range', timeRange);

            url += '?' + params.toString();

            console.log('请求URL:', url);

            const response = await fetch(url);
            if (!response.ok) {
                throw new Error(`HTTP错误! 状态: ${response.status}`);
            }

            const result = await response.json();
            console.log('时间趋势数据加载完成:', result);

            if (result.success && result.data) {
                this.renderTrendsCharts(result.data);
            } else {
                console.error('API返回错误:', result.error || '未知错误');
                this.showError('加载趋势数据失败');
            }
        } catch (error) {
            console.error('加载时间趋势数据失败:', error);
            this.showError('加载趋势数据失败: ' + error.message);
        } finally {
            // 恢复按钮状态
            if (this.elements.refreshTrendsBtn) {
                this.elements.refreshTrendsBtn.innerHTML = '<i class="fas fa-sync-alt"></i>';
            }
        }
    }

    renderTrendsCharts(trendsData) {
        console.log('渲染时间趋势图表...', trendsData);

        // 销毁之前的趋势图表
        ['creation', 'completion', 'progress'].forEach(chartName => {
            if (this.charts[chartName + 'Trend']) {
                this.charts[chartName + 'Trend'].destroy();
                delete this.charts[chartName + 'Trend'];
            }
        });

        // 渲染创建趋势图表
        if (trendsData.creation_trend && this.elements.creationTrendChart) {
            this.renderTrendChart(
                'creationTrend',
                trendsData.creation_trend,
                this.elements.creationTrendChart,
                'line'
            );
        }

        // 渲染完成趋势图表
        if (trendsData.completion_trend && this.elements.completionTrendChart) {
            this.renderTrendChart(
                'completionTrend',
                trendsData.completion_trend,
                this.elements.completionTrendChart,
                'line'
            );
        }

        // 渲染进度趋势图表
        if (trendsData.progress_trend && this.elements.progressTrendChart) {
            this.renderTrendChart(
                'progressTrend',
                trendsData.progress_trend,
                this.elements.progressTrendChart,
                'line'
            );
        }

        console.log('时间趋势图表渲染完成');
    }

    renderTrendChart(chartName, trendData, canvasElement, chartType = 'line') {
        if (!trendData || !trendData.data_points || trendData.data_points.length === 0) {
            console.warn(`没有数据可用于渲染图表: ${chartName}`);
            return;
        }

        const ctx = canvasElement.getContext('2d');

        // 准备数据
        const labels = trendData.data_points.map(point => {
            // 简化日期显示，例如 "01-15"
            const date = new Date(point.date);
            return `${(date.getMonth() + 1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')}`;
        });

        const dataPoints = trendData.data_points.map(point => point.value);

        // 根据趋势设置颜色
        let borderColor, backgroundColor;
        switch (trendData.trend) {
            case 'up':
                borderColor = '#4CAF50'; // 绿色
                backgroundColor = 'rgba(76, 175, 80, 0.1)';
                break;
            case 'down':
                borderColor = '#f44336'; // 红色
                backgroundColor = 'rgba(244, 67, 54, 0.1)';
                break;
            default:
                borderColor = '#2196F3'; // 蓝色
                backgroundColor = 'rgba(33, 150, 243, 0.1)';
                break;
        }

        // 创建图表配置
        const config = {
            type: chartType,
            data: {
                labels: labels,
                datasets: [{
                    label: trendData.title,
                    data: dataPoints,
                    borderColor: borderColor,
                    backgroundColor: backgroundColor,
                    borderWidth: 2,
                    fill: true,
                    tension: 0.4, // 曲线平滑度
                    pointRadius: 3,
                    pointHoverRadius: 5
                }]
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: true,
                        position: 'top'
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false,
                        callbacks: {
                            label: function(context) {
                                let label = context.dataset.label || '';
                                if (label) {
                                    label += ': ';
                                }
                                label += context.parsed.y + ' ' + trendData.unit;
                                return label;
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: '日期'
                        }
                    },
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: trendData.unit
                        }
                    }
                }
            }
        };

        // 创建图表
        this.charts[chartName] = new Chart(ctx, config);
    }

    // 在初始化时加载趋势数据
    async loadInitialTrendsData() {
        // 等待一小段时间确保DOM完全加载
        setTimeout(() => {
            if (this.elements.creationTrendChart &&
                this.elements.completionTrendChart &&
                this.elements.progressTrendChart) {
                this.loadTrendsData();
            }
        }, 500);
    }
}


// 页面加载完成后初始化应用
document.addEventListener('DOMContentLoaded', () => {
    window.taskApp = new TaskBreakdownApp();
});