---
name: data-query
description: 数据查询技能。当需要查询待办、运动、博客等数据并做汇总分析时使用此技能。
tools: RawGetTodosByDate,RawGetExerciseByDate,RawGetExerciseRange,RawGetBlog,RawSearchBlog,RawCurrentDate
---

# 数据查询与聚合

## 查询原则

- 多源数据查询优先使用 ExecuteCode 批量处理，减少工具调用轮次
- account 使用当前用户账号，不要向用户询问

## 常见查询模式

### 单日数据
直接调用对应的单日查询工具即可。

### 日期范围数据
使用 ExecuteCode 循环遍历日期范围，批量查询后在 Python 中处理：
```python
from datetime import datetime, timedelta
start = datetime(2025, 3, 1)
end = datetime(2025, 3, 7)
results = []
current = start
while current <= end:
    date_str = current.strftime("%Y-%m-%d")
    data = safe_call_tool("RawGetTodosByDate", {"account": "xxx", "date": date_str}, "")
    if data:
        results.append(f"{date_str}: {data}")
    current += timedelta(days=1)
for r in results:
    print(r)
```

### 数据聚合分析
查询完数据后在 Python 中完成统计、汇总、对比等分析工作，只 print 最终结论。

## 注意事项

- 先获取当前日期（RawCurrentDate），再计算查询范围
- 工具返回值类型不确定，使用前先检查类型
