// 统计页面 JavaScript

let statisticsData = null;
let refreshTimer = null;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializePage();
    loadStatistics();
    
    // 设置定期刷新（每5分钟）
    refreshTimer = setInterval(loadStatistics, 5 * 60 * 1000);
});

// 页面销毁时清理定时器
window.addEventListener('beforeunload', function() {
    if (refreshTimer) {
        clearInterval(refreshTimer);
    }
});

// 初始化页面
function initializePage() {
    // 侧边栏功能
    const bubble = document.getElementById('bubble');
    const sidebarContainer = document.getElementById('sidebar-container');
    
    if (bubble && sidebarContainer) {
        bubble.addEventListener('click', function() {
            sidebarContainer.classList.toggle('show');
        });
        
        // 点击页面其他地方关闭侧边栏
        document.addEventListener('click', function(e) {
            if (!sidebarContainer.contains(e.target) && !bubble.contains(e.target)) {
                sidebarContainer.classList.remove('show');
            }
        });
    }
    
    // 设置更新时间
    updateTimestamp();
}

// 加载统计数据
async function loadStatistics() {
    showLoading(true);
    
    try {
        const response = await fetch('/api/statistics', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
            }
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        statisticsData = await response.json();
        renderStatistics(statisticsData);
        updateTimestamp();
        showToast('统计数据已更新', 'success');
        
    } catch (error) {
        console.error('加载统计数据失败:', error);
        showToast('加载统计数据失败: ' + error.message, 'error');
        
    } finally {
        showLoading(false);
    }
}

// 渲染统计数据
function renderStatistics(data) {
    if (!data) return;
    
    // 渲染概览卡片
    renderOverviewCards(data);
    
    // 渲染各个统计区域
    renderBlogStats(data.blog_stats);
    renderAccessStats(data.access_stats);
    renderEditStats(data.edit_stats);
    renderUserStats(data.user_stats);
    renderIPStats(data.ip_stats);
    renderCommentStats(data.comment_stats);
    renderTagStats(data.tag_stats);
    renderTimeAnalysis(data.time_analysis);
    renderContentStats(data.content_stats);
    renderSystemStats(data.system_stats);
    
    // 显示所有统计区域
    showAllStatsSections();
}

// 渲染概览卡片
function renderOverviewCards(data) {
    setElementText('total-blogs', data.blog_stats.total_blogs);
    setElementText('total-access', formatNumber(data.access_stats.total_access));
    setElementText('total-edits', formatNumber(data.edit_stats.total_edits));
    setElementText('total-comments', data.comment_stats.total_comments);
    setElementText('total-logins', formatNumber(data.user_stats.total_logins));
    setElementText('unique-visitors', data.ip_stats.unique_visitors);
    setElementText('total-tags', data.tag_stats.total_tags);
    setElementText('cooperation-users', data.cooperation_stats.cooperation_users);
}

// 渲染博客统计
function renderBlogStats(blogStats) {
    setElementText('public-blogs', blogStats.public_blogs);
    setElementText('private-blogs', blogStats.private_blogs);
    setElementText('encrypt-blogs', blogStats.encrypt_blogs);
    setElementText('cooperation-blogs', blogStats.cooperation_blogs);
    setElementText('today-new-blogs', blogStats.today_new_blogs);
    setElementText('week-new-blogs', blogStats.week_new_blogs);
    setElementText('month-new-blogs', blogStats.month_new_blogs);
}

// 渲染访问统计
function renderAccessStats(accessStats) {
    setElementText('today-access', formatNumber(accessStats.today_access));
    setElementText('week-access', formatNumber(accessStats.week_access));
    setElementText('month-access', formatNumber(accessStats.month_access));
    setElementText('average-access', formatDecimal(accessStats.average_access));
    setElementText('zero-access-blogs', accessStats.zero_access_blogs);
    
    // 渲染热门博客排行
    renderRankingList('top-accessed-blogs', accessStats.top_accessed_blogs, 'access_num', '次');
    
    // 渲染最近访问博客
    renderRecentList('recent-access-blogs', accessStats.recent_access_blogs, 'access_time');
}

// 渲染编辑统计
function renderEditStats(editStats) {
    setElementText('today-edits', formatNumber(editStats.today_edits));
    setElementText('week-edits', formatNumber(editStats.week_edits));
    setElementText('month-edits', formatNumber(editStats.month_edits));
    setElementText('average-edits', formatDecimal(editStats.average_edits));
    setElementText('never-edited-blogs', editStats.never_edited_blogs);
    
    // 渲染最常修改博客
    renderRankingList('top-edited-blogs', editStats.top_edited_blogs, 'modify_num', '次');
    
    // 渲染最近修改博客
    renderRecentList('recent-edited-blogs', editStats.recent_edited_blogs, 'modify_time');
}

// 渲染用户统计
function renderUserStats(userStats) {
    setElementText('today-logins', formatNumber(userStats.today_logins));
    setElementText('week-logins', formatNumber(userStats.week_logins));
    setElementText('month-logins', formatNumber(userStats.month_logins));
    setElementText('average-daily-logins', formatDecimal(userStats.average_daily_logins));
    setElementText('last-login-time', userStats.last_login_time || '暂无');
}

// 渲染IP统计
function renderIPStats(ipStats) {
    setElementText('unique-visitors-detail', ipStats.unique_visitors);
    setElementText('today-unique-visitors', ipStats.today_unique_visitors);
    
    // 渲染最活跃IP
    renderIPRankingList('top-active-ips', ipStats.top_active_ips);
    
    // 渲染最近访问IP
    renderIPRecentList('recent-access-ips', ipStats.recent_access_ips);
}

// 渲染评论统计
function renderCommentStats(commentStats) {
    setElementText('blogs-with-comments', commentStats.blogs_with_comments);
    setElementText('today-new-comments', commentStats.today_new_comments);
    setElementText('week-new-comments', commentStats.week_new_comments);
    setElementText('month-new-comments', commentStats.month_new_comments);
    setElementText('average-comments', formatDecimal(commentStats.average_comments));
    
    // 渲染最多评论博客
    renderRankingList('top-commented-blogs', commentStats.top_commented_blogs, 'comment_count', '条');
    
    // 渲染活跃评论用户
    renderCommentUserList('active-comment-users', commentStats.active_comment_users);
    
    // 渲染最新评论
    renderRecentComments('recent-comments', commentStats.recent_comments);
}

// 渲染标签统计
function renderTagStats(tagStats) {
    setElementText('public-tags', tagStats.public_tags);
    
    // 渲染热门标签云
    renderTagCloud('hot-tags', tagStats.hot_tags);
    
    // 渲染最近使用标签
    renderTagList('recent-used-tags', tagStats.recent_used_tags);
}

// 渲染时间分析
function renderTimeAnalysis(timeAnalysis) {
    // 渲染创建时间分布图表（简化显示）
    renderSimpleChart('creation-time-chart', '博客创建时间分布图');
    
    // 渲染访问时段分布图表
    renderSimpleChart('access-hour-chart', '访问时段分布图');
    
    // 渲染活跃时段排行
    renderTimeSlotList('active-time-slots', timeAnalysis.active_time_slots);
}

// 渲染内容统计
function renderContentStats(contentStats) {
    setElementText('total-characters', formatNumber(contentStats.total_characters));
    setElementText('average-article-length', formatDecimal(contentStats.average_article_length));
    setElementText('empty-content-blogs', contentStats.empty_content_blogs);
    
    // 渲染最长和最短文章
    setElementText('longest-title', contentStats.longest_article.title || '暂无');
    setElementText('longest-length', formatNumber(contentStats.longest_article.length));
    setElementText('shortest-title', contentStats.shortest_article.title || '暂无');
    setElementText('shortest-length', formatNumber(contentStats.shortest_article.length));
}

// 渲染系统统计
function renderSystemStats(systemStats) {
    setElementText('system-uptime', systemStats.system_uptime);
    setElementText('data-size', systemStats.data_size);
    setElementText('static-files', systemStats.static_files);
    setElementText('template-files', systemStats.template_files);
    setElementText('today-operations', systemStats.today_operations);
}

// 渲染排行榜列表
function renderRankingList(containerId, items, valueKey, unit = '') {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item, index) => {
        const rankClass = index === 0 ? 'top1' : index === 1 ? 'top2' : index === 2 ? 'top3' : '';
        html += `
            <div class="ranking-item">
                <div class="ranking-number ${rankClass}">${index + 1}</div>
                <div class="ranking-title" title="${item.title}">${item.title}</div>
                <div class="ranking-value">${item[valueKey]}${unit}</div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染最近列表
function renderRecentList(containerId, items, timeKey) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item) => {
        html += `
            <div class="ranking-item">
                <div class="ranking-title" title="${item.title}">${item.title}</div>
                <div class="ranking-value">${formatTime(item[timeKey])}</div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染IP排行榜
function renderIPRankingList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item, index) => {
        const rankClass = index === 0 ? 'top1' : index === 1 ? 'top2' : index === 2 ? 'top3' : '';
        html += `
            <div class="ranking-item">
                <div class="ranking-number ${rankClass}">${index + 1}</div>
                <div class="ranking-title">${item.ip}</div>
                <div class="ranking-value">${item.access_count}次</div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染最近访问IP
function renderIPRecentList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item) => {
        html += `
            <div class="ranking-item">
                <div class="ranking-title">${item.ip}</div>
                <div class="ranking-value">${formatTime(item.last_access)}</div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染评论用户列表
function renderCommentUserList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item, index) => {
        const rankClass = index === 0 ? 'top1' : index === 1 ? 'top2' : index === 2 ? 'top3' : '';
        html += `
            <div class="ranking-item">
                <div class="ranking-number ${rankClass}">${index + 1}</div>
                <div class="ranking-title">${item.owner}</div>
                <div class="ranking-value">${item.comment_count}条</div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染最新评论
function renderRecentComments(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item) => {
        html += `
            <div class="comment-item">
                <div class="comment-blog">${item.blog_title}</div>
                <div class="comment-content">${truncateText(item.msg, 100)}</div>
                <div class="comment-meta">
                    ${item.owner} · ${formatTime(item.create_time)}
                </div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染标签云
function renderTagCloud(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无标签</div>';
        return;
    }
    
    let html = '';
    items.forEach((item, index) => {
        let className = 'tag-item';
        if (index < 3) className += ' hot';
        else if (index < 8) className += ' medium';
        
        html += `
            <span class="${className}" data-tooltip="使用 ${item.count} 次">
                ${item.tag}
            </span>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染标签列表
function renderTagList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item) => {
        html += `
            <div class="tag-list-item">
                <span>${item.tag}</span>
                <span>${item.count}次</span>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染时段列表
function renderTimeSlotList(containerId, items) {
    const container = document.getElementById(containerId);
    if (!container || !items) return;
    
    if (items.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无数据</div>';
        return;
    }
    
    let html = '';
    items.forEach((item, index) => {
        const rankClass = index === 0 ? 'top1' : index === 1 ? 'top2' : index === 2 ? 'top3' : '';
        html += `
            <div class="ranking-item">
                <div class="ranking-number ${rankClass}">${index + 1}</div>
                <div class="ranking-title">${item.time_slot}</div>
                <div class="ranking-value">${item.activity}次</div>
            </div>
        `;
    });
    
    container.innerHTML = html;
}

// 渲染简单图表占位符
function renderSimpleChart(containerId, title) {
    const container = document.getElementById(containerId);
    if (!container) return;
    
    container.innerHTML = `<div>📊 ${title}<br><small>(图表功能待实现)</small></div>`;
}

// 显示所有统计区域
function showAllStatsSections() {
    const sections = [
        'overview', 'blog-stats', 'access-stats', 'edit-stats', 
        'user-stats', 'ip-stats', 'comment-stats', 'tag-stats', 
        'time-analysis', 'content-stats', 'system-stats'
    ];
    
    sections.forEach(sectionId => {
        const section = document.getElementById(sectionId);
        if (section) {
            section.classList.remove('hide');
        }
    });
}

// 显示/隐藏加载状态
function showLoading(show) {
    const loading = document.getElementById('loading');
    if (loading) {
        loading.classList.toggle('hide', !show);
    }
}

// 刷新统计数据
function refreshStatistics() {
    const button = document.getElementById('refresh-button');
    if (button) {
        button.classList.add('refreshing');
        button.disabled = true;
    }
    
    loadStatistics().finally(() => {
        if (button) {
            button.classList.remove('refreshing');
            button.disabled = false;
        }
    });
}

// 导出统计数据
function exportStatistics() {
    if (!statisticsData) {
        showToast('暂无数据可导出', 'warning');
        return;
    }
    
    try {
        const dataStr = JSON.stringify(statisticsData, null, 2);
        const dataBlob = new Blob([dataStr], { type: 'application/json' });
        const url = URL.createObjectURL(dataBlob);
        
        const link = document.createElement('a');
        link.href = url;
        link.download = `statistics_${formatDateForFilename(new Date())}.json`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
        
        showToast('统计数据已导出', 'success');
    } catch (error) {
        console.error('导出失败:', error);
        showToast('导出失败', 'error');
    }
}

// 工具函数

// 设置元素文本
function setElementText(id, text) {
    const element = document.getElementById(id);
    if (element) {
        element.textContent = text;
        element.classList.add('highlight-number');
        setTimeout(() => {
            element.classList.remove('highlight-number');
        }, 1000);
    }
}

// 格式化数字
function formatNumber(num) {
    if (num === null || num === undefined) return '0';
    return num.toLocaleString();
}

// 格式化小数
function formatDecimal(num, digits = 2) {
    if (num === null || num === undefined) return '0';
    return Number(num).toFixed(digits);
}

// 格式化时间
function formatTime(timeStr) {
    if (!timeStr) return '暂无';
    try {
        const date = new Date(timeStr);
        return date.toLocaleString();
    } catch (e) {
        return timeStr;
    }
}

// 格式化文件名日期
function formatDateForFilename(date) {
    return date.toISOString().slice(0, 19).replace(/[T:]/g, '_');
}

// 截断文本
function truncateText(text, maxLength) {
    if (!text) return '';
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
}

// 更新时间戳
function updateTimestamp() {
    const timeElement = document.getElementById('update-time');
    if (timeElement) {
        timeElement.textContent = new Date().toLocaleString();
    }
}

// 显示提示消息
function showToast(message, type = 'info') {
    // 使用现有的 toast 功能（如果有的话）
    console.log(`${type.toUpperCase()}: ${message}`);
    
    // 简单的提示实现
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    toast.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 12px 20px;
        background: ${type === 'success' ? '#28a745' : type === 'error' ? '#dc3545' : type === 'warning' ? '#ffc107' : '#17a2b8'};
        color: white;
        border-radius: 4px;
        z-index: 10000;
        animation: slideInRight 0.3s ease-out;
    `;
    
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.style.animation = 'slideOutRight 0.3s ease-out';
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, 300);
    }, 3000);
}

// 生成模拟数据（用于测试）
function generateMockData() {
    return {
        blog_stats: {
            total_blogs: 156,
            public_blogs: 89,
            private_blogs: 45,
            encrypt_blogs: 12,
            cooperation_blogs: 10,
            today_new_blogs: 3,
            week_new_blogs: 8,
            month_new_blogs: 25
        },
        access_stats: {
            total_access: 12567,
            today_access: 89,
            week_access: 456,
            month_access: 1234,
            average_access: 80.56,
            zero_access_blogs: 23,
            top_accessed_blogs: [
                { title: "Go语言入门教程", access_num: 567, access_time: "2024-01-15 14:30:00" },
                { title: "Docker容器化实践", access_num: 445, access_time: "2024-01-15 13:20:00" },
                { title: "微服务架构设计", access_num: 389, access_time: "2024-01-15 12:10:00" }
            ],
            recent_access_blogs: [
                { title: "最新技术动态", access_num: 45, access_time: "2024-01-15 15:30:00" },
                { title: "开发经验分享", access_num: 67, access_time: "2024-01-15 15:25:00" }
            ]
        },
        edit_stats: {
            total_edits: 2345,
            today_edits: 12,
            week_edits: 67,
            month_edits: 234,
            average_edits: 15.03,
            never_edited_blogs: 34,
            top_edited_blogs: [
                { title: "项目开发日志", modify_num: 78, modify_time: "2024-01-15 14:30:00" },
                { title: "学习笔记汇总", modify_num: 56, modify_time: "2024-01-15 13:20:00" }
            ],
            recent_edited_blogs: [
                { title: "最新修改的博客", modify_num: 5, modify_time: "2024-01-15 15:30:00" }
            ]
        },
        user_stats: {
            total_logins: 1234,
            today_logins: 5,
            week_logins: 23,
            month_logins: 89,
            last_login_time: "2024-01-15 15:30:00",
            average_daily_logins: 4.1
        },
        ip_stats: {
            unique_visitors: 456,
            today_unique_visitors: 12,
            top_active_ips: [
                { ip: "192.168.1.1", access_count: 234, last_access: "2024-01-15 15:30:00" },
                { ip: "10.0.0.1", access_count: 189, last_access: "2024-01-15 15:25:00" }
            ],
            recent_access_ips: [
                { ip: "203.0.113.1", access_count: 5, last_access: "2024-01-15 15:30:00" }
            ]
        },
        comment_stats: {
            total_comments: 567,
            blogs_with_comments: 89,
            today_new_comments: 8,
            week_new_comments: 34,
            month_new_comments: 123,
            average_comments: 3.6,
            top_commented_blogs: [
                { title: "热门讨论话题", comment_count: 45 },
                { title: "技术交流分享", comment_count: 32 }
            ],
            active_comment_users: [
                { owner: "张三", comment_count: 67 },
                { owner: "李四", comment_count: 45 }
            ],
            recent_comments: [
                { blog_title: "最新博客", owner: "王五", msg: "写得很好，学到了很多", create_time: "2024-01-15 15:30:00" }
            ]
        },
        tag_stats: {
            total_tags: 89,
            public_tags: 45,
            hot_tags: [
                { tag: "Go", count: 45 },
                { tag: "Docker", count: 32 },
                { tag: "微服务", count: 28 },
                { tag: "前端", count: 23 },
                { tag: "后端", count: 19 }
            ],
            recent_used_tags: [
                { tag: "最新技术", count: 5 },
                { tag: "学习笔记", count: 8 }
            ]
        },
        cooperation_stats: {
            cooperation_users: 5,
            cooperation_blogs: 23,
            active_cooperation_users: ["用户A", "用户B"],
            cooperation_tag_stats: { "协作": 15, "团队": 8 }
        },
        time_analysis: {
            creation_time_distribution: { "2024-01": 15, "2024-02": 23 },
            access_hour_distribution: { 9: 45, 14: 67, 20: 34 },
            edit_time_distribution: { "2024-01": 12, "2024-02": 18 },
            active_time_slots: [
                { time_slot: "14:00-14:59", activity: 67 },
                { time_slot: "09:00-09:59", activity: 45 },
                { time_slot: "20:00-20:59", activity: 34 }
            ]
        },
        content_stats: {
            total_characters: 1234567,
            average_article_length: 2345.67,
            empty_content_blogs: 12,
            longest_article: { title: "超长技术文档", length: 15678 },
            shortest_article: { title: "简短说明", length: 89 }
        },
        system_stats: {
            system_uptime: "7天 12小时 34分钟",
            data_size: "博客: 156, 评论: 567",
            static_files: 127,
            template_files: 19,
            today_operations: 234
        }
    };
}

// 添加CSS动画样式
const style = document.createElement('style');
style.textContent = `
    @keyframes slideInRight {
        from { transform: translateX(100%); opacity: 0; }
        to { transform: translateX(0); opacity: 1; }
    }
    @keyframes slideOutRight {
        from { transform: translateX(0); opacity: 1; }
        to { transform: translateX(100%); opacity: 0; }
    }
`;
document.head.appendChild(style);