# 图鉴主题基座与首批 3 主题实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把视觉图鉴的色板变成可应用的全站主题：建立 atlas 基座（5 色推导语义令牌），实现青瓷雨/瑞士网格/液态铬 3 个主题，并让图鉴页可直接应用。

**Architecture:** theme.css 新增 `:root[data-theme^="atlas-"]` 共享基座，从 `--c1`~`--c5` 用 `color-mix()` 推导全套 `--ui-*` 令牌与历史兼容变量；每个主题只声明 5 色 + 材质覆盖。theme.js 的选择器改为从 themes 表动态渲染。

**Tech Stack:** 原生 CSS（自定义属性、color-mix）、原生 JS（无框架）、Go 测试（读取静态文件断言契约）。

**Spec:** `docs/superpowers/specs/2026-08-18-atlas-themes-design.md`

## Global Constraints

- 每个主题只做单一模式（light 或 dark），忠于图鉴 5 色，不新增色板之外的品牌色
- 不得破坏 classic / terminal / watercolor 三个现有主题的视觉与测试契约
- `visual_themes.js` 中禁止新增 `#RRGGBB` 色值和 `['NNN',` 编号行（有测试断言全文件恰为 500 个色值、100 个编号）
- 主题色值在 theme.css 中统一大写 hex；Go 测试断言时对小写化后的文本匹配
- Go 测试运行目录为 `cmd/blog-agent`，命令 `go test ./pkgs/http/`
- 提交信息使用中文，格式 `type: 描述`（参考 git log 现有风格）

---

### Task 1: atlas 基座与 --ui-card-shadow 钩子（theme.css）

**Files:**
- Modify: `cmd/blog-agent/statics/css/theme.css`（两处卡片阴影规则 + 文件末尾追加基座）
- Test: `cmd/blog-agent/pkgs/http/atlas_themes_test.go`（新建）

**Interfaces:**
- Produces: `:root[data-theme^="atlas-"]` 基座（后续 3 个主题依赖它推导令牌）；`--ui-card-shadow` 钩子变量（后续主题用它覆盖卡片阴影）；测试辅助函数 `readThemeStyles(t)`（后续任务的测试复用）

- [ ] **Step 1: 写失败测试**

新建 `cmd/blog-agent/pkgs/http/atlas_themes_test.go`：

```go
package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readThemeStyles(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "css", "theme.css"))
	if err != nil {
		t.Fatalf("读取主题样式失败: %v", err)
	}
	return string(content)
}

func TestAtlasThemeBaseDerivesTokens(t *testing.T) {
	styles := readThemeStyles(t)
	for _, expected := range []string{
		`:root[data-theme^="atlas-"]`,
		`--ui-canvas: var(--c1)`,
		`--ui-text: var(--ink, var(--c5))`,
		`--ui-coral: var(--accent, var(--c4))`,
		`var(--ui-card-shadow, 4px 4px 0 var(--ui-shadow-soft))`,
	} {
		if !strings.Contains(styles, expected) {
			t.Errorf("图鉴基座缺少 %q", expected)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run TestAtlasThemeBaseDerivesTokens -v`
Expected: FAIL，提示图鉴基座缺少 `:root[data-theme^="atlas-"]`

- [ ] **Step 3: 改造两处通用卡片阴影规则**

theme.css 中第一处（约 236-242 行）：

```css
html[data-theme] body :where(
    .card, .panel, .recent-card, .goal-card, .goal-summary-card,
    .reading-card, .book-card, .tool-card, .product-card, .exercise-card,
    .clipboard-card, .stat-card, .metric-card
) {
    box-shadow: 4px 4px 0 var(--ui-shadow-soft);
}
```

改为：

```css
html[data-theme] body :where(
    .card, .panel, .recent-card, .goal-card, .goal-summary-card,
    .reading-card, .book-card, .tool-card, .product-card, .exercise-card,
    .clipboard-card, .stat-card, .metric-card
) {
    box-shadow: var(--ui-card-shadow, 4px 4px 0 var(--ui-shadow-soft));
}
```

第二处（约 576-584 行）：

```css
html[data-theme] body :where(
    .password-container, .error-container, .login-container, .config-notice,
    .clipboard-composer, .insight-card, .progress-card, .notes-section,
    .editor-panel, .preview-panel, .markdown-card, .comment-card,
    .comment-form-container, .answer-block, .book-detail-container,
    .progress-section, .info-item
) {
    box-shadow: 4px 4px 0 var(--ui-shadow-soft);
}
```

同样把 `box-shadow` 一行改为 `box-shadow: var(--ui-card-shadow, 4px 4px 0 var(--ui-shadow-soft));`

- [ ] **Step 4: 文件末尾追加 atlas 基座**

在 theme.css 末尾（`@media print` 块之后）追加：

```css
/* ===== 图鉴主题基座：从 --c1~--c5 推导全站语义令牌 ===== */
:root[data-theme^="atlas-"] {
    --ui-canvas: var(--c1);
    --ui-surface: color-mix(in srgb, var(--c1) 92%, var(--c2));
    --ui-surface-raised: var(--c2);
    --ui-text: var(--ink, var(--c5));
    --ui-text-soft: color-mix(in srgb, var(--ui-text) 78%, var(--c1));
    --ui-text-muted: color-mix(in srgb, var(--ui-text) 56%, var(--c1));
    --ui-line: var(--ui-text);
    --ui-line-soft: color-mix(in srgb, var(--ui-text) 26%, var(--c1));
    --ui-cream: var(--c1);
    --ui-amber: var(--accent, var(--c4));
    --ui-amber-soft: color-mix(in srgb, var(--accent, var(--c4)) 24%, var(--c1));
    --ui-coral: var(--accent, var(--c4));
    --ui-coral-soft: color-mix(in srgb, var(--accent, var(--c4)) 20%, var(--c1));
    --ui-sky: var(--c3);
    --ui-sky-soft: color-mix(in srgb, var(--c3) 26%, var(--c1));
    --ui-rose: var(--c3);
    --ui-rose-soft: color-mix(in srgb, var(--c3) 22%, var(--c1));
    --ui-success: var(--c3);
    --ui-warning: var(--accent, var(--c4));
    --ui-danger: var(--accent, var(--c4));
    --ui-shadow: var(--ui-text);
    --ui-shadow-soft: color-mix(in srgb, var(--ui-text) 14%, transparent);
    --ui-dot: color-mix(in srgb, var(--ui-text) 8%, transparent);
    --ui-font-display: "Avenir Next", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-font-body: Georgia, "Noto Serif SC", "Songti SC", serif;
    --ui-font-sans: "Avenir Next", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-font-mono: "Cascadia Mono", Consolas, "SFMono-Regular", monospace;
    --ui-radius: 14px;
    --ui-radius-sm: 9px;

    --bg: var(--ui-canvas);
    --surface: var(--ui-surface);
    --text: var(--ui-text);
    --secondary: var(--ui-text-soft);
    --muted: var(--ui-text-muted);
    --accent-soft: var(--ui-coral-soft);
    --navy: var(--ui-text);
    --border: var(--ui-line-soft);
    --font: var(--ui-font-sans);
    --paper: var(--ui-surface);
    --card: var(--ui-surface);
    --canvas: var(--ui-canvas);
    --line: var(--ui-line-soft);
    --soft-line: var(--ui-line-soft);
    --signal: var(--ui-sky);
    --signal-soft: var(--ui-sky-soft);
    --violet: var(--ui-rose);
    --coral: var(--ui-coral);
    --coral-dark: color-mix(in srgb, var(--ui-coral) 82%, var(--ui-text));
    --teal: var(--ui-success);
    --orange: var(--ui-amber);
    --orange-soft: var(--ui-amber-soft);
    --green: var(--ui-success);
    --green-soft: color-mix(in srgb, var(--c3) 22%, var(--c1));
    --clay: var(--ui-coral);
    --clay-dark: var(--coral-dark);
    --blue-soft: var(--ui-sky-soft);
    --danger: var(--ui-danger);
    --danger-soft: var(--ui-coral-soft);
    --shadow: 4px 4px 0 var(--ui-shadow);
    --primary-color: var(--ui-surface-raised);
    --primary-hover: var(--ui-amber-soft);
    --secondary-color: var(--ui-line-soft);
    --accent-color: var(--ui-coral);
    --accent-hover: var(--coral-dark);
    --text-color: var(--ui-text);
    --bg-color: var(--ui-canvas);
    --card-bg: var(--ui-surface);
    --lighter-bg: var(--ui-surface-raised);
    --border-color: var(--ui-line-soft);
    --success-color: var(--ui-success);
    --warning-color: var(--ui-warning);
    --danger-color: var(--ui-danger);
    --code-bg: var(--ui-surface-raised);
    --shadow-color: var(--ui-shadow-soft);
    --text-dark: var(--ui-text);
    --text-light: var(--ui-text-soft);
    --bg-light: var(--ui-canvas);
    --bg-white: var(--ui-surface);
    --goal-bg: var(--ui-canvas);
    --goal-card: var(--ui-surface);
    --goal-card-hover: var(--ui-surface-raised);
    --goal-accent: var(--ui-coral);
    --goal-accent-hover: var(--coral-dark);
    --goal-accent-light: var(--ui-coral-soft);
    --goal-text: var(--ui-text);
    --goal-text-secondary: var(--ui-text-soft);
    --goal-text-muted: var(--ui-text-muted);
    --goal-border: var(--ui-line-soft);
    --goal-border-light: var(--ui-line-soft);
    --goal-success: var(--ui-success);
    --goal-warning: var(--ui-warning);
    --goal-danger: var(--ui-danger);
}
```

注意：基座刻意不定义 `--accent: ...` 变量本身（`--accent` 是各主题可选的纠正钩子），也不定义 `--primary-color-rgb`（无法从 hex 推导 rgb 三元组，由各主题声明）。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'Theme' -v`
Expected: PASS（含既有 3 个主题测试，确认未破坏现有契约）

- [ ] **Step 6: 提交**

```bash
git add cmd/blog-agent/statics/css/theme.css cmd/blog-agent/pkgs/http/atlas_themes_test.go
git commit -m "feat: 新增图鉴主题基座，5色推导全站语义令牌"
```

---

### Task 2: theme.js 数据驱动选择器重构

**Files:**
- Modify: `cmd/blog-agent/statics/js/theme.js`（整体重写）
- Modify: `cmd/blog-agent/statics/css/theme.css`（选项列表面板滚动 + 删除 3 个预览修饰块）
- Test: `cmd/blog-agent/pkgs/http/theme_template_test.go:55-75`（更新断言清单）、`:84-99`（移除预览类断言）

**Interfaces:**
- Consumes: 无（独立重构）
- Produces: `themes` 表新结构 `{ name, shortName, colorScheme, colors[3], tagline }`（Task 3-5 向其中追加条目）；`renderOptions(container)` 动态渲染函数

- [ ] **Step 1: 更新失败测试**

`theme_template_test.go` 的 `TestThemeRuntimeIncludesPersistenceAndAccessibility` 断言清单中，删除三行 `data-theme-option="classic"` / `data-theme-option="terminal"` / `data-theme-option="watercolor"`，替换为：

```go
		"option.dataset.themeOption = key",
		"renderOptions",
```

`TestThemeStylesMatchAllVisualContracts` 断言清单中删除 `` `.ui-theme-picker__preview--classic` `` 一行。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run TestThemeRuntimeIncludesPersistenceAndAccessibility -v`
Expected: FAIL，提示缺少 `option.dataset.themeOption = key`

- [ ] **Step 3: 整体重写 theme.js**

完整替换 `cmd/blog-agent/statics/js/theme.js` 为：

```javascript
(function () {
    'use strict';

    var storageKey = 'guccang-theme';
    var root = document.documentElement;
    var systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
    var themes = {
        classic: { name: '经典原版', shortName: '原版', colorScheme: '', colors: ['#F7F8FA', '#5B78C7', '#E39A3B'], tagline: '页面原貌 · 熟悉布局' },
        terminal: { name: '夜间终端', shortName: '终端', colorScheme: 'dark', colors: ['#211A14', '#F15A29', '#E1A82F'], tagline: '硬边票据 · 点阵' },
        watercolor: { name: '水彩小馆', shortName: '水彩', colorScheme: 'light', colors: ['#FFFDF6', '#DC5A3C', '#4056B5'], tagline: '纸张晕染 · 手绘' }
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
        if (normalized === 'classic') {
            root.removeAttribute('data-theme');
            root.style.removeProperty('color-scheme');
        } else {
            root.dataset.theme = normalized;
            root.style.colorScheme = themes[normalized].colorScheme;
        }
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
```

- [ ] **Step 4: theme.css 面板滚动 + 删除旧预览修饰块**

把现有规则 `.ui-theme-picker__options { display: grid; gap: 9px; }` 改为：

```css
.ui-theme-picker__options {
    display: grid;
    max-height: min(430px, 62vh);
    gap: 9px;
    overflow-y: auto;
}
```

删除以下三个修饰块（约 748-784 行）：`.ui-theme-picker__preview--classic` 及其 3 个 `i` 规则、`.ui-theme-picker__preview--terminal` 及其 3 个 `i` 规则、`.ui-theme-picker__preview--watercolor` 及其 4 个 `i` 规则。保留基础规则 `.ui-theme-picker__preview`、`.ui-theme-picker__preview i` 和 3 个 `i:nth-child` 尺寸规则。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'Theme' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add cmd/blog-agent/statics/js/theme.js cmd/blog-agent/statics/css/theme.css cmd/blog-agent/pkgs/http/theme_template_test.go
git commit -m "refactor: 主题选择器改为数据驱动渲染，支持主题扩容"
```

---

### Task 3: atlas-celadon 青瓷雨主题

**Files:**
- Modify: `cmd/blog-agent/statics/css/theme.css`（末尾追加主题块）
- Modify: `cmd/blog-agent/statics/js/theme.js:9`（themes 表追加条目）
- Test: `cmd/blog-agent/pkgs/http/atlas_themes_test.go`（追加测试）

**Interfaces:**
- Consumes: Task 1 的 atlas 基座与 `--ui-card-shadow` 钩子；Task 2 的 themes 表结构
- Produces: 主题键 `atlas-celadon`（Task 6 图鉴联动映射 001 到它）；测试辅助 `readThemeScript(t)`（Task 4-5 复用）

- [ ] **Step 1: 写失败测试**

在 `atlas_themes_test.go` 追加：

```go
func readThemeScript(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "theme.js"))
	if err != nil {
		t.Fatalf("读取主题脚本失败: %v", err)
	}
	return string(content)
}

func TestAtlasThemePalettes(t *testing.T) {
	styles := strings.ToLower(readThemeStyles(t))
	script := readThemeScript(t)
	cases := []struct {
		key    string
		colors []string
	}{
		{key: "atlas-celadon", colors: []string{"#e7efea", "#b8d2c7", "#7fa99a", "#3f6f66", "#283c39"}},
	}
	for _, c := range cases {
		if !strings.Contains(styles, `data-theme="`+c.key+`"`) {
			t.Errorf("theme.css 缺少主题块 %q", c.key)
		}
		for _, color := range c.colors {
			if !strings.Contains(styles, color) {
				t.Errorf("主题 %s 缺少色值 %s", c.key, color)
			}
		}
		if !strings.Contains(script, `'`+c.key+`':`) {
			t.Errorf("theme.js 未注册主题 %q", c.key)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run TestAtlasThemePalettes -v`
Expected: FAIL，提示缺少主题块 `atlas-celadon`

- [ ] **Step 3: theme.css 追加青瓷雨主题块**

在文件末尾追加：

```css
/* 001 青瓷雨 Celadon Rain · 东方器韵 · light */
:root[data-theme="atlas-celadon"] {
    --c1: #E7EFEA;
    --c2: #B8D2C7;
    --c3: #7FA99A;
    --c4: #3F6F66;
    --c5: #283C39;
    --primary-color-rgb: 184, 210, 199;
    --ui-font-display: "Noto Serif SC", "Songti SC", "STSong", serif;
    --ui-font-body: Georgia, "Noto Serif SC", "Songti SC", serif;
    --ui-font-sans: "Noto Serif SC", "Songti SC", "STSong", serif;
    --ui-radius: 14px;
    --ui-radius-sm: 9px;
    --ui-card-shadow: 0 10px 24px color-mix(in srgb, var(--c4) 16%, transparent);
}

html[data-theme="atlas-celadon"] body::before {
    opacity: .5;
    background-image: repeating-linear-gradient(105deg, color-mix(in srgb, var(--c4) 4%, transparent) 0 1px, transparent 1px 9px);
}

html[data-theme="atlas-celadon"] :where(h1, h2, h3) { letter-spacing: .04em; }
```

- [ ] **Step 4: theme.js 注册主题**

themes 表中 `watercolor` 条目后追加（注意 watercolor 行尾逗号）：

```javascript
        'atlas-celadon': { name: '青瓷雨', shortName: '青瓷', colorScheme: 'light', colors: ['#E7EFEA', '#7FA99A', '#3F6F66'], tagline: '雨水洗过青瓷釉面' }
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'Theme' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add cmd/blog-agent/statics/css/theme.css cmd/blog-agent/statics/js/theme.js cmd/blog-agent/pkgs/http/atlas_themes_test.go
git commit -m "feat: 新增图鉴主题「青瓷雨」"
```

---

### Task 4: atlas-swiss 瑞士网格主题

**Files:**
- Modify: `cmd/blog-agent/statics/css/theme.css`（末尾追加主题块）
- Modify: `cmd/blog-agent/statics/js/theme.js`（themes 表追加条目）
- Test: `cmd/blog-agent/pkgs/http/atlas_themes_test.go`（cases 追加一行）

**Interfaces:**
- Consumes: Task 1 基座、Task 2 themes 表、Task 3 的 `TestAtlasThemePalettes`
- Produces: 主题键 `atlas-swiss`（Task 6 映射 051）

- [ ] **Step 1: 写失败测试**

`atlas_themes_test.go` 的 `TestAtlasThemePalettes` cases 中，在 celadon 行后追加：

```go
		{key: "atlas-swiss", colors: []string{"#f5f4ef", "#1a1c1f", "#d9382a", "#b8bdc2", "#6c7178"}},
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run TestAtlasThemePalettes -v`
Expected: FAIL，提示缺少主题块 `atlas-swiss`

- [ ] **Step 3: theme.css 追加瑞士网格主题块**

注意：此色板 c2 是纯黑、c3 是信号红，不能按位置默认推导，需显式纠正 `--ink` / `--accent` / `--ui-surface-raised`：

```css
/* 051 瑞士网格 Swiss Grid · 编辑印刷 · light */
:root[data-theme="atlas-swiss"] {
    --c1: #F5F4EF;
    --c2: #1A1C1F;
    --c3: #D9382A;
    --c4: #B8BDC2;
    --c5: #6C7178;
    --ink: var(--c2);
    --accent: var(--c3);
    --primary-color-rgb: 184, 189, 194;
    --ui-surface: var(--c1);
    --ui-surface-raised: color-mix(in srgb, var(--c1) 80%, var(--c4));
    --ui-line-soft: var(--c2);
    --ui-sky: var(--c5);
    --ui-rose: var(--c5);
    --ui-success: var(--c5);
    --ui-font-display: "Helvetica Neue", Helvetica, Arial, "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-font-body: "Helvetica Neue", Helvetica, Arial, "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-font-sans: "Helvetica Neue", Helvetica, Arial, "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-radius: 0;
    --ui-radius-sm: 0;
    --ui-card-shadow: none;
}

html[data-theme="atlas-swiss"] :where(h1, h2, h3) { letter-spacing: -.02em; }

html[data-theme="atlas-swiss"] :where(.eyebrow, .kicker, .label) {
    letter-spacing: .14em;
    text-transform: uppercase;
}
```

- [ ] **Step 4: theme.js 注册主题**

在 `atlas-celadon` 条目后追加（celadon 行尾补逗号）：

```javascript
        'atlas-swiss': { name: '瑞士网格', shortName: '瑞士', colorScheme: 'light', colors: ['#F5F4EF', '#1A1C1F', '#D9382A'], tagline: '严格网格 · 信号红' }
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'Theme' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add cmd/blog-agent/statics/css/theme.css cmd/blog-agent/statics/js/theme.js cmd/blog-agent/pkgs/http/atlas_themes_test.go
git commit -m "feat: 新增图鉴主题「瑞士网格」"
```

---

### Task 5: atlas-chrome 液态铬主题

**Files:**
- Modify: `cmd/blog-agent/statics/css/theme.css`（末尾追加主题块）
- Modify: `cmd/blog-agent/statics/js/theme.js`（themes 表追加条目）
- Test: `cmd/blog-agent/pkgs/http/atlas_themes_test.go`（cases 追加一行）

**Interfaces:**
- Consumes: 同 Task 4
- Produces: 主题键 `atlas-chrome`（Task 6 映射 061）

- [ ] **Step 1: 写失败测试**

`TestAtlasThemePalettes` cases 中，在 swiss 行后追加：

```go
		{key: "atlas-chrome", colors: []string{"#080b10", "#222935", "#667589", "#b9d1e6", "#7b5cff"}},
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run TestAtlasThemePalettes -v`
Expected: FAIL，提示缺少主题块 `atlas-chrome`

- [ ] **Step 3: theme.css 追加液态铬主题块**

暗色主题：文字在 c4 银白、强调在 c5 电紫，用 `--ink` / `--accent` 纠正；并覆盖 `color-scheme` 为 dark：

```css
/* 061 液态铬 Liquid Chrome · 数字未来 · dark */
:root[data-theme="atlas-chrome"] {
    --c1: #080B10;
    --c2: #222935;
    --c3: #667589;
    --c4: #B9D1E6;
    --c5: #7B5CFF;
    --ink: var(--c4);
    --accent: var(--c5);
    --primary-color-rgb: 34, 41, 53;
    --ui-font-display: "Avenir Next", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-font-body: "Avenir Next", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-font-sans: "Avenir Next", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
    --ui-radius: 18px;
    --ui-radius-sm: 12px;
    --ui-card-shadow: 0 0 24px color-mix(in srgb, var(--c5) 22%, transparent);
}

html[data-theme="atlas-chrome"] { color-scheme: dark; }

html[data-theme="atlas-chrome"] body::before {
    background-image:
        radial-gradient(ellipse at 18% 12%, color-mix(in srgb, var(--c5) 16%, transparent), transparent 42%),
        radial-gradient(ellipse at 84% 78%, color-mix(in srgb, var(--c4) 8%, transparent), transparent 46%);
}
```

- [ ] **Step 4: theme.js 注册主题**

在 `atlas-swiss` 条目后追加（swiss 行尾补逗号）：

```javascript
        'atlas-chrome': { name: '液态铬', shortName: '液铬', colorScheme: 'dark', colors: ['#080B10', '#B9D1E6', '#7B5CFF'], tagline: '液态银 · 电紫反光' }
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'Theme' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add cmd/blog-agent/statics/css/theme.css cmd/blog-agent/statics/js/theme.js cmd/blog-agent/pkgs/http/atlas_themes_test.go
git commit -m "feat: 新增图鉴主题「液态铬」"
```

---

### Task 6: 图鉴页「应用为网站主题」联动

**Files:**
- Modify: `cmd/blog-agent/statics/js/visual_themes.js`（implementedThemes 映射 + 应用逻辑）
- Modify: `cmd/blog-agent/templates/visual_themes.template:78-79`（弹窗加按钮）、`:10,86`（资产版本 v=1→v=2）
- Modify: `cmd/blog-agent/statics/css/visual_themes.css`（末尾追加按钮样式）
- Test: `cmd/blog-agent/pkgs/http/visual_themes_template_test.go`（版本号断言更新 + 新测试函数）

**Interfaces:**
- Consumes: Task 3-5 的主题键 `atlas-celadon` / `atlas-swiss` / `atlas-chrome`；localStorage 键 `guccang-theme`
- Produces: 弹窗按钮 `#dialog-apply`

**警告：** visual_themes.js 的新增代码不得包含 `#RRGGBB` 色值或 `['NNN',` 行，否则破坏既有 100 主题/500 色值计数测试。

- [ ] **Step 1: 写失败测试**

`visual_themes_template_test.go` 中 `TestVisualThemeAtlasPageIsWiredIntoTools` 的两处版本断言改为 `/css/visual_themes.css?v=2` 和 `/js/visual_themes.js?v=2`。再追加新测试函数：

```go
func TestVisualThemeAtlasAppliesImplementedThemes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "statics", "js", "visual_themes.js"))
	if err != nil {
		t.Fatalf("读取视觉主题脚本失败: %v", err)
	}
	script := string(content)
	for _, expected := range []string{
		"implementedThemes",
		"'001': 'atlas-celadon'",
		"'051': 'atlas-swiss'",
		"'061': 'atlas-chrome'",
		"dialog-apply",
		"guccang-theme",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("图鉴应用能力缺少 %q", expected)
		}
	}

	templateContent, err := os.ReadFile(filepath.Join("..", "..", "templates", "visual_themes.template"))
	if err != nil {
		t.Fatalf("读取视觉主题模板失败: %v", err)
	}
	if !strings.Contains(string(templateContent), `id="dialog-apply"`) {
		t.Error("视觉主题模板缺少应用按钮")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'VisualTheme' -v`
Expected: FAIL（版本断言与新函数均失败）

- [ ] **Step 3: 模板加按钮并升级版本**

`visual_themes.template` 中：
- `/css/visual_themes.css?v=1` 改为 `?v=2`，`/js/visual_themes.js?v=1` 改为 `?v=2`
- 在 `<div class="dialog-palette" ...></div>` 之后、`<p class="dialog-tip">` 之前插入：

```html
                <button class="dialog-apply" id="dialog-apply" type="button" hidden>应用为网站主题</button>
```

- [ ] **Step 4: visual_themes.js 加应用逻辑**

`var activeSeries = 'all';` 一行之后追加：

```javascript
    var implementedThemes = {
        '001': 'atlas-celadon',
        '051': 'atlas-swiss',
        '061': 'atlas-chrome'
    };
```

`openTheme` 函数末尾（`dialog.showModal();` 之前）插入：

```javascript
    var applyButton = document.getElementById('dialog-apply');
    var themeKey = implementedThemes[theme.id];
    applyButton.hidden = !themeKey;
    applyButton.dataset.themeKey = themeKey || '';
```

文件底部 `renderFilters();` 之前插入：

```javascript
    document.getElementById('dialog-apply').addEventListener('click', function () {
        var key = this.dataset.themeKey;
        if (!key) return;
        try {
            window.localStorage.setItem('guccang-theme', key);
        } catch (error) {
            // 隐私模式下仅当前页面临时生效。
        }
        document.documentElement.dataset.theme = key;
        showToast('已应用主题，其他页面将同步生效');
    });
```

已知小限制：应用后主题选择器浮层的当前标签在此页面不实时刷新，跳转页面后即正常（theme.js 初始化时读取 localStorage）。

- [ ] **Step 5: visual_themes.css 追加按钮样式**

文件末尾追加：

```css
.dialog-apply {
    margin-top: 14px;
    padding: 10px 18px;
    border: 1px solid currentColor;
    background: transparent;
    color: inherit;
    font: inherit;
    letter-spacing: .08em;
    cursor: pointer;
}

.dialog-apply:hover { background: rgba(0, 0, 0, .08); }
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'VisualTheme' -v`
Expected: PASS（含 100 主题/500 色值计数测试）

- [ ] **Step 7: 提交**

```bash
git add cmd/blog-agent/statics/js/visual_themes.js cmd/blog-agent/statics/css/visual_themes.css cmd/blog-agent/templates/visual_themes.template cmd/blog-agent/pkgs/http/visual_themes_template_test.go
git commit -m "feat: 图鉴弹窗支持一键应用已实现的主题"
```

---

### Task 7: 全站主题资产版本升级 v=3→v=4

theme.js / theme.css 均已变更，升级版本号避免浏览器旧缓存。

**Files:**
- Modify: `cmd/blog-agent/templates/*.template`（全部含 theme.js/theme.css 引用的模板）
- Test: `cmd/blog-agent/pkgs/http/theme_template_test.go:34`、`cmd/blog-agent/pkgs/http/visual_themes_template_test.go:57-58`

- [ ] **Step 1: 更新测试期望版本**

`theme_template_test.go` 第 34 行改为：

```go
		for _, asset := range []string{`/js/theme.js?v=4`, `/css/theme.css?v=4`} {
```

`visual_themes_template_test.go` 中 `/js/theme.js?v=3` → `?v=4`、`/css/theme.css?v=3` → `?v=4`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -run 'Theme' -v`
Expected: FAIL，提示模板仍引用 v=3

- [ ] **Step 3: 批量升级模板版本号**

Run:

```bash
cd cmd/blog-agent/templates && sed -i '' 's|/js/theme.js?v=3|/js/theme.js?v=4|g; s|/css/theme.css?v=3|/css/theme.css?v=4|g' *.template
grep -l 'theme.js?v=3\|theme.css?v=3' *.template | wc -l
```

Expected: 第二行输出 `0`（无残留旧版本）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd cmd/blog-agent && go test ./pkgs/http/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
git add cmd/blog-agent/templates cmd/blog-agent/pkgs/http/theme_template_test.go cmd/blog-agent/pkgs/http/visual_themes_template_test.go
git commit -m "chore: 全站主题资产版本升级至 v4"
```

---

### Task 8: 全量测试与浏览器人工验证

**Files:** 无改动，仅验证

- [ ] **Step 1: 全量测试**

Run: `cd cmd/blog-agent && go test ./...`
Expected: 全部 PASS

- [ ] **Step 2: 启动服务**

Run: `cd cmd/blog-agent && go run . -port 8080`

- [ ] **Step 3: 浏览器逐项验证**

打开 `http://localhost:8080/main`，通过右上角主题选择器逐一验证：

- atlas-celadon：页面为宣纸青绿底、衬线字体、柔圆卡片带青色弥散阴影、背景有极淡斜雨纹
- atlas-swiss：直角卡片、纯黑边框、无阴影、全站无衬线、红色强调按钮
- atlas-chrome：深色底、银白文字、电紫按钮、卡片带紫色辉光、背景有径向辉光
- 切换回 classic / terminal / watercolor：三者视觉与之前一致（回归检查）
- `/tools/visual-themes`：打开 001/051/061 三张卡片详情，出现「应用为网站主题」按钮，点击后当前页换肤且 toast 提示；打开其他卡片（如 002）不出现该按钮
- 抽查 `/list`、`/tools`、一篇文章详情页在 3 个新主题下无破版、文字可读

- [ ] **Step 4: 发现问题则修复并补测试；全部通过后停止服务**

无需提交（无代码改动）；若有修复则随修复提交。
