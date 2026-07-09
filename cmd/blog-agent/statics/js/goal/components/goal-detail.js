// statics/js/goal/components/goal-detail.js
import './goal-overview.js';
import './task-list.js';
import './task-editor.js';

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
