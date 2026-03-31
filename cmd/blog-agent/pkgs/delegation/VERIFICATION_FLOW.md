# Delegation Token 验证流程文档

## 1. 概述

Delegation Token 机制用于在 app-agent、llm-agent 和 blog-agent 之间建立信任关系，确保用户只能访问自己账户的数据。

### 设计目标
- **身份验证**：确认请求来自可信的 agent
- **权限控制**：确保 token 只能访问其声明的目标账户
- **防重放**：通过 Nonce 机制防止请求重放攻击
- **渠道隔离**：区分 app-agent 渠道和 wechat 渠道，wechat 渠道不要求 token

## 2. 架构与数据流

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           消息流程                                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐          │
│  │ app-agent│    │ gateway  │    │llm-agent │    │blog-agent│          │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘          │
│       │               │               │               │                 │
│       │ 1.登录成功后    │               │               │                 │
│       │ 签发token      │               │               │                 │
│       │───────────────>│               │               │                 │
│       │               │ 2.发送消息     │               │                 │
│       │               │ 带[delegation: │               │                 │
│       │               │  TOKEN]前缀   │               │                 │
│       │               │───────────────>│               │                 │
│       │               │               │               │                 │
│       │               │               │ 3.调用tool    │                 │
│       │               │               │ account="john"│                 │
│       │               │               │───────────────>│                 │
│       │               │               │               │ 4.验证token     │
│       │               │               │               │ 5.执行tool      │
│       │               │               │               │<───────────────│
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3. Token 结构

```go
type DelegationToken struct {
    IssuerAgentID   string   `json:"iss"`  // 签发代理 ID: "app-agent"
    AuthorizedUser  string   `json:"sub"`  // 授权用户: "john"
    TargetAccount   string   `json:"aud"`  // 目标账户: "john"
    Scope           []string `json:"scope"`// 权限范围: ["blog:read", "blog:write"]
    IssuedAt        int64    `json:"iat"`  // 签发时间戳
    ExpiresAt       int64    `json:"exp"`  // 过期时间戳
    Nonce           string   `json:"jti"`  // 随机数 (防重放)
    Signature       string   `json:"sig"`  // HMAC-SHA256 签名
}
```

### 权限范围常量
| 权限 | 描述 |
|------|------|
| `blog:read` | 读取博客 |
| `blog:write` | 写博客 |
| `todo:read` | 读取待办 |
| `todo:write` | 写待办 |
| `yearplan:read` | 读取年计划 |
| `yearplan:write` | 写年计划 |

## 4. 存储结构

### codegen 本地存储 (gateway 层)
```go
// token 存储：key = token.GetTargetAccount()，value = token
delegationTokenStore = make(map[string]DelegationTokenHolder)

// app-agent 验证标记：key = account，value = 是否通过 app-agent 验证
appChannelVerified = make(map[string]bool)
```

### mcp 包存储 (tool 执行层)
```go
// requestID -> token 映射
delegationTokenContext = make(map[string]*delegation.DelegationToken)
```

## 5. 完整验证流程

### 5.1 Token 签发 (app-agent/auth.go)

```
用户登录成功
    │
    ▼
delegationSigner.IssueToken(
    authorizedUser = "john",      // 登录用户
    targetAccount = "john",       // 目标账户（同上）
    scopes = ["blog:read", "blog:write", ...],
    validityDuration = 30分钟
)
    │
    ▼
生成 token 并用 HMAC-SHA256 签名
    │
    ▼
返回 token 字符串
```

### 5.2 Token 缓存 (gateway/handleAppNotify)

```
收到 app-agent 的 Notify 消息（Channel="app"）
    │
    ▼
检查消息内容是否以 "[delegation:" 开头
    │
    ├─ 否 ──> 不提取 token，正常处理消息
    │
    └─ 是 ──> 提取 token
                │
                ▼
            解析 token
                │
                ▼
            delegationTokenStore[token.TargetAccount] = token
            appChannelVerified[token.TargetAccount] = true
                │
                ▼
            消息内容去掉 [delegation:xxx] 前缀后继续处理
```

### 5.3 Tool Call 验证 (gateway/handleToolCall)

```
收到 llm-agent 的 ToolCall 消息
    │
    ▼
从 args 中提取 account 参数
    │
    ├─ account 不存在 ──> 直接执行 tool（无 account 验证）
    │
    └─ account 存在 ──> 检查 appChannelVerified[account]
                            │
                            ├─ false (wechat 等) ──> 直接执行 tool
                            │
                            └─ true (已验证) ──> 必须有有效 token
                                                    │
                                                    ▼
                                                获取 delegationTokenStore[account]
                                                    │
                                                    ├─ token 不存在 ──> 拒绝访问
                                                    │
                                                    └─ token 存在 ──> 验证 token
                                                                        │
                                                                        ▼
                                                                    调用 VerifyDelegationToken
                                                                        │
                                                                        ├─ 验证失败 ──> 拒绝访问
                                                                        │
                                                                        └─ 验证成功 ──> 检查 account 匹配
                                                                                            │
                                                                                            ├─ account 不匹配 ──> 拒绝访问
                                                                                            │
                                                                                            └─ account 匹配 ──> 执行 tool
```

### 5.4 Token 验证细节 (mcp/manager.go)

```
Verify(token) 验证以下项：
    │
    ├─ 1. 检查签发者是否可信（trustedAgents 中存在）
    │
    ├─ 2. 验证签名（HMAC-SHA256）
    │
    ├─ 3. 检查过期时间（now < ExpiresAt）
    │
    ├─ 4. 检查生效时间（now >= IssuedAt）
    │
    └─ 5. 检查 Nonce 是否已使用（防重放）
                │
                ├─ 已使用 ──> 拒绝
                │
                └─ 未使用 ──> 标记为已使用，继续
                            │
                            ▼
                        返回 authorizedAccount = token.TargetAccount
```

## 6. 场景矩阵

| 请求来源 | account 参数 | appChannelVerified | delegationTokenStore | 验证结果 |
|---------|-------------|-------------------|---------------------|---------|
| app-agent | "john" | true | 有 token | 验证 token，检查 account 匹配 |
| app-agent | "john" | true | 无 token | **拒绝访问** |
| app-agent | "john" | true | 有 token 但过期 | **拒绝访问** |
| app-agent | "john" | true | token account="ztt" | **拒绝访问** (account 不匹配) |
| app-agent | "ztt" | true | 有 token | 验证 token，检查 account 匹配 |
| wechat | "john" | false | 无 token | **放行** (wechat 渠道) |
| wechat | "ztt" | false | 无 token | **放行** (wechat 渠道) |

## 7. 关键代码位置

### 7.1 Token 签发
- `cmd/app-agent/auth.go:117-119` - 登录时签发 token
- `cmd/app-agent/delegation/signer.go:61-78` - IssueToken 实现

### 7.2 Token 缓存
- `cmd/blog-agent/pkgs/codegen/gateway.go:543-551` - handleAppNotify 提取并缓存 token

### 7.3 Tool Call 验证
- `cmd/blog-agent/pkgs/codegen/gateway.go:652-730` - handleToolCall 验证逻辑
- `cmd/blog-agent/pkgs/mcp/mcp.go:91-115` - ValidateAccountAccess 验证

### 7.4 Token 存储定义
- `cmd/blog-agent/pkgs/codegen/gateway.go:50-65` - delegationTokenStore 和 appChannelVerified
- `cmd/blog-agent/pkgs/codegen/gateway.go:39-45` - DelegationTokenHolder 接口

## 8. 安全特性

### 8.1 签名验证
- 使用 HMAC-SHA256 算法
- 签名内容包含所有 Claims（不含 Signature 本身）
- 密钥为预共享的 secretKey

### 8.2 防重放
- Nonce 必须全局唯一
- Nonce 使用后被缓存 5 分钟
- 5 分钟内的重复 Nonce 请求会被拒绝

### 8.3 账户隔离
- **无通配符权限**：Token 只能访问其声明的 TargetAccount
- **移除前的通配符代码**：
  ```go
  // 已移除：不允许 token 使用 * 权限访问任意账户
  if token.HasScope("*") { ... }
  ```

### 8.4 渠道隔离
- **app-agent 渠道**：需要有效的 delegation token
- **wechat 渠道**：不要求 token（因为 wechat 本身有身份验证）

## 9. 潜在问题与讨论

### 9.1 wechat 用户是否需要 token 验证？
当前设计：wechat 渠道不要求 token，直接放行。

**理由**：
- wechat 用户通过企业微信身份验证
- tool call 请求的 account 是 wechatUser（用户自己的账户）
- 如果要引入 token，需要额外的 wechat 渠道 token 签发机制

**替代方案**（更严格）：
- wechat 也要求 token
- 需要在 wechat 渠道实现 token 签发

### 9.2 appChannelVerified 标记的生命周期？
当前设计：标记后永久有效（不自动清除）。

**潜在问题**：
- 如果用户 A 发送消息后，标记永久保留
- 后续任何 tool call 都需要 token

**替代方案**：
- 标记设置过期时间
- 标记在特定条件后清除

### 9.3 account 参数的可信度？
当前设计：tool call 的 account 参数来自 llm-agent 的解析结果。

**潜在风险**：
- llm-agent 可能被诱导生成恶意 tool call
- 需要确保 llm-agent 生成的 tool call 参数是可信的

**缓解措施**：
- delegation token 的 TargetAccount 必须在验证时匹配
- appChannelVerified 标记确保只有通过 app-agent 的请求才要求 token
