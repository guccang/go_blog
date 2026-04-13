# 测试执行记录

## 基本信息
- run_id: `cron-create-task-requires-authenticated-user-e4eb71cf`
- scenario_id: `cron-create-task-requires-authenticated-user`
- status: `running`
- collection_type: `static`
- entry_type: `tool_call`
- target_agent: `cron-agent`
- trace_id: `e4eb71cf`
- started_at: `2026-04-11 22:04:54`

## 执行步骤
1. `capture_availability` - `passed`: Gateway 可访问
2. `dispatch_entry` - `passed`: 入口消息发送成功
3. `await_execution` - `running`: 轮询等待链路完成

## 评分
- completion: 0
- routing: 0
- tool_usage: 0
- recovery: 0
- final_answer: 0
- total: 0

## Gateway Trace
- trace_status: `completed`
- duration_ms: `22`
- agents: cron-agent -> cron_agent_903 -> test-agent
- msg_types: tool_call, tool_result

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 750093 | msg_in | tool_call | test-agent | cron-agent | cronCreateTask |
| 750094 | msg_out | tool_call | test-agent | cron_agent_903 | cronCreateTask |
| 750095 | msg_in | tool_result | cron_agent_903 | test-agent | error: 权限拒绝：缺少认证用户 |
| 750096 | msg_out | tool_result | cron_agent_903 | test-agent | error: 权限拒绝：缺少认证用户 |

