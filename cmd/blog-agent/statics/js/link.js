// ===== Search =====
function onSearch() {
    var match = document.getElementById('search').value;
    if (match.trim() === '') return;

    var isReloadCommand = match.toLowerCase().startsWith('@reload');
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState == 4 && xhr.status == 200) {
            if (isReloadCommand) {
                document.getElementById('search').value = '';
                setTimeout(function() {
                    window.location.href = xhr.responseURL;
                }, 1000);
            } else {
                window.location.href = xhr.responseURL;
            }
        }
    };
    xhr.open('GET', '/search?match=' + encodeURIComponent(match), true);
    xhr.send();
}

// ===== View Toggle =====
var isGridView = true;

function toggleView() {
    var grid = document.getElementById('blogContainer');
    var icon = document.getElementById('view-icon');

    isGridView = !isGridView;
    grid.classList.toggle('list-view');

    if (isGridView) {
        icon.className = 'fas fa-th-large';
    } else {
        icon.className = 'fas fa-list';
    }

    localStorage.setItem('blogViewPreference', isGridView ? 'grid' : 'list');
}

// ===== Keyboard Shortcuts =====
document.addEventListener('keydown', function(event) {
    if (event.key === "Enter" && document.activeElement === document.getElementById('search')) {
        event.preventDefault();
        onSearch();
    }
});

// ===== Init =====
document.addEventListener('DOMContentLoaded', function() {
    var quotes = [
        ['把今天能完成的一件小事，做到确实完成。', '给正在行动的自己'],
        ['生活不是等待风暴过去，而是学会在雨中跳舞。', '维维安·格林'],
        ['慢一点没关系，方向对了就仍在抵达。', '给长期主义者'],
        ['你不必很厉害才开始，但要开始才会很厉害。', '给今天的第一步'],
        ['真正重要的事，往往安静地发生在重复里。', '给持续练习的人']
    ];
    var quote = quotes[(new Date().getDate() - 1) % quotes.length];
    var quoteText = document.getElementById('dailyQuote');
    var quoteSource = document.getElementById('dailyQuoteSource');
    if (quoteText && quoteSource) {
        quoteText.textContent = quote[0];
        quoteSource.textContent = '— ' + quote[1];
    }

    // Restore view preference
    var savedView = localStorage.getItem('blogViewPreference');
    if (savedView === 'list') {
        toggleView();
    }

    // Top bar scroll effect
    var topBar = document.querySelector('.top-bar');
    if (topBar) {
        window.addEventListener('scroll', function() {
            if (window.scrollY > 10) {
                topBar.classList.add('scrolled');
            } else {
                topBar.classList.remove('scrolled');
            }
        });
    }

    // ===== Blog Category Filters =====
    var container = document.getElementById('blogContainer');
    if (!container) return;

    var cards = Array.from(container.querySelectorAll('.blog-card'));
    if (cards.length === 0) return;

    var categories = [
        { key: 'all',      label: '全部内容' },
        { key: 'blog',     label: '日常博客' },
        { key: 'diary',    label: '日记' },
        { key: 'exercise', label: '锻炼' },
        { key: 'memory',   label: '记忆' },
        { key: 'ai',       label: 'AI 生成' },
        { key: 'system',   label: '系统' },
    ];

    function classifyCard(card) {
        var title = (card.getAttribute('data-title') || '').toLowerCase();
        var isDiary = card.getAttribute('data-diary') === 'true';
        if (title.startsWith('sys_') || title.startsWith('mcp_') || title === 'sys_accounts') return 'system';
        if (title.startsWith('agent_')) return 'ai';
        if (title.includes('memory') || title.includes('\u8bb0\u5fc6')) return 'memory';
        if (title.includes('exercise') || title.includes('\u953b\u70bc') || title.includes('workout') || title.includes('\u5065\u8eab')) return 'exercise';
        if (isDiary || title.startsWith('\u65e5\u8bb0_') || title.startsWith('diary_')) return 'diary';
        return 'blog';
    }

    var counts = { all: cards.length };
    cards.forEach(function(card) {
        var category = classifyCard(card);
        card.dataset.category = category;
        counts[category] = (counts[category] || 0) + 1;
    });

    var filterBar = document.getElementById('blogFilterBar');
    var emptyState = document.getElementById('blogFilterEmpty');
    var activeFilter = 'all';

    function applyFilter(category) {
        activeFilter = category;
        var visibleCount = 0;
        cards.forEach(function(card) {
            var visible = category === 'all' || card.dataset.category === category;
            card.classList.toggle('is-hidden', !visible);
            if (visible) visibleCount++;
        });
        emptyState.hidden = visibleCount > 0;
        filterBar.querySelectorAll('.blog-filter-chip').forEach(function(chip) {
            chip.classList.toggle('active', chip.dataset.category === category);
        });
    }

    categories.forEach(function(category) {
        var chip = document.createElement('button');
        chip.type = 'button';
        chip.className = 'blog-filter-chip' + (category.key === activeFilter ? ' active' : '');
        chip.dataset.category = category.key;
        chip.textContent = category.label;
        var count = document.createElement('span');
        count.className = 'blog-filter-chip-count';
        count.textContent = counts[category.key] || 0;
        chip.appendChild(count);
        chip.addEventListener('click', function() { applyFilter(category.key); });
        filterBar.appendChild(chip);
    });

    document.getElementById('showAllBlogs').addEventListener('click', function() {
        applyFilter('all');
    });

    var loadMore = document.getElementById('loadMoreBlogs');
    if (loadMore) {
        loadMore.addEventListener('click', function() {
            var offset = Number(loadMore.dataset.offset || 0);
            loadMore.disabled = true;
            loadMore.textContent = '正在加载…';
            fetch('/api/blogs/page?offset=' + offset + '&limit=20', { credentials: 'same-origin' })
                .then(function(response) { if (!response.ok) throw new Error('load failed'); return response.json(); })
                .then(function(payload) {
                    (payload.items || []).forEach(function(item) {
                        var card = document.createElement('article');
                        card.className = 'blog-card';
                        card.dataset.title = item.title;
                        card.dataset.diary = item.diary ? 'true' : 'false';
                        card.dataset.encrypted = item.encrypted ? 'true' : 'false';
                        card.innerHTML = '<a href="' + item.url + '" class="blog-card-link"><div class="blog-card-body"><h3 class="blog-card-title">' + (item.encrypted ? '<i class="fas fa-lock lock-icon"></i>' : '') + (item.diary ? '<i class="fas fa-book diary-icon"></i>' : '') + escapeHtml(item.title) + '</h3><div class="blog-card-meta"><span class="blog-date"><i class="far fa-clock"></i> ' + escapeHtml(item.access_time || '') + '</span></div></div><div class="blog-card-arrow"><i class="fas fa-chevron-right"></i></div></a>';
                        container.appendChild(card);
                    });
                    loadMore.dataset.offset = String(offset + (payload.items || []).length);
                    if (!payload.has_more || (payload.items || []).length === 0) {
                        document.getElementById('loadMoreWrap').hidden = true;
                    } else {
                        loadMore.disabled = false;
                        loadMore.textContent = '加载更多内容';
                    }
                })
                .catch(function() { loadMore.disabled = false; loadMore.textContent = '加载失败，点击重试'; });
        });
    }
});

function escapeHtml(value) {
    var node = document.createElement('span');
    node.textContent = value;
    return node.innerHTML;
}

// ===== History Back =====
function PageHistoryBack() {
    // handled by utils.js if needed
}
