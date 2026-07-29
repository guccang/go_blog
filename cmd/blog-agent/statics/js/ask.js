(function () {
    'use strict';

    var page = document.getElementById('askPage');
    var form = document.getElementById('askPageForm');
    var input = document.getElementById('askPageInput');
    var status = document.getElementById('askStatus');
    var answer = document.getElementById('piAnswer');
    var brief = document.getElementById('piBrief');
    var summary = document.getElementById('piSummary');
    var advice = document.getElementById('piAdvice');
    var sources = document.getElementById('piSources');
    var usage = document.getElementById('piUsage');
    var regenerate = document.getElementById('regenerateAnswer');
    var query = (page.dataset.query || '').trim();
    var account = page.dataset.account || '';
    var loading = false;

    function cacheKey(value) {
        return 'guccang:pi-answer:' + account + ':' + value;
    }

    function readCache(value) {
        try {
            var raw = sessionStorage.getItem(cacheKey(value));
            return raw ? JSON.parse(raw) : null;
        } catch (_) {
            return null;
        }
    }

    function writeCache(value, payload) {
        try {
            sessionStorage.setItem(cacheKey(value), JSON.stringify(payload));
        } catch (_) {
            // 浏览器禁用会话存储时仍可正常使用 PI。
        }
    }

    function renderBlock(target, title, text) {
        target.textContent = '';
        if (!text) {
            target.hidden = true;
            return;
        }
        target.hidden = false;
        var heading = document.createElement('h2');
        heading.textContent = title;
        var body = document.createElement('p');
        body.textContent = text;
        target.appendChild(heading);
        target.appendChild(body);
    }

    function render(payload, cached) {
        renderBlock(brief, '初步回答（300 字内）', payload.brief);
        renderBlock(summary, '站内资料总结', payload.text);
        renderBlock(advice, '意图探索与建议', payload.advice);
        sources.textContent = '';
        if (payload.sources && payload.sources.length) {
            var label = document.createElement('strong');
            label.textContent = '参考博客：';
            sources.appendChild(label);
            payload.sources.forEach(function (source, index) {
                if (index) sources.appendChild(document.createTextNode('、'));
                var link = document.createElement('a');
                link.href = '/get?blogname=' + encodeURIComponent(source);
                link.textContent = source;
                sources.appendChild(link);
            });
        }
        var usageText = payload.provider + ' / ' + payload.model;
        if (payload.usage && payload.usage.reported) {
            usageText += ' · 上传 ' + payload.usage.prompt_tokens +
                ' · 下载 ' + payload.usage.completion_tokens +
                ' · 合计 ' + payload.usage.total_tokens + ' Token';
        }
        if (payload.duration_ms) usageText += ' · ' + (payload.duration_ms / 1000).toFixed(1) + ' 秒';
        usage.textContent = usageText;
        answer.hidden = false;
        regenerate.hidden = false;
        status.textContent = cached ? '已恢复本次会话中的回答。' : 'PI 回答完成。';
    }

    function generate(force) {
        if (loading || !query) return;
        var cached = !force && readCache(query);
        if (cached) {
            render(cached, true);
            return;
        }
        loading = true;
        answer.hidden = true;
        regenerate.hidden = true;
        status.textContent = 'PI 正在检索并生成回答…';
        fetch('/api/pi/ask', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ question: query })
        })
            .then(function (response) {
                if (!response.ok) return response.text().then(function (message) { throw new Error(message); });
                return response.json();
            })
            .then(function (payload) {
                writeCache(query, payload);
                render(payload, false);
            })
            .catch(function (error) {
                status.textContent = 'PI 回答失败：' + (error.message || '请检查 Provider 配置。');
                regenerate.textContent = '重试';
                regenerate.hidden = false;
            })
            .finally(function () {
                loading = false;
            });
    }

    form.addEventListener('submit', function (event) {
        event.preventDefault();
        var value = (input.value || '').trim();
        if (!value) {
            status.textContent = '请输入问题。';
            input.focus();
            return;
        }
        window.location.assign('/ask?q=' + encodeURIComponent(value));
    });
    regenerate.addEventListener('click', function () {
        regenerate.textContent = '重新生成';
        try { sessionStorage.removeItem(cacheKey(query)); } catch (_) {}
        generate(true);
    });

    if (query) {
        generate(false);
    } else {
        status.textContent = '输入问题后生成回答。';
        input.focus();
    }
})();
