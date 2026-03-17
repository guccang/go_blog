# llm-agent 核心机制与原理

## 架构概览

llm-agent 是一个 LLM 编排代理，负责接收用户请求、调用 MCP 工具、拆解复杂任务并编排执行。它通过 UAP 协议连接 gateway，与 codegen-agent、deploy-agent 等工具提供方协作。

```
用户（Web/微信）
     ↓
llm-agent
  ├── processTask()        简单任务：LLM 循环直接调用工具
  └── handleComplexTask()  复杂任务：规划 → 审查 → 编排 → 汇总
       ├── PlanTask()          ① 任务拆解
       ├── ReviewPlan()        ② 计划审查
       ├── Orchestrator.Execute()  ③ DAG 编排执行
       │    └── executeSubTask()   ← Agentic Loop（核心）
       └── Synthesize()        ④ 结果汇总
```

---

## 核心机制一：Agentic Loop（自主代理循环）

**位置**: `orchestrator.go` — `executeSubTask()`

这是整个系统最核心的机制。每个子任务不是一次性调用，而是一个 **观察→思考→行动→再观察** 的闭环循环。

### 工作原理

```
子任务描述
     ↓
┌─→ LLM 思考（根据消息历史决定下一步）
│    ↓
│   有工具调用？ ──否──→ 子任务完成，返回文本
│    ↓ 是
│   执行工具调用 → 获取结果
│    ↓
│   结果追加到消息历史
│    ↓
└───回到顶部（LLM 看到工具结果后再次决策）
```

### 代码实现

```go
// orchestrator.go — executeSubTask() 核心循环
messages := []Message{
    {Role: "system", Content: "你正在执行一个子任务..."},
    {Role: "user", Content: subtask.Description},
}

for i := 0; i < maxIter; i++ {
    // ① LLM 根据当前完整对话历史决策
    text, toolCalls, err := SendLLMRequest(&o.cfg.LLM, messages, filteredTools)

    // ② LLM 不再调用工具 → 认为任务完成，退出循环
    if len(toolCalls) == 0 {
        finalText = text
        break
    }

    // ③ 执行工具调用
    for _, tc := range toolCalls {
        result, err := o.bridge.CallTool(originalName, args)
        // 工具结果追加到消息历史 → LLM 下轮能看到
        messages = append(messages, toolMsg)
    }

    // ④ 自动回到循环顶部 → LLM 看到工具结果后再次决策
}
```

### 关键要素

| 要素 | 作用 |
|------|------|
| `messages` 累积 | 每次工具调用的结果都追加到消息历史，LLM 能看到之前所有操作 |
| LLM 自主决策 | LLM 根据工具返回的结果，自行判断是继续操作还是完成任务 |
| `maxIter` 上限 | `SubTaskMaxIterations`（默认 10）防止无限循环 |
| 退出条件 | `len(toolCalls) == 0` — LLM 只返回文本总结而不调用工具时结束 |

### 实际效果（来自日志）

以"编写Go计算器"任务为例，t2 子任务的 Agentic Loop 自主产生了以下迭代：

```
迭代1: LLM 决定创建代码   → 调用 CodegenStartSession → 代码已创建
迭代2: LLM 想查看代码细节 → 调用 CodegenStartSession → 获取代码内容
迭代3: LLM 决定测试功能   → 调用 CodegenStartSession → 获取测试结果
迭代4: LLM 检查改进空间   → 调用 CodegenStartSession → 确认质量
迭代5: LLM 满意，只返回文本总结 → 循环结束
```

每次迭代都是 LLM 自主驱动的——它观察上一步的结果，思考后决定下一步行动。

---

## 核心机制二：简单/复杂任务分流

**位置**: `processor.go` — `processTask()`

### 判断流程

LLM 自行判断任务复杂度。系统在工具列表中注入一个虚拟工具 `plan_and_execute`，当 LLM 认为任务需要拆解时会调用它。

```
用户请求 → LLM 分析
  ├── 简单任务（直接调用工具）→ 普通 Agentic Loop
  └── 调用 plan_and_execute  → 进入复杂任务流程
```

### 简单任务路径

`processTask()` 中的主循环就是一个 Agentic Loop，与子任务执行逻辑相同：

```go
for i := 0; i < maxIter; i++ {
    text, toolCalls, err := SendLLMRequest(...)
    if len(toolCalls) == 0 { break }  // 完成

    // 检查是否触发 plan_and_execute
    if planCallIdx >= 0 {
        return handleComplexTask(...)  // 切换到复杂任务流程
    }

    // 普通工具调用
    for _, tc := range toolCalls {
        result := b.CallTool(...)
        messages = append(messages, toolMsg)
    }
}
```

### 复杂任务路径

`handleComplexTask()` 实现四阶段流程：

```
① PlanTask()       → LLM 生成结构化任务计划（JSON）
② ReviewPlan()     → LLM 审查参数和依赖关系
③ Execute()        → DAG 编排执行所有子任务
④ Synthesize()     → LLM 汇总所有子任务结果
```

---

## 核心机制三：DAG 拓扑编排

**位置**: `orchestrator.go` — `Execute()`

子任务之间存在依赖关系（`depends_on`），编排器通过拓扑排序确定执行顺序。

### 依赖关系示例

```
t1（创建项目）→ t2（编码）→ t3（验证）→ t4（停止会话）→ t5（部署）
```

### 执行逻辑

```go
// 拓扑排序 → 按层级执行
layers := topologicalSort(plan.SubTasks)
// layers = [[t1], [t2], [t3], [t4], [t5]]

for _, layer := range layers {
    for _, subtaskID := range layer {
        // 检查依赖是否完成
        // 检查依赖是否失败/跳过 → 级联跳过
        // 检查依赖是否异步 → 延迟执行
        result := o.executeSubTask(...)  // ← Agentic Loop
    }
}
```

### 状态传递

已完成子任务的结果通过 `siblingContext` 传递给后续子任务：

```go
siblingContext := buildSiblingContext(subtask.DependsOn, completedResults)
// 注入到子任务的 system prompt 中，让 LLM 能引用前置任务的结果
```

---

## 核心机制四：统一任务状态协议

**位置**: `orchestrator.go` — `detectAsyncResults()`

所有工具响应包含标准 `status` 字段，编排器通用判断任务状态：

| status 值 | 含义 | 编排器行为 |
|-----------|------|-----------|
| `completed` | 工具已同步完成 | 继续执行后续子任务 |
| `failed` | 工具执行失败 | 触发失败决策（retry/skip/abort） |
| `in_progress` | 任务仍在进行 | 标记为异步，后续依赖子任务延迟 |
| `started` | 任务刚启动 | 同 in_progress |

```go
switch parsed.Status {
case "completed", "failed":
    // 同步完成，不视为异步
    continue
case "in_progress", "started":
    // 真正的异步任务
    results = append(results, asyncInfo)
}
```

---

## 核心机制五：LLM 驱动的失败决策

**位置**: `planner.go` — `MakeFailureDecision()`

子任务失败后不是简单跳过，而是调用 LLM 分析失败原因并决策：

```
子任务失败 → LLM 分析错误 + 上下文
  ├── retry    → 临时错误，重试一次
  ├── modify   → 参数有误，修改描述后重试
  ├── skip     → 非关键任务，跳过不影响最终结果
  └── abort    → 关键步骤失败，终止整个编排
```

---

## 核心机制六：工具路由与发现

**位置**: `bridge.go`

### 工具发现

```
llm-agent → GET /api/gateway/tools → 获取所有在线 agent 的工具列表
                                          （每 60 秒自动刷新）
```

### 智能路由

当可用工具数 > 15 时，使用 LLM 从工具目录中筛选与用户问题相关的工具子集：

```go
if len(tools) > 15 && query != "" {
    tools = b.routeTools(query, tools)  // LLM 选择相关工具
}
```

### 跨 Agent 调用

```
llm-agent → MsgToolCall → gateway → 目标 agent
                                           ↓
llm-agent ← MsgToolResult ← gateway ← 目标 agent
```

工具调用通过 UAP 协议的请求-响应模式实现，`CallTool()` 使用 channel 同步等待结果。

---

## 核心机制七：计划审查（ReviewPlan）

**位置**: `planner.go` — `ReviewPlan()`

任务规划后、执行前，由 LLM 审查计划质量：

```
PlanTask() 生成计划
     ↓
展示计划详情（含 tool_params 参数）
     ↓
ReviewPlan() 审查
  ├── action=execute  → 审查通过，继续执行
  └── action=optimize → 返回优化后的计划，重新展示后执行
```

审查要点：
1. tool_params 参数是否完整正确
2. 子任务是否有遗漏或冗余
3. 依赖关系是否合理
4. 同步工具是否存在冗余的状态检查子任务

---

## 数据流总结

```
用户消息
  ↓
processTask()
  │
  ├─ 简单任务 ─→ Agentic Loop ──────────────────→ 返回结果
  │
  └─ plan_and_execute 触发
       ↓
     PlanTask()         → 生成 JSON 计划
       ↓
     ReviewPlan()       → LLM 审查/优化
       ↓
     Execute()          → 拓扑排序 → 按层执行
       │
       ├─ 子任务 t1 ──→ executeSubTask() ── Agentic Loop ──→ 结果
       ├─ 子任务 t2 ──→ executeSubTask() ── Agentic Loop ──→ 结果
       ├─ ...
       └─ 子任务 tN ──→ executeSubTask() ── Agentic Loop ──→ 结果
       ↓
     Synthesize()       → LLM 汇总所有子任务结果 → 返回最终回复
```

---

## 关键配置项

| 配置 | 说明 | 默认值 |
|------|------|--------|
| `max_tool_iterations` | 简单任务 Agentic Loop 最大迭代次数 | 15 |
| `subtask_max_iterations` | 子任务 Agentic Loop 最大迭代次数 | 10 |
| `subtask_timeout_sec` | 子任务超时时间 | 120s |
| `long_tool_timeout_sec` | 长工具（编码/部署）超时时间 | 600s |
| `max_sub_tasks` | 最大子任务数量 | 10 |

## 文件职责

| 文件 | 职责 |
|------|------|
| `processor.go` | 统一消息处理入口，简单/复杂任务分流 |
| `orchestrator.go` | DAG 编排执行，Agentic Loop，异步检测，结果汇总 |
| `planner.go` | 任务规划，计划审查，失败决策 |
| `bridge.go` | 工具发现，工具路由，跨 Agent 工具调用 |
| `llm_client.go` | LLM API 调用（同步/流式） |
| `session.go` | 会话持久化，子会话管理 |
| `chat.go` | 微信消息处理，WechatSink 事件推送 |
| `assistant.go` | Web 前端任务处理，StreamingSink 流式输出 |
| `config.go` | 配置加载 |
