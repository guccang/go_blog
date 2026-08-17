(function () {
    'use strict';

    var storageKey = 'guccang-theme';
    var root = document.documentElement;
    var systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
    var pageName = root.dataset.page || window.location.pathname.replace(/^\/+|\/+$/g, '').split('/')[0] || 'index';

    root.dataset.page = pageName.replace(/[^a-z0-9_-]/gi, '-').toLowerCase();

    function savedTheme() {
        try {
            var value = window.localStorage.getItem(storageKey);
            return value === 'dark' || value === 'light' ? value : '';
        } catch (error) {
            return '';
        }
    }

    function preferredTheme() {
        return savedTheme() || (systemTheme.matches ? 'dark' : 'light');
    }

    function updateToggle(theme) {
        var toggle = document.querySelector('[data-theme-toggle]');
        if (!toggle) return;
        var nextTheme = theme === 'dark' ? 'light' : 'dark';
        toggle.setAttribute('aria-label', '切换到' + (nextTheme === 'dark' ? '深色' : '浅色') + '主题');
        toggle.setAttribute('title', '切换到' + (nextTheme === 'dark' ? '深色' : '浅色') + '主题');
        toggle.setAttribute('aria-pressed', String(theme === 'dark'));
        toggle.dataset.currentTheme = theme;
    }

    function applyTheme(theme, persist) {
        root.dataset.theme = theme;
        root.style.colorScheme = theme;
        updateToggle(theme);
        if (persist) {
            try {
                window.localStorage.setItem(storageKey, theme);
            } catch (error) {
                // 隐私模式下无法持久化时，当前页面仍然可以正常切换主题。
            }
        }
        window.dispatchEvent(new CustomEvent('guccang:themechange', { detail: { theme: theme } }));
    }

    function createToggle() {
        if (!document.body || document.querySelector('[data-theme-toggle]')) return;
        var toggle = document.createElement('button');
        toggle.type = 'button';
        toggle.className = 'ui-theme-toggle';
        toggle.dataset.themeToggle = '';
        toggle.innerHTML = '<span class="ui-theme-toggle__sun" aria-hidden="true">☀</span>' +
            '<span class="ui-theme-toggle__moon" aria-hidden="true">☾</span>';
        toggle.addEventListener('click', function () {
            applyTheme(root.dataset.theme === 'dark' ? 'light' : 'dark', true);
        });
        document.body.appendChild(toggle);
        updateToggle(root.dataset.theme || preferredTheme());
    }

    applyTheme(preferredTheme(), false);

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', createToggle, { once: true });
    } else {
        createToggle();
    }

    systemTheme.addEventListener('change', function (event) {
        if (!savedTheme()) applyTheme(event.matches ? 'dark' : 'light', false);
    });

    window.addEventListener('storage', function (event) {
        if (event.key === storageKey) applyTheme(preferredTheme(), false);
    });
})();
