# AGENTS.md - Go Blog 开发指南

本文档为 AI 编码代理（如 Claude、OpenCode 等）提供项目开发规范和命令参考。

## 项目概述

Go Blog 是一个基于 Go 1.24.0 的个人数字生活管理平台，采用"一切皆博客"设计理念。所有数据以 Markdown 文件存储在 `blogs_txt/` 目录，使用 Redis 作为缓存层。

### 架构特点

- **Monorepo 结构**: 40+ 独立模块位于 `pkgs/` 目录
- **本地模块替换**: 主 `go.mod` 通过 `replace` 指令引用本地模块
- **零数据库依赖**: 纯文件存储 + Redis 缓存
- **多账户支持**: 大部分函数以 `WithAccount` 后缀支持多租户

---

## 构建/测试/运行命令

### 构建命令

```bash
# 开发环境构建
go mod tidy && go build

# 生产环境优化编译（减小体积）
go build -ldflags="-s -w" -o go_blog

# 静态链接编译（Linux）
CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o go_blog

# 交叉编译
GOOS=darwin GOARCH=amd64 go build -o go_blog_mac
GOOS=windows GOARCH=amd64 go build -o go_blog.exe
```

### 测试命令

```bash
# 测试所有模块
go test ./...

# 测试单个模块
cd pkgs/encryption && go test

# 测试单个文件
go test -v ./pkgs/encryption -run TestAesSimpleEncrypt

# 生成覆盖率报告
go test -cover ./...

# 带竞态检测的测试
go test -race ./...
```

### 运行命令

```bash
# 启动 Redis
./scripts/start_redis.sh

# 运行应用（HTTP）
./go_blog blogs_txt/sys_conf.md

# 运行应用（HTTPS）
./go_blog blogs_txt/sys_conf.md cert.pem key.pem

# 后台启动/停止/重启
./scripts/start.sh
./scripts/stop.sh
./scripts/restart.sh
```

### 依赖管理

```bash
# 整理依赖
go mod tidy

# 清理模块缓存
go clean -modcache

# 重新下载依赖
go mod download
```

---

## 代码风格指南

### 导入规范

```go
import (
    // 标准库
    "encoding/json"
    "fmt"
    "sync"
    "time"

    // 第三方库
    h "net/http"  // 使用别名避免命名冲突

    // 内部模块（使用 replace 的本地模块）
    "auth"
    "blog"
    "config"
    log "mylog"   // 日志模块统一使用 log 别名
    db "persistence"  // 持久化层统一使用 db 别名
    "module"
)
```

**规则**:
1. 导入分组顺序: 标准库 → 第三方库 → 内部模块
2. `mylog` 统一使用 `log` 别名
3. `persistence` 统一使用 `db` 别名
4. `net/http` 在处理函数中使用 `h` 别名

### 命名规范

#### 函数命名

```go
// 多账户函数: 以 WithAccount 后缀
func GetBlogWithAccount(account, title string) *module.Blog
func AddBlogWithAccount(account string, udb *module.UploadedBlogData) int

// 内部/私有函数: 小写开头
func getBlogStore(account string) *BlogStore
func deduplicateTags(tags string) string

// 对外接口: 大写开头
func Init()
func Info()
```

#### 结构体命名

```go
// 存储结构: 以 Store 后缀
type BlogStore struct {
    blogs map[string]*module.Blog
    mu    sync.RWMutex
}

// 管理器结构: 以 Manager 后缀
type BlogManager struct {
    stores map[string]*BlogStore
    mu     sync.Mutex
}

// 数据传输对象: 以 Data 后缀
type YearPlanData struct {
    YearOverview string `json:"yearOverview"`
    MonthPlans   []string `json:"monthPlans"`
}
```

#### 常量命名

```go
// 枚举常量: 使用 E 前缀 + 类型名
const (
    EAuthType_private     = 1
    EAuthType_public      = 2
    EAuthType_encrypt     = 4
    EAuthType_cooperation = 8
    EAuthType_diary       = 16
)
```

### 类型定义

```go
// 结构体字段使用 CamelCase，JSON 标签使用 snake_case
type Book struct {
    ID          string   `json:"id"`
    Title       string   `json:"title"`
    Author      string   `json:"author"`
    TotalPages  int      `json:"total_pages"`
    CurrentPage int      `json:"current_page"`
    Category    []string `json:"category"`
    Status      string   `json:"status"`
}

// 使用指针表示可选字段
type Comment struct {
    Owner      string
    Msg        string
    UserID     *string `json:"user_id"`      // 可选
    IsVerified bool    `json:"is_verified"`
}
```

### 并发安全

```go
// 使用 sync.RWMutex 保护共享资源
type BlogStore struct {
    blogs map[string]*module.Blog
    mu    sync.RWMutex
}

// 读操作使用 RLock
func (store *BlogStore) GetBlog(title string) *module.Blog {
    store.mu.RLock()
    defer store.mu.RUnlock()
    return store.blogs[title]
}

// 写操作使用 Lock
func (store *BlogStore) AddBlog(blog *module.Blog) {
    store.mu.Lock()
    defer store.mu.Unlock()
    store.blogs[blog.Title] = blog
}

// sync.Once 用于单例初始化
var (
    globalHub     *NotificationHub
    initOnce      sync.Once
)

func Init(account string) {
    initOnce.Do(func() {
        globalHub = NewNotificationHub()
        globalHub.Start()
    })
}
```

### 错误处理

```go
// 返回 int 错误码（0=成功，非零=错误）
func AddBlogWithAccount(account string, udb *module.UploadedBlogData) int {
    if _, ok := store.blogs[udb.Title]; ok {
        return 1  // 已存在
    }
    // ...
    return 0  // 成功
}

// 返回 error 用于复杂操作
func GetYearPlanWithAccount(account string, year int) (*YearPlanData, error) {
    planTitle := fmt.Sprintf("年计划_%d", year)
    blog := GetBlogWithAccount(account, planTitle)
    if blog == nil {
        return nil, fmt.Errorf("未找到年份 %d 的计划", year)
    }
    // ...
    return &planData, nil
}

// JSON 错误响应格式
return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
return fmt.Sprintf(`{"success":true,"data":%s}`, jsonData)
```

### 日志规范

```go
// 使用自定义日志模块
log.Debug(log.ModuleBlog, "message")
log.DebugF(log.ModuleBlog, "format %s", arg)
log.InfoF(log.ModuleBlog, "message %s", arg)
log.WarnF(log.ModuleAgent, "warning: %v", err)
log.ErrorF(log.ModuleConfig, "error: %s", err.Error())
log.Message(log.ModuleAgent, "message")
log.MessageF(log.ModuleAgent, "format %s", arg)

// 模块常量
log.ModuleCommon
log.ModuleBlog
log.ModuleConfig
log.ModuleHandler
log.ModuleLLM
log.ModuleAgent
log.ModuleControl
```

### HTTP 处理函数

```go
// 标准处理函数模式
func HandleEditor(w h.ResponseWriter, r *h.Request) {
    LogRemoteAddr("HandleEditor", r)
    
    // 1. 检查登录
    if checkLogin(r) != 0 {
        h.Redirect(w, r, "/index", 302)
        return
    }
    
    // 2. 获取账户
    account := getAccountFromRequest(r)
    
    // 3. 处理业务逻辑
    view.PageEditor(w, "", "")
}

// 获取 session 和 account
func getsession(r *h.Request) string {
    session, err := r.Cookie("session")
    if err != nil {
        return ""
    }
    return session.Value
}

func getAccountFromRequest(r *h.Request) string {
    s := getsession(r)
    if s == "" {
        return ""
    }
    return auth.GetAccountBySession(s)
}
```

### MCP 工具注册

```go
// 注册 MCP 回调
mcp.RegisterCallBack("ToolName", func(args map[string]interface{}) string {
    account, _ := args["account"].(string)
    param, _ := args["param"].(string)
    
    if account == "" || param == "" {
        return `{"error": "缺少必要参数"}`
    }
    
    result := doSomething(account, param)
    return fmt.Sprintf(`{"success":true,"result":"%s"}`, result)
})

// 注册工具描述
mcp.RegisterCallBackPrompt("ToolName", "工具描述")
```

### 模块初始化模式

```go
// 每个模块需要提供 Info() 和 Init() 函数
func Info() {
    log.InfoF(log.ModuleBlog, "info blog v4.0 (simple)")
}

func Init() {
    log.Debug(log.ModuleBlog, "blog module Init (simple)")
    blogManager = &BlogManager{
        stores: make(map[string]*BlogStore),
    }
}
```

---

## 添加新模块

1. 创建目录 `pkgs/newmodule/`
2. 初始化模块:
   ```bash
   cd pkgs/newmodule
   go mod init newmodule
   ```
3. 在主 `go.mod` 添加:
   ```go
   replace newmodule => ./pkgs/newmodule
   ```
4. 在 `require` 部分添加:
   ```go
   require (
       newmodule v0.0.0
   )
5. 实现 `Info()` 和 `Init()` 函数
6. 在 `main.go` 中导入并调用初始化

---

## 测试规范

```go
package encryption

import "testing"

// 测试函数以 Test 开头
func TestAesSimpleEncrypt(t *testing.T) {
    data := "Hello World!"
    key := "16bit secret key"
    expected := "PuMhKY8ZFLnDAwlQ7v/2SQ=="
    
    if got := AesSimpleEncrypt(data, key); got != expected {
        t.Errorf("AesSimpleEncrypt() = %s, want %v", got, expected)
    }
}

// 表驱动测试
func TestGenIVFromKey(t *testing.T) {
    tests := []struct {
        name    string
        key     string
        wantIv  string
    }{
        {"test", "16bit secret key", "ba79295cdabd3a86"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if gotIv := GenIVFromKey(tt.args.key); gotIv != tt.wantIv {
                t.Errorf("GenIVFromKey() = %v, want %v", gotIv, tt.wantIv)
            }
        })
    }
}
```

---

## 注意事项

1. **不要提交敏感配置**: API Key、密码等不要提交到 Git
2. **多账户支持**: 新增功能应考虑多账户场景，使用 `WithAccount` 后缀
3. **线程安全**: 共享资源必须使用 `sync.RWMutex` 保护
4. **错误码约定**: 0=成功，1=已存在，2=系统文件，3=数据库错误
5. **中文注释**: 代码注释使用中文
6. **无外部 Linter**: 项目未配置 golangci-lint，使用 `go vet` 检查
