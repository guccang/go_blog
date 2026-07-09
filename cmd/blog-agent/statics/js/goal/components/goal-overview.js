// statics/js/goal/components/goal-overview.js
import { store } from '../store.js';
import { api } from '../api.js';
import { escapeHtml } from '../utils.js';

class GoalOverview extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { goal, parentGoal } = store.state;
    if (!goal) {
      this.innerHTML = '<div class="goal-card"><p style="color:var(--goal-text-muted)">加载中...</p></div>';
      return;
    }

    const statusClass = goal.status === 'completed' ? 'status-done' : 'status-active';
    const statusText = goal.status === 'completed' ? '已完成' : '进行中';

    this.innerHTML = `
      <div class="goal-card">
        ${parentGoal ? `
          <div class="parent-breadcrumb">
            <i class="fas fa-link"></i>
            <span>对齐到: ${escapeHtml(parentGoal.overview || parentGoal.period)}</span>
          </div>
        ` : ''}
        <div class="goal-header">
          <span class="goal-status ${statusClass}">${statusText}</span>
          <button class="btn-sm btn-toggle" data-action="toggle">${goal.status === 'completed' ? '重新开始' : '标记完成'}</button>
        </div>
        <textarea class="overview-input" placeholder="写一下你的目标概述..." rows="3">${escapeHtml(goal.overview || '')}</textarea>
        <div class="overview-actions">
          <button class="btn-sm btn-primary" data-action="save">保存概述</button>
          <span class="progress-text">进度 ${goal.progress || 0}%</span>
        </div>
        <div class="progress-bar">
          <div class="progress-fill" style="width:${goal.progress || 0}%"></div>
        </div>
      </div>
    `;

    this.querySelector('[data-action="toggle"]')?.addEventListener('click', () => {
      const newStatus = goal.status === 'completed' ? 'active' : 'completed';
      api.saveGoal(goal.level, goal.period, undefined, newStatus).then(() => {
        store.dispatch('level:changed', goal.level);
      });
    });

    this.querySelector('[data-action="save"]')?.addEventListener('click', () => {
      const textarea = this.querySelector('.overview-input');
      api.saveGoal(goal.level, goal.period, textarea.value).then(() => {
        store.dispatch('level:changed', goal.level);
      });
    });
  }
}

customElements.define('goal-overview', GoalOverview);
