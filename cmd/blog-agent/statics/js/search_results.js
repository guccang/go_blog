(function () {
    'use strict';

    var page = document.getElementById('searchPage');
    var form = document.getElementById('searchPageForm');
    var input = document.getElementById('searchPageInput');
    var status = document.getElementById('searchPageStatus');
    var results = document.getElementById('searchResults');
    var loadMore = document.getElementById('searchLoadMore');
    var query = (page.dataset.query || '').trim();
    var offset = 0;
    var loading = false;

    function appendSnippet(target, snippet) {
        var remaining = String(snippet || '');
        while (remaining.length) {
            var start = remaining.indexOf('<mark>');
            if (start < 0) {
                target.appendChild(document.createTextNode(remaining));
                return;
            }
            if (start > 0) target.appendChild(document.createTextNode(remaining.slice(0, start)));
            remaining = remaining.slice(start + 6);
            var end = remaining.indexOf('</mark>');
            if (end < 0) {
                target.appendChild(document.createTextNode(remaining));
                return;
            }
            var mark = document.createElement('mark');
            mark.textContent = remaining.slice(0, end);
            target.appendChild(mark);
            remaining = remaining.slice(end + 7);
        }
    }

    function renderItem(item) {
        var link = document.createElement('a');
        link.className = 'search-result';
        link.href = item.url;
        var title = document.createElement('strong');
        title.textContent = item.title;
        var snippet = document.createElement('p');
        appendSnippet(snippet, item.snippet);
        link.appendChild(title);
        link.appendChild(snippet);
        results.appendChild(link);
    }

    function load(reset) {
        if (loading || !query) return;
        if (reset) {
            offset = 0;
            results.textContent = '';
        }
        loading = true;
        loadMore.hidden = true;
        status.textContent = offset ? '正在加载更多结果…' : '正在检索本地博客…';
        fetch('/api/blogs/fts?q=' + encodeURIComponent(query) + '&limit=20&offset=' + offset, {
            credentials: 'same-origin'
        })
            .then(function (response) {
                if (!response.ok) throw new Error('HTTP ' + response.status);
                return response.json();
            })
            .then(function (payload) {
                var items = payload.items || [];
                items.forEach(renderItem);
                offset = Number(payload.next_offset || (offset + items.length));
                if (!results.children.length) {
                    var empty = document.createElement('div');
                    empty.className = 'empty-query';
                    empty.textContent = '没有找到匹配的非加密、非日记内容。';
                    results.appendChild(empty);
                }
                status.textContent = '已显示 ' + (results.querySelectorAll('.search-result').length) + ' 条结果';
                loadMore.hidden = !payload.has_more;
            })
            .catch(function () {
                status.textContent = '搜索失败，请稍后重试。';
            })
            .finally(function () {
                loading = false;
            });
    }

    form.addEventListener('submit', function (event) {
        event.preventDefault();
        var value = (input.value || '').trim();
        if (!value) {
            status.textContent = '请输入搜索内容。';
            input.focus();
            return;
        }
        window.location.assign('/search?match=' + encodeURIComponent(value));
    });
    loadMore.addEventListener('click', function () { load(false); });

    if (query) {
        load(true);
    } else {
        status.textContent = '输入关键词开始搜索。';
        input.focus();
    }
})();
