# AGENTS.md - Go Blog 开发指南

本文档为 AI 编码代理提供项目开发规范和命令参考。

## 项目概述

Go Blog 是一个基于 Go 1.24.0 的个人数字生活管理平台，采用"一切皆博客"设计理念。所有数据以 Markdown 文件存储在 `blogs_txt/` 目录，使用 Redis 作为缓存层。

**架构特点**:
- **Monorepo 结构**: 40+ 独立模块位于 `pkgs/` 目录
- **本地模块替换**: 主 `go.mod` 通过 `replace` 指令引用本地模块
- **零数据库依赖**: 纯文件存储 + Redis 缓存
- **多账户支持**: 大部分函数以 `WithAccount` 后缀支持多租户
- **多Agent系统**: 分布式微服务架构，位于 `cmd/` 目录，通过 Gateway 协同工作

### 🤖 多Agent系统概览

项目采用**微服务+多Agent**架构，通过 `cmd/` 目录下的独立Agent实现功能解耦：

| Agent | 目录 | 职责 |
|-------|------|------|
| **Gateway** | `cmd/gateway/` | Agent通信中枢，消息路由，工具目录管理 |
| **LLM-MCP-Agent** | `cmd/llm-mcp-agent/` | AI大脑，多模型LLM集成，MCP协议支持 |
| **CodeGen-Agent** | `cmd/codegen-agent/` | 代码生成与工程自动化 |
| **Deploy-Agent** | `cmd/deploy-agent/` | 多平台部署自动化，流水线执行 |
| **WeChat-Agent** | `cmd/wechat-agent/` | 企业微信集成，消息转发 |
| **Blog-Agent** | `cmd/blog-agent/` | 主博客应用，包含40+核心模块 |

**通信协议**: 基于WebSocket的UAP (Universal Agent Protocol) 协议，所有Agent通过Gateway进行注册和消息路由。

**详细文档**: 参见 [README.md#多agent系统架构](README.md#多agent系统架构) 和 [docs/architecture.md](docs/architecture.md)、[docs/message_flow.md](docs/message_flow.md)。

---

## 构建/测试/运行命令

```bash
# 主博客应用 (位于 cmd/blog-agent/)
cd cmd/blog-agent
go mod tidy && go build
go build -ldflags="-s -w" -o go_blog

go test ./...                                    # 测试所有模块
cd pkgs/encryption && go test                    # 测试单个模块
go test -v ./pkgs/encryption -run TestAesSimpleEncrypt  # 测试单个函数

./scripts/start_redis.sh                         # 启动 Redis
./go_blog ../blogs_txt/sys_conf.md                  # 运行应用（HTTP）
./go_blog ../blogs_txt/sys_conf.md cert.pem key.pem # 运行应用（HTTPS）

# 构建其他Agent (如需要)
cd ../llm-mcp-agent && go build -o llm-mcp-agent
cd ../deploy-agent && go build -o deploy-agent
cd ../wechat-agent && go build -o wechat-agent
cd ../gateway && go build -o gateway
```

---

## 代码风格指南

### 导入规范

```go
import (
    "encoding/json"
    "fmt"
    "sync"
    
    h "net/http"  // 第三方库别名
    
    "auth"        // 内部模块
    "blog"
    log "mylog"   // 日志模块统一使用 log 别名
    db "persistence"  // 持久化层统一使用 db 别名
)
```

**规则**: 导入分组顺序为标准库 → 第三方库 → 内部模块。

### 命名规范

- **多账户函数**: 以 `WithAccount` 后缀，如 `GetBlogWithAccount(account, title string)`
- **私有函数**: 小写开头，如 `getBlogStore(account string)`
- **对外接口**: 大写开头，如 `Init()`, `Info()`
- **存储结构**: 以 `Store` 后缀，如 `BlogStore`
- **管理器结构**: 以 `Manager` 后缀，如 `BlogManager`
- **枚举常量**: 使用 `E` 前缀，如 `EAuthType_private = 1`

### 类型定义与并发安全

```go
type BlogStore struct {
    blogs map[string]*module.Blog
    mu    sync.RWMutex
}

func (store *BlogStore) GetBlog(title string) *module.Blog {
    store.mu.RLock()
    defer store.mu.RUnlock()
    return store.blogs[title]
}

func (store *BlogStore) AddBlog(blog *module.Blog) {
    store.mu.Lock()
    defer store.mu.Unlock()
    store.blogs[blog.Title] = blog
}
```

**使用指针表示可选字段**: `UserID *string`

### 错误处理

```go
// 返回 int 错误码（0=成功，非零=错误）
func AddBlogWithAccount(account string, udb *module.UploadedBlogData) int {
    if _, ok := store.blogs[udb.Title]; ok {
        return 1  // 已存在
    }
    return 0  // 成功
}

// 返回 error 用于复杂操作
func GetYearPlanWithAccount(account string, year int) (*YearPlanData, error) {
    blog := GetBlogWithAccount(account, planTitle)
    if blog == nil {
        return nil, fmt.Errorf("未找到年份 %d 的计划", year)
    }
    return &planData, nil
}
```

**错误码约定**: 0=成功，1=已存在，2=系统文件，3=数据库错误

### 日志规范

```go
log.Debug(log.ModuleBlog, "message")
log.InfoF(log.ModuleBlog, "format %s", arg)
log.WarnF(log.ModuleAgent, "warning: %v", err)
log.ErrorF(log.ModuleConfig, "error: %s", err.Error())
```

### HTTP 处理函数

```go
func HandleEditor(w h.ResponseWriter, r *h.Request) {
    LogRemoteAddr("HandleEditor", r)
    if checkLogin(r) != 0 {
        h.Redirect(w, r, "/index", 302)
        return
    }
    account := getAccountFromRequest(r)
    view.PageEditor(w, "", "")
}
```

### 模块初始化

每个模块需要提供 `Info()` 和 `Init()` 函数:

```go
func Info() {
    log.InfoF(log.ModuleBlog, "info blog v4.0")
}

func Init() {
    log.Debug(log.ModuleBlog, "blog module Init")
    blogManager = &BlogManager{
        stores: make(map[string]*BlogStore),
    }
}
```

---

## 添加新模块

1. 创建目录 `cmd/blog-agent/pkgs/newmodule/`
2. 初始化模块: `cd cmd/blog-agent/pkgs/newmodule && go mod init newmodule`
3. 在主 `go.mod` (`cmd/blog-agent/go.mod`) 添加 `replace newmodule => ./pkgs/newmodule`
4. 在 `require` 部分添加 `newmodule v0.0.0`
5. 实现 `Info()` 和 `Init()` 函数
6. 在 `main.go` (`cmd/blog-agent/main.go`) 中导入并调用初始化

---

## 测试规范

```go
func TestAesSimpleEncrypt(t *testing.T) {
    data := "Hello World!"
    key := "16bit secret key"
    expected := "PuMhKY8ZFLnDAwlQ7v/2SQ=="
    
    if got := AesSimpleEncrypt(data, key); got != expected {
        t.Errorf("AesSimpleEncrypt() = %s, want %v", got, expected)
    }
}
```

---

## 重要注意事项

1. **不要提交敏感配置**: API Key、密码等不要提交到 Git
2. **多账户支持**: 新增功能应考虑多账户场景，使用 `WithAccount` 后缀
3. **线程安全**: 共享资源必须使用 `sync.RWMutex` 保护
4. **中文注释**: 代码注释使用中文
5. **无外部 Linter**: 项目未配置 golangci-lint，使用 `go vet` 检查
