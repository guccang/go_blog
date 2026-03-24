---
name: blog-data-opt
description: 博客数据查询与操作技能。查询待办、运动、博客等数据并做汇总分析，以及新增/完成/删除运动记录、新增/完成/删除待办、创建博客等数据操作。
summary: 用 Range 接口批量查询，操作前先查 id
tools: RawGetTodosByDate,RawGetTodosRange,RawAddTodo,RawToggleTodo,RawDeleteTodo,RawUpdateTodo,RawGetExerciseByDate,RawGetExerciseRange,RawGetExerciseStats,RawAddExercise,RawToggleExercise,RawDeleteExercise,RawUpdateExercise,RawRecentExerciseRecords,RawAllBlogNameByDate,RawAllBlogNameByDateRange,RawAllBlogNameByDateRangeCount,RawGetCurrentTaskByRageDate,RawGetBlog,RawSearchBlog,RawCurrentDate,RawCreateBlog
keywords: 博客,待办,运动,todo,exercise,blog,数据,查询,记录,周报,统计,日记,锻炼
---

# 数据查询与操作

## call_tool 返回值规范

**所有 `call_tool()` 返回统一结构 `{"data": <实际值>}`**，通过 `result["data"]` 获取实际值：

```python
result = call_tool("RawCurrentDate", {})
date = result["data"]  # "2026-03-24"

result = call_tool("RawAddExercise", {...})
exercise = result["data"]  # {"id": "xxx", "name": "慢跑", ...}

result = call_tool("RawGetTodosRange", {...})
todos = result["data"]  # {"2026-03-09": [...], ...}
```

## 查询原则

- 多源数据查询优先使用 ExecuteCode 批量处理，减少工具调用轮次
- account 使用当前用户账号，不要向用户询问
- **日期范围查询优先使用 Range 接口**，不要循环逐天调用单日接口

## 可用的查询接口

### 基础工具
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawCurrentDate` | (无需参数) | str，如 `"2026-03-24"` |

### 单日查询
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawGetTodosByDate` | account, date | list，每项含 id/content/done |
| `RawGetExerciseByDate` | account, date | list，每项含 id/name/duration 等 |
| `RawAllBlogNameByDate` | account, date | str(空格分隔的标题列表) |

### 范围查询
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawGetTodosRange` | account, startDate, endDate | dict, key 为日期 |
| `RawGetExerciseRange` | account, startDate, endDate | list |
| `RawAllBlogNameByDateRange` | account, startDate, endDate | str(空格分隔的标题列表) |
| `RawAllBlogNameByDateRangeCount` | account, startDate, endDate | str(数字) |
| `RawGetCurrentTaskByRageDate` | account, startDate, endDate | dict, key 为日期 |
| `RawGetExerciseStats` | account, days | str(格式化文本) |
| `RawRecentExerciseRecords` | account, days | str(格式化列表) |

## 可用的操作接口

### 待办操作
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawAddTodo` | account, date, content, hours, minutes, urgency, importance | dict(新建的待办项) |
| `RawToggleTodo` | account, date, id | dict({success:true}) |
| `RawDeleteTodo` | account, date, id | dict({success:true}) |
| `RawUpdateTodo` | account, date, id, hours, minutes | dict({success:true}) |

### 运动记录操作
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawAddExercise` | account, date, name, exerciseType, duration, intensity, calories, notes | dict(新建的运动记录) |
| `RawToggleExercise` | account, date, id | dict({success:true}) |
| `RawDeleteExercise` | account, date, id | dict({success:true}) |
| `RawUpdateExercise` | account, date, id, name, exerciseType, duration, intensity, calories, notes | dict({success:true}) |

### 博客操作
| 接口 | 参数 | data 类型 |
|------|------|-----------|
| `RawCreateBlog` | account, title, content, tags, authType, encrypt(可选) | str(操作结果) |

- `tags`: 多个标签用 `|` 分隔
- `authType`: 1=私有, 2=公开, 4=加密, 8=协作, 16=日记
- `encrypt`: 0=否, 1=是（可选）
- 创建成功后返回博客链接，格式为 `[title](/get?blogname=title)`

## 操作注意事项

- 完成/删除操作需要先查询获取记录的 `id`，再调用对应接口
- `RawToggleExercise` 切换完成状态（未完成→完成，完成→未完成）
- `RawAddExercise` 的 `exerciseType` 常见值：跑步、游泳、力量训练、瑜伽、骑行等
- `RawAddExercise` 的 `intensity` 可选值：low、medium、high
- `RawAddTodo` 的 urgency/importance 取值 1-4（1最高）

## 常见查询模式

### 单日数据
直接调用对应的单日查询工具即可。

### 日期范围数据
使用 ExecuteCode + Range 接口一次性获取，不要循环逐天调用：

**案例1 — 查询本周待办完成情况：**
```python
import json

current_date = call_tool("RawCurrentDate", {})["data"]  # "2026-03-15"
todos = call_tool("RawGetTodosRange", {"account": "xxx", "startDate": "2026-03-09", "endDate": current_date})["data"]
print(f"当前日期: {current_date}")
print(json.dumps(todos, ensure_ascii=False, indent=2))
```

**案例2 — 查询本月运动记录并统计：**
```python
import json

exercise = call_tool("RawGetExerciseRange", {"account": "xxx", "startDate": "2026-03-01", "endDate": "2026-03-15"})["data"]
stats = call_tool("RawGetExerciseStats", {"account": "xxx", "days": 30})["data"]
print(f"运动记录: {json.dumps(exercise, ensure_ascii=False)}")
print(f"统计: {stats}")
```

**案例3 — 新增运动记录：**
```python
result = call_tool("RawAddExercise", {
    "account": "xxx",
    "date": "2026-03-15",
    "name": "晨跑",
    "exerciseType": "跑步",
    "duration": 30,
    "intensity": "medium",
    "calories": 300,
    "notes": "公园跑步"
})["data"]
print(result)
```

**案例4 — 完成运动记录：**
```python
# 先查询获取 id
records = call_tool("RawGetExerciseByDate", {"account": "xxx", "date": "2026-03-15"})["data"]
print(records)
# 然后用 id 完成
result = call_tool("RawToggleExercise", {"account": "xxx", "date": "2026-03-15", "id": "exercise_id"})["data"]
print(result)
```

**案例5 — 多源数据聚合：周报生成（待办 + 运动 + 博客）：**
```python
import json

start, end = "2026-03-09", "2026-03-15"
account = "xxx"

todos = call_tool("RawGetTodosRange", {"account": account, "startDate": start, "endDate": end})["data"]
exercise = call_tool("RawGetExerciseRange", {"account": account, "startDate": start, "endDate": end})["data"]
blogs = call_tool("RawAllBlogNameByDateRange", {"account": account, "startDate": start, "endDate": end})["data"]
blog_count = call_tool("RawAllBlogNameByDateRangeCount", {"account": account, "startDate": start, "endDate": end})["data"]

print(f"=== 周报数据 {start} ~ {end} ===")
print(f"\n【待办】{json.dumps(todos, ensure_ascii=False)}")
print(f"\n【运动】{json.dumps(exercise, ensure_ascii=False)}")
print(f"\n【博客】数量: {blog_count}")
print(f"标题列表: {blogs}")
```

### 数据聚合分析
查询完数据后在 Python 中完成统计、汇总、对比等分析工作，只 print 最终结论。
