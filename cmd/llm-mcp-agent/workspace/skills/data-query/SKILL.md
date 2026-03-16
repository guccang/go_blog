---
name: data-query
description: 数据查询技能。当需要查询待办、运动、博客等数据并做汇总分析时使用此技能。
tools: RawGetTodosByDate,RawGetTodosRange,RawGetExerciseByDate,RawGetExerciseRange,RawGetExerciseStats,RawRecentExerciseRecords,RawAllBlogNameByDate,RawAllBlogNameByDateRange,RawAllBlogNameByDateRangeCount,RawGetCurrentTaskByRageDate,RawGetBlog,RawSearchBlog,RawCurrentDate
---

# 数据查询与聚合

## 查询原则

- 多源数据查询优先使用 ExecuteCode 批量处理，减少工具调用轮次
- account 使用当前用户账号，不要向用户询问
- **日期范围查询优先使用 Range 接口**，不要循环逐天调用单日接口

## 可用的范围查询接口

| 接口 | 参数 | 返回值 |
|------|------|--------|
| `RawGetTodosRange` | account, startDate, endDate | JSON(dict, key为日期) |
| `RawGetExerciseRange` | account, startDate, endDate | JSON(list) |
| `RawAllBlogNameByDateRange` | account, startDate, endDate | str(空格分隔的标题列表) |
| `RawAllBlogNameByDateRangeCount` | account, startDate, endDate | str(数字) |
| `RawGetCurrentTaskByRageDate` | account, startDate, endDate | JSON(dict, key为日期) |
| `RawGetExerciseStats` | account, days | str(格式化文本) |
| `RawRecentExerciseRecords` | account, days | str(格式化列表) |

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

**案例3 — 查询日期范围内的博客数量和标题：**
```python
# 获取博客数量
count = call_tool("RawAllBlogNameByDateRangeCount", {"account": "xxx", "startDate": "2026-03-01", "endDate": "2026-03-15"})
# 获取博客标题列表
titles = call_tool("RawAllBlogNameByDateRange", {"account": "xxx", "startDate": "2026-03-01", "endDate": "2026-03-15"})
print(f"博客数量: {count}")
print(f"博客标题: {titles}")
```

**案例4 — 多源数据聚合：周报生成（待办 + 运动 + 博客）：**
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

**案例5 — 近期运动趋势分析：**
```python
# 获取近7天运动记录（格式化列表）
recent = call_tool("RawRecentExerciseRecords", {"account": "xxx", "days": 7})
# 获取近7天运动统计
stats = call_tool("RawGetExerciseStats", {"account": "xxx", "days": 7})
print(f"近7天运动记录:\n{recent}")
print(f"\n运动统计:\n{stats}")
```

**案例6 — 待办任务与日常任务对比（使用两种任务接口）：**
```python
start, end = "2026-03-09", "2026-03-15"
account = "xxx"

# TodoList 待办
todos = call_tool("RawGetTodosRange", {"account": account, "startDate": start, "endDate": end})
# 每日任务（另一种任务系统）
tasks = call_tool("RawGetCurrentTaskByRageDate", {"account": account, "startDate": start, "endDate": end})

print(f"TodoList 待办:\n{todos}")
print(f"\n每日任务:\n{tasks}")
```

### 数据聚合分析
查询完数据后在 Python 中完成统计、汇总、对比等分析工作，只 print 最终结论。

## 注意事项

- 先获取当前日期（RawCurrentDate），再计算查询范围
- 工具返回值类型不确定，使用前先检查类型（参考 code-execution 技能规则）
- **禁止循环逐天调用单日接口**，必须使用对应的 Range 接口
