package mcp

import (
	"statistics"
)

// ============================================================================
// Exercise 模块工具函数
// ============================================================================

func Inner_blog_RawGetExerciseByDate(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetExerciseByDate(account, date)
}

func Inner_blog_RawGetExerciseRange(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
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
	return statistics.RawGetExerciseRange(account, startDate, endDate)
}

func Inner_blog_RawAddExercise(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	name, err := getStringParam(arguments, "name")
	if err != nil {
		return errorJSON(err.Error())
	}
	exerciseType, err := getStringParam(arguments, "exerciseType")
	if err != nil {
		return errorJSON(err.Error())
	}
	duration, err := getIntParam(arguments, "duration")
	if err != nil {
		return errorJSON(err.Error())
	}
	intensity, _ := getStringParam(arguments, "intensity")
	if intensity == "" {
		intensity = "medium"
	}
	calories := getOptionalIntParam(arguments, "calories", 0)
	notes, _ := getStringParam(arguments, "notes")
	return statistics.RawAddExercise(account, date, name, exerciseType, duration, intensity, calories, notes)
}

func Inner_blog_RawGetExerciseStats(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	days := getOptionalIntParam(arguments, "days", 7)
	return statistics.RawGetExerciseStats(account, days)
}
