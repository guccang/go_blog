# 测试执行记录

## 基本信息
- run_id: `cron-create-task-requires-authenticated-user-3ffd8c48`
- scenario_id: `cron-create-task-requires-authenticated-user`
- status: `passed`
- collection_type: `static`
- entry_type: `tool_call`
- target_agent: `cron-agent`
- trace_id: `3ffd8c48`
- started_at: `2026-04-11 23:40:28`
- finished_at: `2026-04-11 23:40:28`

## 执行步骤
1. `capture_availability` - `passed`: Gateway 可访问
2. `dispatch_entry` - `passed`: 入口消息发送成功
3. `await_execution` - `passed`: 链路执行结束
4. `collect_trace` - `passed`: completed
5. `collect_llm_trace` - `skipped`: 未匹配到 llm-agent trace
6. `evaluate_assertions` - `passed`: passed

## 断言结果
- PASS `direct_message_type`: got=tool_result want=tool_result
- PASS `tool_success`: got=false want=false
- PASS `error_contains`: needle="权限拒绝：缺少认证用户"
- PASS `trace_events`: got=4 want>=2
- PASS `require_agent`: cron-agent
- PASS `require_msg_type`: tool_call
- PASS `require_msg_type`: tool_result

## 评分
- completion: 0
- routing: 100
- tool_usage: 100
- recovery: 0
- final_answer: 100
- total: 60

## Gateway Trace
- trace_status: `completed`
- duration_ms: `1`
- agents: cron-agent -> cron_agent_903 -> test-agent
- msg_types: tool_call, tool_result

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 763468 | msg_in | tool_call | test-agent | cron-agent | cronCreateTask |
| 763469 | msg_out | tool_call | test-agent | cron_agent_903 | cronCreateTask |
| 763470 | msg_in | tool_result | cron_agent_903 | test-agent | error: 权限拒绝：缺少认证用户 |
| 763471 | msg_out | tool_result | cron_agent_903 | test-agent | error: 权限拒绝：缺少认证用户 |


## 最终错误
权限拒绝：缺少认证用户
