---
name: code-execution
description: 代码执行技能。当需要批量工具调用、数据处理、多源数据聚合时使用 ExecuteCode 沙箱执行 Python 代码。
summary: 两个及以上工具调用优先走 ExecuteCode，报错后修正代码再重试
tools: ExecuteCode
agents: exec_code
keywords: 执行,execute,python,脚本,计算,批量
---

# 代码执行（ExecuteCode 沙箱）

**call_tool 使用规范见 workspace/CALL_TOOL.md**

## 适用场景

- 需要调用两个及以上工具，并在中间做数据处理
- 需要循环、批量、聚合、过滤、排序、格式化
- 需要把多次工具结果合并后再给用户结论
- 需要在不中断上下文的前提下完成较复杂的计算逻辑

## 必须遵守

- 只有 `print()` 的内容会返回，中间变量不会进入上下文
- `call_tool()` 失败会抛异常；可用 `safe_call_tool(name, args, default)` 做容错
- 批量、循环、聚合场景必须优先使用 `ExecuteCode`
- `ExecuteCode` 返回语法或运行时错误后，必须修正代码并重试，不能因为一次报错就退回逐个工具调用

## 推荐流程

1. 先明确需要哪些工具结果，以及最终要输出什么。
2. 在 Python 中按“调用工具 -> 清洗数据 -> 汇总结论”的顺序组织代码。
3. 复杂任务优先保留中间变量，只 `print()` 最终结果。
4. 如果第一次执行失败，针对错误点修正脚本后再次调用 `ExecuteCode`。

## 工具选择规则

- 只调用一个简单工具时，可直接调用该工具，不必包进 `ExecuteCode`
- 会话类工具和同步执行类工具通常直接调用，例如 `AcpStartSession`、`CodegenStartSession`、`DeployProject`、`DeployPipeline`
- 需要批量调用博客、锻炼、待办、项目或读书工具时，优先放进 `ExecuteCode`

## 禁止行为

- 因为一次 Python 报错就放弃沙箱路径
- 在 `ExecuteCode` 里只做单次简单工具调用，没有任何处理逻辑
- 输出大量中间调试信息，挤占上下文
- 用 Python 直接替代已有的专业工具工作流

## 示例

- “统计最近 30 天运动时长并按类型分组”
  用 `ExecuteCode` 批量调用查询接口，再在 Python 中分组汇总
- “帮我汇总最近 30 天的待办、运动和阅读进度”
  在 `ExecuteCode` 里批量调用对应业务工具，再统一整理输出
