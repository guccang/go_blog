---
name: goal-management
description: 目标管理技能。覆盖日/周/月/年 4 个层级的目标查询、任务管理和周期导航。
summary: 今日目标用 RawGetDailyGoal，本周用 RawGetWeeklyGoal；添加目标任务用 RawAddDailyGoalTask/RawAddWeeklyGoalTask；修改任务用 RawUpdateGoalTask
tools: RawGetDailyGoal,RawGetWeeklyGoal,RawGetMonthlyGoal,RawGetYearlyGoal,RawGetGoal,RawGetCurrentGoals,RawSaveGoal,RawAddGoalTask,RawUpdateGoalTask,RawDeleteGoalTask,RawDeleteGoal,RawListGoalsByLevel,RawPrevPeriod,RawNextPeriod,RawAddDailyGoalTask,RawAddWeeklyGoalTask,RawAddMonthlyGoalTask
agents: blog
keywords: 目标,日目标,周目标,月目标,年目标,goal,今日目标,本周目标,本月目标,本年目标,添加目标,目标管理,添加日目标,daily,weekly,monthly,yearly
---

# 目标管理

## 适用场景

- 查询今日/本周/本月/本年目标及任务列表
- 为目标添加子任务、更新任务状态（标记完成）
- 跨周期目标导航和汇总
- 日常目标管理不同于待办清单（Todo），"添加日目标"属于目标管理域

## 必须遵守

- `account` 默认使用当前用户账号，不要追问
- 明确的时间查询优先用专用工具：今日→`RawGetDailyGoal`，本周→`RawGetWeeklyGoal`，本月→`RawGetMonthlyGoal`，本年→`RawGetYearlyGoal`
- 添加日目标任务用 `RawAddDailyGoalTask`，添加周目标任务用 `RawAddWeeklyGoalTask`
- 需要跨周期汇总或不确定层级时才用 `RawGetCurrentGoals`
- 修改任务前先通过查询获取 `taskID`

## 推荐流程

1. 先确认用户意图的时间范围（今日/本周/本月/本年）。
2. 按时间范围选择接口：
   - 查询：`RawGetDailyGoal`(今日)、`RawGetWeeklyGoal`(本周)、`RawGetMonthlyGoal`(本月)、`RawGetYearlyGoal`(本年)
   - 跨周期汇总：`RawGetCurrentGoals`
   - 添加任务：`RawAddDailyGoalTask`(日)、`RawAddWeeklyGoalTask`(周)、`RawAddMonthlyGoalTask`(月)
   - 修改/完成/删除任务：先查到 `taskID`，再调用 `RawUpdateGoalTask` 或 `RawDeleteGoalTask`
   - 修改目标概述/状态：`RawSaveGoal`
3. 如果是跨周期分析，用 `RawListGoalsByLevel` 获取历史数据。

## 工具选择规则

- "今日目标" → `RawGetDailyGoal` （不是 Todo）
- "本周目标" → `RawGetWeeklyGoal`
- "添加日目标 睡觉" → `RawAddDailyGoalTask(account, title="睡觉")` （不是 RawAddTodo）
- "完成今天的第2个目标任务" → 先查 `RawGetDailyGoal`，找到 taskID，再调 `RawUpdateGoalTask`

## 禁止行为

- 将目标管理请求错发为待办操作（添加目标 ≠ RawAddTodo，查目标 ≠ RawGetTodosByDate）
- 让用户手动提供 level/period 参数
- 把大段原始数据直接回灌给用户，不做整理

## 示例

- "今日目标是什么"
  用 `RawGetDailyGoal` 获取今日目标及任务列表
- "添加日目标 跑步5公里"
  用 `RawAddDailyGoalTask`，account=当前用户，title="跑步5公里"
- "把本周第一个目标任务标记完成"
  先 `RawGetWeeklyGoal` 查 taskID，再 `RawUpdateGoalTask(status="completed")`
