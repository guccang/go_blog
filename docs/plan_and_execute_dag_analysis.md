# plan_and_execute DAG 架构分析

## 1. 概述

plan_and_execute 是一个基于 DAG（有向无环图）的任务编排系统，支持：
- 任务分解为子任务并定义依赖关系
- 并行执行无依赖的子任务
- 根据执行状态动态修改 DAG
- 失败处理和重试机制
- 异步任务支持

---

## 2. 核心文件

| 文件 | 职责 |
|------|------|
| `cmd/llm-agent/planner.go` | 任务规划、计划修订、失败决策 |
| `cmd/llm-agent/orchestrator.go` | DAG 调度、并发执行、状态管理 |
| `cmd/llm-agent/processor.go` | 子任务处理、工具调用执行 |

---

## 3. DAG 数据模型

### 3.1 TaskPlan 结构

```go
type TaskPlan struct {
	SubTasks      []SubTaskPlan `json:"subtasks"`
	ExecutionMode string        `json:"execution_mode"` // sequential/parallel/dag
	Reasoning     string        `json:"reasoning"`
}
```

### 3.2 SubTaskPlan 结构

```go
type SubTaskPlan struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	DependsOn   []string               `json:"depends_on"`      // DAG 依赖关系
	ToolsHint   []string               `json:"tools_hint,omitempty"`
	ToolParams  map[string]interface{} `json:"tool_params,omitempty"`
}
```

**关键点：**
- `DependsOn` 字段定义子任务间的依赖关系，形成 DAG
- 无依赖的子任务（`DependsOn` 为空）可并行执行
- LLM 通过 `PlanTask()` 生成结构化计划

---

## 4. DAG 调度器

### 4.1 dagScheduler 结构

```go
type dagScheduler struct {
	plan         *TaskPlan
	completedSet map[string]bool    // 已完成任务
	failedSet    map[string]bool    // 失败任务
	asyncSet     map[string]bool    // 异步执行中任务
	scheduledSet map[string]bool    // 已调度任务
	resultMap    map[string]SubTaskResult
	mu           sync.Mutex
}
```

### 4.2 核心方法

#### getInitialTasks() - 获取初始任务

```go
func (ds *dagScheduler) getInitialTasks() []SubTaskPlan {
	var initial []SubTaskPlan
	for _, st := range ds.plan.SubTasks {
		if len(st.DependsOn) == 0 {
			initial = append(initial, st)
			ds.scheduledSet[st.ID] = true
		}
	}
	return initial
}
```

返回所有无依赖的子任务，作为 DAG 的起点。

#### allDepsResolved() - 检查依赖是否满足

```go
func (ds *dagScheduler) allDepsResolved(st SubTaskPlan) bool {
	for _, dep := range st.DependsOn {
		if !ds.completedSet[dep] && !ds.failedSet[dep] && !ds.asyncSet[dep] {
			return false
		}
	}
	return true
}
```

**关键设计：** 依赖任务完成、失败或异步执行中，都视为"已解决"，允许后续任务继续执行。

#### markDone() - 标记完成并解锁后续任务

```go
func (ds *dagScheduler) markDone(id string, result SubTaskResult) []SubTaskPlan {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// 根据状态更新对应集合
	switch result.Status {
	case "done":
		ds.completedSet[id] = true
	case "failed", "skipped":
		ds.failedSet[id] = true
	case "async", "deferred":
		ds.asyncSet[id] = true
	}

	ds.resultMap[id] = result

	// 查找新解锁的子任务
	var unblocked []SubTaskPlan
	for _, st := range ds.plan.SubTasks {
		if ds.scheduledSet[st.ID] {
			continue
		}
		if ds.allDepsResolved(st) {
			unblocked = append(unblocked, st)
			ds.scheduledSet[st.ID] = true
		}
	}
	return unblocked
}
```

每当一个任务完成，自动检查并返回所有新解锁的任务。

---

## 5. DAG 执行流程

### 5.1 主执行循环

```go
// 1. 初始化调度器
scheduler := newDAGScheduler(plan)

// 2. 并发控制（默认 3 个并行子任务）
maxP := o.cfg.MaxParallelSubtasks
sem := make(chan struct{}, maxP)

// 3. 调度初始无依赖任务
initialTasks := scheduler.getInitialTasks()
for _, st := range initialTasks {
	scheduleTask(st)
}

// 4. 事件循环：收集结果 + 解锁后续任务
for completedCount < totalTasks && !aborted {
	msg := <-resultCh
	completedCount++

	// 解锁后续任务
	unblocked := scheduler.markDone(msg.result.SubTaskID, msg.result)
	for _, st := range unblocked {
		scheduleTask(st)
	}
}
```

### 5.2 并发控制

使用信号量（semaphore）限制并发数：

```go
scheduleTask := func(st SubTaskPlan) {
	sem <- struct{}{}  // 获取信号量
	go func(subtask SubTaskPlan) {
		defer func() { <-sem }()  // 释放信号量

		result := o.executeSubTask(...)
		resultCh <- taskResult{subtask: subtask, result: result}
	}(st)
}
```

---

## 6. 动态 DAG 修改

### 6.1 修订触发机制

```go
revisionCheckInterval := 2  // 每完成 2 个任务检查一次
lastRevisionCheck := 0
maxRevisions := 3           // 最多修订 3 次

for completedCount < totalTasks && !aborted {
	msg := <-resultCh
	completedCount++

	// 动态计划修订检查
	if revisionCount < maxRevisions &&
	   completedCount-lastRevisionCheck >= revisionCheckInterval {

		// 收集剩余未执行的子任务
		var remaining []SubTaskPlan
		for _, st := range plan.SubTasks {
			if !scheduler.completedSet[st.ID] &&
			   !scheduler.failedSet[st.ID] &&
			   !scheduler.asyncSet[st.ID] {
				remaining = append(remaining, st)
			}
		}

		// 调用 LLM 评估是否需要修订
		revResult, err := EvaluateAndRevisePlan(
			cfg, originalQuery, plan, completedResults, remaining, tools, ...
		)

		if revResult.Action == "revise" && revResult.Plan != nil {
			revisionCount++
			plan = revResult.Plan

			// 为新增子任务创建 session
			for _, st := range plan.SubTasks {
				if _, exists := childSessions[st.ID]; !exists {
					child := NewChildSession(rootSession, st.Title, st.Description)
					childSessions[st.ID] = child
				}
			}

			// 更新调度器的计划
			scheduler.mu.Lock()
			scheduler.plan = plan
			scheduler.mu.Unlock()
			totalTasks = len(plan.SubTasks)

			// 调度新解锁的任务
			for _, st := range plan.SubTasks {
				if !scheduler.scheduledSet[st.ID] && scheduler.allDepsResolved(st) {
					scheduleTask(st)
				}
			}
		}

		lastRevisionCheck = completedCount
	}
}
```
