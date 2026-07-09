// statics/js/goal/components/task-list.js
import { store } from '../store.js';
import { api } from '../api.js';
import './task-item.js';

class TaskList extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { goal } = store.state;
    if (!goal) { this.innerHTML = ''; return; }

    const tasks = goal.tasks || [];
    this.innerHTML = `
      <div class="goal-card">
        <h3 class="section-title">任务 (${tasks.length})</h3>
        <div class="task-list" id="taskListContainer"></div>
        <div class="task-add-row">
          <input type="text" class="task-add-input" placeholder="添加新任务..." id="newTaskTitle">
          <select class="task-add-priority" id="newTaskPriority">
            <option value="medium">普通</option>
            <option value="high">高优</option>
            <option value="low">低优</option>
          </select>
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
    this.querySelector('#newTaskTitle')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this._addTask();
    });
    this.querySelector('[data-action="delete-goal"]')?.addEventListener('click', () => {
      if (confirm(`确定删除该目标及其所有任务？此操作不可恢复。`)) {
        api.deleteGoal(goal.level, goal.period).then(() => {
          store.dispatch('level:changed', goal.level);
        });
      }
    });
  }

  async _addTask() {
    const input = this.querySelector('#newTaskTitle');
    const title = input.value.trim();
    if (!title) return;
    const priority = this.querySelector('#newTaskPriority').value;
    const { goal } = store.state;
    await api.addTask(goal.level, goal.period, {
      title,
      priority,
      status: 'pending',
      subtasks: [],
      notes: [],
    });
    input.value = '';
    store.dispatch('level:changed', goal.level);
  }
}

customElements.define('task-list', TaskList);
