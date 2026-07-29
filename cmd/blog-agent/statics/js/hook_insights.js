(function () {
    'use strict';

    var featureFilter = document.getElementById('featureFilter');
    var eventFilter = document.getElementById('eventFilter');
    var statusFilter = document.getElementById('statusFilter');
    var updated = document.getElementById('insightsUpdated');
    var refresh = document.getElementById('insightsRefresh');
    var clear = document.getElementById('clearFilters');
    var loading = false;

    var featureLabels = {
        content_workspace: '内容工作台',
        content_library: 'BLOG',
        blog_reader: '博客阅读',
        blog_editor: '博客编辑',
        blog_search: '普通搜索',
        fts_search: 'FTS 搜索',
        pi_agent: 'PI 问答',
        article_assistant: '文章阅读助手',
        goal_management: '目标管理',
        exercise_management: '锻炼管理',
        reading_management: '读书管理',
        large_blog_reader: '大文档阅读',
        image_upload: '图片上传',
        hook_insights: '使用洞察',
        tools: '实用工具',
        help: '帮助页面'
    };

    var eventLabels = {
        'blog.created': '创建博客',
        'diary.written': '写入日记',
        'diary.deleted': '删除日记',
        'page.opened': '打开页面',
        'feature.used': '使用功能',
        'ai.asked': '向 PI 提问',
        'ai.answered': 'PI 完成回答',
        'ai.accepted': '采用 PI 内容',
        'ai.summarized': 'PI 生成总结'
    };

    function node(tag, className, text) {
        var element = document.createElement(tag);
        if (className) element.className = className;
        if (text !== undefined) element.textContent = text;
        return element;
    }

    function featureName(value) {
        return featureLabels[value] || value || '未分类';
    }

    function eventName(value) {
        return eventLabels[value] || value || '未知事件';
    }

    function statusName(value) {
        if (value === 'success') return '成功';
        if (value === 'error') return '失败';
        return '未标记';
    }

    function populateSelect(select, values, labeler) {
        var selected = select.value;
        while (select.options.length > 1) select.remove(1);
        values.forEach(function (value) {
            var option = document.createElement('option');
            option.value = value;
            option.textContent = labeler(value);
            select.appendChild(option);
        });
        select.value = selected;
    }

    function renderSummary(summary) {
        document.getElementById('todayEvents').textContent = summary.today_events;
        document.getElementById('totalEvents').textContent = summary.total_events;
        document.getElementById('activeDays').textContent = summary.active_days + ' 天';
        document.getElementById('successRate').textContent =
            summary.success_rate < 0 ? '暂无' : summary.success_rate.toFixed(0) + '%';
    }

    function recentDates(days) {
        var values = [];
        var now = new Date();
        now.setHours(12, 0, 0, 0);
        for (var offset = days - 1; offset >= 0; offset--) {
            var date = new Date(now);
            date.setDate(now.getDate() - offset);
            values.push(
                date.getFullYear() + '-' +
                String(date.getMonth() + 1).padStart(2, '0') + '-' +
                String(date.getDate()).padStart(2, '0')
            );
        }
        return values;
    }

    function renderHeatmap(cells, days) {
        var target = document.getElementById('hookHeatmap');
        target.textContent = '';
        target.appendChild(node('span', 'heat-hour', '日期'));
        for (var hour = 0; hour < 24; hour++) {
            target.appendChild(node('span', 'heat-hour', String(hour).padStart(2, '0')));
        }
        var counts = {};
        var maximum = 0;
        cells.forEach(function (cell) {
            counts[cell.date + '-' + cell.hour] = cell.count;
            maximum = Math.max(maximum, cell.count);
        });
        recentDates(days).forEach(function (date) {
            var dateValue = new Date(date + 'T12:00:00');
            var dayLabel = (dateValue.getMonth() + 1) + '/' + dateValue.getDate() + ' 周' + '日一二三四五六'[dateValue.getDay()];
            target.appendChild(node('span', 'heat-day', dayLabel));
            for (var hour = 0; hour < 24; hour++) {
                var count = counts[date + '-' + hour] || 0;
                var level = count === 0 ? 0 : Math.max(1, Math.ceil(count / Math.max(1, maximum) * 4));
                var cell = node('span', 'heat-cell');
                cell.dataset.level = level;
                cell.setAttribute('aria-label', date + ' ' + String(hour).padStart(2, '0') + ':00 · ' + count + ' 次');
                target.appendChild(cell);
            }
        });
    }

    function renderTimeline(items) {
        var target = document.getElementById('hookTimeline');
        target.textContent = '';
        document.getElementById('timelineCount').textContent = items.length;
        if (!items.length) {
            target.appendChild(node('div', 'empty-state', '今天还没有符合筛选条件的行为。'));
            return;
        }
        items.forEach(function (item) {
            var row = node('article', 'timeline-item ' + item.status);
            row.appendChild(node('time', 'timeline-time', item.created_at.slice(11, 16)));
            var content = node('div', 'timeline-content');
            content.appendChild(node('strong', '', eventName(item.event_type) + ' · ' + featureName(item.feature)));
            content.appendChild(node('span', '', item.title || item.query || item.object_id || '没有附加对象'));
            row.appendChild(content);
            row.appendChild(node('span', 'timeline-status ' + item.status, statusName(item.status)));
            target.appendChild(row);
        });
    }

    function renderFeatures(items) {
        var target = document.getElementById('featureRanking');
        target.textContent = '';
        if (!items.length) {
            target.appendChild(node('div', 'empty-state', '暂无功能使用数据。'));
            return;
        }
        var maximum = items[0].count || 1;
        items.forEach(function (item) {
            var row = node('div', 'feature-row');
            var head = node('div', 'feature-row-head');
            head.appendChild(node('span', '', featureName(item.feature)));
            head.appendChild(node('strong', '', item.count));
            var track = node('div', 'feature-track');
            var fill = node('div', 'feature-fill');
            fill.style.width = Math.max(4, item.count / maximum * 100) + '%';
            track.appendChild(fill);
            row.appendChild(head);
            row.appendChild(track);
            target.appendChild(row);
        });
    }

    function renderPaths(items) {
        var target = document.getElementById('commonPaths');
        target.textContent = '';
        if (!items.length) {
            target.appendChild(node('div', 'empty-state', '还没有形成可识别的连续操作路径。'));
            return;
        }
        items.forEach(function (item) {
            var row = node('div', 'path-row');
            var steps = node('div', 'path-steps');
            item.steps.forEach(function (step, index) {
                if (index > 0) steps.appendChild(node('span', 'path-arrow', '→'));
                steps.appendChild(node('span', 'path-step', featureName(step)));
            });
            row.appendChild(steps);
            row.appendChild(node('span', 'path-count', '出现 ' + item.count + ' 次'));
            target.appendChild(row);
        });
    }

    function loadInsights() {
        if (loading) return;
        loading = true;
        refresh.disabled = true;
        updated.textContent = '正在聚合 Hook…';
        var params = new URLSearchParams({ days: '7' });
        if (featureFilter.value) params.set('feature', featureFilter.value);
        if (eventFilter.value) params.set('event', eventFilter.value);
        if (statusFilter.value) params.set('status', statusFilter.value);
        fetch('/api/hooks/insights?' + params.toString(), { credentials: 'same-origin' })
            .then(function (response) {
                if (!response.ok) throw new Error('HTTP ' + response.status);
                return response.json();
            })
            .then(function (data) {
                populateSelect(featureFilter, data.available_features || [], featureName);
                populateSelect(eventFilter, data.available_events || [], eventName);
                document.getElementById('periodDays').textContent = data.days;
                renderSummary(data.summary);
                renderHeatmap(data.heatmap || [], data.days);
                renderTimeline(data.timeline || []);
                renderFeatures(data.features || []);
                renderPaths(data.paths || []);
                updated.textContent = '更新于 ' + data.generated_at;
            })
            .catch(function (error) {
                updated.textContent = '读取失败：' + error.message;
            })
            .finally(function () {
                loading = false;
                refresh.disabled = false;
            });
    }

    [featureFilter, eventFilter, statusFilter].forEach(function (select) {
        select.addEventListener('change', loadInsights);
    });
    clear.addEventListener('click', function () {
        featureFilter.value = '';
        eventFilter.value = '';
        statusFilter.value = '';
        loadInsights();
    });
    refresh.addEventListener('click', loadInsights);
    loadInsights();
})();
