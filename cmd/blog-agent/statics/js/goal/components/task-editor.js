// statics/js/goal/components/task-editor.js
import { store } from '../store.js';
import { api } from '../api.js';
import { STATUS_LABELS, escapeHtml } from '../utils.js';

const WEEKDAYS = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];

class TaskEditor extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
	const { editTask, goal, parentGoal } = store.state;
    if (!editTask) { this.innerHTML = ''; return; }

    const task = editTask || {};
    this.innerHTML = `
      <div class="modal-overlay" data-action="close">
        <div class="modal-content">
          <h3>编辑任务</h3>
          <div class="form-group">
            <label>标题</label>
            <input type="text" class="field-title" value="${escapeHtml(task.title || '')}">
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>重要性</label>
              <select class="field-importance">
				${[5, 4, 3, 2, 1].map(value => `<option value="${value}" ${task.importance === value ? 'selected' : ''}>${this._importanceLabel(value)}</option>`).join('')}
              </select>
            </div>
            <div class="form-group">
              <label>状态</label>
              <select class="field-status">
                ${Object.entries(STATUS_LABELS).map(([k, v]) =>
                  `<option value="${k}" ${task.status === k ? 'selected' : ''}>${v}</option>`
                ).join('')}
              </select>
            </div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>截止日期</label>
              <input type="date" class="field-deadline" value="${task.deadline || ''}">
            </div>
            <div class="form-group">
              <label>预估耗时 (小时)</label>
              <input type="number" class="field-estimate" value="${task.estimate_hours || ''}" step="0.5" min="0">
            </div>
          </div>
		  ${this._renderScheduleFields(goal?.level, task)}
		  ${parentGoal?.tasks?.length ? `<div class="form-group"><label>承接上层任务</label><select class="field-source-task"><option value="">对齐整个上层目标</option>${parentGoal.tasks.filter(item => item.status !== 'cancelled').map(item => `<option value="${item.id}" ${task.source_task_id === item.id ? 'selected' : ''}>${escapeHtml(item.title)}</option>`).join('')}</select></div>` : ''}
          <div class="form-group">
            <label>描述</label>
            <textarea class="field-desc" rows="2">${escapeHtml(task.description || '')}</textarea>
          </div>
          <div class="modal-actions">
            <button class="btn-sm" data-action="close">取消</button>
            <button class="btn-sm btn-primary" data-action="save">保存</button>
          </div>
        </div>
      </div>
    `;

    this.querySelector('[data-action="close"]')?.addEventListener('click', (e) => {
      if (e.target.dataset.action === 'close') store.setState({ editTask: null });
    });
    this.querySelector('[data-action="save"]')?.addEventListener('click', () => this.save());
	this.querySelector('.field-source-task')?.addEventListener('change', event => {
		const source = (parentGoal?.tasks || []).find(item => item.id === event.target.value);
		if (source) this.querySelector('.field-importance').value = String(source.importance || 3);
	});
  }

	_renderScheduleFields(level, task) {
		if (level !== 'weekly' && level !== 'daily') return '';
		const schedules = task.schedules || [];
		const weekdays = level === 'weekly' ? [1, 2, 3, 4, 5, 6, 7] : [0];
		return `<div class="form-group schedule-form-group">
			<label>投入时间 <span>${schedules.length} 个半天时段</span></label>
			<div class="schedule-grid ${level === 'daily' ? 'schedule-grid-daily' : ''}">
				${['morning', 'afternoon'].flatMap(timeSlot => weekdays.map(weekday => {
					const checked = schedules.some(item => item.weekday === weekday && item.time_slot === timeSlot);
					const label = level === 'weekly' ? `${WEEKDAYS[weekday - 1]}${timeSlot === 'morning' ? '上午' : '下午'}` : (timeSlot === 'morning' ? '上午' : '下午');
					return `<label class="schedule-cell"><input type="checkbox" class="field-schedule" data-weekday="${weekday}" data-time-slot="${timeSlot}" ${checked ? 'checked' : ''}><span>${label}</span></label>`;
				})).join('')}
			</div>
		</div>`;
	}

	_importanceLabel(value) {
		return { 5: '5 · 核心', 4: '4 · 重要', 3: '3 · 正常', 2: '2 · 次要', 1: '1 · 可选' }[value];
	}

  save() {
    const { goal, editTask } = store.state;
    if (!goal || !editTask) return;

    const updated = {
      ...editTask,
      title: this.querySelector('.field-title').value,
	  importance: parseInt(this.querySelector('.field-importance').value, 10),
	  source_task_id: this.querySelector('.field-source-task')?.value || '',
      status: this.querySelector('.field-status').value,
      deadline: this.querySelector('.field-deadline').value,
      estimate_hours: parseFloat(this.querySelector('.field-estimate').value) || 0,
      description: this.querySelector('.field-desc').value,
	  schedules: [...this.querySelectorAll('.field-schedule:checked')].map(input => ({
		  weekday: parseInt(input.dataset.weekday || '0', 10),
		  time_slot: input.dataset.timeSlot,
	  })),
    };

    api.updateTask(goal.level, goal.period, editTask.id, updated).then(() => {
      store.setState({ editTask: null });
      store.dispatch('level:changed', goal.level);
    });
  }
}

customElements.define('task-editor', TaskEditor);
