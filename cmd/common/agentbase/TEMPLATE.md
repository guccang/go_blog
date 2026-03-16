# AgentBase 开发模板

快速开发新 agent 的模板和最佳实践。

## 基础 Agent 模板（无协议层）

适用于：不需要向 go_blog backend 注册的 agent（如工具提供者）

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "agentbase"
    "uap"
)

type MyAgent struct {
    *agentbase.AgentBase
    cfg *Config
}

func main() {
    cfg := loadConfig()

    agent := &MyAgent{
        AgentBase: agentbase.NewAgentBase(&agentbase.Config{
            ServerURL: cfg.ServerURL,
            AgentID:   cfg.AgentID,
            AgentType: "my_agent",
            AgentName: cfg.AgentName,
            AuthToken: cfg.AuthToken,
            Capacity:  cfg.MaxConcurrent,
            Tools:     buildTools(),
        }),
        cfg: cfg,
    }

    // 注册消息处理器
    agent.RegisterHandler(uap.MsgToolCall, agent.handleToolCall)

    // 优雅退出
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
        <-sigCh
        log.Println("Shutting down...")
        agent.Stop()
        os.Exit(0)
    }()

    // 启动连接（阻塞）
    agent.Run()
}

func (a *MyAgent) handleToolCall(msg *uap.Message) {
    // 处理工具调用
}

func buildTools() []uap.ToolDef {
    return []uap.ToolDef{
        {
            Name:        "MyTool",
            Description: "工具描述",
            Parameters:  mustMarshalJSON(/* JSON Schema */),
        },
    }
}
```

---

## 协议层 Agent 模板

适用于：需要向 go_blog backend 注册的 agent（如 codegen-agent, deploy-agent）

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "agentbase"
    "uap"
)

type MyAgent struct {
    *agentbase.AgentBase
    cfg *Config
}

func main() {
    cfg := loadConfig()

    agent := &MyAgent{
        AgentBase: agentbase.NewAgentBase(&agentbase.Config{
            ServerURL: cfg.ServerURL,
            AgentID:   cfg.AgentID,
            AgentType: "my_agent",
            AgentName: cfg.AgentName,
            AuthToken: cfg.AuthToken,
            Capacity:  cfg.MaxConcurrent,
            Tools:     buildTools(),
        }),
        cfg: cfg,
    }

    // 注册消息处理器
    agent.RegisterHandler("task_assign", agent.handleTaskAssign)
    agent.RegisterHandler("task_stop", agent.handleTaskStop)

    // 启用协议层
    agent.EnableProtocolLayer(&agentbase.ProtocolLayerConfig{
        TargetAgentID:  cfg.GoBackendAgentID,
        BuildRegister:  agent.buildRegisterPayload,
        BuildHeartbeat: agent.buildHeartbeatPayload,
    })

    // 启动协议层
    go agent.StartProtocolLayer()

    // 优雅退出
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
        <-sigCh
        log.Println("Shutting down...")
        agent.Stop()
        os.Exit(0)
    }()

    // 启动连接（阻塞）
    agent.Run()
}

// 构建注册消息载荷
func (a *MyAgent) buildRegisterPayload() interface{} {
    return map[string]interface{}{
        "agent_id": a.AgentID,
        "name":     a.cfg.AgentName,
        // 自定义字段...
    }
}

// 构建心跳消息载荷
func (a *MyAgent) buildHeartbeatPayload() interface{} {
    return map[string]interface{}{
        "agent_id": a.AgentID,
        "load":     0.5,
        // 自定义字段...
    }
}

func (a *MyAgent) handleTaskAssign(msg *uap.Message) {
    // 处理任务分配
}

func (a *MyAgent) handleTaskStop(msg *uap.Message) {
    // 处理停止任务
}
```

---

## 工具发现 Agent 模板

适用于：需要调用其他 agent 工具的 agent（如 execute-code-agent, llm-mcp-agent）

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "time"

    "agentbase"
    "uap"
)

type MyAgent struct {
    *agentbase.AgentBase
    cfg         *Config
    toolCatalog *agentbase.ToolCatalog

    // 请求-响应关联
    pending map[string]chan *uap.ToolResultPayload
    pendMu  sync.Mutex
}

func main() {
    cfg := loadConfig()

    agent := &MyAgent{
        AgentBase:   agentbase.NewAgentBase(baseCfg),
        cfg:         cfg,
        toolCatalog: agentbase.NewToolCatalog(cfg.GatewayHTTP),
        pending:     make(map[string]chan *uap.ToolResultPayload),
    }

    // 注册消息处理器
    agent.RegisterHandler(uap.MsgToolCall, agent.handleToolCall)
    agent.RegisterHandler(uap.MsgToolResult, agent.handleToolResult)

    // 首次工具发现
    if err := agent.toolCatalog.Discover(agent.AgentID); err != nil {
        log.Printf("Initial tool discovery failed: %v", err)
    }

    // 启动后台刷新
    agent.toolCatalog.StartRefreshLoop(60*time.Second, agent.AgentID)

    // 启动连接
    agent.Run()
}

// 调用远程工具
func (a *MyAgent) callRemoteTool(toolName string, args interface{}) (string, error) {
    agentID, ok := a.toolCatalog.GetAgentID(toolName)
    if !ok {
        return "", fmt.Errorf("tool not found: %s", toolName)
    }

    msgID := uap.NewMsgID()
    ch := make(chan *uap.ToolResultPayload, 1)

    a.pendMu.Lock()
    a.pending[msgID] = ch
    a.pendMu.Unlock()

    defer func() {
        a.pendMu.Lock()
        delete(a.pending, msgID)
        a.pendMu.Unlock()
    }()

    // 发送 tool_call
    err := a.Client.Send(&uap.Message{
        Type: uap.MsgToolCall,
        ID:   msgID,
        From: a.AgentID,
        To:   agentID,
        Payload: mustMarshalJSON(uap.ToolCallPayload{
            ToolName:  toolName,
            Arguments: mustMarshalJSON(args),
        }),
        Ts: time.Now().UnixMilli(),
    })
    if err != nil {
        return "", err
    }

    // 等待结果
    select {
    case result := <-ch:
        if !result.Success {
            return "", fmt.Errorf("tool error: %s", result.Error)
        }
        return result.Result, nil
    case <-time.After(120 * time.Second):
        return "", fmt.Errorf("tool timeout")
    }
}

func (a *MyAgent) handleToolResult(msg *uap.Message) {
    var payload uap.ToolResultPayload
    json.Unmarshal(msg.Payload, &payload)

    a.pendMu.Lock()
    ch, ok := a.pending[payload.RequestID]
    a.pendMu.Unlock()

    if ok {
        ch <- &payload
    }
}
```

---

## 配置文件模板

`my-agent.conf`:
```ini
# Gateway 连接
server_url=ws://127.0.0.1:10086/ws
gateway_http=http://127.0.0.1:10086
auth_token=your-token-here

# Agent 信息
agent_name=my-agent
max_concurrent=5

# 协议层（如果需要）
go_backend_agent_id=go_blog_backend
```

加载配置：
```go
func loadConfig() *Config {
    configMap, err := agentbase.LoadKeyValueConfig("my-agent.conf")
    if err != nil {
        log.Fatal(err)
    }

    return &Config{
        ServerURL:         agentbase.GetString(configMap, "server_url", "ws://127.0.0.1:10086/ws"),
        GatewayHTTP:       agentbase.GetString(configMap, "gateway_http", "http://127.0.0.1:10086"),
        AuthToken:         agentbase.GetString(configMap, "auth_token", ""),
        AgentName:         agentbase.GetString(configMap, "agent_name", "my-agent"),
        MaxConcurrent:     agentbase.GetInt(configMap, "max_concurrent", 5),
        GoBackendAgentID:  agentbase.GetString(configMap, "go_backend_agent_id", "go_blog_backend"),
    }
}
```

---

## go.mod 模板

```go
module my-agent

go 1.24.0

require (
    agentbase v0.0.0
    uap v0.0.0
)

require (
    github.com/google/uuid v1.6.0 // indirect
    github.com/gorilla/websocket v1.5.0 // indirect
)

replace (
    agentbase => ../common/agentbase
    uap => ../common/uap
)
```

---

## 最佳实践

### 1. 消息处理器命名

- `handleXxx` - 处理 UAP 消息
- `handleXxxMsg` - 包装器（用于 goroutine）

### 2. 错误处理

```go
// 工具调用失败时返回错误
c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
    RequestID: msg.ID,
    Success:   false,
    Error:     "error message",
})
```

### 3. 日志规范

```go
log.Printf("[AgentName] message")
log.Printf("[INFO] normal operation")
log.Printf("[WARN] warning message")
log.Printf("[ERROR] error: %v", err)
```

### 4. 优雅退出

始终实现信号处理：
```go
go func() {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    <-sigCh
    log.Println("Shutting down...")
    agent.Stop()
    os.Exit(0)
}()
```

### 5. 并发安全

- 使用 `sync.Mutex` 保护共享状态
- 使用 channel 进行 goroutine 通信
- 避免在锁内执行耗时操作

---

## 参考实现

- **基础 Agent**: `cmd/execute-code-agent/`
- **协议层 Agent**: `cmd/codegen-agent/`, `cmd/deploy-agent/`
- **工具发现**: `cmd/execute-code-agent/connection.go`

---

## 开发流程

1. 复制模板代码
2. 修改 agent 类型和名称
3. 实现工具定义 `buildTools()`
4. 实现消息处理器
5. 配置 go.mod
6. 测试连接和功能
7. 编写文档

预计开发时间：2-4 小时（相比传统方式减少 50%）
