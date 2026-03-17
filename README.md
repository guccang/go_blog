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
│   ├── llm-agent/     # LLM MCP 协议代理
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

## 🤖 多 Agent 系统与 LLM 核心技术

Go Blog 采用 **UAP (Unified Agent Protocol) + 多 Agent** 架构，通过 Gateway 实现 Agent 间的工具发现、消息路由和跨 Agent 工具调用。llm-agent 作为系统大脑，集成了多项 LLM 工程化技术。

### 系统架构

```
                        企业微信 / Web UI
                              │
                    ┌─────────▼──────────┐
                    │      Gateway       │  WebSocket + HTTP
                    │  (UAP 消息路由中枢)  │  Agent 注册/发现/心跳
                    └─────────┬──────────┘
          ┌───────────┬───────┼───────┬────────────┐
          │           │       │       │            │
   ┌──────▼───┐ ┌────▼────┐ ┌▼────┐ ┌▼─────────┐ ┌▼──────────┐
   │ LLM-MCP  │ │ Codegen │ │Blog │ │  Deploy   │ │Execute-Code│
   │  Agent   │ │  Agent  │ │Agent│ │  Agent    │ │   Agent    │
   │ (AI大脑) │ │(编码引擎)│ │(数据)│ │(部署引擎) │ │ (代码沙箱) │
   └──────────┘ └─────────┘ └─────┘ └──────────┘ └───────────┘
```

### 🧠 LLM 渐进式披露 (Progressive Disclosure)

当系统中 Agent 和工具数量增长时，将全部工具一次性注入 LLM 上下文会浪费 token 并降低准确率。渐进式披露通过**两级路由**按需筛选：

```
用户问题
  │
  ▼
Level 1: Agent 路由 (工具数 > 15 时触发)
  │  LLM 从 Agent 目录中选择相关 Agent
  │  execute-code-agent + 文件工具 agent 始终保留
  ▼
Level 2: Tool 路由 (工具数 > 10 时触发)
  │  LLM 从工具目录中选择相关工具
  │  ExecuteCode + ReadFile/WriteFile/ExecBash 始终保留
  ▼
最终工具列表 → LLM Function Calling
```

- Level 1 只传 agent 摘要（name + description + tool names），不传参数 schema
- Level 2 只传工具摘要（name + description），不传参数 schema
- 两级路由各自独立调用 LLM，均有降级兜底（LLM 失败时返回全部）
- 基础能力（代码执行、文件操作）不参与筛选，始终可用

### ⚡ Execute-Code-with-MCP

代码执行作为**元工具**始终注入 LLM 上下文，不受路由筛选影响。LLM 可以在任何对话中直接调用代码执行来完成计算、数据处理等任务。

```
llm-agent                    Execute-Code-Agent
     │                                  │
     │── tool_call(ExecuteCode) ───────▶│
     │                                  │── 沙箱执行代码
     │◀── tool_result(stdout/stderr) ───│
     │                                  │
```

同时，每个 Agent 通过 **FileToolKit** 暴露文件操作能力：

| 工具 | 说明 |
|------|------|
| `{Prefix}ReadFile` | 读取项目文件（路径安全校验，防目录穿越） |
| `{Prefix}WriteFile` | 写入文件（自动创建父目录） |
| `{Prefix}ExecBash` | 在项目目录执行命令（超时保护，上限 300s） |

文件工具与 ExecuteCode 一样作为基础能力始终保留，确保 LLM 在任何场景下都能读写文件和执行命令。

### 📝 提示词工程 (Prompt Engineering)

系统中有多个精心设计的提示词层：

**1. 系统提示词** — 动态构建，注入当前上下文：
- 可用 Agent 能力描述（从 Gateway 实时发现）
- 当前日期时间
- 用户账户信息
- 工具使用规范

**2. 路由提示词** — Agent 路由和工具路由各有专用提示词：
- 强调"宁多勿少"的选择策略
- 要求返回纯 JSON 数组，便于解析
- 包含示例输出格式

**3. 任务规划提示词** — 指导 LLM 进行任务拆解：
- 提供完整工具目录（含参数 hint）
- 强调子任务间的依赖关系
- 区分同步工具和异步工具（sync-wait 标记）

**4. 失败决策提示词** — 子任务失败时的 LLM 决策：
- 注入失败上下文（错误信息 + 已完成的兄弟任务结果）
- 四种决策：retry / skip / abort / modify

### 🔄 上下文工程 (Context Engineering)

**消息历史管理**：
- 会话消息超过 30 条时自动压缩：保留 system prompt + 最近消息
- 子任务执行时注入兄弟任务结果作为上下文
- 工具调用记录完整保留（参数、结果、耗时）

**会话持久化**：
- 根会话（Root Session）：主任务入口
- 子会话（Child Session）：每个子任务独立会话
- 文件级持久化（JSON），支持任务恢复和续跑

**微信对话管理**：
- 按用户维度的独立会话，超时自动清理（默认 30 分钟）
- 单会话最大消息数 40，最大轮次 15
- 会话隔离，不同用户互不干扰

### 🔀 任务拆解与任务循环 (Task Decomposition & Loop)

llm-agent 支持两条执行路径：

**简单路径** — 直接 Function Calling 循环：
```
用户问题 → LLM → tool_call → 执行 → LLM → ... → 最终回答
                    (最多 15 轮迭代)
```

**复杂路径** — Plan-and-Execute（LLM 主动触发）：
```
用户问题 → LLM 调用 plan_and_execute 虚拟工具
  │
  ▼
Planner: LLM 生成结构化任务计划
  │  每个子任务: id, title, description, dependencies, tools_hint
  │  执行模式: sequential / parallel / DAG
  ▼
Plan Review: LLM 审核计划（参数正确性、依赖逻辑）
  │
  ▼
Orchestrator: DAG 拓扑排序执行
  │  每个子任务独立 agentic loop（最多 10 轮）
  │  异步操作检测（tool result 中的 status 字段）
  │  失败处理: LLM 决策 retry/skip/abort/modify
  ▼
Synthesizer: LLM 聚合所有子任务结果，生成统一回答
```

关键设计：
- `plan_and_execute` 是虚拟工具，由 LLM 自主判断是否需要任务拆解
- 子任务支持 DAG 依赖，非阻塞的子任务可并行执行
- 每个子任务有独立的 agentic loop，可多轮调用工具
- 任务失败不直接终止，而是由 LLM 分析上下文后决策下一步
- 全程会话持久化，支持断点续跑

### 🛠️ Agent 技能一览

| Agent | 工具 | 能力 |
|-------|------|------|
| **Blog Agent** | `blog.*` `exercise.*` `reading.*` `finance.*` `todolist.*` 等 | 博客 CRUD、锻炼记录、读书管理、财务统计、待办事项 |
| **Codegen Agent** | `CodegenStartSession` `CodegenSendMessage` `CodegenReadFile` `CodegenWriteFile` `CodegenExecBash` | 集成 Claude Code / OpenCode，管理编码会话，文件读写，命令执行 |
| **Deploy Agent** | `DeployProject` `DeployPipeline` `DeployGetStatus` `DeployReadFile` `DeployWriteFile` `DeployExecBash` | 多目标部署（本地/SSH）、交叉编译、流水线、SFTP 上传 |
| **Execute-Code Agent** | `ExecuteCode` | 代码沙箱执行，支持多语言，超时保护 |
| **WeChat Agent** | 消息接收/转发 | 企业微信接入，消息路由到 llm-agent |

### 微信指令（cg 系列）

通过企业微信发送 `cg` 前缀指令，直接操控编码和部署流程：

```
cg list                          — 列出所有编码项目
cg create <名称[@agent]>         — 创建项目
cg start <项目> <需求>           — 启动编码会话
cg start <项目> #<模型> <需求>   — 指定模型编码
cg start <项目> !deploy <需求>   — 编码后自动部署
cg deploy <项目>                 — 仅部署
cg agents                        — 查看在线 Agent
cg models                        — 查看可用模型
```

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

# 2. 启动llm-agent (AI大脑)
cd cmd/llm-agent
go build -o llm-agent main.go
./llm-agent &

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
