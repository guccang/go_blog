# hermes-agent

`hermes-agent` 将原版 Nous Research Hermes `AIAgent` 接入 go_blog 的 UAP
网络，使 Flutter App 可以通过现有 `app-agent` 自动执行 Hermes 任务。

核心执行逻辑直接使用本机 Hermes 源码，不维护一份容易落后的分叉：

- Hermes 提供模型调用、工具循环、技能、记忆、上下文压缩和子 Agent。
- 本目录提供 UAP 注册/重连、Flutter 消息适配、并发队列、会话续接和结果回推。
- 支持 `notify(channel=app)`、`task_assign(assistant_chat/llm_request)`、
  `task_event` 和 `task_complete`。

## 启动

先确保 Hermes 已安装在 `~/.hermes/hermes-agent`，并完成模型配置：

```bash
cd cmd/hermes-agent
cp hermes-agent.json.example hermes-agent.json
./hermes-agent --config hermes-agent.json
```

启动脚本会使用 Hermes 自带的 Python 虚拟环境，并强制启用 UTF-8。
也可通过 `HERMES_AGENT_SOURCE` 指定另一份 Hermes 主源码。

让 Flutter 消息进入 Hermes 时，把 `app-agent` 配置中的 `llm_agent_id`
改为 `hermes-agent`。Flutter 仍使用原有接口：

- `POST /api/app/message`
- `GET /ws/app`

## 配置说明

- `workspace_dir`：Hermes 执行任务的工作目录。
- `session_dir`：按 Flutter 用户或 UAP task 保存的对话历史。
- `model/provider/base_url/api_key`：为空时沿用 Hermes 自身配置。
- `enabled_toolsets/disabled_toolsets`：限制 Hermes 本轮可使用的工具集。
- `max_concurrent/task_queue_size`：并发任务数和等待队列容量。

## 验证

```bash
./hermes-agent --genconf --config /tmp/hermes-agent.json
~/.hermes/hermes-agent/venv/bin/python -m unittest discover -s tests -v
```
