(function () {
    'use strict';

    var toggle = document.getElementById('articleAssistantToggle');
    var panel = document.getElementById('articleAssistantPanel');
    if (!toggle || !panel) return;

    var closeButton = document.getElementById('articleAssistantClose');
    var overlay = document.getElementById('articleAssistantOverlay');
    var status = document.getElementById('articleAssistantStatus');
    var answer = document.getElementById('articleAssistantAnswer');
    var answerLabel = document.getElementById('articleAssistantAnswerLabel');
    var answerText = document.getElementById('articleAssistantAnswerText');
    var sources = document.getElementById('articleAssistantSources');
    var usage = document.getElementById('articleAssistantUsage');
    var form = document.getElementById('articleAssistantForm');
    var question = document.getElementById('articleAssistantQuestion');
    var submit = form.querySelector('button[type="submit"]');
    var actionButtons = Array.prototype.slice.call(panel.querySelectorAll('[data-article-action]'));
    var title = panel.dataset.title || '';
    var pending = false;

    var actionLabels = {
        summary: '总结本文',
        key_points: '关键结论',
        related: '关联站内博客',
        next_steps: '下一步建议',
        question: '针对本文的回答'
    };

    function setOpen(open) {
        document.body.classList.toggle('article-assistant-open', open);
        toggle.setAttribute('aria-expanded', String(open));
        panel.setAttribute('aria-hidden', String(!open));
        if (overlay) overlay.hidden = !open;
        if (open) {
            window.setTimeout(function () { question.focus(); }, 180);
        } else {
            toggle.focus();
        }
    }

    function setPending(value, activeAction) {
        pending = value;
        submit.disabled = value;
        actionButtons.forEach(function (button) {
            button.disabled = value;
            button.classList.toggle('active', value && button.dataset.articleAction === activeAction);
        });
    }

    function setStatus(message, kind) {
        status.textContent = message;
        status.classList.toggle('loading', kind === 'loading');
        status.classList.toggle('error', kind === 'error');
    }

    function appendSource(container, name, current) {
        var item;
        if (current) {
            item = document.createElement('span');
            item.className = 'article-assistant-current-source';
            item.textContent = name + '（主要依据）';
        } else {
            item = document.createElement('a');
            item.className = 'article-assistant-source';
            item.href = '/get?blogname=' + encodeURIComponent(name);
            item.target = '_blank';
            item.rel = 'noopener';
            item.textContent = name;
        }
        container.appendChild(item);
    }

    function renderAnswer(payload, action) {
        answerLabel.textContent = actionLabels[action] || 'PI 回答';
        answerText.textContent = payload.text || '没有生成回答。';
        sources.textContent = '';

        var sourceLabel = document.createElement('div');
        sourceLabel.className = 'article-assistant-source-label';
        sourceLabel.textContent = '回答依据';
        sources.appendChild(sourceLabel);
        appendSource(sources, title, true);
        (payload.sources || []).forEach(function (source) {
            if (source && source !== title) appendSource(sources, source, false);
        });

        if (payload.usage && payload.usage.reported) {
            usage.textContent = '上传 ' + payload.usage.prompt_tokens +
                ' · 下载 ' + payload.usage.completion_tokens +
                ' · 合计 ' + payload.usage.total_tokens +
                ' Token · ' + (payload.duration_ms / 1000).toFixed(1) + ' 秒';
        } else {
            usage.textContent = payload.provider + ' / ' + payload.model + ' · Provider 未返回 Token 用量';
        }
        answer.hidden = false;
    }

    function ask(action, text) {
        if (pending) return;
        setPending(true, action);
        answer.hidden = true;
        setStatus(action === 'question' ? '正在结合本文和站内资料回答…' : '正在阅读当前文章…', 'loading');

        fetch('/api/pi/article', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title: title, action: action, question: text || '' })
        })
            .then(function (response) {
                if (!response.ok) {
                    return response.text().then(function (message) {
                        throw new Error((message || '请求失败').trim());
                    });
                }
                return response.json();
            })
            .then(function (payload) {
                renderAnswer(payload, action);
                setStatus('回答已生成。关联博客会在依据区域单独列出。', 'ready');
            })
            .catch(function (error) {
                setStatus('阅读助手失败：' + (error.message || '请检查 Provider 配置。'), 'error');
            })
            .finally(function () {
                setPending(false, '');
            });
    }

    toggle.addEventListener('click', function () { setOpen(true); });
    closeButton.addEventListener('click', function () { setOpen(false); });
    if (overlay) overlay.addEventListener('click', function () { setOpen(false); });

    actionButtons.forEach(function (button) {
        button.addEventListener('click', function () {
            ask(button.dataset.articleAction, '');
        });
    });

    form.addEventListener('submit', function (event) {
        event.preventDefault();
        var text = (question.value || '').trim();
        if (!text) {
            setStatus('先输入一个针对当前文章的问题。', 'error');
            question.focus();
            return;
        }
        ask('question', text);
    });

    question.addEventListener('keydown', function (event) {
        if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
            event.preventDefault();
            form.requestSubmit();
        }
    });

    document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape' && document.body.classList.contains('article-assistant-open')) {
            setOpen(false);
        }
    });
})();
