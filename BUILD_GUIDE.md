# Go Blog 项目编译配置使用说明

## 📋 项目概述

Go Blog 是一个功能丰富的 Go 语言博客系统，采用"一切皆博客"理念，将个人博客与生产力工具完美融合。

## 🛠️ 技术栈

- **后端**: Go 1.21+
- **前端**: HTML/CSS/JavaScript (原生)
- **存储**: Markdown文件 + Redis缓存
- **架构**: 20+独立模块化设计

## 📋 环境要求

### 必需环境
- **Go**: 版本 1.21 或更高
- **Redis**: 用于缓存和会话管理
- **Git**: 用于版本控制

### 可选环境
- **systemd**: 用于服务管理（Linux）
- **nginx**: 用于反向代理（生产环境）

## 📁 项目结构

```
go_blog/
├── main.go                 # 主程序入口
├── go.mod                  # Go模块依赖
├── go.sum                  # 依赖校验文件
├── pkgs/                   # 核心模块包
│   ├── blog/              # 博客核心功能
│   ├── comment/           # 评论系统
│   ├── exercise/          # 锻炼管理
│   ├── http/              # HTTP服务器
│   ├── lifecountdown/     # 人生倒计时
│   ├── llm/               # 大语言模型集成
│   ├── login/             # 登录认证
│   ├── mcp/               # MCP协议支持
│   ├── mylog/             # 日志系统
│   ├── reading/           # 阅读管理
│   ├── statistics/        # 统计分析
│   ├── todolist/          # 任务管理
│   └── yearplan/          # 年度计划
├── scripts/               # 构建和部署脚本
├── statics/              # 静态资源
│   ├── css/              # 样式文件
│   ├── js/               # JavaScript文件
│   └── images/           # 图片资源
├── templates/            # HTML模板
├── redis/                # Redis配置
└── datas/               # 数据存储目录
```

## 🚀 快速开始

### 1. 克隆项目
```bash
git clone <repository_url>
cd go_blog
```

### 2. 检查Go环境
```bash
go version
# 确保版本 >= 1.21
```

### 3. 安装依赖
```bash
go mod tidy
```

### 4. 启动Redis
```bash
# Ubuntu/Debian
sudo systemctl start redis-server

# 或使用项目脚本
./scripts/start_redis.sh
```

### 5. 编译项目
```bash
# 使用构建脚本（推荐）
./scripts/build.sh

# 或手动编译
go build -o go_blog main.go
```

### 6. 运行项目
```bash
# 使用启动脚本（推荐）
./scripts/start.sh

# 或直接运行
./go_blog
```

## 🔧 详细配置说明

### Go模块配置

项目使用Go Modules管理依赖，主要依赖包括：

```go
module go_blog

go 1.21

require (
    // 核心依赖会在go mod tidy时自动添加
)
```

### 模块间依赖关系

```
main.go
├── http (HTTP服务器)
│   ├── control (博客控制)
│   ├── exercise (锻炼管理)
│   ├── todolist (任务管理)
│   ├── reading (阅读管理)
│   ├── statistics (统计分析)
│   ├── llm (AI助手)
│   └── mcp (MCP协议)
├── view (视图渲染)
├── module (数据模型)
└── mylog (日志系统)
```

## 📝 构建脚本说明

### build.sh - 主构建脚本
```bash
#!/bin/bash
# 位置: ./scripts/build.sh

# 获取项目根目录
p=$(dirname $0)
p=$(realpath $p)
base_path=$(dirname "$p")

# 清理旧的可执行文件
if [ -e $base_path/go_blog ];then
    rm $base_path/go_blog
fi

echo "Building in: $base_path"
cd $base_path

# 整理依赖
go mod tidy

# 编译项目
go build
```

**使用方法**:
```bash
./scripts/build.sh
```

### 其他重要脚本

#### start.sh - 启动服务
```bash
./scripts/start.sh
```

#### stop.sh - 停止服务
```bash
./scripts/stop.sh
```

#### restart.sh - 重启服务
```bash
./scripts/restart.sh
```

#### start_all.sh - 启动所有服务（包括Redis）
```bash
./scripts/start_all.sh
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

### 应用配置
主要配置通过环境变量或代码中的常量设置:

- **端口**: 默认8080
- **数据目录**: `./datas/`
- **日志级别**: 可在代码中调整
- **Redis连接**: `127.0.0.1:6666`

## 🏗️ 编译选项和优化

### 开发环境编译
```bash
# 快速编译（开发时使用）
go build -o go_blog main.go

# 启用竞态检测
go build -race -o go_blog main.go
```

### 生产环境编译
```bash
# 优化编译（生产环境）
go build -ldflags="-s -w" -o go_blog main.go

# 静态链接编译
CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o go_blog main.go
```

### 交叉编译
```bash
# 编译为Windows版本
GOOS=windows GOARCH=amd64 go build -o go_blog.exe main.go

# 编译为macOS版本
GOOS=darwin GOARCH=amd64 go build -o go_blog_mac main.go
```

## 🐛 常见问题和解决方案

### 1. 编译错误：找不到模块
```bash
# 解决方案
go mod tidy
go clean -modcache
go mod download
```

### 2. Redis连接失败
```bash
# 检查Redis状态
redis-cli -p 6666 ping

# 启动Redis
./scripts/start_redis.sh
```

### 3. 端口占用问题
```bash
# 查看端口占用
lsof -i :8080

# 终止占用进程
kill -9 <PID>
```

### 4. 权限问题
```bash
# 给脚本执行权限
chmod +x scripts/*.sh

# 检查数据目录权限
ls -la datas/
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
    "go.useLanguageServer": true
}
```

### 推荐的Go工具
```bash
go install golang.org/x/tools/gopls@latest
go install github.com/ramya-rao-a/go-outline@latest
go install github.com/go-delve/delve/cmd/dlv@latest
```

## 📦 部署指南

### 开发部署
1. 使用构建脚本编译
2. 启动Redis服务
3. 运行应用程序
4. 访问 `http://localhost:8080`

### 生产部署
1. 使用优化编译选项
2. 配置systemd服务
3. 设置nginx反向代理
4. 配置SSL证书
5. 设置定时备份

### Docker部署（可选）
创建 `Dockerfile`:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o go_blog main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/go_blog .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/statics ./statics
CMD ["./go_blog"]
```

## 📊 性能优化建议

### 编译优化
- 使用 `-ldflags="-s -w"` 减小可执行文件大小
- 启用Go编译器优化
- 考虑使用 `upx` 压缩可执行文件

### 运行时优化
- 合理设置Redis内存限制
- 启用gzip压缩静态资源
- 使用CDN加速静态资源
- 配置适当的连接池大小

## 🧪 测试和调试

### 运行测试
```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./pkgs/http

# 生成测试覆盖率报告
go test -cover ./...
```

### 调试配置
```bash
# 使用delve调试器
dlv debug main.go

# 启用pprof性能分析
go build -o go_blog main.go
./go_blog -cpuprofile cpu.prof -memprofile mem.prof
```

## 📝 维护和更新

### 依赖更新
```bash
# 检查可更新的依赖
go list -u -m all

# 更新所有依赖
go get -u ./...
go mod tidy
```

### 版本管理
```bash
# 创建版本标签
git tag v1.0.0
git push origin v1.0.0

# 构建特定版本
git checkout v1.0.0
./scripts/build.sh
```

## 📞 支持和反馈

如遇到问题，请检查：
1. Go版本是否符合要求
2. 所有依赖是否正确安装
3. Redis服务是否正常运行
4. 端口是否被占用
5. 文件权限是否正确

---

**版本**: v1.0  
**更新日期**: 2024年  
**维护者**: Go Blog开发团队