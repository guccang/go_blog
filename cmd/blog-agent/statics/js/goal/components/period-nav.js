// statics/js/goal/components/period-nav.js
import { store } from '../store.js';
import { currentPeriod, periodLabel } from '../utils.js';

class PeriodNav extends HTMLElement {
  connectedCallback() {
    this.render();
    this._unsub = store.on('state:changed', () => this.render());
  }

  disconnectedCallback() {
    if (this._unsub) this._unsub();
  }

  render() {
	const { level, period, nav, loading, view } = store.state;
	if (view === 'map') { this.innerHTML = ''; return; }
    const label = periodLabel(level, period);

    this.innerHTML = `
      <div class="period-nav">
        <button class="period-btn" data-action="prev" ${!nav ? 'disabled' : ''}>
          <i class="fas fa-chevron-left"></i>
        </button>
        <span class="period-label">${label || period || ''}</span>
        <button class="period-btn" data-action="next" ${!nav ? 'disabled' : ''}>
          <i class="fas fa-chevron-right"></i>
        </button>
        <button class="period-btn period-today" data-action="today">今天</button>
        ${loading ? '<span class="period-loading">加载中...</span>' : ''}
      </div>
    `;

    this.querySelector('[data-action="prev"]')?.addEventListener('click', () => {
      if (!nav) return;
      store.setState({ period: nav.prev });
      store.dispatch('period:changed', nav.prev);
    });

    this.querySelector('[data-action="next"]')?.addEventListener('click', () => {
      if (!nav) return;
      store.setState({ period: nav.next });
      store.dispatch('period:changed', nav.next);
    });

    this.querySelector('[data-action="today"]')?.addEventListener('click', () => {
      const p = currentPeriod(level);
      store.setState({ period: p });
      store.dispatch('period:changed', p);
    });
  }
}

customElements.define('period-nav', PeriodNav);
