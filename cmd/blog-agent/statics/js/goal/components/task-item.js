// statics/js/goal/components/task-item.js
import { store } from '../store.js';
import { api } from '../api.js';
import { PRIORITY_LABELS, escapeHtml } from '../utils.js';

class TaskItem extends HTMLElement {
  set task(value) { this._task = value; this.render(); }
  get task() { return this._task; }

  render() {
    const t = this._task;
    if (!t) return;
    const checked = t.status === 'completed' ? 'checked' : '';
    const doneClass = t.status === 'completed' ? 'task-done' : '';

    this.innerHTML = `
      <div class="task-item ${doneClass}">
        <div class="task-main">
          <input type="checkbox" class="task-check" ${checked} data-action="toggle">
          <span class="task-title" data-action="edit-title">${escapeHtml(t.title)}</span>
          ${t.deadline ? `<span class="task-deadline">📅${t.deadline}</span>` : ''}
          ${t.estimate_hours ? `<span class="task-estimate">⏱${t.estimate_hours}h</span>` : ''}
          <span class="task-priority priority-${t.priority}">${PRIORITY_LABELS[t.priority] || ''}</span>
          <span class="task-status-label">${t.status === 'in_progress' ? '进行中' : ''}${t.status === 'cancelled' ? '已取消' : ''}</span>
          <button class="btn-icon-sm" data-action="expand">${this._expanded ? '▲' : '▼'}</button>
          <button class="btn-icon-sm" data-action="edit">✏</button>
          <button class="btn-icon-sm btn-danger" data-action="delete">✕</button>
        </div>
        ${this._expanded ? this._renderExpanded() : ''}
      </div>
    `;

    this.querySelector('[data-action="toggle"]')?.addEventListener('change', (e) => {
      const { goal } = store.state;
      const updated = { ...this._task, status: e.target.checked ? 'completed' : 'pending' };
      api.updateTask(goal.level, goal.period, this._task.id, updated).then(() => {
        store.dispatch('level:changed', goal.level);
      });
    });

    this.querySelector('[data-action="edit-title"]')?.addEventListener('dblclick', () => this._inlineEdit());
    this.querySelector('[data-action="expand"]')?.addEventListener('click', () => {
      this._expanded = !this._expanded;
      this.render();
    });
    this.querySelector('[data-action="edit"]')?.addEventListener('click', () => {
      store.setState({ editTask: this._task });
    });
    this.querySelector('[data-action="delete"]')?.addEventListener('click', () => {
      const { goal } = store.state;
      if (confirm('确定删除该任务？')) {
        api.deleteTask(goal.level, goal.period, this._task.id).then(() => {
          store.dispatch('level:changed', goal.level);
        });
      }
    });

    // 子任务事件
    this.querySelectorAll('.subtask-check').forEach(cb =>
      cb.addEventListener('change', (e) => this._toggleSubtask(e))
    );
    this.querySelector('[data-action="add-subtask"]')?.addEventListener('click', () => this._addSubtask());
    this.querySelector('[data-action="add-note"]')?.addEventListener('click', () => this._addNote());
  }

  _renderExpanded() {
    const subtasks = this._task.subtasks || [];
    const notes = this._task.notes || [];
    return `
      <div class="task-expanded">
        <div class="subtask-section">
          <div class="subtask-list">
            ${subtasks.map(s => `
              <div class="subtask-item">
                <input type="checkbox" class="subtask-check" data-id="${s.id}" ${s.status === 'completed' ? 'checked' : ''}>
                <span class="${s.status === 'completed' ? 'line-through' : ''}">${escapeHtml(s.title)}</span>
              </div>
            `).join('')}
          </div>
          <div class="subtask-add">
            <input type="text" class="subtask-input" placeholder="添加子任务...">
            <button class="btn-sm" data-action="add-subtask">添加</button>
          </div>
        </div>
        <div class="note-section">
          <div class="note-list">
            ${notes.slice().reverse().map(n => `
              <div class="note-item">
                <span class="note-date">${n.created_at?.split(' ')[0] || ''}</span>
                <span class="note-content">${escapeHtml(n.content)}</span>
              </div>
            `).join('')}
          </div>
          <div class="note-add">
            <input type="text" class="note-input" placeholder="添加备注...">
            <button class="btn-sm" data-action="add-note">添加</button>
          </div>
        </div>
      </div>
    `;
  }

  _inlineEdit() {
    const titleEl = this.querySelector('.task-title');
    const old = titleEl.textContent;
    titleEl.innerHTML = `<input type="text" class="inline-edit" value="${escapeHtml(old)}">`;
    const input = titleEl.querySelector('input');
    input.focus();
    input.select();
    const save = () => {
      const val = input.value.trim();
      if (val && val !== old) {
        const { goal } = store.state;
        const updated = { ...this._task, title: val };
        api.updateTask(goal.level, goal.period, this._task.id, updated).then(() => {
          store.dispatch('level:changed', goal.level);
        });
      } else {
        this.render();
      }
    };
    input.addEventListener('blur', save);
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter') save(); if (e.key === 'Escape') this.render(); });
  }

  async _toggleSubtask(e) {
    const { goal } = store.state;
    const sid = e.target.dataset.id;
    const subtasks = (this._task.subtasks || []).map(s =>
      s.id === sid ? { ...s, status: e.target.checked ? 'completed' : 'pending' } : s
    );
    const updated = { ...this._task, subtasks };
    await api.updateTask(goal.level, goal.period, this._task.id, updated);
    store.dispatch('level:changed', goal.level);
  }

  async _addSubtask() {
    const input = this.querySelector('.subtask-input');
    const val = input.value.trim();
    if (!val) return;
    const { goal } = store.state;
    const subtasks = [...(this._task.subtasks || []), {
      id: Date.now().toString(),
      title: val,
      status: 'pending',
    }];
    const updated = { ...this._task, subtasks };
    await api.updateTask(goal.level, goal.period, this._task.id, updated);
    store.dispatch('level:changed', goal.level);
  }

  async _addNote() {
    const input = this.querySelector('.note-input');
    const val = input.value.trim();
    if (!val) return;
    const { goal } = store.state;
    await api.addTaskNote(goal.level, goal.period, this._task.id, val);
    store.dispatch('level:changed', goal.level);
  }
}

customElements.define('task-item', TaskItem);
