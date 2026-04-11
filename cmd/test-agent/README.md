# test-agent

`test-agent` 用来模拟用户触发任务，并把每次测试执行的完整路径持续写入磁盘，方便确认系统是否按预期链路执行。

现在它默认按完整评估流水线运行：

1. 发现当前在线 agents 和 Gateway 健康状态
2. 加载静态评估集合（配置文件固定，便于横向对比）
3. 调用 `llm-agent` 生成动态评估集合（依赖在线 agent 接口，每次可能不同）
4. 执行静态集与动态集
5. 输出多维度最终报告和完整落盘数据

## 能力

- 发送 `notify` / `task_assign` / `tool_call` 三类入口消息
- 轮询 Gateway trace，收集 agent 路径、消息类型、摘要和耗时
- 尝试匹配 `llm-agent` 的 `trace_*.json`
- 自动发现在线 agents，先执行静态评估集，再执行动态评估集
- 动态评估集通过 `llm-agent` 生成，并在本地做在线性/风险校验
- 输出 completion / routing / tool_usage / recovery / final_answer 五个维度的聚合评分
- 输出每个在线 agent 的参与度、通过率、平均分
- 每个场景执行中持续写入：
  - `scenario.json`
  - `run.json`
  - `timeline.json`
  - `messages.json`
  - `result.json`
  - `gateway_trace.json`
  - `llm_trace.json`（若匹配到）
  - `trace_summary.md`
- 评估根目录持续写入：
  - `gateway_health.json`
  - `online_agents.json`
  - `execution_plan.json`
  - `static_report.json` / `static_report.md`
  - `dynamic_report.json` / `dynamic_report.md`
  - `final_report.json` / `final_report.md`

## 使用

1. 生成配置：

```bash
cd cmd/test-agent && go run . -genconf
```

2. 编写静态 suite 文件，例如 `suites/system-smoke.json`

3. 默认运行完整评估：

```bash
cd cmd/test-agent && go run . -config test-agent.json
```

4. 如果要单独调试某个 suite：

```bash
cd cmd/test-agent && go run . -config test-agent.json -suite system-smoke.json
```

5. 查看输出目录：

```text
runtime/test_runs/<date>/<evaluation_id>/
runtime/test_runs/<date>/<evaluation_id>/runs/<scenario_run_id>/
```

## 仓库内置 suites

- `suites/smoke-local.json`
  - 推荐第一步先跑这个。
  - 只验证稳定的本地契约，不依赖真实大模型输出。
- `suites/cron-auth-guard.json`
  - 单独验证 `cron-agent` 的授权守卫。
- `suites/llm-cron-reminder-observe.json`
  - 观测 `llm-agent -> wechat-agent -> task_complete` 链路。
  - 这个场景主要看路径，不强依赖最终 `task_complete.status` 必须是 `success`。
  - 如果 `app-agent` 不在线，`llm-agent` 仍可能完成微信投递，但最终状态会因为缺少 app 通道而变成 `failed`。这属于环境前提，不代表链路没有走通。

## 示例命令

```bash
cd cmd/test-agent && go run . -config test-agent.json
cd cmd/test-agent && go run . -config test-agent.json -suite smoke-local.json
cd cmd/test-agent && go run . -config test-agent.json -suite cron-auth-guard.json
cd cmd/test-agent && go run . -config test-agent.json -suite llm-cron-reminder-observe.json
```

## 场景格式

```json
{
  "id": "system-smoke",
  "title": "系统冒烟",
  "scenarios": [
    {
      "id": "echo-tool",
      "title": "工具回路",
      "entry": {
        "type": "tool_call",
        "to_agent": "echo-agent",
        "tool": {
          "tool_name": "Echo",
          "arguments": {"text": "hello"}
        }
      },
      "assertions": {
        "expect_message_type": "tool_result",
        "result_contains": ["hello"],
        "require_agents": ["echo-agent"],
        "require_msg_types": ["tool_call", "tool_result"],
        "min_trace_events": 2
      }
    }
  ]
}
```

## 环境建议

- `gateway` 必须开启 `GET /api/gateway/agents`、`GET /api/gateway/health` 和 `GET /api/gateway/events/trace/{traceID}`。
- 如果想抓取 `llm-agent` 的 `trace_*.json`，把 `test-agent.json` 里的 `llm_trace_dir` 指到 `llm-agent` 实际使用的 `session_dir`。
- 如果要启用动态评估集，需要 `llm-agent` 在线，并支持把 `llm_request` 的 `task_complete` 回发给发起方。
- 推荐先保证下列 agent 在线，再跑链路类 suite：
  - `llm-agent`
  - `cron-agent`
  - `wechat-agent`
  - `app-agent`（如果你希望 cron 双通道投递也成功）
