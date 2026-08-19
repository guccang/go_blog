# 图鉴主题基座与首批 3 主题设计

日期：2026-08-18
状态：已获用户批准

## 背景

博客已有全站主题系统（`:root[data-theme]` + 语义令牌 `--ui-*` + 历史兼容变量），现有 classic / terminal / watercolor 三个主题。另有「百款视觉主题图鉴」页（visual_themes），以 10 系列 × 10 款共 100 个五色色板陈列于 `visual_themes.js`。

目标：把图鉴色板变成真正可应用的全站主题，首批实现 3 个，并为后续扩展到 100 个打好架构基础。

## 决策记录（来自头脑风暴）

- 选题：每个系列挑 1 个代表作，首批 3 个由我选定 → 001 青瓷雨、051 瑞士网格、061 液态铬
- 明暗：每个主题只做单一模式，忠于图鉴原色氛围
- 深度：色彩 + 材质气质（字体/圆角/阴影/纹理），非仅换色
- 架构：方案 A —— 图鉴基座 + 5 色声明（而非每主题完整 CSS 块）

## 架构

### atlas 基座（theme.css）

新增共享基座 `:root[data-theme^="atlas-"]`，从 `--c1`~`--c5` 推导整套语义令牌：

- 图鉴五色按「背景端 → 前景端」排列：c1=画布、c2=层叠面、c3=中间调、c4/c5=文字与强调
- 默认映射：`--ui-text: var(--ink, var(--c5))`、`--ui-coral(强调): var(--accent, var(--c4))`
- 派生色用 `color-mix()` 生成：text-soft/muted、line-soft、各 soft 变体、shadow-soft
- 基座同时包含现有 `:root[data-theme="terminal"], :root[data-theme="watercolor"]` 块中的全套历史兼容变量映射（--bg、--accent、--goal-* 等），保证老页面正常

### 单主题声明（每个 ~20 行）

每个具体主题块只写：

1. `--c1`~`--c5` 五个色值
2. 需要时用 `--ink` / `--accent` 纠正推导方向（暗色主题文字在 c4、强调在 c5 的情况）
3. 材质覆盖：字体族、圆角、边框宽度、阴影风格、body::before 纹理
4. `color-scheme`（light/dark）

## 首批 3 个主题

### atlas-celadon（001 青瓷雨，light）

- 色板：#E7EFEA #B8D2C7 #7FA99A #3F6F66 #283C39
- 文字=c5 墨绿，强调=c4 深松石
- 字体：宋体系衬线（display/body），宽字距
- 圆角 14px 柔圆；1px 淡青边框；青调弥散阴影
- body 纹理：极淡细雨斜线

### atlas-swiss（051 瑞士网格，light）

- 色板：#F5F4EF #1A1C1F #D9382A #B8BDC2 #6C7178
- 文字=c2 纯黑，强调=c3 信号红
- 字体：全套无衬线，紧字距，眉题大写
- 圆角 0 直角；2px 纯黑边框；无阴影或 4px 硬黑
- 无背景纹理（留白即网格）

### atlas-chrome（061 液态铬，dark）

- 色板：#080B10 #222935 #667589 #B9D1E6 #7B5CFF
- 文字=c4 银白（--ink: var(--c4)），强调=c5 电紫（--accent: var(--c5)）
- 字体：几何无衬线 + 等宽点缀
- 圆角 18px 大圆；1px 半透银边框；电紫辉光阴影
- body 纹理：银紫双色径向辉光

## theme.js 改造

- `themes` 表扩至 6 项，每项增加 `colors`（3 个代表色）与 `tagline`
- 选择器面板改为从 themes 表动态渲染按钮与预览色块（替代硬编码 HTML + 每主题预览 CSS 类）
- 面板加 max-height + 滚动，容纳主题数量增长

## 图鉴页联动

visual_themes 弹窗中，对已实现的 3 个主题显示「应用为网站主题」按钮：写入 localStorage（`guccang-theme`）并应用 data-theme。

## 涉及文件

- `cmd/blog-agent/statics/css/theme.css`：atlas 基座 + 3 个主题块 + 选择器面板滚动样式
- `cmd/blog-agent/statics/js/theme.js`：themes 表扩展 + 动态渲染选择器
- `cmd/blog-agent/statics/js/visual_themes.js`：已实现主题标记 + 应用按钮
- `cmd/blog-agent/statics/css/visual_themes.css`：应用按钮样式（如需）
- `cmd/blog-agent/pkgs/http/theme_template_test.go`：断言新主题注册

## 测试

- 单测：断言 3 个新主题出现在 theme.js 且 theme.css 含对应规则块
- 浏览器验证：首页/列表/文章/工具页逐一切换 3 个主题，重点确认瑞士网格直角+无衬线全局生效、液态铬暗色对比度可读、青瓷雨衬线字体生效
