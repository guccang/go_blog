# 工具参数路由丢失问题修复记录

> 日期：2026-03-16
> 涉及模块：gateway / codegen-agent / llm-mcp-agent（planner / processor / orchestrator）

---

## 1. 问题现象

用户发送：`编码 go语言实现一个计算器网页,监听端口8883,claude使用deepseek模型，部署到ssh-prod`

codegen-agent 日志显示 CodegenStartSession 参数错误：

```
# 第1次调用 — tool 和 model 均为空
ExecuteTask: session=tc_..., project=calculator-web, tool=, model=

# 第2次调用 — model 错误传为 "claude"，期望 "deepseek"
ExecuteTask: session=tc_..., project=calculator-web, tool=claudecode, model=claude
```

**期望**：`tool=claudecode, model=deepseek`

---

## 2. 根因分析

### 2.1 工具参数 schema 是否在路由中丢失？

**否。** `LLMTool.Parameters` 是 `json.RawMessage`，在整条链路中完整保留：

```
DiscoverTools → getLLMTools → routeCapabilities → mergeCapabilityTools
→ routeTools → SendLLMRequest
```

每个阶段操作的都是完整的 `LLMTool` 结构体，参数 schema 未丢失。

### 2.2 真正的根因：三个阶段的上下文缺失

问题出在 **LLM 看到的上下文不够**，导致生成的工具调用缺少关键参数。

#### 缺失 1：Agent 动态能力信息未传递给 LLM

codegen-agent 注册时上报了可用模型列表：

```go
// codegen-agent/connection.go — buildRegisterPayload
Models:           c.agent.ScanSettings(),      // ["default", "deepseek"]
ClaudeCodeModels: c.agent.ScanClaudeCodeSettings(),
OpenCodeModels:   c.agent.ScanOpenCodeSettings(),
Tools:            c.agent.ScanTools(),          // ["claudecode"]
```

但这些信息在两个地方被丢弃：

1. **Gateway**：`GetAllAgents()` 返回的 agent 信息不含 `Meta` 字段 → 模型列表无法被其他 agent 获取
2. **llm-mcp-agent**：`AgentInfo` 结构体只有 `ID/Name/Description/ToolNames`，没有模型/工具字段

结果：LLM 看到的 `model` 参数描述只是 `"模型配置名称（可选）"` — 不知道合法值是什么。

#### 缺失 2：PlanTask 参数描述被截断

`extractParamInfo()` 将参数描述截断为 **15 字符**：

```
原始: tool(编码工具（可选，claudecode/opencode）)
截断: tool(编码工具（可选，claude)  ← "claudecode/opencode" 被截掉
```

规划 LLM 看不到合法值 → 子任务描述中不提 model/tool 参数。

#### 缺失 3：ReviewPlan 跳过审查

```go
if len(plan.SubTasks) <= 2 {
    return &PlanReview{Action: "execute", Reason: "子任务数量较少，直接执行"}, nil
}
```

本案例只有 2 个子任务 → **审查直接跳过** → 无人发现参数遗漏。

#### 缺失 4：子任务 LLM 缺少能力信息

`executeSubTask` 的 system prompt 不含 `getAgentDescriptionBlock()` → 子任务 LLM 不知道 `"可用模型配置: default, deepseek"`。

### 2.3 因果链

```
codegen-agent 注册: Models=["default","deepseek"], Tools=["claudecode"]
    ↓
Gateway GetAllAgents 不返回 Meta → 模型信息丢弃
    ↓
llm-mcp-agent AgentInfo 没有 Models 字段 → system prompt 无可用模型列表
    ↓
extractParamInfo 截断 15 字符 → 规划 LLM 看不到 tool 合法值
    ↓
PlanTask 生成的子任务描述只提 project + prompt → 忽略 model + tool
    ↓
ReviewPlan: 2 个子任务 → 跳过 → 无人检查参数完整性
    ↓
executeSubTask system prompt 无 agent 能力描述 → 子任务 LLM 不知道 deepseek 是合法值
    ↓
CodegenStartSession(project, prompt) → model="" tool="" → 使用默认配置
```

---

## 3. 修复方案

### 3.1 Gateway 暴露 Meta 字段

**文件**：`cmd/common/uap/server.go` — `GetAllAgents()`

```go
// 修改前：不返回 Meta
result = map[string]any{
    "agent_id": a.ID, "name": a.Name, ...
}

// 修改后：透传 Meta 扩展字段
if len(a.Meta) > 0 {
    info["meta"] = a.Meta
}
```

### 3.2 codegen-agent 注册时在 Meta 中包含完整能力

**文件**：`cmd/codegen-agent/connection.go` — `NewConnection()`

```go
// 修改前
Meta: map[string]any{"workspaces": cfg.Workspaces}

// 修改后
Meta: map[string]any{
    "workspaces":        cfg.Workspaces,
    "models":            agent.ScanSettings(),
    "claudecode_models": agent.ScanClaudeCodeSettings(),
    "opencode_models":   agent.ScanOpenCodeSettings(),
    "coding_tools":      agent.ScanTools(),
}
```

### 3.3 AgentInfo 扩展 + system prompt 注入

**文件**：`cmd/llm-mcp-agent/bridge.go`

```go
// AgentInfo 新增字段
type AgentInfo struct {
    // ...原有字段
    Models           []string // 合并后的模型配置名列表
    ClaudeCodeModels []string
    OpenCodeModels   []string
    CodingTools      []string // 可用编码工具（claudecode, opencode）
}

// DiscoverAgents 解析 meta
info.Models = parseStringSlice(a.Meta["models"])
info.CodingTools = parseStringSlice(a.Meta["coding_tools"])

// getAgentDescriptionBlock 注入到 system prompt
// LLM 将看到：
// - **codegen-home** (codegen_xxx): 代码编写、项目管理、编码会话
//   - 可用编码工具(tool参数): claudecode
//   - 可用模型配置(model参数): default, deepseek
```

### 3.4 ReviewPlan 始终执行 + 参数完整性审查

**文件**：`cmd/llm-mcp-agent/planner.go` — `ReviewPlan()`

核心改动：

1. **移除跳过逻辑**：删除 `if len(plan.SubTasks) <= 2 { return ... }` — 所有计划都必须审查
2. **新增 `agentCapabilities` 参数**：传入 agent 能力描述（可用模型/编码工具），帮助审查参数有效性
3. **审查 prompt 增加最高优先级审查项**：

```
### 1. 工具参数完整性（最重要）
- 对照用户原始请求和工具参数schema，检查子任务描述中是否遗漏了用户指定的参数
- 例如：用户说"使用deepseek模型"，但子任务描述中未提到model参数 → 必须补充
- 工具的可选参数如果用户明确指定了值，则必须在子任务描述中体现
```

4. **审查 prompt 包含完整工具参数 schema**（而非截断版）和 agent 能力信息

### 3.5 extractParamInfo 截断长度提升

**文件**：`cmd/llm-mcp-agent/planner.go` — `extractParamInfo()`

描述截断从 15→40 字符，保留合法值信息：

```
修改前: tool(编码工具（可选，claude)     ← 15字符，合法值被截断
修改后: tool(编码工具（可选，claudecode/opencode）)  ← 40字符，完整
```

### 3.6 子任务 LLM 注入 agent 能力描述

**文件**：`cmd/llm-mcp-agent/orchestrator.go` — `executeSubTask()`

在子任务的 system prompt 中注入 `getAgentDescriptionBlock()`，子任务 LLM 可见：

```
## 可用 Agent 能力
- **codegen-home** (codegen_xxx): 代码编写、项目管理、编码会话
  - 可用编码工具(tool参数): claudecode
  - 可用模型配置(model参数): default, deepseek
```

---

## 4. 日志增强

同步补充了因果链日志，遇到类似问题时可通过日志定位：

### 4.1 新增/增强的日志点

| 位置 | 日志内容 | 作用 |
|------|---------|------|
| `DiscoverAgents` | `agent: codegen-home (...) tools=[...] models=[default,deepseek] coding_tools=[claudecode]` | 确认 agent 能力信息是否正确获取 |
| `collectSkillTools` | `skills: coding→[CodegenStartSession,...] → matched 5 tools` | 追踪 skill→工具的匹配关系 |
| `mergeCapabilityTools` | `agent=8 skill=5 base=2 → total=10 tools=[...]` | 追踪工具合并来源 |
| `logLLMContext` | `CodegenStartSession: params={project, prompt, model, tool} required=[project, prompt]` | 确认 LLM 收到的参数 schema |
| processTask 工具调用 | 参数截断 200→500 字符 | 能看到 model/tool 参数值 |
| skill 在线检测 | `skill 工具在线检测: 2→1, 剔除: [deploy(...)]` | 追踪 skill 被剔除的原因 |

### 4.2 日志因果链示例

修复后遇到同样问题，日志会展示完整因果链：

```
[Bridge] agent: codegen-home (codegen_xxx) tools=[...] models=[default,deepseek] coding_tools=[claudecode]
  → agent 注册成功，模型列表已获取

[Planner] ▶ 开始规划 ... availableTools=10
  → 规划 LLM 看到: CodegenStartSession [参数: project(项目名称)[必填], prompt(...)[必填], model(模型配置名称（可选）), tool(编码工具（可选，claudecode/opencode）)]

[Planner] ▶ 审查计划 subtasks=2
  → 审查 LLM 看到完整参数 schema + agent 能力信息

[Planner] ✓ 审查结果: action=optimize reason=子任务描述缺少model=deepseek参数
  → 审查发现参数遗漏，自动补充

[Orchestrator] subtask=t1 → 调用工具: CodegenStartSession args={"project":"calculator-web","prompt":"...","model":"deepseek","tool":"claudecode"}
  → 参数正确传递
```

---

## 5. 修改文件索引

| 文件 | 改动摘要 |
|------|---------|
| `cmd/common/uap/server.go` | `GetAllAgents()` 返回 `meta` 字段 |
| `cmd/codegen-agent/connection.go` | `NewConnection()` Meta 增加 models/coding_tools |
| `cmd/llm-mcp-agent/bridge.go` | `AgentInfo` 扩展字段 + `DiscoverAgents` 解析 meta + `getAgentDescriptionBlock` 输出模型信息 + `collectSkillTools`/`mergeCapabilityTools` 增加日志 + 新增 `parseStringSlice` |
| `cmd/llm-mcp-agent/planner.go` | `ReviewPlan` 移除跳过、增加 agentCapabilities 参数、强化参数完整性审查 + `extractParamInfo` 截断 15→40 |
| `cmd/llm-mcp-agent/processor.go` | `handleComplexTask` 传 agentCapabilities 给 ReviewPlan + 工具调用参数截断 200→500 + skill 在线检测日志增强 |
| `cmd/llm-mcp-agent/orchestrator.go` | `executeSubTask` system prompt 注入 agent 能力描述 + 工具调用参数截断 200→500 |
| `cmd/llm-mcp-agent/llm_client.go` | `logLLMContext` 增加工具参数 schema 摘要 |
