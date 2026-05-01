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

    // ===== Blog Category Fold =====
    var container = document.getElementById('blogContainer');
    if (!container) return;

    var cards = Array.from(container.querySelectorAll('.blog-card'));
    if (cards.length === 0) return;

    var categories = [
        { key: 'blog',     label: '📝 日常博客', color: '#d4734a' },
        { key: 'diary',    label: '📔 日记文件', color: '#e67e22' },
        { key: 'exercise', label: '💪 锻炼文件', color: '#27ae60' },
        { key: 'memory',   label: '🧠 记忆文件', color: '#3498db' },
        { key: 'ai',       label: '🤖 AI 生成', color: '#9b59b6' },
        { key: 'system',   label: '⚙ 系统文件', color: '#6c757d' },
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

    var groups = {};
    categories.forEach(function(c) { groups[c.key] = []; });
    cards.forEach(function(card) { groups[classifyCard(card)].push(card); });

    function getCollapseState(key) {
        try {
            var saved = localStorage.getItem('blog_category_state');
            if (saved) { var s = JSON.parse(saved); if (s[key] !== undefined) return s[key]; }
        } catch(e) {}
        return key !== 'blog';
    }

    function saveCollapseState(key, collapsed) {
        try {
            var s = {};
            var saved = localStorage.getItem('blog_category_state');
            if (saved) s = JSON.parse(saved);
            s[key] = collapsed;
            localStorage.setItem('blog_category_state', JSON.stringify(s));
        } catch(e) {}
    }

    container.innerHTML = '';
    container.classList.add('categorized');

    categories.forEach(function(cat) {
        var items = groups[cat.key];
        if (items.length === 0) return;

        var collapsed = getCollapseState(cat.key);

        var section = document.createElement('div');
        section.className = 'category-section';

        var header = document.createElement('div');
        header.className = 'category-header';
        header.style.borderLeftColor = cat.color;

        var left = document.createElement('div');
        left.className = 'category-header-left';
        left.innerHTML = '<span class="category-label">' + cat.label + '</span><span class="category-count">' + items.length + '</span>';

        var toggle = document.createElement('span');
        toggle.className = 'category-toggle';
        toggle.innerHTML = '<i class="fas fa-chevron-' + (collapsed ? 'right' : 'down') + '"></i>';

        header.appendChild(left);
        header.appendChild(toggle);

        var body = document.createElement('div');
        body.className = 'category-body' + (collapsed ? ' collapsed' : '');
        items.forEach(function(card) { body.appendChild(card); });

        header.addEventListener('click', function() {
            var isC = body.classList.toggle('collapsed');
            toggle.innerHTML = '<i class="fas fa-chevron-' + (isC ? 'right' : 'down') + '"></i>';
            saveCollapseState(cat.key, isC);
        });

        section.appendChild(header);
        section.appendChild(body);
        container.appendChild(section);
    });

    // Adjust category body grid for list view
    if (savedView === 'list') {
        document.querySelectorAll('.category-body').forEach(function(b) {
            b.style.gridTemplateColumns = '1fr';
        });
    }
});

// ===== History Back =====
function PageHistoryBack() {
    // handled by utils.js if needed
}
