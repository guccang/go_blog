// statics/js/goal/components/goal-list.js
import { store } from '../store.js';
import { api } from '../api.js';
import { periodLabel } from '../utils.js';

class GoalList extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.loadGoals();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  async loadGoals() {
    const { level } = store.state;
    store.setState({ loading: true });
    try {
      const res = await api.listGoals(level, '');
      if (res.success) {
        store.setState({ goals: res.data || [] });
      }
    } catch (e) {
      console.error('Failed to load goals:', e);
    } finally {
      store.setState({ loading: false });
    }
  }

  render() {
    const { goals, level, loading } = store.state;

    if (loading) {
      this.innerHTML = '<div class="loading">加载中...</div>';
      return;
    }

    if (!goals.length) {
      this.innerHTML = '<div class="empty-state"><p>暂无目标</p></div>';
      return;
    }

    this.innerHTML = `
      <div class="goal-grid">
        ${goals.map(g => `
          <div class="goal-summary-card" data-period="${g.period}">
            <div class="card-header">
              <span class="card-period">${periodLabel(level, g.period)}</span>
              <span class="card-status status-${g.status}">${g.status === 'completed' ? '已完成' : '进行中'}</span>
            </div>
            <p class="card-overview">${escapeHtml(g.overview || '未设置概述')}</p>
            <div class="card-progress">
              <div class="progress-bar">
                <div class="progress-fill" style="width:${g.progress || 0}%"></div>
              </div>
              <span class="card-progress-text">${g.progress || 0}%</span>
            </div>
            <div class="card-stats">
              <span>${g.done_tasks}/${g.total_tasks} 任务</span>
              <span>${g.pending_tasks} 待办</span>
            </div>
          </div>
        `).join('')}
      </div>
    `;

    this.querySelectorAll('.goal-summary-card').forEach(card =>
      card.addEventListener('click', () => {
        store.setState({ period: card.dataset.period, view: 'detail' });
        store.dispatch('period:changed', card.dataset.period);
        store.dispatch('view:changed', 'detail');
      })
    );
  }
}

function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s || ''; return d.innerHTML; }

customElements.define('goal-list', GoalList);
