package mcp

import (
	"statistics"
	"strconv"
)

// 提供内部mcp接口,接口名称为Inner_blog.xxx
var callBacks = make(map[string]func(arguments map[string]interface{}) string)

func Inner_blog_RawAllBlogName(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawAllBlogName(account)
}

func Inner_blog_RawGetBlogData(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	title := arguments["title"].(string)
	return statistics.RawGetBlogData(account, title)
}

func Inner_blog_RawAllCommentData(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawAllCommentData(account)
}

func Inner_blog_RawCommentData(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	title := arguments["title"].(string)
	return statistics.RawCommentData(account, title)
}

func Inner_blog_RawAllBlogNameByDate(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	date := arguments["date"].(string)
	return statistics.RawAllBlogNameByDate(account, date)
}

func Inner_blog_RawAllBlogNameByDateRange(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	startDate := arguments["startDate"].(string)
	endDate := arguments["endDate"].(string)
	return statistics.RawAllBlogNameByDateRange(account, startDate, endDate)
}

func Inner_blog_RawAllBlogNameByDateRangeCount(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	startDate := arguments["startDate"].(string)
	endDate := arguments["endDate"].(string)
	return string(statistics.RawAllBlogNameByDateRangeCount(account, startDate, endDate))
}

func Inner_blog_RawGetBlogDataByDate(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	date := arguments["date"].(string)
	return statistics.RawGetBlogDataByDate(account, date)
}

func Inner_blog_RawCurrentDate(arguments map[string]interface{}) string {
	return statistics.RawCurrentDate()
}

func Inner_blog_RawCurrentTime(arguments map[string]interface{}) string {
	return statistics.RawCurrentTime()
}

func Inner_blog_RawAllBlogCount(arguments map[string]interface{}) string {
	// int to string
	account := arguments["account"].(string)
	return strconv.Itoa(statistics.RawAllBlogCount(account))
}

func Inner_blog_RawAllDiaryCount(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return strconv.Itoa(statistics.RawAllDiaryCount(account))
}

func Inner_blog_RawCurrentDiaryContent(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawCurrentDiaryContent(account)
}

func Inner_blog_RawAllExerciseCount(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return strconv.Itoa(statistics.RawAllExerciseCount(account))
}

func Inner_blog_RawAllExerciseTotalMinutes(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return strconv.Itoa(statistics.RawAllExerciseTotalMinutes(account))
}

func Inner_blog_RawAllExerciseDistance(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return strconv.Itoa(statistics.RawAllExerciseDistance(account))
}

func Inner_blog_RawAllExerciseCalories(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return strconv.Itoa(statistics.RawAllExerciseCalories(account))
}

func Inner_blog_RawAllDiaryContent(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawAllDiaryContent(account)
}

func Inner_blog_RawGetBlogByTitleMatch(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	match := arguments["match"].(string)
	return statistics.RawGetBlogByTitleMatch(account, match)
}

func Inner_blog_RawGetCurrentTask(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawGetCurrentTask(account)
}

func Inner_blog_RawGetCurrentTaskByDate(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	date := arguments["date"].(string)
	return statistics.RawGetCurrentTaskByDate(account, date)
}

func Inner_blog_RawGetCurrentTaskByRageDate(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	startDate := arguments["startDate"].(string)
	endDate := arguments["endDate"].(string)
	return statistics.RawGetCurrentTaskByRageDate(account, startDate, endDate)
}

// =================================== 扩展Inner_blog接口 =========================================

// 博客统计相关接口
func Inner_blog_RawBlogStatistics(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawBlogStatistics(account)
}

func Inner_blog_RawAccessStatistics(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawAccessStatistics(account)
}

func Inner_blog_RawTopAccessedBlogs(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawTopAccessedBlogs(account)
}

func Inner_blog_RawRecentAccessedBlogs(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawRecentAccessedBlogs(account)
}

func Inner_blog_RawEditStatistics(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawEditStatistics(account)
}

func Inner_blog_RawTagStatistics(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawTagStatistics(account)
}

func Inner_blog_RawCommentStatistics(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawCommentStatistics(account)
}

func Inner_blog_RawContentStatistics(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawContentStatistics(account)
}

// 博客查询相关接口
func Inner_blog_RawBlogsByAuthType(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	authType := int(arguments["authType"].(float64)) // JSON数字默认为float64
	return statistics.RawBlogsByAuthType(account, authType)
}

func Inner_blog_RawBlogsByTag(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	tag := arguments["tag"].(string)
	return statistics.RawBlogsByTag(account, tag)
}

func Inner_blog_RawBlogMetadata(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	title := arguments["title"].(string)
	return statistics.RawBlogMetadata(account, title)
}

func Inner_blog_RawRecentActiveBlog(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawRecentActiveBlog(account)
}

func Inner_blog_RawMonthlyCreationTrend(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawMonthlyCreationTrend(account)
}

func Inner_blog_RawSearchBlogContent(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	keyword := arguments["keyword"].(string)
	return statistics.RawSearchBlogContent(account, keyword)
}

// 锻炼相关接口
func Inner_blog_RawExerciseDetailedStats(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	return statistics.RawExerciseDetailedStats(account)
}

func Inner_blog_RawRecentExerciseRecords(arguments map[string]interface{}) string {
	account := arguments["account"].(string)
	days := int(arguments["days"].(float64))
	return statistics.RawRecentExerciseRecords(account, days)
}

func RegisterCallBack(name string, callback func(arguments map[string]interface{}) string) {
	callBacks[name] = callback
}

func CallInnerTools(name string, arguments map[string]interface{}) string {
	callback, ok := callBacks[name]
	if !ok {
		return "Error NOT find callback: " + name
	}
	return callback(arguments)
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
	}

	return tools
}
