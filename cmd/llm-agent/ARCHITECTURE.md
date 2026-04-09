# llm-agent 运行时架构

详细的任务执行时序见：[docs/TASK-PROCESSING-FLOW.md](/Users/guccang/github_repo/go_blog/cmd/llm-agent/docs/TASK-PROCESSING-FLOW.md)

## 总览

当前运行时只有一条执行主线：

1. 构建稳定 `system prompt`
2. 创建或恢复 `QuerySession`
3. 每轮先注入 mailbox / runtime attachments
4. 调用 LLM
5. 执行工具
6. 保存 transcript + `RuntimeSnapshot`
7. 必要时通过 `resume_task` 从根会话继续

## 核心模块

### Query Runtime

- `runtime_query.go`

职责：

- 根任务消息循环
- tool calling 调度
- attachment 注入
- 消息压缩
- transcript checkpoint 与 `RuntimeSnapshot` 持久化

### Prompt / Attachment 分层

- `runtime_context.go`
- `runtime_session.go`

职责：

- `SystemPromptContext` 保存稳定 system prompt
- `Attachment` 保存动态上下文
- attachment 统一注入到消息流，而不是反复回写 system prompt

### Mailbox / Runtime State

- `runtime_mailbox.go`

职责：

- `mailbox_<session>.json`
- `runtime_<session>.json`
- `RuntimeSnapshot` 持久化 `SystemPromptContext`、attachment、compact history
- 子任务通知、恢复指令、运行时上下文的落盘与读取

### 子任务执行

- `skill_executor.go`
- `orchestrator.go`
- `runtime_subtask.go`
- `subtask.go`

职责：

- `execute_skill` 在隔离 session 中运行
- 父会话与子会话通过 mailbox 交换通知
- `execute_skill` 的直接 `tool_result` 只回状态占位，详细结果只通过 `task_notification` 回流
- 子任务 loop 与主 query loop 使用同一种 attachment 注入机制

### 恢复

- `runtime_resume.go`

职责：

- 读取根会话 transcript 和 `RuntimeSnapshot`
- 清洗 transcript
- 补执行未完成的 tool calls
- 继续原 `QueryLoop`

## 执行顺序

### 普通任务

1. `assistant.go` / `processor.go` 构建 `TaskContext`
2. `runtime_query.go` 创建 `QueryLoop`
3. 每轮先 `DrainMailbox`
4. 再调 LLM
5. 再执行工具
6. 保存根会话 transcript 与 `RuntimeSnapshot`

### execute_skill

1. `runtime_tool_execution.go` 识别 `execute_skill`
2. `skill_executor.go` 创建隔离子会话
3. `orchestrator.go` 构建子任务 prompt
4. `runtime_subtask.go` 运行独立 loop
5. 结果写回父会话 mailbox，父任务下一轮以 attachment 形式读取

### resume_task

1. `runtime_resume.go` 加载根会话
2. 清洗消息历史
3. 回放未完成 tool calls
4. 复用相同 `root_session_id` 继续 `processTask`

## 设计边界

- 根任务和子任务共用相同的 query / attachment / runtime state 模型。
- 动态上下文只通过 mailbox 和 attachment 进入执行链。
- `execute_skill` 用于隔离技能执行，不引入额外的中间执行层。
- 恢复以根会话 transcript 和 `RuntimeSnapshot` 为准。
