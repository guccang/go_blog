# 测试执行记录

## 基本信息
- run_id: `dynamic-plan-generator-8ccf203f`
- evaluation_id: `evaluation-7eed6dfb`
- scenario_id: `dynamic-plan-generator`
- status: `passed`
- collection_type: `dynamic_plan`
- entry_type: `task_assign`
- target_agent: `llm-agent`
- trace_id: `8ccf203f`
- task_id: `task-8ccf203f`
- started_at: `2026-04-12 20:53:11`
- finished_at: `2026-04-12 20:53:34`

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
- duration_ms: `22719`
- agents: llm-agent -> test-agent
- msg_types: task_accepted, task_assign, task_complete, task_event

| Seq | Kind | MsgType | From | To | Summary |
| --- | --- | --- | --- | --- | --- |
| 936439 | msg_in | task_assign | test-agent | llm-agent | task=task-8ccf203f |
| 936440 | msg_out | task_assign | test-agent | llm-agent | task=task-8ccf203f |
| 936441 | msg_in | task_accepted | llm-agent | test-agent | accepted task=task-8ccf203f |
| 936442 | msg_out | task_accepted | llm-agent | test-agent | accepted task=task-8ccf203f |
| 936443 | msg_in | task_event | llm-agent | test-agent | event task=task-8ccf203f |
| 936444 | msg_out | task_event | llm-agent | test-agent | event task=task-8ccf203f |
| 936504 | msg_in | task_event | llm-agent | test-agent | event task=task-8ccf203f |
| 936505 | msg_out | task_event | llm-agent | test-agent | event task=task-8ccf203f |
| 936506 | msg_in | task_event | llm-agent | test-agent | event task=task-8ccf203f |
| 936507 | msg_out | task_event | llm-agent | test-agent | event task=task-8ccf203f |
| 936508 | msg_in | task_complete | llm-agent | test-agent | complete task=task-8ccf203f status=success |
| 936509 | msg_out | task_complete | llm-agent | test-agent | complete task=task-8ccf203f status=success |

## 最终结果
{
  "id": "dynamic-assessment-plan",
  "title": "动态评估场景生成",
  "description": "针对静态评估未覆盖的在线agents生成协同链路和系统能力测试场景",
  "scenarios": [
    {
      "id": "acp-project-list-and-create-chain",
      "title": "ACP项目管理链路验证",
      "description": "验证acp_717d62c8的项目列表查询和创建功能，测试AcpListProjects与AcpCreateProject的协同工作能力",
      "category": "integration",
      "priority": "high",
      "tags": ["acp", "project-management", "read-write-chain"],
      "entry": {
        "type": "tool_call",
        "agent_id": "acp_717d62c8",
        "tool": "AcpListProjects",
        "params": {}
      },
      "assertions": {
        "expect_message_type": "tool_result",
        "require_agents": ["acp_717d62c8"],
        "require_msg_types": ["tool_result"],
        "min_trace_events": 1,
        "expected_path": "AcpListProjects -> [验证项目列表非空/可创建] -> AcpCreateProject -> AcpListProjects"
      }
    },
    {
      "id": "blog-content-search-and-retrieval",
      "title": "博客内容搜索与获取链路",
      "description": "测试blog-agent的RawSearchBlogContent和RawGetBlogData功能，验证博客内容查询和检索的完整性",
      "category": "system-capability",
      "priority": "medium",
      "tags": ["blog-agent", "content-retrieval", "search"],
      "entry": {
        "type": "tool_call",
        "agent_id": "blog-agent",
        "tool": "RawSearchBlogContent",
        "params": {
          "keyword": "test"
        }
      },
      "assertions": {
        "expect_message_type": "tool_result",
        "require_agents": ["blog-agent"],
        "require_msg_types": ["tool_result"],
        "min_trace_events": 1,
        "expected_path": "RawSearchBlogContent -> [获取结果] -> RawGetBlogData"
      }
    },
    {
      "id": "env-health-check-and-install",
      "title": "环境健康检查与依赖安装链路",
      "description": "验证env_agent_17587的EnvCheckAll和EnvCheck功能，测试环境状态检查和条件触发安装的能力",
      "category": "system-capability",
      "priority": "high",
      "tags": ["env-agent", "health-check", "installation"],
      "entry": {
        "type": "tool_call",
        "agent_id": "env_agent_17587",
        "tool": "EnvCheckAll",
        "params": {}
      },
      "assertions": {
        "expect_message_type": "tool_result",
        "require_agents": ["env_agent_17587"],
        "require_msg_types": ["tool_result"],
        "min_trace_events": 1,
        "expected_path": "EnvCheckAll -> [分析缺失] -> EnvCheck(条件触发) -> EnvInstall"
      }
    }
  ]
}
