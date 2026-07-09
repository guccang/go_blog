// statics/js/goal/components/task-editor.js
import { store } from '../store.js';
import { api } from '../api.js';
import { PRIORITY_LABELS, STATUS_LABELS, escapeHtml } from '../utils.js';

class TaskEditor extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { editTask } = store.state;
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
              <label>优先级</label>
              <select class="field-priority">
                ${Object.entries(PRIORITY_LABELS).map(([k, v]) =>
                  `<option value="${k}" ${task.priority === k ? 'selected' : ''}>${v}</option>`
                ).join('')}
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
  }

  save() {
    const { goal, editTask } = store.state;
    if (!goal || !editTask) return;

    const updated = {
      ...editTask,
      title: this.querySelector('.field-title').value,
      priority: this.querySelector('.field-priority').value,
      status: this.querySelector('.field-status').value,
      deadline: this.querySelector('.field-deadline').value,
      estimate_hours: parseFloat(this.querySelector('.field-estimate').value) || 0,
      description: this.querySelector('.field-desc').value,
    };

    api.updateTask(goal.level, goal.period, editTask.id, updated).then(() => {
      store.setState({ editTask: null });
      store.dispatch('level:changed', goal.level);
    });
  }
}

customElements.define('task-editor', TaskEditor);
