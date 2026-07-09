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
            <span style="cursor:pointer">对齐到: ${escapeHtml(parentGoal.overview || parentGoal.period)}</span>
            <button class="btn-icon-sm" data-action="clear-parent">✕</button>
          </div>
        ` : (store.state.level !== 'yearly' ? `
          <div class="parent-selector">
            <button class="btn-sm" data-action="show-parents">
              <i class="fas fa-link"></i> 对齐上层目标
            </button>
            <div class="parent-dropdown hidden" id="parentDropdown">
              <div class="parent-list"></div>
            </div>
          </div>
        ` : '')}
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
      }).catch(err => console.error('Operation failed:', err));
    });

    this.querySelector('[data-action="save"]')?.addEventListener('click', () => {
      const textarea = this.querySelector('.overview-input');
      api.saveGoal(goal.level, goal.period, textarea.value).then(() => {
        store.dispatch('level:changed', goal.level);
      }).catch(err => console.error('Operation failed:', err));
    });

    this.querySelector('[data-action="show-parents"]')?.addEventListener('click', async () => {
      const dropdown = this.querySelector('#parentDropdown');
      dropdown.classList.toggle('hidden');
      if (!dropdown.classList.contains('hidden')) {
        try {
          const res = await api.getParentGoals(goal.level, goal.period);
          if (res.success && res.data) {
            const list = dropdown.querySelector('.parent-list');
            list.innerHTML = res.data.map(g => `
              <div class="parent-option" data-id="${g.level}|${g.period}">
                <span class="parent-level">${g.level}</span>
                <span>${g.overview || g.period}</span>
              </div>
            `).join('');
            list.querySelectorAll('.parent-option').forEach(opt =>
              opt.addEventListener('click', () => {
                api.saveGoal(goal.level, goal.period, undefined, undefined, opt.dataset.id).then(() => {
                  store.dispatch('level:changed', goal.level);
                }).catch(err => console.error('Operation failed:', err));
              })
            );
          }
        } catch (err) {
          console.error('Operation failed:', err);
        }
      }
    });

    this.querySelector('[data-action="clear-parent"]')?.addEventListener('click', () => {
      api.saveGoal(goal.level, goal.period, undefined, undefined, '').then(() => {
        store.dispatch('level:changed', goal.level);
      }).catch(err => console.error('Operation failed:', err));
    });

    this.querySelector('.parent-breadcrumb span')?.addEventListener('click', () => {
      if (parentGoal) {
        store.setState({ level: parentGoal.level, period: parentGoal.period, view: 'detail' });
        store.dispatch('level:changed', parentGoal.level);
      }
    });
  }
}

customElements.define('goal-overview', GoalOverview);
