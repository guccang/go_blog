# call_tool 使用规范

## 返回值

**`call_tool()` 直接返回工具结果值**，无需额外解包：

```python
date = call_tool("RawCurrentDate", {})  # "2026-03-24"

exercise = call_tool("RawAddExercise", {...})  # {"id": "xxx", "name": "慢跑", ...}

todos = call_tool("RawGetTodosRange", {...})  # {"2026-03-09": [...], ...}
```

## 工具名称

**使用裸名称**，不要拼接 agentID 前缀：
- ✅ `call_tool("RawGetExerciseRange", {...})`
- ❌ `call_tool("go_blog_RawGetExerciseRange", {...})`

## 错误处理

- `call_tool` 失败会抛异常，不会返回错误对象
- `safe_call_tool(name, args, default)` 失败时返回 default 而不抛异常

## 示例

**批量查询聚合：**
```python
import json

date = call_tool("RawCurrentDate", {})
todos = call_tool("RawGetTodosRange", {"account": "xxx", "startDate": "2026-03-09", "endDate": date})
exercise = call_tool("RawGetExerciseRange", {"account": "xxx", "startDate": "2026-03-09", "endDate": date})

print(f"日期: {date}")
print(f"待办: {json.dumps(todos, ensure_ascii=False)}")
print(f"运动: {json.dumps(exercise, ensure_ascii=False)}")
```

**新增记录：**
```python
result = call_tool("RawAddExercise", {
    "account": "xxx",
    "date": "2026-03-15",
    "name": "晨跑",
    "exerciseType": "跑步",
    "duration": 30,
    "intensity": "medium"
})
print(result)
```

**安全调用（失败不中断）：**
```python
result = safe_call_tool("WebSearch", {"query": "test"}, default=None)
if result:
    print(result)
```
