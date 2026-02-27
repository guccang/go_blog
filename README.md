# Go Blog - 个人数字生活管理系统

<p align="center">
  一个基于 Go 语言的全功能个人数字生活管理平台，采用"一切皆博客"的创新设计理念，<br>
  将博客、任务管理、健身追踪、读书笔记、AI 助手、休闲游戏等深度整合为统一系统。
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24.0+-00ADD8?style=flat&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/Redis-6.0+-DC382D?style=flat&logo=redis" alt="Redis">
  <img src="https://img.shields.io/badge/AI-DeepSeek%20%7C%20OpenAI%20%7C%20Qwen-blueviolet?style=flat" alt="AI Models">
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat" alt="License">
</p>

---

## 目录

- [项目简介](#项目简介)
- [核心特性](#核心特性)
- [技术栈](#技术栈)
- [系统架构](#系统架构)
- [快速开始](#快速开始)
- [功能模块详解](#功能模块详解)
- [页面与路由](#页面与路由)
- [配置说明](#配置说明)
- [部署指南](#部署指南)
- [开发指南](#开发指南)
- [安全机制](#安全机制)
- [常见问题](#常见问题)
- [相关文档](#相关文档)
- [贡献指南](#贡献指南)
- [联系方式](#联系方式)

---

## 项目简介

Go Blog 不仅仅是一个博客系统——它是一个以 Markdown 文件为核心存储的个人数字生活管理平台。

所有数据（博客文章、待办事项、锻炼记录、读书笔记、年度计划等）统一以博客格式存储在 `blogs_txt/` 目录下，实现了：

- **零数据库依赖**：纯文件存储，`cp` 一下就能备份和迁移
- **数据完全自主**：所有数据都是可读的文本文件，不被任何数据库格式绑定
- **单文件部署**：编译后只有一个可执行文件，配合 Redis 缓存即可运行
- **AI 深度集成**：内置多模型 LLM 支持、MCP 协议、智能 Agent 和可插拔技能系统

---

## 核心特性

### 内容管理
- Markdown 博客编写与实时预览
- 多级权限控制（私有 / 公开 / 加密 / 协作 / 日记）
- 全文搜索、标签系统、评论系统
- 博客分享（支持密码保护和有效期）

### 生产力工具
- **待办事项** - 时间预估、拖拽排序、进度追踪
- **任务拆解** - 复杂任务的分层管理
- **年度计划** - 目标设置、月度分解、进度追踪
- **人生倒计时** - 重要日期提醒
- **财务管理** - 收支记录与分析

### 健康与阅读
- **锻炼记录** - 支持有氧、力量、柔韧、运动 4 种类型，智能卡路里计算
- **读书管理** - 书籍管理、进度追踪、读书笔记、阅读仪表盘

### AI 与智能化
- **多模型 LLM** - DeepSeek / OpenAI / Qwen，运行时动态切换，模型降级链
- **MCP 协议** - 文件系统访问、Redis 访问、博客数据查询、网页抓取
- **智能 Agent** - 任务拆解与自动执行、Cron 定时任务、自动报告生成
- **可插拔技能** - 用户创建 AI 技能卡扩展 AI 行为，无需改代码
- **WebSocket 实时通知** - 跨页面提醒推送

### 通知推送
- 邮件通知（SMTP）
- 短信通知
- 企业微信集成（Webhook 推送 + 应用消息 + 回调指令）

### 休闲游戏
- 五子棋、连连看（AI/PVP/竞速模式）、俄罗斯方块、扫雷、水果消消乐

---

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.24.0 |
| Web 框架 | 基于标准库 + 自研路由 |
| 缓存 | Redis (go-redis) |
| 存储 | Markdown 文件系统 |
| 模板引擎 | Go template |
| 实时通信 | WebSocket (gorilla/websocket) |
| 定时调度 | robfig/cron/v3 |
| JSON 处理 | bytedance/sonic |
| 加密 | AES-CBC |
| AI 集成 | DeepSeek / OpenAI / Qwen API + MCP 协议 |
| 前端 | HTML / CSS / JavaScript（原生） |

---

## 系统架构

### 设计理念

```
┌─────────────────────────────────────────────────────┐
│                   "一切皆博客"                        │
│                                                     │
│  博客文章 ──┐                                        │
│  待办事项 ──┤                                        │
│  锻炼记录 ──┼──→ 统一 Markdown 格式 ──→ blogs_txt/   │
│  读书笔记 ──┤         ↕                              │
│  年度计划 ──┘    Redis 缓存层                         │
└─────────────────────────────────────────────────────┘
```

### 项目结构

```
go_blog/
├── main.go                  # 应用入口
├── go.mod                   # 主模块定义（40+ replace 指令）
├── pkgs/                    # 40+ 独立 Go 模块
│   ├── core/                # 核心工具库
│   ├── module/              # 数据模型定义
│   ├── http/                # HTTP 服务与路由
│   ├── control/             # 控制层
│   ├── view/                # 视图渲染
│   ├── config/              # 配置管理
│   ├── persistence/         # 持久化层
│   ├── blog/                # 博客核心
│   ├── llm/                 # LLM 多模型集成
│   ├── mcp/                 # MCP 协议
│   ├── agent/               # 智能 Agent
│   ├── skill/               # AI 技能系统
│   ├── exercise/            # 锻炼管理
│   ├── reading/             # 读书管理
│   ├── todolist/            # 任务管理
│   └── ...                  # 更多模块
├── templates/               # 50+ HTML 模板
├── statics/                 # 静态资源 (CSS/JS/图片)
├── scripts/                 # 构建与部署脚本
├── blogs_txt/               # 数据存储目录
│   ├── sys_conf.md          # 系统配置
│   └── {username}/          # 用户数据目录
├── redis/                   # Redis 配置
└── logs/                    # 应用日志
```

### 模块化架构

每个 `pkgs/` 下的包都是独立的 Go Module，拥有自己的 `go.mod`。主模块通过 `replace` 指令引用本地模块：

```go
replace blog => ./pkgs/blog
replace exercise => ./pkgs/exercise
replace llm => ./pkgs/llm
// ... 40+ modules
```

### 启动流程

```
main.go
  ├── 信号处理 (SIGTERM/SIGINT) → 优雅关闭
  ├── 加载配置 (config.Init)
  ├── 初始化日志 (mylog.Init)
  ├── 初始化持久化层 (persistence.Init)
  ├── 初始化业务模块 (blog/comment/reading/statistics/...)
  ├── 初始化认证 (auth/login)
  ├── 初始化 AI (mcp/llm/agent)
  ├── 初始化通知 (sms/exercise/share)
  └── 启动 HTTP 服务 (支持 HTTPS)
```

---

## 快速开始

### 环境要求

- Go 1.24.0+
- Redis
- Git

### 安装与运行

```bash
# 1. 克隆仓库
git clone https://github.com/guccang/go_blog.git
cd go_blog

# 2. 构建
./scripts/build.sh
# 或手动：go mod tidy && go build

# 3. 启动 Redis
./scripts/start_redis.sh

# 4. 配置（编辑最小配置）
# 编辑 blogs_txt/sys_conf.md，设置 admin/pwd/port/redis_ip/redis_port

# 5. 启动
./go_blog blogs_txt/sys_conf.md

# 启用 HTTPS
./go_blog blogs_txt/sys_conf.md cert.pem key.pem
```

### 使用脚本管理

```bash
./scripts/start.sh        # 后台启动
./scripts/stop.sh         # 停止服务
./scripts/restart.sh      # 重启服务
./scripts/show.sh         # 查看运行状态
```

### 最小配置

在 `blogs_txt/sys_conf.md` 中配置以下 5 项即可启动：

```ini
admin=yourname
pwd=yourpassword
port=8888
redis_ip=127.0.0.1
redis_port=6379
```

---

## 功能模块详解

### 核心系统

| 模块 | 说明 |
|------|------|
| `module` | 核心数据结构（Blog、User、CommentUser 等类型定义） |
| `core` | 核心工具库 |
| `http` | HTTP 服务器、路由注册与请求处理 |
| `control` | 控制层，请求分发与业务编排 |
| `view` | 视图渲染，模板管理 |
| `config` | 配置管理，读取 sys_conf.md |
| `persistence` | 持久化层，文件读写 + Redis 缓存 |
| `mylog` | 日志系统，支持文件和控制台输出 |
| `ioutils` | 文件 I/O 工具集 |

### 内容与社交

| 模块 | 说明 |
|------|------|
| `blog` | 博客 CRUD、Markdown 编写、权限控制、实时预览 |
| `comment` | 评论系统 |
| `search` | 全文搜索 |
| `share` | 博客分享，支持密码保护和有效期 |
| `statistics` | 数据统计与分析 |

### 用户与认证

| 模块 | 说明 |
|------|------|
| `auth` | 认证与授权 |
| `login` | 登录/登出 |
| `account` | 账户管理 |
| `encryption` | AES-CBC 加密服务 |

### 生产力

| 模块 | 说明 |
|------|------|
| `todolist` | 待办事项，时间预估、拖拽排序 |
| `taskbreakdown` | 任务拆解，分层管理 |
| `yearplan` | 年度计划，目标设置与月度分解 |
| `lifecountdown` | 人生倒计时 |
| `exercise` | 锻炼记录（有氧/力量/柔韧/运动），智能卡路里计算 |
| `reading` | 读书管理，进度追踪与笔记 |
| `finance` | 财务管理 |

### AI 与智能化

| 模块 | 说明 |
|------|------|
| `llm` | 多模型 LLM 集成（DeepSeek/OpenAI/Qwen），流式响应，动态切换，降级链 |
| `mcp` | MCP 协议，文件访问、Redis 访问、网页抓取（GBK/UTF-8 自动编码） |
| `agent` | 智能任务编排，Cron 定时任务，自动报告，WebSocket 实时通知 |
| `skill` | 可插拔 AI 技能系统，用户创建技能卡扩展 AI 行为 |
| `codegen` | 代码生成工具 |

### 通知与通信

| 模块 | 说明 |
|------|------|
| `email` | 邮件发送（SMTP） |
| `sms` | 短信通知 |
| `wechat` | 企业微信集成（Webhook/应用消息/回调处理） |

### 休闲游戏

| 模块 | 说明 |
|------|------|
| `gomoku` | 五子棋 |
| `linkup` | 连连看（AI/PVP/竞速模式） |
| `tetris` | 俄罗斯方块 |
| `minesweeper` | 扫雷 |
| `fruitcrush` | 水果消消乐 |

### 工具

| 模块 | 说明 |
|------|------|
| `tools` | 通用工具集 |
| `constellation` | 星座功能 |
| `realtime` | 实时功能支持 |

---

## 页面与路由

### 主要页面

| 路由 | 页面 | 说明 |
|------|------|------|
| `/main` | 主控台 | 系统仪表盘，集成所有功能入口、全局搜索、热门标签 |
| `/assistant` | AI 助手 | 多模型对话、流式响应、MCP 工具调用、上下文记忆 |
| `/agent` | 智能 Agent | 任务编排、自动执行、可视化进度 |
| `/login` | 登录 | 用户认证 |

### 功能页面

| 路由 | 页面 | 说明 |
|------|------|------|
| `/exercise` | 锻炼管理 | 记录与统计 |
| `/reading` | 读书管理 | 书籍与笔记 |
| `/reading_dashboard` | 阅读仪表盘 | 阅读数据分析 |
| `/todolist` | 待办事项 | 任务管理 |
| `/yearplan` | 年度计划 | 目标追踪 |
| `/taskbreakdown` | 任务拆解 | 分层管理 |
| `/lifecountdown` | 人生倒计时 | 日期提醒 |
| `/finance` | 财务管理 | 收支记录 |
| `/statistics` | 统计分析 | 数据可视化 |
| `/skill` | AI 技能 | 技能卡管理 |
| `/account` | 账户管理 | 用户设置 |
| `/config` | 系统配置 | 配置管理 |

### 游戏页面

| 路由 | 页面 |
|------|------|
| `/games` | 游戏大厅 |
| `/gomoku` | 五子棋 |
| `/linkup` | 连连看 |
| `/tetris` | 俄罗斯方块 |
| `/minesweeper` | 扫雷 |
| `/fruitcrush` | 水果消消乐 |

---

## 配置说明

配置文件为 `blogs_txt/sys_conf.md`，采用 `key=value` 格式。

### 必填配置

```ini
admin=yourname          # 管理员账号
pwd=yourpassword        # 管理员密码
port=8888               # HTTP 端口
redis_ip=127.0.0.1      # Redis 地址
redis_port=6379         # Redis 端口
```

### AI 配置（推荐）

```ini
# DeepSeek（推荐）
deepseek_api_key=sk-xxxxxxxx
deepseek_api_url=https://api.deepseek.com/chat/completions

# OpenAI（可选）
openai_api_key=sk-xxxxxxxx
openai_api_url=https://api.openai.com/v1/chat/completions

# 通义千问（可选）
qwen_api_key=sk-xxxxxxxx
qwen_api_url=https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions

# 模型降级链
llm_fallback_models=["openai","qwen"]
```

### 通知配置（可选）

```ini
# 邮件
email_from=you@gmail.com
email_password=app_password
smtp_host=smtp.gmail.com
smtp_port=587
email_to=notify@example.com

# 企业微信
wechat_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY

# 短信
sms_phone=13800138000
sms_send_url=https://push.spug.cc/send/YOUR_KEY
```

### 更多配置

```ini
publictags=技术|生活|读书       # 公开标签
diary_keywords=日记_|私人_      # 日记关键词（需密码访问）
diary_password=your_password    # 日记密码
main_show_blogs=67              # 主页显示博客数
share_days=7                    # 分享链接有效天数
logs_dir=./logs                 # 日志目录
```

> 详细配置说明请参考 [SYS_CONF_GUIDE.md](SYS_CONF_GUIDE.md)

---

## 部署指南

### 开发环境

```bash
go mod tidy
go build
./go_blog blogs_txt/sys_conf.md
```

### 生产环境编译

```bash
# 优化编译（减小体积）
go build -ldflags="-s -w" -o go_blog

# 静态链接（Linux）
CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o go_blog
```

### 交叉编译

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o go_blog

# Windows
GOOS=windows GOARCH=amd64 go build -o go_blog.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o go_blog_mac
```

### 生产部署清单

1. 编译生产版本
2. 配置 `sys_conf.md`
3. 启动 Redis
4. 启动应用：`nohup ./go_blog blogs_txt/sys_conf.md &`
5. （推荐）配置 Nginx 反向代理 + SSL
6. （推荐）配置定时备份 `blogs_txt/` 目录

### HTTPS 支持

```bash
# 直接使用证书
./go_blog blogs_txt/sys_conf.md cert.pem key.pem

# 或通过 Nginx 反向代理
```

---

## 开发指南

### 添加新模块

1. 创建目录 `pkgs/newmodule/`
2. 初始化模块：

```bash
cd pkgs/newmodule
go mod init newmodule
```

3. 在主 `go.mod` 添加：

```go
replace newmodule => ./pkgs/newmodule
```

4. 实现模块功能（通常包含 `module.go`、`actor.go`、`cmd.go`）
5. 在 `main.go` 中注册初始化

### 模块间依赖

```
main.go
├── config       ← 配置加载
├── persistence  ← 数据持久化
├── blog         ← 博客核心
├── http         ← HTTP 路由
│   ├── control  ← 请求处理
│   └── view     ← 视图渲染
├── llm          ← AI 集成
├── mcp          ← MCP 协议
├── agent        ← 智能 Agent
└── ...
```

### 测试

```bash
# 测试单个模块
cd pkgs/module_name && go test

# 测试所有模块
go test ./...
```

### 调试

```bash
# 查看日志
tail -f logs/blog_*.log

# 监控 Redis
redis-cli -p 6666 monitor

# 查看进程
./scripts/show.sh
```

---

## 安全机制

### 权限模型

```go
EAuthType_private     = 1    // 私有
EAuthType_public      = 2    // 公开
EAuthType_encrypt     = 4    // 加密
EAuthType_cooperation = 8    // 协作
EAuthType_diary       = 16   // 日记（密码保护）
```

支持组合权限，例如 `公开 + 加密` 表示公开但需要密码查看。

### 安全特性

- AES-CBC 内容加密
- 日记密码保护
- 分享链接有效期控制
- 多用户数据隔离（每用户独立目录）
- 支持 HTTPS

### 安全建议

- 生产环境务必修改默认密码
- API Key 不要提交到 Git
- 配置文件设置 `chmod 600`
- 生产环境启用 HTTPS

---

## 常见问题

| 问题 | 解决方案 |
|------|----------|
| 编译找不到模块 | `go mod tidy && go clean -modcache && go mod download` |
| Redis 连接失败 | 检查 Redis 是否启动：`redis-cli -p 6666 ping` |
| 端口被占用 | `lsof -i :8888` 或 `netstat -tlnp \| grep 8888` |
| AI 助手不可用 | 检查 `deepseek_api_key` 等 API Key 配置 |
| 邮件发送失败 | 检查 SMTP 配置和应用专用密码 |
| 企业微信无通知 | 检查 `wechat_webhook` URL 是否有效 |
| 脚本无执行权限 | `chmod +x scripts/*.sh` |

---

## 相关文档

- [编译配置说明](BUILD_GUIDE.md) - 详细的编译选项和环境配置
- [系统配置说明](SYS_CONF_GUIDE.md) - 完整的配置项说明和示例

---

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/your-feature`
3. 提交更改：`git commit -m 'feat: add your feature'`
4. 推送分支：`git push origin feature/your-feature`
5. 提交 Pull Request

---

## 联系方式

[guccang@gmail.com](mailto:guccang@gmail.com)

---

## 许可证

[MIT License](LICENSE)
