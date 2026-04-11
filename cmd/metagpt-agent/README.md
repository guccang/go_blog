# metagpt-agent

`metagpt-agent` 是一个基于 MetaGPT `Role + Action` 框架实现的 UAP agent，目标是兼容现有 `llm-agent` 的核心能力：

- 接收 `assistant_chat / llm_request / cron_query / cron_reminder / resume_task`
- 接收 `wechat / app` 直连 `notify`
- 从 gateway 发现在线工具与 agent
- 通过 UAP `tool_call -> tool_result` 调用其他 agent 工具
- 持久化 transcript、runtime snapshot、trace
- 用 MetaGPT Role/Action 驱动任务执行

## 当前范围

当前实现覆盖 `llm-agent` 的核心任务主线，不覆盖这些重型特性：

- Claude Mode / ACP 权限交互
- 复杂 mailbox 子任务编排
- 音频回复与富媒体输出
- 现有 `llm-agent` 中的所有 prompt 优化细节

换句话说，它是一个可运行的 MetaGPT 版替代内核，而不是对 Go 版 `llm-agent` 的逐文件复刻。

## 目录结构

```text
cmd/metagpt-agent/
  main.py
  requirements.txt
  metagpt-agent.json.example
  metagpt_agent/
    config.py
    protocol.py
    session_store.py
    runtime.py
    role_runtime.py
    service.py
```

## 运行前提

1. Python `>=3.9,<3.12`
2. 已启动 gateway
3. 已配置 OpenAI-compatible LLM

## 安装

```bash
cd cmd/metagpt-agent
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
cp metagpt-agent.json.example metagpt-agent.json
```

## 启动

```bash
cd cmd/metagpt-agent
. .venv/bin/activate
python main.py --config metagpt-agent.json
```

## 与 llm-agent 的协议兼容点

- 任务输入仍走 `uap.MsgTaskAssign`
- 事件仍走 `uap.MsgTaskEvent`
- 完成仍走 `uap.MsgTaskComplete`
- 工具调用仍走 `uap.MsgToolCall`
- 直连聊天仍走 `uap.MsgNotify`

## MetaGPT 接法

实现上采用单角色执行器：

- `MetaGPTTaskRole(Role)`
- `ExecuteQueryLoopAction(Action)`

其中真正的工具调用循环在 `QueryExecutor` 里，`Role/Action` 负责：

- 任务接收
- 任务记忆挂接
- 与 MetaGPT `run()` 生命周期对齐
- 统一的角色前缀与约束注入
