# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-02eb5668`
- evaluation_id: `evaluation-730039f5`
- scenario_id: `dynamic-plan-generator`
- status: `passed`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `02eb5668`
- task_id: `task-02eb5668`
- started_at: `2026-04-11 23:46:00`
- finished_at: `2026-04-11 23:46:16`

## 执行步骤
1. `capture_availability` - `passed`: Gateway 可访问
2. `dispatch_entry` - `passed`: 入口消息发送成功
3. `await_execution` - `passed`: 链路执行结束
4. `collect_trace` - `passed`: completed
5. `collect_llm_trace` - `skipped`: 未匹配到 llm-agent trace
6. `evaluate_assertions` - `passed`: passed

## 断言结果
- PASS `direct_message_type`: got=task_complete want=task_complete
- PASS `task_status`: got=success want=success
- PASS `trace_events`: got=12 want>=2
- PASS `require_agent`: llm-agent
- PASS `require_msg_type`: task_assign
- PASS `require_msg_type`: task_complete

## 评分
- completion: 100
- routing: 100
- tool_usage: 100
- recovery: 100
- final_answer: 100
- total: 100

## Gateway Trace
- trace_status: `completed`
- duration_ms: `15597`
- agents: llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 764235 | msg_in | task_assign | test-agent | llm-agent | task=task-02eb5668 |
| 764236 | msg_out | task_assign | test-agent | llm-agent | task=task-02eb5668 |
| 764237 | msg_in | task_accepted | llm-agent | test-agent | accepted task=task-02eb5668 |
| 764238 | msg_out | task_accepted | llm-agent | test-agent | accepted task=task-02eb5668 |
| 764239 | msg_in | task_event | llm-agent | test-agent | event task=task-02eb5668 |
| 764240 | msg_out | task_event | llm-agent | test-agent | event task=task-02eb5668 |
| 764280 | msg_in | task_event | llm-agent | test-agent | event task=task-02eb5668 |
| 764281 | msg_out | task_event | llm-agent | test-agent | event task=task-02eb5668 |
| 764282 | msg_in | task_event | llm-agent | test-agent | event task=task-02eb5668 |
| 764283 | msg_out | task_event | llm-agent | test-agent | event task=task-02eb5668 |
| 764284 | msg_in | task_complete | llm-agent | test-agent | complete task=task-02eb5668 status=success |
| 764285 | msg_out | task_complete | llm-agent | test-agent | complete task=task-02eb5668 status=success |

## 最终结果
{"id":"dyn_001","title":"博客内容发布协同链路测试","description":"验证 blog-agent 创建博客内容后，通过 cron_agent 设置定时发布任务的协同链路","category":"协同链路","priority":"high","tags":["blog-agent","cron_agent","协同"],"entry":{"type":"task_assign","to_agent":"blog-agent","task":"create_blog"},"assertions":{"expect_message_type":"notify","require_agents":["blog-agent","cron_agent_903"],"require_msg_types":["task_assign","notify"],"min_trace_events":4,"expected_path":["blog-agent::create","cron_agent_903::create_task"]}}

{"id":"dyn_002","title":"环境检查与日志查询系统能力测试","description":"验证 env_agent 检查环境状态后，通过 log_query 查询相关系统日志的集成能力","category":"系统能力","priority":"medium","tags":["env_agent","log_query","集成"],"entry":{"type":"tool_call","to_agent":"env_agent_17587","tool":"EnvCheckAll"},"assertions":{"expect_message_type":"notify","require_agents":["env_agent_17587","log_query_18583"],"require_msg_types":["tool_call","notify"],"min_trace_events":3}}

{"id":"dyn_003","title":"消息推送多通道能力测试","description":"验证 app-agent 和 wechat-agent 各自独立发送消息到不同通道的能力","category":"系统能力","priority":"medium","tags":["app-agent","wechat-agent","消息推送"],"entry":{"type":"task_assign","to_agent":"app-app-agent","task":"send_rich_message"},"assertions":{"expect_message_type":"notify","require_agents":["app-app-agent","wechat-wechat-agent"],"require_msg_types":["task_assign","notify","task_assign"],"min_trace_events":4}}
