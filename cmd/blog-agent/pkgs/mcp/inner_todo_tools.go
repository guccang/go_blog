package mcp

import (
	"statistics"
)

// ============================================================================
// TodoList 模块工具函数
// ============================================================================

func Inner_blog_RawGetTodosByDate(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawGetTodosByDate(account, date))
}

func Inner_blog_RawGetTodosRange(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	startDate, err := getStringParam(arguments, "startDate")
	if err != nil {
		return errorJSON(err.Error())
	}
	endDate, err := getStringParam(arguments, "endDate")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawGetTodosRange(account, startDate, endDate))
}

func Inner_blog_RawAddTodo(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := getStringParam(arguments, "content")
	if err != nil {
		return errorJSON(err.Error())
	}
	hours := getOptionalIntParam(arguments, "hours", 0)
	minutes := getOptionalIntParam(arguments, "minutes", 0)
	urgency := getOptionalIntParam(arguments, "urgency", 2)
	importance := getOptionalIntParam(arguments, "importance", 2)
	return wrapResult(statistics.RawAddTodo(account, date, content, hours, minutes, urgency, importance))
}

func Inner_blog_RawToggleTodo(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	id, err := getStringParam(arguments, "id")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawToggleTodo(account, date, id))
}

func Inner_blog_RawDeleteTodo(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	id, err := getStringParam(arguments, "id")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawDeleteTodo(account, date, id))
}

func Inner_blog_RawUpdateTodo(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	id, err := getStringParam(arguments, "id")
	if err != nil {
		return errorJSON(err.Error())
	}
	hours, err := getIntParam(arguments, "hours")
	if err != nil {
		return errorJSON(err.Error())
	}
	minutes, err := getIntParam(arguments, "minutes")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawUpdateTodo(account, date, id, hours, minutes))
}
