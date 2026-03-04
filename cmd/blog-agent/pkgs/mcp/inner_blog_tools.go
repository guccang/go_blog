package mcp

import (
	"statistics"
	"strconv"
)

// ============================================================================
// 内部工具函数 - 博客核心操作
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
