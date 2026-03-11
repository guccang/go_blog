# AgentBase - Agent 连接管理基础包

AgentBase 是一个用于简化 UAP (Unified Agent Protocol) agent 开发的基础包，提供连接管理、消息分发、协议层和工具发现等通用功能。

## 功能特性

### 1. AgentBase - 核心连接管理 (`agent_base.go`)

提供 UAP 客户端包装和消息分发能力：

```go
// 创建 agent
baseCfg := &agentbase.Config{
    ServerURL: "ws://127.0.0.1:10086/ws",
    AgentID:   "my-agent-123",
    AgentType: "custom",
    AgentName: "My Custom Agent",
    AuthToken: "token",
    Capacity:  5,
    Tools:     []uap.ToolDef{...},
}

agent := agentbase.NewAgentBase(baseCfg)

// 注册消息处理器
agent.RegisterHandler(uap.MsgToolCall, func(msg *uap.Message) {
    // 处理工具调用
})

// 启动连接（阻塞，自动重连）
agent.Run()
```

**特性：**
- 自动连接和重连
- 消息类型分发
- 线程安全的处理器注册
- 直接访问底层 UAP Client

### 2. ProtocolLayer - 协议层管理 (`protocol_layer.go`)

用于 agent 之间的注册和心跳（如 codegen-agent 向 go_blog backend 注册）：

```go
// 启用协议层
agent.EnableProtocolLayer(&agentbase.ProtocolLayerConfig{
    TargetAgentID: "go_blog_backend",
    BuildRegister: func() interface{} {
        return RegisterPayload{
            AgentID: agent.AgentID,
            Name:    "codegen-agent",
            // ... 自定义字段
        }
    },
    BuildHeartbeat: func() interface{} {
        return HeartbeatPayload{
            AgentID: agent.AgentID,
            Load:    0.5,
        }
    },
})

// 启动协议层（单独 goroutine）
go agent.StartProtocolLayer()
```

**特性：**
- 自动注册和心跳（15s 间隔）
- 断线重连后自动重新注册
- 注册失败自动重试
- 可选的注册确认回调

### 3. ToolCatalog - 工具目录发现 (`tool_discovery.go`)

从 gateway HTTP API 获取所有在线 agent 的工具列表：

```go
catalog := agentbase.NewToolCatalog("http://127.0.0.1:10086")

// 首次发现
if err := catalog.Discover(myAgentID); err != nil {
    log.Printf("discover failed: %v", err)
}

// 启动后台刷新（60s 间隔）
catalog.StartRefreshLoop(60*time.Second, myAgentID)

// 查询工具所属 agent
if agentID, ok := catalog.GetAgentID("blog.GetTodos"); ok {
    // 调用工具
}
```

**特性：**
- HTTP 轮询 gateway 工具目录
- 自动排除自己的工具
- 线程安全的目录访问
- 后台定时刷新

### 4. Config - 配置加载工具 (`config.go`)

加载 `key=value` 格式的配置文件：

```go
config, err := agentbase.LoadKeyValueConfig("agent.conf")
if err != nil {
    log.Fatal(err)
}

// 获取配置值（带默认值）
serverURL := agentbase.GetString(config, "server_url", "ws://127.0.0.1:10086/ws")
maxConcurrent := agentbase.GetInt(config, "max_concurrent", 5)
enabled := agentbase.GetBool(config, "enabled", true)

// 获取必需配置（缺失时返回错误）
authToken, err := agentbase.MustGetString(config, "auth_token")
port, err := agentbase.MustGetInt(config, "port")
```

**特性：**
- 支持 `#` 注释和空行
- 类型安全的配置读取
- 默认值支持
- 必需字段验证

## 使用示例

### 简单 Agent（无协议层）

```go
package main

import (
    "agentbase"
    "uap"
)

type MyAgent struct {
    *agentbase.AgentBase
}

func main() {
    cfg := &agentbase.Config{
        ServerURL: "ws://127.0.0.1:10086/ws",
        AgentID:   "my-agent",
        AgentType: "custom",
        AgentName: "My Agent",
        Tools:     buildTools(),
    }

    agent := &MyAgent{
        AgentBase: agentbase.NewAgentBase(cfg),
    }

    // 注册消息处理器
    agent.RegisterHandler(uap.MsgToolCall, agent.handleToolCall)

    // 启动
    agent.Run()
}

func (a *MyAgent) handleToolCall(msg *uap.Message) {
    // 处理工具调用
}
```

### 带协议层的 Agent

```go
func main() {
    cfg := loadConfig()

    agent := &MyAgent{
        AgentBase: agentbase.NewAgentBase(&agentbase.Config{
            ServerURL: cfg.ServerURL,
            AgentID:   cfg.AgentID,
            AgentType: "codegen",
            AgentName: cfg.AgentName,
            Tools:     buildTools(),
        }),
    }

    // 注册消息处理器
    agent.RegisterHandler(uap.MsgTaskAssign, agent.handleTaskAssign)
    agent.RegisterHandler(uap.MsgTaskStop, agent.handleTaskStop)

    // 启用协议层
    agent.EnableProtocolLayer(&agentbase.ProtocolLayerConfig{
        TargetAgentID: "go_blog_backend",
        BuildRegister: func() interface{} {
            return map[string]interface{}{
                "agent_id": agent.AgentID,
                "name":     cfg.AgentName,
                "projects": scanProjects(),
            }
        },
    })

    // 启动协议层
    go agent.StartProtocolLayer()

    // 启动连接
    agent.Run()
}
```

### 带工具发现的 Agent

```go
type MyAgent struct {
    *agentbase.AgentBase
    toolCatalog *agentbase.ToolCatalog
}

func main() {
    cfg := loadConfig()

    agent := &MyAgent{
        AgentBase:   agentbase.NewAgentBase(baseCfg),
        toolCatalog: agentbase.NewToolCatalog(cfg.GatewayHTTP),
    }

    // 首次工具发现
    agent.toolCatalog.Discover(agent.AgentID)

    // 启动后台刷新
    agent.toolCatalog.StartRefreshLoop(60*time.Second, agent.AgentID)

    // 注册消息处理器
    agent.RegisterHandler(uap.MsgToolCall, agent.handleToolCall)

    // 启动
    agent.Run()
}

func (a *MyAgent) callRemoteTool(toolName string, args interface{}) (string, error) {
    agentID, ok := a.toolCatalog.GetAgentID(toolName)
    if !ok {
        return "", fmt.Errorf("tool not found: %s", toolName)
    }

    // 发送 tool_call 消息
    return a.SendMsg(agentID, uap.MsgToolCall, uap.ToolCallPayload{
        ToolName:  toolName,
        Arguments: marshalJSON(args),
    })
}
```

## 设计原则

1. **组合优于继承** - 使用组合模式，agent 保留完全控制权
2. **可选功能** - 协议层、工具发现都是可选的
3. **最小侵入** - 不强制特定的架构或模式
4. **线程安全** - 所有公共 API 都是线程安全的
5. **自动重连** - 内置断线重连和状态恢复

## 迁移指南

### 从原始 UAP Client 迁移

**迁移前：**
```go
type Connection struct {
    client *uap.Client
}

func NewConnection() *Connection {
    client := uap.NewClient(url, id, typ, name)
    client.OnMessage = c.handleMessage
    return &Connection{client: client}
}

func (c *Connection) handleMessage(msg *uap.Message) {
    switch msg.Type {
    case uap.MsgToolCall:
        c.handleToolCall(msg)
    case uap.MsgToolResult:
        c.handleToolResult(msg)
    }
}
```

**迁移后：**
```go
type Connection struct {
    *agentbase.AgentBase
}

func NewConnection() *Connection {
    c := &Connection{
        AgentBase: agentbase.NewAgentBase(cfg),
    }
    c.RegisterHandler(uap.MsgToolCall, c.handleToolCall)
    c.RegisterHandler(uap.MsgToolResult, c.handleToolResult)
    return c
}
```

### 从自定义工具发现迁移

**迁移前（~60 行）：**
```go
func (c *Connection) DiscoverTools() error {
    resp, err := http.Get(url)
    // ... 解析响应
    // ... 更新 catalog map
}

func (c *Connection) StartRefreshLoop() {
    go func() {
        ticker := time.NewTicker(60*time.Second)
        for range ticker.C {
            c.DiscoverTools()
        }
    }()
}
```

**迁移后（~3 行）：**
```go
catalog := agentbase.NewToolCatalog(gatewayHTTP)
catalog.Discover(myAgentID)
catalog.StartRefreshLoop(60*time.Second, myAgentID)
```

## 已迁移的 Agent

- ✅ **execute-code-agent** - 383 行 → 312 行（减少 18.5%）

## 待迁移的 Agent

- **codegen-agent** - 560 行（预计减少 46%）
- **deploy-agent** - 774 行（预计减少 42%）
- **llm-mcp-agent** - 560 行（可选迁移）
- **wechat-agent** - 512 行（可选迁移）

## 依赖

- `uap` - UAP 协议包（`cmd/common/uap`）
- Go 1.24.0+

## 测试

```bash
cd cmd/common/agentbase
go test -v
```

## 许可

与 go_blog 项目相同
