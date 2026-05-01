// ===== State =====
var currentView = 'grid';

// ===== Init =====
document.addEventListener('DOMContentLoaded', function() {
    initViewToggle();
    initSearch();
    addKeyboardShortcuts();

    var savedView = localStorage.getItem('publicViewMode');
    if (savedView === 'list') {
        toggleView();
    }
});

// ===== View Toggle =====
function initViewToggle() {
    var btn = document.querySelector('.btn-icon');
    if (!btn) return;
    btn.removeAttribute('onclick');
    btn.addEventListener('click', function(e) {
        e.preventDefault();
        toggleView();
    });
}

function toggleView() {
    var grid = document.getElementById('blog-container');
    var icon = document.getElementById('view-icon');
    if (!grid) return;

    if (currentView === 'grid') {
        grid.classList.add('list-view');
        if (icon) icon.className = 'fas fa-list';
        currentView = 'list';
    } else {
        grid.classList.remove('list-view');
        if (icon) icon.className = 'fas fa-th-large';
        currentView = 'grid';
    }

    localStorage.setItem('publicViewMode', currentView);
}

// ===== Search =====
function initSearch() {
    var input = document.getElementById('search');
    var btn = document.querySelector('.nav-search .search-icon');
    if (!input) return;

    input.addEventListener('input', function() {
        var q = this.value.toLowerCase().trim();
        if (q.length > 0) performSearch(q);
        else showAll();
    });

    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            var q = this.value.toLowerCase().trim();
            if (q.length > 0) performSearch(q);
            else showAll();
        }
    });

    // Click search icon to focus
    if (btn) {
        btn.style.cursor = 'pointer';
        btn.addEventListener('click', function() { input.focus(); });
    }
}

function onSearch() {
    var input = document.getElementById('search');
    if (!input) return;
    var q = input.value.toLowerCase().trim();
    if (q.length > 0) performSearch(q);
    else showAll();
}

function performSearch(query) {
    var cards = document.querySelectorAll('.blog-card');
    cards.forEach(function(card) {
        var title = (card.querySelector('.blog-card-title')?.textContent || '').toLowerCase();
        card.style.display = title.includes(query) ? '' : 'none';
    });
}

function showAll() {
    document.querySelectorAll('.blog-card').forEach(function(card) {
        card.style.display = '';
    });
}

// ===== Keyboard Shortcuts =====
function addKeyboardShortcuts() {
    document.addEventListener('keydown', function(e) {
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            var input = document.getElementById('search');
            if (input) { input.focus(); input.select(); }
        }
        if ((e.ctrlKey || e.metaKey) && e.key === 'v') {
            e.preventDefault();
            toggleView();
        }
        if (e.key === 'Escape') {
            var input = document.getElementById('search');
            if (input) { input.value = ''; showAll(); input.blur(); }
        }
    });
}

// Exports
window.onSearch = onSearch;
window.toggleView = toggleView;
