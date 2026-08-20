// statics/js/goal/components/goal-detail.js
import './goal-overview.js?v=goal-add-no-flash-2';
import './task-list.js?v=goal-ai-overview-1';
import './task-editor.js?v=goal-map-1';

class GoalDetail extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `
      <goal-overview></goal-overview>
      <task-list></task-list>
      <task-editor></task-editor>
    `;
  }
}

customElements.define('goal-detail', GoalDetail);
