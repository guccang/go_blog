/**
 * 已完成任务页面JavaScript
 */

class CompletedTasksApp {
    constructor() {
        this.completedTasks = [];
        this.allTasks = []; // 保存所有任务，用于查找子任务
        this.init();
    }

    init() {
        // DOM元素
        this.elements = {
            completedTasksList: document.getElementById('completedTasksList'),
            refreshBtn: document.getElementById('refreshBtn'),
            searchInput: document.getElementById('searchInput'),
            totalCompletedTasks: document.getElementById('totalCompletedTasks'),
            totalTimeSpent: document.getElementById('totalTimeSpent'),
            avgCompletionTime: document.getElementById('avgCompletionTime')
        };

        // 绑定事件
        this.bindEvents();

        // 加载数据
        this.loadCompletedTasks();
    }

    bindEvents() {
        // 刷新按钮
        this.elements.refreshBtn.addEventListener('click', () => {
            this.loadCompletedTasks();
        });

        // 搜索输入
        this.elements.searchInput.addEventListener('input', (e) => {
            this.filterTasks(e.target.value);
        });
    }

    async loadCompletedTasks() {
        try {
            this.showLoading();

            // 加载所有任务，而不仅仅是已完成的根任务
            // 这样我们可以在loadSubtasks中查找子任务
            const response = await fetch('/api/tasks');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();
            if (!result.success) {
                throw new Error(result.error || 'API请求失败');
            }

            const allTasks = result.data || [];

            // 过滤出已完成的根任务
            this.completedTasks = allTasks.filter(task => {
                const parentId = task.parent_id || task.parentId || task.parentID || '';
                const isRoot = !parentId || parentId === '';
                const taskStatus = task.status || task.Status || 'planning';
                const isCompleted = taskStatus === 'completed' || task.progress === 100;
                const isDeleted = task.deleted || task.Deleted || false;
                return isRoot && isCompleted && !isDeleted;
            });

            // 保存所有任务供loadSubtasks使用
            this.allTasks = allTasks;

            this.renderTasks();
            this.updateStatistics();
        } catch (error) {
            console.error('加载已完成任务失败:', error);
            this.showError('加载已完成任务失败，请刷新页面重试');
        }
    }

    renderTasks() {
        if (this.completedTasks.length === 0) {
            this.elements.completedTasksList.innerHTML = `
                <div class="empty-state">
                    <i class="fas fa-check-circle fa-3x"></i>
                    <h3>暂无已完成的任务</h3>
                    <p>当有任务标记为已完成时，它们会显示在这里</p>
                </div>
            `;
            return;
        }

        let html = '';
        this.completedTasks.forEach(task => {
            const priorityClass = this.getPriorityClass(task.priority);
            const completionDate = task.updated_at || task.created_at;
            const formattedDate = completionDate ? new Date(completionDate).toLocaleDateString('zh-CN') : '未知';

            html += `
                <div class="task-card completed-task-card" data-task-id="${task.id}">
                    <div class="task-card-header">
                        <div class="task-title-section">
                            <h3 class="task-title">${this.escapeHtml(task.title)}</h3>
                            <span class="task-status-badge completed">已完成</span>
                        </div>
                        <div class="task-meta">
                            <span class="task-priority ${priorityClass}">
                                <i class="fas fa-flag"></i> ${this.getPriorityText(task.priority)}
                            </span>
                            <span class="task-date">
                                <i class="far fa-calendar-check"></i> ${formattedDate}
                            </span>
                        </div>
                    </div>

                    ${task.description ? `
                    <div class="task-description">
                        ${this.escapeHtml(task.description)}
                    </div>
                    ` : ''}

                    <div class="task-stats">
                        <div class="task-stat">
                            <span class="stat-label">进度:</span>
                            <span class="stat-value">${task.progress || 100}%</span>
                        </div>
                        ${task.actual_time ? `
                        <div class="task-stat">
                            <span class="stat-label">实际耗时:</span>
                            <span class="stat-value">${this.formatTime(task.actual_time)}</span>
                        </div>
                        ` : ''}
                        ${task.estimated_time ? `
                        <div class="task-stat">
                            <span class="stat-label">预估时间:</span>
                            <span class="stat-value">${this.formatTime(task.estimated_time)}</span>
                        </div>
                        ` : ''}
                        ${task.start_date && task.end_date ? `
                        <div class="task-stat">
                            <span class="stat-label">时间范围:</span>
                            <span class="stat-value">${task.start_date} ~ ${task.end_date}</span>
                        </div>
                        ` : ''}
                    </div>

                    ${task.tags && task.tags.length > 0 ? `
                    <div class="task-tags">
                        ${task.tags.map(tag => `
                            <span class="tag">${this.escapeHtml(tag)}</span>
                        `).join('')}
                    </div>
                    ` : ''}
                </div>
            `;
        });

        this.elements.completedTasksList.innerHTML = html;

        // 为每个任务卡片添加点击事件
        document.querySelectorAll('.completed-task-card').forEach(card => {
            card.addEventListener('click', (e) => {
                // 防止点击按钮或链接时触发
                if (e.target.tagName === 'BUTTON' || e.target.tagName === 'A' || e.target.closest('button') || e.target.closest('a')) {
                    return;
                }

                const taskId = card.dataset.taskId;
                this.showTaskDetails(taskId);
            });

            // 添加悬停效果
            card.style.cursor = 'pointer';
            card.addEventListener('mouseenter', () => {
                card.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.15)';
            });
            card.addEventListener('mouseleave', () => {
                card.style.boxShadow = '';
            });
        });
    }

    filterTasks(searchTerm) {
        if (!searchTerm.trim()) {
            this.renderTasks();
            return;
        }

        const filteredTasks = this.completedTasks.filter(task => {
            const searchLower = searchTerm.toLowerCase();
            return (
                (task.title && task.title.toLowerCase().includes(searchLower)) ||
                (task.description && task.description.toLowerCase().includes(searchLower)) ||
                (task.tags && task.tags.some(tag => tag.toLowerCase().includes(searchLower)))
            );
        });

        if (filteredTasks.length === 0) {
            this.elements.completedTasksList.innerHTML = `
                <div class="empty-state">
                    <i class="fas fa-search fa-3x"></i>
                    <h3>未找到匹配的任务</h3>
                    <p>尝试使用不同的关键词搜索</p>
                </div>
            `;
            return;
        }

        // 临时渲染过滤后的任务
        const originalTasks = this.completedTasks;
        this.completedTasks = filteredTasks;
        this.renderTasks();
        this.completedTasks = originalTasks;
    }

    updateStatistics() {
        // 总任务数
        this.elements.totalCompletedTasks.textContent = this.completedTasks.length;

        // 总耗时
        const totalTime = this.completedTasks.reduce((sum, task) => sum + (task.actual_time || 0), 0);
        this.elements.totalTimeSpent.textContent = (totalTime / 60).toFixed(1); // 转换为小时

        // 平均完成时间（如果有开始和结束日期）
        const tasksWithDates = this.completedTasks.filter(task => task.start_date && task.end_date);
        if (tasksWithDates.length > 0) {
            const totalDays = tasksWithDates.reduce((sum, task) => {
                const start = new Date(task.start_date);
                const end = new Date(task.end_date);
                const days = Math.ceil((end - start) / (1000 * 60 * 60 * 24));
                return sum + Math.max(1, days); // 至少1天
            }, 0);
            const avgDays = (totalDays / tasksWithDates.length).toFixed(1);
            this.elements.avgCompletionTime.textContent = avgDays;
        } else {
            this.elements.avgCompletionTime.textContent = 'N/A';
        }
    }

    getPriorityClass(priority) {
        switch (priority) {
            case 1: return 'priority-highest';
            case 2: return 'priority-high';
            case 3: return 'priority-medium';
            case 4: return 'priority-low';
            case 5: return 'priority-lowest';
            default: return 'priority-medium';
        }
    }

    getPriorityText(priority) {
        switch (priority) {
            case 1: return '最高';
            case 2: return '高';
            case 3: return '中';
            case 4: return '低';
            case 5: return '最低';
            default: return '中';
        }
    }

    formatTime(minutes) {
        if (minutes < 60) {
            return `${minutes}分钟`;
        } else if (minutes < 60 * 24) {
            return `${(minutes / 60).toFixed(1)}小时`;
        } else {
            return `${(minutes / 60 / 24).toFixed(1)}天`;
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    showLoading() {
        this.elements.completedTasksList.innerHTML = `
            <div class="loading-indicator">
                <i class="fas fa-spinner fa-spin"></i> 加载已完成任务...
            </div>
        `;
    }

    showError(message) {
        this.elements.completedTasksList.innerHTML = `
            <div class="error-state">
                <i class="fas fa-exclamation-triangle fa-3x"></i>
                <h3>加载失败</h3>
                <p>${message}</p>
                <button class="btn btn-primary" onclick="app.loadCompletedTasks()">重试</button>
            </div>
        `;
    }

    async showTaskDetails(taskId) {
        try {
            // 查找任务 - 从所有任务中查找以获取完整信息
            const task = this.allTasks.find(t => {
                const tId = t.id || t.ID || t.Id || '';
                return tId === taskId;
            });
            if (!task) {
                console.error('任务未找到:', taskId);
                return;
            }

            // 创建模态框
            const modal = document.createElement('div');
            modal.className = 'modal';
            modal.style.display = 'block';
            modal.style.position = 'fixed';
            modal.style.zIndex = '1000';
            modal.style.left = '0';
            modal.style.top = '0';
            modal.style.width = '100%';
            modal.style.height = '100%';
            modal.style.backgroundColor = 'rgba(0, 0, 0, 0.5)';
            modal.style.overflow = 'auto';

            // 模态框内容
            modal.innerHTML = `
                <div class="modal-content" style="background-color: #fff; margin: 5% auto; padding: 20px; border-radius: 8px; width: 80%; max-width: 800px; max-height: 80vh; overflow-y: auto;">
                    <div class="modal-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; border-bottom: 1px solid #eee; padding-bottom: 10px;">
                        <h2 style="margin: 0;">${this.escapeHtml(task.title)}</h2>
                        <button class="close-btn" style="background: none; border: none; font-size: 24px; cursor: pointer; color: #666;">&times;</button>
                    </div>

                    <div class="modal-body">
                        <!-- 任务基本信息 -->
                        <div class="task-info-section" style="margin-bottom: 30px;">
                            <h3 style="margin-top: 0; color: #333;">任务信息</h3>
                            <div class="task-info-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 15px;">
                                <div class="info-item">
                                    <strong>状态:</strong> <span class="task-status-badge completed" style="background-color: #28a745; color: white; padding: 2px 8px; border-radius: 12px; font-size: 12px;">已完成</span>
                                </div>
                                <div class="info-item">
                                    <strong>优先级:</strong> <span class="task-priority ${this.getPriorityClass(task.priority)}">${this.getPriorityText(task.priority)}</span>
                                </div>
                                <div class="info-item">
                                    <strong>进度:</strong> ${task.progress || 100}%
                                </div>
                                ${task.start_date ? `
                                <div class="info-item">
                                    <strong>开始日期:</strong> ${task.start_date}
                                </div>
                                ` : ''}
                                ${task.end_date ? `
                                <div class="info-item">
                                    <strong>结束日期:</strong> ${task.end_date}
                                </div>
                                ` : ''}
                                ${task.estimated_time ? `
                                <div class="info-item">
                                    <strong>预估时间:</strong> ${this.formatTime(task.estimated_time)}
                                </div>
                                ` : ''}
                                ${task.actual_time ? `
                                <div class="info-item">
                                    <strong>实际耗时:</strong> ${this.formatTime(task.actual_time)}
                                </div>
                                ` : ''}
                            </div>
                        </div>

                        <!-- 任务描述 -->
                        ${task.description ? `
                        <div class="task-description-section" style="margin-bottom: 30px;">
                            <h3 style="margin-top: 0; color: #333;">任务描述</h3>
                            <div style="background-color: #f8f9fa; padding: 15px; border-radius: 6px; border-left: 4px solid #007bff;">
                                ${this.escapeHtml(task.description).replace(/\n/g, '<br>')}
                            </div>
                        </div>
                        ` : ''}

                        <!-- 子任务 -->
                        <div class="subtasks-section" style="margin-bottom: 30px;">
                            <h3 style="margin-top: 0; color: #333;">子任务</h3>
                            <div id="subtasksList" style="margin-top: 10px;">
                                <div class="loading-indicator">
                                    <i class="fas fa-spinner fa-spin"></i> 加载子任务...
                                </div>
                            </div>
                        </div>

                        <!-- 标签 -->
                        ${task.tags && task.tags.length > 0 ? `
                        <div class="task-tags-section" style="margin-bottom: 30px;">
                            <h3 style="margin-top: 0; color: #333;">标签</h3>
                            <div class="tags-container" style="display: flex; flex-wrap: wrap; gap: 8px;">
                                ${task.tags.map(tag => `
                                    <span class="tag" style="background-color: #e9ecef; padding: 4px 12px; border-radius: 16px; font-size: 14px;">${this.escapeHtml(tag)}</span>
                                `).join('')}
                            </div>
                        </div>
                        ` : ''}
                    </div>

                    <div class="modal-footer" style="margin-top: 20px; padding-top: 20px; border-top: 1px solid #eee; text-align: right;">
                        <button class="btn btn-secondary close-modal-btn" style="padding: 8px 16px; background-color: #6c757d; color: white; border: none; border-radius: 4px; cursor: pointer;">关闭</button>
                    </div>
                </div>
            `;

            // 添加到页面
            document.body.appendChild(modal);

            // 绑定关闭事件
            const closeBtn = modal.querySelector('.close-btn');
            const closeModalBtn = modal.querySelector('.close-modal-btn');
            const closeModal = () => {
                document.body.removeChild(modal);
            };

            closeBtn.addEventListener('click', closeModal);
            closeModalBtn.addEventListener('click', closeModal);
            modal.addEventListener('click', (e) => {
                if (e.target === modal) {
                    closeModal();
                }
            });

            // 加载子任务
            await this.loadSubtasks(taskId, modal);

        } catch (error) {
            console.error('显示任务详情失败:', error);
            alert('加载任务详情失败: ' + error.message);
        }
    }

    async loadSubtasks(taskId, modal) {
        try {
            const subtasksList = modal.querySelector('#subtasksList');

            // 从所有任务中查找子任务
            const subtasks = this.allTasks.filter(task => {
                const parentId = task.parent_id || task.parentId || task.parentID || '';
                return parentId === taskId;
            });

            if (subtasks.length === 0) {
                subtasksList.innerHTML = `
                    <div class="empty-state" style="text-align: center; padding: 20px; color: #6c757d;">
                        <i class="fas fa-check-circle fa-2x" style="margin-bottom: 10px;"></i>
                        <p>暂无子任务</p>
                    </div>
                `;
                return;
            }

            // 渲染子任务
            let html = '<div class="subtasks-container" style="display: flex; flex-direction: column; gap: 10px;">';
            subtasks.forEach(subtask => {
                const priorityClass = this.getPriorityClass(subtask.priority);
                const completionDate = subtask.updated_at || subtask.created_at;
                const formattedDate = completionDate ? new Date(completionDate).toLocaleDateString('zh-CN') : '未知';

                html += `
                    <div class="subtask-card" style="border: 1px solid #dee2e6; border-radius: 6px; padding: 15px; background-color: #f8f9fa;">
                        <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 10px;">
                            <div>
                                <h4 style="margin: 0 0 5px 0;">${this.escapeHtml(subtask.title)}</h4>
                                <div style="display: flex; gap: 10px; font-size: 14px; color: #6c757d;">
                                    <span class="task-status-badge completed" style="background-color: #28a745; color: white; padding: 2px 8px; border-radius: 12px;">已完成</span>
                                    <span class="task-priority ${priorityClass}">
                                        <i class="fas fa-flag"></i> ${this.getPriorityText(subtask.priority)}
                                    </span>
                                    <span><i class="far fa-calendar-check"></i> ${formattedDate}</span>
                                </div>
                            </div>
                            <span style="font-size: 16px; font-weight: bold; color: #28a745;">${subtask.progress || 100}%</span>
                        </div>

                        ${subtask.description ? `
                        <div style="margin-top: 10px; padding: 10px; background-color: white; border-radius: 4px; border-left: 3px solid #007bff;">
                            ${this.escapeHtml(subtask.description)}
                        </div>
                        ` : ''}

                        <div style="display: flex; gap: 15px; margin-top: 10px; font-size: 14px;">
                            ${subtask.actual_time ? `
                            <div>
                                <strong>实际耗时:</strong> ${this.formatTime(subtask.actual_time)}
                            </div>
                            ` : ''}
                            ${subtask.start_date && subtask.end_date ? `
                            <div>
                                <strong>时间:</strong> ${subtask.start_date} ~ ${subtask.end_date}
                            </div>
                            ` : ''}
                        </div>
                    </div>
                `;
            });
            html += '</div>';

            subtasksList.innerHTML = html;

        } catch (error) {
            console.error('加载子任务失败:', error);
            const subtasksList = modal.querySelector('#subtasksList');
            subtasksList.innerHTML = `
                <div class="error-state" style="text-align: center; padding: 20px; color: #dc3545;">
                    <i class="fas fa-exclamation-triangle fa-2x" style="margin-bottom: 10px;"></i>
                    <p>加载子任务失败</p>
                    <button class="btn btn-sm btn-primary" onclick="app.loadSubtasks('${taskId}', this.closest('.modal'))" style="padding: 5px 10px; background-color: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer;">重试</button>
                </div>
            `;
        }
    }
}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    window.app = new CompletedTasksApp();
});