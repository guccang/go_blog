(function () {
    'use strict';

    var storageKey = 'guccang-theme';
    var root = document.documentElement;
    var systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
    var themes = {
        terminal: { name: '夜间终端', shortName: '终端', colorScheme: 'dark' },
        watercolor: { name: '水彩小馆', shortName: '水彩', colorScheme: 'light' }
    };
    var pageName = root.dataset.page || window.location.pathname.replace(/^\/+|\/+$/g, '').split('/')[0] || 'index';

    root.dataset.page = pageName.replace(/[^a-z0-9_-]/gi, '-').toLowerCase();

    function normalizeTheme(value) {
        if (value === 'dark') return 'terminal';
        if (value === 'light') return 'watercolor';
        return themes[value] ? value : '';
    }

    function savedTheme() {
        try {
            return normalizeTheme(window.localStorage.getItem(storageKey));
        } catch (error) {
            return '';
        }
    }

    function preferredTheme() {
        return savedTheme() || (systemTheme.matches ? 'terminal' : 'watercolor');
    }

    function updatePicker(theme) {
        var trigger = document.querySelector('[data-theme-picker-trigger]');
        if (trigger) {
            trigger.setAttribute('aria-label', '选择网站主题，当前：' + themes[theme].name);
            trigger.setAttribute('title', '当前主题：' + themes[theme].name);
            trigger.querySelector('[data-theme-picker-label]').textContent = themes[theme].shortName;
            trigger.dataset.currentTheme = theme;
        }

        document.querySelectorAll('[data-theme-option]').forEach(function (option) {
            var selected = option.dataset.themeOption === theme;
            option.setAttribute('aria-checked', String(selected));
            option.classList.toggle('is-selected', selected);
        });
    }

    function applyTheme(theme, persist) {
        var normalized = normalizeTheme(theme) || preferredTheme();
        root.dataset.theme = normalized;
        root.style.colorScheme = themes[normalized].colorScheme;
        updatePicker(normalized);

        if (persist) {
            try {
                window.localStorage.setItem(storageKey, normalized);
            } catch (error) {
                // 隐私模式下无法持久化时，当前页面仍然可以正常切换主题。
            }
        }

        window.dispatchEvent(new CustomEvent('guccang:themechange', { detail: { theme: normalized } }));
    }

    function setPanelOpen(picker, open) {
        var trigger = picker.querySelector('[data-theme-picker-trigger]');
        var panel = picker.querySelector('[data-theme-picker-panel]');
        trigger.setAttribute('aria-expanded', String(open));
        panel.hidden = !open;
        picker.classList.toggle('is-open', open);
    }

    function createPicker() {
        if (!document.body || document.querySelector('[data-theme-picker]')) return;

        var picker = document.createElement('div');
        picker.className = 'ui-theme-picker';
        picker.dataset.themePicker = '';
        picker.innerHTML =
            '<button class="ui-theme-picker__trigger" type="button" data-theme-picker-trigger ' +
                'aria-expanded="false" aria-controls="guccangThemePanel">' +
                '<span class="ui-theme-picker__trigger-mark" aria-hidden="true">◐</span>' +
                '<span data-theme-picker-label></span>' +
            '</button>' +
            '<div class="ui-theme-picker__panel" id="guccangThemePanel" data-theme-picker-panel hidden>' +
                '<p class="ui-theme-picker__title">选择画风</p>' +
                '<div class="ui-theme-picker__options" role="radiogroup" aria-label="网站主题">' +
                    '<button class="ui-theme-picker__option" type="button" role="radio" data-theme-option="terminal">' +
                        '<span class="ui-theme-picker__preview ui-theme-picker__preview--terminal" aria-hidden="true"><i></i><i></i><i></i></span>' +
                        '<span><strong>夜间终端</strong><small>硬边票据 · 点阵</small></span>' +
                    '</button>' +
                    '<button class="ui-theme-picker__option" type="button" role="radio" data-theme-option="watercolor">' +
                        '<span class="ui-theme-picker__preview ui-theme-picker__preview--watercolor" aria-hidden="true"><i></i><i></i><i></i></span>' +
                        '<span><strong>水彩小馆</strong><small>纸张晕染 · 手绘</small></span>' +
                    '</button>' +
                '</div>' +
            '</div>';

        var trigger = picker.querySelector('[data-theme-picker-trigger]');
        trigger.addEventListener('click', function () {
            setPanelOpen(picker, trigger.getAttribute('aria-expanded') !== 'true');
        });

        picker.querySelectorAll('[data-theme-option]').forEach(function (option) {
            option.addEventListener('click', function () {
                applyTheme(option.dataset.themeOption, true);
                setPanelOpen(picker, false);
                trigger.focus();
            });
        });

        document.addEventListener('click', function (event) {
            if (!picker.contains(event.target)) setPanelOpen(picker, false);
        });
        document.addEventListener('keydown', function (event) {
            if (event.key === 'Escape' && trigger.getAttribute('aria-expanded') === 'true') {
                setPanelOpen(picker, false);
                trigger.focus();
            }
        });

        document.body.appendChild(picker);
        updatePicker(root.dataset.theme || preferredTheme());
    }

    applyTheme(preferredTheme(), false);

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', createPicker, { once: true });
    } else {
        createPicker();
    }

    systemTheme.addEventListener('change', function (event) {
        if (!savedTheme()) applyTheme(event.matches ? 'terminal' : 'watercolor', false);
    });

    window.addEventListener('storage', function (event) {
        if (event.key === storageKey) applyTheme(preferredTheme(), false);
    });
})();
