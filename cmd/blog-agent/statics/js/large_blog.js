(function () {
    'use strict';

    var started = false;
    function initLargeBlog() {
        if (started) return;
        var editor = document.getElementById('editor-inner');
        var preview = document.getElementById('md');
        var controls = document.getElementById('largeBlogControls');
        var status = document.getElementById('largeBlogStatus');
        var loadMore = document.getElementById('largeBlogLoadMore');
        if (!editor || !preview || !controls || !status || !loadMore || editor.dataset.largeBlog !== 'true') return;
        started = true;

        var offset = 0;
        var hasMore = true;
        var loading = false;
        var loadedChunks = 0;

        function updateControls() {
            loadMore.disabled = loading || !hasMore;
            loadMore.textContent = hasMore ? (loading ? '加载中…' : '加载更多') : '已加载全部内容';
        }

        function appendChunk(content) {
            if (loadedChunks === 0) preview.textContent = '';
            loadedChunks++;
            var section = document.createElement('pre');
            section.className = 'large-blog-chunk';
            section.textContent = content;
            preview.appendChild(section);
        }

        function loadChunk() {
            if (loading || !hasMore) return;
            loading = true;
            status.textContent = '正在加载内容…';
            updateControls();
            var params = new URLSearchParams({ blogname: editor.dataset.blogname, account: editor.dataset.account, offset: String(offset) });
            fetch('/api/blog/content?' + params.toString(), { credentials: 'same-origin' })
                .then(function (response) { if (!response.ok) throw new Error('load failed'); return response.json(); })
                .then(function (payload) {
                    appendChunk(payload.content || '');
                    offset = payload.next_offset || offset;
                    hasMore = Boolean(payload.has_more);
                    status.textContent = hasMore ? '继续向下阅读，点击“加载更多”获取下一段。' : '已加载全部内容。';
                })
                .catch(function () { status.textContent = '内容加载失败，请刷新页面后重试。'; })
                .finally(function () { loading = false; updateControls(); });
        }

        loadMore.addEventListener('click', loadChunk);
        preview.textContent = '正在加载第一段内容…';
        loadChunk();
    }

    if (document.readyState === 'complete') {
        initLargeBlog();
    } else {
        window.addEventListener('load', initLargeBlog);
        window.addEventListener('pageshow', initLargeBlog);
    }
})();
