// statics/js/goal/api.js
const api = {
  async _fetch(url, options = {}) {
    const res = await fetch(url, options);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  },

  _post(url, body) {
    return this._fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  },

  getGoal(level, period) {
    return this._fetch(`/api/goal?level=${level}&period=${period}&nav=true`);
  },

  saveGoal(level, period, overview, status, parentId) {
    const body = { level, period };
    if (overview !== undefined) body.overview = overview;
    if (status !== undefined) body.status = status;
    if (parentId !== undefined) body.parent_id = parentId;
    return this._post('/api/goal/save', body);
  },

  addTask(level, period, task) {
    return this._post('/api/goal/task', { level, period, task });
  },

  updateTask(level, period, taskId, task) {
    return this._post('/api/goal/task/update', { level, period, task_id: taskId, task });
  },

  deleteTask(level, period, taskId) {
    return this._post('/api/goal/task/delete', { level, period, task_id: taskId });
  },

  deleteGoal(level, period) {
    return this._post('/api/goal/delete', { level, period });
  },

  getParentGoals(level, period) {
    return this._fetch(`/api/goal/parent?level=${level}&period=${period}`);
  },

  addTaskNote(level, period, taskId, content) {
    return this._post('/api/goal/task/note', { level, period, task_id: taskId, content });
  },

  getReview(level, period) {
    return this._fetch(`/api/goal/review?level=${level}&period=${period}`);
  },

  saveReview(level, period, content) {
    return this._post('/api/goal/review/save', { level, period, content });
  },

  generateReview(level, period) {
    return this._post('/api/goal/review/generate', { level, period });
  },

  getCurrentGoals() {
    return this._fetch('/api/goals/current');
  },

  listGoals(level, year = '') {
    return this._fetch(`/api/goals?level=${level}&year=${year}`);
  },
};

export { api };
