# Go Blog - 个人数字生活管理平台

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.24.0+-00ADD8?style=for-the-badge&logo=go)
![Redis](https://img.shields.io/badge/Redis-6.0+-DC382D?style=for-the-badge&logo=redis)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)
![AI Models](https://img.shields.io/badge/AI-DeepSeek%20|%20OpenAI%20|%20Qwen-blueviolet?style=for-the-badge)
![Architecture](https://img.shields.io/badge/Architecture-Monorepo%20+%20Microservices-blue?style=for-the-badge)

**一个基于 Go 语言的现代化个人数字生活管理平台，采用"一切皆博客"的创新设计理念**

[项目简介](#-项目简介) • [核心特性](#-核心特性) • [快速开始](#-快速开始) • [功能模块](#-功能模块) • [配置说明](#-配置说明) • [部署指南](#-部署指南)

</div>

---

## 🎯 项目简介

Go Blog 不仅仅是一个博客系统——它是一个**以 Markdown 文件为核心存储的个人数字生活管理平台**。所有数据（博客文章、待办事项、锻炼记录、读书笔记、年度计划等）统一以博客格式存储在 `blogs_txt/` 目录下，实现了真正的**数据自主**和**零数据库依赖**。

### ✨ 核心理念：一切皆博客

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

### 🏗️ 架构特点

- **📦 Monorepo 结构**: 40+ 独立 Go 模块，支持本地模块替换 (`replace` 指令)
- **💾 零数据库依赖**: 纯文件存储 + Redis 缓存，数据完全自主可控
- **👥 多账户支持**: 完善的租户隔离，支持 `WithAccount` 后缀的多租户 API
- **🤖 AI 深度集成**: 内置多模型 LLM 支持、MCP 协议、智能 Agent 和可插拔技能系统
- **⚡ 高性能**: 基于 Go 原生 HTTP，支持协程并发，内存占用低
- **🔒 安全可靠**: 完善的权限模型、内容加密、HTTPS 支持

---

## 🚀 核心特性

### 📝 内容管理
- **Markdown 编辑器**: 实时预览、语法高亮、图片上传
- **权限控制**: 5 种权限类型（私有/公开/加密/协作/日记）
- **搜索系统**: 全文搜索、标签系统、智能推荐
- **分享功能**: 密码保护、有效期控制、访问统计

### 🎯 生产力工具
- **✅ 待办事项**: 时间预估、拖拽排序、进度追踪、优先级管理
- **📋 任务拆解**: 复杂任务的层级分解、依赖关系、进度可视化
- **📅 年度计划**: 年度目标设定、月度分解、进度追踪、成果展示
- **⏳ 人生倒计时**: 重要日期提醒、进度可视化
- **💰 财务管理**: 收支记录、分类统计、报表生成

### 🏋️ 健康与阅读
- **💪 锻炼记录**: 4 种类型（有氧/力量/柔韧/运动）、智能卡路里计算、历史统计
- **📚 读书管理**: 书籍管理、进度追踪、读书笔记、阅读仪表盘

### 🤖 AI 与智能化
- **🧠 多模型 LLM**: DeepSeek / OpenAI / Qwen 多模型支持，运行时动态切换
- **🔧 MCP 协议**: 文件系统访问、Redis 操作、博客数据查询、网页抓取
- **🤖 智能 Agent**: 任务自动拆解与执行、Cron 定时任务、WebSocket 实时通知
- **🃏 可插拔技能**: 用户创建 AI 技能卡扩展 AI 行为，无需修改代码
- **💬 实时对话**: 流式响应、上下文记忆、工具调用、历史记录

### 📱 通知推送
- **📧 邮件通知**: SMTP 支持，HTML 模板
- **📱 短信通知**: 支持多个短信平台
- **💬 企业微信**: Webhook 推送、应用消息、回调指令处理

### 🎮 休闲游戏
- **♟️ 五子棋**: 人机对战、双人对战
- **🔗 连连看**: AI/PVP/竞速三种模式
- **🧩 俄罗斯方块**: 经典玩法、高分榜
- **💣 扫雷**: 多种难度、计时模式
- **🍎 水果消消乐**: 休闲益智、关卡设计

---

## 🛠️ 技术栈

| 类别 | 技术选型 | 说明 |
|------|----------|------|
| **后端语言** | Go 1.24.0+ | 高性能、并发支持好 |
| **Web 框架** | 标准库 + 自研路由 | 轻量级、高性能 |
| **缓存系统** | Redis (go-redis) | 会话管理、数据缓存 |
| **数据存储** | Markdown 文件系统 | 零数据库依赖、数据自主 |
| **模板引擎** | Go template | 服务端渲染 |
| **实时通信** | WebSocket (gorilla/websocket) | 实时通知、AI 流式响应 |
| **定时调度** | robfig/cron/v3 | 定时任务、自动报告 |
| **JSON 处理** | bytedance/sonic | 高性能 JSON 编解码 |
| **加密算法** | AES-CBC | 内容加密、密码保护 |
| **AI 集成** | DeepSeek / OpenAI / Qwen API | 多模型支持、MCP 协议 |
| **前端技术** | HTML / CSS / JavaScript | 原生技术、无框架依赖 |

---

## 📁 项目结构

```
go_blog/
├── cmd/                    # 多代理系统（微服务架构）
│   ├── blog-agent/        # 主博客应用（核心）
│   │   ├── go.mod        # 主模块定义（40+ replace 指令）
│   │   ├── main.go       # 主程序入口
│   │   ├── pkgs/         # 40+ 核心模块
│   │   │   ├── auth/    # 认证授权
│   │   │   ├── blog/    # 博客核心功能
│   │   │   ├── config/  # 配置管理
│   │   │   ├── http/    # HTTP 服务器
│   │   │   ├── mylog/   # 日志系统
│   │   │   ├── persistence/ # 持久化存储
│   │   │   └── ...      # 其他 35+ 模块
│   │   └── scripts/     # 构建和运行脚本
│   ├── wechat-agent/      # 微信代理服务
│   ├── llm-mcp-agent/     # LLM MCP 协议代理
│   ├── codegen-agent/     # 代码生成代理
│   ├── deploy-agent/      # 部署代理
│   └── gateway/          # API 网关
├── blogs_txt/            # Markdown 数据存储
├── templates/            # 50+ HTML 模板
├── statics/             # 静态资源（CSS/JS/图片）
├── redis/               # Redis 配置文件
├── logs/               # 应用日志目录
└── docs/               # 项目文档
```

### 🧩 模块化设计

项目采用独特的**本地模块替换**机制，每个模块都是独立的 Go Module：

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

每个模块遵循标准初始化模式：
```go
func Info() {
    log.InfoF(log.ModuleBlog, "info blog v4.0")
}

func Init() {
    log.Debug(log.ModuleBlog, "blog module Init")
    // 初始化逻辑
}
```

---

## 🤖 多Agent系统架构

Go Blog 采用**微服务+多Agent**架构设计，通过 `cmd/` 目录下的多个独立Agent实现功能解耦和水平扩展。所有Agent通过统一的 `gateway` 进行通信和协调，形成一个智能的分布式系统。

### 🧭 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    用户界面 & API 调用                         │
└─────────────────────────────┬───────────────────────────────┘
                              │
                    ┌─────────▼──────────┐
                    │     Gateway        │  ←─ Agent通信中枢
                    │   (消息路由/注册中心)  │
                    └─────────┬──────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼──────┐     ┌───────▼──────┐     ┌───────▼──────┐
│  LLM-MCP-Agent │     │  WeChat-Agent  │     │  Deploy-Agent   │
│   (AI大脑)     │     │  (微信接入)    │     │  (部署引擎)     │
└───────┬──────┘     └───────┬──────┘     └───────┬──────┘
        │                     │                     │
┌───────▼──────┐     ┌───────▼──────┐     ┌───────▼──────┐
│  CodeGen-Agent │     │  外部微信服务  │     │  部署目标服务器  │
│  (代码生成)    │     │              │     │              │
└───────────────┘     └──────────────┘     └──────────────┘
```

### 🚪 Gateway (网关/消息路由)

**位置**: `cmd/gateway/`  
**作用**: Agent通信中枢，负责：
- **Agent注册与发现**: 管理所有Agent的连接状态
- **消息路由**: 在Agent间转发请求和响应
- **工具目录管理**: 收集并发布所有Agent提供的工具
- **健康检查**: 监控Agent状态，自动重连
- **HTTP反向代理**: 将用户请求转发到主博客应用

**关键特性**:
- 基于WebSocket的UAP (Universal Agent Protocol) 协议
- 支持Agent动态加入和离开
- 提供统一的工具发现API
- 与主博客应用无缝集成

### 🧠 LLM-MCP-Agent (AI大脑)

**位置**: `cmd/llm-mcp-agent/`  
**作用**: 系统的智能指挥中心，负责：
- **多模型LLM集成**: 支持DeepSeek、OpenAI、Qwen等模型
- **MCP协议支持**: 提供文件访问、Redis操作、博客查询等工具
- **任务协调**: 根据用户指令调用其他Agent完成任务
- **工具发现**: 动态发现并利用其他Agent提供的工具
- **模型降级**: 智能切换模型确保服务可用性

**工作流程**:
1. 连接Gateway，注册自身为"大脑"Agent
2. 从Gateway获取所有可用工具目录
3. 接收用户指令，分析任务需求
4. 调用合适的工具和Agent完成任务
5. 将结果返回给用户

### 💻 CodeGen-Agent (代码生成)

**位置**: `cmd/codegen-agent/`  
**作用**: 自动化代码生成和工程任务，负责：
- **代码生成**: 根据模板和需求生成代码文件
- **代码重构**: 自动重构和优化现有代码
- **项目分析**: 分析代码结构，提供改进建议
- **与Deploy-Agent协同**: 生成代码后自动触发部署

**开发部署**:
```bash
# 1. 配置Agent连接
cd cmd/codegen-agent
cp agent.conf.example agent.conf
# 编辑agent.conf，设置Gateway地址和工作空间

# 2. 编译运行
go build -o codegen-agent main.go
./codegen-agent

# 3. 通过LLM-MCP-Agent调用
# LLM会自动发现并调用CodeGen-Agent的工具
```

### 🚀 Deploy-Agent (部署引擎)

**位置**: `cmd/deploy-agent/`  
**作用**: 自动化部署和发布，负责：
- **多目标部署**: 支持本地、SSH、多服务器部署
- **交叉编译**: 自动为不同平台编译二进制文件
- **流水线执行**: 支持复杂的部署流水线
- **凭据管理**: 安全存储SSH密码和部署密钥
- **部署验证**: 自动验证部署结果

**代码编写示例**:
```go
// 部署配置示例 (deploy.conf)
[projects]
[projects.go_blog]
project_dir = "/path/to/go_blog"
pack_script = "./scripts/build.sh"
targets = [
    { name = "local", host = "localhost", remote_dir = "/tmp/deploy" },
    { name = "prod", host = "192.168.1.100", remote_dir = "/opt/go_blog" }
]

// 通过LLM-MCP-Agent触发部署
// LLM可以调用 deploy_agent.execute_pipeline 工具
```

### 💬 WeChat-Agent (微信接入)

**位置**: `cmd/wechat-agent/`  
**作用**: 企业微信集成，负责：
- **消息接收**: 处理企业微信回调消息
- **消息转发**: 将微信消息转发给LLM-MCP-Agent处理
- **响应发送**: 将AI响应发送回企业微信
- **指令解析**: 解析微信消息中的指令和参数

**微信接入配置**:
1. 在企业微信后台创建应用，获取Webhook URL
2. 配置回调地址指向WeChat-Agent
3. WeChat-Agent连接Gateway，注册微信工具
4. LLM-MCP-Agent通过Gateway调用微信发送能力

**配置示例** (`wechat-agent.json`):
```json
{
  "http_port": 8081,
  "gateway_url": "ws://localhost:8080/ws/uap",
  "wechat_webhook": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY",
  "callback_token": "YOUR_CALLBACK_TOKEN"
}
```

### 🔧 各Agent协同工作示例

**场景**: 用户通过企业微信发送指令"请更新博客系统并部署到生产环境"

1. **WeChat-Agent** 接收微信消息，通过Gateway转发给LLM-MCP-Agent
2. **LLM-MCP-Agent** 分析指令，拆解为两个任务：
   - 任务A: 使用CodeGen-Agent更新博客代码
   - 任务B: 使用Deploy-Agent部署到生产环境
3. **LLM-MCP-Agent** 通过Gateway调用CodeGen-Agent的代码生成工具
4. **CodeGen-Agent** 完成代码更新，返回结果
5. **LLM-MCP-Agent** 通过Gateway调用Deploy-Agent的部署工具
6. **Deploy-Agent** 执行部署流水线，返回部署结果
7. **LLM-MCP-Agent** 汇总结果，通过Gateway发送给WeChat-Agent
8. **WeChat-Agent** 将最终结果发送回企业微信

### ⚙️ Agent开发指南

#### 1. 创建新的Agent
```bash
# 创建Agent目录结构
mkdir -p cmd/new-agent
cd cmd/new-agent

# 初始化Go模块
go mod init new-agent

# 实现Agent主程序
# 参考现有Agent实现连接Gateway的逻辑
```

#### 2. 实现UAP协议连接
```go
// 连接Gateway的示例代码
func connectToGateway(gatewayURL string, agentID string) {
    conn, _, err := websocket.DefaultDialer.Dial(gatewayURL, nil)
    if err != nil {
        log.Fatal("连接Gateway失败:", err)
    }
    
    // 发送注册消息
    registerMsg := map[string]interface{}{
        "type": "register",
        "agent_id": agentID,
        "capabilities": []string{"tool1", "tool2"},
    }
    conn.WriteJSON(registerMsg)
    
    // 处理消息循环
    for {
        var msg map[string]interface{}
        err := conn.ReadJSON(&msg)
        if err != nil {
            break
        }
        processMessage(msg)
    }
}
```

#### 3. 提供工具接口
```go
// 定义工具描述
type Tool struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Parameters  []Param  `json:"parameters"`
}

// 注册工具到Gateway
func registerTools(conn *websocket.Conn, tools []Tool) {
    msg := map[string]interface{}{
        "type": "tool_list",
        "tools": tools,
    }
    conn.WriteJSON(msg)
}
```

#### 4. 配置管理
每个Agent应有独立的配置文件，支持：
- Gateway连接地址
- Agent特定参数
- 凭据和安全配置
- 工作目录和资源路径

---

## ⚡ 快速开始

### 📋 环境要求

- **Go 1.24.0+** (推荐 1.24.10)
- **Redis 6.0+**
- **Git**

### 🚀 5 分钟快速启动

```bash
# 1. 克隆项目
git clone https://github.com/guccang/go_blog.git
cd go_blog

# 2. 安装依赖
cd cmd/blog-agent
go mod tidy

# 3. 启动 Redis
redis-server &
# 或使用系统 Redis

# 4. 编译项目
./scripts/build.sh

# 5. 配置系统
# 编辑 blogs_txt/sys_conf.md，设置管理员账户和密码

# 6. 启动应用
./scripts/start.sh

# 7. 访问应用
open http://localhost:8080
```

### ⚙️ 最小配置

创建 `blogs_txt/sys_conf.md`，只需 5 行配置即可启动：

```ini
admin=yourname          # 管理员账号
pwd=yourpassword        # 管理员密码
port=8080               # HTTP 端口
redis_ip=127.0.0.1      # Redis 地址
redis_port=6379         # Redis 端口
```

### 🛠️ 常用脚本

```bash
# 构建脚本
./scripts/build.sh       # 清理并重新编译

# 服务管理
./scripts/start.sh       # 后台启动
./scripts/stop.sh        # 停止服务
./scripts/restart.sh     # 重启服务
./scripts/show.sh        # 查看运行状态

# Redis 管理
./scripts/start_redis.sh # 启动 Redis
./scripts/stop_redis.sh  # 停止 Redis
```

### 🤖 启动多Agent系统

```bash
# 1. 启动Gateway (Agent通信中枢)
cd cmd/gateway
go build -o gateway main.go
./gateway &

# 2. 启动LLM-MCP-Agent (AI大脑)
cd cmd/llm-mcp-agent
go build -o llm-mcp-agent main.go
./llm-mcp-agent &

# 3. 启动WeChat-Agent (微信接入)
cd cmd/wechat-agent
go build -o wechat-agent main.go
./wechat-agent &

# 4. 启动其他Agent...
# 所有Agent会自动连接到Gateway，形成协同工作网络
```

---

## 🧩 功能模块详解

### 核心系统模块

| 模块 | 功能 | 关键特性 |
|------|------|----------|
| **`module`** | 数据模型 | Blog、User、CommentUser 等核心数据结构 |
| **`core`** | 核心工具 | 通用工具函数、错误处理、类型转换 |
| **`http`** | HTTP 服务 | 路由注册、中间件、请求处理 |
| **`control`** | 控制层 | 业务逻辑编排、请求分发 |
| **`view`** | 视图层 | HTML 模板渲染、页面组装 |
| **`config`** | 配置管理 | sys_conf.md 解析、配置项读取 |
| **`persistence`** | 持久化 | 文件读写 + Redis 缓存、数据一致性 |
| **`mylog`** | 日志系统 | 文件/控制台输出、日志级别、轮转 |
| **`ioutils`** | I/O 工具 | 文件操作、目录遍历、内容读取 |

### 内容与社交模块

| 模块 | 功能 | 关键特性 |
|------|------|----------|
| **`blog`** | 博客管理 | Markdown 编辑、实时预览、权限控制 |
| **`comment`** | 评论系统 | 评论发布、回复、管理 |
| **`search`** | 全文搜索 | 关键词搜索、标签搜索、结果排序 |
| **`share`** | 内容分享 | 密码保护、有效期控制、访问统计 |
| **`statistics`** | 统计分析 | 数据可视化、报表生成、趋势分析 |

### 用户与认证模块

| 模块 | 功能 | 关键特性 |
|------|------|----------|
| **`auth`** | 认证授权 | 会话管理、权限验证、多账户支持 |
| **`login`** | 登录系统 | 登录/登出、记住我、安全验证 |
| **`account`** | 账户管理 | 用户信息、设置管理、数据导出 |
| **`encryption`** | 加密服务 | AES-CBC 加密、密码哈希、密钥管理 |

### 生产力工具模块

| 模块 | 功能 | 关键特性 |
|------|------|----------|
| **`todolist`** | 待办事项 | 时间预估、拖拽排序、进度追踪 |
| **`taskbreakdown`** | 任务拆解 | 层级分解、依赖关系、进度可视化 |
| **`yearplan`** | 年度计划 | 目标设定、月度分解、成果追踪 |
| **`lifecountdown`** | 人生倒计时 | 重要日期提醒、进度可视化 |
| **`exercise`** | 锻炼管理 | 4 种类型记录、卡路里计算、历史统计 |
| **`reading`** | 读书管理 | 书籍管理、进度追踪、笔记系统 |
| **`finance`** | 财务管理 | 收支记录、分类统计、报表生成 |

### AI 与智能模块

| 模块 | 功能 | 关键特性 |
|------|------|----------|
| **`llm`** | 大语言模型 | 多模型支持、流式响应、动态切换 |
| **`mcp`** | MCP 协议 | 文件访问、Redis 操作、网页抓取 |
| **`agent`** | 智能代理 | 任务自动执行、定时调度、实时通知 |
| **`skill`** | AI 技能系统 | 可插拔技能卡、用户自定义、无需改代码 |
| **`codegen`** | 代码生成 | 模板生成、代码优化、自动重构 |

### 通知与通信模块

| 模块 | 功能 | 关键特性 |
|------|------|----------|
| **`email`** | 邮件通知 | SMTP 支持、HTML 模板、附件发送 |
| **`sms`** | 短信通知 | 多平台支持、模板消息、发送状态 |
| **`wechat`** | 企业微信 | Webhook 推送、应用消息、回调处理 |

### 休闲游戏模块

| 模块 | 游戏 | 特性 |
|------|------|------|
| **`gomoku`** | 五子棋 | 人机对战、双人对战、智能 AI |
| **`linkup`** | 连连看 | AI/PVP/竞速模式、多难度 |
| **`tetris`** | 俄罗斯方块 | 经典玩法、高分榜、音效 |
| **`minesweeper`** | 扫雷 | 多种难度、计时模式、自动标记 |
| **`fruitcrush`** | 水果消消乐 | 休闲益智、关卡设计、特效 |

---

## ⚙️ 配置说明

### 📋 基础配置 (必需)

```ini
# 管理员账户
admin=yourname
pwd=yourpassword

# 服务器配置
port=8080
logs_dir=./logs

# Redis 配置
redis_ip=127.0.0.1
redis_port=6379
```

### 🤖 AI 配置 (推荐)

```ini
# DeepSeek (推荐免费模型)
deepseek_api_key=sk-xxxxxxxx
deepseek_api_url=https://api.deepseek.com/chat/completions

# OpenAI (备用)
openai_api_key=sk-xxxxxxxx
openai_api_url=https://api.openai.com/v1/chat/completions

# 通义千问 (备用)
qwen_api_key=sk-xxxxxxxx
qwen_api_url=https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions

# 模型降级链
llm_fallback_models=["openai","qwen"]
```

### 📧 通知配置 (可选)

```ini
# 邮件通知
email_from=you@gmail.com
email_password=app_password
smtp_host=smtp.gmail.com
smtp_port=587
email_to=notify@example.com

# 企业微信
wechat_webhook=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY

# 短信通知
sms_phone=13800138000
sms_send_url=https://push.spug.cc/send/YOUR_KEY
```

### 🔒 安全配置

```ini
# 权限控制
publictags=技术|生活|读书       # 公开标签
diary_keywords=日记_|私人_      # 日记关键词
diary_password=your_password    # 日记访问密码

# 分享设置
share_days=7                    # 分享链接有效期
main_show_blogs=67              # 主页显示博客数量
```

> 完整配置说明请参考 [SYS_CONF_GUIDE.md](SYS_CONF_GUIDE.md)

---

## 🚢 部署指南

### 🏗️ 编译选项

```bash
# 开发环境编译
go build -o go_blog cmd/blog-agent/main.go

# 生产环境优化编译 (减小 30%+ 体积)
go build -ldflags="-s -w" -o go_blog cmd/blog-agent/main.go

# 静态链接编译 (Linux，无外部依赖)
CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o go_blog cmd/blog-agent/main.go
```

### 🔀 交叉编译

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o go_blog.exe cmd/blog-agent/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o go_blog_mac cmd/blog-agent/main.go

# Linux ARM (树莓派等)
GOOS=linux GOARCH=arm64 go build -o go_blog_arm64 cmd/blog-agent/main.go
```

### 🐳 Docker 部署 (示例)

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN cd cmd/blog-agent && go mod tidy && go build -ldflags="-s -w" -o /go_blog

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go_blog .
COPY blogs_txt/ ./blogs_txt/
COPY templates/ ./templates/
COPY statics/ ./statics/
EXPOSE 8080
CMD ["./go_blog", "blogs_txt/sys_conf.md"]
```

### 📦 生产部署清单

1. **编译优化版本**: `go build -ldflags="-s -w"`
2. **配置安全设置**: 修改默认密码、API Key
3. **启动 Redis**: 配置持久化和内存限制
4. **配置反向代理**: Nginx + SSL (推荐)
5. **设置系统服务**: systemd 或 supervisor
6. **配置监控**: 日志轮转、健康检查
7. **定期备份**: `blogs_txt/` 目录备份

### 🔐 HTTPS 配置

```bash
# 直接使用证书运行
./go_blog blogs_txt/sys_conf.md cert.pem key.pem

# 或通过 Nginx 配置 SSL
server {
    listen 443 ssl;
    server_name yourdomain.com;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

## 🧪 测试与开发

### 🔧 开发环境设置

1. **安装 Go 工具链**:
   ```bash
   go install golang.org/x/tools/gopls@latest      # 语言服务器
   go install github.com/go-delve/delve/cmd/dlv@latest  # 调试器
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest  # 代码检查
   ```

2. **VS Code 配置** (`.vscode/settings.json`):
   ```json
   {
     "go.buildOnSave": "package",
     "go.lintOnSave": "package",
     "go.useLanguageServer": true,
     "go.toolsEnvVars": {"GO111MODULE": "on"}
   }
   ```

### 🧪 运行测试

```bash
# 进入主应用目录
cd cmd/blog-agent

# 运行所有测试
go test ./...

# 运行单个模块测试
cd pkgs/encryption && go test

# 运行特定测试函数
go test -v ./pkgs/encryption -run TestAesSimpleEncrypt

# 带竞态检测的测试
go test -race ./...

# 生成覆盖率报告
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 🐛 调试与监控

```bash
# 查看应用日志
tail -f logs/app.log

# 监控 Redis
redis-cli monitor

# 调试模式运行
dlv debug cmd/blog-agent/main.go

# 性能分析
go build -o go_blog cmd/blog-agent/main.go
./go_blog blogs_txt/sys_conf.md -cpuprofile cpu.prof
go tool pprof cpu.prof
```

### 📦 添加新模块

1. **创建模块目录**:
   ```bash
   mkdir -p cmd/blog-agent/pkgs/newmodule
   cd cmd/blog-agent/pkgs/newmodule
   go mod init newmodule
   ```

2. **配置模块替换**:
   在 `cmd/blog-agent/go.mod` 中添加:
   ```go
   replace newmodule => ./pkgs/newmodule
   require newmodule v0.0.0
   ```

3. **实现模块接口**:
   ```go
   package newmodule
   
   import log "mylog"
   
   func Info() {
       log.InfoF(log.ModuleCommon, "info newmodule v1.0")
   }
   
   func Init() {
       log.Debug(log.ModuleCommon, "newmodule Init")
       // 初始化逻辑
   }
   ```

4. **在主程序中初始化**:
   在 `cmd/blog-agent/main.go` 中添加:
   ```go
   import "newmodule"
   // ...
   newmodule.Init()
   ```

---

## 🔒 安全机制

### 🛡️ 权限模型

系统支持 5 种基础权限类型，可组合使用：

```go
const (
    EAuthType_private     = 1    // 私有：仅自己可见
    EAuthType_public      = 2    // 公开：所有人可见
    EAuthType_encrypt     = 4    // 加密：密码保护
    EAuthType_cooperation = 8    // 协作：指定用户可见
    EAuthType_diary       = 16   // 日记：特殊密码保护
)
```

### 🔐 安全特性

- **内容加密**: AES-CBC 加密敏感内容
- **密码保护**: 日记和加密博客的密码验证
- **会话安全**: Redis 存储会话，支持过期时间
- **数据隔离**: 多用户数据物理隔离，独立目录存储
- **输入验证**: 所有用户输入验证和过滤
- **HTTPS 支持**: 完整的 TLS 支持

### ⚠️ 安全建议

1. **生产环境必做**:
   - 修改默认管理员密码
   - 启用 HTTPS
   - 配置防火墙规则
   - 定期备份数据

2. **API Key 管理**:
   - 不要提交 API Key 到版本控制
   - 使用环境变量或配置文件
   - 定期轮换密钥

3. **文件权限**:
   ```bash
   chmod 600 blogs_txt/sys_conf.md  # 配置文件
   chmod 700 blogs_txt/             # 数据目录
   chmod 755 logs/                  # 日志目录
   ```

---

## ❓ 常见问题

### 🔧 编译问题

| 问题 | 解决方案 |
|------|----------|
| **编译找不到模块** | `cd cmd/blog-agent && go mod tidy && go clean -modcache` |
| **本地模块替换错误** | 检查 `go.mod` 中的 `replace` 指令路径是否正确 |
| **Go 版本不兼容** | 确保 Go 版本 ≥ 1.24.0，使用 `go version` 检查 |

### 🚨 运行问题

| 问题 | 解决方案 |
|------|----------|
| **Redis 连接失败** | `redis-cli ping` 检查服务，确认端口配置 |
| **端口被占用** | `lsof -i :8080` 查看占用进程，修改端口配置 |
| **配置文件错误** | 检查 `blogs_txt/sys_conf.md` 格式和路径 |
| **权限不足** | `chmod +x scripts/*.sh` 给脚本执行权限 |

### 🤖 AI 功能问题

| 问题 | 解决方案 |
|------|----------|
| **AI 助手不可用** | 检查 API Key 配置，确认网络连接 |
| **模型响应慢** | 尝试切换模型，检查降级链配置 |
| **MCP 工具调用失败** | 检查工具权限配置，查看错误日志 |

### 📧 通知问题

| 问题 | 解决方案 |
|------|----------|
| **邮件发送失败** | 检查 SMTP 配置、应用专用密码、防火墙 |
| **企业微信无通知** | 验证 Webhook URL，检查企业微信应用权限 |
| **短信未收到** | 确认手机号格式、短信平台余额、发送频率 |

### 🤖 Agent 通信问题

| 问题 | 解决方案 |
|------|----------|
| **Agent 无法连接 Gateway** | 检查Gateway是否运行，确认WebSocket地址 |
| **工具调用超时** | 检查目标Agent是否在线，查看Gateway日志 |
| **消息路由失败** | 验证Agent注册状态，重启Gateway |

---

## 🤝 贡献指南

欢迎贡献代码、报告问题或提出建议！

### 📋 贡献流程

1. **Fork 仓库**并克隆到本地
2. **创建分支**: `git checkout -b feature/your-feature`
3. **提交更改**: 遵循现有代码风格和提交规范
4. **运行测试**: 确保所有测试通过
5. **推送分支**: `git push origin feature/your-feature`
6. **提交 Pull Request**: 描述变更内容和目的

### 🎯 开发规范

- **代码风格**: 遵循项目现有的命名和格式约定
- **模块设计**: 新模块需要实现 `Info()` 和 `Init()` 函数
- **错误处理**: 使用项目约定的错误码和错误处理模式
- **并发安全**: 共享资源必须使用 `sync.RWMutex` 保护
- **多账户支持**: 新功能应考虑多租户场景，使用 `WithAccount` 后缀

### 📝 提交信息规范

```
feat: 添加新功能
fix: 修复问题
docs: 文档更新
style: 代码格式调整
refactor: 代码重构
test: 测试相关
chore: 构建过程或辅助工具变动
```

---

## 📚 相关文档

- **[AGENTS.md](AGENTS.md)** - AI 编码代理开发指南，包含代码风格和构建命令
- **[BUILD_GUIDE.md](BUILD_GUIDE.md)** - 详细的编译配置和部署指南
- **[SYS_CONF_GUIDE.md](SYS_CONF_GUIDE.md)** - 完整的系统配置项说明
- **[USAGE_GUIDE.md](USAGE_GUIDE.md)** - 用户使用指南和功能说明

---

## 📞 联系方式

如有问题或建议，欢迎联系：

- **邮箱**: [guccang@gmail.com](mailto:guccang@gmail.com)
- **GitHub Issues**: [项目 Issues 页面](https://github.com/guccang/go_blog/issues)

---

## 📄 许可证

本项目采用 **MIT 许可证** - 查看 [LICENSE](LICENSE) 文件了解详情。

---

<div align="center">

**感谢使用 Go Blog！** ✨

*让技术服务于生活，让数据真正属于自己*

</div>
