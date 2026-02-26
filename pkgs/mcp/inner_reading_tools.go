package mcp

import (
	"statistics"
)

// ============================================================================
// Reading 模块工具函数
// ============================================================================

func Inner_blog_RawGetAllBooks(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetAllBooks(account)
}

func Inner_blog_RawGetBooksByStatus(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	status, err := getStringParam(arguments, "status")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetBooksByStatus(account, status)
}

func Inner_blog_RawGetReadingStats(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetReadingStats(account)
}

func Inner_blog_RawUpdateReadingProgress(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	bookID, err := getStringParam(arguments, "bookID")
	if err != nil {
		return errorJSON(err.Error())
	}
	currentPage, err := getIntParam(arguments, "currentPage")
	if err != nil {
		return errorJSON(err.Error())
	}
	notes, _ := getStringParam(arguments, "notes")
	return statistics.RawUpdateReadingProgress(account, bookID, currentPage, notes)
}

func Inner_blog_RawGetBookNotes(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	bookID, err := getStringParam(arguments, "bookID")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetBookNotes(account, bookID)
}
