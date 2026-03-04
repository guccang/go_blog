# Go Blog 项目构建与部署指南

## 📋 项目概述

Go Blog 是一个基于 Go 1.24.0 的个人数字生活管理平台，采用"一切皆博客"设计理念。所有数据以 Markdown 文件存储在 `blogs_txt/` 目录，使用 Redis 作为缓存层。

**架构特点**:
- **Monorepo 结构**: 40+ 独立模块位于 `pkgs/` 目录
- **本地模块替换**: 主 `go.mod` 通过 `replace` 指令引用本地模块
- **零数据库依赖**: 纯文件存储 + Redis 缓存
- **多账户支持**: 大部分函数以 `WithAccount` 后缀支持多租户

## 🛠️ 技术栈

- **后端**: Go 1.24.0+
- **前端**: HTML/CSS/JavaScript (原生)
- **存储**: Markdown文件 + Redis缓存
- **架构**: 模块化设计，支持插件扩展

## 📋 环境要求

### 必需环境
- **Go**: 版本 1.24.0 或更高（建议 1.24.10）
- **Redis**: 用于缓存和会话管理
- **Git**: 用于版本控制

### 可选环境
- **systemd**: 用于服务管理（Linux）
- **nginx**: 用于反向代理（生产环境）

## 📁 项目结构

```
go_blog/
├── main.go                 # 主程序入口（顶层入口）
├── cmd/                    # 多代理系统
│   ├── blog-agent/        # 主博客应用
│   │   ├── go.mod        # 主模块定义
│   │   ├── main.go       # 主程序入口
│   │   ├── pkgs/         # 40+ 核心模块
│   │   │   ├── auth/    # 认证授权
│   │   │   ├── blog/    # 博客核心功能
│   │   │   ├── config/  # 配置管理
│   │   │   ├── http/    # HTTP服务器
│   │   │   ├── mylog/   # 日志系统
│   │   │   ├── persistence/ # 持久化存储
│   │   │   └── ...      # 其他模块
│   │   └── scripts/     # 构建和运行脚本
│   ├── wechat-agent/      # 微信代理
│   ├── llm-mcp-agent/     # LLM MCP代理
│   ├── codegen-agent/     # 代码生成代理
│   ├── deploy-agent/      # 部署代理
│   └── gateway/           # API网关
├── blogs_txt/            # Markdown数据存储
├── templates/            # HTML模板
├── statics/             # 静态资源（CSS/JS/Images）
├── redis/               # Redis配置
├── logs/               # 日志文件目录
└── docs/               # 文档
```

**重要**: 项目使用本地模块替换机制，所有内部模块通过 `replace` 指令指向本地路径。

## 🚀 快速开始

### 1. 克隆项目
```bash
git clone <repository_url>
cd go_blog
```

### 2. 检查Go环境
```bash
go version
# 确保版本 >= 1.24.0
```

### 3. 安装依赖
```bash
# 进入主应用目录
cd cmd/blog-agent

# 整理依赖
go mod tidy
```

### 4. 启动Redis
```bash
# 方法1：使用系统Redis（端口6379）
redis-server &

# 方法2：使用项目配置（如果需要特定端口）
redis-server redis/redis_6666.conf
```

### 5. 编译项目
```bash
# 进入主应用目录
cd cmd/blog-agent

# 使用构建脚本（推荐）
./scripts/build.sh

# 或手动编译
go build -o go_blog main.go
```

### 6. 运行项目
```bash
# 方法1：使用启动脚本（推荐）
./scripts/start.sh

# 方法2：直接运行（需要配置文件路径）
./go_blog ../blogs_txt/sys_conf.md

# HTTPS运行（需要证书文件）
./go_blog ../blogs_txt/sys_conf.md cert.pem key.pem
```

## 🔧 详细配置说明

### Go模块配置

项目使用独特的本地模块替换机制：

```go
// cmd/blog-agent/go.mod 示例
module blog-agent

go 1.24.0

toolchain go1.24.10

replace core => ./pkgs/core
replace module => ./pkgs/module
replace blog => ./pkgs/blog
// ... 40+ 其他模块

require (
    core v0.0.0
    module v0.0.0
    blog v0.0.0
    // ... 其他模块
)
```

### 模块初始化模式

每个模块遵循标准模式：
```go
func Info() {
    log.InfoF(log.ModuleBlog, "info blog v4.0")
}

func Init() {
    log.Debug(log.ModuleBlog, "blog module Init")
    // 初始化代码
}
```

## 📝 构建脚本说明

### build.sh - 主构建脚本
位置: `cmd/blog-agent/scripts/build.sh`

```bash
#!/bin/bash
# 主构建脚本 - 清理旧文件并重新编译

p=$(dirname $0)
p=$(realpath $p)

base_path=$(dirname "$p")

if [ -e $base_path/go_blog ];then
    rm $base_path/go_blog
fi

echo $base_path
cd $base_path
go mod tidy

go build
```

**使用方法**:
```bash
cd cmd/blog-agent
./scripts/build.sh
```

### 其他重要脚本

#### start.sh - 启动服务
```bash
cd cmd/blog-agent
./scripts/start.sh
```

#### stop.sh - 停止服务
```bash
cd cmd/blog-agent
./scripts/stop.sh
```

#### restart.sh - 重启服务
```bash
cd cmd/blog-agent
./scripts/restart.sh
```

#### start_redis.sh - 启动Redis
```bash
cd cmd/blog-agent
./scripts/start_redis.sh
```

## ⚙️ 配置文件说明

### Redis配置
位置: `redis/redis_6666.conf`

关键配置:
```conf
port 6666
bind 127.0.0.1
maxmemory 256mb
maxmemory-policy allkeys-lru
```

### 应用配置文件
主要配置文件: `blogs_txt/sys_conf.md`

配置示例:
```markdown
# 系统配置
admin_account: your_account
admin_password_hash: your_password_hash
logs_dir: ./logs
http_port: 8080
redis_host: 127.0.0.1
redis_port: 6666
```

## 🏗️ 编译选项和优化

### 开发环境编译
```bash
# 进入主应用目录
cd cmd/blog-agent

# 快速编译（开发时使用）
go build -o go_blog main.go

# 启用竞态检测
go build -race -o go_blog main.go
```

### 生产环境编译
```bash
# 优化编译（生产环境）- 减小文件大小
go build -ldflags="-s -w" -o go_blog main.go

# 静态链接编译（Linux）- 无外部依赖
CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o go_blog main.go
```

### 交叉编译
```bash
# 编译为Windows版本
GOOS=windows GOARCH=amd64 go build -o go_blog.exe main.go

# 编译为macOS版本
GOOS=darwin GOARCH=amd64 go build -o go_blog_mac main.go

# 编译为Linux ARM版本
GOOS=linux GOARCH=arm64 go build -o go_blog_arm64 main.go
```

## 🧪 测试命令

### 运行测试
```bash
# 进入主应用目录
cd cmd/blog-agent

# 测试所有模块
go test ./...

# 测试单个模块
cd pkgs/encryption && go test

# 测试单个测试函数
go test -v ./pkgs/encryption -run TestAesSimpleEncrypt

# 带竞态检测的测试
go test -race ./...

# 生成覆盖率报告
go test -cover ./...

# 查看测试覆盖率详情
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 🐛 常见问题和解决方案

### 1. 编译错误：找不到模块
```bash
# 解决方案：清理并重新下载依赖
cd cmd/blog-agent
go mod tidy
go clean -modcache
go mod download
```

### 2. Redis连接失败
```bash
# 检查Redis状态
redis-cli -p 6666 ping

# 如果返回错误，检查Redis是否运行
ps aux | grep redis

# 启动Redis
redis-server redis/redis_6666.conf &
```

### 3. 本地模块替换错误
```bash
# 检查go.mod中的replace指令
cd cmd/blog-agent
cat go.mod | grep replace

# 确保模块路径正确
ls -la pkgs/
```

### 4. 端口占用问题
```bash
# 查看端口占用
lsof -i :8080

# 终止占用进程（谨慎使用）
kill -9 <PID>
```

## 🔧 开发环境设置

### VS Code配置推荐
创建 `.vscode/settings.json`:
```json
{
    "go.buildOnSave": "package",
    "go.lintOnSave": "package",
    "go.testOnSave": false,
    "go.buildTags": "",
    "go.gocodeAutoBuild": false,
    "go.useLanguageServer": true,
    "go.goroot": "",
    "go.toolsEnvVars": {
        "GO111MODULE": "on"
    }
}
```

### 推荐的Go工具
```bash
# 安装语言服务器
go install golang.org/x/tools/gopls@latest

# 安装代码分析工具
go install github.com/ramya-rao-a/go-outline@latest

# 安装调试器
go install github.com/go-delve/delve/cmd/dlv@latest

# 安装代码检查工具（可选）
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## 📦 部署指南

### 开发部署流程
1. 克隆项目到本地
2. 安装Go 1.24.0+ 和 Redis
3. 进入 `cmd/blog-agent` 目录
4. 运行 `go mod tidy` 安装依赖
5. 运行 `./scripts/build.sh` 编译
6. 启动Redis服务
7. 运行 `./scripts/start.sh` 启动应用
8. 访问 `http://localhost:8080`

### 生产部署建议
1. 使用优化编译选项：`-ldflags="-s -w"`
2. 配置systemd服务管理
3. 设置nginx反向代理和SSL
4. 配置日志轮转
5. 设置定时备份数据目录

### 多账户部署配置
项目支持多租户架构：
- 每个账户数据独立存储在 `blogs_txt/<account>/` 目录
- Redis缓存按账户前缀隔离
- 管理员可在配置文件中设置默认账户

## 📊 性能优化建议

### 编译优化
- 使用 `-ldflags="-s -w"` 移除调试信息，减小文件大小
- 启用Go编译器的内联优化
- 使用 `upx` 进一步压缩可执行文件（可选）

### 运行时优化
- 合理设置Redis内存限制和淘汰策略
- 启用HTTP gzip压缩
- 配置静态资源缓存头
- 优化数据库连接池大小

### 文件存储优化
- 定期清理日志文件
- 使用异步文件写入减少IO阻塞
- 合理设置Markdown文件缓存策略

## 🧪 测试和调试

### 运行测试套件
```bash
# 进入主应用目录
cd cmd/blog-agent

# 运行完整测试套件
go test ./...

# 运行特定模块测试
go test ./pkgs/blog

# 运行性能测试
go test -bench=. ./pkgs/encryption
```

### 调试和性能分析
```bash
# 使用delve调试器
dlv debug cmd/blog-agent/main.go

# 启用CPU性能分析
cd cmd/blog-agent
go build -o go_blog main.go
./go_blog ../blogs_txt/sys_conf.md -cpuprofile cpu.prof

# 分析性能数据
go tool pprof cpu.prof
```

## 📝 维护和更新

### 依赖管理
```bash
# 进入主应用目录
cd cmd/blog-agent

# 检查可更新的依赖
go list -u -m all

# 更新所有依赖到最新版本
go get -u ./...
go mod tidy

# 更新特定依赖
go get -u github.com/go-redis/redis/v8
```

### 添加新模块流程
1. 创建目录结构：`cmd/blog-agent/pkgs/newmodule/`
2. 初始化模块：`cd cmd/blog-agent/pkgs/newmodule && go mod init newmodule`
3. 在主go.mod中添加替换：`replace newmodule => ./pkgs/newmodule`
4. 在主go.mod的require部分添加：`newmodule v0.0.0`
5. 实现标准接口：`Info()` 和 `Init()` 函数
6. 在main.go中导入并调用初始化

### 版本管理
```bash
# 创建新版本标签
git tag v1.0.0
git push origin v1.0.0

# 切换到特定版本构建
git checkout v1.0.0
cd cmd/blog-agent
./scripts/build.sh
```

## 📞 支持和故障排除

### 快速检查清单
遇到问题时，请依次检查：
1. ✅ Go版本 >= 1.24.0 (`go version`)
2. ✅ 依赖安装完成 (`go mod tidy` 无错误)
3. ✅ Redis服务运行正常 (`redis-cli ping`)
4. ✅ 配置文件路径正确
5. ✅ 端口未被占用 (`lsof -i :8080`)

### 日志文件位置
- 应用日志：`logs/app.log` (可在配置中修改)
- 访问日志：HTTP请求日志
- 错误日志：系统错误和异常

### 获取帮助
1. 查看项目文档：`docs/` 目录
2. 检查常见问题：本文档"常见问题"部分
3. 查看代码注释和示例

---

**版本**: v2.1  
**更新日期**: 2025年3月  
**维护者**: Go Blog开发团队  
**适用版本**: Go Blog v4.0+  
**Go版本要求**: 1.24.0+
