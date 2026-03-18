---
name: blog-data-opt
description: 博客数据查询与操作技能。查询待办、运动、博客等数据并做汇总分析，以及新增/完成/删除运动记录、新增/完成/删除待办、创建博客等数据操作。
tools: RawGetTodosByDate,RawGetTodosRange,RawAddTodo,RawToggleTodo,RawDeleteTodo,RawUpdateTodo,RawGetExerciseByDate,RawGetExerciseRange,RawGetExerciseStats,RawAddExercise,RawToggleExercise,RawDeleteExercise,RawUpdateExercise,RawRecentExerciseRecords,RawAllBlogNameByDate,RawAllBlogNameByDateRange,RawAllBlogNameByDateRangeCount,RawGetCurrentTaskByRageDate,RawGetBlog,RawSearchBlog,RawCurrentDate,RawCreateBlog
keywords: 博客,待办,运动,todo,exercise,blog,数据,查询,记录,周报,统计,日记
---

# 数据查询与操作

## 查询原则

- 多源数据查询优先使用 ExecuteCode 批量处理，减少工具调用轮次
- account 使用当前用户账号，不要向用户询问
- **日期范围查询优先使用 Range 接口**，不要循环逐天调用单日接口

## 可用的查询接口

| 接口 | 参数 | 返回值 |
|------|------|--------|
| `RawGetTodosRange` | account, startDate, endDate | JSON(dict, key为日期) |
| `RawGetExerciseRange` | account, startDate, endDate | JSON(list) |
| `RawAllBlogNameByDateRange` | account, startDate, endDate | str(空格分隔的标题列表) |
| `RawAllBlogNameByDateRangeCount` | account, startDate, endDate | str(数字) |
| `RawGetCurrentTaskByRageDate` | account, startDate, endDate | JSON(dict, key为日期) |
| `RawGetExerciseStats` | account, days | str(格式化文本) |
| `RawRecentExerciseRecords` | account, days | str(格式化列表) |

## 可用的操作接口

### 待办操作
| 接口 | 参数 | 返回值 |
|------|------|--------|
| `RawAddTodo` | account, date, content, hours, minutes, urgency, importance | JSON(新建的待办项) |
| `RawToggleTodo` | account, date, id | JSON({success:true}) |
| `RawDeleteTodo` | account, date, id | JSON({success:true}) |
| `RawUpdateTodo` | account, date, id, hours, minutes | JSON({success:true}) |

### 运动记录操作
| 接口 | 参数 | 返回值 |
|------|------|--------|
| `RawAddExercise` | account, date, name, exerciseType, duration, intensity, calories, notes | JSON(新建的运动记录) |
| `RawToggleExercise` | account, date, id | JSON({success:true}) |
| `RawDeleteExercise` | account, date, id | JSON({success:true}) |
| `RawUpdateExercise` | account, date, id, name, exerciseType, duration, intensity, calories, notes | JSON({success:true}) |

### 博客操作
| 接口 | 参数 | 返回值 |
|------|------|--------|
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

date_info = call_tool("RawCurrentDate", {})
# 一次性获取整周待办数据
todos = call_tool("RawGetTodosRange", {"account": "xxx", "startDate": "2026-03-09", "endDate": "2026-03-15"})
print(f"当前日期: {date_info}")
print(f"本周待办数据类型: {type(todos)}")
print(todos)
```

**案例2 — 查询本月运动记录并统计：**
```python
import json

# 获取日期范围内的运动记录
exercise = call_tool("RawGetExerciseRange", {"account": "xxx", "startDate": "2026-03-01", "endDate": "2026-03-15"})
# 获取运动统计（近30天）
stats = call_tool("RawGetExerciseStats", {"account": "xxx", "days": 30})
print(f"运动记录类型: {type(exercise)}")
print(f"运动记录: {exercise}")
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
})
print(result)
```

**案例4 — 完成运动记录：**
```python
# 先查询获取 id
records = call_tool("RawGetExerciseByDate", {"account": "xxx", "date": "2026-03-15"})
print(records)
# 然后用 id 完成
result = call_tool("RawToggleExercise", {"account": "xxx", "date": "2026-03-15", "id": "exercise_id"})
print(result)
```

**案例5 — 多源数据聚合：周报生成（待办 + 运动 + 博客）：**
```python
import json

start, end = "2026-03-09", "2026-03-15"
account = "xxx"

# 并行获取三个维度的数据
todos = call_tool("RawGetTodosRange", {"account": account, "startDate": start, "endDate": end})
exercise = call_tool("RawGetExerciseRange", {"account": account, "startDate": start, "endDate": end})
blogs = call_tool("RawAllBlogNameByDateRange", {"account": account, "startDate": start, "endDate": end})
blog_count = call_tool("RawAllBlogNameByDateRangeCount", {"account": account, "startDate": start, "endDate": end})

print(f"=== 周报数据 {start} ~ {end} ===")
print(f"\n【待办】类型: {type(todos)}")
print(todos)
print(f"\n【运动】类型: {type(exercise)}")
print(exercise)
print(f"\n【博客】数量: {blog_count}")
print(f"标题列表: {blogs}")
```

### 数据聚合分析
查询完数据后在 Python 中完成统计、汇总、对比等分析工作，只 print 最终结论。

## 注意事项

- 先获取当前日期（RawCurrentDate），再计算查询范围
- 工具返回值类型不确定，使用前先检查类型（参考 code-execution 技能规则）
- **禁止循环逐天调用单日接口**，必须使用对应的 Range 接口
