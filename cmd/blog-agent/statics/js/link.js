(function () {
    'use strict';

    var container = document.getElementById('blogContainer');
    var filterBar = document.getElementById('blogFilterBar');
    var emptyState = document.getElementById('blogFilterEmpty');
    var loadMore = document.getElementById('loadMoreBlogs');
    var activeFilter = 'all';
    var cards = container ? Array.from(container.querySelectorAll('.blog-card')) : [];
    var categories = [
        { key: 'all', label: '全部内容' },
        { key: 'blog', label: '日常博客' },
        { key: 'diary', label: '日记' },
        { key: 'exercise', label: '锻炼' },
        { key: 'memory', label: '记忆' },
        { key: 'ai', label: 'AI 生成' },
        { key: 'tech', label: 'blog实现技术文档' },
        { key: 'system', label: '系统' }
    ];
    var counts = {};

    window.onSearch = function () {
        var input = document.getElementById('search');
        var match = input ? input.value.trim() : '';
        if (!match) return;
        window.location.assign('/search?match=' + encodeURIComponent(match));
    };

    window.handleSearchCommand = function (select) {
        var option = select.options[select.selectedIndex];
        if (!option || !option.value) return;
        var input = document.getElementById('search');
        var hasParam = option.getAttribute('data-hasparam') === 'true';
        input.value = option.value + (hasParam ? ' ' : '');
        input.focus();
        input.setSelectionRange(input.value.length, input.value.length);
        if (!hasParam) setTimeout(window.onSearch, 100);
        select.selectedIndex = 0;
    };

    window.toggleView = function () {
        if (!container) return;
        var listView = !container.classList.contains('list-view');
        container.classList.toggle('list-view', listView);
        document.getElementById('view-icon').className = listView ? 'fas fa-list' : 'fas fa-th-large';
        localStorage.setItem('blogViewPreference', listView ? 'list' : 'grid');
    };

    function classifyCard(card) {
        var title = (card.dataset.title || '').toLowerCase();
        if (card.dataset.techDoc === 'true') return 'tech';
        if (title.startsWith('sys_') || title.startsWith('mcp_') || title === 'sys_accounts') return 'system';
        if (title.startsWith('agent_')) return 'ai';
        if (title.includes('memory') || title.includes('记忆')) return 'memory';
        if (title.includes('exercise') || title.includes('锻炼') || title.includes('workout') || title.includes('健身')) return 'exercise';
        if (card.dataset.diary === 'true' || title.startsWith('日记_') || title.startsWith('diary_')) return 'diary';
        return 'blog';
    }

    function recount() {
        counts = { all: cards.length };
        cards.forEach(function (card) {
            var category = classifyCard(card);
            card.dataset.category = category;
            counts[category] = (counts[category] || 0) + 1;
        });
        if (!filterBar) return;
        filterBar.querySelectorAll('.blog-filter-chip').forEach(function (chip) {
            var count = chip.querySelector('.blog-filter-chip-count');
            if (count) count.textContent = counts[chip.dataset.category] || 0;
        });
    }

    function applyFilter(category) {
        activeFilter = category;
        var visibleCount = 0;
        cards.forEach(function (card) {
            var visible = category === 'all' || card.dataset.category === category;
            card.classList.toggle('is-hidden', !visible);
            if (visible) visibleCount++;
        });
        if (emptyState) emptyState.hidden = visibleCount > 0;
        filterBar.querySelectorAll('.blog-filter-chip').forEach(function (chip) {
            chip.classList.toggle('active', chip.dataset.category === category);
        });
    }

    function buildFilters() {
        if (!filterBar) return;
        categories.forEach(function (category) {
            var chip = document.createElement('button');
            chip.type = 'button';
            chip.className = 'blog-filter-chip' + (category.key === 'all' ? ' active' : '');
            chip.dataset.category = category.key;
            chip.appendChild(document.createTextNode(category.label));
            var count = document.createElement('span');
            count.className = 'blog-filter-chip-count';
            count.textContent = counts[category.key] || 0;
            chip.appendChild(count);
            chip.addEventListener('click', function () { applyFilter(category.key); });
            filterBar.appendChild(chip);
        });
    }

    function icon(className) {
        var element = document.createElement('i');
        element.className = className;
        return element;
    }

    function createBlogCard(item) {
        var card = document.createElement('article');
        card.className = 'blog-card' + (item.image_url ? ' has-media' : '');
        card.dataset.title = item.title;
        card.dataset.diary = item.diary ? 'true' : 'false';
        card.dataset.encrypted = item.encrypted ? 'true' : 'false';
        card.dataset.techDoc = item.tech_doc ? 'true' : 'false';

        var link = document.createElement('a');
        link.className = 'blog-card-link';
        link.href = item.url;
        if (item.image_url) {
            var image = document.createElement('img');
            image.className = 'blog-card-image';
            image.src = item.image_url;
            image.alt = '';
            image.loading = 'lazy';
            image.decoding = 'async';
            image.addEventListener('error', function () {
                image.remove();
                card.classList.remove('has-media');
            });
            link.appendChild(image);
        }
        var body = document.createElement('div');
        body.className = 'blog-card-body';
        if (item.access_time || item.encrypted || item.diary) {
            var meta = document.createElement('div');
            meta.className = 'blog-card-meta';
            if (item.access_time) {
                var date = document.createElement('span');
                date.className = 'blog-date';
                date.appendChild(document.createTextNode(item.access_time));
                meta.appendChild(date);
            }
            if (item.encrypted || item.diary) {
                var protectedLabel = document.createElement('span');
                protectedLabel.className = 'protected-label' + (item.diary ? ' diary' : '');
                protectedLabel.appendChild(icon(item.diary ? 'fas fa-book' : 'fas fa-lock'));
                protectedLabel.appendChild(document.createTextNode(item.diary ? ' 日记' : ' 加密'));
                meta.appendChild(protectedLabel);
            }
            body.appendChild(meta);
        }
        var title = document.createElement('h2');
        title.className = 'blog-card-title';
        title.appendChild(document.createTextNode(item.title));
        body.appendChild(title);
        var preview = document.createElement('p');
        preview.className = 'blog-card-preview';
        preview.textContent = item.preview || '打开文章继续阅读';
        body.appendChild(preview);
        link.appendChild(body);
        card.appendChild(link);
        return card;
    }

    function loadNextPage() {
        var offset = Number(loadMore.dataset.offset || 0);
        loadMore.disabled = true;
        loadMore.textContent = '正在加载…';
        fetch('/api/blogs/page?offset=' + offset + '&limit=20', { credentials: 'same-origin' })
            .then(function (response) {
                if (!response.ok) throw new Error('load failed');
                return response.json();
            })
            .then(function (payload) {
                (payload.items || []).forEach(function (item) {
                    var card = createBlogCard(item);
                    container.appendChild(card);
                    cards.push(card);
                });
                recount();
                applyFilter(activeFilter);
                loadMore.dataset.offset = String(offset + (payload.items || []).length);
                document.getElementById('loadMoreWrap').hidden =
                    !payload.has_more || (payload.items || []).length === 0;
                loadMore.disabled = false;
                loadMore.textContent = '加载更多内容';
            })
            .catch(function () {
                loadMore.disabled = false;
                loadMore.textContent = '加载失败，点击重试';
            });
    }

    var searchInput = document.getElementById('search');
    if (searchInput) {
        searchInput.addEventListener('keydown', function (event) {
            if (event.key === 'Enter') {
                event.preventDefault();
                window.onSearch();
            }
        });
    }
    var showAll = document.getElementById('showAllBlogs');
    if (showAll) showAll.addEventListener('click', function () { applyFilter('all'); });
    if (loadMore) loadMore.addEventListener('click', loadNextPage);

    document.querySelectorAll('.blog-card-image').forEach(function (image) {
        image.addEventListener('error', function () {
            var card = image.closest('.blog-card');
            image.remove();
            if (card) card.classList.remove('has-media');
        });
    });

    recount();
    buildFilters();
    if (localStorage.getItem('blogViewPreference') === 'list') window.toggleView();
})();
