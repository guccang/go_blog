---
name: code-execution
description: 代码执行技能。当需要批量工具调用、数据处理、多源数据聚合时使用 ExecuteCode 沙箱执行 Python 代码。
tools: ExecuteCode
keywords: 执行,execute,python,脚本,计算,批量
---

# 代码执行（ExecuteCode 沙箱）

## 使用场景

当任务需要调用 2 个及以上工具时，**必须优先使用 ExecuteCode 工具**，而不是逐个调用工具。

## 使用方式

调用 ExecuteCode，传入 Python 代码，代码中通过 call_tool(name, args) 调用 MCP 工具。

**示例 — 获取本周运动数据并汇总：**
```python
import json
# 获取当前日期
date_info = call_tool("RawCurrentDate", {})
# 获取本周运动数据
exercise = call_tool("RawGetExerciseRange", {"account": "xxx", "start": "2025-03-03", "end": "2025-03-09"})
# 获取待办
todos = call_tool("RawGetTodosByDate", {"account": "xxx", "date": "2025-03-09"})
# 在 Python 中处理数据，只输出最终结果
print(f"日期: {date_info}")
print(f"运动数据: {exercise}")
print(f"待办: {todos}")
```

## 关键规则

1. 只有 print() 的内容会返回给你，中间变量不会占用上下文
2. call_tool 失败会抛异常；用 safe_call_tool(name, args, default) 可在失败时返回默认值
3. 循环批量操作特别适合 ExecuteCode（如遍历日期范围逐天查询）
4. 单一工具调用（如只调用 RawCurrentDate）可以直接调用，不需要 ExecuteCode
5. **工具返回值类型不确定**：call_tool 返回值可能是 str（纯文本/markdown）或 dict/list（结构化数据），**绝对不要假设返回类型**。正确做法：先 print(type(result), result[:200]) 查看格式，或者直接 print(result) 输出原始内容让我来分析
6. **禁止对字符串调用 .get()/.items() 等 dict 方法**，必须先用 isinstance(result, dict) 判断类型
7. **ExecuteCode 失败必须修正重试**：当 ExecuteCode 返回 Python syntax error 或运行时错误时，**必须分析错误原因、修正 Python 代码后再次调用 ExecuteCode**。严禁因为代码报错就放弃沙箱执行，转而逐个直接调用工具或用其他方式绕过。沙箱是数据处理的正确路径，代码错误只需要修复代码本身

## 何时直接调用工具（不用 ExecuteCode）

- 只需要调用 1 个工具
- 编码会话工具（CodegenStartSession / CodegenSendMessage）
- 部署工具（DeployProject / DeployPipeline）
