(() => {
    const palettes = ['#3f9c91','#dc704b','#325d80','#b68c43','#765e94','#5b7f70'];
    const escapeHTML = (value) => { const span = document.createElement('span'); span.textContent = value || ''; return span.innerHTML; };
    const progress = (book) => book.total_pages > 0 ? Math.min(100, Math.round((book.current_page || 0) / book.total_pages * 100)) : 0;
    function bookTile(book, index) {
        const icon = book.cover_url ? `<img src="${escapeHTML(book.cover_url)}" alt="">` : `<span>${escapeHTML((book.title || '书').slice(0,1))}</span>`;
        return `<a class="book-tile" href="/reading/book/${encodeURIComponent(book.id)}"><div class="book-icon" style="--book:${palettes[index % palettes.length]}">${icon}</div><strong title="${escapeHTML(book.title)}">${escapeHTML(book.title)}</strong><small>${escapeHTML(book.author || '未知作者')}</small><div class="book-progress"><i style="width:${progress(book)}%"></i></div></a>`;
    }
    async function load() {
        try {
            const [booksResponse, statsResponse] = await Promise.all([fetch('/api/books?sort_by=add_time&sort_order=desc'),fetch('/api/reading-statistics')]);
            if (!booksResponse.ok) throw new Error('书架加载失败');
            const books = (await booksResponse.json()).books || [];
            const stats = statsResponse.ok ? await statsResponse.json() : {};
            document.getElementById('totalBooks').textContent = stats.total_books || books.length;
            document.getElementById('readingCount').textContent = stats.reading_books || books.filter((book) => book.status === 'reading').length;
            document.getElementById('finishedBooks').textContent = stats.finished_books || books.filter((book) => book.status === 'finished').length;
            document.getElementById('totalPages').textContent = stats.total_pages || books.reduce((sum, book) => sum + (book.total_pages || 0), 0);
            const reading = books.filter((book) => book.status === 'reading');
            const finished = books.filter((book) => book.status === 'finished').slice(0, 6);
            document.getElementById('readingShelf').innerHTML = reading.map(bookTile).join('');
            document.getElementById('finishedShelf').innerHTML = finished.map((book,index) => bookTile(book,index + reading.length)).join('');
            document.getElementById('readingEmpty').hidden = reading.length !== 0;
            document.getElementById('finishedEmpty').hidden = finished.length !== 0;
        } catch (error) { document.getElementById('readingEmpty').hidden = false; document.getElementById('readingEmpty').querySelector('p').textContent = '书架暂时无法加载，请稍后重试。'; }
    }
    load();
})();
