# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-f0545788`
- evaluation_id: `evaluation-61cd41b6`
- scenario_id: `dynamic-plan-generator`
- status: `timeout`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `f0545788`
- task_id: `task-f0545788`
- started_at: `2026-04-11 23:27:25`
- finished_at: `2026-04-11 23:27:55`

## 执行步骤
1. `capture_availability` - `passed`: Gateway 可访问
2. `dispatch_entry` - `passed`: 入口消息发送成功
3. `await_execution` - `failed`: scenario timeout after 30s
4. `collect_trace` - `passed`: completed
5. `collect_llm_trace` - `skipped`: 未匹配到 llm-agent trace
6. `evaluate_assertions` - `passed`: timeout

## 断言结果
- FAIL `direct_message_type`: got= want=task_complete
- FAIL `task_status`: got= want=success
- PASS `trace_events`: got=12 want>=2
- PASS `require_agent`: llm-agent
- PASS `require_msg_type`: task_assign
- PASS `require_msg_type`: task_complete

## 评分
- completion: 0
- routing: 100
- tool_usage: 100
- recovery: 0
- final_answer: 0
- total: 40

## Gateway Trace
- trace_status: `completed`
- duration_ms: `17589`
- agents: blog-agent -> llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 761488 | msg_in | task_assign | test-agent | llm-agent | task=task-f0545788 |
| 761489 | msg_out | task_assign | test-agent | llm-agent | task=task-f0545788 |
| 761490 | msg_in | task_accepted | llm-agent | blog-agent | accepted task=task-f0545788 |
| 761491 | msg_out | task_accepted | llm-agent | blog-agent | accepted task=task-f0545788 |
| 761492 | msg_in | task_event | llm-agent | blog-agent | event task=task-f0545788 |
| 761493 | msg_out | task_event | llm-agent | blog-agent | event task=task-f0545788 |
| 761532 | msg_in | task_event | llm-agent | blog-agent | event task=task-f0545788 |
| 761533 | msg_out | task_event | llm-agent | blog-agent | event task=task-f0545788 |
| 761534 | msg_in | task_event | llm-agent | blog-agent | event task=task-f0545788 |
| 761535 | msg_out | task_event | llm-agent | blog-agent | event task=task-f0545788 |
| 761536 | msg_in | task_complete | llm-agent | blog-agent | complete task=task-f0545788 status=success |
| 761537 | msg_out | task_complete | llm-agent | blog-agent | complete task=task-f0545788 status=success |


## 最终错误
scenario timeout after 30s
