# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-123328a4`
- evaluation_id: `evaluation-28902e05`
- scenario_id: `dynamic-plan-generator`
- status: `timeout`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `123328a4`
- task_id: `task-123328a4`
- started_at: `2026-04-11 23:29:39`
- finished_at: `2026-04-11 23:30:09`

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
- duration_ms: `22499`
- agents: blog-agent -> llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 761810 | msg_in | task_assign | test-agent | llm-agent | task=task-123328a4 |
| 761811 | msg_out | task_assign | test-agent | llm-agent | task=task-123328a4 |
| 761812 | msg_in | task_accepted | llm-agent | blog-agent | accepted task=task-123328a4 |
| 761813 | msg_out | task_accepted | llm-agent | blog-agent | accepted task=task-123328a4 |
| 761814 | msg_in | task_event | llm-agent | blog-agent | event task=task-123328a4 |
| 761815 | msg_out | task_event | llm-agent | blog-agent | event task=task-123328a4 |
| 761872 | msg_in | task_event | llm-agent | blog-agent | event task=task-123328a4 |
| 761873 | msg_out | task_event | llm-agent | blog-agent | event task=task-123328a4 |
| 761874 | msg_in | task_event | llm-agent | blog-agent | event task=task-123328a4 |
| 761875 | msg_out | task_event | llm-agent | blog-agent | event task=task-123328a4 |
| 761876 | msg_in | task_complete | llm-agent | blog-agent | complete task=task-123328a4 status=success |
| 761877 | msg_out | task_complete | llm-agent | blog-agent | complete task=task-123328a4 status=success |


## 最终错误
scenario timeout after 30s
