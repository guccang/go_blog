// 全局变量
let currentView = 'grid'; // 'grid' 或 'list'
let currentFilter = 'all';
let allBlogs = [];
let tagFiltersExpanded = false;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializePage();
});

// 初始化页面
function initializePage() {
    // 获取所有博客卡片
    const blogCards = document.querySelectorAll('.link-card');
    allBlogs = Array.from(blogCards);
    
    // 初始化搜索功能
    initializeSearch();
    
    // 初始化视图切换
    initializeViewToggle();
    
    // 初始化标签筛选
    initializeTagFilters();
    
    // 添加键盘快捷键
    addKeyboardShortcuts();
    
    // 从localStorage恢复视图模式
    const savedViewMode = localStorage.getItem('publicViewMode');
    if (savedViewMode && savedViewMode !== currentView) {
        toggleView();
    }
    
    // 显示页面加载完成
    showToast('公开博客页面加载完成', 'success');
}

// 初始化搜索功能
function initializeSearch() {
    const searchInput = document.getElementById('search');
    const searchBtn = document.querySelector('.search-btn');
    
    if (searchInput) {
        // 实时搜索
        searchInput.addEventListener('input', function() {
            const query = this.value.toLowerCase().trim();
            if (query.length > 0) {
                performSearch(query);
            } else {
                showAllBlogs();
            }
        });
        
        // 回车键搜索
        searchInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                const query = this.value.toLowerCase().trim();
                if (query.length > 0) {
                    performSearch(query);
                }
            }
        });
    }
    
    if (searchBtn) {
        // 移除可能存在的onclick属性，使用addEventListener
        searchBtn.removeAttribute('onclick');
        searchBtn.addEventListener('click', function(e) {
            e.preventDefault();
            const query = searchInput.value.toLowerCase().trim();
            if (query.length > 0) {
                performSearch(query);
            } else {
                showAllBlogs();
            }
        });
    }
}

// 执行搜索
function performSearch(query) {
    const blogCards = document.querySelectorAll('.link-card');
    let foundCount = 0;
    
    blogCards.forEach(card => {
        const title = card.querySelector('.blog-title')?.textContent.toLowerCase() || '';
        const tags = card.querySelectorAll('.tag');
        let hasMatchingTag = false;
        
        tags.forEach(tag => {
            if (tag.textContent.toLowerCase().includes(query)) {
                hasMatchingTag = true;
            }
        });
        
        if (title.includes(query) || hasMatchingTag) {
            card.style.display = 'block';
            card.style.animation = 'fadeIn 0.3s ease-out';
            foundCount++;
        } else {
            card.style.display = 'none';
        }
    });
    
    // 显示搜索结果统计
    updateSearchResults(foundCount, query);
}

// 显示所有博客
function showAllBlogs() {
    const blogCards = document.querySelectorAll('.link-card');
    blogCards.forEach(card => {
        card.style.display = 'block';
        card.style.animation = 'fadeIn 0.3s ease-out';
    });
    
    // 清除搜索结果统计
    clearSearchResults();
}

// 更新搜索结果统计
function updateSearchResults(count, query) {
    let statsSection = document.querySelector('.stats-section');
    if (!statsSection) return;
    
    // 创建或更新搜索结果统计
    let searchStats = document.getElementById('search-stats');
    if (!searchStats) {
        searchStats = document.createElement('div');
        searchStats.id = 'search-stats';
        searchStats.className = 'stat-card';
        statsSection.appendChild(searchStats);
    }
    
    searchStats.innerHTML = `
        <div class="stat-number">${count}</div>
        <div class="stat-label">搜索结果: "${query}"</div>
    `;
}

// 清除搜索结果统计
function clearSearchResults() {
    const searchStats = document.getElementById('search-stats');
    if (searchStats) {
        searchStats.remove();
    }
}

// 初始化视图切换
function initializeViewToggle() {
    const viewToggle = document.querySelector('.view-toggle');
    if (viewToggle) {
        // 移除可能存在的onclick属性，使用addEventListener
        viewToggle.removeAttribute('onclick');
        viewToggle.addEventListener('click', function(e) {
            e.preventDefault();
            toggleView();
        });
        console.log('视图切换按钮已初始化');
    } else {
        console.error('未找到视图切换按钮');
    }
}

// 切换视图
function toggleView() {
    console.log('toggleView 被调用，当前视图:', currentView);
    
    const container = document.querySelector('.container');
    const viewIcon = document.getElementById('view-icon');
    const viewText = document.getElementById('view-text');
    
    if (!container) {
        console.error('未找到容器元素');
        return;
    }
    
    if (currentView === 'grid') {
        // 切换到列表视图
        container.classList.add('list-view');
        if (viewIcon) viewIcon.textContent = '📋';
        if (viewText) viewText.textContent = '列表视图';
        currentView = 'list';
        console.log('切换到列表视图');
    } else {
        // 切换到网格视图
        container.classList.remove('list-view');
        if (viewIcon) viewIcon.textContent = '📑';
        if (viewText) viewText.textContent = '网格视图';
        currentView = 'grid';
        console.log('切换到网格视图');
    }
    
    // 保存视图偏好
    localStorage.setItem('publicViewMode', currentView);
    
    showToast(`已切换到${currentView === 'grid' ? '网格' : '列表'}视图`, 'info');
}

// 初始化标签筛选
function initializeTagFilters() {
    const tagFilters = document.querySelectorAll('.tag-filter');
    tagFilters.forEach(filter => {
        // 移除可能存在的onclick属性，使用addEventListener
        filter.removeAttribute('onclick');
        filter.addEventListener('click', function(e) {
            e.preventDefault();
            const tag = this.getAttribute('data-tag');
            filterByTag(tag);
        });
    });

    const toggleBtn = document.getElementById('tag-filters-toggle');
    if (toggleBtn) {
        toggleBtn.addEventListener('click', function(e) {
            e.preventDefault();
            tagFiltersExpanded = !tagFiltersExpanded;
            syncTagFilterCollapse();
        });
    }

    window.addEventListener('resize', syncTagFilterCollapse);
    syncTagFilterCollapse();
}

// 按标签筛选
function filterByTag(tag) {
    // 更新活动状态
    document.querySelectorAll('.tag-filter').forEach(btn => {
        btn.classList.remove('active');
    });
    const targetBtn = document.querySelector(`[data-tag="${tag}"]`);
    if (targetBtn) {
        targetBtn.classList.add('active');
    }
    
    currentFilter = tag;
    syncTagFilterCollapse();
    
    const blogCards = document.querySelectorAll('.link-card');
    let visibleCount = 0;
    
    blogCards.forEach(card => {
        if (tag === 'all') {
            card.style.display = 'block';
            card.style.animation = 'fadeIn 0.3s ease-out';
            visibleCount++;
        } else {
            const cardTags = card.getAttribute('data-tags');
            // 检查标签是否匹配（忽略大小写）
            let hasMatchingTag = false;
            if (cardTags) {
                // 将标签字符串分割成数组，过滤空字符串
                const tagArray = cardTags.trim().split(/\s+/);
                // 检查是否有标签匹配（忽略大小写）
                hasMatchingTag = tagArray.some(cardTag =>
                    cardTag.toLowerCase() === tag.toLowerCase()
                );
            }
            if (hasMatchingTag) {
                card.style.display = 'block';
                card.style.animation = 'fadeIn 0.3s ease-out';
                visibleCount++;
            } else {
                card.style.display = 'none';
            }
        }
    });
    
    // 更新统计信息
    updateFilterStats(visibleCount, tag);
    
    showToast(`已筛选标签: ${tag === 'all' ? '全部' : tag}`, 'info');
}

function getTagCollapseLimit() {
    if (window.innerWidth <= 480) {
        return 8;
    }
    if (window.innerWidth <= 768) {
        return 12;
    }
    return 18;
}

function syncTagFilterCollapse() {
    const tagFilters = document.getElementById('tag-filters');
    const toggleBtn = document.getElementById('tag-filters-toggle');
    if (!tagFilters || !toggleBtn) {
        return;
    }

    const allTagButtons = Array.from(tagFilters.querySelectorAll('.tag-filter'));
    const collapsibleButtons = allTagButtons.filter(btn => btn.getAttribute('data-tag') !== 'all');
    const limit = getTagCollapseLimit();
    const hiddenCount = Math.max(collapsibleButtons.length - limit, 0);

    if (hiddenCount <= 0) {
        collapsibleButtons.forEach(btn => btn.classList.remove('is-collapsed-hidden'));
        toggleBtn.hidden = true;
        toggleBtn.setAttribute('aria-expanded', 'true');
        return;
    }

    collapsibleButtons.forEach((btn, index) => {
        const isActive = btn.classList.contains('active');
        const shouldHide = !tagFiltersExpanded && index >= limit && !isActive;
        btn.classList.toggle('is-collapsed-hidden', shouldHide);
    });

    toggleBtn.hidden = false;
    toggleBtn.setAttribute('aria-expanded', String(tagFiltersExpanded));
    toggleBtn.textContent = tagFiltersExpanded
        ? '收起标签'
        : `展开更多标签 (${hiddenCount})`;
}

// 更新筛选统计
function updateFilterStats(count, tag) {
    let statsSection = document.querySelector('.stats-section');
    if (!statsSection) return;
    
    // 创建或更新筛选统计
    let filterStats = document.getElementById('filter-stats');
    if (!filterStats) {
        filterStats = document.createElement('div');
        filterStats.id = 'filter-stats';
        filterStats.className = 'stat-card';
        statsSection.appendChild(filterStats);
    }
    
    filterStats.innerHTML = `
        <div class="stat-number">${count}</div>
        <div class="stat-label">筛选结果: ${tag === 'all' ? '全部' : tag}</div>
    `;
}

// 搜索功能（兼容现有代码）
function onSearch() {
    const searchInput = document.getElementById('search');
    if (searchInput) {
        const query = searchInput.value.toLowerCase().trim();
        if (query.length > 0) {
            performSearch(query);
        } else {
            showAllBlogs();
        }
    }
}

// 添加键盘快捷键
function addKeyboardShortcuts() {
    document.addEventListener('keydown', function(e) {
        // Ctrl/Cmd + K: 聚焦搜索框
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            const searchInput = document.getElementById('search');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }
        
        // Ctrl/Cmd + V: 切换视图
        if ((e.ctrlKey || e.metaKey) && e.key === 'v') {
            e.preventDefault();
            toggleView();
        }
        
        // Escape: 清除搜索
        if (e.key === 'Escape') {
            const searchInput = document.getElementById('search');
            if (searchInput) {
                searchInput.value = '';
                showAllBlogs();
                searchInput.blur();
            }
        }
    });
}

// 显示提示消息
function showToast(message, type = 'info') {
    // 创建toast容器（如果不存在）
    let toastContainer = document.getElementById('toast-container');
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.id = 'toast-container';
        toastContainer.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 10000;
            display: flex;
            flex-direction: column;
            gap: 10px;
        `;
        document.body.appendChild(toastContainer);
    }
    
    // 创建toast元素
    const toast = document.createElement('div');
    toast.style.cssText = `
        background: ${type === 'success' ? '#4caf50' : type === 'error' ? '#f44336' : '#2196f3'};
        color: white;
        padding: 12px 20px;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        font-size: 14px;
        font-weight: 500;
        max-width: 300px;
        word-wrap: break-word;
        animation: slideIn 0.3s ease-out;
    `;
    toast.textContent = message;
    
    // 添加到容器
    toastContainer.appendChild(toast);
    
    // 自动移除
    setTimeout(() => {
        toast.style.animation = 'slideOut 0.3s ease-out';
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, 300);
    }, 3000);
}

// 添加CSS动画
const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from {
            transform: translateX(100%);
            opacity: 0;
        }
        to {
            transform: translateX(0);
            opacity: 1;
        }
    }
    
    @keyframes slideOut {
        from {
            transform: translateX(0);
            opacity: 1;
        }
        to {
            transform: translateX(100%);
            opacity: 0;
        }
    }
`;
document.head.appendChild(style);

// 页面可见性变化处理
document.addEventListener('visibilitychange', function() {
    if (document.visibilityState === 'visible') {
        // 页面重新可见时，恢复视图模式
        const savedViewMode = localStorage.getItem('publicViewMode');
        if (savedViewMode && savedViewMode !== currentView) {
            toggleView();
        }
    }
});

// 按标签搜索
function searchByTag(tagName) {
    window.location.href = '/search?match=' + encodeURIComponent('@tag match ' + tagName);
}

// 导出函数供模板使用
window.onSearch = onSearch;
window.toggleView = toggleView;
window.filterByTag = filterByTag;
window.searchByTag = searchByTag; 
