import { api } from '../api.js?v=goal-map-1';
import { store } from '../store.js';
import { escapeHtml, LEVEL_LABELS, periodLabel } from '../utils.js';

const LEVELS = ['yearly', 'monthly', 'weekly', 'daily'];
const LEVEL_HINTS = { yearly: '方向', monthly: '阶段', weekly: '主线', daily: '行动' };

class GoalMap extends HTMLElement {
  constructor() {
    super();
    this.graph = null;
    this.zoom = 1;
    this.year = new Date().getFullYear();
  }

  connectedCallback() {
    this.load();
    this._resize = () => this.drawConnections();
    window.addEventListener('resize', this._resize);
  }

  disconnectedCallback() {
    window.removeEventListener('resize', this._resize);
    window.cancelAnimationFrame(this._zoomFrame);
  }

  async load() {
    this.innerHTML = '<div class="goal-map-loading">正在展开目标全景…</div>';
    try {
      const response = await api.getGoalGraph(this.year);
      this.graph = response.data;
      this.render();
    } catch (error) {
      this.innerHTML = '<div class="goal-map-empty">目标地图暂时无法加载</div>';
      console.error('Failed to load goal graph:', error);
    }
  }

  render() {
    const goals = this.graph?.goals || [];
    this.innerHTML = `
      <section class="goal-map-shell">
        <header class="goal-map-header">
          <div>
            <span class="goal-map-eyebrow">目标全景 · ${this.year}</span>
            <h2>从长期方向，一直看到今天</h2>
            <p>滚轮缩放；按住空白或卡片拖动画布，点击卡片打开详情。</p>
          </div>
          <div class="goal-map-tools" aria-label="地图工具">
            <button class="btn-sm" data-action="previous-year">← ${this.year - 1}</button>
            <button class="btn-sm" data-action="zoom-out" aria-label="缩小">−</button>
            <span data-role="zoom-value">${Math.round(this.zoom * 100)}%</span>
            <button class="btn-sm" data-action="zoom-in" aria-label="放大">＋</button>
            <button class="btn-sm" data-action="current-year">今年</button>
          </div>
        </header>
        ${goals.length ? `
          <div class="goal-map-viewport">
            <div class="goal-map-stage" style="zoom:${this.zoom}">
              <svg class="goal-map-lines" aria-hidden="true"></svg>
              <div class="goal-map-columns">
                ${LEVELS.map(level => this.renderColumn(level, goals.filter(goal => goal.level === level))).join('')}
              </div>
            </div>
          </div>
        ` : `<div class="goal-map-empty"><strong>还没有可以连接的目标</strong><span>先创建年度方向，再逐层连接到月、周和今天。</span><button class="btn-sm btn-primary" data-action="create-year">创建年度目标</button></div>`}
        <aside class="goal-map-drawer" aria-hidden="true"></aside>
      </section>`;

    this.bindEvents();
    requestAnimationFrame(() => this.drawConnections());
  }

  renderColumn(level, goals) {
    return `<section class="goal-map-column" data-level="${level}">
      <header><span>${LEVEL_HINTS[level]}</span><strong>${LEVEL_LABELS[level]}</strong><em>${goals.length}</em></header>
      <div class="goal-map-stack">
        ${goals.map(goal => this.renderGoal(goal)).join('') || '<div class="goal-map-column-empty">尚未建立</div>'}
      </div>
    </section>`;
  }

  renderGoal(goal) {
    const goalKey = this.goalKey(goal.level, goal.period);
    const current = this.graph.current?.[goal.level] === goal.period;
    const tasks = (goal.tasks || []).filter(task => task.status !== 'cancelled');
    return `<article class="goal-map-group ${current ? 'is-current' : ''}" data-node-id="${goalKey}" data-level="${goal.level}" data-period="${goal.period}" data-parent-id="${escapeHtml(goal.parent_id || '')}">
      <button class="goal-map-goal" data-action="open-goal">
        <span>${periodLabel(goal.level, goal.period)}</span>
        <strong>${escapeHtml(goal.overview || '未写目标概述')}</strong>
        <em>${goal.progress || 0}%</em>
      </button>
      <div class="goal-map-tasks">
        ${tasks.map(task => `<button class="goal-map-task importance-${task.importance || 3} ${task.status === 'completed' ? 'is-done' : ''}" data-action="open-task" data-task-id="${task.id}" data-node-id="${this.taskKey(task.id)}" data-source-task-id="${escapeHtml(task.source_task_id || '')}">
          <i></i><span>${escapeHtml(task.title)}</span><small>${task.importance || 3}</small>
        </button>`).join('') || '<span class="goal-map-no-tasks">还没有行动</span>'}
      </div>
    </article>`;
  }

  bindEvents() {
    this.querySelectorAll('[data-action="open-goal"]').forEach(button => button.addEventListener('click', () => {
      const group = button.closest('.goal-map-group');
	  this.openDrawer(group.dataset.level, group.dataset.period);
    }));
    this.querySelectorAll('[data-action="open-task"]').forEach(button => button.addEventListener('click', () => {
      const group = button.closest('.goal-map-group');
	  this.openDrawer(group.dataset.level, group.dataset.period, button.dataset.taskId);
    }));
	this.querySelector('[data-action="create-year"]')?.addEventListener('click', () => this.openDetailPage('yearly', String(this.year)));
    this.querySelector('[data-action="previous-year"]')?.addEventListener('click', () => { this.year--; this.load(); });
    this.querySelector('[data-action="current-year"]')?.addEventListener('click', () => { this.year = new Date().getFullYear(); this.load(); });
    this.querySelector('[data-action="zoom-in"]')?.addEventListener('click', () => this.setZoom(this.zoom + 0.1));
    this.querySelector('[data-action="zoom-out"]')?.addEventListener('click', () => this.setZoom(this.zoom - 0.1));
    this.enableWheelZoom();
    this.enablePan();
  }

  openDetailPage(level, period) {
    store.setState({ level, period, view: 'detail' });
    store.dispatch('view:changed', 'detail');
    store.dispatch('period:changed', period);
  }

	async openDrawer(level, period, taskID = '') {
		const drawer = this.querySelector('.goal-map-drawer');
		if (!drawer) return;
		drawer.classList.add('is-open');
		drawer.setAttribute('aria-hidden', 'false');
		drawer.innerHTML = '<div class="goal-map-drawer-loading">正在打开目标…</div>';
		try {
			const response = await api.getGoal(level, period);
			const goal = response.data;
			let parentGoal = null;
			if (goal.parent_id) {
				const [parentLevel, parentPeriod] = goal.parent_id.split('|');
				if (parentLevel && parentPeriod) {
					const parentResponse = await api.getGoal(parentLevel, parentPeriod);
					parentGoal = parentResponse.data || null;
				}
			}
			drawer.innerHTML = `<div class="goal-map-drawer-bar"><strong>${periodLabel(level, period)}</strong><button class="btn-icon-sm" data-action="close-drawer" aria-label="关闭">✕</button></div><div class="goal-map-drawer-content"><goal-detail></goal-detail></div>`;
			store.setState({ level, period, goal, parentGoal, editTask: taskID ? (goal.tasks || []).find(task => task.id === taskID) || null : null });
			drawer.querySelector('[data-action="close-drawer"]')?.addEventListener('click', () => this.closeDrawer());
		} catch (error) {
			drawer.innerHTML = '<div class="goal-map-drawer-loading">目标无法打开</div>';
		}
	}

	closeDrawer() {
		const drawer = this.querySelector('.goal-map-drawer');
		if (!drawer) return;
		drawer.classList.remove('is-open');
		drawer.setAttribute('aria-hidden', 'true');
		drawer.innerHTML = '';
		store.setState({ editTask: null });
		this.load();
	}

  setZoom(value, focusPoint = null) {
    const nextZoom = Math.min(1.3, Math.max(0.7, Number(value.toFixed(2))));
    if (nextZoom === this.zoom) return;

    const viewport = this.querySelector('.goal-map-viewport');
    const stage = this.querySelector('.goal-map-stage');
    const zoomValue = this.querySelector('[data-role="zoom-value"]');
    const previousZoom = this.zoom;
    this.zoom = nextZoom;
    if (zoomValue) zoomValue.textContent = `${Math.round(this.zoom * 100)}%`;
    if (!viewport || !stage) return;

    const focus = focusPoint || { x: viewport.clientWidth / 2, y: viewport.clientHeight / 2 };
    const contentX = (viewport.scrollLeft + focus.x) / previousZoom;
    const contentY = (viewport.scrollTop + focus.y) / previousZoom;
    stage.style.zoom = this.zoom;
    viewport.scrollLeft = contentX * this.zoom - focus.x;
    viewport.scrollTop = contentY * this.zoom - focus.y;
    window.cancelAnimationFrame(this._zoomFrame);
    this._zoomFrame = requestAnimationFrame(() => this.drawConnections());
  }

  enableWheelZoom() {
    const viewport = this.querySelector('.goal-map-viewport');
    if (!viewport) return;
    viewport.addEventListener('wheel', event => {
      event.preventDefault();
      if (!event.deltaY) return;

      const rect = viewport.getBoundingClientRect();
      const deltaUnit = event.deltaMode === WheelEvent.DOM_DELTA_LINE
        ? 16
        : (event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? viewport.clientHeight : 1);
      const zoomFactor = Math.exp(-event.deltaY * deltaUnit * 0.001);
      this.setZoom(this.zoom * zoomFactor, {
        x: event.clientX - rect.left,
        y: event.clientY - rect.top,
      });
    }, { passive: false });
  }

  enablePan() {
    const viewport = this.querySelector('.goal-map-viewport');
    if (!viewport) return;
    const dragThreshold = 6;
    let gesture = null;
    let suppressNextClick = false;
    let suppressClickTimer = null;

    viewport.addEventListener('pointerdown', event => {
      if (event.button !== 0) return;
      const interactive = event.target.closest('button, input, textarea, select, a, [contenteditable="true"]');
      const draggableCard = interactive?.matches('[data-action="open-goal"], [data-action="open-task"]');
      if (interactive && !draggableCard) return;

      gesture = {
        pointerId: event.pointerId,
        x: event.clientX,
        y: event.clientY,
        left: viewport.scrollLeft,
        top: viewport.scrollTop,
        dragging: false,
      };
      viewport.setPointerCapture(event.pointerId);
    });

    viewport.addEventListener('pointermove', event => {
      if (!gesture || event.pointerId !== gesture.pointerId) return;
      const offsetX = event.clientX - gesture.x;
      const offsetY = event.clientY - gesture.y;
      if (!gesture.dragging && Math.hypot(offsetX, offsetY) < dragThreshold) return;

      gesture.dragging = true;
      viewport.classList.add('is-panning');
      viewport.scrollLeft = gesture.left - offsetX;
      viewport.scrollTop = gesture.top - offsetY;
      event.preventDefault();
    });

    const finishPan = event => {
      if (!gesture || event.pointerId !== gesture.pointerId) return;
      const pointerId = gesture.pointerId;
      const dragged = gesture.dragging;
      gesture = null;
      viewport.classList.remove('is-panning');
      if (viewport.hasPointerCapture(pointerId)) viewport.releasePointerCapture(pointerId);
      if (!dragged) return;

      suppressNextClick = true;
      window.clearTimeout(suppressClickTimer);
      suppressClickTimer = window.setTimeout(() => { suppressNextClick = false; }, 250);
    };

    viewport.addEventListener('pointerup', finishPan);
    viewport.addEventListener('pointercancel', finishPan);
    viewport.addEventListener('lostpointercapture', finishPan);
    viewport.addEventListener('click', event => {
      if (!suppressNextClick) return;
      suppressNextClick = false;
      window.clearTimeout(suppressClickTimer);
      event.preventDefault();
      event.stopImmediatePropagation();
    }, true);
  }

  drawConnections() {
    const stage = this.querySelector('.goal-map-stage');
    const svg = this.querySelector('.goal-map-lines');
    if (!stage || !svg) return;
    const stageRect = stage.getBoundingClientRect();
    const width = stage.scrollWidth;
    const height = stage.scrollHeight;
    svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
    svg.setAttribute('width', width);
    svg.setAttribute('height', height);
    const paths = [];

    this.querySelectorAll('.goal-map-group').forEach(group => {
      const parentID = group.dataset.parentId;
	  let parentGoalNode = null;
      if (parentID) {
		const [level, period] = parentID.split('|');
		parentGoalNode = this.findNode(this.goalKey(level, period))?.querySelector('.goal-map-goal') || null;
		this.pushPath(paths, parentGoalNode, group.querySelector('.goal-map-goal'), stageRect, group.classList.contains('is-current'));
      }
      group.querySelectorAll('.goal-map-task').forEach(task => {
        const sourceID = task.dataset.sourceTaskId;
		const sourceNode = sourceID ? this.findNode(this.taskKey(sourceID)) : parentGoalNode;
		this.pushPath(paths, sourceNode, task, stageRect, group.classList.contains('is-current'));
      });
    });
    svg.innerHTML = paths.join('');
  }

  pushPath(paths, from, to, stageRect, active) {
    if (!from || !to) return;
    const a = from.getBoundingClientRect();
    const b = to.getBoundingClientRect();
    const x1 = (a.right - stageRect.left) / this.zoom;
    const y1 = (a.top + a.height / 2 - stageRect.top) / this.zoom;
    const x2 = (b.left - stageRect.left) / this.zoom;
    const y2 = (b.top + b.height / 2 - stageRect.top) / this.zoom;
    const bend = Math.max(36, (x2 - x1) * 0.48);
    paths.push(`<path class="${active ? 'is-active' : ''}" d="M ${x1} ${y1} C ${x1 + bend} ${y1}, ${x2 - bend} ${y2}, ${x2} ${y2}"/>`);
  }

  findNode(id) { return [...this.querySelectorAll('[data-node-id]')].find(node => node.dataset.nodeId === id); }
  goalKey(level, period) { return `goal:${level}:${period}`; }
  taskKey(id) { return `task:${id}`; }
}

customElements.define('goal-map', GoalMap);
