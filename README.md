# Go Blog - 个人数字生活管理系统

一个功能丰富的 Go 语言博客系统，采用创新的"一切皆博客"设计理念，将传统博客功能与个人生产力工具深度整合。

## 📚 相关文档

- [编译配置说明](BUILD_GUIDE.md)
- [配置说明](SYS_CONF_GUIDE.md)

## 🏗️ 系统架构

### 核心设计理念

1. **统一数据模型**: 所有功能数据都以博客格式存储在 `blogs_txt/` 目录
2. **模块化架构**: 30+独立功能模块，每个模块都有独立的 `go.mod` 文件
3. **双重存储**: 文件系统作为主存储 + Redis 作为缓存层
4. **无数据库依赖**: 纯文件存储，便于迁移和备份

### 技术栈

- **后端**: Go 1.24.0，模块化设计
- **前端**: HTML/CSS/JavaScript，响应式设计
- **存储**: Markdown 文件 + Redis 缓存
- **加密**: AES-CBC 算法
- **模板引擎**: Go template 系统
- **AI 集成**: 多模型支持 (DeepSeek / OpenAI / Qwen) + MCP 协议 + 可插拔技能系统
- **定时调度**: robfig/cron 库，支持 Cron 表达式 + 博客持久化

## 📦 功能模块

### 核心系统模块

| 模块 | 功能 |
|------|------|
| `blog` | 博客系统，支持 Markdown 编写、实时预览、权限控制 |
| `comment` | 评论系统 |
| `auth` | 认证系统 |
| `login` | 登录功能 |
| `account` | 账户管理 |
| `config` | 配置管理 |
| `persistence` | 持久化层 |
| `mylog` | 日志系统 |
| `ioutils` | I/O 工具 |

### 生产力工具

| 模块 | 功能 |
|------|------|
| `todolist` | 待办事项管理，支持时间预估、拖拽排序 |
| `taskbreakdown` | 任务拆解，支持复杂任务的分层管理 |
| `exercise` | 锻炼记录，支持 4 种类型（有氧、力量、柔韧、运动），智能卡路里计算 |
| `reading` | 读书管理，书籍管理、进度追踪、读书笔记 |
| `yearplan` | 年度计划，目标设置、月度分解、进度追踪 |
| `lifecountdown` | 人生倒计时，重要日期提醒 |
| `finance` | 财务管理 |

### AI 与智能化

| 模块 | 功能 |
|------|------|
| `llm` | LLM 集成，多模型支持 (DeepSeek/OpenAI/Qwen)，流式响应，动态切换 |
| `mcp` | MCP 协议支持，文件访问、Redis 访问、自定义 MCP 服务器，网页抓取 (GBK/UTF-8 自动编码) |
| `agent` | 智能任务编排，任务拆解/自动执行，Cron 定时任务，自动报告生成，WebSocket 实时通知 |
| `skill` | 可插拔 AI 技能系统，用户创建技能卡扩展 AI 行为 |

### 娱乐与游戏

| 模块 | 功能 |
|------|------|
| `gomoku` | 五子棋 |
| `linkup` | 连连看，支持 AI、PVP、竞速模式 |
| `tetris` | 俄罗斯方块 |
| `minesweeper` | 扫雷 |
| `fruitcrush` | 水果消消乐 |

### 辅助功能

| 模块 | 功能 |
|------|------|
| `search` | 全文搜索 |
| `share` | 分享功能 |
| `statistics` | 统计分析 |
| `sms` | 短信服务 |
| `email` | 邮件服务 |
| `encryption` | 加密服务 |
| `constellation` | 星座相关功能 |
| `tools` | 工具集 |

### Web 模块

| 模块 | 功能 |
|------|------|
| `http` | HTTP 服务，路由处理 |
| `view` | 视图层 |
| `control` | 控制层 |
| `module` | 模块定义 |

## 💻 核心页面展示

### 1. 主控台 (`/main`)
系统的核心仪表盘，集成了所有功能的入口。
- **快捷导航**: 顶部导航栏快速访问所有子模块
- **全局搜索**: 支持标签搜索和命令搜索（如 `@tag match`）
- **最近访问**: 自动记录并展示最近访问的博客和功能
- **热门标签**: 展示高频使用的标签，支持点击筛选

### 2. 智能 Agent (`/agent`)
强大的 AI 任务执行中心。
- **任务编排**: 支持复杂的任务拆解和执行
- **自动化流程**: 能够自动执行文件操作任务
- **可视化进度**: 实时展示任务执行状态和结果

### 3. AI 助手 (`/assistant`)
基于大模型的智能对话界面。
- **实时交互**: 与 AI 进行自然语言对话，支持流式响应
- **上下文记忆**: 跨会话连续性，LLM 压缩上下文，进度保存/加载
- **工具调用**: MCP 工具集成，支持工具选择和结果展示
- **WebSocket 通知**: 实时接收定时任务和报告推送
- **多模型切换**: 支持在 DeepSeek/OpenAI/Qwen 之间动态切换

### 4. 可插拔 AI 技能系统
用户可创建“AI 技能卡”扩展 AI 行为，无需改代码。
- **动态加载**: 技能卡保存为博客，启动时自动加载
- **触发匹配**: 根据用户输入关键词自动激活对应技能
- **System Prompt 注入**: 活跃技能自动注入系统提示词

### 5. 定时任务自动化
基于 `robfig/cron` 库的智能定时调度系统。
- **Cron 表达式**: 支持秒级精度的 Cron 表达式（如 `0 0 21 * * *` 每天21:00）
- **博客持久化**: 所有提醒保存到博客，重启自动恢复
- **AI 定时任务**: 定时执行 AI 查询并推送结果（如“每周一分析运动数据”）
- **自动报告**: 日报/周报/月报自动生成，保存为博客并推送通知

## 🔐 安全机制

### 权限控制

```go
const (
    EAuthType_private = 1       // 私有
    EAuthType_public  = 2       // 公开
    EAuthType_encrypt = 4       // 加密
    EAuthType_cooperation = 8   // 协作
    EAuthType_diary   = 16      // 日记博客，需要密码保护
    EAuthType_all     = 0xffff  // 所有权限
)
```

### 加密存储
- 基于 AES-CBC 算法的内容加密
- 支持组合权限设置
- 密码保护的分享功能

## 🚀 快速开始

### 环境要求
- Go 1.24.0+
- Redis

### 安装步骤

```bash
# 克隆仓库
git clone https://github.com/guccang/go_blog.git
cd go_blog

# 构建项目
./scripts/build.sh

# 启动 Redis
./scripts/start_redis.sh

# 启动服务
./scripts/start.sh blogs_txt/sys_conf.md
```

### 手动构建

```bash
# 清理构建
rm -f go_blog

# 整理依赖并构建
go mod tidy
go build

# 运行
./go_blog blogs_txt/sys_conf.md
```

## 🌟 技术亮点

### 1. 创新的数据存储方式
所有功能数据都转换为博客格式存储，例如：
- 锻炼记录 → `exercise-2024-01-15` 博客
- 任务列表 → `todolist-2024-01-15` 博客
- 这种方式使数据高度统一且易于管理

### 2. 高度模块化设计
每个功能包都是独立模块，支持独立开发和测试：
```go
replace module => ./pkgs/module
replace control => ./pkgs/control
replace view => ./pkgs/view
// ... 30+ modules
```

### 3. AI 深度集成
- 多模型支持: DeepSeek / OpenAI / Qwen，运行时动态切换
- MCP 协议: 访问本地文件系统、Redis、博客数据
- 可插拔 AI 技能: 用户创建技能卡扩展 AI 行为，无需改代码
- 会话记忆: 跨上下文窗口连续性，LLM 压缩 + 进度保存
- Cron 定时任务: 持久化定时任务，支持 AI 定时查询
- 自动报告: 日报/周报/月报自动生成并推送
- WebSocket 实时通知: 跨页面提醒推送

### 4. 丰富的前端交互
- 多视图切换
- 实时数据计算
- 拖拽操作支持
- 响应式设计

## 💡 项目特色

### 优势
1. **数据自主性**: 所有数据都是文本文件，用户完全掌控
2. **功能完整性**: 集博客、任务、健身、读书、娱乐于一体
3. **高度可定制**: 模板系统、主题切换、快捷键支持
4. **部署简单**: 单一可执行文件，支持 HTTPS
5. **扩展性强**: 模块化设计便于添加新功能
6. **AI 赋能**: 多模型支持，可插拔技能系统，Cron 定时任务，自动报告，实时通知

### 适用场景
- 个人博客网站
- 生产力工具集成平台
- 私人数据管理系统
- 团队协作平台
- AI 辅助的个人知识库

## 📊 项目结构

```
go_blog/
├── main.go                 # 应用入口
├── go.mod                  # 主模块定义
├── blogs_txt/              # 数据存储目录
│   ├── user1/              # 用户数据目录
│   └── sys_conf.md         # 系统配置
├── pkgs/                   # 30+ 独立 Go 模块
│   ├── core/               # 核心模块
│   ├── module/             # 模块定义
│   ├── blog/               # 博客功能
│   ├── agent/              # AI Agent
│   ├── llm/                # LLM 集成
│   ├── mcp/                # MCP 协议
│   ├── exercise/           # 锻炼管理
│   ├── todolist/           # 任务管理
│   ├── reading/            # 读书管理
│   └── ... 更多模块
├── scripts/                # 构建和部署脚本
├── templates/              # HTML 模板
├── statics/                # 静态资源
└── redis/                  # Redis 配置
```

## 📝 许可证

[待添加]

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📧 联系方式

[EMAIL_ADDRESS](guccang@gmail.com)
