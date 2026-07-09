// statics/js/goal/components/review-panel.js
import { store } from '../store.js';
import { api } from '../api.js';
import { periodLabel } from '../utils.js';
import './review-editor.js';

class ReviewPanel extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.loadReview();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  async loadReview() {
    const { level, period } = store.state;
    store.setState({ loading: true });
    try {
      const res = await api.getReview(level, period);
      store.setState({ review: res.data || null, loading: false });
    } catch (e) {
      store.setState({ review: null, loading: false });
    }
  }

  render() {
    const { review, level, period, loading } = store.state;

    if (loading) {
      this.innerHTML = '<div class="loading">加载中...</div>';
      return;
    }

    this.innerHTML = `
      <div class="goal-card">
        <div class="review-header">
          <h3>${periodLabel(level, period)} 回顾</h3>
          ${review ? `
            <button class="btn-sm" data-action="edit-review">编辑</button>
          ` : ''}
        </div>
        ${review ? `
          <div class="review-stats">
            <span>已完成 ${review.completed}/${review.total} 任务</span>
          </div>
          <div class="review-content markdown-body">${this._renderMarkdown(review.content || '')}</div>
        ` : `
          <div class="empty-state">
            <p>还没有回顾记录</p>
            <button class="btn-sm btn-primary" data-action="generate">生成回顾</button>
          </div>
        `}
      </div>
      <review-editor></review-editor>
    `;

    this.querySelector('[data-action="generate"]')?.addEventListener('click', async () => {
      store.setState({ loading: true });
      const res = await api.generateReview(level, period);
      if (res.success && res.data) {
        store.setState({ review: res.data, loading: false });
      } else {
        store.setState({ loading: false });
      }
    });

    this.querySelector('[data-action="edit-review"]')?.addEventListener('click', () => {
      const editor = this.querySelector('review-editor');
      editor.show();
    });
  }

  _renderMarkdown(text) {
    return text
      .replace(/^### (.*$)/gm, '<h4>$1</h4>')
      .replace(/^## (.*$)/gm, '<h3>$1</h3>')
      .replace(/^# (.*$)/gm, '<h2>$1</h2>')
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      .replace(/^- (.*$)/gm, '<li>$1</li>')
      .replace(/\n\n/g, '</p><p>')
      .replace(/\[x\]/g, '<span style="color:var(--goal-success)">✓</span>')
      .replace(/\[ \]/g, '<span style="color:var(--goal-text-muted)">○</span>');
  }
}

customElements.define('review-panel', ReviewPanel);
