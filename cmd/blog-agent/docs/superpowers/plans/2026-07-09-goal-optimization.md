# 目标管理优化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 对博客目标管理模块进行综合优化——OKR层级对齐、任务增强、周期回顾、Web Components组件化重构、清理死代码。

**Architecture:** 扩展 `pkgs/goal/` 数据模型和API，前端用 Web Components + pub/sub store 取代当前的单一 `goal.js` + 内联CSS，模板退化为轻量骨架。死代码（yearplan包、monthgoal模板/JS/CSS）全部移除。

**Tech Stack:** Go (后端API), Web Components / Custom Elements (前端), vanilla JS ES modules, pub/sub state management, CSS custom properties

## Global Constraints

- 零前端依赖，原生浏览器API
- ES modules (`type="module"`)，import map 或相对路径导入
- 所有API返回JSON，认证通过session cookie
- 目标数据持久化为博客条目JSON
- PC端优先，移动端响应式基础
- 每个task独立可测试

---

### Task 1: 清理死代码

**Files:**
- Delete: `templates/monthgoal.template`
- Delete: `statics/js/monthgoal.js`
- Delete: `statics/css/monthgoal.css`
- Delete: `pkgs/yearplan/yearplan.go`
- Delete: `pkgs/yearplan/handlers.go`
- Delete: `pkgs/yearplan/go.mod`
- Delete: `pkgs/mcp/inner_yearplan_tasks_tools.go`
- Modify: `pkgs/view/view.go` — 移除 PageMonthGoal, PageYearPlan 函数及相关 struct
- Modify: `pkgs/http/http_lifecycle.go` — 移除 HandleMonthGoal, HandleYearPlan
- Modify: `pkgs/http/http_core.go` — 清理已注释的旧路由
- Modify: `pkgs/mcp/innter_mcp.go` — 移除 yearplan/taskbreakdown 工具注册
- Modify: `pkgs/statistics/statistics_modules.go` — 移除 yearplan 相关 Raw 函数和 import

**Interfaces:**
- Consumes: nothing
- Produces: clean codebase for subsequent tasks

- [ ] **Step 1: 删除旧模板/静态文件**

```bash
rm templates/monthgoal.template
rm statics/js/monthgoal.js
rm statics/css/monthgoal.css
rm -rf pkgs/yearplan/
```

- [ ] **Step 2: 从 view.go 移除 PageMonthGoal 和 PageYearPlan**

在 `pkgs/view/view.go` 中，删除 `PageYearPlan` 函数（line 698-719）和 `PageMonthGoal` 函数（line 740-761），以及相关 struct 定义。

先查看 struct 定义位置：
```bash
grep -n "type YearPlanData\|type MonthGoalData" pkgs/view/view.go
```

然后删除这两个 struct 和两个函数。

- [ ] **Step 3: 从 http_lifecycle.go 移除 HandleMonthGoal 和 HandleYearPlan**

删除 `HandleYearPlan`（line 36-53）和 `HandleMonthGoal`（line 56-79）。检查 `strconv` import 是否仍被其他函数使用（HandleTodolist 也用了 `strconv`，所以 import 保留）。

- [ ] **Step 4: 清理 http_core.go 中已注释的旧路由**

删除 line 269-281（yearplan/monthgoal 注释块）和 line 283-285（statistics 注释块）和 line 287-292（projectmgmt 注释块）。

- [ ] **Step 5: 移除 MCP 旧工具注册**

在 `pkgs/mcp/innter_mcp.go` 中：
- 删除 line 165 的注释
- 删除 line 309-319（RegisterCallBack for yearplan 和 taskbreakdown 工具）

删除整个文件 `pkgs/mcp/inner_yearplan_tasks_tools.go`。

- [ ] **Step 6: 从 statistics_modules.go 移除 yearplan 相关代码**

删除 line 350-418（YearPlan Raw 接口部分：RawGetMonthGoal, RawGetYearGoals, RawAddYearTask, RawUpdateYearTask）。
删除 import 中的 `"yearplan"`（line 13）。

- [ ] **Step 7: 验证编译**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功，无错误。

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor(goal): 清理目标管理遗留死代码

移除 yearplan 包、monthgoal 模板/JS/CSS、旧 MCP 工具注册、
HTTP 路由注释块，及相关 view/http handler 函数。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 2: 扩展数据模型

**Files:**
- Modify: `pkgs/goal/goal.go` — 扩展 Goal, Task, 新增 Subtask/TaskNote/Review 类型及 CRUD

**Interfaces:**
- Consumes: Task 1 (clean codebase)
- Produces:
  - `Goal.ParentID string`
  - `Task.Deadline string`, `Task.EstimateHours float64`, `Task.Subtasks []Subtask`, `Task.Notes []TaskNote`
  - `type Subtask struct { ID, Title, Status string }`
  - `type TaskNote struct { ID, Content, CreatedAt string }`
  - `type Review struct { ID, Level, Period, Content string; Completed, Total int; CreatedAt, UpdatedAt string }`
  - `func GetParentGoals(account, level, period string) ([]*GoalSummary, error)`
  - `func AddTaskNote(account, level, period, taskID, content string) error`
  - `func GetReview(account, level, period string) (*Review, error)`
  - `func SaveReview(account string, review *Review) error`
  - `func GenerateReview(account, level, period string) (*Review, error)`

- [ ] **Step 1: 扩展 Task struct 和新增 Subtask/TaskNote 类型**

在 `pkgs/goal/goal.go` 中，修改 Task struct：

```go
type Task struct {
    ID            string     `json:"id"`
    Title         string     `json:"title"`
    Description   string     `json:"description,omitempty"`
    Status        string     `json:"status"`   // pending, in_progress, completed, cancelled
    Priority      string     `json:"priority"` // low, medium, high
    Deadline      string     `json:"deadline,omitempty"`       // YYYY-MM-DD
    EstimateHours float64    `json:"estimate_hours,omitempty"`
    Subtasks      []Subtask  `json:"subtasks,omitempty"`
    Notes         []TaskNote `json:"notes,omitempty"`
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

- [ ] **Step 2: 扩展 Goal struct 和新增 Review 类型**

```go
type Goal struct {
    ID        string `json:"id"`
    Level     string `json:"level"`
    Period    string `json:"period"`
    ParentID  string `json:"parent_id,omitempty"` // OKR 对齐
    Overview  string `json:"overview"`
    Judge     string `json:"judge,omitempty"`
    Tasks     []Task `json:"tasks"`
    Progress  int    `json:"progress"`
    Status    string `json:"status"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}

type Review struct {
    ID        string `json:"id"`
    Level     string `json:"level"`     // weekly, monthly
    Period    string `json:"period"`
    Content   string `json:"content"`   // markdown
    Completed int    `json:"completed"`
    Total     int    `json:"total"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}
```

- [ ] **Step 3: 添加 parentGoalTitle 和 reviewTitle 辅助函数**

```go
func parentGoalTitle(level, period string) string {
    return fmt.Sprintf("目标_%s_%s", level, period)
}
// 复用现有 goalTitle

func reviewTitle(level, period string) string {
    return fmt.Sprintf("回顾_%s_%s", level, period)
}
```

- [ ] **Step 4: 添加 GetParentGoals 函数**

获取可作为父目标的上一层级目标列表：

```go
func GetParentGoals(account, level, period string) ([]*GoalSummary, error) {
    if level == LevelYearly {
        return nil, nil // 年目标没有父级
    }

    parentLevels := map[string]string{
        LevelDaily:   LevelWeekly,
        LevelWeekly:  LevelMonthly,
        LevelMonthly: LevelYearly,
    }

    parentLevel := parentLevels[level]
    var candidates []*GoalSummary

    for _, b := range blog.GetBlogsWithAccount(account) {
        if !strings.Contains(b.Title, "目标_"+parentLevel) {
            continue
        }
        var goal Goal
        if err := json.Unmarshal([]byte(b.Content), &goal); err != nil {
            continue
        }
        if goal.Status == "archived" {
            continue
        }
        candidates = append(candidates, goal.Summary())
    }
    return candidates, nil
}
```

- [ ] **Step 5: 添加 AddTaskNote 函数**

```go
func AddTaskNote(account, level, period, taskID, content string) error {
    goal, err := GetGoal(account, level, period)
    if err != nil {
        return err
    }

    for i, t := range goal.Tasks {
        if t.ID == taskID {
            note := TaskNote{
                ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
                Content:   content,
                CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
            }
            goal.Tasks[i].Notes = append(goal.Tasks[i].Notes, note)
            return SaveGoal(account, goal)
        }
    }
    return fmt.Errorf("task %s not found", taskID)
}
```

- [ ] **Step 6: 添加 Review CRUD 函数**

```go
func GetReview(account, level, period string) (*Review, error) {
    title := reviewTitle(level, period)
    b := blog.GetBlogWithAccount(account, title)
    if b == nil {
        return nil, nil
    }
    var review Review
    if err := json.Unmarshal([]byte(b.Content), &review); err != nil {
        return nil, fmt.Errorf("failed to parse review: %w", err)
    }
    return &review, nil
}

func SaveReview(account string, review *Review) error {
    review.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
    if review.CreatedAt == "" {
        review.CreatedAt = review.UpdatedAt
    }

    title := reviewTitle(review.Level, review.Period)
    content, err := json.MarshalIndent(review, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal review: %w", err)
    }

    udb := &module.UploadedBlogData{
        Title:    title,
        Content:  string(content),
        AuthType: module.EAuthType_private,
        Tags:     "回顾_" + review.Level,
        Encrypt:  0,
        Account:  account,
    }

    existing := blog.GetBlogWithAccount(account, title)
    if existing != nil {
        blog.ModifyBlogWithAccount(account, udb)
    } else {
        blog.AddBlogWithAccount(account, udb)
    }
    return nil
}

func GenerateReview(account, level, period string) (*Review, error) {
    goal, err := GetGoal(account, level, period)
    if err != nil {
        return nil, err
    }

    completed := 0
    for _, t := range goal.Tasks {
        if t.Status == "completed" {
            completed++
        }
    }

    total := len(goal.Tasks)
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("# %s 回顾\n\n", periodLabel(level, period)))
    sb.WriteString(fmt.Sprintf("## 完成情况\n\n"))
    sb.WriteString(fmt.Sprintf("- 总任务: %d\n", total))
    sb.WriteString(fmt.Sprintf("- 已完成: %d\n", completed))
    sb.WriteString(fmt.Sprintf("- 完成率: %d%%\n\n", goal.Progress))
    sb.WriteString("## 任务详情\n\n")
    for _, t := range goal.Tasks {
        icon := "[ ]"
        if t.Status == "completed" {
            icon = "[x]"
        }
        sb.WriteString(fmt.Sprintf("- %s %s\n", icon, t.Title))
    }
    sb.WriteString("\n## 总结反思\n\n")

    review := &Review{
        ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
        Level:     level,
        Period:    period,
        Content:   sb.String(),
        Completed: completed,
        Total:     total,
    }
    return review, nil
}

func periodLabel(level, period string) string {
    switch level {
    case LevelWeekly:
        year, week, _ := parseISOWeek(period)
        return fmt.Sprintf("%d年第%d周", year, week)
    case LevelMonthly:
        return period + "月"
    }
    return period
}
```

- [ ] **Step 7: 验证编译**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 8: Commit**

```bash
git add pkgs/goal/goal.go
git commit -m "feat(goal): 扩展数据模型 - ParentID对齐、任务增强、Review类型"
```

---

### Task 3: 扩展 API handlers

**Files:**
- Modify: `pkgs/goal/handlers.go` — 新增 handler + 扩展已有 handler
- Modify: `pkgs/http/http_core.go` — 注册新路由

**Interfaces:**
- Consumes: Task 2 (data model)
- Produces:
  - `HandleGetParentGoals` — `GET /api/goal/parent?level=&period=`
  - `HandleAddTaskNote` — `POST /api/goal/task/note`
  - `HandleGetReview` — `GET /api/goal/review?level=&period=`
  - `HandleSaveReview` — `POST /api/goal/review/save`
  - `HandleGenerateReview` — `POST /api/goal/review/generate`
  - 扩展 `HandleSaveGoal` 接受 `parent_id`
  - 扩展 `HandleUpdateGoalTask` 接受 `deadline`, `estimate_hours`, `subtasks`

- [ ] **Step 1: 添加 HandleGetParentGoals**

在 `handlers.go` 末尾添加：

```go
func HandleGetParentGoals(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    level := r.URL.Query().Get("level")
    period := r.URL.Query().Get("period")
    account := getAccount(r)

    parents, err := GetParentGoals(account, level, period)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false, "message": err.Error(),
        })
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true, "data": parents,
    })
}
```

- [ ] **Step 2: 添加 HandleAddTaskNote**

```go
func HandleAddTaskNote(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Method not allowed"})
        return
    }

    body, _ := io.ReadAll(r.Body)
    var req struct {
        Level   string `json:"level"`
        Period  string `json:"period"`
        TaskID  string `json:"task_id"`
        Content string `json:"content"`
    }
    if err := json.Unmarshal(body, &req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid JSON"})
        return
    }

    account := getAccount(r)
    if err := AddTaskNote(account, req.Level, req.Period, req.TaskID, req.Content); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}
```

- [ ] **Step 3: 添加 Review handlers**

```go
func HandleGetReview(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    level := r.URL.Query().Get("level")
    period := r.URL.Query().Get("period")
    account := getAccount(r)

    review, err := GetReview(account, level, period)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "data":    review,
    })
}

func HandleSaveReview(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    body, _ := io.ReadAll(r.Body)
    var req struct {
        Level   string `json:"level"`
        Period  string `json:"period"`
        Content string `json:"content"`
    }
    if err := json.Unmarshal(body, &req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    account := getAccount(r)
    review, _ := GetReview(account, req.Level, req.Period)
    if review == nil {
        review = &Review{
            ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
            Level: req.Level, Period: req.Period,
        }
    }
    review.Content = req.Content

    if err := SaveReview(account, review); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
        return
    }
    json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func HandleGenerateReview(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    body, _ := io.ReadAll(r.Body)
    var req struct {
        Level  string `json:"level"`
        Period string `json:"period"`
    }
    json.Unmarshal(body, &req)

    account := getAccount(r)
    review, err := GenerateReview(account, req.Level, req.Period)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": review})
}
```

- [ ] **Step 4: 扩展 HandleSaveGoal 接受 parent_id**

修改 `HandleSaveGoal` 中的请求 struct，添加 `ParentID` 字段：

```go
var req struct {
    Level    string  `json:"level"`
    Period   string  `json:"period"`
    Overview *string `json:"overview"`
    Status   *string `json:"status"`
    ParentID *string `json:"parent_id"`
}
```

在更新 goal 的逻辑中添加：

```go
if req.ParentID != nil {
    goal.ParentID = *req.ParentID
}
```

- [ ] **Step 5: 注册新路由**

在 `pkgs/http/http_core.go` 中，目标是注册行（紧跟现有 `/api/goals` 之后）：

```go
h.HandleFunc("/api/goal/parent", goalpkg.HandleGetParentGoals)
h.HandleFunc("/api/goal/task/note", goalpkg.HandleAddTaskNote)
h.HandleFunc("/api/goal/review", goalpkg.HandleGetReview)
h.HandleFunc("/api/goal/review/save", goalpkg.HandleSaveReview)
h.HandleFunc("/api/goal/review/generate", goalpkg.HandleGenerateReview)
```

- [ ] **Step 6: 验证编译**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 7: Commit**

```bash
git add pkgs/goal/handlers.go pkgs/http/http_core.go
git commit -m "feat(goal): 新增 Review/Parent/TaskNote API handlers"
```

---

### Task 4: 前端骨架 — store + api + utils + 根组件 + tabs + period-nav

**Files:**
- Create: `statics/js/goal/store.js`
- Create: `statics/js/goal/api.js`
- Create: `statics/js/goal/utils.js`
- Create: `statics/js/goal/components/goal-app.js`
- Create: `statics/js/goal/components/goal-tabs.js`
- Create: `statics/js/goal/components/period-nav.js`
- Create: `statics/css/goal.css` (初始设计系统)
- Modify: `templates/goal.template` — 精简为 HTML 骨架

**Interfaces:**
- Consumes: Task 3 (API handlers)
- Produces:
  - `store` — `{ state, on(event, fn), off(event, fn), dispatch(event, data), setState(partial) }`
  - `api` — `{ getGoal, saveGoal, addTask, updateTask, deleteTask, deleteGoal, getParentGoals, addTaskNote, getReview, saveReview, generateReview, getCurrentGoals, listGoals }`
  - `utils` — `{ formatPeriod, formatDate, levelLabel, periodLabel, parentLevel }`
  - `<goal-app>` — 根 Custom Element，挂载到 `#app`
  - `<goal-tabs>` — 层级标签页
  - `<period-nav>` — 期间导航

- [ ] **Step 1: 创建 store.js**

```javascript
// statics/js/goal/store.js
const store = {
  state: {
    level: 'daily',
    period: '',
    view: 'detail',       // detail | list | review
    goal: null,
    goals: [],
    parentGoal: null,
    parentCandidates: [],
    review: null,
    loading: false,
  },

  _listeners: {},

  on(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
    return () => this.off(event, fn);
  },

  off(event, fn) {
    const arr = this._listeners[event];
    if (arr) this._listeners[event] = arr.filter(f => f !== fn);
  },

  dispatch(event, data) {
    const arr = this._listeners[event];
    if (arr) arr.forEach(fn => fn(data));
  },

  setState(partial) {
    Object.assign(this.state, partial);
    this.dispatch('state:changed', this.state);
  },
};

export { store };
```

- [ ] **Step 2: 创建 api.js**

```javascript
// statics/js/goal/api.js
const api = {
  async _fetch(url, options = {}) {
    const res = await fetch(url, options);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  },

  _post(url, body) {
    return this._fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  },

  getGoal(level, period) {
    return this._fetch(`/api/goal?level=${level}&period=${period}&nav=true`);
  },

  saveGoal(level, period, overview, status, parentId) {
    const body = { level, period };
    if (overview !== undefined) body.overview = overview;
    if (status !== undefined) body.status = status;
    if (parentId !== undefined) body.parent_id = parentId;
    return this._post('/api/goal/save', body);
  },

  addTask(level, period, task) {
    return this._post('/api/goal/task', { level, period, task });
  },

  updateTask(level, period, taskId, task) {
    return this._post('/api/goal/task/update', { level, period, task_id: taskId, task });
  },

  deleteTask(level, period, taskId) {
    return this._post('/api/goal/task/delete', { level, period, task_id: taskId });
  },

  deleteGoal(level, period) {
    return this._post('/api/goal/delete', { level, period });
  },

  getParentGoals(level, period) {
    return this._fetch(`/api/goal/parent?level=${level}&period=${period}`);
  },

  addTaskNote(level, period, taskId, content) {
    return this._post('/api/goal/task/note', { level, period, task_id: taskId, content });
  },

  getReview(level, period) {
    return this._fetch(`/api/goal/review?level=${level}&period=${period}`);
  },

  saveReview(level, period, content) {
    return this._post('/api/goal/review/save', { level, period, content });
  },

  generateReview(level, period) {
    return this._post('/api/goal/review/generate', { level, period });
  },

  getCurrentGoals() {
    return this._fetch('/api/goals/current');
  },

  listGoals(level, year = '') {
    return this._fetch(`/api/goals?level=${level}&year=${year}`);
  },
};

export { api };
```

- [ ] **Step 3: 创建 utils.js**

```javascript
// statics/js/goal/utils.js
const LEVELS = ['daily', 'weekly', 'monthly', 'yearly'];
const LEVEL_LABELS = { daily: '日目标', weekly: '周目标', monthly: '月目标', yearly: '年目标' };
const PRIORITY_LABELS = { high: '高优', medium: '普通', low: '低优' };
const STATUS_LABELS = { pending: '待开始', in_progress: '进行中', completed: '已完成', cancelled: '已取消' };
const PARENT_LEVEL = { daily: 'weekly', weekly: 'monthly', monthly: 'yearly', yearly: null };

function today() {
  return new Date().toISOString().split('T')[0];
}

function currentPeriod(level) {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  const d = String(now.getDate()).padStart(2, '0');

  switch (level) {
    case 'daily': return `${y}-${m}-${d}`;
    case 'weekly':
      const jan1 = new Date(y, 0, 1);
      const week = Math.ceil(((now - jan1) / 86400000 + jan1.getDay() + 1) / 7);
      return `${y}-W${String(week).padStart(2, '0')}`;
    case 'monthly': return `${y}-${m}`;
    case 'yearly': return `${y}`;
  }
}

function periodLabel(level, period) {
  if (!period) return '';
  switch (level) {
    case 'daily': {
      const d = new Date(period);
      const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
      return `${period} ${days[d.getDay()]}`;
    }
    case 'weekly': {
      const [y, w] = period.split('-W');
      return `${y}年第${parseInt(w)}周`;
    }
    case 'monthly': {
      const [y, m] = period.split('-');
      return `${y}年${parseInt(m)}月`;
    }
    case 'yearly': return `${period}年`;
    default: return period;
  }
}

export { LEVELS, LEVEL_LABELS, PRIORITY_LABELS, STATUS_LABELS, PARENT_LEVEL, today, currentPeriod, periodLabel };
```

- [ ] **Step 4: 创建 goal-app.js 根组件**

```javascript
// statics/js/goal/components/goal-app.js
import { store } from '../store.js';
import { api } from '../api.js';
import { currentPeriod } from '../utils.js';
import './goal-tabs.js';
import './period-nav.js';

class GoalApp extends HTMLElement {
  constructor() {
    super();
    this.unsubs = [];
  }

  connectedCallback() {
    this.innerHTML = `
      <div class="goal-container">
        <goal-tabs></goal-tabs>
        <period-nav></period-nav>
        <div id="goal-view" class="goal-view"></div>
      </div>
    `;

    this.unsubs.push(
      store.on('level:changed', () => this.loadGoal()),
      store.on('period:changed', () => this.loadGoal()),
      store.on('view:changed', () => this.renderView()),
    );

    // 初始化
    const period = currentPeriod(store.state.level);
    store.setState({ period });
    this.loadGoal();
  }

  disconnectedCallback() {
    this.unsubs.forEach(fn => fn());
  }

  async loadGoal() {
    store.setState({ loading: true });
    try {
      const res = await api.getGoal(store.state.level, store.state.period);
      if (res.success) {
        store.setState({ goal: res.data });
        if (res.nav) store.setState({ nav: res.nav });
        // 如果有 parent_id，加载父目标
        if (res.data.parent_id) {
          this.loadParentGoal(res.data.parent_id);
        } else {
          store.setState({ parentGoal: null });
        }
      }
    } catch (e) {
      console.error('Failed to load goal:', e);
    } finally {
      store.setState({ loading: false });
    }
  }

  async loadParentGoal(parentId) {
    // parentId 是 "level|period" 格式或直接的 goal ID
    // 遍历查找
    try {
      const res = await api.listGoals(store.state.parentLevel || 'monthly', '');
      if (res.success && res.data) {
        const parent = res.data.find(g =>
          `${g.level}|${g.period}` === parentId || g.period === parentId
        );
        store.setState({ parentGoal: parent || null });
      }
    } catch (e) {
      store.setState({ parentGoal: null });
    }
  }

  renderView() {
    const viewEl = this.querySelector('#goal-view');
    if (!viewEl) return;
    switch (store.state.view) {
      case 'detail':
        viewEl.innerHTML = '<goal-detail></goal-detail>';
        break;
      case 'list':
        viewEl.innerHTML = '<goal-list></goal-list>';
        break;
      case 'review':
        viewEl.innerHTML = '<review-panel></review-panel>';
        break;
    }
  }
}

customElements.define('goal-app', GoalApp);
```

- [ ] **Step 5: 创建 goal-tabs.js**

```javascript
// statics/js/goal/components/goal-tabs.js
import { store } from '../store.js';
import { LEVELS, LEVEL_LABELS } from '../utils.js';

class GoalTabs extends HTMLElement {
  connectedCallback() {
    this.render();
    this._unsub = store.on('state:changed', () => this.render());
  }

  disconnectedCallback() {
    if (this._unsub) this._unsub();
  }

  render() {
    const { level, view } = store.state;
    const tabsHtml = LEVELS.map(l =>
      `<button class="goal-tab ${l === level ? 'active' : ''}" data-level="${l}">${LEVEL_LABELS[l]}</button>`
    ).join('');

    this.innerHTML = `
      <div class="goal-tabs">
        <div class="goal-tabs-nav">${tabsHtml}</div>
        <div class="goal-view-toggle">
          <button class="view-btn ${view === 'detail' ? 'active' : ''}" data-view="detail">详情</button>
          <button class="view-btn ${view === 'list' ? 'active' : ''}" data-view="list">列表</button>
          <button class="view-btn ${view === 'review' ? 'active' : ''}" data-view="review">回顾</button>
        </div>
      </div>
    `;

    this.querySelectorAll('.goal-tab').forEach(btn =>
      btn.addEventListener('click', () => {
        store.setState({ level: btn.dataset.level });
        store.dispatch('level:changed', btn.dataset.level);
      })
    );

    this.querySelectorAll('.view-btn').forEach(btn =>
      btn.addEventListener('click', () => {
        store.setState({ view: btn.dataset.view });
        store.dispatch('view:changed', btn.dataset.view);
      })
    );
  }
}

customElements.define('goal-tabs', GoalTabs);
```

- [ ] **Step 6: 创建 period-nav.js**

```javascript
// statics/js/goal/components/period-nav.js
import { store } from '../store.js';
import { today, currentPeriod, periodLabel } from '../utils.js';

class PeriodNav extends HTMLElement {
  connectedCallback() {
    this.render();
    this._unsub = store.on('state:changed', () => this.render());
  }

  disconnectedCallback() {
    if (this._unsub) this._unsub();
  }

  render() {
    const { level, period, nav, loading } = store.state;
    const label = periodLabel(level, period);

    this.innerHTML = `
      <div class="period-nav">
        <button class="period-btn" data-action="prev" ${!nav ? 'disabled' : ''}>
          <i class="fas fa-chevron-left"></i>
        </button>
        <span class="period-label">${label || period || ''}</span>
        <button class="period-btn" data-action="next" ${!nav ? 'disabled' : ''}>
          <i class="fas fa-chevron-right"></i>
        </button>
        <button class="period-btn period-today" data-action="today">今天</button>
        ${loading ? '<span class="period-loading">加载中...</span>' : ''}
      </div>
    `;

    this.querySelector('[data-action="prev"]')?.addEventListener('click', () => {
      if (!nav) return;
      store.setState({ period: nav.prev });
      store.dispatch('period:changed', nav.prev);
    });

    this.querySelector('[data-action="next"]')?.addEventListener('click', () => {
      if (!nav) return;
      store.setState({ period: nav.next });
      store.dispatch('period:changed', nav.next);
    });

    this.querySelector('[data-action="today"]')?.addEventListener('click', () => {
      const p = currentPeriod(level);
      store.setState({ period: p });
      store.dispatch('period:changed', p);
    });
  }
}

customElements.define('period-nav', PeriodNav);
```

- [ ] **Step 7: 创建 goal.css 设计系统**

```css
/* statics/css/goal.css */
:root {
  --goal-bg: #0f0f1a;
  --goal-card: #1e1e2e;
  --goal-card-hover: #252538;
  --goal-accent: #4f46e5;
  --goal-accent-hover: #4338ca;
  --goal-text: #e4e4ed;
  --goal-text-muted: #8b8ba0;
  --goal-border: #2a2a3e;
  --goal-success: #22c55e;
  --goal-warning: #f59e0b;
  --goal-danger: #ef4444;
  --goal-radius: 8px;
  --goal-transition: 0.2s ease;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
  background: var(--goal-bg);
  color: var(--goal-text);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  line-height: 1.6;
  min-height: 100vh;
}

.goal-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px 16px;
}

/* Tabs */
.goal-tabs {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.goal-tabs-nav {
  display: flex;
  gap: 4px;
  background: var(--goal-card);
  border-radius: var(--goal-radius);
  padding: 4px;
}

.goal-tab {
  padding: 8px 16px;
  border: none;
  background: transparent;
  color: var(--goal-text-muted);
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: all var(--goal-transition);
}
.goal-tab:hover { color: var(--goal-text); }
.goal-tab.active {
  background: var(--goal-accent);
  color: #fff;
}

.goal-view-toggle {
  display: flex;
  gap: 4px;
  background: var(--goal-card);
  border-radius: var(--goal-radius);
  padding: 4px;
}
.view-btn {
  padding: 6px 12px;
  border: none;
  background: transparent;
  color: var(--goal-text-muted);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: all var(--goal-transition);
}
.view-btn.active { background: var(--goal-border); color: var(--goal-text); }

/* Period Nav */
.period-nav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-bottom: 20px;
}
.period-label {
  font-size: 16px;
  font-weight: 600;
  min-width: 150px;
  text-align: center;
}
.period-btn {
  width: 36px; height: 36px;
  border: 1px solid var(--goal-border);
  background: var(--goal-card);
  color: var(--goal-text);
  border-radius: 50%;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--goal-transition);
}
.period-btn:hover:not(:disabled) { background: var(--goal-card-hover); }
.period-btn:disabled { opacity: 0.3; cursor: default; }
.period-today {
  width: auto;
  border-radius: 20px;
  padding: 0 12px;
  font-size: 13px;
}
.period-loading {
  font-size: 12px;
  color: var(--goal-text-muted);
  animation: pulse 1.5s infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* Card */
.goal-card {
  background: var(--goal-card);
  border-radius: var(--goal-radius);
  padding: 20px;
  margin-bottom: 16px;
  border: 1px solid var(--goal-border);
}

/* 响应式 */
@media (max-width: 640px) {
  .goal-container { padding: 16px 12px; }
  .goal-tabs { flex-direction: column; }
}
```

- [ ] **Step 8: 精简 goal.template 为 HTML 骨架**

替换现存的 `templates/goal.template`（当前有约 200+ 行内联 HTML/CSS/JS）：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>目标管理 - GUCCANG</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <link rel="stylesheet" href="/css/goal.css">
</head>
<body>
    <goal-app></goal-app>
    <script type="module" src="/js/goal/components/goal-app.js"></script>
</body>
</html>
```

- [ ] **Step 9: 验证** — 启动服务器，访问 `/goal`，确认骨架渲染（tabs + period-nav 可见，但详情/列表/回顾视图为空）。

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 10: Commit**

```bash
git add statics/js/goal/ statics/css/goal.css templates/goal.template
git commit -m "feat(goal): 前端骨架 - store/api/utils + tabs/period-nav Web Components"
```

---

### Task 5: 详情视图 — goal-detail, goal-overview, task-list, task-item, task-editor

**Files:**
- Create: `statics/js/goal/components/goal-detail.js`
- Create: `statics/js/goal/components/goal-overview.js`
- Create: `statics/js/goal/components/task-list.js`
- Create: `statics/js/goal/components/task-item.js`
- Create: `statics/js/goal/components/task-editor.js`
- Modify: `statics/css/goal.css` — 添加详情视图样式

**Interfaces:**
- Consumes: Task 4 (store, api, utils, goal-app)
- Produces: `<goal-detail>`, `<goal-overview>`, `<task-list>`, `<task-item>`, `<task-editor>`

- [ ] **Step 1: 创建 goal-overview.js**

```javascript
// statics/js/goal/components/goal-overview.js
import { store } from '../store.js';
import { api } from '../api.js';

class GoalOverview extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { goal, parentGoal } = store.state;
    if (!goal) {
      this.innerHTML = '<div class="goal-card"><p style="color:var(--goal-text-muted)">加载中...</p></div>';
      return;
    }

    const statusClass = goal.status === 'completed' ? 'status-done' : 'status-active';
    const statusText = goal.status === 'completed' ? '已完成' : '进行中';

    this.innerHTML = `
      <div class="goal-card">
        ${parentGoal ? `
          <div class="parent-breadcrumb">
            <i class="fas fa-link"></i>
            <span>对齐到: ${parentGoal.overview || parentGoal.period}</span>
          </div>
        ` : ''}
        <div class="goal-header">
          <span class="goal-status ${statusClass}">${statusText}</span>
          <button class="btn-sm btn-toggle" data-action="toggle">${goal.status === 'completed' ? '重新开始' : '标记完成'}</button>
        </div>
        <textarea class="overview-input" placeholder="写一下你的目标概述..." rows="3">${escapeHtml(goal.overview || '')}</textarea>
        <div class="overview-actions">
          <button class="btn-sm btn-primary" data-action="save">保存概述</button>
          <span class="progress-text">进度 ${goal.progress || 0}%</span>
        </div>
        <div class="progress-bar">
          <div class="progress-fill" style="width:${goal.progress || 0}%"></div>
        </div>
      </div>
    `;

    this.querySelector('[data-action="toggle"]')?.addEventListener('click', () => {
      const newStatus = goal.status === 'completed' ? 'active' : 'completed';
      api.saveGoal(goal.level, goal.period, undefined, newStatus).then(() => {
        store.dispatch('level:changed', goal.level);
      });
    });

    this.querySelector('[data-action="save"]')?.addEventListener('click', () => {
      const textarea = this.querySelector('.overview-input');
      api.saveGoal(goal.level, goal.period, textarea.value).then(() => {
        store.dispatch('level:changed', goal.level);
      });
    });
  }
}

function escapeHtml(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

customElements.define('goal-overview', GoalOverview);
```

- [ ] **Step 2: 创建 task-editor.js（任务编辑弹窗）**

```javascript
// statics/js/goal/components/task-editor.js
import { store } from '../store.js';
import { api } from '../api.js';
import { PRIORITY_LABELS, STATUS_LABELS } from '../utils.js';

class TaskEditor extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { editTask } = store.state;
    if (!editTask) { this.innerHTML = ''; return; }

    const task = editTask || {};
    this.innerHTML = `
      <div class="modal-overlay" data-action="close">
        <div class="modal-content">
          <h3>编辑任务</h3>
          <div class="form-group">
            <label>标题</label>
            <input type="text" class="field-title" value="${escapeHtml(task.title || '')}">
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>优先级</label>
              <select class="field-priority">
                ${Object.entries(PRIORITY_LABELS).map(([k, v]) =>
                  `<option value="${k}" ${task.priority === k ? 'selected' : ''}>${v}</option>`
                ).join('')}
              </select>
            </div>
            <div class="form-group">
              <label>状态</label>
              <select class="field-status">
                ${Object.entries(STATUS_LABELS).map(([k, v]) =>
                  `<option value="${k}" ${task.status === k ? 'selected' : ''}>${v}</option>`
                ).join('')}
              </select>
            </div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>截止日期</label>
              <input type="date" class="field-deadline" value="${task.deadline || ''}">
            </div>
            <div class="form-group">
              <label>预估耗时 (小时)</label>
              <input type="number" class="field-estimate" value="${task.estimate_hours || ''}" step="0.5" min="0">
            </div>
          </div>
          <div class="form-group">
            <label>描述</label>
            <textarea class="field-desc" rows="2">${escapeHtml(task.description || '')}</textarea>
          </div>
          <div class="modal-actions">
            <button class="btn-sm" data-action="close">取消</button>
            <button class="btn-sm btn-primary" data-action="save">保存</button>
          </div>
        </div>
      </div>
    `;

    this.querySelector('[data-action="close"]')?.addEventListener('click', (e) => {
      if (e.target.dataset.action === 'close') store.setState({ editTask: null });
    });
    this.querySelector('[data-action="save"]')?.addEventListener('click', () => this.save());
  }

  save() {
    const { goal, editTask } = store.state;
    if (!goal || !editTask) return;

    const updated = {
      ...editTask,
      title: this.querySelector('.field-title').value,
      priority: this.querySelector('.field-priority').value,
      status: this.querySelector('.field-status').value,
      deadline: this.querySelector('.field-deadline').value,
      estimate_hours: parseFloat(this.querySelector('.field-estimate').value) || 0,
      description: this.querySelector('.field-desc').value,
    };

    api.updateTask(goal.level, goal.period, editTask.id, updated).then(() => {
      store.setState({ editTask: null });
      store.dispatch('level:changed', goal.level);
    });
  }
}

function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }

customElements.define('task-editor', TaskEditor);
```

- [ ] **Step 3: 创建 task-item.js（含子任务展开/备注日志）**

```javascript
// statics/js/goal/components/task-item.js
import { store } from '../store.js';
import { api } from '../api.js';
import { PRIORITY_LABELS } from '../utils.js';

class TaskItem extends HTMLElement {
  set task(value) { this._task = value; this.render(); }
  get task() { return this._task; }

  render() {
    const t = this._task;
    if (!t) return;
    const checked = t.status === 'completed' ? 'checked' : '';
    const doneClass = t.status === 'completed' ? 'task-done' : '';

    this.innerHTML = `
      <div class="task-item ${doneClass}">
        <div class="task-main">
          <input type="checkbox" class="task-check" ${checked} data-action="toggle">
          <span class="task-title" data-action="edit-title">${escapeHtml(t.title)}</span>
          ${t.deadline ? `<span class="task-deadline">📅${t.deadline}</span>` : ''}
          ${t.estimate_hours ? `<span class="task-estimate">⏱${t.estimate_hours}h</span>` : ''}
          <span class="task-priority priority-${t.priority}">${PRIORITY_LABELS[t.priority] || ''}</span>
          <span class="task-status-label">${t.status === 'in_progress' ? '进行中' : ''}${t.status === 'cancelled' ? '已取消' : ''}</span>
          <button class="btn-icon-sm" data-action="expand">${this._expanded ? '▲' : '▼'}</button>
          <button class="btn-icon-sm" data-action="edit">✏</button>
          <button class="btn-icon-sm btn-danger" data-action="delete">✕</button>
        </div>
        ${this._expanded ? this._renderExpanded() : ''}
      </div>
    `;

    this.querySelector('[data-action="toggle"]')?.addEventListener('change', (e) => {
      const { goal } = store.state;
      const updated = { ...this._task, status: e.target.checked ? 'completed' : 'pending' };
      api.updateTask(goal.level, goal.period, this._task.id, updated).then(() => {
        store.dispatch('level:changed', goal.level);
      });
    });

    this.querySelector('[data-action="edit-title"]')?.addEventListener('dblclick', () => this._inlineEdit());
    this.querySelector('[data-action="expand"]')?.addEventListener('click', () => {
      this._expanded = !this._expanded;
      this.render();
    });
    this.querySelector('[data-action="edit"]')?.addEventListener('click', () => {
      store.setState({ editTask: this._task });
    });
    this.querySelector('[data-action="delete"]')?.addEventListener('click', () => {
      const { goal } = store.state;
      if (confirm('确定删除该任务？')) {
        api.deleteTask(goal.level, goal.period, this._task.id).then(() => {
          store.dispatch('level:changed', goal.level);
        });
      }
    });

    // 子任务事件
    this.querySelectorAll('.subtask-check').forEach(cb =>
      cb.addEventListener('change', (e) => this._toggleSubtask(e))
    );
    this.querySelector('[data-action="add-subtask"]')?.addEventListener('click', () => this._addSubtask());
    this.querySelector('[data-action="add-note"]')?.addEventListener('click', () => this._addNote());
  }

  _renderExpanded() {
    const subtasks = this._task.subtasks || [];
    const notes = this._task.notes || [];
    return `
      <div class="task-expanded">
        <div class="subtask-section">
          <div class="subtask-list">
            ${subtasks.map(s => `
              <div class="subtask-item">
                <input type="checkbox" class="subtask-check" data-id="${s.id}" ${s.status === 'completed' ? 'checked' : ''}>
                <span class="${s.status === 'completed' ? 'line-through' : ''}">${escapeHtml(s.title)}</span>
              </div>
            `).join('')}
          </div>
          <div class="subtask-add">
            <input type="text" class="subtask-input" placeholder="添加子任务...">
            <button class="btn-sm" data-action="add-subtask">添加</button>
          </div>
        </div>
        <div class="note-section">
          <div class="note-list">
            ${notes.slice().reverse().map(n => `
              <div class="note-item">
                <span class="note-date">${n.created_at?.split(' ')[0] || ''}</span>
                <span class="note-content">${escapeHtml(n.content)}</span>
              </div>
            `).join('')}
          </div>
          <div class="note-add">
            <input type="text" class="note-input" placeholder="添加备注...">
            <button class="btn-sm" data-action="add-note">添加</button>
          </div>
        </div>
      </div>
    `;
  }

  _inlineEdit() {
    const titleEl = this.querySelector('.task-title');
    const old = titleEl.textContent;
    titleEl.innerHTML = `<input type="text" class="inline-edit" value="${escapeHtml(old)}">`;
    const input = titleEl.querySelector('input');
    input.focus();
    input.select();
    const save = () => {
      const val = input.value.trim();
      if (val && val !== old) {
        const { goal } = store.state;
        const updated = { ...this._task, title: val };
        api.updateTask(goal.level, goal.period, this._task.id, updated).then(() => {
          store.dispatch('level:changed', goal.level);
        });
      } else {
        this.render();
      }
    };
    input.addEventListener('blur', save);
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter') save(); if (e.key === 'Escape') this.render(); });
  }

  async _toggleSubtask(e) {
    const { goal } = store.state;
    const sid = e.target.dataset.id;
    const subtasks = (this._task.subtasks || []).map(s =>
      s.id === sid ? { ...s, status: e.target.checked ? 'completed' : 'pending' } : s
    );
    const updated = { ...this._task, subtasks };
    await api.updateTask(goal.level, goal.period, this._task.id, updated);
    store.dispatch('level:changed', goal.level);
  }

  async _addSubtask() {
    const input = this.querySelector('.subtask-input');
    const val = input.value.trim();
    if (!val) return;
    const { goal } = store.state;
    const subtasks = [...(this._task.subtasks || []), {
      id: Date.now().toString(),
      title: val,
      status: 'pending',
    }];
    const updated = { ...this._task, subtasks };
    await api.updateTask(goal.level, goal.period, this._task.id, updated);
    store.dispatch('level:changed', goal.level);
  }

  async _addNote() {
    const input = this.querySelector('.note-input');
    const val = input.value.trim();
    if (!val) return;
    const { goal } = store.state;
    await api.addTaskNote(goal.level, goal.period, this._task.id, val);
    store.dispatch('level:changed', goal.level);
  }
}

function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s || ''; return d.innerHTML; }

customElements.define('task-item', TaskItem);
```

- [ ] **Step 4: 创建 task-list.js**

```javascript
// statics/js/goal/components/task-list.js
import { store } from '../store.js';
import { api } from '../api.js';
import './task-item.js';

class TaskList extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.render();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  render() {
    const { goal } = store.state;
    if (!goal) { this.innerHTML = ''; return; }

    const tasks = goal.tasks || [];
    this.innerHTML = `
      <div class="goal-card">
        <h3 class="section-title">任务 (${tasks.length})</h3>
        <div class="task-list" id="taskListContainer"></div>
        <div class="task-add-row">
          <input type="text" class="task-add-input" placeholder="添加新任务..." id="newTaskTitle">
          <select class="task-add-priority" id="newTaskPriority">
            <option value="medium">普通</option>
            <option value="high">高优</option>
            <option value="low">低优</option>
          </select>
          <button class="btn-sm btn-primary" id="addTaskBtn">添加</button>
        </div>
        <div class="task-actions">
          <button class="btn-sm btn-danger" data-action="delete-goal">删除目标</button>
        </div>
      </div>
    `;

    const container = this.querySelector('#taskListContainer');
    tasks.forEach(t => {
      const item = document.createElement('task-item');
      item.task = t;
      container.appendChild(item);
    });

    this.querySelector('#addTaskBtn')?.addEventListener('click', () => this._addTask());
    this.querySelector('#newTaskTitle')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this._addTask();
    });
    this.querySelector('[data-action="delete-goal"]')?.addEventListener('click', () => {
      if (confirm(`确定删除该目标及其所有任务？此操作不可恢复。`)) {
        api.deleteGoal(goal.level, goal.period).then(() => {
          store.dispatch('level:changed', goal.level);
        });
      }
    });
  }

  async _addTask() {
    const input = this.querySelector('#newTaskTitle');
    const title = input.value.trim();
    if (!title) return;
    const priority = this.querySelector('#newTaskPriority').value;
    const { goal } = store.state;
    await api.addTask(goal.level, goal.period, {
      title,
      priority,
      status: 'pending',
      subtasks: [],
      notes: [],
    });
    input.value = '';
    store.dispatch('level:changed', goal.level);
  }
}

customElements.define('task-list', TaskList);
```

- [ ] **Step 5: 创建 goal-detail.js（组合 overview + task-list）**

```javascript
// statics/js/goal/components/goal-detail.js
import './goal-overview.js';
import './task-list.js';
import './task-editor.js';

class GoalDetail extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `
      <goal-overview></goal-overview>
      <task-list></task-list>
      <task-editor></task-editor>
    `;
  }
}

customElements.define('goal-detail', GoalDetail);
```

- [ ] **Step 6: 在 goal-app.js 中注册详情视图的 import**

更新 `goal-app.js` 的 `renderView()` 方法中的 detail 分支——当 `view === 'detail'` 时，需要先确保 `goal-detail.js` 已加载。在文件顶部添加：

```javascript
import './goal-detail.js';
```

- [ ] **Step 7: 追加 goal.css 详情视图样式**

在 `goal.css` 末尾追加：

```css
/* Goal Overview */
.goal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.goal-status { font-size: 12px; padding: 2px 10px; border-radius: 12px; }
.status-active { background: var(--goal-accent); color: #fff; }
.status-done { background: var(--goal-success); color: #fff; }
.parent-breadcrumb {
  font-size: 13px;
  color: var(--goal-text-muted);
  margin-bottom: 8px;
  padding: 6px 10px;
  background: var(--goal-border);
  border-radius: 4px;
}
.overview-input {
  width: 100%;
  background: var(--goal-bg);
  border: 1px solid var(--goal-border);
  color: var(--goal-text);
  border-radius: 6px;
  padding: 10px;
  font-size: 14px;
  resize: vertical;
  font-family: inherit;
}
.overview-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
}
.progress-text { font-size: 13px; color: var(--goal-text-muted); }
.progress-bar {
  height: 4px;
  background: var(--goal-border);
  border-radius: 2px;
  margin-top: 12px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: var(--goal-accent);
  border-radius: 2px;
  transition: width 0.3s ease;
}

/* Buttons */
.btn-sm {
  padding: 6px 14px;
  border: 1px solid var(--goal-border);
  background: var(--goal-card);
  color: var(--goal-text);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: all var(--goal-transition);
}
.btn-sm:hover { background: var(--goal-card-hover); }
.btn-primary { background: var(--goal-accent); border-color: var(--goal-accent); color: #fff; }
.btn-primary:hover { background: var(--goal-accent-hover); }
.btn-danger { color: var(--goal-danger); border-color: transparent; }
.btn-danger:hover { background: rgba(239,68,68,0.1); }
.btn-icon-sm {
  background: none;
  border: none;
  color: var(--goal-text-muted);
  cursor: pointer;
  font-size: 14px;
  padding: 2px 4px;
}
.btn-icon-sm:hover { color: var(--goal-text); }

/* Task List */
.section-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
  color: var(--goal-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.task-item {
  border-bottom: 1px solid var(--goal-border);
  padding: 8px 0;
}
.task-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.task-check { accent-color: var(--goal-accent); }
.task-title { flex: 1; font-size: 14px; cursor: pointer; }
.task-done .task-title { text-decoration: line-through; color: var(--goal-text-muted); }
.task-deadline, .task-estimate {
  font-size: 11px;
  color: var(--goal-text-muted);
}
.task-priority {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 10px;
}
.priority-high { background: rgba(239,68,68,0.15); color: var(--goal-danger); }
.priority-medium { background: rgba(245,158,11,0.15); color: var(--goal-warning); }
.priority-low { background: rgba(139,139,160,0.15); color: var(--goal-text-muted); }
.task-status-label {
  font-size: 10px;
  color: var(--goal-text-muted);
}

/* Subtasks */
.task-expanded {
  margin-left: 24px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--goal-border);
}
.subtask-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
  font-size: 13px;
}
.subtask-item .line-through { text-decoration: line-through; color: var(--goal-text-muted); }
.subtask-add, .note-add {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}
.subtask-input, .note-input {
  flex: 1;
  background: var(--goal-bg);
  border: 1px solid var(--goal-border);
  color: var(--goal-text);
  border-radius: 4px;
  padding: 4px 8px;
  font-size: 12px;
}

/* Notes */
.note-section { margin-top: 12px; }
.note-item {
  display: flex;
  gap: 8px;
  padding: 4px 0;
  font-size: 12px;
}
.note-date { color: var(--goal-text-muted); white-space: nowrap; }
.note-content { color: var(--goal-text); }

/* Task Add Row */
.task-add-row {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed var(--goal-border);
}
.task-add-input {
  flex: 1;
  background: var(--goal-bg);
  border: 1px solid var(--goal-border);
  color: var(--goal-text);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 14px;
}
.task-add-input:focus { outline: none; border-color: var(--goal-accent); }
.task-add-priority {
  background: var(--goal-bg);
  border: 1px solid var(--goal-border);
  color: var(--goal-text);
  border-radius: 6px;
  padding: 8px;
  font-size: 13px;
}
.task-actions { margin-top: 12px; display: flex; justify-content: flex-end; }

/* Modal */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal-content {
  background: var(--goal-card);
  border: 1px solid var(--goal-border);
  border-radius: 12px;
  padding: 24px;
  width: 90%;
  max-width: 480px;
  max-height: 80vh;
  overflow-y: auto;
}
.modal-content h3 { margin-bottom: 16px; }
.form-group { margin-bottom: 12px; }
.form-group label {
  display: block;
  font-size: 12px;
  color: var(--goal-text-muted);
  margin-bottom: 4px;
}
.form-group input, .form-group select, .form-group textarea {
  width: 100%;
  background: var(--goal-bg);
  border: 1px solid var(--goal-border);
  color: var(--goal-text);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 14px;
  font-family: inherit;
}
.form-group input:focus, .form-group select:focus, .form-group textarea:focus {
  outline: none;
  border-color: var(--goal-accent);
}
.form-row { display: flex; gap: 12px; }
.form-row .form-group { flex: 1; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }

/* Inline edit */
.inline-edit {
  background: var(--goal-bg);
  border: 1px solid var(--goal-accent);
  color: var(--goal-text);
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 14px;
  width: 100%;
}
```

- [ ] **Step 8: 构建验证 + 手动测试**

启动服务器，访问 `/goal`，验证：
- 详情视图加载目标数据
- 概述文本编辑保存
- 添加任务成功
- 任务切换完成状态
- 子任务 check/uncheck
- 添加备注
- 任务编辑器弹窗

- [ ] **Step 9: Commit**

```bash
git add statics/js/goal/components/goal-detail.js statics/js/goal/components/goal-overview.js statics/js/goal/components/task-list.js statics/js/goal/components/task-item.js statics/js/goal/components/task-editor.js statics/css/goal.css statics/js/goal/components/goal-app.js
git commit -m "feat(goal): 详情视图组件 - overview/task-list/task-item/task-editor"
```

---

### Task 6: OKR 对齐功能

**Files:**
- Modify: `statics/js/goal/components/goal-overview.js` — 添加父目标选择器
- Modify: `statics/css/goal.css` — 添加选择器样式

**Interfaces:**
- Consumes: Task 5 (详情视图)
- Produces: 父目标选择器 UI + 面包屑展示

- [ ] **Step 1: 在 goal-overview.js 添加父目标选择器**

在 `goal-overview.js` 的 `render()` 方法中，在顶部（parent-breadcrumb 区域）扩展：

在 `parentGoal` 渲染块的 else 分支添加选择器：

```javascript
// 在 render() 中找到 parent-breadcrumb 区域，修改为：
this.innerHTML = `
  <div class="goal-card">
    ${parentGoal ? `
      <div class="parent-breadcrumb">
        <i class="fas fa-link"></i>
        <span>对齐到: ${parentGoal.overview || parentGoal.period}</span>
        <button class="btn-icon-sm" data-action="clear-parent">✕</button>
      </div>
    ` : (store.state.level !== 'yearly' ? `
      <div class="parent-selector">
        <button class="btn-sm" data-action="show-parents">
          <i class="fas fa-link"></i> 对齐上层目标
        </button>
        <div class="parent-dropdown hidden" id="parentDropdown">
          <div class="parent-list"></div>
        </div>
      </div>
    ` : '')}
    ...
  </div>
`;
```

添加事件处理加载父目标候选列表：

```javascript
this.querySelector('[data-action="show-parents"]')?.addEventListener('click', async () => {
  const dropdown = this.querySelector('#parentDropdown');
  dropdown.classList.toggle('hidden');
  if (!dropdown.classList.contains('hidden')) {
    const res = await api.getParentGoals(goal.level, goal.period);
    if (res.success && res.data) {
      const list = dropdown.querySelector('.parent-list');
      list.innerHTML = res.data.map(g => `
        <div class="parent-option" data-id="${g.level}|${g.period}">
          <span class="parent-level">${g.level}</span>
          <span>${g.overview || g.period}</span>
        </div>
      `).join('');
      list.querySelectorAll('.parent-option').forEach(opt =>
        opt.addEventListener('click', () => {
          api.saveGoal(goal.level, goal.period, undefined, undefined, opt.dataset.id).then(() => {
            store.dispatch('level:changed', goal.level);
          });
        })
      );
    }
  }
});

this.querySelector('[data-action="clear-parent"]')?.addEventListener('click', () => {
  api.saveGoal(goal.level, goal.period, undefined, undefined, '').then(() => {
    store.dispatch('level:changed', goal.level);
  });
});
```

- [ ] **Step 2: 追加 CSS 样式**

```css
/* Parent selector */
.parent-selector { margin-bottom: 12px; position: relative; }
.parent-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--goal-card);
  border: 1px solid var(--goal-border);
  border-radius: var(--goal-radius);
  margin-top: 4px;
  z-index: 10;
  max-height: 200px;
  overflow-y: auto;
}
.parent-option {
  padding: 8px 12px;
  cursor: pointer;
  display: flex;
  gap: 8px;
  font-size: 13px;
}
.parent-option:hover { background: var(--goal-card-hover); }
.parent-level {
  font-size: 10px;
  background: var(--goal-accent);
  color: #fff;
  padding: 1px 6px;
  border-radius: 8px;
}
.hidden { display: none; }
```

- [ ] **Step 3: 构建验证**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 4: Commit**

```bash
git add statics/js/goal/components/goal-overview.js statics/css/goal.css
git commit -m "feat(goal): OKR 父目标对齐选择器"
```

---

### Task 7: 列表视图 — goal-list

**Files:**
- Create: `statics/js/goal/components/goal-list.js`
- Modify: `statics/css/goal.css` — 列表视图样式
- Modify: `statics/js/goal/components/goal-app.js` — 注册 goal-list import

**Interfaces:**
- Consumes: Task 4 (store, api), Task 2 (ListGoalsByLevel)
- Produces: `<goal-list>` — 目标摘要卡片网格

- [ ] **Step 1: 创建 goal-list.js**

```javascript
// statics/js/goal/components/goal-list.js
import { store } from '../store.js';
import { api } from '../api.js';
import { periodLabel } from '../utils.js';

class GoalList extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.loadGoals();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  async loadGoals() {
    const { level } = store.state;
    store.setState({ loading: true });
    try {
      const res = await api.listGoals(level, '');
      if (res.success) {
        store.setState({ goals: res.data || [] });
      }
    } catch (e) {
      console.error('Failed to load goals:', e);
    } finally {
      store.setState({ loading: false });
    }
  }

  render() {
    const { goals, level, loading } = store.state;

    if (loading) {
      this.innerHTML = '<div class="loading">加载中...</div>';
      return;
    }

    if (!goals.length) {
      this.innerHTML = '<div class="empty-state"><p>暂无目标</p></div>';
      return;
    }

    this.innerHTML = `
      <div class="goal-grid">
        ${goals.map(g => `
          <div class="goal-summary-card" data-period="${g.period}">
            <div class="card-header">
              <span class="card-period">${periodLabel(level, g.period)}</span>
              <span class="card-status status-${g.status}">${g.status === 'completed' ? '已完成' : '进行中'}</span>
            </div>
            <p class="card-overview">${escapeHtml(g.overview || '未设置概述')}</p>
            <div class="card-progress">
              <div class="progress-bar">
                <div class="progress-fill" style="width:${g.progress || 0}%"></div>
              </div>
              <span class="card-progress-text">${g.progress || 0}%</span>
            </div>
            <div class="card-stats">
              <span>${g.done_tasks}/${g.total_tasks} 任务</span>
              <span>${g.pending_tasks} 待办</span>
            </div>
          </div>
        `).join('')}
      </div>
    `;

    this.querySelectorAll('.goal-summary-card').forEach(card =>
      card.addEventListener('click', () => {
        store.setState({ period: card.dataset.period, view: 'detail' });
        store.dispatch('period:changed', card.dataset.period);
        store.dispatch('view:changed', 'detail');
      })
    );
  }
}

function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s || ''; return d.innerHTML; }

customElements.define('goal-list', GoalList);
```

- [ ] **Step 2: 更新 goal-app.js 导入**

在 goal-app.js 顶部添加：
```javascript
import './goal-list.js';
```

- [ ] **Step 3: 追加 CSS 样式**

```css
/* Goal List */
.goal-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px;
}
.goal-summary-card {
  background: var(--goal-card);
  border: 1px solid var(--goal-border);
  border-radius: var(--goal-radius);
  padding: 16px;
  cursor: pointer;
  transition: all var(--goal-transition);
}
.goal-summary-card:hover {
  border-color: var(--goal-accent);
  background: var(--goal-card-hover);
}
.card-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}
.card-period { font-weight: 600; font-size: 14px; }
.card-overview {
  font-size: 13px;
  color: var(--goal-text-muted);
  margin-bottom: 12px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.card-progress { display: flex; align-items: center; gap: 8px; }
.card-progress .progress-bar { flex: 1; }
.card-progress-text { font-size: 12px; color: var(--goal-text-muted); }
.card-stats {
  display: flex;
  justify-content: space-between;
  margin-top: 8px;
  font-size: 12px;
  color: var(--goal-text-muted);
}

.empty-state { text-align: center; padding: 40px; color: var(--goal-text-muted); }
.loading { text-align: center; padding: 40px; color: var(--goal-text-muted); }
```

- [ ] **Step 4: 构建验证**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 5: Commit**

```bash
git add statics/js/goal/components/goal-list.js statics/js/goal/components/goal-app.js statics/css/goal.css
git commit -m "feat(goal): 列表视图 - 目标摘要卡片网格"
```

---

### Task 8: 回顾系统 — review-panel + review-editor

**Files:**
- Create: `statics/js/goal/components/review-panel.js`
- Create: `statics/js/goal/components/review-editor.js`
- Modify: `statics/js/goal/components/goal-app.js` — 导入 review 组件
- Modify: `statics/css/goal.css` — 回顾视图样式

**Interfaces:**
- Consumes: Task 4 (store, api)
- Produces: `<review-panel>`, `<review-editor>`

- [ ] **Step 1: 创建 review-panel.js**

```javascript
// statics/js/goal/components/review-panel.js
import { store } from '../store.js';
import { api } from '../api.js';
import { periodLabel } from '../utils.js';
import './review-editor.js';

class ReviewPanel extends HTMLElement {
  connectedCallback() {
    this._unsub = store.on('state:changed', () => this.render());
    this.loadReview();
  }

  disconnectedCallback() { if (this._unsub) this._unsub(); }

  async loadReview() {
    const { level, period } = store.state;
    store.setState({ loading: true });
    try {
      const res = await api.getReview(level, period);
      store.setState({ review: res.data || null, loading: false });
    } catch (e) {
      store.setState({ review: null, loading: false });
    }
  }

  render() {
    const { review, level, period, loading } = store.state;

    if (loading) {
      this.innerHTML = '<div class="loading">加载中...</div>';
      return;
    }

    this.innerHTML = `
      <div class="goal-card">
        <div class="review-header">
          <h3>${periodLabel(level, period)} 回顾</h3>
          ${review ? `
            <button class="btn-sm" data-action="edit-review">编辑</button>
          ` : ''}
        </div>
        ${review ? `
          <div class="review-stats">
            <span>已完成 ${review.completed}/${review.total} 任务</span>
          </div>
          <div class="review-content markdown-body">${this._renderMarkdown(review.content || '')}</div>
        ` : `
          <div class="empty-state">
            <p>还没有回顾记录</p>
            <button class="btn-sm btn-primary" data-action="generate">生成回顾</button>
          </div>
        `}
      </div>
      <review-editor></review-editor>
    `;

    this.querySelector('[data-action="generate"]')?.addEventListener('click', async () => {
      store.setState({ loading: true });
      const res = await api.generateReview(level, period);
      if (res.success && res.data) {
        store.setState({ review: res.data, loading: false });
      } else {
        store.setState({ loading: false });
      }
    });

    this.querySelector('[data-action="edit-review"]')?.addEventListener('click', () => {
      const editor = this.querySelector('review-editor');
      editor.show();
    });
  }

  _renderMarkdown(text) {
    return text
      .replace(/^### (.*$)/gm, '<h4>$1</h4>')
      .replace(/^## (.*$)/gm, '<h3>$1</h3>')
      .replace(/^# (.*$)/gm, '<h2>$1</h2>')
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      .replace(/^- (.*$)/gm, '<li>$1</li>')
      .replace(/\n\n/g, '</p><p>')
      .replace(/\[x\]/g, '<span style="color:var(--goal-success)">✓</span>')
      .replace(/\[ \]/g, '<span style="color:var(--goal-text-muted)">○</span>');
  }
}

customElements.define('review-panel', ReviewPanel);
```

- [ ] **Step 2: 创建 review-editor.js（Markdown 编辑模态框）**

```javascript
// statics/js/goal/components/review-editor.js
import { store } from '../store.js';
import { api } from '../api.js';

class ReviewEditor extends HTMLElement {
  connectedCallback() {
    this._visible = false;
    this.render();
  }

  show() { this._visible = true; this.render(); }
  hide() { this._visible = false; this.render(); }

  render() {
    if (!this._visible) { this.innerHTML = ''; return; }
    const { review, level, period } = store.state;

    this.innerHTML = `
      <div class="modal-overlay" data-action="cancel">
        <div class="modal-content review-modal">
          <h3>编辑回顾</h3>
          <textarea class="review-textarea" rows="20">${escapeHtml((review && review.content) || '')}</textarea>
          <div class="modal-actions">
            <button class="btn-sm" data-action="cancel">取消</button>
            <button class="btn-sm btn-primary" data-action="save">保存回顾</button>
          </div>
        </div>
      </div>
    `;

    this.querySelector('[data-action="cancel"]')?.addEventListener('click', (e) => {
      if (e.target.dataset.action === 'cancel') this.hide();
    });
    this.querySelector('[data-action="save"]')?.addEventListener('click', async () => {
      const content = this.querySelector('.review-textarea').value;
      store.setState({ loading: true });
      await api.saveReview(level, period, content);
      const res = await api.getReview(level, period);
      store.setState({ review: res.data, loading: false });
      this.hide();
    });
  }
}

function escapeHtml(s) { const d = document.createElement('div'); d.textContent = s || ''; return d.innerHTML; }

customElements.define('review-editor', ReviewEditor);
```

- [ ] **Step 3: 更新 goal-app.js 导入**

```javascript
import './review-panel.js';
```

- [ ] **Step 4: 追加 CSS 样式**

```css
/* Review */
.review-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.review-header h3 { font-size: 16px; }
.review-stats {
  display: flex;
  gap: 16px;
  font-size: 13px;
  color: var(--goal-text-muted);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--goal-border);
}
.review-content {
  font-size: 14px;
  line-height: 1.8;
}
.review-content h2, .review-content h3, .review-content h4 {
  margin-top: 16px;
  margin-bottom: 8px;
}
.review-content li {
  margin-left: 16px;
  list-style: none;
}
.review-textarea {
  width: 100%;
  background: var(--goal-bg);
  border: 1px solid var(--goal-border);
  color: var(--goal-text);
  border-radius: 8px;
  padding: 12px;
  font-size: 14px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  resize: vertical;
  line-height: 1.6;
}
.review-textarea:focus { outline: none; border-color: var(--goal-accent); }
.review-modal { max-width: 640px; }
```

- [ ] **Step 5: 构建验证**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 6: Commit**

```bash
git add statics/js/goal/components/review-panel.js statics/js/goal/components/review-editor.js statics/js/goal/components/goal-app.js statics/css/goal.css
git commit -m "feat(goal): 回顾系统 - 自动生成 + markdown 编辑"
```

---

### Task 9: 交互打磨 + 移动端适配

**Files:**
- Modify: `statics/css/goal.css` — 完整响应式 + 交互动效
- Modify: `statics/js/goal/store.js` — 添加 toast 通知
- Modify: `statics/js/goal/components/goal-app.js` — toast 容器

- [ ] **Step 1: 在 store.js 添加 toast 通知**

```javascript
// 在 store 对象上添加:
showToast(message, type = 'info') {
  this.dispatch('toast:show', { message, type });
},
```

- [ ] **Step 2: 在 goal-app.js 添加 toast 容器和监听**

```javascript
// 在 connectedCallback 中添加:
const toastContainer = document.createElement('div');
toastContainer.className = 'toast-container';
this.appendChild(toastContainer);

this.unsubs.push(
  store.on('toast:show', ({ message, type }) => {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    toastContainer.appendChild(toast);
    requestAnimationFrame(() => toast.classList.add('show'));
    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 300);
    }, 2500);
  })
);
```

- [ ] **Step 3: 在 api.js 中添加自动 toast 错误提示**

在 `_fetch` 方法中：
```javascript
async _fetch(url, options = {}) {
  try {
    const res = await fetch(url, options);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    if (!data.success && data.message) {
      store.showToast(data.message, 'error');
    }
    return data;
  } catch (e) {
    store.showToast(e.message, 'error');
    throw e;
 }
},
```

- [ ] **Step 4: 追加完整响应式 + 动效 CSS**

```css
/* Toast */
.toast-container {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.toast {
  background: var(--goal-card);
  border: 1px solid var(--goal-border);
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 13px;
  opacity: 0;
  transform: translateY(10px);
  transition: all 0.3s ease;
}
.toast.show { opacity: 1; transform: translateY(0); }
.toast-error { border-color: var(--goal-danger); color: var(--goal-danger); }
.toast-success { border-color: var(--goal-success); color: var(--goal-success); }

/* Animations */
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(8px); }
  to   { opacity: 1; transform: translateY(0); }
}
.goal-card { animation: fadeIn 0.3s ease; }
.task-item { animation: fadeIn 0.2s ease; }

/* Focus visible */
button:focus-visible, input:focus-visible, select:focus-visible, textarea:focus-visible {
  outline: 2px solid var(--goal-accent);
  outline-offset: 2px;
}

/* Scrollbar */
::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--goal-border); border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: var(--goal-text-muted); }

/* Responsive */
@media (max-width: 640px) {
  .goal-container { padding: 12px 8px; }
  .goal-tabs { flex-direction: column; align-items: stretch; }
  .goal-tabs-nav { justify-content: center; }
  .goal-view-toggle { justify-content: center; }
  .goal-grid { grid-template-columns: 1fr; }
  .modal-content { width: 95%; padding: 16px; }
  .task-main { font-size: 13px; }
  .form-row { flex-direction: column; gap: 0; }
  .task-add-row { flex-wrap: wrap; }
}
```

- [ ] **Step 5: 构建验证**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功。

- [ ] **Step 6: Commit**

```bash
git add statics/css/goal.css statics/js/goal/store.js statics/js/goal/api.js statics/js/goal/components/goal-app.js
git commit -m "feat(goal): 交互打磨 - toast通知/动画/响应式"
```

---

### Task 10: 集成测试 + 清理旧 goal.js

**Files:**
- Delete: `statics/js/goal.js` (旧文件，已被模块化版本替代)
- Test: 手动验证所有功能

- [ ] **Step 1: 删除旧 goal.js**

```bash
rm statics/js/goal.js
```

- [ ] **Step 2: 确认 goal.template 只加载新的模块入口**

确认 `goal.template` 中没有引用旧的 `goal.js`。

- [ ] **Step 3: 构建验证**

```bash
cd /Users/guccang/github_repo/go_blog/cmd/blog-agent && go build ./...
```
Expected: 编译成功，无警告。

- [ ] **Step 4: 手动测试清单**

启动服务器后验证：
- [ ] `/goal` 页面正常加载，无 JS 控制台错误
- [ ] 四个层级标签页切换，数据正确加载
- [ ] 期间导航（prev/next/today）正常工作
- [ ] 详情视图：编辑概述、切换状态
- [ ] 任务：添加、切换完成、编辑、删除
- [ ] 子任务：展开、添加、勾选
- [ ] 备注：展开、添加
- [ ] OKR 对齐：选择父目标、面包屑展示、取消对齐
- [ ] 列表视图：卡片展示、点击跳转详情
- [ ] 回顾：生成回顾、编辑保存
- [ ] Toast 通知：错误/成功提示
- [ ] 移动端布局不崩溃（Chrome DevTools 模拟）

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(goal): 移除旧 goal.js，完成模块化迁移"
```
