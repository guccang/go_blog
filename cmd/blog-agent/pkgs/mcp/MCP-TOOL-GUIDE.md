# MCP 工具开发规范

本文档描述如何在 `go_blog` 项目中添加新的 MCP 工具，涵盖工具定义、回调注册、命名规范、返回值类型标注等全流程。

---

## 1. 架构概览

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        MCP 工具注册与调用链路                             │
│                                                                          │
│  innter_mcp.go          mcp.go              gateway.go       bridge.go   │
│  ┌────────────┐    ┌──────────────┐    ┌──────────────┐  ┌────────────┐  │
│  │ 工具定义    │    │ RegisterCall │    │ buildToolDefs│  │ Discover   │  │
│  │ (LLMTool)  │───▶│ Back()注册   │───▶│ → UAP ToolDef│─▶│ Tools()    │  │
│  │ +回调函数   │    │ 回调到map    │    │ → gateway注册│  │ → LLM工具  │  │
│  └────────────┘    └──────────────┘    └──────────────┘  └────────────┘  │
│                                                                          │
│  inner_xxx_tools.go                                                      │
│  ┌────────────┐                                                          │
│  │ 回调函数   │ ← 实际业务逻辑（调用 statistics/control 等模块）          │
│  └────────────┘                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### 关键文件

| 文件 | 职责 |
|------|------|
| `innter_mcp.go` | 工具定义中心：LLMTool 列表 + RegisterCallBack 注册 |
| `inner_blog_tools.go` | Blog 核心工具的回调函数实现 |
| `inner_todo_tools.go` | TodoList 模块回调函数 |
| `inner_exercise_tools.go` | Exercise 模块回调函数 |
| `inner_reading_tools.go` | Reading 模块回调函数 |
| `inner_yearplan_tasks_tools.go` | YearPlan/TaskBreakdown 模块回调函数 |
| `ai_tools.go` | AI 增强工具（跨模块智能）回调函数 |
| `web_fetch.go` | Web 抓取与搜索回调函数 |
| `mcp.go` | RegisterCallBack/CallInnerTools 核心机制 |

---

## 2. 添加新工具的步骤

### 步骤一：实现回调函数

在对应的 `inner_xxx_tools.go` 文件中（或新建文件），编写回调函数。

**函数签名必须为：**

```go
func 函数名(arguments map[string]interface{}) string
```

**规范：**
- 函数名格式：`Inner_blog_Raw<工具名>`（如 `Inner_blog_RawGetTodosByDate`）
- 使用 `getStringParam()` / `getIntParam()` / `getOptionalIntParam()` 安全提取参数
- 错误时返回 `errorJSON("错误描述")`
- 成功时返回工具结果字符串（纯文本或 JSON）

**示例：**

```go
// inner_todo_tools.go

func Inner_blog_RawGetTodosByDate(arguments map[string]interface{}) string {
    account, err := getStringParam(arguments, "account")
    if err != nil {
        return errorJSON(err.Error())
    }
    date, err := getStringParam(arguments, "date")
    if err != nil {
        return errorJSON(err.Error())
    }
    return statistics.RawGetTodosByDate(account, date)
}
```

**可选参数示例：**

```go
func Inner_blog_RawAddTodo(arguments map[string]interface{}) string {
    account, err := getStringParam(arguments, "account")
    if err != nil {
        return errorJSON(err.Error())
    }
    date, err := getStringParam(arguments, "date")
    if err != nil {
        return errorJSON(err.Error())
    }
    content, err := getStringParam(arguments, "content")
    if err != nil {
        return errorJSON(err.Error())
    }
    // 可选整数参数，带默认值
    hours := getOptionalIntParam(arguments, "hours", 0)
    minutes := getOptionalIntParam(arguments, "minutes", 0)
    urgency := getOptionalIntParam(arguments, "urgency", 2)
    return statistics.RawAddTodo(account, date, content, hours, minutes, urgency, 2)
}
```

### 步骤二：在 `RegisterInnerTools()` 中注册回调

在 `innter_mcp.go` 的 `RegisterInnerTools()` 函数中添加注册：

```go
// innter_mcp.go — RegisterInnerTools() 内

// 你的新模块工具
RegisterCallBack("RawMyNewTool", Inner_blog_RawMyNewTool)
```

**如果工具需要附加提示词**（工具结果后追加 prompt 引导 LLM 行为）：

```go
RegisterCallBack("RawCreateBlog", Inner_blog_RawCreateBlog)
RegisterCallBackPrompt("RawCreateBlog", "完成创建后返回博客链接格式为[title](/get?blogname=title)")
```

### 步骤三：添加 LLMTool 定义

在 `innter_mcp.go` 的 `GetInnerMCPTools()` 函数返回的 `tools` 列表中添加工具定义。

---

## 3. LLMTool 定义规范

### 3.1 结构

```go
{
    Type: "function",
    Function: LLMFunction{
        Name:        "Inner_blog.<回调名>",
        Description: "<工具描述>。返回<类型标注>",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "参数名": map[string]string{"type": "string", "description": "参数描述"},
                // 或数字类型
                "参数名": map[string]interface{}{"type": "number", "description": "参数描述"},
            },
            "required": []string{"必填参数1", "必填参数2"},
        },
    },
}
```

### 3.2 命名规范

| 项目 | 规范 | 示例 |
|------|------|------|
| **Name** | `Inner_blog.<回调名>` | `Inner_blog.RawGetTodosByDate` |
| **回调名** | `Raw` 前缀 + 动词 + 名词 | `RawGetExerciseByDate`、`RawAddTodo`、`RawCreateBlog` |
| **常用动词** | Get/Add/Create/Update/Delete/Toggle/Search/List | — |

**名称转换链路：**

```
工具定义名                       回调注册名          LLM 调用名(sanitized)      网关路由名
Inner_blog.RawGetTodosByDate → RawGetTodosByDate → RawGetTodosByDate       → RawGetTodosByDate
```

`extractFunctionName()` 会提取最后一个 `.` 后面的部分作为短名称，并建立映射表。
`sanitizeToolName()` 将 `.` 替换为 `_`（LLM function calling 不支持 `.`）。

### 3.3 Description 书写规范（关键）

**格式：`<功能描述>。返回<类型标注>`**

Description 末尾 **必须** 包含返回值类型标注，这对 ExecuteCode 模式至关重要。LLM 生成的 Python 代码需要知道 `call_tool()` 返回的是字符串还是 dict/list，否则会产生 `AttributeError: 'str' object has no attribute 'get'` 等运行时错误。

**返回值类型标注分类：**

| 标注 | 含义 | 示例工具 |
|------|------|---------|
| `返回str` | 纯文本字符串 | RawAllBlogName |
| `返回str(纯文本)` | 纯文本，强调非 JSON | RawAllDiaryContent |
| `返回str(markdown纯文本)` | Markdown 格式文本 | RawGetBlogData、RawCurrentDiaryContent |
| `返回str(数字)` | 数字的字符串表示 | RawAllBlogCount、RawAllExerciseCalories |
| `返回str(YYYY-MM-DD格式)` | 日期字符串 | RawCurrentDate |
| `返回str(格式化文本)` | 结构化但非 JSON 的文本 | RawBlogStatistics、RawGetExerciseStats |
| `返回str(操作结果)` | 操作反馈文本 | RawAddTodo、RawCreateBlog、RawDeleteTodo |
| `返回str(空格分隔的标题列表)` | 特殊格式的文本 | RawGetBlogDataByDate |
| `返回JSON(list)` | JSON 数组 | RawGetTodosByDate、RawGetAllBooks |
| `返回JSON(list,每项含xx字段)` | JSON 数组（说明字段） | RawGetTodosByDate |
| `返回JSON(dict)` | JSON 对象 | RawGetMonthGoal |
| `返回JSON(dict,key为日期)` | JSON 对象（说明 key） | RawGetTodosRange |
| `返回JSON(dict,key为月份)` | JSON 对象（说明 key） | RawGetYearGoals |

**特殊场景标注：**

对于容易被误用的工具（如返回纯文本但工具名暗示可能有结构），需要额外说明：

```go
Description: "获取所有日记内容。返回str(纯文本,每篇以'日记_日期:'开头,不是JSON,不可调用.get())"
```

### 3.4 Parameters JSON Schema 规范

**字符串参数：**
```go
"account": map[string]string{"type": "string", "description": "账号"},
"date":    map[string]string{"type": "string", "description": "日期格式为2026-01-01"},
"status":  map[string]string{"type": "string", "description": "状态:reading/completed/want-to-read/paused"},
```

**数字参数：**
```go
"days":     map[string]interface{}{"type": "number", "description": "统计天数,默认7天"},
"calories": map[string]interface{}{"type": "number", "description": "卡路里"},
"year":     map[string]interface{}{"type": "number", "description": "年份如2026"},
```

**布尔参数：**
```go
"save_result": map[string]interface{}{"type": "boolean", "description": "是否保存AI查询结果到博客"},
```

**required 数组：** 只列出必填参数，可选参数不要放入。

```go
"required": []string{"account", "date", "content"},  // hours/minutes/urgency 是可选的
```

---

## 4. 辅助函数

在回调函数实现中使用以下辅助函数（定义在 `innter_mcp.go`）：

```go
// 安全提取字符串参数（不存在或类型错误时返回 error）
getStringParam(arguments, "key") (string, error)

// 安全提取整数参数（支持 float64/int/int64 类型）
getIntParam(arguments, "key") (int, error)

// 安全提取可选整数参数（不存在时返回默认值）
getOptionalIntParam(arguments, "key", defaultVal) int

// 返回 JSON 格式的错误消息
errorJSON("错误描述") string  // → {"error": "错误描述"}
```

---

## 5. 完整示例：添加一个新工具

假设要添加 `RawGetWeekSummary` 工具，获取本周汇总数据。

### 5.1 回调函数

```go
// inner_blog_tools.go（或新文件 inner_summary_tools.go）

func Inner_blog_RawGetWeekSummary(arguments map[string]interface{}) string {
    account, err := getStringParam(arguments, "account")
    if err != nil {
        return errorJSON(err.Error())
    }
    date, err := getStringParam(arguments, "date")
    if err != nil {
        return errorJSON(err.Error())
    }
    return statistics.RawGetWeekSummary(account, date)
}
```

### 5.2 注册回调

```go
// innter_mcp.go — RegisterInnerTools()

RegisterCallBack("RawGetWeekSummary", Inner_blog_RawGetWeekSummary)
// 如果需要附加提示词：
// RegisterCallBackPrompt("RawGetWeekSummary", "汇总数据后给出改进建议")
```

### 5.3 工具定义

```go
// innter_mcp.go — GetInnerMCPTools() 返回列表中添加

{
    Type: "function",
    Function: LLMFunction{
        Name:        "Inner_blog.RawGetWeekSummary",
        Description: "获取指定日期所在周的汇总数据(待办完成率、运动量、阅读量)。返回JSON(dict)",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "account": map[string]string{"type": "string", "description": "账号"},
                "date":    map[string]string{"type": "string", "description": "日期,返回该日期所在周的汇总"},
            },
            "required": []string{"account", "date"},
        },
    },
},
```

### 5.4 验证

1. 重新编译 `blog-agent`：`go build`
2. 启动后检查日志：确认工具数量增加
3. 通过 gateway API 验证：`GET /api/gateway/tools` 中应包含新工具
4. 通过 LLM 调用测试：在对话中请求相关功能，确认 LLM 能发现并调用新工具

---

## 6. 注意事项

### 6.1 工具定义与回调名必须一致

```
LLMTool.Name = "Inner_blog.RawGetWeekSummary"
                            ↓ extractFunctionName
RegisterCallBack 名 = "RawGetWeekSummary"  ← 必须完全匹配
```

### 6.2 account 参数

几乎所有工具都需要 `account` 参数（用于多用户隔离）。始终将其放在 `required` 列表中。LLM 系统提示已配置自动填充 account。

### 6.3 日期参数格式

统一使用 `YYYY-MM-DD` 格式（如 `2026-01-01`），在 description 中明确说明。

### 6.4 返回值类型一致性

回调函数的实际返回值类型必须与 Description 中的类型标注一致：
- 标注 `返回JSON(list)` → 回调函数必须返回 `[...]` 格式的 JSON 字符串
- 标注 `返回str(纯文本)` → 回调函数返回普通文本，不是 JSON

### 6.5 工具书写格式

工具定义支持两种书写风格，按可读性选择：

**展开式（参数多或含说明时）：**
```go
{
    Type: "function",
    Function: LLMFunction{
        Name:        "Inner_blog.CreateReminder",
        Description: "创建定时提醒任务。返回str(操作结果)",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "account": map[string]string{"type": "string", "description": "账号"},
                "title":   map[string]string{"type": "string", "description": "提醒标题"},
                "cron":    map[string]string{"type": "string", "description": "Cron表达式"},
            },
            "required": []string{"account", "title"},
        },
    },
},
```

**单行式（参数少且简单时，节省篇幅）：**
```go
{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetTodosByDate", Description: "获取指定日期的待办列表。返回JSON(list)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期"}}, "required": []string{"account", "date"}}}},
```

### 6.6 外部 agent 工具

CodeGen、Deploy 等外部 agent 的工具不在此文件中定义。它们通过各自 agent 的 UAP 注册机制自动加入工具目录。本文件只管理 blog-agent 内部的 MCP 工具。
