# 套件评估结果

- run_id: `evaluation-7eed6dfb`
- suite_id: ``
- status: `failed`
- collection_type: `static`
- total_scenarios: 3
- executed_scenarios: 3
- passed_scenarios: 2
- failed_scenarios: 1
- average_score: 70

- source_files:
  - `suites/cron-auth-guard.json`
  - `suites/llm-cron-reminder-observe.json`
  - `suites/smoke-local.json`

## 维度评分
- completion: avg=33 pass=1/3
- routing: avg=83 pass=2/3
- tool_usage: avg=100 pass=3/3
- recovery: avg=33 pass=1/3
- final_answer: avg=100 pass=3/3

| Scenario | Status | Score | Final |
| --- | --- | --- | --- |
| cron-create-task-requires-authenticated-user | passed | 60 | failed |
| cron-create-task-requires-authenticated-user | passed | 60 | failed |
| llm-cron-reminder-delivery-path | failed | 90 | success |
