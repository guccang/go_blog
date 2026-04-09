# llm-agent 任务处理全流程

本文档描述当前 `llm-agent` 从接收任务到完成、挂起、恢复的完整执行链。内容基于现有实现，不描述已废弃方案。

## 1. 总体结构

当前运行时分成 4 条主线：

1. 根任务执行：`TaskContext -> QueryLoop -> tool execution -> checkpoint`
2. 隔离子任务执行：`execute_skill -> runSkillSubtask -> executeSubTask -> subtask loop`
3. 跨会话上下文传递：`mailbox -> Attachment -> 下一轮注入`
4. 中断恢复：`resumeQuery -> replayPendingToolCalls -> 恢复原 QueryLoop`

其中根任务和子任务复用同一套核心模型：

- 稳定上下文：`SystemPromptContext`
- 动态上下文：`Attachment`
- 运行时快照：`RuntimeSnapshot`
- 消息循环：`QueryLoop` / `QuerySession`

## 2. 核心对象

### 2.1 TaskContext

`TaskContext` 是任务入口参数，包含：

- 基本信息：`TaskID`、`Account`、`Source`、`Query`
- 初始消息：`Messages`
- 执行控制：`Ctx`、`NoTools`、`AllowedTools`
  其中 `AllowedTools` 仅用于系统内部约束，不对用户暴露工具筛选入口。
- 运行期引用：`Sink`、`Trace`、`CurrentSession`
- 恢复信息：`ResumeSession`、`ResumeSnapshot`

它由上游入口构建，然后进入 `processTask()`。

### 2.2 TaskSession

`TaskSession` 是持久化会话对象，保存：

- 会话身份：`ID`、`RootID`、`ParentID`
- 对话历史：`Messages`
- 工具调用记录：`ToolCalls`
- 执行结果：`Result`、`Status`、`Error`

根任务和子任务都持久化为 `session_<id>.json`。

### 2.3 QuerySession

`QuerySession` 是 loop 内部使用的运行态会话，保存：

- 当前消息流
- 当前轮可见工具视图
- attachment 集合
- compact history

它比 `TaskSession` 多了“本轮运行所需的 typed state”，用于构建 `RuntimeSnapshot`。

### 2.4 RuntimeSnapshot

`RuntimeSnapshot` 保存“非 transcript 的运行态”，包括：

- `Query`
- `Status`
- `SystemPromptContext`
- `Attachments`
- `CompactHistory`

它持久化到 `runtime_<session>.json`。

## 3. 根任务执行流程

### 3.1 入口

标准入口是：

1. 上游请求生成 `TaskContext`
2. 调用 `Bridge.processTask(ctx)`
3. `processTask()` 调用 `prepareQueryRuntime(ctx)`
4. 构造 `QueryLoop`
5. 执行 `QueryLoop.Run()`

### 3.2 prepareQueryRuntime

`prepareQueryRuntime()` 负责初始化根任务运行态：

1. 解析 `Query`
2. 选择可见工具
3. 构建或刷新 system prompt
4. 创建或复用根 `TaskSession`
5. 创建 `QuerySession`
6. 构造 `QueryLoopState`
7. 立即保存首个 checkpoint

这里有两个分支：

- 普通新任务：创建新的根 session
- 恢复任务：复用 `ResumeSession`，并通过 `ResumeSnapshot` 恢复 `QuerySession`

### 3.3 QueryLoop 每轮执行顺序

根任务每一轮都严格按下面顺序执行：

1. 检查取消状态
2. drain mailbox
3. 把 mailbox 消息转成 `Attachment` 并注入当前 `QuerySession`
4. 视需要做消息压缩
5. 到达上限时禁用工具并追加“强制总结”提示
6. 调用 LLM
7. 把 assistant 消息同时写入 `QuerySession` 和根 `TaskSession`
8. 若无工具调用，则结束
9. 若有工具调用，则逐个执行
10. 每个工具结果写入 transcript，并立即保存 checkpoint
11. 如果发现业务失败，扩展 sibling tools，并把恢复提示作为 attachment 注入
12. 进入下一轮

可以简化为：

```text
drain mailbox
-> compact
-> call LLM
-> append assistant
-> execute tools
-> append tool results
-> save checkpoint
-> next turn
```

### 3.4 checkpoint 保存点

当前根任务会在这些位置保存 checkpoint：

1. `QueryLoop` 初始化完成后
2. mailbox attachment 注入后
3. 压缩后
4. 强制总结提示写入后
5. assistant 消息写入后
6. 每个 tool result 写入后
7. 最终结束时

checkpoint 由两部分组成：

- `session_<root>.json`
- `runtime_<root>.json`

这保证中途崩溃时 transcript 和 runtime snapshot 同时存在。

## 4. 工具执行流程

### 4.1 普通工具

普通工具执行路径：

1. `QueryLoop.executeToolCalls()`
2. `ToolExecutionRuntime.Execute()`
3. 优先检查本地 handler
4. 否则走 `DispatchTool()` / `CallToolCtxWithProgress()`
5. 记录 `ToolCallRecord`
6. 返回 `ToolExecutionResult`

返回后由 `QueryLoop`：

1. 追加 `tool` 消息
2. 保存 checkpoint
3. 继续下一轮推理

### 4.2 execute_skill

`execute_skill` 是特殊工具，不直接完成业务，而是触发一个隔离子任务。

处理顺序：

1. `ToolExecutionRuntime.dispatchSkill()`
2. `Bridge.runSkillSubtask()`
3. 创建隔离子会话
4. 交给 `Orchestrator.executeSubTask()`
5. 子任务完成后向父会话写入 `task_notification`
6. `execute_skill` 当前轮只返回一个状态占位结果

当前占位结果只包含：

- skill 状态
- `session_id`
- “详细结果将通过 task_notification 注入下一轮上下文”

这意味着父任务当前轮不会直接拿到子任务完整日志，完整内容要等下一轮 mailbox 注入。

## 5. 子任务执行流程

### 5.1 子任务创建

`runSkillSubtask()` 会做这些事：

1. 校验 skill 是否存在
2. 校验所需 agent 是否在线
3. 为 skill 构建独立 system prompt
4. 创建子 `TaskSession`
5. 创建 `Orchestrator`
6. 生成 `SubTaskPlan`
7. 调用 `executeSubTask()`

### 5.2 executeSubTask

`Orchestrator.executeSubTask()` 负责子任务初始化：

1. 设置子任务状态为 `running`
2. 构建子任务 tool view
3. 构建 system prompt
4. 写入 system/user 初始消息
5. 视情况向子会话 mailbox 写入：
   - dependency context
   - fork context
6. 保存初始 session
7. 调用 `runSubTaskLoop()`

### 5.3 runSubTaskLoop

子任务 loop 与根任务非常接近：

1. 检查取消和超时
2. drain 子会话 mailbox
3. 把 mailbox 注入为 attachment
4. 保存 checkpoint
5. 调用 LLM
6. 追加 assistant 消息
7. 执行工具
8. 写入 tool result
9. 保存 checkpoint
10. 如遇业务失败，注入恢复 attachment
11. 工具全部完成或无工具可调时结束

特殊点：

- 如果子任务调用的是终止型 session tool，例如 `AcpStartSession`、`DeployProject`，子任务可提前结束
- 子任务结束后返回 `SubTaskResult`

### 5.4 子任务回流到父任务

子任务结束后，`Orchestrator.enqueueTaskNotification()` 会把结果写入父会话 mailbox。

内容包含：

- 子任务 ID / 标题
- 状态
- 结果摘要
- 关键工具返回
- 错误信息
- 异步会话信息

父任务不会立刻看到这条消息，而是在下一轮开始前通过 `drainMailboxAttachments()` 读取。

## 6. mailbox / attachment 机制

mailbox 是跨会话的异步上下文队列，文件格式是：

- `mailbox_<session>.json`

主要写入来源：

1. 父任务给子任务的 dependency context
2. 父任务给子任务的 fork context
3. 子任务给父任务的 task notification
4. 恢复提示和系统补充上下文

读取流程：

1. `SessionStore.DrainMailbox()`
2. `attachmentsFromMailbox()`
3. `QuerySession.InjectAttachments()`
4. 变成一条或多条 `role=user` 的上下文消息进入当前轮消息流

设计目的很明确：

- 稳定 prompt 不被动态信息污染
- 跨会话信息统一走 mailbox
- 动态上下文进入 transcript，可被恢复

## 7. transcript 与 RuntimeSnapshot

### 7.1 transcript 的职责

transcript 是“模型真实看到过什么”的事实记录，保存在 `TaskSession.Messages` 中。

它负责恢复：

- user / assistant / tool 消息顺序
- 已经发生过的 tool call
- 最后一轮是否停在 assistant tool_call 上

### 7.2 RuntimeSnapshot 的职责

`RuntimeSnapshot` 不替代 transcript，它只补充 typed state：

- 稳定 prompt context
- 当前 attachment 集合
- compact history
- query 和 status

恢复时必须两者一起用：

1. transcript 负责消息历史
2. snapshot 负责运行态补丁

## 8. resume_task 恢复流程

恢复入口是 `resumeQuery()`。

执行顺序如下：

1. 加载根 `TaskSession`
2. 如果已有终态结果，直接返回
3. 复制根 transcript
4. `sanitizeResumeMessages()` 清洗消息历史
5. 加载根 `RuntimeSnapshot`
6. 优先从 snapshot 恢复 query 和 prompt context
7. 构造带 `ResumeSession` 和 `ResumeSnapshot` 的 `TaskContext`
8. `replayPendingToolCalls()` 回放未完成工具调用
9. 把回放后的消息再次持久化
10. 重新进入 `processTask()`

### 8.1 sanitizeResumeMessages

清洗逻辑主要解决两类问题：

1. 过滤空 assistant 片段
2. 如果最后存在未闭合的 assistant tool_calls，只保留仍需补执行的最后一段

目标是避免恢复时重复执行更早已经完成过的工具。

### 8.2 replayPendingToolCalls

如果 transcript 最后停在 assistant tool_calls 上，恢复时会：

1. 逐个执行未完成 tool call
2. 生成 `tool` 消息
3. 追加回 `TaskSession.Messages`
4. 保存回放后的 session
5. 更新 `RuntimeSnapshot.Status`

这样即使恢复过程再次中断，也不会丢掉已经补回的 tool result。

## 9. 特殊分支

### 9.1 直接工具目录回复

如果请求命中 MCP 工具目录查询，`prepareQueryRuntime()` 会直接生成结果，不进入完整 query loop。

### 9.2 NoTools 模式

如果 `TaskContext.NoTools=true`：

- 工具不会暴露给模型
- 即使模型返回 tool calls，也会被直接忽略并结束

### 9.3 强制总结

如果达到 `MaxToolIterations`：

1. 当前 `QuerySession` 先禁用所有工具
2. 注入一条强制总结 user 消息
3. 下一轮只能输出最终总结

### 9.4 业务失败工具扩展

如果工具返回业务失败而不是系统错误：

1. 当前 agent 视图会扩展 sibling tools
2. 同时注入一条 system hint attachment
3. 让下一轮模型优先考虑替代工具或修正参数重试

## 10. 持久化文件布局

根目录：`<session_dir>/<root_id>/`

常见文件：

- `session_<session_id>.json`
- `runtime_<session_id>.json`
- `mailbox_<session_id>.json`
- `trace_<session_id>.json`
- `trace_<session_id>.md`
- `index.json`

含义如下：

- `session_*.json`：事实 transcript 和工具记录
- `runtime_*.json`：typed runtime state
- `mailbox_*.json`：跨会话动态上下文队列
- `trace_*.json`：结构化执行轨迹，包含任务描述、执行路径、LLM 轮次、工具调用明细
- `trace_*.md`：面向人工排障的执行报告，内含 Mermaid 流程图和时间线
- `index.json`：列表展示和汇总信息

## 11. 端到端时序

### 11.1 普通任务

```text
TaskContext
  -> processTask
  -> prepareQueryRuntime
  -> QueryLoop
     -> drain mailbox
     -> call LLM
     -> execute tools
     -> save checkpoint
     -> next turn / finish
```

### 11.2 execute_skill

```text
root QueryLoop
  -> execute_skill
  -> runSkillSubtask
  -> executeSubTask
  -> runSubTaskLoop
  -> enqueue task_notification to parent mailbox
  -> current tool_result returns placeholder
  -> parent next turn drains mailbox
  -> attachment enters parent context
```

### 11.3 resume_task

```text
resumeQuery
  -> load root session
  -> load RuntimeSnapshot
  -> sanitize transcript
  -> replay pending tool calls
  -> rebuild QuerySession
  -> continue QueryLoop
```

## 12. 调试轨迹

当前 root task 和 child task 都会在 checkpoint 时同步落盘 trace 文件，方便判断：

- 任务描述是否正确进入了当前 session
- 工具是否按预期被收缩到正确子集
- LLM 是否走了预期路径，比如直接工具调用还是 `execute_skill`
- 子任务是否创建成功、是否通过 `task_notification` 回流
- 某一轮是否被 mailbox 注入、compact、tool expand 改写了执行路径

trace 文件重点信息：

- 基本信息：`task_id / root_id / session_id / parent_session_id / scope / status`
- 工具视图：`policy / matched_skills / all_tools / visible_tools / discovered_tools`
- 执行路径：按步骤记录 `task_start -> tool_view_ready -> round_n_llm -> tool_call -> task_finish`
- 轮次明细：每轮 LLM 耗时、assistant 摘要、tool_calls、mailbox 注入、compact 记录
- 事件时间线：tool call、context compaction、child session、task notification 等
- Mermaid 流程图：快速判断任务实际走了哪条路径

## 13. 当前设计结论

当前 `llm-agent` 的任务处理模型可以概括为：

- 所有任务最终都落到 loop 驱动的消息执行链
- 稳定上下文和动态上下文分层保存
- 父子会话只通过 mailbox 交换动态信息
- transcript 与 `RuntimeSnapshot` 一起构成恢复依据
- `execute_skill` 是隔离执行机制，不是额外的规划层

如果要排查问题，通常先判断它属于哪一层：

1. 入口构造问题：`TaskContext`
2. loop 执行问题：`QueryLoop` / `runSubTaskLoop`
3. 跨会话传递问题：mailbox / attachment
4. 中断恢复问题：`RuntimeSnapshot` / `resumeQuery`
