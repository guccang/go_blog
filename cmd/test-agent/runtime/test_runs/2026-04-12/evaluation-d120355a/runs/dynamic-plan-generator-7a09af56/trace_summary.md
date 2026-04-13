# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-7a09af56`
- evaluation_id: `evaluation-d120355a`
- scenario_id: `dynamic-plan-generator`
- status: `running`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `7a09af56`
- task_id: `task-7a09af56`
- started_at: `2026-04-12 20:28:35`

## 执行步骤
1. `capture_availability` - `passed`: Gateway 可访问
2. `dispatch_entry` - `passed`: 入口消息发送成功
3. `await_execution` - `running`: 轮询等待链路完成 · 已耗时 8s · 剩余 22s · trace=in_progress · events=6 · 最近事件=llm-agent -> test-agent | task_event | event task=task-7a09af56

## 评分
- completion: 0
- routing: 0
- tool_usage: 0
- recovery: 0
- final_answer: 0
- total: 0

## Gateway Trace
- trace_status: `in_progress`
- duration_ms: `0`
- agents: llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 932884 | msg_in | task_assign | test-agent | llm-agent | task=task-7a09af56 |
| 932885 | msg_out | task_assign | test-agent | llm-agent | task=task-7a09af56 |
| 932886 | msg_in | task_accepted | llm-agent | test-agent | accepted task=task-7a09af56 |
| 932887 | msg_out | task_accepted | llm-agent | test-agent | accepted task=task-7a09af56 |
| 932888 | msg_in | task_event | llm-agent | test-agent | event task=task-7a09af56 |
| 932889 | msg_out | task_event | llm-agent | test-agent | event task=task-7a09af56 |

