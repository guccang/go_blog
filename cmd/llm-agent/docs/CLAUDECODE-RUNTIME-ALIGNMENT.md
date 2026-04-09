# llm-agent 与 Claude Code 的运行时对齐说明

本文档只描述当前仍然有效的 `llm-agent` 运行时，不保留历史方案说明。

## 核心原则

当前实现对齐 Claude Code 的 4 个核心机制：

1. 稳定 `system prompt` 与动态 `attachment` 分层
2. `QueryLoop` 驱动的单一执行主线
3. mailbox 驱动的父子任务通信
4. 基于 transcript 与 `RuntimeSnapshot` 的恢复

## 运行时主线

根任务和子任务共用同一种执行模型：

1. 构建稳定 prompt
2. 创建或恢复 `QuerySession`
3. 每轮先 drain mailbox 并注入动态 attachment
4. 调用 LLM
5. 执行工具
6. 在关键检查点保存 transcript、运行时状态和附件
7. 在下一轮继续，或通过 `resume_task` 从根会话恢复

## 模块映射

| Claude Code 概念 | llm-agent 落点 |
| --- | --- |
| 稳定 system prompt | `assistant.go` + `runtime_context.go` |
| query loop | `runtime_query.go` |
| 动态 attachment | `runtime_mailbox.go` + `runtime_session.go` |
| 隔离子任务 | `skill_executor.go` + `runtime_subtask.go` + `subtask.go` |
| task notification | `orchestrator.go` |
| transcript + runtime snapshot resume | `runtime_resume.go` |

## Prompt 与上下文

- `assistant.go` 负责拼装稳定的 system prompt section。
- `runtime_context.go` 定义 `SystemPromptContext`、`Attachment` 和 `RuntimeSnapshot`。
- 稳定规则、项目指令、Agent 能力和工具目录进入 prompt。
- 依赖结果、子任务通知、恢复提示和运行时补充信息进入 attachment。
- attachment 每轮注入消息流，不回写到稳定 prompt。

## 父子任务模型

- 根任务在 `runtime_query.go` 中运行。
- `execute_skill` 会创建隔离子会话，由 `skill_executor.go` 和 `runtime_subtask.go` 执行。
- 子任务完成后，由 `orchestrator.go` 生成结构化通知并写入父会话 mailbox。
- `execute_skill` 的直接 `tool_result` 只返回子任务状态和 `session_id`，详细结果只通过 `task_notification` 注入父会话下一轮上下文。
- 父会话下一轮执行前会 drain mailbox，把通知转换为 attachment 注入上下文。
- 同一套 mailbox 也承载依赖上下文、fork 上下文和恢复提示。

## 恢复模型

- `runtime_resume.go` 负责根任务恢复。
- 恢复时会同时加载根会话 transcript 与 `runtime_<session>.json`。
- 历史消息会先经过清洗，去掉无效 assistant 片段和未闭合状态。
- 如果最后一轮存在未完成 tool call，会先回放对应工具执行，再继续 query loop。
- 恢复后的任务继续使用原 `root_session_id`、原 prompt context、attachment 集合和 compact history。

## 当前实现边界

- 运行时只有一条执行主线：`QueryLoop -> tool execution -> checkpoint persist -> resume`。
- 子任务只用于隔离技能执行，不承担额外的规划职责。
- 所有跨轮动态信息都通过 mailbox 与 attachment 传递。
- 根任务和子任务都在 loop 关键节点同时保存 transcript 与 `RuntimeSnapshot`。
- 根任务恢复以 transcript 和 `RuntimeSnapshot` 为准，而不是重新构造新的任务上下文。
