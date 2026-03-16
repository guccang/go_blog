# 微信对话上下文连续性技术文档

本文档描述微信多轮对话上下文管理方案的设计与实现，解决 LLM-MCP Agent 在微信场景下对话历史丢失的问题。

---

## 1. 问题背景

### 1.1 问题描述

微信用户与 LLM-MCP Agent 交互时，每条消息都被当作完全独立的新任务处理，不携带任何历史对话上下文。这导致：

1. **对话断裂**：第一个任务执行中需要用户输入，用户回复后 LLM 完全不知道之前的对话
2. **指代消解失败**：用户说"把第一个标记完成"时，LLM 不知道"第一个"指什么
3. **任务延续不可能**：多轮协作场景（先查数据、再分析、再操作）无法衔接

### 1.2 根因分析

```
微信消息 A: "帮我查今天待办"
    │
    ├─ handleWechatMessage()
    │   taskID = "wechat_abc123"    ← 全新 ID
    │   ctx.Messages = nil           ← 无历史
    │   processTask: system + "帮我查今天待办"
    │   → LLM 返回待办列表
    │
微信消息 B: "把第一个标记完成"
    │
    ├─ handleWechatMessage()
    │   taskID = "wechat_def456"    ← 又一个全新 ID
    │   ctx.Messages = nil           ← 无历史！
    │   processTask: system + "把第一个标记完成"
    │   → LLM: "第一个是什么？请提供更多信息"   ← 上下文丢失
```

**代码定位**（改动前 `chat.go:102-128`）：

```go
func (b *Bridge) handleWechatMessage(fromAgent, wechatUser, content string) {
    taskID := "wechat_" + newSessionID()  // 每次都生成新 ID
    ctx := &TaskContext{
        TaskID:  taskID,
        Account: b.cfg.DefaultAccount,
        Query:   content,               // 只有当前消息，无历史
        Source:  "wechat",
        Sink:    sink,
    }
    result, _ := b.processTask(ctx)      // processTask 构建: system + 单条 user
}
```

### 1.3 对比：Web 前端如何解决此问题

Web 前端的 `assistant_chat` 路径通过客户端维护完整对话历史，每次请求将全部 `Messages` 传入：

```
Web 前端 → AssistantTaskPayload.Messages = [system, user1, assistant1, user2, ...]
         → processTask 使用预构建消息
```

`processTask`（`processor.go:132-135`）已原生支持 `ctx.Messages` 预构建消息路径：

```go
if ctx.Messages != nil {
    messages = make([]Message, len(ctx.Messages))
    copy(messages, ctx.Messages)
}
```

**核心思路**：在微信侧维护对话历史，复用 `processTask` 已有的预构建消息路径。

---

## 2. 方案设计

### 2.1 方案选型

| 方案 | 描述 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| **内存会话 + TTL** | Per-User 内存 map，超时自动过期 | 简单、零 IO、低延迟 | 重启丢失 | **采用** |
| SessionStore 持久化 | 复用现有会话存储写磁盘 | 重启不丢 | 复杂、IO 开销、微信短对话无需 | 不采用 |
| LLM 智能判断 | 每条消息先用 LLM 判断是否关联上文 | 智能 | 额外 API 调用、增加延迟 | 不采用 |
| 客户端维护 | 让微信侧维护历史 | 解耦 | 微信无客户端状态能力 | 不可行 |

**选择理由**：微信对话天然是短时多轮的，30 分钟超时足够覆盖绝大多数场景。重启丢失可接受（重启后用户发新消息即开始新对话，体验自然）。

### 2.2 整体架构

```
微信用户
    │
    │ 发送消息
    ▼
┌───────────────────────────────────────────────────────────────┐
│                    handleWechatMessage()                       │
│                                                               │
│  ① 重置命令检测 ──"新对话"──→ Reset() → 回复确认              │
│         │ 否                                                  │
│  ② GetOrCreate(wechatUser) ──→ WechatConversationManager     │
│         │                      ┌──────────────────────────┐   │
│         │                      │  conversations map       │   │
│         │                      │  user_A → Conversation   │   │
│         │                      │  user_B → Conversation   │   │
│         │                      │  ...                     │   │
│         │                      └──────────────────────────┘   │
│         ▼                                                     │
│  ③ conv.processing.Lock()  ← 序列化同一用户消息               │
│         │                                                     │
│  ④ 新会话？──→ buildSystemPrompt + user 消息                  │
│    续接？──→ append user 消息到 conv.Messages                  │
│         │                                                     │
│  ⑤ compactWechatMessages()  ← 超 maxMessages 时压缩旧消息     │
│         │                                                     │
│  ⑥ 复制 Messages 快照 → TaskContext.Messages                  │
│         │                                                     │
│  ⑦ processTask(ctx)  ← 使用预构建消息路径，无需改动            │
│         │                                                     │
│  ⑧ append assistant 回复到 conv.Messages                      │
│         │                                                     │
│  ⑨ 发送结果回微信                                             │
│         │                                                     │
│  ⑩ conv.processing.Unlock()                                  │
└───────────────────────────────────────────────────────────────┘
```

### 2.3 对话生命周期

```
创建                  活跃                            过期
  │                    │                               │
  ├─ 首条消息触发       ├─ 每条消息刷新 LastActiveAt     ├─ 30 分钟无活动
  ├─ 构建 system prompt ├─ 追加 user/assistant 消息     ├─ 或达到 maxTurns
  ├─ 生成 SessionID    ├─ TurnCount++                  ├─ 或用户发送"新对话"
  │                    │                               │
  ▼                    ▼                               ▼
  WechatConversation   复用同一 SessionID              delete from map
  写入 map                                            下条消息触发新建
```

---

## 3. 数据模型

### 3.1 WechatConversation

```go
// 位置: cmd/llm-mcp-agent/chat.go:102-111

type WechatConversation struct {
    mu           sync.Mutex // 保护 Messages 等字段
    processing   sync.Mutex // 序列化同一用户的消息处理，避免并发交错
    WechatUser   string
    SessionID    string     // "wechat_" + 8位hex，首次创建，同一会话复用
    Messages     []Message  // 完整对话历史: [system, user1, assistant1, user2, ...]
    LastActiveAt time.Time  // 最后活跃时间
    TurnCount    int        // 对话轮次
}
```

**双锁设计**：

| 锁 | 保护范围 | 持有时间 |
|----|---------|---------|
| `mu` | Messages/LastActiveAt/TurnCount 字段读写 | 毫秒级（仅操作内存） |
| `processing` | 整个消息处理流程（含 LLM 调用） | 秒～分钟级 |

`processing` 锁确保同一用户的消息严格串行处理，避免并发交错导致对话历史错乱。

### 3.2 WechatConversationManager

```go
// 位置: cmd/llm-mcp-agent/chat.go:113-120

type WechatConversationManager struct {
    mu            sync.RWMutex
    conversations map[string]*WechatConversation // wechatUser → conversation
    timeout       time.Duration                  // 默认 30 分钟
    maxMessages   int                            // 默认 40
    maxTurns      int                            // 默认 15
}
```

### 3.3 对话历史消息结构

对话历史中只保留 3 种角色的消息，保持上下文精简：

```
Messages[0]: {Role: "system",    Content: systemPrompt}        ← 含日期、账户、任务拆解指引
Messages[1]: {Role: "user",      Content: "帮我查今天待办"}
Messages[2]: {Role: "assistant", Content: "今天有3个待办：..."}
Messages[3]: {Role: "user",      Content: "把第一个标记完成"}
Messages[4]: {Role: "assistant", Content: "已完成：..."}
...
```

**不包含 tool call 消息**：`processTask` 内部的 tool_call/tool_result 消息仅在其本地 `messages` 切片中，不会回写到对话历史。这是有意为之——tool call 细节已被 assistant 的最终文本回复所概括，避免上下文膨胀。

### 3.4 配置项

```go
// 位置: cmd/llm-mcp-agent/config.go

WechatSessionTimeoutMin int  `json:"wechat_session_timeout_min"` // 默认 30
WechatMaxMessages       int  `json:"wechat_max_messages"`        // 默认 40
WechatMaxTurns          int  `json:"wechat_max_turns"`           // 默认 15
```

| 配置项 | 默认值 | 说明 |
|-------|--------|------|
| `wechat_session_timeout_min` | 30 | 无活动超过此分钟数后自动开启新会话 |
| `wechat_max_messages` | 40 | 超过后触发消息压缩（旧消息 → 摘要） |
| `wechat_max_turns` | 15 | 超过后下条消息自动开启新会话 |

---

## 4. 核心流程

### 4.1 消息处理主流程

```
handleWechatMessage(fromAgent, wechatUser, content)
│
├── isConversationResetCommand(content)?
│   ├── YES → Reset(wechatUser) → 回复"已开始新对话。" → return
│   └── NO ↓
│
├── conv, isNew := GetOrCreate(wechatUser)
│   内部逻辑:
│   ├── 存在 + 未超时 + 未超轮次 → 返回现有 (isNew=false)
│   └── 不存在 / 超时 / 超轮次 → 创建新 (isNew=true)
│
├── conv.processing.Lock()          ← 阻塞等待前一条消息处理完成
│
├── 即时反馈: "⏳ 收到消息，正在处理..."
│
├── conv.mu.Lock()
│   ├── isNew? → buildAssistantSystemPrompt() + user 消息
│   └── !isNew → append user 消息
│   ├── compactWechatMessages()     ← 压缩检查
│   ├── copy(messagesCopy, conv.Messages)
│   ├── taskID = sessionID + "_" + turnCount
│   ├── turnCount++
│   conv.mu.Unlock()
│
├── processTask(&TaskContext{Messages: messagesCopy})
│   └── processTask 检测到 ctx.Messages != nil
│       → 直接使用预构建消息（不再构建 system prompt）
│       → 正常执行 agentic loop（工具调用等）
│       → 返回最终文本结果
│
├── conv.mu.Lock()
│   └── append assistant 回复
│   conv.mu.Unlock()
│
├── 发送结果回微信
│
└── conv.processing.Unlock()        ← 释放，允许处理该用户下一条消息
```

### 4.2 消息压缩算法

当 `len(Messages) > maxMessages` 时触发：

```
压缩前 (45 条):
  [system] [user1] [asst1] [user2] [asst2] ... [user20] [asst20] [user21] [asst21] [user22] [asst22]

keepCount = maxMessages * 2/3 = 26 (最少 6)

压缩后:
  [system]
  [压缩摘要: "用户: 帮我查...\nAI: 今天有...\n用户: 把第一个...\nAI: 已完成..."]  ← 旧消息的摘要
  [user8] [asst8] ... [user22] [asst22]                                           ← 保留最近 26 条
```

摘要格式：
```
[之前的对话摘要（已压缩 18 条消息）]
用户: 帮我查今天待办
AI: 今天有3个待办：1. 写周报 2. 买菜 3. 锻炼...
用户: 把第一个标记完成
AI: 已将"写周报"标记为完成...
```

### 4.3 会话重置命令

支持以下文本精确匹配（大小写不敏感）触发对话重置：

| 命令 | 效果 |
|------|------|
| `新对话` | 删除当前会话，回复确认 |
| `重新开始` | 同上 |
| `清除上下文` | 同上 |
| `reset` | 同上 |
| `new chat` | 同上 |

### 4.4 后台清理

```go
// 每 5 分钟扫描，删除超过 timeout 的会话
StartWechatCleanupLoop()
    └── ticker(5 min) → CleanupExpired()
        └── 遍历 map，删除 time.Since(LastActiveAt) >= timeout 的条目
```

---

## 5. 并发安全设计

### 5.1 锁层级

```
WechatConversationManager.mu (RWMutex)      ← 保护 conversations map
    └── WechatConversation.mu (Mutex)        ← 保护单个会话的字段
        └── WechatConversation.processing (Mutex)  ← 序列化消息处理
```

### 5.2 同一用户并发消息场景

```
消息 A ("查待办")                    消息 B ("创建提醒")
    │                                   │
    ├── GetOrCreate → conv              ├── GetOrCreate → 同一 conv
    ├── conv.processing.Lock() ✓        ├── conv.processing.Lock() ← 阻塞
    ├── append user A                   │   等待消息 A 处理完成...
    ├── processTask (3 秒)              │
    ├── append assistant A              │
    ├── conv.processing.Unlock()        ├── Lock() ✓ 获得锁
    │                                   ├── append user B（此时已有 A 的历史）
    │                                   ├── processTask (2 秒)
    │                                   ├── append assistant B
    │                                   ├── conv.processing.Unlock()
```

**效果**：消息 B 的 LLM 请求中会包含消息 A 的完整历史，保证上下文连贯。

### 5.3 不同用户并发

不同用户的 `WechatConversation` 是独立对象，`processing` 锁互不影响，完全并行处理。

---

## 6. 与现有系统的集成

### 6.1 processTask 无需改动

`processTask`（`processor.go:124-417`）已有两条路径：

| 条件 | 行为 |
|------|------|
| `ctx.Messages == nil` | 构建 system prompt + user query（原微信路径） |
| `ctx.Messages != nil` | 直接使用预构建消息（llm_request / assistant_chat 路径） |

本方案通过设置 `ctx.Messages = messagesCopy`，将微信流量导入已有的预构建消息路径，**零改动 processTask**。

### 6.2 SessionStore 持久化不受影响

`processTask` 内部仍会为每次调用创建 `TaskSession` 并持久化到 `SessionStore`。微信对话的每一轮都有独立的 `TaskSession` 记录（含完整 tool call 细节），可用于审计和调试。

### 6.3 Bridge 集成

```go
// bridge.go

type Bridge struct {
    // ... 原有字段 ...
    wechatConvMgr *WechatConversationManager  // 新增
}

func NewBridge(cfg *Config) *Bridge {
    // ... 原有初始化 ...
    b := &Bridge{
        // ... 原有字段 ...
        wechatConvMgr: NewWechatConversationManager(timeout, maxMessages, maxTurns),
    }
}
```

### 6.4 main.go 集成

```go
bridge.StartRefreshLoop()
bridge.StartWechatCleanupLoop()  // 新增：启动过期会话清理
```

---

## 7. 改动文件索引

| 文件 | 改动类型 | 改动内容 |
|------|---------|---------|
| `cmd/llm-mcp-agent/chat.go` | **重写** | 新增 `WechatConversation`、`WechatConversationManager` 结构体及方法；新增 `isConversationResetCommand`、`compactWechatMessages`、`StartWechatCleanupLoop`；重写 `handleWechatMessage` |
| `cmd/llm-mcp-agent/bridge.go` | 修改 | `Bridge` 新增 `wechatConvMgr` 字段；`NewBridge` 中初始化 |
| `cmd/llm-mcp-agent/config.go` | 修改 | `Config` 新增 3 个微信会话配置字段；`DefaultConfig` 新增默认值 |
| `cmd/llm-mcp-agent/main.go` | 修改 | 新增 `bridge.StartWechatCleanupLoop()` 调用 |
| `cmd/llm-mcp-agent/processor.go` | **无改动** | `processTask` 已支持 `ctx.Messages` 预构建消息路径 |

---

## 8. 设计决策与权衡

### 8.1 为什么不把 tool call 消息加入对话历史？

| 方案 | 上下文消耗 | 信息完整度 | 结论 |
|------|-----------|-----------|------|
| 只保留 user/assistant | 低（~200 字/轮） | 足够（assistant 已概括 tool 结果） | **采用** |
| 保留全部含 tool call | 高（~2000 字/轮） | 完整 | 不采用 |

微信场景下 token 预算有限（DeepSeek 128K），保留 tool call 会快速耗尽上下文。assistant 的最终回复已概括了工具执行结果，LLM 在后续对话中可以正确理解上下文。

### 8.2 为什么不用 LLM 判断消息是否关联？

每条消息增加一次额外 LLM 调用，在微信场景下增加 2-5 秒延迟，用户体验差。基于 TTL 的时间窗口判断足够准确：30 分钟内的消息几乎都是同一话题。

### 8.3 为什么 system prompt 不每轮刷新？

`buildAssistantSystemPrompt` 会调用 MCP 工具获取当日待办和运动数据（2 次工具调用，各 3 秒超时）。每轮刷新增加不必要的延迟。同一会话（30 分钟内）的 system prompt 数据变化极小，首次构建后复用是合理的。

### 8.4 为什么选择内存而非持久化？

- 微信对话是短时交互（通常 5-10 分钟完成），不需要跨重启恢复
- 重启后用户发新消息自然开始新对话，无感知断裂
- 内存方案零 IO 延迟，实现简单，无额外依赖

---

## 9. 验证方案

### 9.1 基本多轮对话

```
用户: 帮我查今天待办
AI:   今天有3个待办：1. 写周报 2. 买菜 3. 锻炼

用户: 把第一个标记完成                    ← LLM 应知道"第一个"指"写周报"
AI:   已将"写周报"标记为完成
```

### 9.2 超时自动重置

```
用户: 帮我查今天待办                      ← 新会话
AI:   今天有3个待办...

（等待 30+ 分钟）

用户: 你好                               ← 自动开始新会话（无之前上下文）
AI:   你好！有什么可以帮你的？
```

### 9.3 手动重置

```
用户: 帮我查今天待办
AI:   今天有3个待办...

用户: 新对话                              ← 显式重置
AI:   已开始新对话。

用户: 你好                               ← 新会话
AI:   你好！有什么可以帮你的？
```

### 9.4 并发消息串行处理

```
用户: 快速发送 "查待办"                    ← 消息 A，立即处理
用户: 快速发送 "查运动记录"                 ← 消息 B，等 A 完成后处理

AI 回复 A: 待办列表...
AI 回复 B: 运动记录...（此时 B 的 LLM 请求包含 A 的完整对话历史）
```

### 9.5 长对话消息压缩

连续发送 20+ 轮对话后，检查：
- LLM 仍能理解最近几轮的上下文
- 早期对话被压缩为摘要，不影响最新交互
- 总 token 消耗保持在合理范围
