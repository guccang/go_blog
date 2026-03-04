# 消息数据流程文档

本文档描述各核心场景下的完整消息流转路径、载荷格式和处理逻辑。

---

## 1. Agent 注册流程

### 1.1 Deploy Agent 注册（通过 UAP Gateway）

```
Deploy Agent                    UAP Gateway                   Go Blog
    │                               │                            │
    │──── UAP register ────────────►│                            │
    │     {agent_id, type:"deploy", │                            │
    │      name, auth_token}        │                            │
    │                               │── 验证 token ──            │
    │                               │── 记录连接 ──              │
    │◄─── UAP register_ack ────────│                            │
    │     {success:true}            │                            │
    │                               │                            │
    │──── CodeGen register ────────►│── 路由 To:"go_blog" ─────►│
    │     RegisterPayload {         │                            │
    │       agent_id,               │                            │
    │       name: "win",            │                     handleRegister()
    │       projects: ["go_blog"],  │                            │
    │       tools: ["deploy"],      │                     ┌──────┤
    │       deploy_targets: [...],  │                     │校验token
    │       host_platform: "win",   │                     │检查重名
    │       pipelines: ["prod-all"] │                     │创建 RemoteAgent
    │     }                         │                     │加入 AgentPool
    │                               │                     └──────┤
    │◄─── register_ack ───────────-│◄─── register_ack ─────────│
    │     {success:true}            │     {success:true}         │
    │                               │                            │
```

**注册载荷（deploy-agent → go_blog）**：
```json
{
    "agent_id":       "deploy_win_12345",
    "name":           "win",
    "workspaces":     [],
    "projects":       ["go_blog", "myapp"],
    "tools":          ["deploy"],
    "max_concurrent": 2,
    "auth_token":     "shared_secret",
    "deploy_targets": ["local", "ssh-prod"],
    "host_platform":  "win",
    "pipelines":      ["prod-all", "staging"]
}
```

**Go Blog 处理逻辑** (`gateway.go:handleRegister()`):
1. 反序列化 `RegisterPayload`
2. 验证 `auth_token`（与 `gateway_token` 匹配）
3. 检查是否有同名在线 Agent（拒绝重复）
4. 使用 `msg.From`（Gateway 填充的 UAP ID）作为 AgentID
5. 创建 `RemoteAgent`，填入所有字段（含 Pipelines）
6. 添加到 `AgentPool`
7. 回复 `register_ack {success:true}`

### 1.2 心跳保活

```
Deploy Agent              Go Blog
    │                        │
    │  每 15s                │
    │── heartbeat ──────────►│
    │   {agent_id,           │  更新 LastHeartbeat
    │    active_sessions:1,  │  更新 Projects/Tools（如有变化）
    │    load:0.5}           │
    │◄── heartbeat_ack ─────│
    │                        │
```

**超时清理**：Go Blog 端每 15s 检查，45s 无心跳的 Agent 标记为 offline 并移除。

---

## 2. 编码会话流程（Web UI → Claude Code）

### 2.1 启动编码

```
Browser                    Go Blog HTTP              AgentPool           Codegen Agent
   │                           │                        │                      │
   │ POST /api/codegen/run     │                        │                      │
   │ {project:"myapp",         │                        │                      │
   │  prompt:"写HTTP服务",      │                        │                      │
   │  model:"sonnet",          │                        │                      │
   │  tool:"claudecode"}       │                        │                      │
   │                           │                        │                      │
   │                    HandleCodeGenRun()               │                      │
   │                    StartSession()                   │                      │
   │                           │                        │                      │
   │                           │── Execute(session) ───►│                      │
   │                           │                        │                      │
   │                           │                 SelectAgent("myapp",          │
   │                           │                   "claudecode")               │
   │                           │                  负载均衡选择                   │
   │                           │                        │                      │
   │                           │                        │── task_assign ──────►│
   │                           │                        │   {session_id,       │
   │                           │                        │    project:"myapp",  │
   │                           │                        │    prompt:"写HTTP服务",│
   │                           │                        │    system_prompt:...,│
   │                           │                        │    model:"sonnet",   │
   │                           │                        │    tool:"claudecode"}│
   │                           │                        │                      │
   │◄── {session_id, status} ──│                        │◄── task_accepted ───│
   │                           │                        │                      │
```

### 2.2 实时事件流

```
Browser WS                 Go Blog                     Codegen Agent
   │                           │                            │
   │ WS /ws/codegen            │                            │
   │ ?session_id=cg_xxx        │                            │
   │                           │                            │
   │◄── 历史消息回放 ────────────│                            │
   │    (Messages[])           │                            │
   │                           │                            │
   │                           │◄──── stream_event ────────│
   │                           │      {session_id,          │
   │                           │       event:{              │
   │                           │         type:"thinking",   │
   │                           │         text:"分析需求..."  │
   │                           │       }}                   │
   │                           │                            │
   │                    handleStreamEvent()                  │
   │                    processEvent(session)                │
   │                    session.broadcast()                  │
   │                           │                            │
   │◄── StreamEvent ───────────│                            │
   │    {type:"thinking",      │                            │
   │     text:"分析需求..."}    │                            │
   │                           │                            │
   │                           │◄──── stream_event ────────│
   │                           │      type:"tool"           │
   │◄── StreamEvent ───────────│                            │
   │    {type:"tool",          │                            │
   │     tool_name:"Write",    │                            │
   │     text:"创建 main.go"}  │                            │
   │                           │                            │
   │                           │◄──── task_complete ───────│
   │                           │      {session_id,          │
   │                           │       status:"done"}       │
   │                           │                            │
   │                    handleTaskComplete()                 │
   │                    session.Status = done                │
   │                    session.broadcast(Done)              │
   │                           │                            │
   │◄── StreamEvent ───────────│                            │
   │    {type:"result",        │                            │
   │     text:"✅ 编码完成",    │                            │
   │     cost_usd:0.05,        │                            │
   │     done:true}            │                            │
   │                           │                            │
```

**事件类型**：

| type | 含义 | 前端展示 |
|------|------|---------|
| `thinking` | AI 思考过程 | 可折叠面板 |
| `assistant` | AI 文本回复 | Markdown 渲染 |
| `tool` | 工具调用 | 工具名 + 详情 |
| `system` | 系统消息 | 灰色提示 |
| `result` | 最终结果 | 绿色高亮 |
| `error` | 错误消息 | 红色高亮 |
| `summary` | 摘要 | 同 result |

---

## 3. 部署流程（Web UI → Deploy Agent）

### 3.1 直接部署

```
Browser                    Go Blog                    Deploy Agent
   │                           │                           │
   │ POST /api/codegen/run     │                           │
   │ {project:"go_blog",       │                           │
   │  deploy_only:true,        │                           │
   │  deploy_target:"ssh-prod",│                           │
   │  build_platform:"linux"}  │                           │
   │                           │                           │
   │                    StartSession(deployOnly=true)       │
   │                    tool = ToolDeploy                   │
   │                    SelectAgent("go_blog", "deploy")   │
   │                           │                           │
   │                           │── task_assign ───────────►│
   │                           │   {session_id,            │
   │                           │    project:"go_blog",     │
   │                           │    deploy_only:true,      │
   │                           │    deploy_target:"ssh-prod",
   │                           │    build_platform:"linux"} │
   │                           │                           │
   │                           │◄─ task_accepted ─────────│
   │                           │                           │
   │                           │                    executeDeploy()
   │                           │                    ┌──────┤
   │                           │                    │1. go build
   │◄── system:"🚀 开始部署"───│◄─ stream_event ───│2. tar -czf
   │◄── system:"📦 编译..."───│◄─ stream_event ───│3. scp upload
   │◄── system:"📦 上传..."───│◄─ stream_event ───│4. ssh restart
   │◄── system:"⏳ 等待5s"────│◄─ stream_event ───│5. HTTP verify
   │◄── system:"✅ 验证通过"──│◄─ stream_event ───│
   │                           │                    └──────┤
   │                           │◄─ task_complete ─────────│
   │◄── result:{done:true} ────│  {status:"done"}         │
   │                           │                           │
```

### 3.2 Pipeline 编排

```
Browser                    Go Blog                    Deploy Agent
   │                           │                           │
   │ POST /api/codegen/run     │                           │
   │ {pipeline:"prod-all"}     │                           │
   │                           │                           │
   │                    StartSession(pipeline="prod-all")  │
   │                    tool = ToolDeploy                   │
   │                    SelectAgent("", "deploy")          │
   │                    ← project 为空，匹配任意 deploy agent │
   │                           │                           │
   │                           │── task_assign ───────────►│
   │                           │   {pipeline:"prod-all"}   │
   │                           │                           │
   │                           │                    executePipeline()
   │                           │                    LoadPipelines()
   │                           │                    ┌──────┤
   │                           │                    │ 加载 prod-all.json
   │                           │                    │ 校验所有 step 的 project
   │                           │                    └──────┤
   │◄── "🔄 Pipeline: prod-all (3步)" ◄─ stream_event ───│
   │                           │                           │
   │                           │                    Step 1/3: go_blog
   │◄── "🚀 [1/3] 部署 go_blog" ◄─ stream_event ────────│
   │◄── "📦 编译..." ─────────│◄─ stream_event ──────────│
   │◄── "✅ [1/3] 完成" ──────│◄─ stream_event ──────────│
   │                           │                           │
   │                           │                    Step 2/3: myapp
   │◄── "🚀 [2/3] 部署 myapp" │◄─ stream_event ──────────│
   │◄── "✅ [2/3] 完成" ──────│◄─ stream_event ──────────│
   │                           │                           │
   │                           │                    Step 3/3: frontend
   │◄── "📦 [3/3] 打包 frontend"◄─ stream_event ─────────│
   │◄── "✅ [3/3] 完成" ──────│◄─ stream_event ──────────│
   │                           │                           │
   │◄── "✅ Pipeline 全部完成" │◄─ stream_event ──────────│
   │◄── result:{done:true} ────│◄─ task_complete ─────────│
   │                           │   {status:"done"}         │
```

**Pipeline JSON 示例** (`settings/pipelines/prod-all.json`):
```json
{
    "name": "prod-all",
    "description": "生产环境全量部署",
    "steps": [
        {"project": "go_blog", "target": "ssh-prod", "build_platform": "linux"},
        {"project": "myapp", "target": "ssh-prod"},
        {"project": "frontend", "pack_only": true}
    ]
}
```

**失败处理**：任意步骤失败即停止，返回 `task_complete {status:"error"}`。

---

## 4. 微信命令流程

### 4.1 确定性命令（`cg` 前缀 → go_blog 直接处理）

```
WeChat User             WeChat Agent              Go Blog
    │                       │                        │
    │ "cg pipeline list"    │                        │
    │──────────────────────►│                        │
    │                       │                        │
    │                       │  识别 "cg " 前缀       │
    │                       │  路由到 go_blog        │
    │                       │                        │
    │                       │── notify ────────────►│
    │                       │   {channel:"wechat",   │
    │                       │    to:"user123",       │
    │                       │    content:"cg pipeline list"}
    │                       │                        │
    │                       │                 handleWechatCommand()
    │                       │                 handleCodegenCommand()
    │                       │                        │
    │                       │                 case "pipeline":
    │                       │                   param == "list"
    │                       │                   pool.ListPipelines()
    │                       │                        │
    │                       │◄── notify ────────────│
    │                       │   {channel:"wechat",   │
    │                       │    to:"user123",       │
    │                       │    content:"📋 可用 Pipeline (2个)\n
    │                       │      🔄 prod-all (agent: win)\n
    │                       │      🔄 staging (agent: mac)"}
    │                       │                        │
    │◄──────────────────────│                        │
    │  "📋 可用 Pipeline..." │                        │
    │                       │                        │
```

### 4.2 编码命令（带微信通知中继）

```
WeChat User         WeChat Agent        Go Blog              Codegen Agent
    │                   │                   │                       │
    │ "cg start myapp   │                   │                       │
    │  写个HTTP服务"     │                   │                       │
    │──────────────────►│                   │                       │
    │                   │── notify ────────►│                       │
    │                   │                   │                       │
    │                   │            handleCodegenCommand()          │
    │                   │            case "start":                   │
    │                   │              StartSessionForWeChat()       │
    │                   │                   │                       │
    │                   │            ┌──────┤                       │
    │                   │            │ 1. 检查是否有运行中会话        │
    │                   │            │ 2. StartSession()             │
    │                   │            │ 3. 创建 UserSessionState      │
    │                   │            │ 4. 启动 subscribeAndRelay()   │
    │                   │            └──────┤                       │
    │                   │                   │                       │
    │                   │                   │── task_assign ───────►│
    │                   │                   │                       │
    │                   │◄── notify ────────│                       │
    │◄──────────────────│ "🚀 编码会话已启动"│                       │
    │                   │                   │                       │
    │                   │            subscribeAndRelay():            │
    │                   │            每 10s 刷新缓冲区              │
    │                   │                   │                       │
    │                   │                   │◄── stream_event ─────│
    │                   │                   │    type:"tool"         │
    │                   │                   │                       │
    │                   │            flushBuffer():                  │
    │                   │            合并工具步骤                     │
    │                   │                   │                       │
    │                   │◄── notify ────────│                       │
    │◄──────────────────│ "⚙️ myapp · 第5步 │                       │
    │                   │  · 30s            │                       │
    │                   │  🔧 Write main.go │                       │
    │                   │  🔧 Read go.mod"  │                       │
    │                   │                   │                       │
    │                   │                   │◄── task_complete ────│
    │                   │            sendCompletionSummary():        │
    │                   │◄── notify ────────│                       │
    │◄──────────────────│ "✅ myapp 编码完成 │                       │
    │                   │  2m30s · 12步     │                       │
    │                   │  · $0.0523"       │                       │
    │                   │                   │                       │
```

### 4.3 自然语言查询（→ LLM-MCP Agent）

```
WeChat User       WeChat Agent       LLM-MCP Agent        Go Blog (MCP)
    │                  │                   │                    │
    │ "今天待办有哪些" │                   │                    │
    │─────────────────►│                   │                    │
    │                  │                   │                    │
    │                  │  非 "cg" 前缀     │                    │
    │                  │  路由到 llm-mcp   │                    │
    │                  │                   │                    │
    │                  │── notify ────────►│                    │
    │                  │  {channel:"wechat",│                    │
    │                  │   content:"今天待办"}                    │
    │                  │                   │                    │
    │                  │            handleChat()                 │
    │                  │            1. 构建 system prompt        │
    │                  │            2. 发现工具 (GET /api/gateway/tools)
    │                  │            3. 智能工具路由              │
    │                  │                   │                    │
    │                  │            LLM 推理:                    │
    │                  │            "需要调用 GetTodosByDate"     │
    │                  │                   │                    │
    │                  │                   │── tool_call ──────►│
    │                  │                   │   {tool_name:       │
    │                  │                   │    "GetTodosByDate",│
    │                  │                   │    arguments:       │
    │                  │                   │    {date:"today"}}  │
    │                  │                   │                    │
    │                  │                   │            MCPCallInnerTools()
    │                  │                   │                    │
    │                  │                   │◄── tool_result ────│
    │                  │                   │   {result:"[...]"}  │
    │                  │                   │                    │
    │                  │            LLM 生成回复:                │
    │                  │            "你今天有3个待办..."          │
    │                  │                   │                    │
    │                  │◄── notify ────────│                    │
    │◄─────────────────│ "你今天有3个待办:  │                    │
    │                  │  1. 买菜          │                    │
    │                  │  2. 开会          │                    │
    │                  │  3. 写文档"       │                    │
    │                  │                   │                    │
```

---

## 5. 前端数据加载流程

### 5.1 项目列表加载

```
Browser                              Go Blog
   │                                     │
   │ GET /api/codegen/projects           │
   │────────────────────────────────────►│
   │                                     │
   │                              pool.GetAgents()
   │                              pool.ListRemoteProjects()
   │                              pool.GetAllClaudeCodeModels()
   │                              pool.GetAllOpenCodeModels()
   │                              pool.GetAllTools()
   │                                     │
   │◄────────────────────────────────────│
   │ {                                   │
   │   "success": true,                  │
   │   "data": {                         │
   │     "agents": [                     │
   │       {"id":"deploy_win_123",       │
   │        "name":"win",                │
   │        "status":"online",           │
   │        "projects":["go_blog"],      │
   │        "active_sessions":0}         │
   │     ],                              │
   │     "remote_projects": [            │
   │       {"name":"go_blog",            │
   │        "agent_id":"deploy_win_123", │
   │        "agent":"win",               │
   │        "tools":["deploy"],          │
   │        "deploy_targets":["local",   │
   │          "ssh-prod"],               │
   │        "host_platform":"win",       │
   │        "pipelines":["prod-all"]}    │
   │     ],                              │
   │     "claudecode_models":["sonnet"], │
   │     "opencode_models":["gemini"],   │
   │     "tools":["claudecode","deploy", │
   │       "opencode"]                   │
   │   }                                 │
   │ }                                   │
   │                                     │
   │ JavaScript 处理:                     │
   │ 1. cachedAllRemote = remote_projects│
   │ 2. 按 agent_id 分组                 │
   │ 3. 每个项目检查 tools:              │
   │    - claudecode/opencode → 编码条目  │
   │    - deploy → 部署条目              │
   │    - deploy + pipelines → 编排条目   │
   │ 4. renderProjects() 渲染侧边栏      │
   │                                     │
```

### 5.2 RemoteProjectInfo 数据结构

```json
{
    "name":           "go_blog",
    "agent_id":       "deploy_win_123",
    "agent":          "win",
    "tools":          ["deploy"],
    "deploy_targets": ["local", "ssh-prod"],
    "host_platform":  "win",
    "pipelines":      ["prod-all", "staging"]
}
```

**前端侧边栏渲染逻辑**：

```javascript
cachedAllRemote.forEach(rp => {
    // 按 agent_id 分组
    const g = agentMap.get(rp.agent_id);

    // 检查工具类型，生成不同模式的条目
    if (hasCoding)  → g.entries.push({mode:'code',     icon:'💻'})
    if (hasDeploy)  → g.entries.push({mode:'deploy',   icon:'🚀'})
    if (hasDeploy && rp.pipelines) {
        rp.pipelines.forEach(pip => {
            g.entries.push({name:pip, mode:'pipeline', icon:'🔄'})
        })
    }
})
```

---

## 6. Agent 选择策略

### 6.1 SelectAgent 算法

```
SelectAgent(project, tool)
    │
    │── 遍历所有在线 Agent ──►
    │
    │   对每个 Agent:
    │   ├── Status == "offline" → 跳过
    │   ├── ActiveSessions >= MaxConcurrent → 跳过（满载）
    │   │
    │   ├── 项目匹配检查:
    │   │   ├── project 为空 → 跳过检查（pipeline 模式）
    │   │   ├── Agent.Projects 包含 project → 匹配
    │   │   ├── Agent.Projects 为空 → 宽松匹配
    │   │   └── 否则 → 跳过
    │   │
    │   ├── 工具匹配检查:
    │   │   ├── tool 为空或 "claudecode" → 所有 Agent 支持
    │   │   ├── Agent.Tools 包含 tool → 匹配
    │   │   └── 否则 → 跳过
    │   │
    │   └── 选择可用容量最大的 Agent
    │
    ▼
    返回 best Agent (或 nil)
```

### 6.2 Execute 路由逻辑

```
Execute(session)
    │
    ├── deployOnly 或 pipeline → tool = "deploy"
    │
    ├── 指定 agentID?
    │   └── 是 → 检查该 Agent:
    │       ├── 在线?
    │       ├── pipeline模式? → 跳过项目检查
    │       └── 有该项目? → 使用该 Agent
    │
    ├── fallback → SelectAgent(project, tool)
    │   └── pipeline 模式: project 传空字符串
    │
    ├── 无可用 Agent → 返回错误
    │
    └── dispatchTask(agent, session)
        └── 构建 TaskAssignPayload
            ├── deploy → 无 system_prompt
            ├── opencode → opencode system prompt
            └── claudecode → claude code system prompt
```

---

## 7. 消息类型速查表

### 7.1 UAP 协议层

| 消息类型 | 发送方 | 接收方 | 载荷 |
|---------|--------|--------|------|
| `register` | Agent | Gateway | `{agent_id, agent_type, name, auth_token, tools, capacity}` |
| `register_ack` | Gateway | Agent | `{success, error}` |
| `heartbeat` | Agent | Gateway | `{agent_id}` |
| `heartbeat_ack` | Gateway | Agent | `{}` |
| `tool_call` | Agent | Agent (via GW) | `{tool_name, arguments, request_id}` |
| `tool_result` | Agent | Agent (via GW) | `{request_id, success, result, error}` |
| `task_assign` | Agent | Agent (via GW) | `{task_id, payload}` |
| `task_accepted` | Agent | Agent (via GW) | `{task_id}` |
| `task_event` | Agent | Agent (via GW) | `{task_id, event}` |
| `task_complete` | Agent | Agent (via GW) | `{task_id, status, result, error}` |
| `notify` | Agent | Agent (via GW) | `{channel, to, content}` |
| `error` | Gateway | Agent | `{code, message}` |

### 7.2 CodeGen 协议层（封装在 UAP 消息体内）

| 消息类型 | 发送方 | 接收方 | 载荷关键字段 |
|---------|--------|--------|-------------|
| `register` | codegen/deploy agent | go_blog | `{agent_id, name, projects, tools, deploy_targets, pipelines}` |
| `register_ack` | go_blog | agent | `{success, error}` |
| `heartbeat` | agent | go_blog | `{agent_id, active_sessions, load, projects, tools}` |
| `task_assign` | go_blog | agent | `{session_id, project, prompt, model, tool, pipeline, deploy_target}` |
| `task_accepted` | agent | go_blog | `{session_id}` |
| `task_rejected` | agent | go_blog | `{session_id, reason}` |
| `stream_event` | agent | go_blog | `{session_id, event:{type, text, tool_name, done}}` |
| `task_complete` | agent | go_blog | `{session_id, status, error}` |
| `file_read` / `file_read_resp` | go_blog ↔ agent | 远程文件读取 | `{request_id, project, path}` → `{content}` |
| `tree_read` / `tree_read_resp` | go_blog ↔ agent | 远程目录树 | `{request_id, project}` → `{tree}` |
| `project_create` / `project_create_resp` | go_blog ↔ agent | 创建项目 | `{request_id, name}` → `{success}` |

---

## 8. 会话状态管理

### 8.1 Go Blog 端 Session 状态机

```
                    ┌─────────────────────────────────┐
                    │                                 │
                    │          StartSession()         │
                    │                                 │
                    └────────────┬────────────────────┘
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │    running              │
                    │                         │
                    │  stream_event 持续更新   │
                    │  broadcast → WebSocket  │
                    │  broadcast → 微信通知    │
                    └───┬────────┬───────┬────┘
                        │        │       │
              task_complete  Agent离线  StopSession()
              status=done  removeAgent  用户手动
                        │        │       │
                        ▼        ▼       ▼
                    ┌──────┐ ┌──────┐ ┌──────┐
                    │ done │ │error │ │stopped│
                    └──────┘ └──────┘ └──────┘
                        │        │       │
                        └────────┼───────┘
                                 │
                                 ▼
                    1h 后自动清理 / 上限 50 保活
```

### 8.2 前端项目状态缓存

```javascript
// 每个项目独立状态，key = "projectName:agentId"
projectStates = {
    "go_blog:deploy_win_123": {
        sessionId:  "cg_1709000000",
        outputHtml: "<div>...</div>",   // 缓存的输出 HTML
        status:     "running",          // idle | running | done | error
        costText:   "$0.0523",
        model:      "sonnet",
        tool:       "claudecode"
    }
}
```

切换项目时：
1. `saveCurrentProjectState()` — 保存当前输出到缓存
2. `getProjectState(name, agentId)` — 加载目标项目状态
3. 恢复 outputHtml、状态栏、按钮状态
4. 若 `running`，重新连接 WebSocket

---

## 9. 跨 Agent 工具调用流程

```
LLM-MCP Agent              UAP Gateway               Go Blog (MCP)
    │                           │                         │
    │  LLM 决定调用工具         │                         │
    │                           │                         │
    │── tool_call ─────────────►│                         │
    │   {to:"go_blog",          │── 路由 ───────────────►│
    │    tool_name:"GetTodosByDate",                      │
    │    arguments:{date:"today"},                        │
    │    request_id:"tc_123"}   │                         │
    │                           │                  handleToolCall()
    │                           │                  查找 MCP 回调
    │                           │                  MCPCallInnerTools()
    │                           │                         │
    │                           │◄── tool_result ────────│
    │◄── tool_result ──────────│   {request_id:"tc_123",  │
    │   {success:true,          │    success:true,         │
    │    result:"[{title:...}]"}│    result:"[...]"}       │
    │                           │                         │
    │  将结果追加到 LLM 上下文   │                         │
    │  继续推理迭代...          │                         │
```

**工具发现**：LLM-MCP Agent 启动时通过 HTTP GET `/api/gateway/tools` 获取所有注册工具的 JSON Schema 定义，转换为 LLM function calling 格式。
