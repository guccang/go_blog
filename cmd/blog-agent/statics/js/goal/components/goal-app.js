// statics/js/goal/components/goal-app.js
import { store } from '../store.js';
import { api } from '../api.js';
import { currentPeriod } from '../utils.js';
import './goal-tabs.js';
import './period-nav.js';

class GoalApp extends HTMLElement {
  constructor() {
    super();
    this.unsubs = [];
  }

  connectedCallback() {
    this.innerHTML = `
      <div class="goal-container">
        <goal-tabs></goal-tabs>
        <period-nav></period-nav>
        <div id="goal-view" class="goal-view"></div>
      </div>
    `;

    this.unsubs.push(
      store.on('level:changed', () => this.loadGoal()),
      store.on('period:changed', () => this.loadGoal()),
      store.on('view:changed', () => this.renderView()),
    );

    // 初始化
    const period = currentPeriod(store.state.level);
    store.setState({ period });
    this.loadGoal();
  }

  disconnectedCallback() {
    this.unsubs.forEach(fn => fn());
  }

  async loadGoal() {
    store.setState({ loading: true });
    try {
      const res = await api.getGoal(store.state.level, store.state.period);
      if (res.success) {
        store.setState({ goal: res.data });
        if (res.nav) store.setState({ nav: res.nav });
        // 如果有 parent_id，加载父目标
        if (res.data.parent_id) {
          this.loadParentGoal(res.data.parent_id);
        } else {
          store.setState({ parentGoal: null });
        }
      }
    } catch (e) {
      console.error('Failed to load goal:', e);
    } finally {
      store.setState({ loading: false });
    }
  }

  async loadParentGoal(parentId) {
    // parentId 是 "level|period" 格式或直接的 goal ID
    // 遍历查找
    try {
      const res = await api.listGoals(store.state.parentLevel || 'monthly', '');
      if (res.success && res.data) {
        const parent = res.data.find(g =>
          `${g.level}|${g.period}` === parentId || g.period === parentId
        );
        store.setState({ parentGoal: parent || null });
      }
    } catch (e) {
      store.setState({ parentGoal: null });
    }
  }

  renderView() {
    const viewEl = this.querySelector('#goal-view');
    if (!viewEl) return;
    switch (store.state.view) {
      case 'detail':
        viewEl.innerHTML = '<goal-detail></goal-detail>';
        break;
      case 'list':
        viewEl.innerHTML = '<goal-list></goal-list>';
        break;
      case 'review':
        viewEl.innerHTML = '<review-panel></review-panel>';
        break;
    }
  }
}

customElements.define('goal-app', GoalApp);
