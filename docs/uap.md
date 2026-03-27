# UAP (Universal Agent Protocol) 实现机制分析

## 1. 概述

UAP 是一个跨 Agent 通信协议，基于 WebSocket 实现，提供：
- Agent 注册与发现
- 消息路由与转发
- 跨 Agent 工具调用
- 心跳检测与健康检查
- 事件追踪

核心定位：**Gateway 与各 Agent 之间的通信协议**

---

## 2. 核心文件

| 文件 | 职责 |
|------|------|
| `cmd/common/uap/protocol.go` | 消息类型定义、数据结构 |
| `cmd/common/uap/server.go` | Gateway 端 WebSocket 服务 |
| `cmd/common/uap/client.go` | Agent 侧 SDK |
| `cmd/common/agentbase/remote_caller.go` | 远程工具调用封装 |

---

## 3. 消息模型

### 3.1 统一消息信封

```go
type Message struct {
    Type    string          `json:"type"`   // 消息类型
    ID      string          `json:"id"`     // 唯一消息 ID（请求-响应关联）
    From    string          `json:"from"`   // 源 agent ID
    To      string          `json:"to"`     // 目标 agent ID
    Payload json.RawMessage `json:"payload"`
    Ts      int64           `json:"ts"`
}
```

### 3.2 消息类型分类

#### 生命周期消息
| 类型 | 说明 |
|------|------|
| `register` | Agent 注册 |
| `register_ack` | 注册确认 |
| `heartbeat` | 心跳 |
| `heartbeat_ack` | 心跳回复 |

#### 工具调用
| 类型 | 说明 |
|------|------|
| `tool_call` | 跨 Agent 工具调用请求 |
| `tool_result` | 工具调用结果 |

#### 长任务
| 类型 | 说明 |
|------|------|
| `task_assign` | 任务分派 |
| `task_accepted` | 任务接受 |
| `task_rejected` | 任务拒绝 |
| `task_event` | 任务进度事件 |
| `task_complete` | 任务完成 |
| `task_stop` | 停止任务 |

#### Claude Mode 权限交互
| 类型 | 说明 |
|------|------|
| `permission_request` | 权限请求 |
| `permission_response` | 权限回复 |
| `set_mode` | 模式切换 |

#### 控制协议（AgentBase 内置处理）
| 类型 | 说明 |
|------|------|
| `ctrl_shutdown` | 关闭请求 |
| `ctrl_status` | 状态查询 |
| `ctrl_status_report` | 状态报告 |

#### Describe 协议
| 类型 | 说明 |
|------|------|
| `describe` | 查询 Agent 能力 |
| `describe_result` | 能力描述回复 |

---

## 4. Client 端实现 (Agent SDK)

### 4.1 Client 结构

```go
type Client struct {
    GatewayURL   string
    AgentID      string
    AgentType    string
    Name         string
    Description  string
    HostPlatform string
    HostIP       string
    Workspace    string
    Tools        []ToolDef
    Capacity     int
    Meta         map[string]any
    AuthToken    string

    // 内部状态
    conn       *websocket.Conn
    connected  bool
    stopCh     chan struct{}

    // 回调
    OnMessage    func(msg *Message)
    OnRegistered func(success bool)
}
```

### 4.2 核心流程

```
Run() → connect() → register() → runLoop()
                              ↓
                     heartbeatLoop()  // 定时发送心跳
                              ↓
                     接收并分发消息
```

### 4.3 自动重连机制

指数退避策略（连接失败时）：
- 1s → 2s → 5s → 10s → 30s → 60s

```go
func (c *Client) backoffSleep() {
    delays := []time.Duration{
        1 * time.Second,
        2 * time.Second,
        5 * time.Second,
        10 * time.Second,
        30 * time.Second,
        60 * time.Second,
    }
    delay := delays[c.backoffIdx]
    // ...
}
```

### 4.4 消息发送

```go
// SendTo 向指定 Agent 发送消息
func (c *Client) SendTo(toAgentID, msgType string, payload any) error {
    return c.Send(&Message{
        Type:    msgType,
        ID:      NewMsgID(),
        From:    c.AgentID,
        To:      toAgentID,
        Payload: mustMarshal(payload),
        Ts:      time.Now().UnixMilli(),
    })
}
```

---

## 5. Server 端实现 (Gateway)

### 5.1 Server 结构

```go
type Server struct {
    agents   map[string]*AgentConn  // agent_id → AgentConn
    mu       sync.RWMutex
    upgrader websocket.Upgrader
    AuthToken string

    // 回调
    OnAgentOnline   func(agent *AgentConn)
    OnAgentOffline  func(agent *AgentConn)
    OnMessage       func(from *AgentConn, msg *Message)
    OnMessageReceived func(from *AgentConn, msg *Message)
    OnMessageForwarded func(from *AgentConn, to *AgentConn, msg *Message)
    OnRouteError    func(from *AgentConn, msg *Message)
    OnHeartbeatTimeout func(agent *AgentConn)
}
```

### 5.2 连接处理流程

```
HandleWebSocket() → handleConn()
                        ↓
              等待 register 消息
                        ↓
            handleRegister() 注册 agent
                        ↓
              更新 agent.Online = true
                        ↓
              启动心跳检测循环
                        ↓
              routeMessage() 路由消息
```

### 5.3 消息路由规则

```go
func (s *Server) routeMessage(from *AgentConn, msg *Message) {
    if msg.To == "" {
        // To 为空，交给 OnMessage 回调处理
        s.OnMessage(from, msg)
        return
    }

    target := s.GetAgentByIDOrName(msg.To)
    if target == nil {
        // 目标离线，返回错误
        from.Send(&Message{Type: MsgError, ...})
        return
    }

    // 转发给目标 agent
    target.Send(msg)
}
```

### 5.4 心跳超时检测

```go
func (s *Server) StartHealthCheck(timeout time.Duration) {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            // 检查所有 agent 的 LastHB
            for _, a := range s.agents {
                if time.Since(a.LastHB) > timeout {
                    s.removeAgent(a)
                }
            }
        }
    }()
}
```

---

## 6. 跨 Agent 工具调用

### 6.1 RemoteCaller 组件

封装 pending channel 模式，提供请求-响应关联能力：

```go
type RemoteCaller struct {
    ab      *AgentBase
    catalog *ToolCatalog
    pending map[string]chan *uap.ToolResultPayload
}
```

### 6.2 调用流程

```
CallTool(toolName, args)
    ↓
查找 toolName 对应的 agentID
    ↓
生成 msgID，存入 pending map
    ↓
发送 tool_call 消息
    ↓
等待 ch <- result 或超时
    ↓
返回 result, agentID, error
```

### 6.3 结果分发

```go
func (rc *RemoteCaller) DispatchToolResult(payload *uap.ToolResultPayload) bool {
    ch, ok := rc.pending[payload.RequestID]
    if ok {
        ch <- payload
        return true
    }
    return false
}
```

---

## 7. 数据结构

### 7.1 RegisterPayload

```go
type RegisterPayload struct {
    AgentID      string         `json:"agent_id"`
    AgentType    string         `json:"agent_type"`    // "wechat", "blog-agent", "llm_mcp"
    Name         string         `json:"name"`
    Description  string         `json:"description"`
    HostPlatform string         `json:"host_platform"` // macOS/Linux/Windows
    HostIP       string         `json:"host_ip"`
    Workspace    string         `json:"workspace"`
    Tools        []ToolDef      `json:"tools"`
    Capacity     int            `json:"capacity"`
    Meta         map[string]any `json:"meta"`
    AuthToken    string         `json:"auth_token"`
}
```

### 7.2 ToolDef

```go
type ToolDef struct {
    Name        string          `json:"name"`        // "blog.GetTodos"
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`   // JSON Schema
}
```

### 7.3 ToolResultPayload

```go
type ToolResultPayload struct {
    RequestID string `json:"request_id"`  // 对应 Message.ID
    Success   bool   `json:"success"`
    Result    string `json:"result,omitempty"`  // 成功时为业务数据
    Error     string `json:"error,omitempty"`  // 失败时为错误描述
}
```

---

## 8. 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                          Gateway                             │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    UAP Server                           │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │  │
│  │  │  Registry    │  │   Router     │  │ HealthCheck  │ │  │
│  │  │  (agents)    │  │   (route)    │  │ (心跳超时)    │ │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘ │  │
│  └─────────────────────────────────────────────────────────┘  │
│                          │                                    │
│              /ws/uap (WebSocket)                              │
└──────────────────────────┼───────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
   ┌────▼────┐       ┌────▼────┐        ┌────▼────┐
   │ wechat  │       │  blog   │        │ deploy  │
   │ -agent  │       │ -agent  │        │ -agent  │
   │         │       │         │        │         │
   │ Client  │       │ Client  │        │ Client  │
   └─────────┘       └─────────┘        └─────────┘
```

---

## 9. 消息流向示例

### 9.1 Agent 注册

```
wechat-agent                    Gateway
     │                             │
     │─────── connect ────────────▶│
     │─────── register ──────────▶│
     │◀────── register_ack ───────│
     │                             │
```

### 9.2 跨 Agent 工具调用

```
llm-agent                Gateway               deploy-agent
     │                      │                        │
     │─── tool_call ───────▶│                        │
     │                      │──── tool_call ───────▶│
     │                      │◀─── tool_result ─────│
     │◀── tool_result ─────│                        │
     │                      │                        │
```

### 9.3 心跳检测

```
agent                    Gateway
 │                         │
 │──── heartbeat ─────────▶│
 │◀─── heartbeat_ack ─────│
 │                         │
 │  (30s 超时检测)          │
 │◀─── removed ───────────│
```

---

## 10. 关键设计

### 10.1 消息 ID 关联

- 每条消息有唯一 `ID`
- 请求-响应对通过 `ID` 关联
- `tool_result.RequestID` = 对应 `tool_call.ID`

### 10.2 Pending Channel 模式

```
CallTool:
    msgID = NewMsgID()
    pending[msgID] = make(chan)
    Send(tool_call with msgID)
    <-pending[msgID]  // 等待响应

DispatchToolResult:
    pending[requestID] <- payload  // 唤醒等待者
```

### 10.3 应用层消息透传

当 `To` 非空时，gateway 不拦截 `heartbeat`、`register` 等消息，直接路由：

```go
case MsgHeartbeat:
    if msg.To != "" && agent != nil {
        s.routeMessage(agent, &msg)  // 透传给目标 agent
        continue
    }
    // 否则 gateway 自身处理
```

### 10.4 指数退避重连

防止频繁重连造成服务端压力：
- 初始重试间隔 1s
- 最大重试间隔 60s
- 达到最大间隔后保持，直到连接成功

---

## 11. HTTP API

Gateway 提供管理接口：

| 端点 | 说明 |
|------|------|
| `GET /api/gateway/agents` | 获取所有在线 Agent |
| `GET /api/gateway/tools` | 获取所有 Agent 的工具列表 |
| `GET /api/gateway/health` | 健康检查 |
| `GET /api/gateway/events` | 查询事件日志 |
| `GET /api/gateway/events/trace/{traceID}` | 获取完整调用链 |
| `WS /ws/uap` | Agent WebSocket 接入点 |
