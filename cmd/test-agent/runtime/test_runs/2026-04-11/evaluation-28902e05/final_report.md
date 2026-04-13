# 系统评估总览

- run_id: `evaluation-28902e05`
- status: `failed`
- overall_score: 40
- started_at: `2026-04-11 23:29:39`
- finished_at: `2026-04-11 23:30:09`

## 静态评估集
- status: `skipped`
- total: 3
- executed: 0
- passed: 0
- failed: 0
- avg_score: 0

## 动态评估集
- status: `failed`
- total: 1
- executed: 1
- skipped: 0
- passed: 0
- failed: 1
- avg_score: 40

## 综合维度评分
- completion: avg=0 pass=0/1
- routing: avg=100 pass=1/1
- tool_usage: avg=100 pass=1/1
- recovery: avg=0 pass=0/1
- final_answer: avg=0 pass=0/1

## Agent 评估
- `acp_717d62c8`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `app-app-agent`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `audio_agent_27252`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `blog-agent`: targeted=0 observed=1 passed=0 failed=0 avg=0
- `cmd-agent`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `cron_agent_903`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `env_agent_17587`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `exec_code_21360`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `llm-agent`: targeted=1 observed=1 passed=0 failed=1 avg=40
- `log_query_18583`: targeted=0 observed=0 passed=0 failed=0 avg=0
- `test-agent`: targeted=0 observed=1 passed=0 failed=0 avg=0
- `wechat-wechat-agent`: targeted=0 observed=0 passed=0 failed=0 avg=0

## 关键结论
- static collection: executed=0 passed=0 failed=0 skipped=3 avg=0
- dynamic collection: executed=1 passed=0 failed=1 skipped=0 avg=40
- dynamic plan execution failed
