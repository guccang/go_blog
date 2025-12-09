/**
 * Task Breakdown Tracking Page JavaScript
 */

class TaskBreakdownApp {
    constructor() {
        this.currentTask = null;
        this.tasks = [];
        this.charts = {};

        // Get root task ID (from URL params or data attribute)
        this.rootTaskID = this.getRootTaskID();

        this.init();
    }

    init() {
        // DOM Elements
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
            taskDailyTime: document.getElementById('taskDailyTime'),
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
            dailyTime: document.getElementById('dailyTime'),
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
            rootTasksList: document.getElementById('rootTasksList'),
            timeAnalysisPanel: document.getElementById('timeAnalysisPanel'),
            selfEstimatedTime: document.getElementById('selfEstimatedTime'),
            subtasksEstimatedTime: document.getElementById('subtasksEstimatedTime'),
            estimatedTimeDiff: document.getElementById('estimatedTimeDiff'),
            estimatedTimeStatus: document.getElementById('estimatedTimeStatus'),
            selfDailyTime: document.getElementById('selfDailyTime'),
            subtasksDailyTime: document.getElementById('subtasksDailyTime'),
            dailyTimeDiff: document.getElementById('dailyTimeDiff'),
            dailyTimeStatus: document.getElementById('dailyTimeStatus'),
            timeOverlapResult: document.getElementById('timeOverlapResult'),
            // Time comparison evaluation elements
            evalEstimatedTime: document.getElementById('evalEstimatedTime'),
            evalActualTime: document.getElementById('evalActualTime'),
            evalTimeDiff: document.getElementById('evalTimeDiff'),
            evalTimePercent: document.getElementById('evalTimePercent'),
            evalResult: document.getElementById('evalResult')
        };

        // Debug: Check if key elements exist
        console.log('TaskBreakdownApp initializing...');
        console.log('addRootTask element:', this.elements.addRootTask);
        console.log('saveTask element:', this.elements.saveTask);
        console.log('taskModal element:', this.elements.taskModal);

        // Event listeners
        this.bindEvents();

        // Initialize date to today
        const today = new Date().toISOString().split('T')[0];
        this.elements.startDate.value = today;
        this.elements.endDate.value = today;

        // Progress slider event
        this.elements.progress.addEventListener('input', (e) => {
            this.elements.progressValue.textContent = `${e.target.value}%`;
        });

        // Load data
        this.loadData();

        // Load trends data (delayed load to ensure chart container is ready)
        this.loadInitialTrendsData();
    }

    bindEvents() {
        console.log('Binding events...');

        // Button events
        if (this.elements.addRootTask) {
            this.elements.addRootTask.addEventListener('click', () => {
                console.log('Clicked Add Root Task button');
                this.openModal('add');
            });
        } else {
            console.error('addRootTask element not found');
        }

        if (this.elements.saveTask) {
            this.elements.saveTask.addEventListener('click', () => {
                console.log('Clicked Save button');
                this.saveTask();
            });
        } else {
            console.error('saveTask element not found');
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

        // Tab switch events
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

        // Status filter events
        if (this.elements.statusFilter) {
            this.elements.statusFilter.addEventListener('change', () => this.applyStatusFilter());
        }

        // Modal close
        this.elements.closeButtons.forEach(btn => {
            btn.addEventListener('click', () => this.closeModal());
        });

        // Click outside modal to close
        window.addEventListener('click', (e) => {
            if (e.target === this.elements.taskModal) {
                this.closeModal();
            }
        });

        // Estimated time auto-calculation events
        if (this.elements.startDate && this.elements.endDate && this.elements.dailyTime && this.elements.estimatedTime) {
            const calculateHandler = () => this.calculateEstimatedTime();
            this.elements.startDate.addEventListener('change', calculateHandler);
            this.elements.endDate.addEventListener('change', calculateHandler);
            this.elements.dailyTime.addEventListener('input', calculateHandler);
            this.elements.dailyTime.addEventListener('change', calculateHandler);
            console.log('Added estimated time auto-calculation event listeners');
        } else {
            console.log('Estimated time calculation related elements not found, skipping event binding');
        }
    }

    // Calculate estimated time (based on start date, end date, and daily allocated time)
    calculateEstimatedTime() {
        console.log('Start calculating estimated time...');

        // Get input values
        const startDate = this.elements.startDate.value;
        const endDate = this.elements.endDate.value;
        const dailyTime = parseInt(this.elements.dailyTime.value) || 0;

        console.log(`Calculation params: startDate=${startDate}, endDate=${endDate}, dailyTime=${dailyTime}`);

        // Validate input
        if (!startDate || !endDate || dailyTime <= 0) {
            console.log('Calculation conditions not met, not clearing estimated time field');
            // Do not clear estimated time field, allow user to manually input
            return;
        }

        // Calculate day difference
        const start = new Date(startDate);
        const end = new Date(endDate);

        // Ensure end date is not earlier than start date
        if (end < start) {
            console.log('End date is earlier than start date, skipping estimated time calculation');
            return;
        }

        // Calculate days (inclusive of start and end dates)
        const timeDiff = end.getTime() - start.getTime();
        const days = Math.floor(timeDiff / (1000 * 3600 * 24)) + 1;

        if (days <= 0) {
            console.log('Invalid day calculation, skipping estimated time calculation');
            return;
        }

        // Calculate estimated time (minutes)
        const estimatedTime = days * dailyTime;

        console.log(`Calculation result: days=${days}, dailyTime=${dailyTime}, estimatedTime=${estimatedTime}`);

        // Update estimated time field
        this.elements.estimatedTime.value = estimatedTime;

        // Optional: Trigger change event so other listeners know the value has changed
        this.elements.estimatedTime.dispatchEvent(new Event('change', { bubbles: true }));
    }

    switchTab(event) {
        const tabLink = event.currentTarget;
        const tabId = tabLink.getAttribute('data-tab');

        // Remove active state from all tabs and panes
        this.elements.tabLinks.forEach(link => link.classList.remove('active'));
        document.querySelectorAll('.tab-pane').forEach(pane => {
            pane.classList.remove('active');
        });

        // Add active state to clicked tab and corresponding pane
        tabLink.classList.add('active');
        const tabPane = document.getElementById(tabId);
        if (tabPane) {
            tabPane.classList.add('active');
        }

        // Load tab data if needed
        if (tabId === 'statsTab') {
            // If statistics charts not rendered, reload data to render charts
            if (!this.charts.status) {
                this.loadData(); // This will load statistics data and render charts
            }
        } else if (tabId === 'trendsTab') {
            // Load trends data
            this.loadTrendsData();
        } else if (tabId === 'timelineTab') {
            // If timeline data not loaded, reload data
            if (!this.timelineTasks || this.timelineTasks.length === 0) {
                this.loadData(); // This will load timeline data and render
            }
        }
        // Task tree tab needs no special handling, data loaded in loadData
    }

    async loadData() {
        try {
            console.log('Start loading data...');
            // Show loading state
            this.showLoading();

            // Load all data in parallel
            const [tasks, stats, timeline] = await Promise.all([
                this.fetchTasks(),
                this.fetchStatistics(),
                this.fetchTimeline()
            ]);

            console.log('Task data loaded, count:', tasks.length);
            console.log('Task data sample (full):', tasks.length > 0 ? tasks[0] : 'No tasks');
            console.log('All task IDs:', tasks.map(t => t.id || t.ID || t.Id || 'No ID'));
            console.log('Statistics data:', stats);

            this.tasks = tasks;
            console.log('this.tasks set, length:', this.tasks.length);

            // Render task tree
            console.log('Rendering task tree...');
            this.renderTaskTree();
            console.log('Task tree rendering complete');

            // Render root task list
            console.log('Rendering root task list...');
            this.renderRootTasksList(tasks);
            console.log('Root task list rendering complete');

            // If root task ID exists, automatically select it
            if (this.rootTaskID) {
                console.log(`Attempting to select root task: ${this.rootTaskID}`);
                const rootTask = this.findTaskById(this.rootTaskID);
                if (rootTask) {
                    console.log('Root task found, auto-selecting:', rootTask);
                    this.selectTask(rootTask);

                    // Update breadcrumb navigation
                    if (this.elements.rootTaskBreadcrumb && this.elements.rootTaskTitle) {
                        this.elements.rootTaskBreadcrumb.style.display = 'flex';
                        const taskTitle = rootTask.title || rootTask.Title || 'Unnamed Task';
                        this.elements.rootTaskTitle.textContent = taskTitle;
                    }
                } else {
                    console.log('Root task not found, ID invalid or data not loaded');
                    // Still show breadcrumb, but as Unknown Task
                    if (this.elements.rootTaskBreadcrumb && this.elements.rootTaskTitle) {
                        this.elements.rootTaskBreadcrumb.style.display = 'flex';
                        this.elements.rootTaskTitle.textContent = 'Unknown Task';
                    }
                }
            }

            console.log('Updating statistics...');
            this.updateStatistics(stats);

            console.log('Rendering charts...');
            this.renderCharts(stats);

            console.log('Rendering timeline...');
            this.renderTimeline(timeline);

            // If there is a currently selected task, update details
            if (this.currentTask) {
                const task = this.findTaskById(this.currentTask.id);
                if (task) {
                    this.showTaskDetails(task);
                }
            }

            console.log('Data loading and rendering complete');

        } catch (error) {
            console.error('Failed to load data:', error);
            this.showError('Failed to load data, please refresh and try again');
        } finally {
            this.hideLoading();
        }
    }

    async fetchTasks() {
        console.log('Start fetching task data...');

        let url = '/api/tasks';
        if (this.rootTaskID) {
            console.log(`Root task ID: ${this.rootTaskID}, fetching subtree`);
            url = `/api/tasks/subtasks?parent_id=${encodeURIComponent(this.rootTaskID)}`;
        }

        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        console.log('Raw API response:', data);
        let tasks = data.data || [];

        // If response is tree structure (single task object), flatten it
        if (tasks && !Array.isArray(tasks)) {
            tasks = this.flattenTaskTree(tasks);
        }

        console.log('Parsed task data:', tasks);
        if (tasks.length > 0) {
            console.log('Fields of first task:', Object.keys(tasks[0]));
        }
        return tasks;
    }

    flattenTaskTree(taskTree, parentId = '') {
        const flatTasks = [];

        // Copy task object, add parent_id (if provided)
        const taskCopy = { ...taskTree };
        if (parentId) {
            taskCopy.parent_id = parentId;
        }

        flatTasks.push(taskCopy);

        // Recursively process subtasks
        if (taskTree.subtasks && Array.isArray(taskTree.subtasks)) {
            for (const subtask of taskTree.subtasks) {
                const subtaskFlat = this.flattenTaskTree(subtask, taskTree.id || taskTree.ID || taskTree.Id || '');
                flatTasks.push(...subtaskFlat);
            }
        }

        // Also process Subtasks field (capitalized)
        if (taskTree.Subtasks && Array.isArray(taskTree.Subtasks)) {
            for (const subtask of taskTree.Subtasks) {
                const subtaskFlat = this.flattenTaskTree(subtask, taskTree.id || taskTree.ID || taskTree.Id || '');
                flatTasks.push(...subtaskFlat);
            }
        }

        return flatTasks;
    }

    // Render root task list
    renderRootTasksList(tasks) {
        if (!this.elements.rootTasksList || !this.elements.rootTasksPanel) {
            return;
        }

        // Get root tasks (no parent_id or empty parent_id), and not completed/cancelled/deleted
        const rootTasks = tasks.filter(task => {
            const parentId = task.parent_id || task.parentId || task.parentID || '';
            const isRoot = !parentId || parentId === '';
            const taskStatus = task.status || task.Status || 'planning';
            const isCompleted = taskStatus === 'completed' || task.progress === 100;
            const isCancelled = taskStatus === 'cancelled';
            const isDeleted = task.deleted || task.Deleted || false;
            return isRoot && !isCompleted && !isCancelled && !isDeleted;
        });

        console.log('Root task count:', rootTasks.length);

        // If no root tasks, hide panel
        if (rootTasks.length === 0) {
            this.elements.rootTasksPanel.style.display = 'none';
            return;
        }

        // Show panel
        this.elements.rootTasksPanel.style.display = 'block';
        this.elements.rootTasksList.innerHTML = '';

        // Add "All Tasks" link
        const allTasksItem = document.createElement('a');
        allTasksItem.href = '/taskbreakdown';
        allTasksItem.className = `root-task-item ${this.rootTaskID === '' ? 'active' : ''}`;
        allTasksItem.innerHTML = `
            <span class="root-task-status all"></span>
            <span>All Tasks</span>
        `;
        allTasksItem.addEventListener('click', (e) => {
            e.preventDefault();
            window.location.href = '/taskbreakdown';
        });
        this.elements.rootTasksList.appendChild(allTasksItem);

        // Add each root task
        rootTasks.forEach(task => {
            const taskId = task.id || task.ID || task.Id || '';
            const taskTitle = task.title || task.Title || 'Unnamed Task';
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

    // Apply status filter
    applyStatusFilter() {
        if (!this.elements.statusFilter) {
            return;
        }

        const filterValue = this.elements.statusFilter.value;
        console.log('Applying status filter:', filterValue);

        if (!filterValue) {
            // Reset filter, show all tasks (use default filter, hide completed root tasks)
            this.renderTaskTree();
            return;
        }

        // Get filtered tasks
        const filteredTasks = this.tasks.filter(task => {
            const taskStatus = (task.status || task.Status || 'planning').toLowerCase();

            if (filterValue.includes(',')) {
                // Multiple status values (e.g. "planning,in-progress,blocked" means incomplete)
                const allowedStatuses = filterValue.split(',').map(s => s.trim());
                return allowedStatuses.includes(taskStatus);
            } else {
                // Single status value
                return taskStatus === filterValue;
            }
        });

        // Re-render task tree with filtered tasks, passing isStatusFilter=true
        this.renderTaskTree(filteredTasks, true);
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

    renderTaskTree(tasks = null, isStatusFilter = false) {
        const tasksToRender = tasks || this.tasks;
        console.log('=== Start Rendering Task Tree ===');
        console.log('Total tasks to render:', tasksToRender.length);
        console.log('Is status filter active:', isStatusFilter);

        // Check task data structure
        if (tasksToRender.length > 0) {
            const firstTask = tasksToRender[0];
            console.log('All fields of first task:', Object.keys(firstTask));
            console.log('parent_id value of first task:', firstTask.parent_id);
            console.log('parentId value of first task:', firstTask.parentId);
            console.log('parentID value of first task:', firstTask.parentID);
            console.log('Full object of first task:', firstTask);
        }

        // Clear task tree
        this.elements.taskTree.innerHTML = '';

        // Reset rendered tasks record
        this.renderedTasks = new Set();

        // Get root tasks - support multiple possible field names
        const rootTasks = tasksToRender.filter(task => {
            const taskId = task.id || task.ID || task.Id || '';
            const parentId = task.parent_id || task.parentId || task.parentID || '';
            console.log(`Task ${taskId} parentId: "${parentId}"`);

            // Root task conditions:
            // 1. parent_id is empty
            // 2. parent_id equals own id (data error case)
            // 3. parent task does not exist (orphan task)
            const isRoot = (!parentId || parentId === '') ||
                (parentId === taskId) ||
                (!tasksToRender.find(t => {
                    const tId = t.id || t.ID || t.Id || '';
                    return tId === parentId;
                }));

            if (!isRoot) {
                console.log(`   -> Not a root task`);
                return false;
            }

            console.log(`   -> Is a root task`);

            // If status filter mode, show all root tasks matching filter
            // Otherwise, filter out completed/cancelled/deleted root tasks (default behavior)
            if (!isStatusFilter) {
                const taskStatus = task.status || task.Status || 'planning';
                const taskProgress = task.progress || task.Progress || 0;
                const isCompleted = taskStatus === 'completed' || taskProgress === 100;
                const isCancelled = taskStatus === 'cancelled';
                const isDeleted = task.deleted || task.Deleted || false;

                if (isCompleted || isCancelled || isDeleted) {
                    console.log(`   -> Completed/cancelled/deleted root task, skipping`);
                    return false;
                }
            }

            console.log(`   -> Showing root task`);
            return true;
        });
        console.log('Root task count:', rootTasks.length);
        console.log('Root task details:', rootTasks);

        if (rootTasks.length === 0) {
            // If status filter mode, there should be no orphan tasks as root tasks are shown
            if (isStatusFilter) {
                console.log('Status filter mode: No tasks match filter');
                // Show empty state
                this.elements.taskTree.innerHTML = `
                    <div class="empty-state">
                        <i class="fas fa-search fa-3x"></i>
                        <h3>No tasks match filter conditions</h3>
                        <p>Try selecting a different status filter</p>
                    </div>
                `;
                return;
            }

            console.log('No incomplete root tasks, checking for orphan tasks');

            // Find orphan tasks (parent does not exist or parent is completed/cancelled/deleted)
            const orphanTasks = tasksToRender.filter(task => {
                const taskId = task.id || task.ID || task.Id || '';
                const parentId = task.parent_id || task.parentId || task.parentID || '';

                // If no parent task, not an orphan (is root task but filtered out)
                if (!parentId || parentId === '') {
                    return false;
                }

                // Find parent task
                const parentTask = tasksToRender.find(t => {
                    const tId = t.id || t.ID || t.Id || '';
                    return tId === parentId;
                });

                // If parent task does not exist, is orphan
                if (!parentTask) {
                    return true;
                }

                // If parent task is completed/cancelled/deleted, is orphan
                const parentStatus = parentTask.status || parentTask.Status || 'planning';
                const parentProgress = parentTask.progress || parentTask.Progress || 0;
                const parentDeleted = parentTask.deleted || parentTask.Deleted || false;
                const parentCompleted = parentStatus === 'completed' || parentProgress === 100;
                const parentCancelled = parentStatus === 'cancelled';

                return parentCompleted || parentCancelled || parentDeleted;
            });

            console.log('Orphan task count:', orphanTasks.length);

            if (orphanTasks.length === 0) {
                // Show empty state
                this.elements.taskTree.innerHTML = `
                    <div class="empty-state">
                        <i class="fas fa-check-circle fa-3x"></i>
                        <h3>No tasks in progress</h3>
                        <p>All tasks completed or cancelled</p>
                        <a href="/taskbreakdown/completed" class="btn btn-primary">
                            <i class="fas fa-list-check"></i> View Completed Tasks
                        </a>
                    </div>
                `;
                return;
            }

            // Show orphan tasks
            console.log('Showing orphan tasks as flat list');
            orphanTasks.forEach((task, index) => {
                const taskTitle = task.title || task.Title || 'No Title';
                console.log(`Flat rendering orphan task ${index + 1}/${orphanTasks.length}: ${taskTitle}`);
                const rendered = this.renderTaskNode(task, this.elements.taskTree, 0, new Set(), true);
                if (!rendered) {
                    console.log(`Orphan task ${taskTitle} render failed or skipped`);
                }
            });
            return;
        }

        // Sort by order - support multiple possible field names
        rootTasks.sort((a, b) => {
            const orderA = a.order || a.Order || 0;
            const orderB = b.order || b.Order || 0;
            console.log(`Sort: Task A order=${orderA}, Task B order=${orderB}`);
            return orderA - orderB;
        });

        console.log('Start rendering root tasks...');
        // Render root tasks
        rootTasks.forEach((task, index) => {
            const taskTitle = task.title || task.Title || 'No Title';
            console.log(`Rendering root task ${index + 1}/${rootTasks.length}: ${taskTitle}`);
            const rendered = this.renderTaskNode(task, this.elements.taskTree, 0);
            if (!rendered) {
                console.log(`Root task ${taskTitle} render failed or skipped`);
            }
        });

        // Initialize sortable
        this.initSortable();
        console.log('=== Task Tree Rendering Complete ===');
    }

    renderTaskNode(task, container, level, visited = new Set(), isOrphan = false) {
        // Support multiple possible field names
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || 'Unnamed Task';
        const taskStatus = task.status || task.Status || 'planning';
        const taskPriority = task.priority || task.Priority || 3;
        const taskProgress = task.progress || task.Progress || 0;

        // Check for circular references
        if (visited.has(taskId)) {
            console.error(`Circular reference detected! Task ${taskId} (${taskTitle}) is already in render path`);
            console.error('Visited tasks:', Array.from(visited));
            return null;
        }

        // Check if already rendered (avoid duplicate rendering)
        if (this.renderedTasks && this.renderedTasks.has(taskId)) {
            console.log(`Task ${taskId} (${taskTitle}) already rendered, skipping`);
            return null;
        }

        console.log(`Rendering task: ${taskId} (${taskTitle}), Level: ${level}, Orphan: ${isOrphan}`);

        const taskElement = document.createElement('div');
        taskElement.className = `task-node ${isOrphan ? 'task-orphan' : ''}`;
        taskElement.dataset.taskId = taskId;
        taskElement.dataset.isOrphan = isOrphan;
        taskElement.style.paddingLeft = `${level * 20 + 10}px`;

        // Get subtasks - support multiple possible field names
        const subtasks = this.tasks.filter(t => {
            const tParentId = t.parent_id || t.parentId || t.parentID || '';
            return tParentId === taskId && t !== task; // Avoid self-reference
        });

        // Sort by order - support multiple possible field names
        subtasks.sort((a, b) => {
            const orderA = a.order || a.Order || 0;
            const orderB = b.order || b.Order || 0;
            return orderA - orderB;
        });

        // Status and priority styles
        const statusClass = `status-${taskStatus.replace('-', '')}`;
        const priorityClass = `priority-${taskPriority}`;

        // Build HTML
        taskElement.innerHTML = `
            <div class="task-node-header">
                <div class="task-node-title-container">
                    <span class="task-selected-indicator">
                        <i class="fas fa-circle-notch"></i>
                    </span>
                    <div class="task-node-title">${this.escapeHtml(taskTitle)}</div>
                    ${isOrphan ? `
                    <span class="task-orphan-indicator" title="Parent task completed or cancelled">
                        <i class="fas fa-exclamation-triangle"></i>
                        <span class="tooltip-text">Parent task completed or cancelled</span>
                    </span>
                    ` : ''}
                </div>
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

        // Click event
        taskElement.addEventListener('click', (e) => {
            // Stop propagation to avoid parent task handling this click
            e.stopPropagation();

            if (!e.target.closest('.task-node-meta')) {
                console.log(`Clicked task element, ID: ${taskId}, Title: ${taskTitle}`);
                console.log('Clicked task object:', task);

                // Re-find task from list to ensure using latest data
                const freshTask = this.findTaskById(taskId);
                if (freshTask) {
                    console.log('Found latest task data:', freshTask);
                    this.selectTask(freshTask);
                } else {
                    console.error(`Task ID not found: ${taskId}, trying to fetch from server`);
                    // No longer use potentially incorrect original task object
                    // Instead try to fetch this task from server
                    this.fetchTaskById(taskId).then(fetchedTask => {
                        if (fetchedTask) {
                            console.log('Fetched task from server:', fetchedTask);
                            this.selectTask(fetchedTask);
                        } else {
                            console.error(`Cannot fetch task ID: ${taskId}`);
                            this.showError(`Cannot find task: ${taskTitle}`);
                        }
                    }).catch(err => {
                        console.error(`Fetch task failed: ${err}`);
                        this.showError(`Fetch task failed: ${taskTitle}`);
                    });
                }
            }
        });

        container.appendChild(taskElement);

        // Mark as rendered
        if (this.renderedTasks) {
            this.renderedTasks.add(taskId);
        }

        // If has subtasks, recursively render
        if (subtasks.length > 0) {
            console.log(`Task ${taskId} has ${subtasks.length} subtasks`);
            const subtasksContainer = document.createElement('div');
            subtasksContainer.className = 'task-subtasks';
            taskElement.appendChild(subtasksContainer);

            // Create new visited set, including current task
            const newVisited = new Set(visited);
            newVisited.add(taskId);

            subtasks.forEach((subtask, index) => {
                const subtaskId = subtask.id || subtask.ID || subtask.Id || '';
                const subtaskTitle = subtask.title || subtask.Title || 'No Title';
                console.log(`  Rendering subtask ${index + 1}/${subtasks.length}: ${subtaskId} (${subtaskTitle})`);

                const rendered = this.renderTaskNode(subtask, subtasksContainer, level + 1, newVisited);
                if (!rendered) {
                    console.log(`  Subtask ${subtaskId} render failed or skipped`);
                }
            });
        } else {
            console.log(`Task ${taskId} has no subtasks`);
        }

        return taskElement;
    }

    initSortable() {
        // Initialize sortable functionality
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
            console.log(`Update task order: taskId = , newOrder = `);

            const requestBody = { task_id: taskId, order: newOrder };
            console.log('Request body:', requestBody);

            const response = await fetch(`/api/tasks/order`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody)
            });

            console.log('Response status:', response.status, response.statusText);

            if (!response.ok) {
                let errorMessage = `HTTP error : `;
                try {
                    const errorData = await response.json();
                    console.log('Error response data:', errorData);
                    errorMessage = errorData.error || errorData.message || errorMessage;
                } catch (e) {
                    console.log('Cannot parse error response:', e);
                }
                throw new Error(errorMessage);
            }

            const result = await response.json();
            console.log('Update success:', result);

            // Reload data
            await this.loadData();
        } catch (error) {
            console.error('Update task order failed:', error);
            this.showError(`Update order failed: `);
        }
    }

    selectTask(task) {
        console.log('Select task:', task);

        // Clear previous selection timeout to prevent race conditions
        if (this.selectionTimeout) {
            clearTimeout(this.selectionTimeout);
            this.selectionTimeout = null;
        }

        // Support multiple field names
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || 'No Title';
        console.log(`Task ID: , Task Title: `);

        // Remove previous selection styles
        document.querySelectorAll('.task-node.selected').forEach(el => {
            el.classList.remove('selected');
            // Remove activating class
            el.classList.remove('task-activating');
        });

        // Add selection style
        const taskElement = document.querySelector(`[data-task-id="${taskId}"]`);
        if (taskElement) {
            // First add activating class
            taskElement.classList.add('task-activating');

            // Add selected class after short delay
            this.selectionTimeout = setTimeout(() => {
                taskElement.classList.add('selected');
                taskElement.classList.remove('task-activating');
            }, 300);

            console.log('Found task element, adding selection style and animation');
        } else {
            console.error(`Task element not found: data-task-id="${taskId}"`);
            // Try to find all possible task-ids
            const allTaskElements = document.querySelectorAll('[data-task-id]');
            console.log('All task elements:', Array.from(allTaskElements).map(el => el.dataset.taskId));
        }

        // Show task details
        this.showTaskDetails(task);
        this.currentTask = task;
        console.log('Current task set to:', this.currentTask);
    }

    showTaskDetails(task) {
        console.log('Show task details, task object:', task);
        console.log('Task fields:', {
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

        // Support multiple field names
        const taskTitle = task.title || task.Title || 'No Title';
        const taskStatus = task.status || task.Status || 'planning';
        const taskPriority = task.priority || task.Priority || 3;
        const taskProgress = task.progress || task.Progress || 0;
        const taskDescription = task.description || task.Description || 'No description';
        const taskStartDate = task.start_date || task.startDate || task.StartDate || '-';
        const taskEndDate = task.end_date || task.endDate || task.EndDate || '-';
        const taskEstimatedTime = task.estimated_time || task.estimatedTime || task.EstimatedTime || 0;
        const taskDailyTime = task.daily_time || task.dailyTime || task.DailyTime || 0;
        const taskActualTime = task.actual_time || task.actualTime || task.ActualTime || 0;

        // Update task details
        this.elements.taskTitle.textContent = taskTitle;
        this.elements.taskStatus.textContent = this.getStatusText(taskStatus);
        this.elements.taskStatus.className = `task-status status-${taskStatus.replace('-', '')}`;

        this.elements.taskPriority.textContent = this.getPriorityText(taskPriority);
        this.elements.taskPriority.className = `task-priority priority-${taskPriority}`;

        this.elements.taskProgress.textContent = `${taskProgress}%`;
        this.elements.taskDescription.textContent = taskDescription;
        this.elements.taskStartDate.textContent = taskStartDate;
        this.elements.taskEndDate.textContent = taskEndDate;
        this.elements.taskEstimatedTime.textContent = `${taskEstimatedTime} mins`;
        this.elements.taskDailyTime.textContent = `${taskDailyTime} mins`;
        this.elements.taskActualTime.textContent = `${taskActualTime} mins`;

        // Update tags
        this.elements.taskTags.innerHTML = '';
        if (task.tags && task.tags.length > 0) {
            task.tags.forEach(tag => {
                const tagElement = document.createElement('span');
                tagElement.className = 'tag';
                tagElement.textContent = tag;
                this.elements.taskTags.appendChild(tagElement);
            });
        }

        // Show time analysis panel
        this.elements.timeAnalysisPanel.style.display = 'block';

        // Load time analysis data
        this.loadTaskTimeAnalysis(task.id || task.ID || task.Id);

        // Update time evaluation
        this.updateTimeEvaluation(task);
    }

    // Update time evaluation
    updateTimeEvaluation(task) {
        console.log('Update time evaluation:', task);

        // Get estimated and actual time
        const estimatedTime = task.estimated_time || task.estimatedTime || task.EstimatedTime || 0;
        const actualTime = task.actual_time || task.actualTime || task.ActualTime || 0;
        const taskStatus = task.status || task.Status || 'planning';

        console.log(`Evaluation params: estimatedTime = ${estimatedTime}, actualTime = ${actualTime}, status = ${taskStatus}`);

        // Update base values
        this.elements.evalEstimatedTime.textContent = `${estimatedTime} mins`;
        this.elements.evalActualTime.textContent = `${actualTime} mins`;

        // Check if task is completed
        if (taskStatus !== 'completed') {
            this.elements.evalTimeDiff.textContent = '0 mins';
            this.elements.evalTimePercent.textContent = '0%';
            this.elements.evalResult.innerHTML = '<i class="fas fa-info-circle"></i> Task not completed';
            this.elements.evalResult.className = 'evaluation-result';
            return;
        }

        // Calculate time difference and percentage
        const timeDiff = actualTime - estimatedTime;
        let percentDiff = 0;
        if (estimatedTime > 0) {
            percentDiff = (timeDiff / estimatedTime) * 100;
        }

        // Update difference and percentage display
        const diffText = timeDiff >= 0 ? `+${timeDiff} mins` : `${timeDiff} mins`;
        this.elements.evalTimeDiff.textContent = diffText;
        this.elements.evalTimePercent.textContent = `${percentDiff.toFixed(1)}%`;

        // Set percentage color
        let percentColorClass = '';
        if (percentDiff < -10) percentColorClass = 'eval-good';
        else if (percentDiff <= 10) percentColorClass = 'eval-ok';
        else if (percentDiff <= 30) percentColorClass = 'eval-warning';
        else percentColorClass = 'eval-bad';

        this.elements.evalTimePercent.className = `evaluation-percent ${percentColorClass}`;

        // Determine evaluation result
        let evaluationText = '';
        let evaluationClass = '';

        if (estimatedTime <= 0) {
            evaluationText = '<i class="fas fa-info-circle"></i> Cannot evaluate: Estimated time is 0';
            evaluationClass = '';
        } else if (percentDiff < -10) {
            evaluationText = `<i class="fas fa-check-circle"></i> Completed early (Saved ${Math.abs(percentDiff).toFixed(1)}%)`;
            evaluationClass = 'eval-good';
        } else if (percentDiff <= 10) {
            evaluationText = `<i class="fas fa-check-circle"></i> Completed on time (Deviation ${percentDiff.toFixed(1)}%)`;
            evaluationClass = 'eval-ok';
        } else if (percentDiff <= 30) {
            evaluationText = `<i class="fas fa-exclamation-circle"></i> Slightly overdue (Overdue ${percentDiff.toFixed(1)}%)`;
            evaluationClass = 'eval-warning';
        } else {
            evaluationText = `<i class="fas fa-times-circle"></i> Seriously overdue (Overdue ${percentDiff.toFixed(1)}%)`;
            evaluationClass = 'eval-bad';
        }

        // Update evaluation result
        this.elements.evalResult.innerHTML = evaluationText;
        this.elements.evalResult.className = `evaluation-result ${evaluationClass}`;

        console.log(`Evaluation result: diff = ${timeDiff}, percent = ${percentDiff.toFixed(1)}%, class = ${evaluationClass}`);
    }

    // Load task time analysis data
    loadTaskTimeAnalysis(taskId) {
        if (!taskId) {
            console.error('Cannot load time analysis: Task ID is empty');
            return;
        }

        console.log('Loading task time analysis data, Task ID:', taskId);

        // Show loading state
        this.elements.timeOverlapResult.innerHTML = '<p><i class="fas fa-spinner fa-spin"></i> Detecting...</p>';

        // First get time analysis data
        fetch(`/api/tasks/time-analysis?task_id=${encodeURIComponent(taskId)}`)
            .then(response => {
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status} `);
                }
                return response.json();
            })
            .then(data => {
                if (data.success && data.data) {
                    this.updateTimeAnalysisUI(data.data);
                } else {
                    console.error('Time analysis API returned error:', data.error || data.message);
                    this.showTimeAnalysisError('Failed to fetch time analysis data');
                }
            })
            .catch(error => {
                console.error('Failed to fetch time analysis data:', error);
                this.showTimeAnalysisError('Cannot connect to server');
            });

        // Then get time overlap detection data
        fetch(`/api/tasks/daily-overlap?task_id=${encodeURIComponent(taskId)} `)
            .then(response => {
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status} `);
                }
                return response.json();
            })
            .then(data => {
                if (data.success) {
                    this.updateTimeOverlapUI(data);
                } else {
                    console.error('Time overlap detection API returned error:', data.error || data.message);
                    this.updateTimeOverlapUI({ has_overlap: false, message: 'Detection failed' });
                }
            })
            .catch(error => {
                console.error('Failed to fetch time overlap detection data:', error);
                this.updateTimeOverlapUI({ has_overlap: false, message: 'Detection failed' });
            });
    }

    // Update time analysis UI
    updateTimeAnalysisUI(analysis) {
        console.log('Update time analysis UI:', analysis);

        if (analysis.has_subtasks) {
            // Has subtasks: Show normally
            this.elements.subtasksEstimatedTime.textContent = ` mins`;
            this.elements.subtasksDailyTime.textContent = ` mins`;

            const estimatedDiff = analysis.estimated_time_diff;
            const estimatedDiffText = estimatedDiff >= 0 ? `+ mins` : ` mins`;
            this.elements.estimatedTimeDiff.textContent = `(Diff: )`;

            const dailyDiff = analysis.daily_time_diff;
            const dailyDiffText = dailyDiff >= 0 ? `+ mins` : ` mins`;
            this.elements.dailyTimeDiff.textContent = `(Diff: )`;
        } else {
            // No subtasks (Leaf task): Show friendly text
            this.elements.subtasksEstimatedTime.textContent = 'No subtasks';
            this.elements.subtasksDailyTime.textContent = 'No subtasks';
            this.elements.estimatedTimeDiff.textContent = '(Leaf task)';
            this.elements.dailyTimeDiff.textContent = '(Leaf task)';
        }

        // Update self time (Always show)
        this.elements.selfEstimatedTime.textContent = ` mins`;
        this.elements.selfDailyTime.textContent = ` mins`;

        // Update time status (Use original diff value, status function will handle)
        this.updateTimeStatusUI(this.elements.estimatedTimeStatus, analysis.estimated_time_status, analysis.estimated_time_diff);
        this.updateTimeStatusUI(this.elements.dailyTimeStatus, analysis.daily_time_status, analysis.daily_time_diff);

        // Update subtask count info
        if (analysis.has_subtasks) {
            console.log(`Task has  subtasks`);
        } else {
            console.log('Task has no subtasks');
        }
    }

    // Update time status UI
    updateTimeStatusUI(element, status, diff) {
        // Clear all status classes
        element.classList.remove('status-sufficient', 'status-insufficient', 'status-excessive', 'status-warning', 'status-leaf');

        // Add new status class
        element.classList.add(`status-${status}`);

        // Update icon and text
        const iconClass = this.getStatusIconClass(status);
        const statusText = this.getStatusTextByStatus(status, diff);

        element.innerHTML = `<i class="${iconClass}"></i> ${statusText}`;
    }

    // Get status icon class
    getStatusIconClass(status) {
        switch (status) {
            case 'sufficient':
                return 'fas fa-check-circle';
            case 'insufficient':
                return 'fas fa-exclamation-circle';
            case 'excessive':
                return 'fas fa-info-circle';
            case 'warning':
                return 'fas fa-exclamation-triangle';
            case 'leaf':
                return 'fas fa-leaf';
            default:
                return 'fas fa-question-circle';
        }
    }

    // Get status text by status
    getStatusTextByStatus(status, diff) {
        switch (status) {
            case 'sufficient':
                if (diff < 0) {
                    return `Time sufficient (Surplus  mins)`;
                } else {
                    return 'Time sufficient';
                }
            case 'insufficient':
                return `Time insufficient (Shortage  mins)`;
            case 'excessive':
                return `Time allocation excessive (Excess  mins)`;
            case 'warning':
                return 'Time allocation warning';
            case 'leaf':
                return 'Leaf task (No subtasks)';
            default:
                return 'Unknown status';
        }
    }

    // Update time overlap UI
    updateTimeOverlapUI(data) {
        console.log('Update time overlap UI:', data);

        if (data.has_overlap) {
            this.elements.timeOverlapResult.innerHTML = `
    <p class="overlap-warning" >
        <i class="fas fa-exclamation-triangle"></i> ${data.message || 'Subtask time overlap detected'}
                </p>
        <p style="font-size: 0.85rem; margin-top: 0.5rem; color: #666;">
            Subtask time sum exceeds parent task daily allocation on some dates.
        </p>
    `;
        } else {
            this.elements.timeOverlapResult.innerHTML = `
        <p class="overlap-ok">
            <i class="fas fa-check-circle"></i> ${data.message || 'Subtask time allocation reasonable'}
                </p>
            <p style="font-size: 0.85rem; margin-top: 0.5rem; color: #666;">
                No subtask time overlap detected.
            </p>
        `;
        }
    }

    // Show time analysis error
    showTimeAnalysisError(message) {
        this.elements.timeAnalysisPanel.innerHTML = `
            < div style = "padding: 1rem; text-align: center; color: #dc3545;" >
                <i class="fas fa-exclamation-circle"></i>
                <p>${message}</p>
                <button onclick="location.reload()" style="margin-top: 0.5rem; padding: 0.25rem 0.5rem; background: #dc3545; color: white; border: none; border-radius: 4px; cursor: pointer;">
                    Retry
                </button>
            </div >
            `;
    }

    openModal(mode, parentTask = null) {
        console.log(`Open modal, mode: `);

        if (!this.elements.taskModal) {
            console.error('taskModal element not found');
            return;
        }

        this.elements.taskModal.style.display = 'block';
        console.log('Modal display status set to block');

        this.elements.taskForm.reset();

        // Set default date
        const today = new Date().toISOString().split('T')[0];
        this.elements.startDate.value = today;
        this.elements.endDate.value = today;
        this.elements.progress.value = 0;
        this.elements.progressValue.textContent = '0%';

        switch (mode) {
            case 'add':
                this.elements.modalTitle.textContent = 'Add Root Task';
                this.elements.parentId.value = '';
                break;

            case 'edit':
                console.log('Edit mode, current task:', this.currentTask);
                if (!this.currentTask) {
                    console.error('Cannot edit: Current task is empty');
                    this.showError('Please select a task to edit first');
                    this.closeModal();
                    return;
                }
                console.log('Editing Task ID:', this.currentTask.id || this.currentTask.ID || this.currentTask.Id);
                console.log('Editing Task Title:', this.currentTask.title || this.currentTask.Title);
                this.elements.modalTitle.textContent = 'Edit Task';

                // Critical fix: Must clear parentId field
                this.elements.parentId.value = '';

                this.fillFormWithTask(this.currentTask);
                break;

            case 'addSubtask':
                console.log('Add subtask mode, current task:', this.currentTask);
                if (!this.currentTask) {
                    console.error('Cannot add subtask: Current task is empty');
                    this.showError('Please select a task to add subtask to');
                    this.closeModal();
                    return;
                }
                console.log('Parent Task ID:', this.currentTask.id || this.currentTask.ID || this.currentTask.Id);
                this.elements.modalTitle.textContent = 'Add Subtask';
                this.elements.parentId.value = this.currentTask.id || this.currentTask.ID || this.currentTask.Id;
                break;
        }
    }

    fillFormWithTask(task) {
        console.log('Filling form data:', task);

        // Support multiple field names
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || '';
        const taskDescription = task.description || task.Description || '';
        const taskStatus = task.status || task.Status || 'planning';
        const taskPriority = task.priority || task.Priority || 3;
        const taskStartDate = task.start_date || task.startDate || task.StartDate || '';
        const taskEndDate = task.end_date || task.endDate || task.EndDate || '';
        const taskEstimatedTime = task.estimated_time || task.estimatedTime || task.EstimatedTime || 0;
        const taskDailyTime = task.daily_time || task.dailyTime || task.DailyTime || 0;
        const taskProgress = task.progress || task.Progress || 0;
        const taskTags = task.tags || task.Tags || [];

        this.elements.taskId.value = taskId;
        // Critical fix: Correctly populate parentId
        this.elements.parentId.value = task.parent_id || task.parentId || task.parentID || '';
        this.elements.title.value = taskTitle;
        this.elements.description.value = taskDescription;
        this.elements.status.value = taskStatus;
        this.elements.priority.value = taskPriority.toString();
        this.elements.startDate.value = taskStartDate;
        this.elements.endDate.value = taskEndDate;
        this.elements.estimatedTime.value = taskEstimatedTime;
        this.elements.dailyTime.value = taskDailyTime;
        this.elements.progress.value = taskProgress;
        this.elements.progressValue.textContent = `${taskProgress}%`;
        this.elements.tags.value = taskTags.join(', ');

        // After filling form, check if estimated time calculation is needed
        // Use setTimeout to ensure DOM is updated
        setTimeout(() => {
            this.calculateEstimatedTime();
        }, 0);
    }

    closeModal() {
        this.elements.taskModal.style.display = 'none';
        this.elements.taskForm.reset();
    }

    async saveTask() {
        // Validate form
        if (!this.elements.title.value.trim()) {
            this.showError('Please enter task title');
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
            daily_time: parseInt(this.elements.dailyTime.value) || 0,
            progress: parseInt(this.elements.progress.value) || 0,
            tags: this.elements.tags.value ? this.elements.tags.value.split(',').map(tag => tag.trim()).filter(tag => tag) : []
        };

        // If parent task ID exists, add to data
        // Critical fix: Ensure parent_id does not equal current task ID
        const parentId = this.elements.parentId.value;
        const currentTaskId = this.elements.taskId.value;

        if (parentId && parentId !== currentTaskId) {
            taskData.parent_id = parentId;
        } else if (parentId === currentTaskId) {
            console.warn('Self-reference detected: parent_id equals task_id, ignoring parent_id');
        }

        try {
            let response;
            const taskId = this.elements.taskId.value;

            if (taskId) {
                // Update task
                response = await fetch(`/api/tasks/${taskId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(taskData)
                });
            } else {
                // Create task
                response = await fetch('/api/tasks', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(taskData)
                });
            }

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Save failed');
            }

            console.log('Task saved successfully, reloading data...');
            this.closeModal();

            // Save currently selected task ID to re-select after reload
            const currentTaskId = this.currentTask ? (this.currentTask.id || this.currentTask.ID || this.currentTask.Id) : null;
            console.log('Currently selected task ID:', currentTaskId);

            // Clear current task reference as data is about to refresh
            this.currentTask = null;
            this.elements.taskDetailsCard.style.display = 'none';
            this.elements.timeAnalysisPanel.style.display = 'none';

            await this.loadData();

            // If there was a selected task, try to re-select it
            if (currentTaskId) {
                console.log('Attempting to re-select task:', currentTaskId);
                const task = this.findTaskById(currentTaskId);
                if (task) {
                    console.log('Task found, re-selecting:', task);
                    this.selectTask(task);
                } else {
                    console.log('Task not found, ID might have changed or structure updated');
                    // Try finding by title or other means
                    const savedResponse = await response.json();
                    if (savedResponse.data && savedResponse.data.id) {
                        const newTask = this.findTaskById(savedResponse.data.id);
                        if (newTask) {
                            console.log('Found new task via API response:', newTask);
                            this.selectTask(newTask);
                        }
                    }
                }
            }

            this.showSuccess('Saved successfully');

        } catch (error) {
            console.error('Save task failed:', error);
            this.showError(`Save failed: `);
        }
    }

    async deleteCurrentTask() {
        if (!this.currentTask || !confirm('Are you sure you want to delete this task?')) {
            return;
        }

        try {
            const response = await fetch(`/api/tasks/${this.currentTask.id}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                throw new Error('Delete failed');
            }

            this.currentTask = null;
            this.elements.taskDetailsCard.style.display = 'none';
            this.elements.timeAnalysisPanel.style.display = 'none';
            await this.loadData();
            this.showSuccess('Deleted successfully');

        } catch (error) {
            console.error('Delete task failed:', error);
            this.showError('Delete failed');
        }
    }

    updateStatistics(stats) {
        console.log('Update statistics, received data:', stats);
        console.log('All fields:', Object.keys(stats));
        console.log('Field details:', {
            total_tasks: stats.total_tasks,
            completed_tasks: stats.completed_tasks,
            in_progress_tasks: stats.in_progress_tasks,
            blocked_tasks: stats.blocked_tasks,
            total_time: stats.total_time,
            status_distribution: stats.status_distribution,
            // Time analysis fields
            daily_available_time: stats.daily_available_time,
            total_daily_time: stats.total_daily_time,
            required_days: stats.required_days,
            time_margin: stats.time_margin,
            time_utilization: stats.time_utilization,
            time_status: stats.time_status
        });

        // Debug: Check completed_tasks value
        console.log('completed_tasks value:', stats.completed_tasks);
        console.log('completed_tasks type:', typeof stats.completed_tasks);

        this.elements.totalTasks.textContent = stats.total_tasks || 0;
        this.elements.completedTasks.textContent = stats.completed_tasks || 0;
        this.elements.inProgressTasks.textContent = stats.in_progress_tasks || 0;
        this.elements.blockedTasks.textContent = stats.blocked_tasks || 0;

        // Convert minutes to hours
        const totalHours = Math.round((stats.total_time || 0) / 60);
        this.elements.totalTime.textContent = `${totalHours} h`;
    }

    renderCharts(stats) {
        // Destroy previous charts
        Object.values(this.charts).forEach(chart => {
            if (chart) chart.destroy();
        });

        // Status distribution chart
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

        // Priority distribution chart
        const priorityCtx = this.elements.priorityChart.getContext('2d');
        this.charts.priority = new Chart(priorityCtx, {
            type: 'bar',
            data: {
                labels: Object.keys(stats.priority_distribution || {}).map(key => this.getPriorityText(parseInt(key))),
                datasets: [{
                    label: 'Task Count',
                    data: Object.values(stats.priority_distribution || {}),
                    backgroundColor: [
                        '#ffebee', // Highest
                        '#fff3e0', // High
                        '#e8f5e9', // Medium
                        '#e3f2fd', // Low
                        '#f3e5f5'  // Lowest
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

            // Format as YYYY-MM-DD
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            return `${year} -${month} -${day} `;
        } catch (error) {
            console.error('Date formatting error:', error);
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

            if (diffDays === 0) return 'Today';
            if (diffDays === 1) return '1 day';
            return ` days`;
        } catch (error) {
            console.error('Calculate duration error:', error);
            return '-';
        }
    }

    addTimelineEventListeners() {
        // Add event listeners for view task buttons
        const viewButtons = document.querySelectorAll('.view-task-btn');
        viewButtons.forEach(button => {
            button.addEventListener('click', (e) => {
                e.stopPropagation();
                const taskId = button.getAttribute('data-task-id');
                console.log('View timeline task:', taskId);

                const task = this.findTaskById(taskId);
                if (task) {
                    this.selectTask(task);
                    // Switch to task tree tab
                    const taskTabLink = document.querySelector('.tab-link[data-tab="taskTab"]');
                    if (taskTabLink) {
                        taskTabLink.click();
                    }
                } else {
                    this.showError('Task not found');
                }
            });
        });

        // Add click event for timeline markers - toggle display content
        const timelineMarkers = document.querySelectorAll('.timeline-marker');
        timelineMarkers.forEach(marker => {
            marker.addEventListener('click', (e) => {
                e.stopPropagation();
                const taskId = marker.getAttribute('data-task-id');
                const index = parseInt(marker.getAttribute('data-index'));
                console.log('Clicked timeline marker:', taskId, 'Index:', index);

                // First show corresponding task card
                this.showTimelineTaskCard(index);

                // Then update active state (card already rendered)
                this.updateTimelineActiveState(index);
            });
        });
    }

    updateTimelineActiveState(activeIndex) {
        // Remove all active states
        const markers = document.querySelectorAll('.timeline-marker');
        const cards = document.querySelectorAll('.timeline-card-item');

        markers.forEach(marker => {
            marker.classList.remove('active');
        });

        cards.forEach(card => {
            card.classList.remove('active');
        });

        // Add current active state
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
            console.error('Timeline task not found:', index);
            return;
        }

        const task = this.timelineTasks[index];
        const placeholder = document.getElementById('timelineContentPlaceholder');

        if (!placeholder) {
            console.error('Timeline content placeholder not found');
            return;
        }

        // Clear and add new task card
        placeholder.innerHTML = this.renderTimelineTaskCard(task, index, true);

        // Re-bind view task button event
        const viewButton = placeholder.querySelector('.view-task-btn');
        if (viewButton) {
            viewButton.addEventListener('click', (e) => {
                e.stopPropagation();
                const taskId = viewButton.getAttribute('data-task-id');
                console.log('View timeline task:', taskId);

                const task = this.findTaskById(taskId);
                if (task) {
                    this.selectTask(task);
                    // Switch to task tree tab
                    const taskTabLink = document.querySelector('.tab-link[data-tab="taskTab"]');
                    if (taskTabLink) {
                        taskTabLink.click();
                    }
                } else {
                    this.showError('Task not found');
                }
            });
        }

        console.log('Show timeline task card:', index, task.title || task.Title);
    }

    renderTimelineTaskCard(task, index, isActive = false) {
        // Support multiple field names
        const taskId = task.id || task.ID || task.Id || '';
        const taskTitle = task.title || task.Title || 'No Title';
        const taskStatus = task.status || task.Status || 'planning';
        const taskStartDate = task.start_date || task.StartDate || task.startDate || '';
        const taskEndDate = task.end_date || task.EndDate || task.endDate || '';
        const taskProgress = task.progress || task.Progress || 0;
        const taskDescription = task.description || task.Description || '';
        const taskPriority = task.priority || task.Priority || 3;
        const taskEstimatedTime = task.estimated_time || task.EstimatedTime || task.estimatedTime || 0;

        const statusClass = `status-${taskStatus.replace('-', '')}`;
        const priorityClass = `priority-${taskPriority}`;

        // Format date
        const formattedStartDate = this.formatDateForDisplay(taskStartDate);
        const formattedEndDate = this.formatDateForDisplay(taskEndDate);

        // Calculate duration
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
                            <span class="date-label">Start</span>
                            <span class="date-value highlight-date">${formattedStartDate}</span>
                        </div>
                        <div class="date-item">
                            <i class="fas fa-flag-checkered"></i>
                            <span class="date-label">End</span>
                            <span class="date-value highlight-date">${formattedEndDate}</span>
                        </div>
                        <div class="date-item">
                            <i class="fas fa-clock"></i>
                            <span class="date-label">Duration</span>
                            <span class="date-value">${durationText}</span>
                        </div>
                    </div>
                </div>
                <div class="timeline-card-progress">
                    <div class="progress-info">
                        <span class="progress-label">Progress</span>
                        <span class="progress-value">${taskProgress}%</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${taskProgress}%"></div>
                    </div>
                </div>
                ${taskDescription ? `<div class="timeline-card-description">${this.escapeHtml(taskDescription)}</div>` : ''}
                <div class="timeline-card-footer">
                    <span class="estimated-time"><i class="fas fa-hourglass-half"></i> ${taskEstimatedTime}鍒嗛挓</span>
                    <button class="btn btn-small btn-outline view-task-btn" data-task-id="${taskId}">
                        <i class="fas fa-eye"></i> View Task
                    </button>
                </div>
            </div>
        `;
    }

    renderTimeline(timelineData) {
        console.log('Render timeline data:', timelineData);
        // Simplified timeline rendering
        let tasks = timelineData.tasks || [];

        if (tasks.length === 0) {
            this.elements.timelineChart.innerHTML = `
                <div class="empty-state">
                    <i class="fas fa-calendar-alt"></i>
                    <p>No timeline data</p>
                </div>
            `;
            return;
        }

        // Sort on frontend to ensure consistency
        tasks.sort((a, b) => {
            // Support multiple field names
            const aStartDateStr = a.start_date || a.StartDate || a.startDate || '';
            const bStartDateStr = b.start_date || b.StartDate || b.startDate || '';
            const aEndDateStr = a.end_date || a.EndDate || a.endDate || '';
            const bEndDateStr = b.end_date || b.EndDate || b.endDate || '';
            const aParentID = a.parent_id || a.ParentID || a.parentId || '';
            const bParentID = b.parent_id || b.ParentID || b.parentId || '';

            // Convert date strings to Date objects for comparison
            const parseDate = (dateStr) => {
                if (!dateStr || dateStr === '-') return new Date(0); // Set empty date to min date
                const date = new Date(dateStr);
                return isNaN(date.getTime()) ? new Date(0) : date;
            };

            const aStartDate = parseDate(aStartDateStr);
            const bStartDate = parseDate(bStartDateStr);
            const aEndDate = parseDate(aEndDateStr);
            const bEndDate = parseDate(bEndDateStr);

            // 1. First, parent tasks prioritize over subtasks
            // If a is parent (parent_id empty) and b is subtask, a should be first
            if (aParentID === '' && bParentID !== '') {
                return -1;
            }
            // If a is subtask and b is parent, b should be first
            if (aParentID !== '' && bParentID === '') {
                return 1;
            }

            // 2. Both parent or both subtasks, sort by start time (earliest first)
            if (aStartDate.getTime() !== bStartDate.getTime()) {
                return aStartDate.getTime() - bStartDate.getTime();
            }

            // 3. Start time same, sort by end time
            return aEndDate.getTime() - bEndDate.getTime();
        });

        console.log('Sorted timeline tasks:', tasks);

        // Debug: Show detailed info for each task
        console.log('Task details:');
        tasks.forEach((task, index) => {
            const taskId = task.id || task.ID || task.Id || '';
            const taskTitle = task.title || task.Title || 'No Title';
            const startDate = task.start_date || task.StartDate || task.startDate || '';
            const endDate = task.end_date || task.EndDate || task.endDate || '';
            const parentID = task.parent_id || task.ParentID || task.parentId || '';
            console.log(`[] ID: , Title: "", Start: , End: , ParentID: ""`);
        });

        // Create visual timeline
        let html = `
            <div class="timeline-container">
                <div class="timeline-axis">
                    <div class="timeline-line"></div>
        `;

        tasks.forEach((task, index) => {
            // Support multiple field names
            const taskId = task.id || task.ID || task.Id || '';
            const taskTitle = task.title || task.Title || 'No Title';
            const taskStatus = task.status || task.Status || 'planning';
            const taskStartDate = task.start_date || task.StartDate || task.startDate || '';
            const taskEndDate = task.end_date || task.EndDate || task.endDate || '';
            const taskProgress = task.progress || task.Progress || 0;
            const taskDescription = task.description || task.Description || '';
            const taskPriority = task.priority || task.Priority || 3;

            const statusClass = `status-${taskStatus.replace('-', '')}`;
            const priorityClass = `priority-${taskPriority}`;

            // Get status icon
            const statusIcon = this.getStatusIcon(taskStatus);

            // Format date
            const formattedStartDate = this.formatDateForDisplay(taskStartDate);
            const formattedEndDate = this.formatDateForDisplay(taskEndDate);

            // First task active by default
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
                        <!-- Content will load dynamically based on clicked time point -->
        `;

        // Default show first task
        if (tasks.length > 0) {
            const firstTask = tasks[0];
            html += this.renderTimelineTaskCard(firstTask, 0, true);
        }

        html += `
                    </div>
                </div>
        `;

        this.elements.timelineChart.innerHTML = html;

        // Store task data for click usage
        this.timelineTasks = tasks;

        // Add timeline interaction event listeners
        this.addTimelineEventListeners();

        console.log('Timeline rendering complete');
    }


    // 宸ュ叿鏂规硶
    getStatusText(status) {
        const statusMap = {
            'planning': 'Planning',
            'in-progress': 'In Progress',
            'completed': 'Completed',
            'blocked': 'Blocked',
            'cancelled': 'Cancelled'
        };
        return statusMap[status] || status;
    }

    getPriorityText(priority) {
        const priorityMap = {
            1: 'Highest',
            2: 'High',
            3: 'Medium',
            4: 'Low',
            5: 'Lowest'
        };
        return priorityMap[priority] || `Priority `;
    }

    findTaskById(taskId) {
        if (!taskId) return null;

        // First search in flat task list
        const flatResult = this.tasks.find(task => {
            // Support multiple possible field names
            const taskIdToCompare = task.id || task.ID || task.Id || '';
            return taskIdToCompare === taskId;
        });

        if (flatResult) {
            return flatResult;
        }

        // If not found in flat list, recursively search in task tree
        return this.findTaskInTree(taskId, this.tasks);
    }

    findTaskInTree(taskId, tasks) {
        if (!taskId || !tasks || !Array.isArray(tasks)) return null;

        for (const task of tasks) {
            // Check current task
            const taskIdToCompare = task.id || task.ID || task.Id || '';
            if (taskIdToCompare === taskId) {
                return task;
            }

            // Recursively check subtasks
            if (task.subtasks && Array.isArray(task.subtasks) && task.subtasks.length > 0) {
                const foundInSubtasks = this.findTaskInTree(taskId, task.subtasks);
                if (foundInSubtasks) {
                    return foundInSubtasks;
                }
            }

            // Also check Subtasks field (capitalized)
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
            console.log(`Fetch task ID from server: `);
            const response = await fetch(`/api/tasks/${taskId}`);
            if (!response.ok) {
                console.error(`Fetch task failed, status code: `);
                return null;
            }
            const data = await response.json();
            console.log('Task data fetched from server:', data);
            return data.data || null;
        } catch (error) {
            console.error(`Fetch task exception: `);
            return null;
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    getRootTaskID() {
        // Prioritize getting from body data attribute
        const body = document.body;
        if (body && body.dataset.rootTaskId) {
            return body.dataset.rootTaskId;
        }

        // Then get from URL parameter
        const urlParams = new URLSearchParams(window.location.search);
        const rootParam = urlParams.get('root');
        if (rootParam) {
            return rootParam;
        }

        // No root task ID
        return '';
    }

    showLoading() {
        // Can add loading indicator
        this.elements.refreshBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
    }

    hideLoading() {
        this.elements.refreshBtn.innerHTML = '<i class="fas fa-sync-alt"></i>';
    }

    showError(message) {
        alert(`閿欒: ${message}`);
    }

    showSuccess(message) {
        // Can add more elegant success prompt
        console.log(`Success: `);
    }


    // ==================== 鏃堕棿瓒嬪娍鐩稿叧鏂规硶 ====================

    async loadTrendsData() {
        try {
            console.log('Start loading trends data...');

            // Show loading state
            if (this.elements.refreshTrendsBtn) {
                this.elements.refreshTrendsBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
            }

            // Build request URL
            let url = '/api/tasks/trends';
            const params = new URLSearchParams();

            // Add root task ID (if any)
            if (this.rootTaskID) {
                params.append('root', this.rootTaskID);
            }

            // Add time range
            const timeRange = this.elements.timeRangeSelect ? this.elements.timeRangeSelect.value : '30d';
            params.append('range', timeRange);

            url += '?' + params.toString();

            console.log('Request URL:', url);

            const response = await fetch(url);
            if (!response.ok) {
                throw new Error(`HTTP Error! Status: `);
            }

            const result = await response.json();
            console.log('Trends data loaded:', result);

            if (result.success && result.data) {
                this.renderTrendsCharts(result.data);
            } else {
                console.error('API returned error:', result.error || 'Unknown error');
                this.showError('Failed to load trends data');
            }
        } catch (error) {
            console.error('Failed to load trends data:', error);
            this.showError('Failed to load trends data: ' + error.message);
        } finally {
            // Restore button state
            if (this.elements.refreshTrendsBtn) {
                this.elements.refreshTrendsBtn.innerHTML = '<i class="fas fa-sync-alt"></i>';
            }
        }
    }

    renderTrendsCharts(trendsData) {
        console.log('Rendering trends charts...', trendsData);

        // Destroy previous trends charts
        ['creation', 'completion', 'progress'].forEach(chartName => {
            if (this.charts[chartName + 'Trend']) {
                this.charts[chartName + 'Trend'].destroy();
                delete this.charts[chartName + 'Trend'];
            }
        });

        // Render creation trend chart
        if (trendsData.creation_trend && this.elements.creationTrendChart) {
            this.renderTrendChart(
                'creationTrend',
                trendsData.creation_trend,
                this.elements.creationTrendChart,
                'line'
            );
        }

        // Render completion trend chart
        if (trendsData.completion_trend && this.elements.completionTrendChart) {
            this.renderTrendChart(
                'completionTrend',
                trendsData.completion_trend,
                this.elements.completionTrendChart,
                'line'
            );
        }

        // Render progress trend chart
        if (trendsData.progress_trend && this.elements.progressTrendChart) {
            this.renderTrendChart(
                'progressTrend',
                trendsData.progress_trend,
                this.elements.progressTrendChart,
                'line'
            );
        }

        console.log('Trends charts rendering complete');
    }

    renderTrendChart(chartName, trendData, canvasElement, chartType = 'line') {
        if (!trendData || !trendData.data_points || trendData.data_points.length === 0) {
            console.warn(`No data available for chart: `);
            return;
        }

        const ctx = canvasElement.getContext('2d');

        // Prepare data
        const labels = trendData.data_points.map(point => {
            // Simplify date display, e.g. "01-15"
            const date = new Date(point.date);
            return `${(date.getMonth() + 1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')}`;
        });

        const dataPoints = trendData.data_points.map(point => point.value);

        // Set color based on trend
        let borderColor, backgroundColor;
        switch (trendData.trend) {
            case 'up':
                borderColor = '#4CAF50'; // Green
                backgroundColor = 'rgba(76, 175, 80, 0.1)';
                break;
            case 'down':
                borderColor = '#f44336'; // Red
                backgroundColor = 'rgba(244, 67, 54, 0.1)';
                break;
            default:
                borderColor = '#2196F3'; // Blue
                backgroundColor = 'rgba(33, 150, 243, 0.1)';
                break;
        }

        // Create chart config
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
                    tension: 0.4, // Curve smoothness
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
                            label: function (context) {
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
                            text: 'Date'
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

        // 鍒涘缓鍥捐〃
        this.charts[chartName] = new Chart(ctx, config);
    }

    // 鍦ㄥ垵濮嬪寲鏃跺姞杞借秼鍔挎暟鎹?
    async loadInitialTrendsData() {
        // 绛夊緟涓€灏忔鏃堕棿纭繚DOM瀹屽叏鍔犺浇
        setTimeout(() => {
            if (this.elements.creationTrendChart &&
                this.elements.completionTrendChart &&
                this.elements.progressTrendChart) {
                this.loadTrendsData();
            }
        }, 500);
    }
}


// 椤甸潰鍔犺浇瀹屾垚鍚庡垵濮嬪寲搴旂敤
document.addEventListener('DOMContentLoaded', () => {
    window.taskApp = new TaskBreakdownApp();
});

