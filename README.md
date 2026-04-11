# Go Blog Agent Platform

一个以 `UAP Gateway + 多 Agent` 为核心的消息与任务编排平台。当前仓库的重点已经不再是传统博客介绍，而是：

- Flutter App 如何接入 agent 网络
- 企业微信如何接入 agent 网络
- `llm-agent` / `metagpt-agent` 如何作为智能中枢调用其他 agent
- 各 agent 如何通过统一协议协同完成工具调用、任务分派、通知回推

## 项目定位

这个仓库现在可以理解成一个面向多终端的 Agent 平台：

- `app-agent` 负责承接 Flutter / App 侧消息、附件、会话与回推
- `wechat-agent` 负责承接企业微信消息、回调、通知推送
- `llm-agent` 负责通用 LLM 编排、工具调用、任务执行
- `metagpt-agent` 提供一个基于 MetaGPT 的兼容实现，可替代 `llm-agent` 的核心任务流
- `gateway` 负责 agent 注册、发现、消息路由、健康检查和 trace 观测
- `blog-agent` 继续作为业务数据与站点服务节点存在，但不是 README 的重点

一句话概括：

> Flutter、企业微信、Web 都不是直接连具体工具，而是先连各自的接入 agent，再通过 Gateway 接入统一的智能体网络。

## 核心能力

### 1. 多终端统一接入

- Flutter / App 通过 `app-agent` 接入
- 企业微信通过 `wechat-agent` 接入
- Web / HTTP 任务通过 `blog-agent` 或其它服务接入
- 所有入口最终都走 UAP 消息协议，进入同一套 agent 网络

### 2. 智能体任务编排

- `llm-agent` 支持 `assistant_chat`
- `llm-agent` 支持 `llm_request`
- `llm-agent` 支持 `cron_query / cron_reminder / resume_task`
- `llm-agent` 能从 Gateway 动态发现在线 tools / agents
- `llm-agent` 能通过 `tool_call -> tool_result` 跨 agent 调用工具

### 3. Flutter 消息能力

`app-agent` 提供完整的 App 接入层能力：

- HTTP 登录接口：`/api/app/login`
- App 消息入口：`/api/app/message`
- WebSocket 推送入口：`/ws/app`
- 支持文本、图片、音频、文件等消息类型
- 支持附件落盘、附件描述、语音转写、图片识别预处理
- 支持 agent 执行过程中的进度消息、最终回复、富消息回推

Flutter 侧不需要直接理解具体工具，只需要和 `app-agent` 对接：

- 发送用户消息给 `app-agent`
- 接收 `app-agent` 回推的普通文本或富消息
- 由 `app-agent` 将消息路由到 `llm-agent`、`codegen-agent`、`deploy-agent` 等后端 agent

### 4. 企业微信消息能力

`wechat-agent` 提供完整的企业微信接入层能力：

- 接收企业微信回调消息
- 将微信用户消息转发到 `llm-agent` 或命令类 agent
- 通过 `wechat.SendMessage` / `wechat.SendMarkdown` 作为远程工具被调用
- 将任务结果、进度、通知重新推送回企业微信用户或群

这意味着微信端既可以作为：

- 自然语言对话入口
- 命令入口
- 任务执行结果接收端
- 定时提醒与自动推送接收端

### 5. MetaGPT 兼容智能中枢

仓库中已提供 `cmd/metagpt-agent/`，用于基于 MetaGPT 框架实现 `llm-agent` 的核心能力兼容：

- 接收 `assistant_chat / llm_request / cron_query / cron_reminder / resume_task`
- 接收 `wechat / app` 的 `notify`
- 从 Gateway 拉取在线工具目录
- 执行工具调用循环
- 持久化 session、runtime snapshot、trace

适用场景：

- 想保留现有 UAP 协议与 agent 网络
- 但希望把核心任务执行器替换成 MetaGPT Role/Action 模式

## 关键消息流

### Flutter -> agent 网络

```text
Flutter App
  -> app-agent (/api/app/message, /ws/app)
  -> Gateway
  -> llm-agent / metagpt-agent
  -> 目标工具 agent（blog-agent / deploy-agent / audio-agent / image-agent ...）
  -> llm-agent / metagpt-agent 汇总结果
  -> app-agent
  -> Flutter App
```

典型能力：

- 文本提问
- 语音提问后自动转写
- 图片提问后自动识别
- 执行过程中的进度推送
- 富消息和附件回推

### 企业微信 -> agent 网络

```text
企业微信用户
  -> wechat-agent (callback)
  -> Gateway
  -> llm-agent / codegen-agent / 其它命令 agent
  -> 目标工具 agent
  -> 结果回到 wechat-agent
  -> 企业微信用户
```

典型能力：

- 微信里直接问业务问题
- 微信里触发编码 / 部署 / 查询 / 定时提醒
- 接收实时进度和最终结果

### 定时任务 -> 微信 / App 双端推送

```text
cron-agent
  -> llm-agent / metagpt-agent
  -> 生成提醒或查询结果
  -> wechat-agent + app-agent
  -> 企业微信用户 / Flutter 用户
```

## UAP 协议能力

所有 agent 通过 UAP 互通。当前最关键的消息类型有：

| 消息类型 | 作用 |
| --- | --- |
| `register` | agent 向 Gateway 注册 |
| `heartbeat` | agent 保活 |
| `notify` | 单向消息通知，常用于微信 / App 回推 |
| `tool_call` | 调用另一个 agent 暴露的工具 |
| `tool_result` | 工具执行结果 |
| `task_assign` | 派发任务 |
| `task_accepted` | 任务接收确认 |
| `task_event` | 任务执行过程中的流式进度 |
| `task_complete` | 任务完成结果 |

对 Flutter / 微信特别重要的点：

- 前端不需要直接面对工具目录
- 前端只和接入 agent 通信
- 真正的工具选择、任务编排、执行过程都在 agent 网络内部完成

## Agent 清单

| Agent | 位置 | 主要职责 |
| --- | --- | --- |
| `gateway` | `cmd/gateway/` | 注册、发现、路由、trace、health |
| `llm-agent` | `cmd/llm-agent/` | 通用 LLM 编排中枢 |
| `metagpt-agent` | `cmd/metagpt-agent/` | MetaGPT 版任务执行中枢 |
| `app-agent` | `cmd/app-agent/` | Flutter / App 接入、消息桥接、附件与回推 |
| `wechat-agent` | `cmd/wechat-agent/` | 企业微信接入、通知发送、微信工具 |
| `blog-agent` | `cmd/blog-agent/` | 业务数据、内容服务、主站能力 |
| `deploy-agent` | `cmd/deploy-agent/` | 构建与部署 |
| `codegen-agent` | `cmd/codegen-agent/` | 代码生成与编码会话 |
| `execute-code-agent` | `cmd/execute-code-agent/` | 沙箱执行 |
| `audio-agent` | `cmd/audio-agent/` | 语音相关能力 |
| `image-agent` | `cmd/image-agent/` | 图像相关能力 |
| `cron-agent` | `cmd/cron-agent/` | 定时任务 |
| `log-agent` | `cmd/log-agent/` | 日志采集与分析 |
| `mcp-agent` | `cmd/mcp-agent/` | MCP 工具接入 |
| `test-agent` | `cmd/test-agent/` | 全链路测试与评估 |

## Flutter 接入重点

如果你的目标是做 Flutter + Agent 通信，建议重点关注这些目录：

- [cmd/app-agent](cmd/app-agent/)
- [cmd/llm-agent](cmd/llm-agent/)
- [cmd/gateway](cmd/gateway/)

Flutter 侧建议的接入方式：

1. 用户先通过 `app-agent` 登录接口获取身份和会话能力
2. 文本或附件消息发到 `app-agent`
3. 长连接通过 `ws/app` 接收进度和回推
4. 由 `app-agent` 将消息包装成 `notify(channel="app")` 发往目标 agent
5. `llm-agent` 在内部完成工具调用，再把结果回推给 `app-agent`
6. `app-agent` 再将结果投递回 Flutter 客户端

这样做的好处：

- Flutter 客户端不需要内置复杂工具协议
- Flutter 客户端不需要直连多种后端 agent
- 终端协议收敛，便于后续扩展到 Web、桌面、其它 IM

## 企业微信接入重点

如果你的目标是做微信 + Agent 通信，建议重点关注这些目录：

- [cmd/wechat-agent](cmd/wechat-agent/)
- [cmd/llm-agent](cmd/llm-agent/)
- [docs/wechat_conversation_context.md](docs/wechat_conversation_context.md)
- [docs/message_flow.md](docs/message_flow.md)

企业微信侧当前支持的核心模式：

- 微信消息作为自然语言入口
- 微信消息作为命令入口
- 微信侧接收任务进度
- 微信侧接收定时提醒与查询结果
- `wechat-agent` 作为工具被其它 agent 反向调用

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

### 3. 启动 app-agent

```bash
cd cmd/app-agent
go build
./app-agent -config app-agent.json
```

### 4. 启动 wechat-agent

```bash
cd cmd/wechat-agent
go build
./wechat-agent -config wechat-agent.json
```

### 5. 启动 metagpt-agent

```bash
cd cmd/metagpt-agent
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
python main.py --config metagpt-agent.json
```

## 推荐阅读顺序

- [docs/multi_agent_architecture.md](docs/multi_agent_architecture.md)
- [docs/message_flow.md](docs/message_flow.md)
- [docs/wechat_conversation_context.md](docs/wechat_conversation_context.md)
- [cmd/metagpt-agent/README.md](cmd/metagpt-agent/README.md)
- [cmd/test-agent/README.md](cmd/test-agent/README.md)

## 当前 README 的取舍

这个 README 故意弱化了原先大篇幅的博客业务介绍，改为突出：

- agent 网络能力
- Flutter 接入能力
- 企业微信接入能力
- `llm-agent` / `metagpt-agent` 的智能中枢角色

如果你关注的是博客业务本身，可以再看：

- [cmd/blog-agent](cmd/blog-agent/)
- [cmd/blog-agent/README.md](cmd/blog-agent/README.md)
