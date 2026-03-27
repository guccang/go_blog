# Multi-Agent 多 Agent 架构文档

## 1. 架构概览

系统采用**多 Agent 分布式架构**，以 **UAP Gateway** 为消息中枢，连接多种专业化 Agent 协同工作。

```
                         ┌──────────────────────┐
                         │   Enterprise WeChat  │
                         │    (企业微信回调)      │
                         └──────────┬───────────┘
                                    │ HTTP Callback
                         ┌──────────▼───────────┐
                         │    WeChat Agent      │
                         │   (wechat-agent)     │
                         │  消息路由 / 通知推送   │
                         └──────────┬───────────┘
                                    │ UAP WebSocket
┌──────────────┐          ┌─────────▼───────────┐          ┌──────────────────┐
│  Deploy Agent │◄────────►│    UAP  Gateway     │◄────────►│    ACP Agent     │
│ (deploy-agent)│  UAP WS │   (消息路由中枢)     │  UAP WS  │  (acp-agent)     │
└──────────────┘          └─────────┬───────────┘          └──────────────────┘
                                    │
                         ┌──────────▼───────────┐
                         │   codegen-agent     │
                         │  Claude Code 会话    │
                         └──────────┬───────────┘
                                    │
┌──────────────┐          ┌─────────▼───────────┐
│ execute-code │◄────────►│    LLM Agent        │
│   -agent     │  UAP WS  │   (llm-agent)       │
└──────────────┘          │  LLM 编排中枢        │
                          │  plan_and_execute   │
                          │  DAG 任务编排        │
                          └─────────────────────┘
```

---

## 2. Agent 类型总览

| Agent | AgentType | 代码位置 | 核心职责 |
|-------|-----------|----------|----------|
| **Gateway** | gateway | `cmd/gateway/` | 消息路由、WebSocket 中介、健康检查 |
| **llm-agent** | llm_mcp | `cmd/llm-agent/` | LLM 编排中枢、任务规划、DAG 执行 |
| **acp-agent** | acp | `cmd/acp-agent/` | Claude Code 会话管理、代码生成 |
| **deploy-agent** | deploy | `cmd/deploy-agent/` | 项目构建、部署、Pipeline 编排 |
| **wechat-agent** | wechat | `cmd/wechat-agent/` | 企业微信消息收发、通知推送 |
| **codegen-agent** | codegen | `cmd/codegen-agent/` | 远程编码 Agent 池管理 |
| **execute-code-agent** | execute_code | `cmd/execute-code-agent/` | Python 沙箱代码执行 |
| **mcp-agent** | mcp | `cmd/mcp-agent/` | MCP 工具服务器 |
| **cron-agent** | cron | `cmd/cron-agent/` | 定时任务调度 |
| **log-agent** | log | `cmd/log-agent/` | 日志收集 |

---

## 3. UAP 协议层

### 3.1 消息信封

```go
type Message struct {
    Type    string          `json:"type"`   // 消息类型
    ID      string          `json:"id"`     // 唯一消息 ID（请求-响应关联）
    From    string          `json:"from"`   // 源 agent ID
    To      string          `json:"to"`     // 目标 agent ID
    Payload json.RawMessage `json:"payload"`
    Ts      int64           `json:"ts"`    // 毫秒时间戳
}
```

### 3.2 核心消息类型

| 类型 | 说明 |
|------|------|
| `register` / `register_ack` | Agent 注册与确认 |
| `heartbeat` / `heartbeat_ack` | 心跳保活（15s 间隔） |
| `tool_call` / `tool_result` | 跨 Agent 工具调用 |
| `task_assign` / `task_accepted` / `task_rejected` | 任务分派与响应 |
| `task_event` / `task_complete` | 任务进度与完成通知 |
| `task_stop` | 停止任务 |
| `notify` | 单向通知 |
| `permission_request` / `permission_response` | Claude Mode 权限交互 |
| `ctrl_shutdown` / `ctrl_status` | 控制协议 |

### 3.3 Gateway 路由规则

```
有 To 字段 → 查找目标 Agent WebSocket 连接 → 转发消息
无 To 字段 → 交给 OnMessage 回调处理
目标离线   → 返回 error 消息（code: agent_offline）
```

---

## 4. Gateway (`cmd/gateway/`)

### 4.1 职责

- WebSocket 服务端（`/ws/uap` 入口）
- Agent 注册表管理
- 消息路由转发
- 心跳健康检查（120s 超时清理）
- 事件追踪（可选）

### 4.2 核心组件

```
Gateway
├── Server (uap.Server)     WebSocket 服务，agent 连接管理
├── Registry                Agent 注册表，agent 在线状态
├── Router                  消息路由器，OnMessage 回调
├── Tracker                 事件追踪（可选）
└── Proxy                   HTTP 反向代理到 blog-agent
```

### 4.3 HTTP API

| 端点 | 说明 |
|------|------|
| `GET /api/gateway/agents` | 所有在线 Agent |
| `GET /api/gateway/tools` | 所有在线 Agent 的工具列表 |
| `GET /api/gateway/health` | 健康检查 |
| `WS /ws/uap` | Agent WebSocket 连接 |

---

## 5. llm-agent (`cmd/llm-agent/`)

### 5.1 职责

- **LLM 编排中枢**：接收用户请求，调用 MCP 工具
- **任务规划**：复杂任务拆解为 DAG 子任务
- **工具发现**：从 Gateway 动态获取所有在线 Agent 工具
- **Agentic Loop**：观察→思考→行动→再观察 的自主代理循环

### 5.2 处理流程

```
用户请求
     ↓
processTask()
     │
     ├─ 简单任务 ─→ Agentic Loop ──────────────────→ 返回结果
     │
     └─ plan_and_execute 触发
          ↓
       PlanTask()       → LLM 生成任务计划
          ↓
       ReviewPlan()     → LLM 审查计划
          ↓
       Execute()        → DAG 拓扑编排执行
          │
          ├─ 子任务 t1 ──→ executeSubTask() ── Agentic Loop
          ├─ 子任务 t2 ──→ executeSubTask() ── Agentic Loop
          └─ ...
          ↓
       Synthesize()     → LLM 汇总结果
```

### 5.3 核心文件

| 文件 | 职责 |
|------|------|
| `processor.go` | 统一消息处理，简单/复杂任务分流 |
| `orchestrator.go` | DAG 编排执行，Agentic Loop |
| `planner.go` | 任务规划，计划审查，失败决策 |
| `bridge.go` | 工具发现，跨 Agent 调用 |
| `skill_executor.go` | 技能子任务执行 |
| `llm_client.go` | LLM API 调用 |

---

## 6. acp-agent (`cmd/acp-agent/`)

### 6.1 职责

- Claude Code 会话管理（基于 ACP SDK）
- 代码生成、编辑、测试
- 多轮 Prompt 对话
- 项目管理（创建/扫描/切换）

### 6.2 核心结构

```go
type Agent struct {
    ID      string
    sessions map[string]*sessionRecord  // ACP 会话记录
    completionChs map[string]chan taskResult  // 同步等待通道
}

type sessionRecord struct {
    Project    string
    Status     string  // in_progress/completed/failed/stopped
    ACPSession *ACPSession
    ACPClient  *ACPClientImpl
}
```

### 6.3 工具列表

| 工具名 | 说明 |
|--------|------|
| `AcpStartSession` | 创建 Claude Code 会话 |
| `AcpSendMessage` | 发送消息到会话 |
| `AcpAnalyzeProject` | 分析项目结构 |
| `AcpStopSession` | 停止会话 |

---

## 7. deploy-agent (`cmd/deploy-agent/`)

### 7.1 职责

- 项目构建（交叉编译 Linux/macOS/Windows）
- 打包（tar.gz）
- 部署（本地/SSH 远程）
- Pipeline 编排
- 部署后 HTTP 验证

### 7.2 部署模式

| 模式 | 说明 |
|------|------|
| `local` | 本地部署 |
| `ssh-prod` | SSH 远程部署 |
| `adhoc` | 一次性部署（无需配置文件） |

### 7.3 工具列表

| 工具名 | 说明 |
|--------|------|
| `DeployProject` | 部署项目 |
| `DeployAdhoc` | 一次性部署 |
| `DeployPipeline` | 执行部署编排 |

---

## 8. wechat-agent (`cmd/wechat-agent/`)

### 8.1 职责

- 企业微信消息接收（HTTP Callback）
- 消息路由（命令 → go_blog，自然语言 → llm-agent）
- 通知推送（`wechat.SendMessage`、`wechat.SendMarkdown`）
- 编码/部署进度中继

### 8.2 工具列表

| 工具名 | 说明 |
|--------|------|
| `wechat.SendMessage` | 发送文本消息给指定用户 |
| `wechat.SendMarkdown` | 推送 Markdown 到群 |

---

## 9. execute-code-agent (`cmd/execute-code-agent/`)

### 9.1 职责

- Python 沙箱执行
- 通过 `call_tool()` 调用 MCP 工具
- 工具调用结果透传

### 9.2 核心机制

Python 代码中通过 `call_tool()` 调用工具：

```python
# 注入的桥接代码
def call_tool(tool_name, arguments=None):
    request = json.dumps({"type": "tool_call", "tool": tool_name, "args": arguments or {}})
    print(f"__TOOL_CALL__{request}__END__", flush=True)
    line = sys.stdin.readline().strip()
    result = json.loads(line)
    if not result.get("success"):
        raise Exception(f"Tool {tool_name} failed: {result.get('error')}")
    return _auto_parse(result.get("data"))  # 自动解析 JSON
```

Executor 捕获 `__TOOL_CALL__...__END__` 协议，调用 `callTool` 函数并将结果写回 stdin。

---

## 10. 工具调用流程

### 10.1 llm-agent → deploy-agent

```
llm-agent                      Gateway                    deploy-agent
    │                              │                            │
    │─── tool_call ──────────────▶│                            │
    │                              │──── tool_call ─────────────▶│
    │                              │◀─── tool_result ───────────│
    │◀── tool_result ─────────────│                            │
```

### 10.2 ExecuteCode 内部 call_tool

```
llm-agent                      execute-code-agent
    │◀── ExecuteCode 调用 ──────│
    │                           │
    │                           │ Python 代码执行
    │                           │   call_tool("RawGetTodos", {...})
    │                           │◀─── tool_result ─────────────│
    │◀── ExecuteCode 结果 ──────│
```

---

## 11. 通信协议架构

### 11.1 UAP 协议（通用）

所有 Agent 通过 UAP WebSocket 连接 Gateway：

```
Agent ──── UAP WebSocket ──── Gateway ──── UAP WebSocket ──── Agent
                          │
                          └────── OnMessage 回调（无 To 字段消息）
```

### 11.2 Agent 注册信息

```go
type RegisterPayload struct {
    AgentID      string         `json:"agent_id"`
    AgentType    string         `json:"agent_type"`
    Name         string         `json:"name"`
    Description  string         `json:"description"`
    HostPlatform string         `json:"host_platform"`
    HostIP       string         `json:"host_ip"`
    Workspace    string         `json:"workspace"`
    Tools        []ToolDef      `json:"tools"`
    Capacity     int            `json:"capacity"`
    Meta         map[string]any `json:"meta"`
    AuthToken    string         `json:"auth_token"`
}
```

---

## 12. 任务编排（DAG）

### 12.1 TaskPlan 结构

```go
type TaskPlan struct {
    SubTasks      []SubTaskPlan `json:"subtasks"`
    ExecutionMode string        `json:"execution_mode"` // sequential/parallel/dag
    Reasoning     string        `json:"reasoning"`
}

type SubTaskPlan struct {
    ID          string                 `json:"id"`
    Title       string                 `json:"title"`
    Description string                 `json:"description"`
    DependsOn   []string               `json:"depends_on"`
    ToolsHint   []string               `json:"tools_hint"`
    ToolParams  map[string]interface{} `json:"tool_params"`
}
```

### 12.2 DAG 执行

- 无依赖子任务自动并行执行（`maxParallelSubtasks` 控制并发数）
- 依赖完成/失败/异步执行中，都视为"已解决"
- 失败后 LLM 决策 retry/skip/modify/abort

---

## 13. 健康检查与容错

### 13.1 心跳机制

- Agent 每 15s 发送 `heartbeat`
- Gateway 30s 检测一次超时
- 120s 无心跳则移除 Agent

### 13.2 自动重连

Agent 端指数退避重连：
```
1s → 2s → 5s → 10s → 30s → 60s（最大）
```

### 13.3 Agent 离线广播

当 Agent 离线时，Gateway 向所有其他在线 Agent 广播 `agent_offline` 通知，使其立即移除离线 Agent。

---

## 14. 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Enterprise WeChat                         │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │ HTTP Callback
┌─────────────────────────────────────▼───────────────────────────────┐
│                          wechat-agent                               │
│  - 消息接收/发送  - 进度推送  - 限流（10s/session）                  │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │ UAP WebSocket
┌─────────────────────────────────────▼───────────────────────────────┐
│                          UAP Gateway                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │
│  │  Registry    │  │   Router     │  │     HealthCheck          │ │
│  │  (agent管理)  │  │   (消息路由)  │  │   (120s 超时清理)        │ │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘ │
│                           │                                         │
│         ┌─────────────────┼─────────────────┐                      │
│         │                 │                 │                       │
└─────────▼─────────────────▼─────────────────▼───────────────────────┘
          │                 │                 │
    ┌─────▼─────┐     ┌─────▼─────┐    ┌─────▼─────┐
    │ llm-agent │     │ acp-agent │    │ deploy-   │
    │           │     │           │    │ agent     │
    │ LLM 编排   │     │ Claude    │    │           │
    │ 工具发现   │     │ Code 会话  │    │ 构建部署   │
    │ DAG 编排   │     │            │    │ Pipeline  │
    └───────────┘     └───────────┘    └───────────┘
          │                 │                 │
          │          ┌───────▼───────┐        │
          │          │ codegen-     │        │
          │          │ agent        │        │
          │          │ (Agent池)    │        │
          │          └─────────────┘        │
          │                                   │
          │    ┌─────────────────────────┐    │
          └────│ execute-code-agent      │────┘
               │ (Python 沙箱)           │
               │ call_tool() 工具调用    │
               └─────────────────────────┘
```
