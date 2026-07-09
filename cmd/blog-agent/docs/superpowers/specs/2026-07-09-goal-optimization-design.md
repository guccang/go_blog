# 目标管理优化

## 概述

对博客目标管理模块进行综合优化：引入 OKR 式层级对齐、增强任务管理能力、新增周期回顾功能、前端组件化重构、清理遗留死代码。

## 数据模型

### Goal（扩展 `pkgs/goal/goal.go`）

新增 `ParentID` 字段实现松散 OKR 对齐（子目标单向引用父目标）。

```go
type Goal struct {
    ID        string `json:"id"`
    Level     string `json:"level"`                // daily, weekly, monthly, yearly
    Period    string `json:"period"`
    ParentID  string `json:"parent_id,omitempty"`  // 新增：引用上层目标
    Overview  string `json:"overview"`
    Judge     string `json:"judge,omitempty"`
    Tasks     []Task `json:"tasks"`
    Progress  int    `json:"progress"`
    Status    string `json:"status"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}
```

### Task（增强）

新增截止日期、预估耗时、子任务检查清单、备注日志。

```go
type Task struct {
    ID            string     `json:"id"`
    Title         string     `json:"title"`
    Description   string     `json:"description,omitempty"`
    Status        string     `json:"status"`
    Priority      string     `json:"priority"`
    Deadline      string     `json:"deadline,omitempty"`       // 新增 YYYY-MM-DD
    EstimateHours float64    `json:"estimate_hours,omitempty"` // 新增
    Subtasks      []Subtask  `json:"subtasks,omitempty"`       // 新增
    Notes         []TaskNote `json:"notes,omitempty"`          // 新增
    CreatedAt     string     `json:"created_at"`
    UpdatedAt     string     `json:"updated_at"`
}

type Subtask struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Status string `json:"status"` // pending, completed
}

type TaskNote struct {
    ID        string `json:"id"`
    Content   string `json:"content"`
    CreatedAt string `json:"created_at"`
}
```

### Review（新增）

```go
type Review struct {
    ID        string `json:"id"`
    Level     string `json:"level"`     // weekly, monthly
    Period    string `json:"period"`
    Content   string `json:"content"`   // markdown
    Completed int    `json:"completed"` // 自动统计
    Total     int    `json:"total"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}
```

Review 持久化方式：与 Goal 一致，序列化为 JSON 存入博客条目，标题 `回顾_{level}_{period}`。

## 前端架构

### 技术选择

Web Components（Custom Elements）+ 轻量 pub/sub 状态管理，原生浏览器支持，零依赖。

### 文件结构

```
statics/js/goal/
  store.js           # 全局状态（pub/sub）
  api.js             # API 封装
  components/
    goal-app.js      # 根组件
    goal-tabs.js     # 层级标签页
    period-nav.js    # 期间导航
    goal-detail.js   # 详情视图
    goal-overview.js # 概述 + 判据编辑
    task-list.js     # 任务列表
    task-item.js     # 单个任务（含子任务/备注展开）
    task-editor.js   # 任务编辑弹窗
    goal-list.js     # 列表视图
    review-panel.js  # 回顾面板
    review-editor.js # 回顾编辑
  utils.js           # 日期格式化、期间计算

statics/css/
  goal.css           # 设计系统 + 所有样式

templates/
  goal.template      # 精简 HTML 骨架
```

### 状态管理（store.js）

```javascript
const store = {
  state: {
    level: 'daily',
    period: '',
    view: 'detail',      // detail | list | review
    goal: null,
    goals: [],
    parentGoal: null,
    review: null,
    loading: false,
  },
  listeners: {},
  on(event, fn) { ... },
  dispatch(event, data) { ... },
};
```

组件通过 `store.on('goal:loaded', this.render)` 订阅状态变更，`store.dispatch` 触发更新。

### 组件树

```
<goal-app>
  <goal-tabs>
  <period-nav>
  [view=detail]  <goal-overview> + <task-list> → <task-item>*
  [view=list]    <goal-list>
  [view=review]  <review-panel> + <review-editor>
```

## API 变更

### 新增路由

| 路由 | 用途 |
|------|------|
| `GET /api/goal/parent?level=&period=` | 获取可选父目标列表 |
| `POST /api/goal/task/note` | 追加任务备注 |
| `GET /api/goal/review?level=&period=` | 获取回顾 |
| `POST /api/goal/review/save` | 保存回顾 |
| `POST /api/goal/review/generate` | 自动生成回顾草稿 |

### 扩展路由

- `POST /api/goal/save` — body 新增 `parent_id`
- `POST /api/goal/task` / `POST /api/goal/task/update` — body 新增 `deadline`, `estimate_hours`, `subtasks`, `notes`

## 关键交互

### OKR 对齐

- 创建/编辑非年目标时，调用 `/api/goal/parent` 获取上层目标列表
- 用户选择后设置 `parent_id`，详情页顶部展示对齐面包屑
- 面包屑可点击跳转到父目标

### 任务增强

- 任务展开态：子任务（内联检查清单）+ 备注日志（时间倒序，自动日期戳）
- 截止日期：日期选择器
- 预估耗时：小时数输入

### 周期回顾

- review 视图检测当期是否有回顾
- 无则展示"生成回顾"按钮，调用 `/api/goal/review/generate`
- 后端自动汇总：完成任务数、子任务数、进度百分比、生成 markdown 模板
- 用户在 markdown 编辑器中补充后保存

## 清理计划

移除以下死代码：

| 文件 | 说明 |
|------|------|
| `templates/monthgoal.template` | 旧月度目标模板 |
| `statics/js/monthgoal.js` | 旧月度目标 JS |
| `statics/css/monthgoal.css` | 旧月度目标 CSS |
| `pkgs/yearplan/` (整个包) | 旧 yearplan 数据模型 + 处理器 |
| `pkgs/view/view.go` 中 `PageMonthGoal` 函数 | 旧渲染函数 |
| `pkgs/http/http_lifecycle.go` 中 `HandleMonthGoal` 及相关 import | 旧 HTTP 处理器 |
| `pkgs/mcp/innter_mcp.go` 中旧 yearplan 工具注册/MCP 回调 | MCP 死引用 |

## 实施顺序

1. 清理死代码（先减后加，减少干扰）
2. 扩展数据模型 + API（`pkgs/goal/`）
3. 搭建前端骨架（store + api + goal-app + goal-tabs + period-nav）
4. 详情视图组件（goal-detail, goal-overview, task-list, task-item, task-editor）
5. OKR 对齐功能
6. 列表视图组件（goal-list）
7. 回顾系统（review-panel, review-editor + API）
8. CSS 设计系统（`goal.css`）+ 交互打磨
