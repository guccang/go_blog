// statics/js/goal/components/task-list.js
import { store } from '../store.js';
import { api } from '../api.js?v=goal-ai-task-fill-2';
import { escapeHtml } from '../utils.js';
import './task-item.js?v=goal-importance-time-1';

const WEEKDAYS = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
const TIME_SLOTS = { morning: '上午', afternoon: '下午' };

class TaskList extends HTMLElement {
  constructor() {
    super();
    this.drafts = null;
    this.generating = false;
  }

  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { goal, parentGoal } = store.state;
    if (!goal) { this.innerHTML = ''; return; }

    const tasks = goal.tasks || [];
    const supportsTaskGeneration = goal.level !== 'yearly';
    const hasAlignment = Boolean(goal.parent_id);
    this.innerHTML = `
      <div class="goal-card">
        <div class="task-section-heading">
          <h3 class="section-title">任务 (${tasks.length})</h3>
          ${supportsTaskGeneration ? `<button class="btn-sm ai-task-button" data-action="generate-tasks" ${hasAlignment ? '' : 'disabled title="请先对齐上层目标"'}>${hasAlignment ? 'AI 生成任务草稿' : '先对齐目标，再生成任务'}</button>` : ''}
        </div>
        ${hasAlignment ? `
          <div class="ai-task-controls">
            <input type="text" class="ai-task-instruction" placeholder="可选：补充限制，例如“今天优先做后端”">
          </div>
        ` : ''}
        ${this._renderDrafts()}
        <div class="task-list" id="taskListContainer"></div>
        <div class="task-add-row">
          <input type="text" class="task-add-input" placeholder="添加新任务..." id="newTaskTitle">
          <select class="task-add-priority task-add-importance" id="newTaskImportance" aria-label="重要性">
            <option value="5">5 · 核心</option>
            <option value="4">4 · 重要</option>
            <option value="3" selected>3 · 正常</option>
            <option value="2">2 · 次要</option>
            <option value="1">1 · 可选</option>
          </select>
          ${parentGoal?.tasks?.length ? `<select class="task-add-priority task-add-source" id="newTaskSource" aria-label="对齐的上层任务">
            <option value="">对齐整个上层目标</option>
            ${parentGoal.tasks.filter(task => task.status !== 'cancelled').map(task => `<option value="${task.id}">${escapeHtml(task.title)}</option>`).join('')}
          </select>` : ''}
          <button class="btn-sm btn-primary" id="addTaskBtn">添加</button>
        </div>
        <div class="task-actions">
          <button class="btn-sm btn-danger" data-action="delete-goal">删除目标</button>
        </div>
      </div>
    `;

    const container = this.querySelector('#taskListContainer');
    tasks.forEach(t => {
      const item = document.createElement('task-item');
      item.task = t;
      container.appendChild(item);
    });

    this.querySelector('#addTaskBtn')?.addEventListener('click', () => this._addTask());
	this.querySelector('#newTaskSource')?.addEventListener('change', event => {
		const source = (parentGoal?.tasks || []).find(task => task.id === event.target.value);
		if (source) this.querySelector('#newTaskImportance').value = String(source.importance || 3);
	});
    this.querySelector('[data-action="generate-tasks"]')?.addEventListener('click', () => this._generateTaskDrafts());
    this.querySelector('[data-action="cancel-drafts"]')?.addEventListener('click', () => {
      this.drafts = null;
      this.render();
    });
    this.querySelector('[data-action="confirm-drafts"]')?.addEventListener('click', () => this._confirmTaskDrafts());
    this.querySelector('#newTaskTitle')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this._addTask();
    });
    this.querySelector('[data-action="delete-goal"]')?.addEventListener('click', () => {
      if (confirm(`确定删除该目标及其所有任务？此操作不可恢复。`)) {
        api.deleteGoal(goal.level, goal.period).then(() => {
          store.dispatch('level:changed', goal.level);
        }).catch(err => console.error('Operation failed:', err));
      }
    });
  }

  _renderDrafts() {
    if (this.generating) {
      return '<div class="ai-task-preview"><p class="ai-task-status">正在根据对齐目标生成任务草稿…</p></div>';
    }
	if (!this.drafts) return '';
	const { goal } = store.state;
    return `
      <div class="ai-task-preview">
        <div class="ai-task-preview-title"><strong>任务草稿</strong><span>确认后才会添加</span></div>
        <div class="ai-task-drafts">
          ${this.drafts.map((task, index) => `
            <label class="ai-task-draft" data-draft-index="${index}">
              <input type="checkbox" class="ai-task-selected" checked>
              <span class="ai-task-fields">
                <input type="text" class="ai-task-title" value="${escapeHtml(task.title).replaceAll('"', '&quot;')}" aria-label="任务标题">
                <textarea class="ai-task-description" rows="2" placeholder="任务说明（可选）">${escapeHtml(task.description || '')}</textarea>
                ${task.source_task_title ? `<small class="ai-task-source">承接：${escapeHtml(task.source_task_title)}</small>` : ''}
              </span>
              <select class="ai-task-priority ai-task-importance" aria-label="重要性">
                ${[5, 4, 3, 2, 1].map(value => `<option value="${value}" ${task.importance === value ? 'selected' : ''}>重要性 ${value}</option>`).join('')}
              </select>
			  ${this._renderDraftSchedule(task, goal.level)}
            </label>
          `).join('')}
        </div>
        <div class="ai-task-preview-actions">
          <button class="btn-sm" data-action="cancel-drafts">取消</button>
          <button class="btn-sm btn-primary" data-action="confirm-drafts">确认添加</button>
        </div>
      </div>
    `;
  }

	_renderDraftSchedule(task, level) {
		if (level !== 'weekly' && level !== 'daily') return '';
		const schedules = task.schedules || [];
		const weekdays = level === 'weekly' ? [1, 2, 3, 4, 5, 6, 7] : [0];
		return `<details class="ai-task-schedule">
			<summary>${schedules.length} 个时段</summary>
			<span class="schedule-grid ${level === 'daily' ? 'schedule-grid-daily' : ''}">
				${Object.entries(TIME_SLOTS).flatMap(([timeSlot, slotLabel]) => weekdays.map(weekday => {
					const checked = schedules.some(item => item.weekday === weekday && item.time_slot === timeSlot);
					const label = level === 'weekly' ? `${WEEKDAYS[weekday - 1]}${slotLabel}` : slotLabel;
					return `<label class="schedule-cell"><input type="checkbox" class="ai-task-slot" data-weekday="${weekday}" data-time-slot="${timeSlot}" ${checked ? 'checked' : ''}><span>${label}</span></label>`;
				})).join('')}
			</span>
		</details>`;
	}

  async _generateTaskDrafts() {
    if (this.generating) return;
    const { goal, parentGoal } = store.state;
    const instruction = this.querySelector('.ai-task-instruction')?.value.trim() || '';
    this.generating = true;
    this.drafts = null;
    this.render();
    try {
      const response = await api.generateTaskDrafts(goal.level, goal.period, instruction);
      this.drafts = response.data || [];
      if (!this.drafts.length) store.showToast('没有生成可添加的新任务', 'info');
    } catch (err) {
      console.error('Failed to generate task drafts:', err);
    } finally {
      this.generating = false;
      this.render();
    }
  }

  async _confirmTaskDrafts() {
    const rows = [...this.querySelectorAll('.ai-task-draft')];
    const selected = rows.filter(row => row.querySelector('.ai-task-selected').checked).map(row => ({
      title: row.querySelector('.ai-task-title').value.trim(),
      description: row.querySelector('.ai-task-description').value.trim(),
	  importance: parseInt(row.querySelector('.ai-task-importance').value, 10),
	  source_task_id: this.drafts[parseInt(row.dataset.draftIndex, 10)]?.source_task_id || '',
	  schedules: [...row.querySelectorAll('.ai-task-slot:checked')].map(input => ({
		  weekday: parseInt(input.dataset.weekday || '0', 10),
		  time_slot: input.dataset.timeSlot,
	  })),
      status: 'pending',
      subtasks: [],
      notes: [],
    })).filter(task => task.title);
    if (!selected.length) {
      store.showToast('请至少选择一项任务', 'info');
      return;
    }

	const { goal } = store.state;
	if (!this._validateSelectedSchedules(selected, goal)) return;

    const button = this.querySelector('[data-action="confirm-drafts"]');
    button.disabled = true;
    button.textContent = '添加中…';
    try {
      for (const task of selected) {
        await api.addTask(goal.level, goal.period, task);
      }
      this.drafts = null;
      store.showToast(`已添加 ${selected.length} 项任务`, 'success');
    } catch (err) {
      console.error('Failed to add generated tasks:', err);
      this.drafts = null;
    } finally {
      store.dispatch('level:changed', goal.level);
    }
  }

	_validateSelectedSchedules(selected, goal) {
		if (goal.level !== 'daily' && goal.level !== 'weekly') return true;
		const occupied = new Set((goal.tasks || []).filter(task => task.status !== 'cancelled').flatMap(task => task.schedules || []).map(schedule =>
			goal.level === 'weekly' ? `${schedule.weekday}:${schedule.time_slot}` : schedule.time_slot
		));
		for (const task of selected) {
			for (const schedule of task.schedules) {
				const key = goal.level === 'weekly' ? `${schedule.weekday}:${schedule.time_slot}` : schedule.time_slot;
				if (occupied.has(key)) {
					store.showToast('同一天的上午或下午只能安排一件事，请调整草稿排期', 'info');
					return false;
				}
				occupied.add(key);
			}
		}
		return true;
	}

  async _addTask() {
    const input = this.querySelector('#newTaskTitle');
    const button = this.querySelector('#addTaskBtn');
    const title = input.value.trim();
    if (!title || button.disabled) return;
	const importance = parseInt(this.querySelector('#newTaskImportance').value, 10);
	const sourceTaskId = this.querySelector('#newTaskSource')?.value || '';
    const { goal } = store.state;
    button.disabled = true;
    button.textContent = '添加中…';
    try {
      const response = await api.addTask(goal.level, goal.period, {
        title,
		importance,
		source_task_id: sourceTaskId,
        status: 'pending',
        subtasks: [],
        notes: [],
      });
      const addedTask = response.data || {
        id: response.task_id,
        title,
		importance,
		source_task_id: sourceTaskId,
        status: 'pending',
        subtasks: [],
        notes: [],
      };
      const tasks = [...(goal.tasks || []), addedTask];
      const completed = tasks.filter(task => task.status === 'completed').length;
      const progress = tasks.length ? Math.floor(completed * 100 / tasks.length) : 0;
      store.state.goal = {
        ...goal,
        tasks,
        progress,
      };
      const item = document.createElement('task-item');
      item.task = addedTask;
      this.querySelector('#taskListContainer')?.appendChild(item);
      const titleElement = this.querySelector('.section-title');
      if (titleElement) titleElement.textContent = `任务 (${tasks.length})`;
      document.querySelector('goal-overview')?.updateProgress(progress);
      input.value = '';
      button.disabled = false;
      button.textContent = '添加';
      input.focus();
      store.showToast('任务已添加', 'success');
    } catch (err) {
      console.error('Operation failed:', err);
      button.disabled = false;
      button.textContent = '添加';
    }
  }
}

customElements.define('task-list', TaskList);
