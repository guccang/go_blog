# LLM Agent 和 Skill 筛选流程分析

## 一、整体架构

基于 **渐进式发现（Progressive Discovery）** 的多 Agent 协作系统：

- **llm-agent**: LLM 编排中枢，负责任务规划和工具调度
- **skill-manager**: 技能管理器，封装领域专业能力
- **bridge**: UAP 客户端，负责跨 agent 工具调用
- **planner**: 任务规划器，将复杂任务拆解为子任务 DAG

---

## 二、Agent 筛选流程

### 1. Agent 发现与注册

**位置**: `cmd/llm-agent/bridge.go:806-906`

```
DiscoverAgents() → 从 gateway HTTP API 获取所有在线 agent 元数据
  ├─ 解析 agent 基础信息（ID, Name, Description, Tools）
  ├─ 提取 Meta 扩展能力（models, coding_tools, deploy_targets, ssh_hosts 等）
  └─ 构建 agentInfo map 和 agentTools map
```

**关键数据结构**:
```go
type AgentInfo struct {
    ID, Name, Description string
    ToolNames []string
    Models, CodingTools, DeployTargets, SSHHosts []string
    HostPlatform, HostIP, Workspace string
    // ... 更多能力标签
}
```

### 2. Agent 在线检查机制

**位置**: `cmd/llm-agent/bridge.go:230-239`, `cmd/llm-agent/skill.go:286-317`

```go
// 注入到 SkillManager 的在线检查函数
b.skillMgr.SetAgentOnlineChecker(func(prefix string) bool {
    b.catalogMu.RLock()
    defer b.catalogMu.RUnlock()
    for agentID := range b.agentInfo {
        if strings.HasPrefix(agentID, prefix) {
            return true  // 找到匹配前缀的在线 agent
        }
    }
    return false
})
```

**用途**:
- Skill 声明 `agents: go_blog, exec_code` 时，系统检查这些前缀的 agent 是否在线
- 不在线的 skill 在目录中标注为 `~~不可用~~`

### 3. Agent 目录构建

**位置**: `cmd/llm-agent/bridge.go:1219-1284`

系统提示词中注入的 Agent 目录格式：
```
## 可用 Agent
需要使用某 agent 的工具时，先调用 get_agent_tools(agent_id) 获取完整工具列表。

- **llm-agent** [llm_mcp]: LLM 编排中枢 (平台: macOS)
- **go_blog** [go_blog]: 博客管理 (15个工具) | 部署: ssh-prod,ssh-test | 模型: default,deepseek
- **exec_code** [exec_code]: Python 代码执行 (3个工具) | Python: 3.11.5
```

**关键能力标签**: 部署目标、SSH 主机、可用模型、编码工具、日志源等

---

## 三、Skill 筛选流程

### 1. Skill 加载

**位置**: `cmd/llm-agent/skill.go:42-87`

```
Load() → 扫描 workspace/skills/*/SKILL.md
  ├─ 解析 YAML frontmatter（name, description, tools, agents, keywords）
  ├─ 提取 Markdown 正文（技能详细文档）
  └─ 构建 SkillEntry 列表
```

**SKILL.md 示例**:
```yaml
---
name: coding
description: 编码任务（新建项目、修改代码、调试）
tools: CodegenStartSession, CodegenSendMessage
agents: go_blog
keywords: 编码, 开发, 代码, 项目
---
# 编码技能详细说明
...
```

### 2. Skill 可用性过滤

**位置**: `cmd/llm-agent/skill.go:175-184`, `skill.go:291-303`

```go
GetAvailableSkills() []SkillEntry {
    for _, skill := range sm.skills {
        if sm.isSkillAvailable(&skill) {  // 检查所需 agent 是否全部在线
            available = append(available, skill)
        }
    }
}
```

**过滤逻辑**:
- 遍历 skill 的 `agents` 字段（如 `["go_blog", "exec_code"]`）
- 调用 `agentOnlineChecker(prefix)` 检查每个前缀
- 任一 agent 离线 → skill 不可用

### 3. Skill 目录构建

**位置**: `cmd/llm-agent/skill.go:239-269`

系统提示词中注入的 Skill 目录：
```
## 可用技能
当用户请求匹配以下技能时，调用 execute_skill 工具执行。
使用前可调用 get_skill_detail(skill_name) 查看详细文档。

- **coding**: 编码任务 — 支持 claudecode/opencode 两种工具
  适用: 编码, 开发, 代码, 项目
- ~~**deploy**~~: 部署任务 [不可用: agent deploy_agent offline]
```

**标注规则**:
- 可用 skill：显示 description + summary + keywords
- 不可用 skill：删除线标注 + 离线 agent 列表

---

## 四、任务拆解执行流程

### 阶段 1：规划（Planner）

**位置**: `cmd/llm-agent/processor.go:1000-1073`

```
用户请求 → processTask() → handlePlanAndExecute()
  ├─ 1. 获取可用 skill（过滤离线 agent）
  ├─ 2. 构建 skillBlock（注入规划提示词）
  ├─ 3. 调用 PlanTask() 生成子任务 DAG
  └─ 4. 返回 TaskPlan（subtasks, execution_mode, reasoning）
```

**规划提示词关键部分** (`cmd/llm-agent/planner.go:96-153`):
```
## 可用工具
- CodegenStartSession: 启动编码会话 [参数: project(项目名)[必填], model, tool]
- DeployProject: 部署项目 [参数: project_dir[必填], target[必填]]
...

## 领域指引（来自 skill）
### coding
使用 CodegenStartSession 时：
- project 参数必须使用描述性名称（如 helloworld-web），禁止使用 account
- tool 参数可选 claudecode/opencode，根据 agent 能力选择
...

## 核心规划原则
1. 优先使用 ExecuteCode 合并操作（数据获取+分析）
2. 最大化并行执行（无依赖的子任务 depends_on 为空）
3. 精简子任务数量（通常 2-3 个）
```

**输出示例**:
```json
{
  "subtasks": [
    {
      "id": "t1",
      "title": "启动编码会话创建项目",
      "description": "调用 CodegenStartSession(project='blog-api', tool='claudecode', account='xxx')",
      "depends_on": [],
      "tools_hint": ["CodegenStartSession"]
    },
    {
      "id": "t2",
      "title": "部署到生产环境",
      "description": "使用 t1 返回的 project_dir 调用 DeployProject(target='ssh-prod')",
      "depends_on": ["t1"],
      "tools_hint": ["DeployProject"]
    }
  ],
  "execution_mode": "dag"
}
```

### 阶段 2：审查（Reviewer）

**位置**: `cmd/llm-agent/processor.go:1106-1142`

```
ReviewPlan() → 注入 agentCapabilities（agent 详细能力信息）
  ├─ 检查工具参数完整性（如 DeployProject 的 target 是否在可用列表中）
  ├─ 优化子任务描述（补充缺失的上下文）
  └─ 返回 "continue" 或 "optimize"（附优化后的计划）
```

**审查提示词关键部分** (`cmd/llm-agent/planner.go:420-520`):
```
## Agent 能力信息
- **go_blog** [go_blog]: 博客管理
  部署目标:
    - ssh-prod → root@114.115.214.86
    - ssh-test → root@192.168.1.100
  可用模型: default, deepseek
  编码工具: claudecode, opencode

## 审查重点
1. 工具参数完整性（target 必须在 deploy_targets 中）
2. 依赖关系合理性（t2 依赖 t1 的 project_dir）
3. 子任务描述清晰度（是否包含足够上下文）
```

### 阶段 3：执行（Orchestrator）

**位置**: `cmd/llm-agent/orchestrator.go:334-450`

```
Execute() → 事件驱动 DAG 调度
  ├─ 1. 初始化 dagScheduler（管理依赖解锁）
  ├─ 2. 并发执行无依赖子任务（maxParallelSubtasks=3）
  ├─ 3. 子任务完成 → markDone() → 解锁后续任务
  └─ 4. 失败处理 → MakeFailureDecision()（retry/skip/abort/modify）
```

**子任务执行流程** (`cmd/llm-agent/orchestrator.go:145-151`, `processor.go:1145-1200`):
```
executeSubTask()
  ├─ 1. 加载子任务 system prompt（workspace/SUBTASK.md）
  ├─ 2. 注入兄弟任务结果（enriched sibling context）
  ├─ 3. 按 tools_hint 过滤工具（ApplySubtaskPolicy）
  ├─ 4. LLM 循环调用工具
  └─ 5. 检测异步会话（status: in_progress → 标记为 async）
```

**工具过滤策略** (`cmd/llm-agent/tool_policy.go:30-58`):
```go
ApplySubtaskPolicy(tools, hints) {
    // 保留 hints 中的工具 + 基础工具（ExecuteCode, Bash, 文件操作）
    for _, tool := range tools {
        if hintSet[tool.Name] || isBaseTool(tool.Name) {
            filtered = append(filtered, tool)
        }
    }
}
```

### 阶段 4：失败决策

**位置**: `cmd/llm-agent/planner.go:210-302`

```
MakeFailureDecision(subtask, errorMsg, completedResults)
  ├─ retry: 临时错误（超时、网络、agent_offline）
  ├─ modify: 参数/代码错误（Python 语法错误、TypeError）→ 修正后重试
  ├─ skip: 非关键子任务
  └─ abort: 关键步骤失败且无法修复
```

---

## 五、关键设计亮点

### 1. 渐进式工具发现

**位置**: `cmd/llm-agent/processor.go:318-335`

```
初始状态：只加载基础工具（ExecuteCode, Bash, 文件操作）
LLM 需要时：调用 get_agent_tools(agent_id) 动态加载
优势：减少初始 token 消耗，避免工具列表过长
```

### 2. Skill 与 Agent 解耦

```
Skill 声明：agents: ["go_blog", "exec_code"]
运行时检查：agentOnlineChecker(prefix) → 动态过滤
好处：agent 下线时自动隐藏相关 skill，避免规划出不可执行任务
```

### 3. 两级工具路由

**位置**: `cmd/llm-agent/bridge.go:1428-1520`

```
DispatchTool(toolName, args)
  ├─ 1. 查找 toolCatalog[toolName] → agentID
  ├─ 2. 参数路由（account/target → 选择目标 agent）
  └─ 3. 发送 UAP MsgToolCall → 等待 MsgToolResult
```

### 4. DAG 调度器

**位置**: `cmd/llm-agent/orchestrator.go:230-332`

```
dagScheduler
  ├─ completedSet: 已完成子任务
  ├─ failedSet: 失败/跳过子任务
  ├─ asyncSet: 异步子任务（等待外部完成）
  └─ markDone() → 解锁 depends_on 满足的子任务
```

---

## 六、执行流程示例

**用户请求**: "用 claudecode 创建一个 blog-api 项目并部署到生产环境"

```
1. 规划阶段
   ├─ 匹配 skill: coding（agents: go_blog ✓）
   ├─ 注入 skillBlock: "project 参数禁止使用 account"
   └─ 生成计划:
       t1: CodegenStartSession(project='blog-api', tool='claudecode')
       t2: DeployProject(project_dir=<t1.project_dir>, target='ssh-prod')

2. 审查阶段
   ├─ 注入 agentCapabilities: go_blog 的 deploy_targets=['ssh-prod', 'ssh-test']
   ├─ 检查 target='ssh-prod' ✓ 在可用列表中
   └─ 审查通过

3. 执行阶段
   ├─ t1 执行:
   │   ├─ 过滤工具: CodegenStartSession + 基础工具
   │   ├─ LLM 调用 CodegenStartSession → 返回 {project_dir: "/path/to/blog-api"}
   │   └─ 标记完成 → 解锁 t2
   ├─ t2 执行:
   │   ├─ 注入 t1 结果: "关键工具返回数据: project_dir=/path/to/blog-api"
   │   ├─ LLM 调用 DeployProject(project_dir='/path/to/blog-api', target='ssh-prod')
   │   └─ 检测 status='in_progress' → 标记为 async

4. 综合阶段
   └─ 汇总结果: "项目已创建并提交部署，session_id=xxx"
```

---

## 七、核心代码位置总结

| 功能 | 文件 | 关键函数 | 行号 |
|------|------|----------|------|
| Agent 发现 | bridge.go | DiscoverAgents() | 806-906 |
| Agent 在线检查 | bridge.go, skill.go | SetAgentOnlineChecker(), isSkillAvailable() | 230-239, 291-303 |
| Skill 加载 | skill.go | Load() | 42-87 |
| Skill 过滤 | skill.go | GetAvailableSkills() | 175-184 |
| 任务规划 | planner.go | PlanTask() | 54-208 |
| 计划审查 | planner.go | ReviewPlan() | 420-559 |
| DAG 执行 | orchestrator.go | Execute() | 334-450 |
| 子任务执行 | orchestrator.go | executeSubTask() | 145-151 |
| 工具过滤 | tool_policy.go | ApplySubtaskPolicy() | 30-58 |
| 失败决策 | planner.go | MakeFailureDecision() | 211-302 |

---

## 八、总结

该系统通过 **动态 agent 发现 + skill 可用性过滤 + 渐进式工具加载 + DAG 调度** 实现了灵活的多 agent 协作。

**核心优势**:
1. 根据 agent 在线状态自动调整可用能力
2. 避免规划出不可执行的任务
3. 最小化初始 token 消耗（渐进式发现）
4. 支持并行执行和失败自动恢复
