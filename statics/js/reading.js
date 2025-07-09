// 全局变量
let allBooks = [];
let filteredBooks = [];
let currentFilter = {
    status: '',
    category: '',
    rating: '',
    search: ''
};
let currentSort = {
    sortBy: 'add_time',
    sortOrder: 'desc'
};

// DOM元素
const sidebar = document.getElementById('sidebar-container');
const bubble = document.getElementById('bubble');
const container = document.querySelector('.container');
const searchInput = document.getElementById('search-input');
const searchBtn = document.getElementById('search-btn');
const addBookBtn = document.getElementById('add-book-btn');
const importUrlBtn = document.getElementById('import-url-btn');
const booksGrid = document.getElementById('books-grid');
const emptyState = document.getElementById('empty-state');
const toastContainer = document.getElementById('toast-container');

// 模态框元素
const addBookModal = document.getElementById('add-book-modal');
const importUrlModal = document.getElementById('import-url-modal');
const addBookForm = document.getElementById('add-book-form');

// 筛选元素
const statusFilter = document.getElementById('status-filter');
const categoryFilter = document.getElementById('category-filter');
const ratingFilter = document.getElementById('rating-filter');
const clearFiltersBtn = document.getElementById('clear-filters');

// 排序元素
const sortBySelect = document.getElementById('sort-by');
const sortOrderSelect = document.getElementById('sort-order');

// 页面初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeEventListeners();
    loadSortPreferences();
    loadBooksData();
    loadStatistics();
});

// 初始化事件监听器
function initializeEventListeners() {
    // 侧边栏切换
    bubble.addEventListener('click', toggleSidebar);
    
    // 搜索功能
    searchBtn.addEventListener('click', handleSearch);
    searchInput.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            handleSearch();
        }
    });
    
    // 添加书籍
    addBookBtn.addEventListener('click', () => showModal(addBookModal));
    document.getElementById('add-first-book').addEventListener('click', () => showModal(addBookModal));
    addBookForm.addEventListener('submit', handleAddBook);
    
    // URL导入
    importUrlBtn.addEventListener('click', () => showModal(importUrlModal));
    document.getElementById('parse-url-btn').addEventListener('click', parseBookUrl);
    document.getElementById('confirm-import-btn').addEventListener('click', confirmImport);
    
    // 筛选功能
    statusFilter.addEventListener('change', handleFilterChange);
    categoryFilter.addEventListener('change', handleFilterChange);
    ratingFilter.addEventListener('change', handleFilterChange);
    clearFiltersBtn.addEventListener('click', clearFilters);
    
    // 排序功能
    sortBySelect.addEventListener('change', handleSortChange);
    sortOrderSelect.addEventListener('change', handleSortChange);
    
    // 模态框关闭
    document.querySelectorAll('.modal-close, [data-dismiss="modal"]').forEach(btn => {
        btn.addEventListener('click', function() {
            const modal = this.closest('.modal');
            hideModal(modal);
        });
    });
    
    // 点击模态框外部关闭
    document.querySelectorAll('.modal').forEach(modal => {
        modal.addEventListener('click', function(e) {
            if (e.target === this) {
                hideModal(this);
            }
        });
    });
}

// 侧边栏切换
function toggleSidebar() {
    sidebar.classList.toggle('hide-sidebar');
}

// 显示Toast通知
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span class="toast-message">${message}</span>`;
    toastContainer.appendChild(toast);
    
    setTimeout(() => {
        toast.remove();
    }, 4000);
}

// 模态框操作
function showModal(modal) {
    modal.classList.remove('hide');
}

function hideModal(modal) {
    modal.classList.add('hide');
    
    // 清理表单
    const form = modal.querySelector('form');
    if (form) {
        form.reset();
    }
    
    // 清理导入预览
    const preview = modal.querySelector('#import-preview');
    if (preview) {
        preview.classList.add('hide');
    }
}

// 加载书籍数据
async function loadBooksData() {
    try {
        showToast('正在加载书籍数据...', 'info');
        
        // 构建API URL，包含排序参数
        const params = new URLSearchParams();
        params.append('sort_by', currentSort.sortBy);
        params.append('sort_order', currentSort.sortOrder);
        
        const response = await fetch(`/api/books?${params.toString()}`);
        if (!response.ok) {
            throw new Error('获取书籍数据失败');
        }
        
        const data = await response.json();
        allBooks = data.books || [];
        filteredBooks = [...allBooks];
        
        updateCategoryFilter();
        applyFilters(); // 应用当前筛选条件
        updateEmptyState();
        
    } catch (error) {
        console.error('加载书籍数据失败:', error);
        showToast('加载书籍数据失败: ' + error.message, 'error');
    }
}

// 加载统计数据
async function loadStatistics() {
    try {
        const response = await fetch('/api/reading-statistics');
        if (!response.ok) {
            throw new Error('获取统计数据失败');
        }
        
        const stats = await response.json();
        updateStatisticsDisplay(stats);
        
    } catch (error) {
        console.error('加载统计数据失败:', error);
    }
}

// 更新统计显示
function updateStatisticsDisplay(stats) {
    document.getElementById('total-books').textContent = stats.total_books || 0;
    document.getElementById('reading-books').textContent = stats.reading_books || 0;
    document.getElementById('finished-books').textContent = stats.finished_books || 0;
    document.getElementById('total-pages').textContent = stats.total_pages || 0;
    document.getElementById('total-notes').textContent = stats.total_notes || 0;
}

// 更新分类筛选器
function updateCategoryFilter() {
    const categories = new Set();
    allBooks.forEach(book => {
        if (book.category) {
            book.category.forEach(cat => categories.add(cat));
        }
    });
    
    categoryFilter.innerHTML = '<option value="">全部分类</option>';
    categories.forEach(category => {
        const option = document.createElement('option');
        option.value = category;
        option.textContent = category;
        categoryFilter.appendChild(option);
    });
}

// 渲染书籍列表
function renderBooks() {
    booksGrid.innerHTML = '';
    
    filteredBooks.forEach(book => {
        const bookCard = createBookCard(book);
        booksGrid.appendChild(bookCard);
    });
}

// 创建书籍卡片
function createBookCard(book) {
    const card = document.createElement('div');
    card.className = 'book-card';
    card.setAttribute('data-book-id', book.id);
    
    // 计算阅读进度
    const progress = book.total_pages > 0 ? 
        Math.round((book.current_page || 0) / book.total_pages * 100) : 0;
    
    // 状态显示
    const statusMap = {
        'unstart': { text: '未开始', class: 'status-unstart' },
        'reading': { text: '阅读中', class: 'status-reading' },
        'finished': { text: '已完成', class: 'status-finished' },
        'paused': { text: '暂停', class: 'status-paused' }
    };
    
    const status = statusMap[book.status] || statusMap['unstart'];
    
    // 评分显示
    const rating = book.rating > 0 ? '⭐'.repeat(Math.floor(book.rating)) : '';
    
    card.innerHTML = `
        <div class="book-cover ${book.cover_url ? '' : 'no-image'}">
            ${book.cover_url ? 
                `<img src="${book.cover_url}" alt="${book.title}" onerror="handleBookCoverError(this, '${book.cover_url}')">` : 
                '📚'
            }
        </div>
        <div class="book-title" title="${book.title}">${book.title}</div>
        <div class="book-author">${book.author}</div>
        <div class="book-status ${status.class}">${status.text}</div>
        <div class="book-progress">
            <div class="progress-bar">
                <div class="progress-fill" style="width: ${progress}%"></div>
            </div>
            <div class="progress-text">${progress}%</div>
        </div>
        <div class="book-rating">${rating}</div>
        <div class="book-actions">
            <button class="btn-action btn-edit" onclick="editBookFromCard('${book.id}', event)" title="编辑">✏️</button>
            <button class="btn-action btn-delete" onclick="deleteBookFromCard('${book.id}', event)" title="删除">🗑️</button>
        </div>
    `;
    
    // 点击事件
    card.addEventListener('click', () => openBookDetail(book.id));
    
    return card;
}

// 更新空状态显示
function updateEmptyState() {
    if (filteredBooks.length === 0) {
        booksGrid.classList.add('hide');
        emptyState.classList.remove('hide');
    } else {
        booksGrid.classList.remove('hide');
        emptyState.classList.add('hide');
    }
}

// 搜索功能
function handleSearch() {
    const keyword = searchInput.value.trim().toLowerCase();
    currentFilter.search = keyword;
    applyFilters();
}

// 筛选功能
function handleFilterChange() {
    currentFilter.status = statusFilter.value;
    currentFilter.category = categoryFilter.value;
    currentFilter.rating = ratingFilter.value;
    applyFilters();
}

// 应用筛选
function applyFilters() {
    filteredBooks = allBooks.filter(book => {
        // 搜索筛选
        if (currentFilter.search) {
            const searchLower = currentFilter.search.toLowerCase();
            const matchSearch = 
                book.title.toLowerCase().includes(searchLower) ||
                book.author.toLowerCase().includes(searchLower) ||
                (book.description && book.description.toLowerCase().includes(searchLower));
            if (!matchSearch) return false;
        }
        
        // 状态筛选
        if (currentFilter.status && book.status !== currentFilter.status) {
            return false;
        }
        
        // 分类筛选
        if (currentFilter.category) {
            if (!book.category || !book.category.includes(currentFilter.category)) {
                return false;
            }
        }
        
        // 评分筛选
        if (currentFilter.rating) {
            const ratingThreshold = parseInt(currentFilter.rating);
            if (!book.rating || Math.floor(book.rating) < ratingThreshold) {
                return false;
            }
        }
        
        return true;
    });
    
    renderBooks();
    updateEmptyState();
}

// 清除筛选
function clearFilters() {
    currentFilter = { status: '', category: '', rating: '', search: '' };
    
    statusFilter.value = '';
    categoryFilter.value = '';
    ratingFilter.value = '';
    searchInput.value = '';
    
    // 重置排序为默认值
    currentSort = { sortBy: 'add_time', sortOrder: 'desc' };
    sortBySelect.value = 'add_time';
    sortOrderSelect.value = 'desc';
    saveSortPreferences();
    
    // 重新加载数据以应用排序
    loadBooksData();
    
    showToast('已清除所有筛选和排序条件', 'success');
}

// 添加书籍
async function handleAddBook(e) {
    e.preventDefault();
    
    const formData = new FormData(addBookForm);
    const bookData = {
        title: formData.get('title') || document.getElementById('book-title').value,
        author: formData.get('author') || document.getElementById('book-author').value,
        isbn: document.getElementById('book-isbn').value,
        publisher: document.getElementById('book-publisher').value,
        publish_date: document.getElementById('book-publish-date').value,
        cover_url: document.getElementById('book-cover-url').value,
        description: document.getElementById('book-description').value,
        total_pages: parseInt(document.getElementById('book-total-pages').value) || 0,
        category: document.getElementById('book-category').value.split(',').map(s => s.trim()).filter(s => s),
        tags: document.getElementById('book-tags').value.split(',').map(s => s.trim()).filter(s => s),
        source_url: document.getElementById('book-source-url').value
    };
    
    if (!bookData.title || !bookData.author) {
        showToast('请填写书名和作者', 'error');
        return;
    }
    
    try {
        showToast('正在添加书籍...', 'info');
        
        const response = await fetch('/api/books', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(bookData)
        });
        
        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }
        
        const result = await response.json();
        showToast('书籍添加成功！', 'success');
        hideModal(addBookModal);
        
        // 重新加载数据
        loadBooksData();
        loadStatistics();
        
    } catch (error) {
        console.error('添加书籍失败:', error);
        showToast('添加书籍失败: ' + error.message, 'error');
    }
}

// URL解析功能
async function parseBookUrl() {
    const url = document.getElementById('import-url').value.trim();
    if (!url) {
        showToast('请输入书籍URL', 'error');
        return;
    }
    
    try {
        showToast('正在解析URL...', 'info');
        
        const response = await fetch('/api/parse-book-url', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ url: url })
        });
        
        if (!response.ok) {
            throw new Error('URL解析失败');
        }
        
        const bookData = await response.json();
        displayImportPreview(bookData);
        
        showToast('URL解析成功！', 'success');
        
    } catch (error) {
        console.error('URL解析失败:', error);
        showToast('URL解析失败: ' + error.message, 'error');
    }
}

// 显示导入预览
function displayImportPreview(bookData) {
    const preview = document.getElementById('import-preview');
    const content = document.getElementById('preview-content');
    
    content.innerHTML = `
        <div class="preview-item">
            <strong>书名:</strong> ${bookData.title || '未知'}
        </div>
        <div class="preview-item">
            <strong>作者:</strong> ${bookData.author || '未知'}
        </div>
        <div class="preview-item">
            <strong>出版社:</strong> ${bookData.publisher || '未知'}
        </div>
        <div class="preview-item">
            <strong>ISBN:</strong> ${bookData.isbn || '未知'}
        </div>
        <div class="preview-item">
            <strong>简介:</strong> ${bookData.description ? bookData.description.substring(0, 100) + '...' : '无'}
        </div>
        ${bookData.cover_url ? `<div class="preview-item"><img src="${bookData.cover_url}" style="max-width: 100px; max-height: 150px; border-radius: 4px;" onerror="this.style.display='none'; this.parentElement.innerHTML='<div style=\\"padding: 10px; background: #f0f0f0; border-radius: 4px; color: #666; font-size: 12px;\\">📚 图片加载失败</div>'"></div>` : ''}
    `;
    
    preview.classList.remove('hide');
    document.getElementById('parse-url-btn').classList.add('hide');
    document.getElementById('confirm-import-btn').classList.remove('hide');
    
    // 保存解析的数据
    window.parsedBookData = bookData;
}

// 确认导入
async function confirmImport() {
    if (!window.parsedBookData) {
        showToast('没有可导入的数据', 'error');
        return;
    }
    
    try {
        showToast('正在导入书籍...', 'info');
        
        const response = await fetch('/api/books', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(window.parsedBookData)
        });
        
        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }
        
        showToast('书籍导入成功！', 'success');
        hideModal(importUrlModal);
        
        // 重新加载数据
        loadBooksData();
        loadStatistics();
        
        // 清理临时数据
        window.parsedBookData = null;
        
    } catch (error) {
        console.error('导入书籍失败:', error);
        showToast('导入书籍失败: ' + error.message, 'error');
    }
}

// 打开书籍详情
function openBookDetail(bookId) {
    // 跳转到书籍详情页面
    window.location.href = `/reading/book/${bookId}`;
}

// 处理书籍封面图片加载错误
function handleBookCoverError(img, originalUrl) {
    try {
        console.warn('书籍封面图片加载失败:', originalUrl);
        
        if (!img || !img.parentElement) {
            console.error('图片元素或其父容器不存在');
            return;
        }
        
        const container = img.parentElement;
        
        // 创建带样式的默认图标
        const placeholderDiv = document.createElement('div');
        placeholderDiv.style.cssText = `
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            height: 100%;
            background: linear-gradient(135deg, #e76f51, #f4a261);
            color: white;
            font-size: 36px;
            border-radius: 8px;
            flex-direction: column;
            gap: 4px;
        `;
        
        const icon = document.createElement('div');
        icon.style.fontSize = '36px';
        icon.textContent = '📚';
        
        const text = document.createElement('div');
        text.style.cssText = `
            font-size: 10px;
            color: rgba(255, 255, 255, 0.8);
            text-align: center;
            font-weight: 500;
        `;
        text.textContent = '图片加载失败';
        
        placeholderDiv.appendChild(icon);
        placeholderDiv.appendChild(text);
        
        // 清空容器并添加占位符
        container.innerHTML = '';
        container.appendChild(placeholderDiv);
        
    } catch (error) {
        console.error('handleBookCoverError 发生错误:', error);
        // 简单的备用处理
        if (img && img.parentElement) {
            img.parentElement.innerHTML = '<div style="display: flex; align-items: center; justify-content: center; width: 100%; height: 100%; background: #e76f51; color: white; font-size: 36px; border-radius: 8px;">📚</div>';
        }
    }
}

// 工具函数：格式化日期
function formatDate(dateString) {
    if (!dateString) return '';
    const date = new Date(dateString);
    return date.toLocaleDateString('zh-CN');
}

// 工具函数：截断文本
function truncateText(text, maxLength) {
    if (!text) return '';
    return text.length > maxLength ? text.substring(0, maxLength) + '...' : text;
}

// 从卡片编辑书籍
function editBookFromCard(bookId, event) {
    event.stopPropagation(); // 阻止事件冒泡
    
    // 跳转到书籍详情页面，并加上编辑参数
    window.location.href = `/reading/book/${bookId}?edit=true`;
}

// 从卡片删除书籍
async function deleteBookFromCard(bookId, event) {
    event.stopPropagation(); // 阻止事件冒泡
    
    const book = allBooks.find(b => b.id === bookId);
    if (!book) {
        showToast('书籍不存在', 'error');
        return;
    }
    
    if (!confirm(`确定要删除《${book.title}》吗？此操作不可恢复，将同时删除所有相关的笔记和心得。`)) {
        return;
    }
    
    try {
        showToast('正在删除书籍...', 'info');
        
        const response = await fetch(`/api/books?book_id=${bookId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            throw new Error('删除书籍失败');
        }
        
        showToast('书籍已成功删除！', 'success');
        
        // 重新加载数据
        loadBooksData();
        loadStatistics();
        
    } catch (error) {
        console.error('删除书籍失败:', error);
        showToast('删除书籍失败: ' + error.message, 'error');
    }
}

// 排序功能相关函数

// 处理排序变化
function handleSortChange() {
    currentSort.sortBy = sortBySelect.value;
    currentSort.sortOrder = sortOrderSelect.value;
    
    // 保存排序偏好
    saveSortPreferences();
    
    // 重新加载数据
    loadBooksData();
    
    showToast(`已按${getSortDisplayName()}排序`, 'success');
}

// 获取排序显示名称
function getSortDisplayName() {
    const sortNames = {
        'add_time': '添加时间',
        'title': '书名',
        'author': '作者', 
        'rating': '评分',
        'progress': '阅读进度',
        'status': '阅读状态',
        'pages': '总页数'
    };
    
    const orderNames = {
        'desc': '降序',
        'asc': '升序'
    };
    
    return `${sortNames[currentSort.sortBy] || '添加时间'} ${orderNames[currentSort.sortOrder] || '降序'}`;
}

// 保存排序偏好到localStorage
function saveSortPreferences() {
    try {
        localStorage.setItem('reading_sort_preferences', JSON.stringify(currentSort));
    } catch (error) {
        console.warn('无法保存排序偏好:', error);
    }
}

// 从localStorage加载排序偏好
function loadSortPreferences() {
    try {
        const saved = localStorage.getItem('reading_sort_preferences');
        if (saved) {
            const preferences = JSON.parse(saved);
            currentSort.sortBy = preferences.sortBy || 'add_time';
            currentSort.sortOrder = preferences.sortOrder || 'desc';
            
            // 更新UI选择器
            if (sortBySelect) sortBySelect.value = currentSort.sortBy;
            if (sortOrderSelect) sortOrderSelect.value = currentSort.sortOrder;
        }
    } catch (error) {
        console.warn('无法加载排序偏好:', error);
        // 使用默认值
        currentSort = { sortBy: 'add_time', sortOrder: 'desc' };
    }
} 