# 套件评估结果

- run_id: `evaluation-33c4fced`
- suite_id: ``
- status: `skipped`
- collection_type: `static`
- total_scenarios: 3
- skipped_scenarios: 3
- passed_scenarios: 0
- failed_scenarios: 0
- average_score: 0

- source_files:
  - `suites/cron-auth-guard.json`
  - `suites/llm-cron-reminder-observe.json`
  - `suites/smoke-local.json`

## 维度评分
- completion: avg=0 pass=0/0
- routing: avg=0 pass=0/0
- tool_usage: avg=0 pass=0/0
- recovery: avg=0 pass=0/0
- final_answer: avg=0 pass=0/0

## 跳过场景
- `cron-create-task-requires-authenticated-user`: target agent is offline
- `llm-cron-reminder-delivery-path`: required agent is offline: wechat-agent
- `cron-create-task-requires-authenticated-user`: target agent is offline

