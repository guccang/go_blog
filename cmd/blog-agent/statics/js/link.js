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
	var blogAskForm = document.getElementById('blogAskForm');
	if (blogAskForm) {
		var blogAskInput = document.getElementById('blogAskInput');
		var blogAskStatus = document.getElementById('blogAskStatus');
		var blogAskResults = document.getElementById('blogAskResults');
		var blogAskToolbar = document.getElementById('blogAskToolbar');
		var blogAskActions = document.getElementById('blogAskActions');
		var blogAskShowAll = document.getElementById('blogAskShowAll');
		var blogAskCollapse = document.getElementById('blogAskCollapse');
		var blogAskPI = document.getElementById('blogAskPI');
		var blogPIAnswer = document.getElementById('blogPIAnswer');
		var blogAskQuery = '';
		var blogAskCollapsed = false;
		function setBlogAskCollapsed(collapsed) {
			blogAskCollapsed = collapsed;
			blogAskResults.hidden = collapsed;
			blogPIAnswer.hidden = collapsed || blogPIAnswer.dataset.hasAnswer !== 'true';
			blogAskActions.hidden = collapsed || blogAskShowAll.hidden;
			blogAskCollapse.textContent = collapsed ? '展开查询结果' : '收起查询结果';
			blogAskCollapse.setAttribute('aria-expanded', String(!collapsed));
			if (collapsed) blogAskStatus.textContent = '查询结果已收起。';
		}
		function renderBlogAskResults(payload, showingAll) {
			var items = payload.items || [];
			blogAskResults.textContent = '';
			if (!items.length) {
				blogAskStatus.textContent = '没有找到匹配的非加密、非日记内容。';
				blogAskToolbar.hidden = true;
				blogAskActions.hidden = true;
				blogAskShowAll.hidden = true;
				return;
			}
			blogAskStatus.textContent = showingAll ? '共找到 ' + items.length + ' 篇相关笔记' : '展示前 ' + items.length + ' 篇相关笔记';
			items.forEach(function(item) {
				var link = document.createElement('a');
				link.className = 'blog-ask-result';
				link.href = item.url;
				var title = document.createElement('strong');
				title.textContent = item.title;
				var snippet = document.createElement('p');
				appendHighlightedSnippet(snippet, item.snippet);
				link.appendChild(title);
				link.appendChild(snippet);
				blogAskResults.appendChild(link);
			});
			blogAskResults.hidden = false;
			blogPIAnswer.hidden = blogPIAnswer.dataset.hasAnswer !== 'true';
			blogAskToolbar.hidden = false;
			blogAskActions.hidden = showingAll || !payload.has_more;
			blogAskShowAll.hidden = showingAll || !payload.has_more;
			blogAskCollapsed = false;
			blogAskCollapse.textContent = '收起查询结果';
			blogAskCollapse.setAttribute('aria-expanded', 'true');
		}
		function searchBlogAsk(query, all) {
			blogAskStatus.textContent = all ? '正在加载全部搜索结果…' : '正在检索你的博客…';
			blogAskResults.hidden = true;
			blogAskToolbar.hidden = true;
			blogAskActions.hidden = true;
			blogAskShowAll.hidden = true;
			var suffix = all ? '&all=1' : '';
			fetch('/api/blogs/fts?q=' + encodeURIComponent(query) + suffix, { credentials: 'same-origin' })
				.then(function(response) { if (!response.ok) throw new Error('search failed'); return response.json(); })
				.then(function(payload) { renderBlogAskResults(payload, all); })
				.catch(function() { blogAskStatus.textContent = '检索失败，请稍后重试。'; });
		}
		blogAskForm.addEventListener('submit', function(event) {
			event.preventDefault();
			var query = (blogAskInput.value || '').trim();
			if (!query) return;
			blogAskQuery = query;
			searchBlogAsk(query, false);
		});
		blogAskPI.addEventListener('click', function() {
			var query = (blogAskInput.value || '').trim();
			if (!query) return;
			blogAskStatus.textContent = 'PI 正在检索并生成回答…';
			blogPIAnswer.hidden = true;
			fetch('/api/pi/ask', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ question: query }) })
				.then(function(response) {
					if (!response.ok) return response.text().then(function(message) { throw new Error(message); });
					return response.json();
				})
				.then(function(payload) {
					blogPIAnswer.textContent = '';
					if (payload.brief) {
						var brief = document.createElement('div');
						brief.className = 'blog-pi-brief';
						brief.textContent = '初步回答（300字内）\n' + payload.brief;
						blogPIAnswer.appendChild(brief);
					}
					var summary = document.createElement('div');
					summary.className = 'blog-pi-summary';
					summary.textContent = '站内资料总结\n' + payload.text;
					blogPIAnswer.appendChild(summary);
					if (payload.advice) {
						var advice = document.createElement('div');
						advice.className = 'blog-pi-advice';
						advice.textContent = '意图探索与建议\n' + payload.advice;
						blogPIAnswer.appendChild(advice);
					}
					if (payload.sources && payload.sources.length) {
						var sourceBlock = document.createElement('div');
						sourceBlock.className = 'blog-pi-sources';
						var sourceLabel = document.createElement('strong');
						sourceLabel.textContent = '参考博客：';
						sourceBlock.appendChild(sourceLabel);
						payload.sources.forEach(function(source, index) {
							var link = document.createElement('a');
							link.href = '/get?blogname=' + encodeURIComponent(source);
							link.textContent = source;
							sourceBlock.appendChild(link);
							if (index < payload.sources.length - 1) sourceBlock.appendChild(document.createTextNode('、'));
						});
						blogPIAnswer.appendChild(sourceBlock);
					}
					blogPIAnswer.dataset.hasAnswer = 'true';
					blogPIAnswer.hidden = blogAskCollapsed;
					blogAskToolbar.hidden = false;
					blogAskCollapse.textContent = blogAskCollapsed ? '展开查询结果' : '收起查询结果';
					blogAskCollapse.setAttribute('aria-expanded', String(!blogAskCollapsed));
					blogAskStatus.textContent = blogAskCollapsed
						? 'PI 回答已生成，查询结果仍处于收起状态。'
						: 'PI 已通过 ' + payload.provider + ' / ' + payload.model + ' 回答。' + (payload.usage && payload.usage.reported ? ' 本次：上传 ' + payload.usage.prompt_tokens + '，下载 ' + payload.usage.completion_tokens + '，合计 ' + payload.usage.total_tokens + ' Token；' + (payload.duration_ms / 1000).toFixed(1) + ' 秒。' : ' Provider 未返回 Token 用量。');
				})
				.catch(function(error) { blogAskStatus.textContent = 'PI 回答失败：' + (error.message || '请检查 Provider 配置。'); });
		});
		blogAskShowAll.addEventListener('click', function() {
			if (blogAskQuery) searchBlogAsk(blogAskQuery, true);
		});
		blogAskCollapse.addEventListener('click', function() {
			setBlogAskCollapsed(!blogAskCollapsed);
		});
	}

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
        { key: 'tech',     label: 'blog实现技术文档' },
        { key: 'system',   label: '系统' },
    ];

    function classifyCard(card) {
        var title = (card.getAttribute('data-title') || '').toLowerCase();
        var isDiary = card.getAttribute('data-diary') === 'true';
        if (card.getAttribute('data-tech-doc') === 'true') return 'tech';
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
                        card.dataset.techDoc = item.tech_doc ? 'true' : 'false';
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

function appendHighlightedSnippet(target, snippet) {
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

// ===== History Back =====
function PageHistoryBack() {
    // handled by utils.js if needed
}
