# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-0cce07b3`
- evaluation_id: `evaluation-01a4a1d9`
- scenario_id: `dynamic-plan-generator`
- status: `timeout`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `0cce07b3`
- task_id: `task-0cce07b3`
- started_at: `2026-04-11 23:22:19`
- finished_at: `2026-04-11 23:22:49`

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
- duration_ms: `21893`
- agents: blog-agent -> llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 760761 | msg_in | task_assign | test-agent | llm-agent | task=task-0cce07b3 |
| 760762 | msg_out | task_assign | test-agent | llm-agent | task=task-0cce07b3 |
| 760763 | msg_in | task_accepted | llm-agent | blog-agent | accepted task=task-0cce07b3 |
| 760764 | msg_out | task_accepted | llm-agent | blog-agent | accepted task=task-0cce07b3 |
| 760765 | msg_in | task_event | llm-agent | blog-agent | event task=task-0cce07b3 |
| 760766 | msg_out | task_event | llm-agent | blog-agent | event task=task-0cce07b3 |
| 760812 | msg_in | task_event | llm-agent | blog-agent | event task=task-0cce07b3 |
| 760813 | msg_out | task_event | llm-agent | blog-agent | event task=task-0cce07b3 |
| 760814 | msg_in | task_event | llm-agent | blog-agent | event task=task-0cce07b3 |
| 760815 | msg_out | task_event | llm-agent | blog-agent | event task=task-0cce07b3 |
| 760816 | msg_in | task_complete | llm-agent | blog-agent | complete task=task-0cce07b3 status=success |
| 760817 | msg_out | task_complete | llm-agent | blog-agent | complete task=task-0cce07b3 status=success |


## 最终错误
scenario timeout after 30s
