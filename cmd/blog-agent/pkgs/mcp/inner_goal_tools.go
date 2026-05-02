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
	if err := goal.UpdateTask(account, level, period, taskID, updatedTask); err != nil {
		return errorJSON(err.Error())
	}
	return `{"success": true}`
}
