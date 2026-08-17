// statics/js/goal/components/goal-tabs.js
import { store } from '../store.js';
import { LEVELS, LEVEL_LABELS, currentPeriod } from '../utils.js';

class GoalTabs extends HTMLElement {
  connectedCallback() {
    this.render();
    this._unsub = store.on('state:changed', () => this.render());
  }

  disconnectedCallback() {
    if (this._unsub) this._unsub();
  }

  render() {
    const { level, view } = store.state;
    const icons = { daily: '☀️', weekly: '🧭', monthly: '🌙', yearly: '🏔️' };
    const tabsHtml = LEVELS.map(l =>
      `<button class="goal-tab ${l === level ? 'active' : ''}" data-level="${l}"><span>${icons[l] || '•'}</span>${LEVEL_LABELS[l]}</button>`
    ).join('');

    this.innerHTML = `
      <div class="goal-tabs">
		<div class="goal-tabs-nav">${view === 'map' ? '<span class="goal-map-mode-label">年度 → 月度 → 周度 → 今日</span>' : tabsHtml}</div>
        <div class="goal-view-toggle">
		  <button class="view-btn ${view === 'map' ? 'active' : ''}" data-view="map">全景</button>
          <button class="view-btn ${view === 'detail' ? 'active' : ''}" data-view="detail">详情</button>
          <button class="view-btn ${view === 'list' ? 'active' : ''}" data-view="list">列表</button>
          <button class="view-btn ${view === 'review' ? 'active' : ''}" data-view="review">回顾</button>
        </div>
      </div>
    `;

    this.querySelectorAll('.goal-tab').forEach(btn =>
      btn.addEventListener('click', () => {
        const newLevel = btn.dataset.level;
        store.setState({ level: newLevel, period: currentPeriod(newLevel) });
        store.dispatch('level:changed', newLevel);
      })
    );

    this.querySelectorAll('.view-btn').forEach(btn =>
      btn.addEventListener('click', () => {
        store.setState({ view: btn.dataset.view });
        store.dispatch('view:changed', btn.dataset.view);
      })
    );
  }
}

customElements.define('goal-tabs', GoalTabs);
