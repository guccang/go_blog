# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-bce45e3c`
- evaluation_id: `evaluation-97bb3ab3`
- scenario_id: `dynamic-plan-generator`
- status: `passed`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `bce45e3c`
- task_id: `task-bce45e3c`
- started_at: `2026-04-11 23:39:41`
- finished_at: `2026-04-11 23:40:01`

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
- duration_ms: `19288`
- agents: llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 763350 | msg_in | task_assign | test-agent | llm-agent | task=task-bce45e3c |
| 763351 | msg_out | task_assign | test-agent | llm-agent | task=task-bce45e3c |
| 763352 | msg_in | task_accepted | llm-agent | test-agent | accepted task=task-bce45e3c |
| 763353 | msg_out | task_accepted | llm-agent | test-agent | accepted task=task-bce45e3c |
| 763354 | msg_in | task_event | llm-agent | test-agent | event task=task-bce45e3c |
| 763355 | msg_out | task_event | llm-agent | test-agent | event task=task-bce45e3c |
| 763400 | msg_in | task_event | llm-agent | test-agent | event task=task-bce45e3c |
| 763401 | msg_out | task_event | llm-agent | test-agent | event task=task-bce45e3c |
| 763402 | msg_in | task_event | llm-agent | test-agent | event task=task-bce45e3c |
| 763403 | msg_out | task_event | llm-agent | test-agent | event task=task-bce45e3c |
| 763404 | msg_in | task_complete | llm-agent | test-agent | complete task=task-bce45e3c status=success |
| 763405 | msg_out | task_complete | llm-agent | test-agent | complete task=task-bce45e3c status=success |

## 最终结果
{"id":"dynamic-test-plan-001","title":"动态评估测试场景集","description":"针对未被静态评估覆盖的在线 agent 进行协同链路和系统能力验证","scenarios":[{"id":"scenario-acp-001","title":"ACP项目管理链路测试","description":"测试 acp agent 的项目创建、查询功能链路，验证项目管理能力","category":"system_capability","priority":"medium","tags":["acp","project","read_only"],"entry":{"type":"tool_call","agent_id":"acp_717d62c8","tool":"AcpListProjects","params":{}},"assertions":{"expect_message_type":"tool_result","require_agents":["acp_717d62c8"],"require_msg_types":["tool_result"],"min_trace_events":2}},{"id":"scenario-blog-001","title":"博客阅读数据查询链路测试","description":"测试 blog-agent 的博客数据查询、书籍管理功能，验证内容管理能力","category":"system_capability","priority":"medium","tags":["blog","read_only","query"],"entry":{"type":"tool_call","agent_id":"blog-agent","tool":"RawAllBlogName","params":{}},"assertions":{"expect_message_type":"tool_result","require_agents":["blog-agent"],"require_msg_types":["tool_result"],"min_trace_events":2}},{"id":"scenario-env-001","title":"环境检查链路测试","description":"测试 env-agent 的环境检查和状态查询功能，验证环境管理能力","category":"system_capability","priority":"low","tags":["env","check","read_only"],"entry":{"type":"tool_call","agent_id":"env_agent_17587","tool":"EnvCheckAll","params":{}},"assertions":{"expect_message_type":"tool_result","require_agents":["env_agent_17587"],"require_msg_types":["tool_result"],"min_trace_events":2}}]}
