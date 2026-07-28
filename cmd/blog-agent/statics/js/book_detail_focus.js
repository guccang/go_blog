(() => {
    const body = document.body;
    const bookID = body.dataset.bookId;
    const total = Number(body.dataset.totalPages || 0);
    let current = Number(body.dataset.currentPage || 0);
    const input = document.getElementById('pageInput');
    const toast = document.getElementById('toast');
    let timer;

    function message(text) { toast.textContent = text; toast.classList.add('show'); clearTimeout(timer); timer = setTimeout(() => toast.classList.remove('show'), 2600); }
    function statusText(status) { return { reading:'正在阅读', finished:'已读完', paused:'暂停阅读', unstart:'尚未开始' }[status] || '阅读中'; }
    function renderProgress() {
        current = Math.max(0, total ? Math.min(total, Number(input.value || 0)) : Number(input.value || 0));
        input.value = current;
        const percent = total ? Math.round(current / total * 100) : 0;
        document.getElementById('progressPercent').textContent = `${percent}%`;
        document.getElementById('progressRing').style.setProperty('--progress', `${percent}%`);
        document.getElementById('progressBar').style.width = `${percent}%`;
        document.getElementById('pageSummary').textContent = total ? `${current} / ${total} 页` : `${current} 页`;
        document.getElementById('statusFact').textContent = statusText(body.dataset.status);
        document.getElementById('statusLabel').textContent = statusText(body.dataset.status);
    }
    async function request(url, options) { const response = await fetch(url, options); if (!response.ok) throw new Error(await response.text()); return response.json(); }
    document.querySelectorAll('.page-step').forEach((button) => button.addEventListener('click', () => { input.value = Number(input.value || 0) + Number(button.dataset.step); renderProgress(); }));
    input.addEventListener('input', renderProgress);
    document.getElementById('saveProgress').addEventListener('click', async () => { try { await request(`/api/books/progress?book_id=${encodeURIComponent(bookID)}`, { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({ current_page:current }) }); body.dataset.status = current > 0 ? 'reading' : body.dataset.status; renderProgress(); message('阅读进度已保存'); } catch (error) { message(`保存失败：${error.message}`); } });
    document.getElementById('finishBook').addEventListener('click', async () => { try { await request(`/api/books/finish?book_id=${encodeURIComponent(bookID)}`, { method:'POST' }); body.dataset.status = 'finished'; if (total) { input.value = total; } renderProgress(); message('恭喜，已经读完这本书'); } catch (error) { message(`操作失败：${error.message}`); } });
    function escapeHTML(value) { const item = document.createElement('span'); item.textContent = value || ''; return item.innerHTML; }
    async function loadNotes() { try { const notes = await request(`/api/books/notes?book_id=${encodeURIComponent(bookID)}`); document.getElementById('notesList').innerHTML = notes.length ? notes.map((note) => `<article class="note"><p>${escapeHTML(note.content)}</p><small>${escapeHTML(note.chapter || '阅读笔记')} · 第 ${note.page || 0} 页</small></article>`).join('') : '<p class="muted">还没有笔记。读到有感触的地方就记一句。</p>'; } catch { document.getElementById('notesList').innerHTML = '<p class="muted">笔记暂时无法加载。</p>'; } }
    document.getElementById('noteForm').addEventListener('submit', async (event) => { event.preventDefault(); const chapter = document.getElementById('noteChapter'); const content = document.getElementById('noteContent'); try { await request(`/api/books/notes?book_id=${encodeURIComponent(bookID)}`, { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({ chapter:chapter.value, page:current, content:content.value }) }); chapter.value = ''; content.value = ''; message('笔记已保存'); loadNotes(); } catch (error) { message(`保存失败：${error.message}`); } });
    renderProgress();
    loadNotes();
})();
