package mcp

import (
	"encoding/json"
	"goal"
)

// ============================================================================
// Goal 模块工具函数 - cortana-agent 友好访问
// ============================================================================

func toJSONString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return errorJSON(err.Error())
	}
	return string(data)
}

func Inner_blog_RawGetGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	g, err := goal.GetGoal(account, level, period)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(g))
}

func Inner_blog_RawGetCurrentGoals(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	goals, err := goal.GetCurrentGoals(account)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(goals))
}

func Inner_blog_RawSaveGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	overview, _ := getStringParam(arguments, "overview")
	status, _ := getStringParam(arguments, "status")
	judge, _ := getStringParam(arguments, "judge")

	g, err := goal.GetGoal(account, level, period)
	if err != nil {
		return errorJSON(err.Error())
	}
	if overview != "" {
		g.Overview = overview
	}
	if status != "" {
		g.Status = status
	}
	if judge != "" {
		g.Judge = judge
	}
	if err := goal.SaveGoal(account, g); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

func Inner_blog_RawAddGoalTask(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	priority, _ := getStringParam(arguments, "priority")
	if priority == "" {
		priority = "medium"
	}

	task := goal.Task{
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      "pending",
	}
	if err := goal.AddTask(account, level, period, task); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

func Inner_blog_RawUpdateGoalTask(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	taskID, err := getStringParam(arguments, "taskID")
	if err != nil {
		return errorJSON(err.Error())
	}
	status, _ := getStringParam(arguments, "status")
	title, _ := getStringParam(arguments, "title")
	description, _ := getStringParam(arguments, "description")
	priority, _ := getStringParam(arguments, "priority")

	g, err := goal.GetGoal(account, level, period)
	if err != nil {
		return errorJSON(err.Error())
	}

	var updatedTask goal.Task
	found := false
	for _, t := range g.Tasks {
		if t.ID == taskID {
			updatedTask = t
			found = true
			break
		}
	}
	if !found {
		return errorJSON("task not found: " + taskID)
	}
	if status != "" {
		updatedTask.Status = status
	}
	if title != "" {
		updatedTask.Title = title
	}
	if description != "" {
		updatedTask.Description = description
	}
	if priority != "" {
		updatedTask.Priority = priority
	}
	if err := goal.UpdateTask(account, level, period, taskID, updatedTask); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

func Inner_blog_RawDeleteGoalTask(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	taskID, err := getStringParam(arguments, "taskID")
	if err != nil {
		return errorJSON(err.Error())
	}
	if err := goal.DeleteTask(account, level, period, taskID); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

func Inner_blog_RawDeleteGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	if err := goal.DeleteGoal(account, level, period); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

func Inner_blog_RawListGoalsByLevel(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	year := getOptionalIntParam(arguments, "year", 0)
	goals, err := goal.ListGoalsByLevel(account, level, year)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(goals))
}

func Inner_blog_RawPrevPeriod(arguments map[string]interface{}) string {
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	prev, err := goal.PrevPeriod(level, period)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(`{"prev_period":"` + prev + `"}`)
}

func Inner_blog_RawNextPeriod(arguments map[string]interface{}) string {
	level, err := getStringParam(arguments, "level")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, err := getStringParam(arguments, "period")
	if err != nil {
		return errorJSON(err.Error())
	}
	next, err := goal.NextPeriod(level, period)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(`{"next_period":"` + next + `"}`)
}

// RawGetDailyGoal 获取今日目标（自动计算当前周期，无需传 level/period）
func Inner_blog_RawGetDailyGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	daily, _, _, _ := goal.CurrentPeriods()
	g, err := goal.GetGoal(account, "daily", daily)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(g))
}

// RawGetWeeklyGoal 获取本周目标
func Inner_blog_RawGetWeeklyGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	_, weekly, _, _ := goal.CurrentPeriods()
	g, err := goal.GetGoal(account, "weekly", weekly)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(g))
}

// RawGetMonthlyGoal 获取本月目标
func Inner_blog_RawGetMonthlyGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	_, _, monthly, _ := goal.CurrentPeriods()
	g, err := goal.GetGoal(account, "monthly", monthly)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(g))
}

// RawGetYearlyGoal 获取本年目标
func Inner_blog_RawGetYearlyGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	_, _, _, yearly := goal.CurrentPeriods()
	g, err := goal.GetGoal(account, "yearly", yearly)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(g))
}

// ============================================================================
// 便捷目标操作工具 — 自动计算周期，对标 Todo 的简洁度
// ============================================================================

// RawAddDailyGoalTask 为今日目标添加任务（自动计算周期，目标不存在则自动创建）
func Inner_blog_RawAddDailyGoalTask(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	priority, _ := getStringParam(arguments, "priority")
	if priority == "" {
		priority = "medium"
	}

	daily, _, _, _ := goal.CurrentPeriods()
	task := goal.Task{Title: title, Description: description, Priority: priority, Status: "pending"}
	if err := goal.AddTask(account, "daily", daily, task); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

// RawAddWeeklyGoalTask 为本周目标添加任务
func Inner_blog_RawAddWeeklyGoalTask(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	priority, _ := getStringParam(arguments, "priority")
	if priority == "" {
		priority = "medium"
	}

	_, weekly, _, _ := goal.CurrentPeriods()
	task := goal.Task{Title: title, Description: description, Priority: priority, Status: "pending"}
	if err := goal.AddTask(account, "weekly", weekly, task); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}

// RawAddMonthlyGoalTask 为本月目标添加任务
func Inner_blog_RawAddMonthlyGoalTask(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	priority, _ := getStringParam(arguments, "priority")
	if priority == "" {
		priority = "medium"
	}

	_, _, monthly, _ := goal.CurrentPeriods()
	task := goal.Task{Title: title, Description: description, Priority: priority, Status: "pending"}
	if err := goal.AddTask(account, "monthly", monthly, task); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}
