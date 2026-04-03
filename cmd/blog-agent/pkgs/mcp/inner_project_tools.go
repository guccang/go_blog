package mcp

import "statistics"

func Inner_blog_RawCreateProject(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	name, err := getStringParam(arguments, "name")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	status, _ := getStringParam(arguments, "status")
	if status == "" {
		status = "planning"
	}
	priority, _ := getStringParam(arguments, "priority")
	if priority == "" {
		priority = "medium"
	}
	owner, _ := getStringParam(arguments, "owner")
	startDate, _ := getStringParam(arguments, "startDate")
	endDate, _ := getStringParam(arguments, "endDate")
	tags, _ := getStringParam(arguments, "tags")
	return wrapResult(statistics.RawCreateProject(account, name, description, status, priority, owner, startDate, endDate, tags))
}

func Inner_blog_RawGetProject(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawGetProject(account, projectID))
}

func Inner_blog_RawListProjects(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	status, _ := getStringParam(arguments, "status")
	return wrapResult(statistics.RawListProjects(account, status))
}

func Inner_blog_RawUpdateProject(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	name, _ := getStringParam(arguments, "name")
	description, _ := getStringParam(arguments, "description")
	status, _ := getStringParam(arguments, "status")
	priority, _ := getStringParam(arguments, "priority")
	owner, _ := getStringParam(arguments, "owner")
	startDate, _ := getStringParam(arguments, "startDate")
	endDate, _ := getStringParam(arguments, "endDate")
	tags, _ := getStringParam(arguments, "tags")
	return wrapResult(statistics.RawUpdateProject(account, projectID, name, description, status, priority, owner, startDate, endDate, tags))
}

func Inner_blog_RawDeleteProject(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawDeleteProject(account, projectID))
}

func Inner_blog_RawAddProjectGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	status, _ := getStringParam(arguments, "status")
	if status == "" {
		status = "pending"
	}
	priority, _ := getStringParam(arguments, "priority")
	if priority == "" {
		priority = "medium"
	}
	startDate, _ := getStringParam(arguments, "startDate")
	endDate, _ := getStringParam(arguments, "endDate")
	progress := getOptionalIntParam(arguments, "progress", 0)
	return wrapResult(statistics.RawAddProjectGoal(account, projectID, title, description, status, priority, startDate, endDate, progress))
}

func Inner_blog_RawUpdateProjectGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	goalID, err := getStringParam(arguments, "goalID")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	description, _ := getStringParam(arguments, "description")
	status, err := getStringParam(arguments, "status")
	if err != nil {
		return errorJSON(err.Error())
	}
	priority, err := getStringParam(arguments, "priority")
	if err != nil {
		return errorJSON(err.Error())
	}
	startDate, _ := getStringParam(arguments, "startDate")
	endDate, _ := getStringParam(arguments, "endDate")
	progress := getOptionalIntParam(arguments, "progress", 0)
	return wrapResult(statistics.RawUpdateProjectGoal(account, projectID, goalID, title, description, status, priority, startDate, endDate, progress))
}

func Inner_blog_RawDeleteProjectGoal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	goalID, err := getStringParam(arguments, "goalID")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawDeleteProjectGoal(account, projectID, goalID))
}

func Inner_blog_RawAddProjectOKR(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	objective, err := getStringParam(arguments, "objective")
	if err != nil {
		return errorJSON(err.Error())
	}
	status, _ := getStringParam(arguments, "status")
	if status == "" {
		status = "draft"
	}
	period, _ := getStringParam(arguments, "period")
	progress := getOptionalIntParam(arguments, "progress", 0)
	return wrapResult(statistics.RawAddProjectOKR(account, projectID, objective, status, period, progress))
}

func Inner_blog_RawUpdateProjectOKR(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	okrID, err := getStringParam(arguments, "okrID")
	if err != nil {
		return errorJSON(err.Error())
	}
	objective, err := getStringParam(arguments, "objective")
	if err != nil {
		return errorJSON(err.Error())
	}
	status, err := getStringParam(arguments, "status")
	if err != nil {
		return errorJSON(err.Error())
	}
	period, _ := getStringParam(arguments, "period")
	progress := getOptionalIntParam(arguments, "progress", 0)
	return wrapResult(statistics.RawUpdateProjectOKR(account, projectID, okrID, objective, status, period, progress))
}

func Inner_blog_RawDeleteProjectOKR(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	okrID, err := getStringParam(arguments, "okrID")
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawDeleteProjectOKR(account, projectID, okrID))
}

func Inner_blog_RawUpdateProjectKeyResult(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	projectID, err := getStringParam(arguments, "projectID")
	if err != nil {
		return errorJSON(err.Error())
	}
	okrID, err := getStringParam(arguments, "okrID")
	if err != nil {
		return errorJSON(err.Error())
	}
	keyResultID, _ := getStringParam(arguments, "keyResultID")
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	metricType, err := getStringParam(arguments, "metricType")
	if err != nil {
		return errorJSON(err.Error())
	}
	targetValue, err := getFloatParam(arguments, "targetValue")
	if err != nil {
		return errorJSON(err.Error())
	}
	currentValue := getOptionalFloatParam(arguments, "currentValue", 0)
	unit, _ := getStringParam(arguments, "unit")
	status, _ := getStringParam(arguments, "status")
	if status == "" {
		status = "pending"
	}
	return wrapResult(statistics.RawUpdateProjectKeyResult(account, projectID, okrID, keyResultID, title, metricType, targetValue, currentValue, unit, status))
}

func Inner_blog_RawGetProjectSummary(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(statistics.RawGetProjectSummary(account))
}
