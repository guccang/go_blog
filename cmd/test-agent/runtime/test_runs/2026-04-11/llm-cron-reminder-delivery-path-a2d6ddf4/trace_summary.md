# 测试执行记录

## 基本信息
- run_id: `llm-cron-reminder-delivery-path-a2d6ddf4`
- scenario_id: `llm-cron-reminder-delivery-path`
- status: `failed`
- collection_type: `static`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `a2d6ddf4`
- task_id: `task-a2d6ddf4`
- started_at: `2026-04-11 23:29:32`
- finished_at: `2026-04-11 23:29:32`

## 执行步骤
1. `capture_availability` - `passed`: Gateway 可访问
2. `dispatch_entry` - `passed`: 入口消息发送成功
3. `await_execution` - `passed`: 链路执行结束
4. `collect_trace` - `passed`: completed
5. `collect_llm_trace` - `skipped`: 未匹配到 llm-agent trace
6. `evaluate_assertions` - `passed`: failed

## 断言结果
- PASS `direct_message_type`: got=task_complete want=task_complete
- PASS `trace_events`: got=6 want>=4
- PASS `require_agent`: llm-agent
- FAIL `require_agent`: wechat-agent
- PASS `require_msg_type`: task_assign
- FAIL `require_msg_type`: notify
- PASS `require_msg_type`: task_complete
- FAIL `expected_path`: actual=test-agent -> llm-agent -> test-agent -> llm-agent -> blog-agent -> llm-agent -> blog-agent -> llm-agent -> test-agent -> llm-agent -> test-agent

## 评分
- completion: 100
- routing: 50
- tool_usage: 100
- recovery: 100
- final_answer: 100
- total: 90

## Gateway Trace
- trace_status: `completed`
- duration_ms: `0`
- agents: blog-agent -> llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 761788 | msg_in | task_assign | test-agent | llm-agent | task=task-a2d6ddf4 |
| 761789 | msg_out | task_assign | test-agent | llm-agent | task=task-a2d6ddf4 |
| 761790 | msg_in | task_accepted | llm-agent | blog-agent | accepted task=task-a2d6ddf4 |
| 761791 | msg_out | task_accepted | llm-agent | blog-agent | accepted task=task-a2d6ddf4 |
| 761796 | msg_in | task_complete | llm-agent | test-agent | complete task=task-a2d6ddf4 status=success |
| 761797 | msg_out | task_complete | llm-agent | test-agent | complete task=task-a2d6ddf4 status=success |

