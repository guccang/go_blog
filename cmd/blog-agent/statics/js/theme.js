(function () {
    'use strict';

    var storageKey = 'guccang-theme';
    var root = document.documentElement;
    var systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
    var themes = {
        classic: { name: '墨纸经典', shortName: '经典', colorScheme: 'light', colors: ['#F4F1E8', '#C84F35', '#566F76'], tagline: '清晰骨架 · 克制陈列' },
        terminal: { name: '夜间终端', shortName: '终端', colorScheme: 'dark', colors: ['#211A14', '#F15A29', '#E1A82F'], tagline: '硬边票据 · 点阵' },
        watercolor: { name: '水彩小馆', shortName: '水彩', colorScheme: 'light', colors: ['#FFFDF6', '#DC5A3C', '#4056B5'], tagline: '纸张晕染 · 手绘' },
        'atlas-celadon': { name: '青瓷雨', shortName: '青瓷', colorScheme: 'light', colors: ['#E7EFEA', '#7FA99A', '#3F6F66'], tagline: '雨水洗过青瓷釉面' },
        'atlas-dunhuang': { name: '敦煌暮色', shortName: '敦煌', colorScheme: 'dark', colors: ['#2B2533', '#B56A4C', '#D9A441'], tagline: '矿物金穿过风沙与飘带' },
        'atlas-swiss': { name: '瑞士网格', shortName: '瑞士', colorScheme: 'light', colors: ['#F5F4EF', '#1A1C1F', '#D9382A'], tagline: '严格网格 · 信号红' },
        'atlas-chrome': { name: '液态铬', shortName: '液铬', colorScheme: 'dark', colors: ['#080B10', '#B9D1E6', '#7B5CFF'], tagline: '液态银 · 电紫反光' }
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
        return savedTheme() || 'classic';
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

    function renderOptions(container) {
        Object.keys(themes).forEach(function (key) {
            var meta = themes[key];
            var option = document.createElement('button');
            option.className = 'ui-theme-picker__option';
            option.type = 'button';
            option.setAttribute('role', 'radio');
            option.dataset.themeOption = key;

            var preview = document.createElement('span');
            preview.className = 'ui-theme-picker__preview';
            preview.style.background = meta.colors[0];
            preview.style.color = meta.colors[1];
            preview.setAttribute('aria-hidden', 'true');
            meta.colors.forEach(function (color) {
                var bar = document.createElement('i');
                bar.style.background = color;
                preview.appendChild(bar);
            });

            var copy = document.createElement('span');
            var name = document.createElement('strong');
            name.textContent = meta.name;
            var tagline = document.createElement('small');
            tagline.textContent = meta.tagline;
            copy.appendChild(name);
            copy.appendChild(tagline);

            option.appendChild(preview);
            option.appendChild(copy);
            container.appendChild(option);
        });
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
                '<div class="ui-theme-picker__options" role="radiogroup" aria-label="网站主题"></div>' +
            '</div>';
        renderOptions(picker.querySelector('.ui-theme-picker__options'));

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
        if (!savedTheme()) applyTheme('classic', false);
    });

    window.addEventListener('storage', function (event) {
        if (event.key === storageKey) applyTheme(preferredTheme(), false);
    });
})();
