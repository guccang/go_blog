# 系统架构文档

## 概览

go_blog 采用 **多 Agent 分布式架构**，以 **UAP Gateway** 为消息中枢，连接 4 种 Agent 协同工作：

```
                        ┌──────────────────────┐
                        │   Enterprise WeChat  │
                        └──────────┬───────────┘
                                   │ HTTP Callback
                        ┌──────────▼───────────┐
                        │    WeChat Agent      │
                        │  (wechat-agent)      │
                        └──────────┬───────────┘
                                   │ UAP WebSocket
┌─────────────┐         ┌──────────▼───────────┐         ┌─────────────────┐
│ Deploy Agent│◄───────►│    UAP  Gateway      │◄───────►│  LLM-MCP Agent  │
│(deploy-agent)│  UAP WS │   (消息路由中枢)      │  UAP WS │(llm-agent)  │
└─────────────┘         └──────────┬───────────┘         └─────────────────┘
                                   │ UAP WebSocket
                        ┌──────────▼───────────┐
                        │    Go Blog Server    │
                        │   (go_blog-agent)    │
                        │                      │
                        │  ┌────────────────┐  │
                        │  │  CodeGen 模块   │  │         ┌─────────────────┐
                        │  │  Agent Pool    │  │         │   Web Browser   │
                        │  │  Session Mgmt  │  │◄───────►│  /codegen 前端   │
                        │  └────────────────┘  │  HTTP   └─────────────────┘
                        │  ┌────────────────┐  │  + WS
                        │  │  MCP 工具系统   │  │
                        │  │  Blog/Todo/..  │  │
                        │  └────────────────┘  │
                        └──────────────────────┘
```

---

## 1. UAP 协议层 (`pkgs/uap/`)

**UAP (Universal Agent Protocol)** 是底层通用消息路由协议，所有 Agent 通过 UAP WebSocket 连接到 Gateway。

### 1.1 消息信封

```go
type Message struct {
    Type    string          `json:"type"`    // 消息类型
    ID      string          `json:"id"`      // 请求-响应关联 ID
    From    string          `json:"from"`    // 源 Agent ID（Gateway 填充）
    To      string          `json:"to"`      // 目标 Agent ID（Gateway 路由）
    Payload json.RawMessage `json:"payload"` // 类型特定载荷
    Ts      int64           `json:"ts"`      // 毫秒时间戳
}
```

### 1.2 消息类型

| 类型 | 方向 | 用途 |
|------|------|------|
| `register` / `register_ack` | Agent → Gateway → Agent | Agent 注册/确认 |
| `heartbeat` / `heartbeat_ack` | Agent ↔ Gateway | 心跳保活（15s 间隔） |
| `tool_call` / `tool_result` | Agent → Gateway → Agent | 跨 Agent 工具调用 |
| `task_assign` / `task_accepted` | Agent → Gateway → Agent | 长任务分派/接受 |
| `task_rejected` / `task_stop` | Agent → Gateway → Agent | 任务拒绝/停止 |
| `task_event` | Agent → Gateway → Agent | 任务进度事件 |
| `task_complete` | Agent → Gateway → Agent | 任务完成通知 |
| `notify` | Agent → Gateway → Agent | 单向通知（微信等） |
| `error` | Gateway → Agent | 错误响应 |

### 1.3 Gateway 路由规则

- **有 `To` 字段**：Gateway 查找目标 Agent 的 WebSocket 连接，转发消息
- **无 `To` 字段**：交给 Gateway 的 `OnMessage` 回调处理
- **目标离线**：返回 `error` 消息，code = `agent_offline`

### 1.4 Client SDK

```go
client := uap.NewClient(gatewayURL, agentID, agentType, agentName)
client.AuthToken = "shared_secret"
client.Capacity = 4
client.Tools = toolDefs        // 注册工具定义
client.OnMessage = handler     // 消息回调
client.Run()                   // 阻塞，内置自动重连 + 心跳
```

- 自动重连：指数退避 1s → 60s
- 心跳：每 15s 发送 `heartbeat`
- 健康检查：Gateway 端 30s 超时清理

---

## 2. Agent 类型

### 2.1 Go Blog Server (`go_blog`)

| 属性 | 值 |
|------|-----|
| AgentType | `go_blog` |
| AgentID | `go_blog` |
| 注册工具 | 所有 MCP 回调（Blog/Todo/Exercise 等） |
| 代码位置 | `pkgs/codegen/gateway.go` |

**职责**：
- 系统后端，HTTP API 服务
- MCP 工具执行中枢（Blog CRUD、Todo、Exercise 等）
- CodeGen 会话管理（Agent Pool、Session 生命周期）
- 接收微信命令路由（`cg` 命令拦截）
- 转发 `stream_event` / `task_complete` 给 wechat-agent

**关键模块**：

| 模块 | 路径 | 功能 |
|------|------|------|
| CodeGen | `pkgs/codegen/` | 编码会话管理、远程 Agent 池、协议处理 |
| Gateway Bridge | `pkgs/codegen/gateway.go` | UAP 连接 + 工具调用桥接 |
| HTTP API | `pkgs/http/http_codegen.go` | REST + WebSocket 端点 |
| WeChat Bridge | `pkgs/codegen/wechat.go` | 微信通知中继 |
| Agent 命令 | `pkgs/agent/agent.go` | `cg` 命令解析与执行 |

### 2.2 Deploy Agent (`deploy`)

| 属性 | 值 |
|------|-----|
| AgentType | `deploy` |
| AgentID | `deploy_<name>_<pid>` |
| 注册工具 | `["deploy"]` |
| 代码位置 | `cmd/deploy-agent/` |

**职责**：
- 项目构建（交叉编译 Linux/macOS/Win）
- 打包（tar.gz）
- 部署（本地/SSH 远程）
- Pipeline 编排（多步骤顺序部署）
- 部署后 HTTP 验证

**注册上报字段**：
```json
{
    "projects":       ["go_blog", "myapp"],
    "tools":          ["deploy"],
    "deploy_targets": ["local", "ssh-prod"],
    "host_platform":  "win",
    "pipelines":      ["prod-all", "staging"]
}
```

### 2.3 LLM-MCP Agent (`llm_mcp`)

| 属性 | 值 |
|------|-----|
| AgentType | `llm_mcp` |
| AgentID | `llm_<name>` |
| 注册工具 | 无（动态发现） |
| 代码位置 | `cmd/llm-agent/` |

**职责**：
- LLM 推理引擎（多模型支持）
- 工具发现：启动时从 Gateway HTTP API 获取所有可用工具
- 多轮工具调用循环（最多 15 轮迭代）
- 智能工具路由：工具数 > 15 时用 LLM 筛选相关工具
- 处理微信自然语言查询
- 处理同步 LLM 请求（go_blog 委托的推理任务）

**任务类型**：
- `assistant_chat`：微信对话（注入用户上下文 + 工具调用）
- `llm_request`：直接 LLM 调用（预构建消息 + 指定工具集）

### 2.4 WeChat Agent (`wechat`)

| 属性 | 值 |
|------|-----|
| AgentType | `wechat` |
| AgentID | `wechat-<name>` |
| 注册工具 | `["wechat.SendMessage", "wechat.SendMarkdown"]` |
| 代码位置 | `cmd/wechat-agent/` |

**职责**：
- 企业微信 Callback 接收
- 消息路由（命令 → go_blog，自然语言 → llm-agent）
- 编码/部署进度中继（`stream_event` → 微信推送）
- 限流推送（每 session 最多 10s/次）
- 提供工具：`wechat.SendMessage`、`wechat.SendMarkdown`

---

## 3. CodeGen 模块架构

### 3.1 会话生命周期

```
创建 (StartSession)
  │
  ▼
running ────────────────────────► done
  │         远程 Agent 完成          │
  │                                  │
  ├──► error (Agent 失败/离线)       │
  │                                  │
  └──► stopped (用户手动停止)        │
                                     ▼
                              清理 (1h 后自动/上限 50)
```

### 3.2 CodeSession 核心字段

```go
type CodeSession struct {
    ID            string           // "cg_<timestamp>"
    Project       string           // 项目名
    Prompt        string           // 用户需求
    Model         string           // 模型配置名
    Tool          string           // "claudecode" | "opencode"
    AutoDeploy    bool             // 编码完成后自动部署
    DeployOnly    bool             // 仅部署模式
    Pipeline      string           // Pipeline 编排名
    Status        SessionStatus    // running | done | error | stopped
    AgentID       string           // 执行此任务的远程 Agent
    Messages      []SessionMessage // 对话历史
    CostUSD       float64          // 累计 LLM 费用
    subscribers   []chan StreamEvent // WebSocket 广播通道
}
```

### 3.3 Agent Pool

```go
type AgentPool struct {
    agents  map[string]*RemoteAgent        // agentID → Agent
    pending map[string]chan json.RawMessage // requestID → 响应通道
}
```

**核心方法**：
- `SelectAgent(project, tool)` — 按项目 + 工具匹配，负载均衡选择
- `Execute(session)` — 分派任务给远程 Agent
- `ListRemoteProjects()` — 聚合所有 Agent 的项目列表
- `ListPipelines()` — 聚合所有 deploy Agent 的 Pipeline 列表
- `ReadRemoteFile()` / `ReadRemoteTree()` — 远程文件读取（请求-响应模式）

### 3.4 两套协议共存

| 协议 | 适用范围 | 标识字段 |
|------|---------|---------|
| **CodeGen 协议** | codegen-agent / deploy-agent | `session_id` |
| **UAP 协议** | llm-agent 任务 | `task_id` |

Gateway Bridge 在处理 `task_accepted` / `task_complete` 时兼容两种协议：
```go
var raw struct {
    SessionID string `json:"session_id"` // codegen 协议
    TaskID    string `json:"task_id"`    // UAP 协议
}
```

---

## 4. HTTP API 端点

### 4.1 CodeGen REST API

| 端点 | 方法 | 功能 | 关键参数 |
|------|------|------|---------|
| `/codegen` | GET | Web UI 页面 | — |
| `/api/codegen/projects` | GET | 项目列表 + Agent 状态 + 模型配置 | — |
| `/api/codegen/projects` | POST | 创建远程项目 | `name`, `agent` |
| `/api/codegen/run` | POST | 启动编码/部署/Pipeline 会话 | `project`, `prompt`, `pipeline`, ... |
| `/api/codegen/message` | POST | 追加消息（续接会话） | `session_id`, `prompt` |
| `/api/codegen/sessions` | GET | 列出活跃会话 | — |
| `/api/codegen/stop` | POST | 停止会话 | `session_id` |
| `/api/codegen/tree` | GET | 远程项目目录树 | `project` |
| `/api/codegen/file` | GET | 远程项目文件内容 | `project`, `path` |

### 4.2 WebSocket 端点

| 端点 | 功能 |
|------|------|
| `/ws/codegen?session_id=X` | 前端实时事件流（编码/部署进度） |
| `/ws/uap` | UAP Agent WebSocket 连接 |
| `/ws/agent` | 旧版 Agent 直连 WebSocket（兼容） |

### 4.3 /api/codegen/run 请求体

```json
{
    "project":        "go_blog",
    "prompt":         "添加健康检查接口",
    "model":          "sonnet",
    "tool":           "claudecode",
    "agent_id":       "",
    "auto_deploy":    true,
    "deploy_only":    false,
    "deploy_target":  "ssh-prod",
    "build_platform": "linux",
    "pack_only":      false,
    "pipeline":       "prod-all"
}
```

---

## 5. 前端架构 (`/codegen`)

### 5.1 页面结构

```
┌──────────────────────────────────────────────────┐
│  Header: 标题 · Agent 状态 · 返回按钮             │
├──────────┬───────────────────────────────────────┤
│ Sidebar  │  Main Content                         │
│          │                                       │
│ 搜索框   │  输出区域 (outputArea)                 │
│          │  ┌─────────────────────────────────┐  │
│ Agent 组  │  │ 实时事件流                      │  │
│ ├ 💻 编码 │  │  💭 thinking (可折叠)           │  │
│ ├ 🚀 部署 │  │  🔧 tool (工具调用详情)         │  │
│ └ 🔄 编排 │  │  💬 assistant (AI 回复)         │  │
│          │  │  ✅ result (最终结果)            │  │
│ + 新建   │  │  ⚠️ error (错误信息)            │  │
│          │  └─────────────────────────────────┘  │
│          │                                       │
│          │  输入区域 (三选一)                      │
│          │  ┌─ codeInputArea ─────────────────┐  │
│          │  │ [模型▾] [工具▾] [需求输入] [发送] │  │
│          │  └─────────────────────────────────┘  │
│          │  ┌─ deployInputArea ────────────────┐  │
│          │  │ [目标▾] [平台▾] [☐仅打包] [🚀部署]│  │
│          │  └─────────────────────────────────┘  │
│          │  ┌─ pipelineInputArea ──────────────┐  │
│          │  │        [🔄 执行 Pipeline]         │  │
│          │  └─────────────────────────────────┘  │
├──────────┴───────────────────────────────────────┤
│  Footer: 状态指示 · 运行时间 · 费用统计            │
└──────────────────────────────────────────────────┘
```

### 5.2 三种模式

| 模式 | 入口 | 输入区域 | API 调用 |
|------|------|---------|---------|
| `code` | 侧边栏 💻 项目 | 模型 + 工具 + Prompt | `POST /api/codegen/run` |
| `deploy` | 侧边栏 🚀 项目 | 目标 + 平台 + 打包选项 | `POST /api/codegen/run` (deploy_only) |
| `pipeline` | 侧边栏 🔄 编排 | 一键执行 | `POST /api/codegen/run` (pipeline) |

### 5.3 数据流

```
loadProjects()
  │ GET /api/codegen/projects
  ▼
cachedAllRemote[] ── 按 agent 分组 ──► renderProjects()
  │                                      │
  │  每个项目检查 tools:                   │
  │  ├ claudecode/opencode → 💻 编码条目   │
  │  ├ deploy → 🚀 部署条目               │
  │  └ deploy + pipelines → 🔄 编排条目   │
  │                                      ▼
  │                              侧边栏 HTML
  │
selectProject(name, mode, agentId)
  │ 切换输入区域 (code/deploy/pipeline)
  │ 恢复会话状态
  ▼
sendPrompt() / deployProject() / runPipeline()
  │ POST /api/codegen/run
  ▼
connectWebSocket(sessionId)
  │ WS /ws/codegen?session_id=X
  ▼
实时事件渲染 ──► outputArea
```

---

## 6. 认证与安全

### 6.1 Token 体系

```
gateway_token (统一共享密钥)
  ├── UAP Agent 注册认证
  ├── codegen-agent / deploy-agent 注册认证
  └── go_blog 端 agentToken 校验
```

所有 Agent 使用同一 `gateway_token` 进行双向认证。

### 6.2 Web UI 认证

CodeGen HTTP API 通过 `checkLogin(r)` 验证用户登录状态（Cookie Session）。

---

## 7. 关键文件索引

| 文件 | 功能 |
|------|------|
| `pkgs/uap/protocol.go` | UAP 消息类型与载荷定义 |
| `pkgs/uap/client.go` | UAP Client SDK（连接/心跳/自动重连） |
| `pkgs/uap/server.go` | UAP Gateway Server（路由/健康检查） |
| `pkgs/codegen/codegen.go` | 会话管理、模块初始化 |
| `pkgs/codegen/protocol.go` | CodeGen 协议消息定义 |
| `pkgs/codegen/remote.go` | Agent Pool、远程任务分派 |
| `pkgs/codegen/gateway.go` | Gateway Bridge（UAP ↔ CodeGen 协议适配） |
| `pkgs/codegen/wechat.go` | 微信通知中继 |
| `pkgs/http/http_codegen.go` | HTTP/WebSocket API 端点 |
| `pkgs/agent/agent.go` | `cg` 命令处理、模块初始化 |
| `cmd/deploy-agent/` | Deploy Agent 全部代码 |
| `cmd/llm-agent/` | LLM-MCP Agent 全部代码 |
| `cmd/wechat-agent/` | WeChat Agent 全部代码 |
| `templates/codegen.template` | /codegen 前端页面（HTML/CSS/JS） |
