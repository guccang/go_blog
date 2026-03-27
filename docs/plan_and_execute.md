# plan_and_execute 机制详解

## 1. 概述

`plan_and_execute` 是 llm-agent 处理复杂任务的核心机制。当用户请求需要多步骤、跨 Agent 协作时，LLM 调用 `plan_and_execute` 虚拟工具触发四阶段流程：**规划 → 审查 → 编排执行 → 汇总**。

---

## 2. 四阶段流程

```
用户请求
     ↓
① PlanTask()       → LLM 生成任务计划（JSON）
     ↓
② ReviewPlan()     → LLM 审查/优化计划
     ↓
③ Execute()        → DAG 拓扑编排执行
     │
     ├─ 子任务 t1 ──→ executeSubTask() ── Agentic Loop
     ├─ 子任务 t2 ──→ executeSubTask() ── Agentic Loop
     └─ ...
     ↓
④ Synthesize()     → LLM 汇总所有子任务结果
```

---

## 3. 核心数据结构

### 3.1 TaskPlan

```go
type TaskPlan struct {
    SubTasks      []SubTaskPlan `json:"subtasks"`
    ExecutionMode string        `json:"execution_mode"` // sequential/parallel/dag
    Reasoning     string        `json:"reasoning"`
}
```

### 3.2 SubTaskPlan

```go
type SubTaskPlan struct {
    ID          string                 `json:"id"`
    Title       string                 `json:"title"`
    Description string                 `json:"description"`
    DependsOn   []string               `json:"depends_on"`       // DAG 依赖关系
    ToolsHint   []string               `json:"tools_hint,omitempty"`  // 提示用到的工具
    ToolParams  map[string]interface{} `json:"tool_params,omitempty"` // 工具调用参数
}
```

---

## 4. 阶段详解

### 4.1 阶段一：PlanTask() 任务规划

**位置**: `planner.go` — `PlanTask()`

LLM 根据用户请求生成结构化任务计划：

```go
func PlanTask(cfg *LLMConfig, query string, tools []LLMTool, account string,
    maxSubTasks int, completedWork string, skillBlock string,
    fallbacks []LLMConfig, cooldown time.Duration) (*TaskPlan, error)
```

**规划原则**：

1. **通过 Skill 执行任务**：编码 → `coding` skill，部署 → `deploy` skill
2. **最大化并行**：无依赖的子任务并行执行（`depends_on` 为空）
3. **精简数量**：用最少的子任务完成任务（通常 2-3 个）
4. **数据处理任务**：用 `ExecuteCode` 编写 Python 代码通过 `call_tool()` 调用

**输出格式**：

```json
{
  "subtasks": [
    {
      "id": "t1",
      "title": "创建编码会话",
      "description": "使用 coding skill 创建项目 helloworld-web，account=xxx",
      "depends_on": [],
      "tools_hint": ["AcpStartSession"]
    },
    {
      "id": "t2",
      "title": "部署到服务器",
      "description": "使用 deploy skill 将项目部署到 ssh-prod，account=xxx",
      "depends_on": ["t1"],
      "tools_hint": ["DeployAdhoc"]
    }
  ],
  "execution_mode": "dag",
  "reasoning": "编码用 coding skill，部署用 deploy skill，有依赖关系顺序执行"
}
```

---

### 4.2 阶段二：ReviewPlan() 计划审查

**位置**: `planner.go` — `ReviewPlan()`

LLM 审查计划质量，检查：

1. **工具参数完整性**：用户指定的参数（如 `model=deepseek`、`tool=opencode`）是否在描述中体现
2. **子任务结构**：是否有遗漏/冗余、是否能合并
3. **依赖关系**：独立任务是否正确并行
4. **禁止新增任务**：只修正参数，不得添加额外确认步骤

**审查结果**：

```go
type PlanReview struct {
    Action string    `json:"action"` // "execute" | "optimize"
    Reason string    `json:"reason"`
    Plan   *TaskPlan `json:"plan,omitempty"` // optimize 时返回修改后的计划
}
```

---

### 4.3 阶段三：Execute() DAG 编排执行

**位置**: `orchestrator.go` — `Execute()`

#### 4.3.1 DAG 调度器

```go
type dagScheduler struct {
    plan         *TaskPlan
    completedSet map[string]bool  // 已完成
    failedSet    map[string]bool  // 失败/跳过
    asyncSet     map[string]bool  // 异步执行中
    scheduledSet map[string]bool  // 已调度
    resultMap    map[string]SubTaskResult
    mu           sync.Mutex
}
```

**核心方法**：

- `getInitialTasks()`：获取无依赖的初始任务
- `allDepsResolved(st)`：检查依赖是否满足
- `markDone(id, result)`：标记完成并返回新解锁的任务

**依赖解决判断**：

```go
// 依赖完成、失败、异步执行中，都视为"已解决"
func (ds *dagScheduler) allDepsResolved(st SubTaskPlan) bool {
    for _, dep := range st.DependsOn {
        if !ds.completedSet[dep] && !ds.failedSet[dep] && !ds.asyncSet[dep] {
            return false
        }
    }
    return true
}
```

#### 4.3.2 并发控制

```go
maxP := o.cfg.MaxParallelSubtasks  // 默认 3
sem := make(chan struct{}, maxP)
```

无依赖的子任务通过信号量控制并发数。

#### 4.3.3 事件驱动调度

```
初始任务调度 → 收集结果 → 解锁后续任务 → 调度新任务 → ...
```

事件循环持续到所有任务完成或中止。

#### 4.3.4 动态计划修订

每完成 2 个任务检查一次是否需要修订计划：

```go
revisionCheckInterval := 2
maxRevisions := 3  // 最多修订 3 次

if revisionCount < maxRevisions && completedCount-lastRevisionCheck >= revisionCheckInterval {
    remaining := collectRemainingSubTasks()
    revResult, err := EvaluateAndRevisePlan(...)
    if revResult.Action == "revise" {
        // 更新计划，调度新增任务
    }
}
```

---

### 4.4 阶段四：Synthesize() 结果汇总

LLM 将所有子任务的结果汇总为最终回复。

---

## 5. Agentic Loop（核心执行机制）

**位置**: `orchestrator.go` — `executeSubTask()`

每个子任务通过 Agentic Loop 执行：

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

### 5.1 关键要素

| 要素 | 作用 |
|------|------|
| `messages` 累积 | 每次工具调用的结果都追加到消息历史 |
| LLM 自主决策 | LLM 根据工具返回结果自行判断下一步 |
| `maxIter` 上限 | 默认 10，防止无限循环 |
| 退出条件 | `len(toolCalls) == 0` — LLM 只返回文本时结束 |

### 5.2 子任务超时

```go
timeout := 120s  // 默认
if hasLongRunningToolHint(subtask.ToolsHint) {
    timeout = 600s + 60s  // 长工具自动扩展
}
```

### 5.3 子任务 system prompt 构建

```go
systemContent = basePrompt
    + fmt.Sprintf("## 当前子任务: %s\n%s\n", subtask.Title, subtask.Description)
    + siblingContext  // 前置任务结果
    + matchedSkillBlock  // 相关技能指引
    + toolParamReference  // 工具参数参考
    + "## 工具使用规范\n"
```

---

## 6. 失败处理

### 6.1 失败决策

**位置**: `planner.go` — `MakeFailureDecision()`

子任务失败后调用 LLM 决策：

```
retry    → 临时错误（超时、网络问题），重试一次
modify   → 参数/代码错误，修正后重试
skip     → 非关键任务，跳过
abort    → 关键步骤失败，终止整个编排
```

### 6.2 失败时扩展兄弟工具

当工具返回业务失败（`success: false`）时，自动补充同 Agent 的替代工具：

```go
// 扩展同 agent 兄弟工具
for _, failedTool := range bizFailedTools {
    siblings := b.getSiblingTools(failedTool)
    for _, s := range siblings {
        if !existingSet[s.Function.Name] {
            filteredTools = append(filteredTools, s)
        }
    }
}
```

---

## 7. 异步检测

**位置**: `orchestrator.go` — `detectAsyncResults()`

通过工具响应的 `status` 字段判断：

```go
switch parsed.Status {
case "completed", "failed":
    // 同步完成，不视为异步
    continue
case "in_progress", "started":
    // 异步执行，延迟后续任务
    results = append(results, asyncInfo)
}
```

异步检测到后：
- 后续依赖子任务标记为 `deferred`
- 最终返回即时确认，不等待异步完成

---

## 8. ExecuteCode 特殊处理

### 8.1 call_tool 协议

Python 代码中通过 `call_tool()` 调用 MCP 工具：

```python
def call_tool(tool_name, arguments=None):
    request = json.dumps({"type": "tool_call", "tool": tool_name, "args": arguments or {}})
    print(f"__TOOL_CALL__{request}__END__", flush=True)
    line = sys.stdin.readline().strip()
    result = json.loads(line)
    return _auto_parse(result.get("data"))  # 自动解析 JSON
```

### 8.2 结果处理

```go
// 提取 stdout 给 LLM，避免结构化 JSON 污染 context
stdout, execSummary := parseExecuteCodeResult(result)
if stdout != "" {
    result = stdout
}
```

### 8.3 虚拟工具拦截

虚拟工具（`plan_and_execute`、`execute_skill` 等）禁止在 ExecuteCode 中调用：

```go
func isVirtualTool(name string) bool {
    switch name {
    case "execute_skill", "plan_and_execute", "set_persona", "set_rule":
        return true
    }
    return false
}
```

---

## 9. 关键配置

| 配置 | 说明 | 默认值 |
|------|------|--------|
| `max_parallel_subtasks` | 子任务并发数 | 3 |
| `subtask_max_iterations` | 子任务 Agentic Loop 最大迭代 | 10 |
| `subtask_timeout_sec` | 子任务超时时间 | 120s |
| `long_tool_timeout_sec` | 长工具超时时间 | 600s |
| `max_sub_tasks` | 最大子任务数 | 10 |
| `max_plan_revisions` | 最大计划修订次数 | 3 |

---

## 10. 完整流程图

```
用户请求
     ↓
processTask() 检测到 plan_and_execute 调用
     ↓
handleComplexTask()
     │
     ├─ ① PlanTask() ────→ TaskPlan (JSON)
     │      ↓
     ├─ ② ReviewPlan() ─→ PlanReview (optimize/execute)
     │      ↓
     ├─ ③ 为每个子任务创建 ChildSession
     │      ↓
     ├─ ④ Execute() ──────────────────────────────────┐
     │      │                                           │
     │      ├─ dagScheduler.getInitialTasks()          │
     │      │      ↓                                    │
     │      ├─ scheduleTask() ───────────────────────┐ │
     │      │      │                                   │ │
     │      │      ├─ shouldSkip() → 依赖失败跳过     │ │
     │      │      ├─ shouldDefer() → 依赖异步延迟    │ │
     │      │      └─ executeSubTask()                 │ │
     │      │             │                            │ │
     │      │             ├─ Agentic Loop              │ │
     │      │             │   └─ CallTool() → 结果    │ │
     │      │             │                            │ │
     │      │             ├─ 检测异步 session_id        │ │
     │      │             └─ SubTaskResult             │ │
     │      │                                           │ │
     │      ├─ 收集结果 → markDone() → 解锁后续任务   │ │
     │      │                                           │ │
     │      └─ 动态修订计划（每2个任务检查一次）       │ │
     │                                                   │ │
     └─ ⑤ Synthesize() ────────────────────────────────┘
              │
              └─→ 返回最终汇总文本
```

---

## 11. 核心文件

| 文件 | 职责 |
|------|------|
| `planner.go` | `PlanTask()`、`ReviewPlan()`、`MakeFailureDecision()` |
| `orchestrator.go` | `Execute()`、`executeSubTask()`、`dagScheduler` |
| `complex_task.go` | `handleComplexTask()` 四阶段入口 |
| `skill_executor.go` | 技能子任务执行 |
