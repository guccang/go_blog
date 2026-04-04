# Claude Code Main 执行主链深拆：Query + Tool + Permission

本文只分析 Claude Code 最关键的一条运行链：

1. `query.ts` 如何驱动主循环
2. 工具如何被分批、并发、流式执行
3. 权限如何在工具调用前做 allow / deny / ask 判定
4. 工具结果如何重新回写消息流并推动下一轮

本文对应的主要源码：

- `src/query.ts`
- `src/services/tools/toolOrchestration.ts`
- `src/services/tools/StreamingToolExecutor.ts`
- `src/services/tools/toolExecution.ts`
- `src/hooks/useCanUseTool.tsx`
- `src/utils/permissions/permissions.ts`
- `src/utils/permissions/permissionSetup.ts`
- `src/Tool.ts`

---

## 1. 先给结论：Claude Code 的主循环本质是什么

如果把 Claude Code 的主执行链压缩成一句话：

`一个带上下文恢复、compact、预算控制、流式工具调度和权限判定的递归 agentic loop`

它不是简单的：

`LLM -> tool call -> tool result -> LLM`

而更像：

`上下文准备 -> compact/缓存处理 -> streaming API -> 中途工具流式启动 -> 权限判定 -> 并发/串行工具执行 -> 附件/记忆/技能结果注入 -> stop hooks / token budget / recovery -> 递归进入下一轮`

Claude Code 的复杂度不在于“支持工具”，而在于它认真处理了下面这些边界条件：

- prompt 太长
- output token 爆了
- streaming 中途 fallback
- 部分工具可以并发，部分不能
- 权限模式不同
- 队列中还夹杂通知/attachment/skill discovery
- query 可能被用户中断
- query 可能处在 subagent / main thread / remote / plan mode 中

---

## 2. `query.ts` 的职责边界

`src/query.ts` 是 Claude Code 的总控循环。

它直接做的事包括：

- 接收 `QueryParams`
- 维护跨迭代状态 `State`
- 预取 memory / skill discovery
- 在每轮开始前做 compact、collapse、tool result budgeting
- 调模型 streaming API
- 把流式响应转换为 `assistantMessages`、`toolUseBlocks`
- 交给 tool runtime 执行工具
- 收集 `toolResults`
- 注入 attachment / memory / skill discovery 结果
- 决定是否进入下一轮
- 处理 stop hooks、token budget continuation、max turns、abort、recovery

也就是说，`query.ts` 同时扮演了：

- loop controller
- message state machine
- recovery coordinator
- tool orchestration entrypoint

---

## 3. 输入对象：`QueryParams` 不是 prompt，而是一次 agent 运行上下文

`QueryParams` 包含：

- `messages`
- `systemPrompt`
- `userContext`
- `systemContext`
- `canUseTool`
- `toolUseContext`
- `fallbackModel`
- `querySource`
- `maxOutputTokensOverride`
- `maxTurns`
- `skipCacheWrite`
- `taskBudget`

这里最重要的两个字段不是 prompt，而是：

### 3.1 `canUseTool`

这是权限决策函数。

含义是：

- 模型提出 tool_use 不等于工具可以直接执行
- 每个工具执行都要通过统一权限入口

### 3.2 `toolUseContext`

这是工具执行的运行时上下文。

里面有：

- 当前工具池
- 当前模型
- app state 读写能力
- abortController
- file state cache
- 通知/UI 回调
- 当前消息历史
- 当前 agent 标识

所以 Claude Code 的 query loop 不是只拿“文本 prompt”跑模型，而是拿完整 runtime 跑。

---

## 4. `State`：Claude Code 如何在多轮循环里保存真正有用的状态

`query.ts` 里定义的 `State` 是跨迭代可变状态。

关键字段有：

- `messages`
- `toolUseContext`
- `autoCompactTracking`
- `maxOutputTokensRecoveryCount`
- `hasAttemptedReactiveCompact`
- `maxOutputTokensOverride`
- `pendingToolUseSummary`
- `stopHookActive`
- `turnCount`
- `transition`

### 4.1 为什么它不直接修改一堆局部变量

源码注释写得很清楚：

- 每个 continue site 不想维护 9 个变量单独赋值
- 所以把跨轮状态集中到 `State`

这个设计很重要，因为 Claude Code 的 continue 路径非常多：

- compact retry
- reactive compact retry
- collapse drain retry
- max_output_tokens recovery
- token budget continuation
- stop hook blocking retry
- 正常 next turn

如果没有统一 `State`，循环很容易变成不可维护的 spaghetti。

### 4.2 `transition` 的意义

`transition` 用来记录上一轮为什么继续。

例如：

- `collapse_drain_retry`
- `reactive_compact_retry`
- `max_output_tokens_escalate`
- `max_output_tokens_recovery`
- `stop_hook_blocking`
- `token_budget_continuation`
- `next_turn`

这是非常好的可观测性设计，因为：

- 测试可以断言到底走了哪条恢复路径
- 逻辑可以避免重复触发同一个恢复动作

---

## 5. 每轮开始前：Claude Code 先做“上下文整备”，而不是直接调模型

在真正发起 API 请求前，`query.ts` 会做一大串准备工作。

这部分经常被低估，但其实是 Claude Code 成熟度很高的地方。

### 5.1 Memory prefetch

调用：

- `startRelevantMemoryPrefetch()`

特点：

- 不是阻塞式立即注入
- 是先启动异步预取
- 等后面合适时机再 consume

设计原因：

- memory 检索可以与模型 streaming 并行
- 减少 turn 的感知延迟

### 5.2 Skill discovery prefetch

调用：

- `skillPrefetch?.startSkillDiscoveryPrefetch(...)`

同样不是立刻阻塞，而是后台预取。

这说明 Claude Code 的理念是：

- 能提前做、又不影响主路径结果的一律提前做

### 5.3 tool result budgeting

调用：

- `applyToolResultBudget(...)`

作用：

- 限制累积 tool result 的上下文膨胀
- 在进入模型之前先控制上下文体积

### 5.4 snip compact

如果 feature 开启，会先做 `snipCompactIfNeeded`。

这是一种比全量 compact 更轻的裁剪。

### 5.5 microcompact

调用：

- `deps.microcompact(...)`

这是更细粒度的上下文压缩层。

### 5.6 context collapse

如果开启，会在 autocompact 前先尝试上下文折叠。

设计目的很明确：

- 如果 collapse 之后已经回到安全上下文范围
- 就不必做更重的 autocompact

### 5.7 autocompact

调用：

- `deps.autocompact(...)`

如果触发：

- 会生成 compact 后消息
- 记录 compaction telemetry
- 更新 `tracking`
- 直接把 compact 结果写回当前回合继续执行

### 5.8 system prompt 不是裸用，而是 `appendSystemContext(systemPrompt, systemContext)`

即使进了 query loop，system prompt 还要和 `systemContext` 组装成 `fullSystemPrompt`。

说明 Claude Code 清楚区分：

- prompt 基础段
- 每轮可变的系统上下文附加段

---

## 6. 真正发模型请求时，Claude Code 传的不是“当前 messages”，而是被处理过的消息投影

进入 API 请求前，代码会构造：

- `messagesForQuery`

它来自：

- `getMessagesAfterCompactBoundary(messages)`
- 经过 tool result budget
- 经过 snip
- 经过 microcompact
- 经过 collapse
- 经过 autocompact

所以模型看到的并不是完整原始 transcript，而是经过多层投影后的“当前有效上下文视图”。

这很关键。

很多项目会把所有消息都直接扔给模型，Claude Code 不是。它先把“当前真正值得给模型看的消息子集”算出来。

---

## 7. streaming API 阶段：Claude Code 如何在输出过程中处理 assistant、tool_use 与 fallback

### 7.1 `deps.callModel(...)` 是真正流式调用模型的入口

传入的核心参数包括：

- `messages: prependUserContext(messagesForQuery, userContext)`
- `systemPrompt: fullSystemPrompt`
- `thinkingConfig`
- `tools`
- `signal`
- `model`
- `fallbackModel`
- `querySource`
- `agents`
- `allowedAgentTypes`
- `mcpTools`
- `taskBudget`

可见 Claude Code 的模型请求参数量很大，它不是“系统 prompt + 消息 + tools”这么简单。

### 7.2 streaming 期间维护 4 个关键集合

在一轮 streaming 里，`query.ts` 会维护：

- `assistantMessages`
- `toolResults`
- `toolUseBlocks`
- `needsFollowUp`

含义：

#### `assistantMessages`

当前轮模型产出的 assistant message。

#### `toolUseBlocks`

当前轮遇到的所有 tool_use block。

#### `toolResults`

当前轮工具执行后得到的 user-side tool_result message。

#### `needsFollowUp`

是否需要进入下一轮。

本质逻辑：

- 只要本轮出现 tool_use，就需要 follow-up

### 7.3 streaming 中途 fallback 是一个专门处理的异常路径

Claude Code 很认真处理了 streaming fallback：

- 原模型 streaming 中途失败
- 切到 fallback model
- tombstone 掉 orphaned messages
- 清空 `assistantMessages`
- 清空 `toolResults`
- 清空 `toolUseBlocks`
- 丢弃旧的 `StreamingToolExecutor`
- 必要时去掉 thinking signature block

这很关键，因为如果不这样做，会出现：

- UI 看到一半消息
- tool_result 对不上旧 tool_use_id
- thinking signature 不再匹配新模型

Claude Code 在这块是产品级稳态处理，不是 best-effort。

---

## 8. 为什么 Claude Code 需要 `StreamingToolExecutor`

### 8.1 普通 agent 的做法

通常是：

1. 等 assistant message 全部流完
2. 提取 tool_calls
3. 一次性执行所有工具

### 8.2 Claude Code 的做法

如果 gate 打开，它会：

1. 在 streaming 时就发现 tool_use
2. 立刻 `streamingToolExecutor.addTool(toolBlock, message)`
3. 工具可边流边执行
4. 在主流式循环中不断 `getCompletedResults()`
5. 结果一完成就 yield 回 UI

### 8.3 这样做的收益

- Bash/Read/Fetch 等慢工具可以更早开始
- 用户更早看到 progress
- 总 turn latency 更低

### 8.4 StreamingToolExecutor 的关键状态

它内部追踪每个工具：

- `queued`
- `executing`
- `completed`
- `yielded`

同时记录：

- 是否并发安全
- progress message
- context modifier
- synthetic cancel result

它其实就是一个小型工具调度器，而不是简单容器。

---

## 9. 工具执行总线：`runTools()` 如何做“并发但不乱序”

`services/tools/toolOrchestration.ts` 的逻辑很清晰。

### 9.1 第一步：先对 tool_use 分批

通过 `partitionToolCalls()`：

- 连续的 concurrency-safe 工具合成一批
- 非 concurrency-safe 工具单独成批

这是按“运行时安全性”切分，不是按工具名字切分。

### 9.2 第二步：并发批次并行，串行批次顺序执行

#### 并发批次

- `runToolsConcurrently()`
- 多个工具并发执行
- context modifier 先暂存
- 最后按原顺序应用 modifier

#### 串行批次

- `runToolsSerially()`
- 每个工具执行完更新 `currentContext`

### 9.3 为什么需要 `contextModifier`

有些工具不只是输出 message，还会修改运行时上下文，例如：

- 更新文件状态缓存
- 更新 permission / working dir / tool registry
- 注入新的动态状态

如果并发直接改共享 context，状态会乱。

所以 Claude Code 采用：

- 工具执行时返回“如何修改 context”的函数
- 主 orchestrator 统一按顺序提交

这是一种很成熟的事务化思路。

---

## 10. `toolExecution.ts`：真正执行单个工具时发生了什么

这是 Claude Code 的单工具执行核心。

虽然文件很大，但可以抽象成几层。

### 10.1 前置阶段

包括：

- 查找工具定义
- 解析 schema
- 记录 telemetry
- 启动 speculative classifier
- 调权限判断
- 执行 pre-tool hooks

### 10.2 权限判定阶段

它会调用 `canUseTool(...)`。

只有权限通过后，工具才真正进入执行阶段。

### 10.3 执行阶段

工具会在 `ToolUseContext` 中执行，期间可以：

- 发 progress
- 产生 attachment
- 修改 context
- 触发 tool-specific telemetry

### 10.4 结果包装阶段

工具执行后，Claude Code 会统一包装成 user-side `tool_result` message。

这点非常重要：对模型来说，工具结果总是通过消息系统回流，而不是某个旁路结构。

### 10.5 失败与拒绝也走 message 通道

无论是：

- 权限拒绝
- hook 阻塞
- 用户中断
- 工具抛错

最终都会生成可以回流给模型或 UI 的 message，而不是只在日志里静默失败。

这保证了 agent loop 的一致性。

---

## 11. 权限判定主入口：`useCanUseTool.tsx`

这是从“模型提出 tool_use”到“最终 allow/deny/ask”之间的调度中枢。

### 11.1 它不是只返回布尔值

返回的是 `PermissionDecision`。

这可能包括：

- `allow`
- `deny`
- `ask`

并且可能带：

- `updatedInput`
- `decisionReason`
- `suggestions`
- `pendingClassifierCheck`

说明 Claude Code 里的权限结果是一个结构化决策，不是 bool。

### 11.2 主流程

逻辑可以概括为：

1. 创建 permission context
2. 调 `hasPermissionsToUseTool(...)`
3. 如果 `allow`
   - 记录 decision
   - 返回 `buildAllow(...)`
4. 如果 `deny`
   - 记录 decision
   - 可能记录 auto-mode denial
   - 返回拒绝
5. 如果 `ask`
   - 可能先走 coordinator permission
   - 可能先走 swarm worker permission
   - 可能等待 speculative classifier
   - 最后才弹 interactive permission UI

### 11.3 `ask` 不是单一弹窗，而是多策略调度

这是 Claude Code 权限系统成熟的地方。

当结果是 `ask`，它不等于立刻弹窗，而是依次考虑：

- 当前是不是 coordinator mode
- 当前是不是 swarm worker
- bash classifier 能不能在 grace period 内自动批准
- 是否要走 bridge callbacks
- 是否要走 channel callbacks

所以 `ask` 的含义是：

- 还需要进一步决定由谁来答复，不是立即 UI 阶段

---

## 12. 规则引擎：`permissions.ts` 怎么判断规则是否命中

### 12.1 权限规则来源是多层的

源码里 `PERMISSION_RULE_SOURCES` 包含：

- setting sources
- `cliArg`
- `command`
- `session`

这意味着权限规则不是只有一个配置文件来源。

### 12.2 规则分三类

- allow
- deny
- ask

### 12.3 规则匹配不是纯字符串比较

`toolMatchesRule()` 支持：

- 整个工具名匹配
- MCP server 级匹配
- `mcp__server__*`
- server 级 rule 覆盖整个 server 下工具

这意味着 Claude Code 的权限系统能原生处理 MCP 工具域，而不是把 MCP 当普通工具名硬塞进去。

### 12.4 agent deny rule 是单独处理的

`getDenyRuleForAgent()` 支持 `Agent(agentType)` 这种语义。

这点很重要，因为 agent 不是普通工具：

- deny 整个 AgentTool 不够
- 需要 deny 某一种 agent subtype

Claude Code 在权限模型里为 agent 单独开了口子。

---

## 13. `permissionSetup.ts`：为什么 Claude Code 要单独识别危险权限规则

这个模块里非常值得注意的点是：

- 不是所有 allow rule 都能放心用于 auto mode

例如：

- `Bash(*)`
- `python:*`
- `node:*`
- `PowerShell(*)`
- `Agent(*)`

都可能绕过 classifier，让模型直接做危险行为。

所以 Claude Code 会：

- 判断规则是否危险
- 在 auto mode 下裁剪危险规则
- 必要时关闭 bypass/auto 能力

这体现出它的安全观：

- 权限规则本身也要被安全审查

而不是“用户写了 allow 就绝对照做”。

---

## 14. 本轮没有工具时，循环如何收束

当 `needsFollowUp == false`，Claude Code 不会立刻简单退出。

它还会做一串收尾逻辑：

### 14.1 Prompt-too-long recovery

如果最后消息是被 withheld 的 prompt-too-long 错误：

- 先尝试 context collapse drain
- 再尝试 reactive compact
- 最后才真正把错误暴露给用户

### 14.2 Media-size recovery

如果最后消息是媒体过大错误：

- 尝试 reactive compact 的 strip-retry

### 14.3 max_output_tokens recovery

如果最后消息是 `max_output_tokens`：

优先级大致是：

1. 如果当前还没 escalate，先把 cap 从默认升到 `ESCALATED_MAX_TOKENS`
2. 还不行，就自动插一条 meta user message 要求模型继续
3. 最多恢复若干次
4. 超过上限才把错误暴露出去

### 14.4 stop hooks

即使模型没有 tool_use，也不代表这一轮可以直接结束。

`handleStopHooks(...)` 还可能：

- 阻止 continuation
- 生成 blocking errors
- 要求再来一轮

### 14.5 token budget continuation

如果开启 token budget，Claude Code 还会检查：

- 当前 turn 的输出 token 是否还没达到预算目标

如果没达到：

- 会自动注入一条 meta user message
- 继续下一轮

这意味着 Claude Code 的“结束条件”不是简单的“没 tool_use 就结束”，而是：

- 没 tool_use
- 没有可恢复错误
- stop hooks 不阻塞
- token budget 不要求继续

才真正结束。

---

## 15. 有工具时，Claude Code 怎么接下一轮

### 15.1 先执行工具

工具执行来源有两种：

- `streamingToolExecutor.getRemainingResults()`
- `runTools(...)`

区别：

- 前者是边流边执行的剩余结果
- 后者是传统批执行

### 15.2 收集 `toolResults`

每个工具执行产出的 message 会：

- 先 `yield` 给 UI
- 再转成 API 可用消息追加到 `toolResults`

### 15.3 插入 attachment / memory / skill discovery

执行完工具后，Claude Code 还会继续往 `toolResults` 塞：

- queued command attachments
- memory attachments
- skill discovery attachments
- file change attachment

这说明“下一轮给模型的上下文”不仅是 tool_result，还包括系统在这一轮顺便发现和生成的附加信息。

### 15.4 刷新工具池

如果 `refreshTools()` 存在，下一轮之前还会刷新可用工具。

典型场景：

- MCP server 在本轮中途连上了

下一轮模型就能看到新工具。

### 15.5 最终递归

新的 `State` 会变成：

- `messages: [...messagesForQuery, ...assistantMessages, ...toolResults]`
- `toolUseContext: updated context`
- `turnCount + 1`
- `pendingToolUseSummary`
- `transition: next_turn`

然后继续 `while (true)`。

这本质上就是显式状态机递归，而不是函数递归。

---

## 16. 中断语义：为什么 Claude Code 的 abort 处理很细

Claude Code 区分两类中断：

- streaming 阶段中断
- tool call 阶段中断

并且还区分：

- 普通 abort
- `interrupt` 类型的用户提交中断

这样做的结果是：

- 某些场景会生成 synthetic tool_result，保证 tool_use/tool_result 成对
- 某些场景不会再额外显示“用户中断”，因为后续排队消息已经提供上下文

这说明 Claude Code 很重视：

- transcript 一致性
- UI 语义一致性
- API replay 正确性

---

## 17. Claude Code 这条主链最强的地方在哪里

如果只看 `query + tool + permission` 这条链，我认为最强的不是单点，而是下面 7 个组合设计：

### 17.1 `State` + 多 continue site

恢复路径非常多，但仍能维持可维护性。

### 17.2 工具执行与流式输出深度耦合

不是先收全 assistant 再执行工具，而是边流边调度。

### 17.3 权限是结构化决策，不是布尔开关

支持 rule、hook、classifier、interactive、bridge、worker。

### 17.4 tool result 总是经消息系统回流

不搞旁路状态返回，保证 loop 一致。

### 17.5 上下文预处理层很多，但职责分明

- budget
- snip
- microcompact
- collapse
- autocompact

### 17.6 恢复机制不是单一 fallback

分别处理：

- prompt too long
- media too large
- max output tokens
- model fallback

### 17.7 “结束条件”比普通 agent 成熟很多

不是简单 `if no tools then done`。

---

## 18. 如果你要把这条链迁移到 `go_blog/llm-agent`，优先级建议

对你现在的 Go 版 `llm-agent` 来说，我建议按下面顺序迁移：

### 18.1 第一优先级：统一 `State` 与恢复路径

先别急着加更多功能，先把 query loop 的 continue/retry/recovery 状态收敛成统一状态对象。

### 18.2 第二优先级：权限从布尔控制升级为结构化决策

至少要支持：

- allow
- deny
- ask
- decisionReason

否则后面很难接 classifier、rules、interactive flow。

### 18.3 第三优先级：工具并发安全标记

为工具增加类似 `isConcurrencySafe()` 的语义，不然后续并发调度会非常粗糙。

### 18.4 第四优先级：tool result 全部消息化

不要让部分工具走旁路数据、部分走文本回填。统一回成消息。

### 18.5 第五优先级：compact / budget / recovery 分层

把：

- 消息压缩
- 大结果裁剪
- 输出 token 恢复

拆成独立层，而不是都塞在一个 `processTask` 里。

### 18.6 第六优先级：subtask/subagent 复用同一主循环

Claude Code 的关键优势之一是：

- 主 agent 和子 agent 不是两套 runtime

你现在的 Go 版如果想继续演进，最终也应走这个方向。

---

## 19. 结论

只看 `query + tool + permission` 这一段，Claude Code 已经明显不是“会调工具的聊天机器人”，而是一套：

- 具备上下文投影层
- 具备结构化权限引擎
- 具备并发工具调度器
- 具备多种恢复策略
- 具备递归 agent loop

的 agent runtime。

这也是为什么它能稳定支持：

- 本地 REPL
- bridge
- remote session
- subagent
- teammate
- MCP
- skills

因为这些能力最终都被压进了同一个总控执行链里，而不是靠多个独立脚本拼起来。

