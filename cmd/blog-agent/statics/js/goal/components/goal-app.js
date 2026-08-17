// statics/js/goal/components/goal-app.js
import { store } from '../store.js';
import { api } from '../api.js';
import { currentPeriod, LEVELS, PARENT_LEVEL } from '../utils.js';
import './goal-tabs.js';
import './period-nav.js';
import './goal-detail.js?v=goal-parent-fix-1';
import './goal-list.js';
import './review-panel.js';
import './goal-map.js?v=goal-wheel-1';

class GoalApp extends HTMLElement {
  constructor() {
    super();
    this.unsubs = [];
  }

  connectedCallback() {
    this.innerHTML = `
      <div class="goal-container">
        <div class="goal-page-header">
          <a class="goal-home-link" href="/goal">← 目标地图</a>
          <span class="goal-workspace-title">目标工作区</span>
          <a class="goal-main-link" href="/main">主页</a>
        </div>
        <goal-tabs></goal-tabs>
        <period-nav></period-nav>
        <div id="goal-view" class="goal-view"></div>
      </div>
    `;

    // Toast 容器
    const toastContainer = document.createElement('div');
    toastContainer.className = 'toast-container';
    this.appendChild(toastContainer);

    this.unsubs.push(
      store.on('level:changed', () => this.loadGoal()),
      store.on('period:changed', () => this.loadGoal()),
      store.on('view:changed', () => this.renderView()),
      store.on('toast:show', ({ message, type }) => {
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.textContent = message;
        toastContainer.appendChild(toast);
        requestAnimationFrame(() => toast.classList.add('show'));
        setTimeout(() => {
          toast.classList.remove('show');
          setTimeout(() => toast.remove(), 300);
        }, 2500);
      }),
    );

    // 初始化：先渲染视图让组件挂载并订阅事件，再加载数据
    this.renderView();
    // 等待组件完成挂载后再设置状态和加载数据
    requestAnimationFrame(() => {
      const params = new URLSearchParams(window.location.search);
      const requestedLevel = params.get('level');
      const level = LEVELS.includes(requestedLevel) ? requestedLevel : store.state.level;
      const requestedPeriod = params.get('period');
      const period = requestedPeriod || currentPeriod(level);
	  const requestedView = params.get('view');
	  const view = ['map', 'detail', 'list', 'review'].includes(requestedView) ? requestedView : (LEVELS.includes(requestedLevel) ? 'detail' : store.state.view);
	  store.setState({ level, period, view });
	  this.renderView();
      this.loadGoal();
    });
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
    try {
      const [savedLevel] = parentId.split('|');
      const parentLevel = LEVELS.includes(savedLevel)
        ? savedLevel
        : (PARENT_LEVEL[store.state.level] || 'monthly');
	  const [, savedPeriod] = parentId.split('|');
	  const res = await api.getGoal(parentLevel, savedPeriod || parentId);
	  if (res.success) store.setState({ parentGoal: res.data || null });
    } catch (e) {
      store.setState({ parentGoal: null });
    }
  }

  renderView() {
    const viewEl = this.querySelector('#goal-view');
    if (!viewEl) return;
    switch (store.state.view) {
	  case 'map':
		viewEl.innerHTML = '<goal-map></goal-map>';
		break;
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
