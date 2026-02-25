package mcp

import (
	"fmt"
	"statistics"
	"strconv"
)

// 提供内部mcp接口,接口名称为Inner_blog.xxx
var callBacks = make(map[string]func(arguments map[string]interface{}) string)
var callBacksPrompt = make(map[string]string)

// ============================================================================
// 安全参数提取辅助函数
// ============================================================================

// getStringParam 安全提取字符串参数
func getStringParam(arguments map[string]interface{}, key string) (string, error) {
	val, ok := arguments[key]
	if !ok {
		return "", fmt.Errorf("缺少参数: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("参数类型错误: %s 应为字符串", key)
	}
	return str, nil
}

// getIntParam 安全提取整数参数 (JSON数字默认为float64)
func getIntParam(arguments map[string]interface{}, key string) (int, error) {
	val, ok := arguments[key]
	if !ok {
		return 0, fmt.Errorf("缺少参数: %s", key)
	}
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("参数类型错误: %s 应为数字", key)
	}
}

// getOptionalIntParam 安全提取可选整数参数
func getOptionalIntParam(arguments map[string]interface{}, key string, defaultVal int) int {
	val, ok := arguments[key]
	if !ok {
		return defaultVal
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return defaultVal
	}
}

// errorJSON 返回JSON格式的错误消息
func errorJSON(msg string) string {
	return fmt.Sprintf(`{"error": "%s"}`, msg)
}

// ============================================================================
// 内部工具函数 - 使用安全类型断言
// ============================================================================

func Inner_blog_RawAllBlogName(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawAllBlogName(account)
}

func Inner_blog_RawGetBlogData(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetBlogData(account, title)
}

func Inner_blog_RawAllCommentData(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawAllCommentData(account)
}

func Inner_blog_RawCommentData(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawCommentData(account, title)
}

func Inner_blog_RawAllBlogNameByDate(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawAllBlogNameByDate(account, date)
}

func Inner_blog_RawAllBlogNameByDateRange(arguments map[string]interface{}) string {
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
	return statistics.RawAllBlogNameByDateRange(account, startDate, endDate)
}

func Inner_blog_RawAllBlogNameByDateRangeCount(arguments map[string]interface{}) string {
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
	return string(statistics.RawAllBlogNameByDateRangeCount(account, startDate, endDate))
}

func Inner_blog_RawGetBlogDataByDate(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetBlogDataByDate(account, date)
}

func Inner_blog_RawCurrentDate(arguments map[string]interface{}) string {
	return statistics.RawCurrentDate()
}

func Inner_blog_RawCurrentTime(arguments map[string]interface{}) string {
	return statistics.RawCurrentTime()
}

func Inner_blog_RawAllBlogCount(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return strconv.Itoa(statistics.RawAllBlogCount(account))
}

func Inner_blog_RawAllDiaryCount(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return strconv.Itoa(statistics.RawAllDiaryCount(account))
}

func Inner_blog_RawCurrentDiaryContent(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawCurrentDiaryContent(account)
}

func Inner_blog_RawAllExerciseCount(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return strconv.Itoa(statistics.RawAllExerciseCount(account))
}

func Inner_blog_RawAllExerciseTotalMinutes(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return strconv.Itoa(statistics.RawAllExerciseTotalMinutes(account))
}

func Inner_blog_RawAllExerciseDistance(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return strconv.Itoa(statistics.RawAllExerciseDistance(account))
}

func Inner_blog_RawAllExerciseCalories(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return strconv.Itoa(statistics.RawAllExerciseCalories(account))
}

func Inner_blog_RawAllDiaryContent(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawAllDiaryContent(account)
}

func Inner_blog_RawGetBlogByTitleMatch(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	match, err := getStringParam(arguments, "match")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetBlogByTitleMatch(account, match)
}

func Inner_blog_RawGetCurrentTask(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetCurrentTask(account)
}

func Inner_blog_RawGetCurrentTaskByDate(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetCurrentTaskByDate(account, date)
}

func Inner_blog_RawGetCurrentTaskByRageDate(arguments map[string]interface{}) string {
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
	return statistics.RawGetCurrentTaskByRageDate(account, startDate, endDate)
}

func Inner_blog_RawCreateBlog(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := getStringParam(arguments, "content")
	if err != nil {
		return errorJSON(err.Error())
	}
	tags, err := getStringParam(arguments, "tags")
	if err != nil {
		return errorJSON(err.Error())
	}
	authType, err := getIntParam(arguments, "authType")
	if err != nil {
		return errorJSON(err.Error())
	}
	encrypt := getOptionalIntParam(arguments, "encrypt", 0)
	return statistics.RawCreateBlog(account, title, content, tags, authType, encrypt)
}

// =================================== 扩展Inner_blog接口 =========================================

// 博客统计相关接口
func Inner_blog_RawBlogStatistics(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawBlogStatistics(account)
}

func Inner_blog_RawAccessStatistics(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawAccessStatistics(account)
}

func Inner_blog_RawTopAccessedBlogs(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawTopAccessedBlogs(account)
}

func Inner_blog_RawRecentAccessedBlogs(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawRecentAccessedBlogs(account)
}

func Inner_blog_RawEditStatistics(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawEditStatistics(account)
}

func Inner_blog_RawTagStatistics(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawTagStatistics(account)
}

func Inner_blog_RawCommentStatistics(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawCommentStatistics(account)
}

func Inner_blog_RawContentStatistics(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawContentStatistics(account)
}

// 博客查询相关接口
func Inner_blog_RawBlogsByAuthType(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	authType, err := getIntParam(arguments, "authType")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawBlogsByAuthType(account, authType)
}

func Inner_blog_RawBlogsByTag(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	tag, err := getStringParam(arguments, "tag")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawBlogsByTag(account, tag)
}

func Inner_blog_RawBlogMetadata(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	title, err := getStringParam(arguments, "title")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawBlogMetadata(account, title)
}

func Inner_blog_RawRecentActiveBlog(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawRecentActiveBlog(account)
}

func Inner_blog_RawMonthlyCreationTrend(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawMonthlyCreationTrend(account)
}

func Inner_blog_RawSearchBlogContent(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	keyword, err := getStringParam(arguments, "keyword")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawSearchBlogContent(account, keyword)
}

// 锻炼相关接口
func Inner_blog_RawExerciseDetailedStats(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawExerciseDetailedStats(account)
}

func Inner_blog_RawRecentExerciseRecords(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	days, err := getIntParam(arguments, "days")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawRecentExerciseRecords(account, days)
}

// ============================================================================
// TodoList 模块工具函数
// ============================================================================

func Inner_blog_RawGetTodosByDate(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, err := getStringParam(arguments, "date")
	if err != nil {
		return errorJSON(err.Error())
	}
	return statistics.RawGetTodosByDate(account, date)
}

func Inner_blog_RawGetTodosRange(arguments map[string]interface{}) string {
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
	return statistics.RawGetTodosRange(account, startDate, endDate)
}

func Inner_blog_RawAddTodo(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
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
	return statistics.RawAddTodo(account, date, content, hours, minutes, urgency, importance)
}

func Inner_blog_RawToggleTodo(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
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
	return statistics.RawToggleTodo(account, date, id)
}

func Inner_blog_RawDeleteTodo(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
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
	return statistics.RawDeleteTodo(account, date, id)
}

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

func RegisterCallBack(name string, callback func(arguments map[string]interface{}) string) {
	callBacks[name] = callback
}

func CallInnerTools(name string, arguments map[string]interface{}) string {
	callback, ok := callBacks[name]
	if !ok {
		return "Error NOT find callback: " + name
	}

	tool_result := callback(arguments)
	prompt, ok := getInnerToolsPrompt(name)
	if ok {
		return fmt.Sprintf("%s /n/n %s", tool_result, prompt)
	} else {
		return tool_result
	}
}

func getInnerToolsPrompt(name string) (string, bool) {
	prompt, ok := callBacksPrompt[name]
	return prompt, ok
}

func RegisterCallBackPrompt(name string, prompt string) {
	callBacksPrompt[name] = prompt
}

func RegisterInnerTools() {

	// 原有接口
	RegisterCallBack("RawAllBlogName", Inner_blog_RawAllBlogName)
	RegisterCallBack("RawGetBlogData", Inner_blog_RawGetBlogData)
	RegisterCallBack("RawAllCommentData", Inner_blog_RawAllCommentData)
	RegisterCallBack("RawCommentData", Inner_blog_RawCommentData)
	RegisterCallBack("RawAllBlogNameByDate", Inner_blog_RawAllBlogNameByDate)
	RegisterCallBack("RawAllBlogNameByDateRange", Inner_blog_RawAllBlogNameByDateRange)
	RegisterCallBack("RawAllBlogNameByDateRangeCount", Inner_blog_RawAllBlogNameByDateRangeCount)
	RegisterCallBack("RawGetBlogDataByDate", Inner_blog_RawGetBlogDataByDate)
	RegisterCallBack("RawCurrentDate", Inner_blog_RawCurrentDate)
	RegisterCallBack("RawCurrentTime", Inner_blog_RawCurrentTime)
	RegisterCallBack("RawAllBlogCount", Inner_blog_RawAllBlogCount)
	RegisterCallBack("RawAllDiaryCount", Inner_blog_RawAllDiaryCount)
	RegisterCallBack("RawAllExerciseCount", Inner_blog_RawAllExerciseCount)
	RegisterCallBack("RawAllExerciseTotalMinutes", Inner_blog_RawAllExerciseTotalMinutes)
	RegisterCallBack("RawAllExerciseDistance", Inner_blog_RawAllExerciseDistance)
	RegisterCallBack("RawAllExerciseCalories", Inner_blog_RawAllExerciseCalories)
	RegisterCallBack("RawAllDiaryContent", Inner_blog_RawAllDiaryContent)
	RegisterCallBack("RawCurrentDiaryContent", Inner_blog_RawCurrentDiaryContent)
	RegisterCallBack("RawGetBlogByTitleMatch", Inner_blog_RawGetBlogByTitleMatch)

	// 新增扩展接口 - 统计类
	RegisterCallBack("RawBlogStatistics", Inner_blog_RawBlogStatistics)
	RegisterCallBack("RawAccessStatistics", Inner_blog_RawAccessStatistics)
	RegisterCallBack("RawTopAccessedBlogs", Inner_blog_RawTopAccessedBlogs)
	RegisterCallBack("RawRecentAccessedBlogs", Inner_blog_RawRecentAccessedBlogs)
	RegisterCallBack("RawEditStatistics", Inner_blog_RawEditStatistics)
	RegisterCallBack("RawTagStatistics", Inner_blog_RawTagStatistics)
	RegisterCallBack("RawCommentStatistics", Inner_blog_RawCommentStatistics)
	RegisterCallBack("RawContentStatistics", Inner_blog_RawContentStatistics)

	// 新增扩展接口 - 查询类
	RegisterCallBack("RawBlogsByAuthType", Inner_blog_RawBlogsByAuthType)
	RegisterCallBack("RawBlogsByTag", Inner_blog_RawBlogsByTag)
	RegisterCallBack("RawBlogMetadata", Inner_blog_RawBlogMetadata)
	RegisterCallBack("RawRecentActiveBlog", Inner_blog_RawRecentActiveBlog)
	RegisterCallBack("RawMonthlyCreationTrend", Inner_blog_RawMonthlyCreationTrend)
	RegisterCallBack("RawSearchBlogContent", Inner_blog_RawSearchBlogContent)

	// 新增扩展接口 - 锻炼类
	RegisterCallBack("RawExerciseDetailedStats", Inner_blog_RawExerciseDetailedStats)
	RegisterCallBack("RawRecentExerciseRecords", Inner_blog_RawRecentExerciseRecords)

	// 新增接口 - 获取每日任务
	RegisterCallBack("RawGetCurrentTask", Inner_blog_RawGetCurrentTask)
	RegisterCallBack("RawGetCurrentTaskByDate", Inner_blog_RawGetCurrentTaskByDate)
	RegisterCallBack("RawGetCurrentTaskByRageDate", Inner_blog_RawGetCurrentTaskByRageDate)

	// 新增接口 - 创建博客
	RegisterCallBack("RawCreateBlog", Inner_blog_RawCreateBlog)
	RegisterCallBackPrompt("RawCreateBlog", "完成创建后返回博客链接格式为[title](/get?blogname=title)")

	// ============================================================================
	// 新增模块工具 - TodoList
	// ============================================================================
	RegisterCallBack("RawGetTodosByDate", Inner_blog_RawGetTodosByDate)
	RegisterCallBack("RawGetTodosRange", Inner_blog_RawGetTodosRange)
	RegisterCallBack("RawAddTodo", Inner_blog_RawAddTodo)
	RegisterCallBack("RawToggleTodo", Inner_blog_RawToggleTodo)
	RegisterCallBack("RawDeleteTodo", Inner_blog_RawDeleteTodo)

	// 新增模块工具 - Exercise
	RegisterCallBack("RawGetExerciseByDate", Inner_blog_RawGetExerciseByDate)
	RegisterCallBack("RawGetExerciseRange", Inner_blog_RawGetExerciseRange)
	RegisterCallBack("RawAddExercise", Inner_blog_RawAddExercise)
	RegisterCallBack("RawGetExerciseStats", Inner_blog_RawGetExerciseStats)

	// 新增模块工具 - Reading
	RegisterCallBack("RawGetAllBooks", Inner_blog_RawGetAllBooks)
	RegisterCallBack("RawGetBooksByStatus", Inner_blog_RawGetBooksByStatus)
	RegisterCallBack("RawGetReadingStats", Inner_blog_RawGetReadingStats)
	RegisterCallBack("RawUpdateReadingProgress", Inner_blog_RawUpdateReadingProgress)
	RegisterCallBack("RawGetBookNotes", Inner_blog_RawGetBookNotes)

	// 新增模块工具 - YearPlan
	RegisterCallBack("RawGetMonthGoal", Inner_blog_RawGetMonthGoal)
	RegisterCallBack("RawGetYearGoals", Inner_blog_RawGetYearGoals)
	RegisterCallBack("RawAddYearTask", Inner_blog_RawAddYearTask)
	RegisterCallBack("RawUpdateYearTask", Inner_blog_RawUpdateYearTask)

	// 新增模块工具 - TaskBreakdown
	RegisterCallBack("RawGetAllComplexTasks", Inner_blog_RawGetAllComplexTasks)
	RegisterCallBack("RawGetComplexTasksByStatus", Inner_blog_RawGetComplexTasksByStatus)
	RegisterCallBack("RawGetComplexTaskStats", Inner_blog_RawGetComplexTaskStats)
	RegisterCallBack("RawCreateComplexTask", Inner_blog_RawCreateComplexTask)

	// 新增模块工具 - Web 网页访问
	RegisterCallBack("FetchWebPage", Inner_web_FetchWebPage)
	RegisterCallBack("WebSearch", Inner_web_WebSearch)
	RegisterCallBackPrompt("FetchWebPage", "返回网页的纯文本内容，用于获取网络实时信息")
	RegisterCallBackPrompt("WebSearch", "返回搜索结果列表，包含标题、URL和摘要，用于搜索互联网信息")
}

func GetInnerMCPTools(toolNameMapping map[string]string) []LLMTool {
	/*
			 Function正确格式如下
			 {
			  "type":"function",
			  "function":{
			   "name":"write_file",
			   "description":". Only works within allowed directories.",
		       "parameters":
		 	    {
		 		  "additionalProperties":false,
		 		  "properties":{
		 			"content":{"type":"string"},
		 			"path":{"type":"string"}
				   },
		 		   "required":["path","content"],
		 		   "type":"object"
				  }
		 	    }
		     }
	*/

	tools := []LLMTool{
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCurrentDiaryContent",
				Description: "获取当天日记数据",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetCurrentTask",
				Description: "获取当天todolist数据,返回json格式",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetCurrentTaskByDate",
				Description: "获取指定日期的todolist数据,返回json格式",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"date":    map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"account", "date"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetCurrentTaskByRageDate",
				Description: "获取指定日期范围的todolist数据,返回json格式",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account":   map[string]string{"type": "string", "description": "账号"},
						"startDate": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
						"endDate":   map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"account", "startDate", "endDate"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllDiaryContent",
				Description: "获取所有日记内容",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetBlogByTitleMatch",
				Description: "通过名称获取blog内容",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"match":   map[string]string{"type": "string", "description": "博客名称匹配字符串，如日记_,匹配日记_开头的博客"},
					},
					"required": []string{"account", "match"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseCalories",
				Description: "获取锻炼总卡路里,单位千卡",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseDistance",
				Description: "获取锻炼总距离,单位公里",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseTotalMinutes",
				Description: "获取锻炼总时长,单位分钟",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllDiaryCount",
				Description: "获取日记数量",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseCount",
				Description: "获取锻炼次数",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogName",
				Description: "获取所有blog名称,以空格分割",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetBlogData",
				Description: "通过名称获取blog内容",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"title":   map[string]string{"type": "string", "description": "blog名称"},
					},
					"required": []string{"account", "title"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetBlogDataByDate",
				Description: "根据日期获取blog内容,如2025-01-01的所有博客",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"date":    map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"account", "date"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllCommentData",
				Description: "通过名称获取comment内容",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"title":   map[string]string{"type": "string", "description": "comment名称"},
					},
					"required": []string{"account", "title"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogNameByDateRange",
				Description: "通过日期范围获取blog内容,如2025-01-01到2025-02-01之间的博客",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account":   map[string]string{"type": "string", "description": "账号"},
						"startDate": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
						"endDate":   map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"account", "startDate", "endDate"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogNameByDateRangeCount",
				Description: "通过日期范围获取blog数量,如2025-01-01到2025-02-01之间的博客数量",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account":   map[string]string{"type": "string", "description": "账号"},
						"startDate": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
						"endDate":   map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"account", "startDate", "endDate"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogCount",
				Description: "获取blog数量",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCurrentDate",
				Description: "获取当前日期",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCurrentTime",
				Description: "获取当前时间",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},

		// =================================== 新增扩展工具 =========================================

		// 统计类工具
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawBlogStatistics",
				Description: "获取博客详细统计信息,包括总数、权限分布、时间统计等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAccessStatistics",
				Description: "获取博客访问统计信息,包括总访问量、今日/周/月访问等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawTopAccessedBlogs",
				Description: "获取热门博客列表(前10名),按访问量排序",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawRecentAccessedBlogs",
				Description: "获取最近访问的博客列表,按访问时间排序",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawEditStatistics",
				Description: "获取博客编辑统计信息,包括编辑次数、频率等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawTagStatistics",
				Description: "获取标签统计信息,包括标签总数和热门标签排行",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCommentStatistics",
				Description: "获取评论统计信息,包括评论总数、活跃度等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawContentStatistics",
				Description: "获取内容统计信息,包括字符数、文章长度分布等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCreateBlog",
				Description: "创建新博客",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account":  map[string]string{"type": "string", "description": "账号"},
						"title":    map[string]string{"type": "string", "description": "博客标题"},
						"content":  map[string]string{"type": "string", "description": "博客内容"},
						"tags":     map[string]string{"type": "string", "description": "标签,多个标签用|分隔"},
						"authType": map[string]string{"type": "number", "description": "权限类型:1=私有,2=公开,4=加密,8=协作,16=日记"},
						"encrypt":  map[string]string{"type": "number", "description": "是否加密:0=否,1=是"},
					},
					"required": []string{"account", "title", "content", "tags", "authType"},
				},
			},
		},

		// 查询类工具
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawBlogsByAuthType",
				Description: "按权限类型获取博客列表。权限类型:1=私有,2=公开,4=加密,8=协作,16=日记",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"authType": map[string]interface{}{
							"type":        "number",
							"description": "权限类型数值:1=私有,2=公开,4=加密,8=协作,16=日记",
						},
					},
					"required": []string{"account", "authType"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawBlogsByTag",
				Description: "按标签获取博客列表",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"tag":     map[string]string{"type": "string", "description": "要查询的标签名称"},
					},
					"required": []string{"account", "tag"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawBlogMetadata",
				Description: "获取指定博客的元数据信息(不包含内容),如创建时间、访问次数等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"title":   map[string]string{"type": "string", "description": "博客标题"},
					},
					"required": []string{"account", "title"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawRecentActiveBlog",
				Description: "获取近期活跃博客列表(近7天有访问或修改的博客)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawMonthlyCreationTrend",
				Description: "获取博客月度创建趋势统计,显示每月创建的博客数量",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawSearchBlogContent",
				Description: "在博客标题和内容中搜索关键词,返回匹配的博客列表",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"keyword": map[string]string{"type": "string", "description": "要搜索的关键词"},
					},
					"required": []string{"account", "keyword"},
				},
			},
		},
		// 锻炼类工具
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawExerciseDetailedStats",
				Description: "获取锻炼详细统计信息,包括总次数、时长、卡路里、类型分布等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawRecentExerciseRecords",
				Description: "获取近期锻炼记录,可指定天数范围",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"days": map[string]interface{}{
							"type":        "number",
							"description": "要查询的天数,如7表示最近7天",
						},
					},
					"required": []string{"account", "days"},
				},
			},
		},

		// =================================== TodoList 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetTodosByDate", Description: "获取指定日期的待办列表,返回JSON格式", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期格式为2026-01-01"}}, "required": []string{"account", "date"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetTodosRange", Description: "获取日期范围内的待办列表", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "startDate": map[string]string{"type": "string", "description": "起始日期"}, "endDate": map[string]string{"type": "string", "description": "结束日期"}}, "required": []string{"account", "startDate", "endDate"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddTodo", Description: "添加待办事项。urgency/importance: 1=最高 2=中等 3=最低", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期"}, "content": map[string]string{"type": "string", "description": "待办内容"}, "hours": map[string]interface{}{"type": "number", "description": "预计小时数"}, "minutes": map[string]interface{}{"type": "number", "description": "预计分钟数"}, "urgency": map[string]interface{}{"type": "number", "description": "紧急度1-3"}, "importance": map[string]interface{}{"type": "number", "description": "重要度1-3"}}, "required": []string{"account", "date", "content"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawToggleTodo", Description: "切换待办事项的完成状态", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期"}, "id": map[string]string{"type": "string", "description": "待办ID"}}, "required": []string{"account", "date", "id"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawDeleteTodo", Description: "删除待办事项", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期"}, "id": map[string]string{"type": "string", "description": "待办ID"}}, "required": []string{"account", "date", "id"}}}},

		// =================================== Exercise 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetExerciseByDate", Description: "获取指定日期的运动记录", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期"}}, "required": []string{"account", "date"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetExerciseRange", Description: "获取日期范围内的运动记录", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "startDate": map[string]string{"type": "string", "description": "起始日期"}, "endDate": map[string]string{"type": "string", "description": "结束日期"}}, "required": []string{"account", "startDate", "endDate"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddExercise", Description: "添加运动记录", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期"}, "name": map[string]string{"type": "string", "description": "运动名称"}, "exerciseType": map[string]string{"type": "string", "description": "运动类型如跑步/游泳/力量训练"}, "duration": map[string]interface{}{"type": "number", "description": "时长(分钟)"}, "intensity": map[string]string{"type": "string", "description": "强度:low/medium/high"}, "calories": map[string]interface{}{"type": "number", "description": "卡路里"}, "notes": map[string]string{"type": "string", "description": "备注"}}, "required": []string{"account", "date", "name", "exerciseType", "duration"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetExerciseStats", Description: "获取运动统计数据,可指定天数", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "days": map[string]interface{}{"type": "number", "description": "统计天数,默认7天"}}, "required": []string{"account"}}}},

		// =================================== Reading 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetAllBooks", Description: "获取所有书籍列表(含状态、作者、页数)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetBooksByStatus", Description: "按状态筛选书籍。status: reading/completed/want-to-read/paused", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "status": map[string]string{"type": "string", "description": "状态:reading/completed/want-to-read/paused"}}, "required": []string{"account", "status"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetReadingStats", Description: "获取阅读统计信息(总数、各状态数量等)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateReadingProgress", Description: "更新阅读进度(当前页数和笔记)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "bookID": map[string]string{"type": "string", "description": "书籍ID"}, "currentPage": map[string]interface{}{"type": "number", "description": "当前页数"}, "notes": map[string]string{"type": "string", "description": "阅读笔记"}}, "required": []string{"account", "bookID", "currentPage"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetBookNotes", Description: "获取指定书籍的读书笔记", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "bookID": map[string]string{"type": "string", "description": "书籍ID"}}, "required": []string{"account", "bookID"}}}},

		// =================================== YearPlan 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetMonthGoal", Description: "获取指定月份的目标和任务", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "year": map[string]interface{}{"type": "number", "description": "年份如2026"}, "month": map[string]interface{}{"type": "number", "description": "月份1-12"}}, "required": []string{"account", "year", "month"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetYearGoals", Description: "获取指定年份所有月度目标", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "year": map[string]interface{}{"type": "number", "description": "年份"}}, "required": []string{"account", "year"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddYearTask", Description: "添加年度计划任务到指定月份", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "year": map[string]interface{}{"type": "number", "description": "年份"}, "month": map[string]interface{}{"type": "number", "description": "月份"}, "title": map[string]string{"type": "string", "description": "任务标题"}, "description": map[string]string{"type": "string", "description": "任务描述"}, "priority": map[string]string{"type": "string", "description": "优先级:highest/high/medium/low/lowest"}, "dueDate": map[string]string{"type": "string", "description": "截止日期"}}, "required": []string{"account", "year", "month", "title"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateYearTask", Description: "更新年度计划任务状态", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "year": map[string]interface{}{"type": "number", "description": "年份"}, "month": map[string]interface{}{"type": "number", "description": "月份"}, "taskID": map[string]string{"type": "string", "description": "任务ID"}, "status": map[string]string{"type": "string", "description": "新状态:planning/in-progress/completed"}}, "required": []string{"account", "year", "month", "taskID", "status"}}}},

		// =================================== TaskBreakdown 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetAllComplexTasks", Description: "获取所有复杂任务列表(含状态、优先级、进度)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetComplexTasksByStatus", Description: "按状态筛选复杂任务。status: planning/in-progress/completed/paused", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "status": map[string]string{"type": "string", "description": "任务状态"}}, "required": []string{"account", "status"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetComplexTaskStats", Description: "获取复杂任务统计信息(总数、完成率等)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawCreateComplexTask", Description: "创建新的复杂任务", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "title": map[string]string{"type": "string", "description": "任务标题"}, "description": map[string]string{"type": "string", "description": "任务描述"}, "priority": map[string]string{"type": "string", "description": "优先级:highest/high/medium/low/lowest"}, "startDate": map[string]string{"type": "string", "description": "开始日期"}, "endDate": map[string]string{"type": "string", "description": "结束日期"}}, "required": []string{"account", "title"}}}},

		// =================================== 定时提醒工具 =========================================
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.CreateReminder",
				Description: "创建定时提醒任务。可以设置间隔时间和重复次数。例如每分钟提醒一次。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account":  map[string]string{"type": "string", "description": "账号"},
						"title":    map[string]string{"type": "string", "description": "提醒标题"},
						"message":  map[string]string{"type": "string", "description": "提醒内容消息"},
						"interval": map[string]interface{}{"type": "number", "description": "间隔秒数，如60表示每分钟提醒一次"},
						"repeat":   map[string]interface{}{"type": "number", "description": "重复次数，-1表示无限重复"},
					},
					"required": []string{"account", "title", "message", "interval"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.ListReminders",
				Description: "列出当前用户的所有定时提醒",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.DeleteReminder",
				Description: "删除指定的定时提醒",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"id":      map[string]string{"type": "string", "description": "提醒ID"},
					},
					"required": []string{"account", "id"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.SendNotification",
				Description: "立即发送一条通知消息给用户",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"message": map[string]string{"type": "string", "description": "通知消息内容"},
					},
					"required": []string{"account", "message"},
				},
			},
		},

		// =================================== 报告生成工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.GenerateReport", Description: "生成报告(日报/周报/月报)。报告包含待办、运动、阅读、任务等数据的AI分析，自动保存为博客并推送通知", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "type": map[string]string{"type": "string", "description": "报告类型: daily/weekly/monthly"}}, "required": []string{"account", "type"}}}},

		// =================================== 模型管理工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.SwitchModel", Description: "切换LLM模型提供者。可选: deepseek/openai/qwen 或其他已配置的provider", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"provider": map[string]string{"type": "string", "description": "模型提供者名称如deepseek/openai/qwen"}}, "required": []string{"provider"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.GetCurrentModel", Description: "获取当前使用的LLM模型信息和所有可用模型列表", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}}},

		// =================================== 网页访问工具 =========================================
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.FetchWebPage",
				Description: "抓取指定URL网页内容，返回纯文本。用于获取网络上的新闻、文章、数据等实时信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url":       map[string]string{"type": "string", "description": "要抓取的网页完整URL,如https://example.com"},
						"maxLength": map[string]string{"type": "integer", "description": "最大返回字符数,默认5000"},
					},
					"required": []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.WebSearch",
				Description: "搜索互联网，返回搜索结果列表(标题+URL+摘要)。用于查找最新信息、新闻、研究数据等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]string{"type": "string", "description": "搜索关键词"},
						"count": map[string]string{"type": "integer", "description": "返回结果数量,默认5,最大10"},
					},
					"required": []string{"query"},
				},
			},
		},
	}

	// 移除原来在此处的工具名称处理逻辑，保持完整的工具名称（包含Inner_blog前缀）
	// 这样前端可以正确识别服务器名称，而LLM层会在GetAvailableLLMTools中处理名称简化和映射

	return tools
}

// GetInnerMCPToolsProcessed returns inner MCP tools with processed function names
// This applies extractFunctionName to simplify tool names (e.g., Inner_blog.RawCurrentDate -> RawCurrentDate)
// and populates toolNameMapping for CallMCPTool to resolve the original names
func GetInnerMCPToolsProcessed() []LLMTool {
	tools := GetInnerMCPTools(nil)
	processedTools := make([]LLMTool, len(tools))

	for i, tool := range tools {
		processedTools[i] = LLMTool{
			Type: tool.Type,
			Function: LLMFunction{
				Name:        extractFunctionName(tool.Function.Name),
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		}
	}

	return processedTools
}
