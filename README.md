# Go Blog Agent Platform

一个面向实际使用场景的多 Agent 平台。

这个仓库的重点不是单一博客系统，而是把 `Flutter`、企业微信、Web 后台、`llm-agent`、`metagpt-agent`、`deploy-agent` 这些能力接到同一套 Agent 网络里，让用户可以直接通过自然语言处理内容、任务和部署流程。

当前最适合把它理解成：

- 一个支持企业微信和 Flutter 客户端接入的 Agent 工作台
- 一个支持多 Agent 协同编排的任务执行平台
- 一个支持直接部署和 Pipeline 部署的工程交付平台

## 用户能拿它做什么

从用户视角看，这个平台适合做这些简单实用的事情：

- 用企业微信管理日记、博客、待办、提醒、阅读记录、运动记录
- 用 Flutter 客户端承接个人或团队的消息、附件、任务进度和结果回推
- 让 `llm-agent` 或 `metagpt-agent` 自动选择工具，帮你查询、整理、执行、汇总
- 让 `deploy-agent` 直接部署项目，或者按 Pipeline 跑完整交付流程

一句话概括：

> 用户面对的是微信或 Flutter，对话背后由 Gateway 把请求路由给多个专业 Agent 协同完成。

## 典型使用案例

### 1. 日记博客管理

适合个人记录、家庭日志、小团队内容沉淀。

用户可以直接在企业微信或 App 里发消息：

- 今天写一篇关于最近工作的日记
- 帮我整理这周博客草稿
- 把这条想法追加到某篇文章后面
- 查询我上个月写了多少篇日记

平台背后会由 `blog-agent + llm-agent` 处理内容存储、检索、总结和组织。

### 2. 锻炼管理

适合记录每日运动、回顾训练情况、做阶段性总结。

典型输入：

- 记录今天跑步 5 公里
- 查询这周运动记录
- 帮我总结最近一个月的锻炼频率
- 提醒我晚上 8 点锻炼

这类任务可以由 `blog-agent` 的业务能力配合 `llm-agent` 的对话编排完成，微信场景尤其顺手。

### 3. 阅读管理

适合管理阅读计划、读书进度、阅读统计和总结。

典型输入：

- 记录今天读了 40 页
- 查询本月阅读进度
- 帮我整理最近读过的书和笔记
- 按阅读时长给我做个简单总结

适合把阅读数据长期沉淀下来，再通过 Agent 做周期性分析。

### 4. 项目管理

适合个人项目、小团队研发任务、日常推进和复盘。

典型输入：

- 帮我整理这个项目当前待办
- 记录今天完成了哪些任务
- 根据聊天内容生成一个本周计划
- 把部署结果发回微信群或企业微信

这里可以把 `wechat-agent`、`app-agent`、`llm-agent`、`deploy-agent` 串起来，形成从任务提出到结果回推的闭环。

### 5. Flutter 客户端的组管理项目

如果你要做一个 Flutter 端的小组管理、团队协作、项目管理类应用，这套仓库比较适合直接复用。

Flutter 侧可以只关心这些事情：

- 用户登录
- 发送文本、图片、文件、语音
- 展示对话消息、进度消息、最终结果
- 展示附件和任务执行状态

复杂的事情放在 Agent 网络里处理：

- `app-agent` 负责 Flutter 消息接入和回推
- `llm-agent` 或 `metagpt-agent` 负责理解任务、选择工具、调度执行
- `wechat-agent` 可以作为补充通知通道
- `deploy-agent` 可以负责发布后端或相关服务

## 平台核心能力

### 1. 多终端统一接入

- Flutter / App 通过 `app-agent` 接入
- 企业微信通过 `wechat-agent` 接入
- Web / HTTP 服务通过 `blog-agent` 或其他业务 Agent 接入
- 所有入口最终都通过 Gateway 进入统一 Agent 网络

### 2. 智能中枢编排

- `llm-agent` 负责通用任务理解、工具选择、工具调用循环
- `metagpt-agent` 提供基于 MetaGPT 的兼容实现
- 两者都可以作为系统的智能执行中枢

### 3. 实时进度和结果回推

- 微信可以收到任务进度和最终结果
- Flutter 可以通过 WebSocket 接收流式回推
- Agent 间执行过程可以被 Gateway 跟踪和审计

### 4. 工具与 Agent 动态发现

- Gateway 维护在线 Agent 列表
- `llm-agent` 动态获取可用工具和部署目标
- 新 Agent 上线后可以自动进入整体网络协作

## 消息接入视角

### Flutter / App

Flutter 客户端不需要直接理解工具协议，只需要接 `app-agent`：

- 登录：`/api/app/login`
- 发送消息：`/api/app/message`
- 实时回推：`/ws/app`

典型消息流：

```text
Flutter App
  -> app-agent
  -> Gateway
  -> llm-agent / metagpt-agent
  -> blog-agent / deploy-agent / 其它工具 agent
  -> app-agent
  -> Flutter App
```

这很适合做：

- 团队工作台
- 个人效率助手
- 小组管理项目
- 任务协同和结果通知客户端

### 企业微信

企业微信更适合日常高频使用，尤其是：

- 查记录
- 记待办
- 发提醒
- 触发部署
- 接收部署结果和执行进度

典型消息流：

```text
企业微信用户
  -> wechat-agent
  -> Gateway
  -> llm-agent / metagpt-agent
  -> blog-agent / deploy-agent / 其它 agent
  -> wechat-agent
  -> 企业微信用户
```

## deploy-agent：部署能力是 README 的重点之一

`deploy-agent` 不只是“上传后执行脚本”，而是把部署做成 Agent 能力，支持两种主要模式。

### 1. 直接部署

适合明确知道项目和目标环境的场景。

例如：

- 把 `blog-agent` 直接部署到 `ssh-prod`
- 把某个服务只做打包，不执行远端发布
- 指定某个 deploy target 做一次临时部署

对应能力包括：

- `DeployProject`：按已配置项目执行部署
- `DeployAdhoc`：对临时目录或临时项目做一次性部署

适用场景：

- 单项目快速发版
- 修复后马上推到测试或生产环境
- 明确知道部署目标，不需要复杂编排

### 2. Pipeline 部署

适合完整交付流程，尤其是多步骤、可复用、可标准化的场景。

例如一个 Pipeline 可以包含：

1. 拉取或准备构建产物
2. 编译和打包
3. 上传到目标机器
4. 执行发布脚本
5. 执行部署后验证
6. 发送结果通知到微信或 App

对应能力：

- `DeployPipeline`

适用场景：

- 标准化发版
- 多环境一致部署
- 团队成员复用同一套交付流程
- 希望把“构建、部署、验证、通知”做成固定流水线

### 3. 什么时候用直接部署，什么时候用 Pipeline

直接部署适合：

- 我就想把某个项目发到 `ssh-prod`
- 我已经知道项目名和目标环境
- 我需要快，不需要一整套流水线

Pipeline 适合：

- 我希望一键执行完整交付流程
- 团队里多人都复用同一套部署标准
- 我希望后续排查问题时，能看到标准步骤和执行路径

### 4. 用户侧使用方式

用户不一定要自己调用底层工具名，通常可以直接通过自然语言或者微信命令触发。

示例：

```text
帮我把 blog-agent 部署到 ssh-prod
```

```text
执行 prod-all pipeline
```

企业微信命令侧也支持直接操作：

```text
cg deploy blog-agent #ssh-prod
cg pipeline list
cg pipeline prod-all
```

## 主要 Agent 清单

| Agent | 位置 | 主要职责 |
| --- | --- | --- |
| `gateway` | `cmd/gateway/` | 注册、发现、路由、健康检查、Trace |
| `llm-agent` | `cmd/llm-agent/` | 通用 LLM 编排中枢 |
| `metagpt-agent` | `cmd/metagpt-agent/` | MetaGPT 版智能执行中枢 |
| `app-agent` | `cmd/app-agent/` | Flutter / App 接入、附件与消息回推 |
| `wechat-agent` | `cmd/wechat-agent/` | 企业微信接入、通知发送 |
| `blog-agent` | `cmd/blog-agent/` | 博客、待办、运动、阅读等业务能力 |
| `deploy-agent` | `cmd/deploy-agent/` | 构建、打包、部署、Pipeline |
| `codegen-agent` | `cmd/codegen-agent/` | 编码与代码会话 |
| `execute-code-agent` | `cmd/execute-code-agent/` | 沙箱执行 |
| `cron-agent` | `cmd/cron-agent/` | 定时任务 |
| `test-agent` | `cmd/test-agent/` | Agent 测试与评估 |

## 快速启动

### 1. 启动 Gateway

```bash
cd cmd/gateway
go build -o gateway
./gateway
```

### 2. 启动 llm-agent

```bash
cd cmd/llm-agent
go build
./llm-agent -config llm-agent.json
```

### 3. 启动 metagpt-agent

```bash
cd cmd/metagpt-agent
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
python main.py --config metagpt-agent.json
```

### 4. 启动 app-agent

```bash
cd cmd/app-agent
go build
./app-agent -config app-agent.json
```

### 5. 启动 wechat-agent

```bash
cd cmd/wechat-agent
go build
./wechat-agent -config wechat-agent.json
```

### 6. 启动 deploy-agent

```bash
cd cmd/deploy-agent
go build
./deploy-agent -config deploy-agent.json
```

## 推荐阅读

- [docs/multi_agent_architecture.md](docs/multi_agent_architecture.md)
- [docs/message_flow.md](docs/message_flow.md)
- [docs/wechat_conversation_context.md](docs/wechat_conversation_context.md)
- [cmd/metagpt-agent/README.md](cmd/metagpt-agent/README.md)
- [cmd/test-agent/README.md](cmd/test-agent/README.md)

## 代码入口建议

如果你关注 Flutter + Agent：

- [cmd/app-agent](cmd/app-agent/)
- [cmd/flutter-client-for-appagent](cmd/flutter-client-for-appagent/)
- [cmd/llm-agent](cmd/llm-agent/)
- [cmd/gateway](cmd/gateway/)

如果你关注企业微信 + Agent：

- [cmd/wechat-agent](cmd/wechat-agent/)
- [cmd/llm-agent](cmd/llm-agent/)
- [docs/wechat_conversation_context.md](docs/wechat_conversation_context.md)

如果你关注部署能力：

- [cmd/deploy-agent](cmd/deploy-agent/)
- [docs/message_flow.md](docs/message_flow.md)
- [docs/multi_agent_architecture.md](docs/multi_agent_architecture.md)
