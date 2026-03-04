package mcp

import (
	"statistics"
)

// ============================================================================
// YearPlan 模块工具函数
// ============================================================================

func Inner_blog_RawGetMonthGoal(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	year, err := getIntParam(arguments, "year")
	if err != nil {
		return errorJSON(err.Error())
	}
	month, err := getIntParam(arguments, "month")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetMonthGoal(account, year, month)
}

func Inner_blog_RawGetYearGoals(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	year, err := getIntParam(arguments, "year")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetYearGoals(account, year)
}

func Inner_blog_RawAddYearTask(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	year, err := getIntParam(arguments, "year")
	if err != nil {
		return errorJSON(err.Error())
	}
	month, err := getIntParam(arguments, "month")
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
	dueDate, _ := getStringParam(arguments, "dueDate")
	return statistics.RawAddYearTask(account, year, month, title, description, priority, dueDate)
}

func Inner_blog_RawUpdateYearTask(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	year, err := getIntParam(arguments, "year")
	if err != nil {
		return errorJSON(err.Error())
	}
	month, err := getIntParam(arguments, "month")
	if err != nil {
		return errorJSON(err.Error())
	}
	taskID, err := getStringParam(arguments, "taskID")
	if err != nil {
		return errorJSON(err.Error())
	}
	status, err := getStringParam(arguments, "status")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawUpdateYearTask(account, year, month, taskID, status)
}

// ============================================================================
// TaskBreakdown 模块工具函数
// ============================================================================

func Inner_blog_RawGetAllComplexTasks(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetAllComplexTasks(account)
}

func Inner_blog_RawGetComplexTasksByStatus(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	status, err := getStringParam(arguments, "status")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetComplexTasksByStatus(account, status)
}

func Inner_blog_RawGetComplexTaskStats(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetComplexTaskStats(account)
}

func Inner_blog_RawCreateComplexTask(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
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
	startDate, _ := getStringParam(arguments, "startDate")
	endDate, _ := getStringParam(arguments, "endDate")
	return statistics.RawCreateComplexTask(account, title, description, priority, startDate, endDate)
}
