// statics/js/goal/components/review-editor.js
import { store } from '../store.js';
import { api } from '../api.js';

class ReviewEditor extends HTMLElement {
  connectedCallback() {
    this._visible = false;
    this.render();
  }

  show() { this._visible = true; this.render(); }
  hide() { this._visible = false; this.render(); }

  render() {
    if (!this._visible) { this.innerHTML = ''; return; }
    const { review, level, period } = store.state;

    this.innerHTML = `
      <div class="modal-overlay" data-action="cancel">
        <div class="modal-content review-modal">
          <h3>编辑回顾</h3>
          <textarea class="review-textarea" rows="20">${escapeHtml((review && review.content) || '')}</textarea>
          <div class="modal-actions">
            <button class="btn-sm" data-action="cancel">取消</button>
            <button class="btn-sm btn-primary" data-action="save">保存回顾</button>
          </div>
        </div>
      </div>
    `;

    this.querySelector('[data-action="cancel"]')?.addEventListener('click', (e) => {
      if (e.target.dataset.action === 'cancel') this.hide();
    });
    this.querySelector('[data-action="save"]')?.addEventListener('click', async () => {
      const content = this.querySelector('.review-textarea').value;
      store.setState({ loading: true });
      try {
        const currentReview = store.state.review;
        await api.saveReview(level, period, content, currentReview?.completed, currentReview?.total);
        const res = await api.getReview(level, period);
        store.setState({ review: res.data, loading: false });
        this.hide();
      } catch (err) {
        console.error('Operation failed:', err);
        store.setState({ loading: false });
      }
    });
  }
}

function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s || ''; return d.innerHTML; }

customElements.define('review-editor', ReviewEditor);
