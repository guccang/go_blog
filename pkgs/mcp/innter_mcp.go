package mcp

import (
	"fmt"
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
// Tool function implementations are in separate domain files:
//   inner_blog_tools.go      - Blog core operations and statistics
//   inner_todo_tools.go      - TodoList module
//   inner_exercise_tools.go  - Exercise module
//   inner_reading_tools.go   - Reading module
//   inner_yearplan_tasks_tools.go - YearPlan and TaskBreakdown modules
//   web_fetch.go             - Web fetch and search
// ============================================================================

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
	RegisterCallBackPrompt("FetchWebPage", "返回网页的纯文本内容。使用网页内容时必须在输出中标注来源URL，格式: [来源](URL)")
	RegisterCallBackPrompt("WebSearch", "返回搜索结果列表。引用搜索结果时必须在文末添加参考来源链接，格式: ## 参考来源\n- [标题](URL)")

	// ============================================================================
	// AI 增强工具 - 跨模块智能
	// ============================================================================
	RegisterCallBack("SmartDailySummary", Inner_blog_RawSmartDailySummary)
	RegisterCallBackPrompt("SmartDailySummary", "这是用户的当日全面数据摘要，请根据数据给出综合分析和个性化建议")
	RegisterCallBack("AutoCarryOverTodos", Inner_blog_RawAutoCarryOverTodos)
	RegisterCallBackPrompt("AutoCarryOverTodos", "根据昨日未完成+今日待办，帮助用户决定哪些任务需要延续到今天")
	RegisterCallBack("TodoGoalAlignment", Inner_blog_RawTodoGoalAlignment)
	RegisterCallBackPrompt("TodoGoalAlignment", "对比待办与月度目标，分析对齐度并给出建议")
	RegisterCallBack("ExerciseCoachAdvice", Inner_blog_RawExerciseCoachAdvice)
	RegisterCallBackPrompt("ExerciseCoachAdvice", "像私人教练一样分析运动数据并给出今日训练建议")
	RegisterCallBack("ReadingCompanion", Inner_blog_RawReadingCompanion)
	RegisterCallBackPrompt("ReadingCompanion", "像阅读伙伴一样分析阅读数据并给出建议和鼓励")
	RegisterCallBack("SmartDecomposeTodo", Inner_blog_RawSmartDecomposeTodo)
	RegisterCallBackPrompt("SmartDecomposeTodo", "将复杂任务拆解为可独立完成的子任务，确认后添加到待办")

	// 注意: CodeGen 工具的回调由 agent 包通过 mcp.RegisterCallBack() 注册
	// 工具名: CodegenListProjects, CodegenCreateProject, CodegenStartSession,
	//         CodegenSendMessage, CodegenGetStatus, CodegenStopSession
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
				Description: "创建定时提醒任务。支持 cron 表达式(如'0 0 21 * * *'每天21:00)和间隔秒数两种模式。可设置 ai_query 让 AI 定时执行查询。所有提醒会持久化保存，重启不丢失",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account":     map[string]string{"type": "string", "description": "账号"},
						"title":       map[string]string{"type": "string", "description": "提醒标题"},
						"message":     map[string]string{"type": "string", "description": "提醒内容消息"},
						"cron":        map[string]string{"type": "string", "description": "Cron表达式(秒级),如: '0 0 21 * * *'每天21:00, '0 0 9 * * 1'每周一9:00, '@every 30m'每30分钟"},
						"interval":    map[string]interface{}{"type": "number", "description": "间隔秒数(与cron二选一),如60表示每分钟"},
						"repeat":      map[string]interface{}{"type": "number", "description": "重复次数,-1表示无限重复"},
						"ai_query":    map[string]string{"type": "string", "description": "AI查询,设置后定时执行该查询并推送结果"},
						"save_result": map[string]interface{}{"type": "boolean", "description": "是否保存AI查询结果到博客"},
					},
					"required": []string{"account", "title"},
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

		// AI 定时任务
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.CreateAIScheduledTask", Description: "创建AI定时任务,按cron表达式定时执行AI查询并推送结果。例如'每周一早上9点分析运动数据'。重启后自动恢复", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "title": map[string]string{"type": "string", "description": "任务标题"}, "cron": map[string]string{"type": "string", "description": "Cron表达式,如'0 0 9 * * 1'每周一9:00, '0 0 21 * * *'每天21:00"}, "ai_query": map[string]string{"type": "string", "description": "要定时执行的AI查询"}, "save_result": map[string]interface{}{"type": "boolean", "description": "是否保存结果到博客"}}, "required": []string{"account", "title", "ai_query"}}}},

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
				Description: "抓取指定URL网页内容，返回纯文本。使用网页内容时必须在输出中标注来源URL，格式: [来源](URL)",
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
				Description: "搜索互联网，返回搜索结果列表(标题+URL+摘要)。引用搜索结果时必须在文末添加参考来源链接，格式: ## 参考来源\n- [标题](URL)",
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

		// =================================== AI 增强工具 =========================================
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.SmartDailySummary",
				Description: "生成智能每日摘要：聚合待办、运动、阅读、年度目标数据，提供跨模块全面分析。当用户问'今天怎么样'、'帮我总结一下'、'今日概览'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"date":    map[string]string{"type": "string", "description": "日期,格式2026-01-01,默认今天"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.AutoCarryOverTodos",
				Description: "检查昨日未完成待办并建议延续到今天。当用户问'昨天有什么没完成'、'帮我整理待办'、'延续任务'时使用",
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
				Name:        "Inner_blog.TodoGoalAlignment",
				Description: "对比今日待办与月度/年度目标的对齐度，分析待办是否在推进目标。当用户问'我的待办和目标对齐吗'、'我今天做的有意义吗'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"date":    map[string]string{"type": "string", "description": "日期,格式2026-01-01,默认今天"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.ExerciseCoachAdvice",
				Description: "AI运动教练：分析近期运动数据，给出个性化训练建议(部位轮换、运动量评估、今日推荐)。当用户问'今天练什么'、'运动建议'、'健身计划'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"days":    map[string]string{"type": "integer", "description": "分析天数,默认7天"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.ReadingCompanion",
				Description: "AI阅读伴读：分析阅读进度、预测完成时间、推荐下一本书。当用户问'阅读建议'、'下一本读什么'、'读书进度'时使用",
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
				Name:        "Inner_blog.SmartDecomposeTodo",
				Description: "智能任务拆解：将复杂待办拆解为3-7个可独立完成的子任务。当用户说'帮我拆解这个任务'、'这个事情太复杂了'、'帮我规划一下'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"task":    map[string]string{"type": "string", "description": "需要拆解的复杂任务描述"},
						"date":    map[string]string{"type": "string", "description": "日期,格式2026-01-01,默认今天"},
					},
					"required": []string{"account", "task"},
				},
			},
		},

		// =================================== AI 技能管理工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.ListAISkills", Description: "列出所有已安装的 AI 技能卡，包含创建新技能的模板。当用户问'我有哪些技能'、'技能列表'时使用", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.CreateAISkill", Description: "创建新的 AI 技能卡。技能卡能让 AI 学会新本领，例如'每次说写周报就自动收集数据生成报告'。当用户说'帮我创建一个技能'、'教你一个新本领'时使用", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "name": map[string]string{"type": "string", "description": "技能名称"}, "description": map[string]string{"type": "string", "description": "一句话描述技能功能"}, "triggers": map[string]string{"type": "string", "description": "触发关键词,逗号分隔,如: 写周报,本周总结"}, "instruction": map[string]string{"type": "string", "description": "详细的技能指令,告诉AI触发时应该怎么做"}, "examples": map[string]string{"type": "string", "description": "可选,示例对话"}}, "required": []string{"account", "name", "instruction"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.ToggleAISkill", Description: "启用或停用指定的 AI 技能。当用户说'停用xxx技能'、'启用xxx'时使用", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "name": map[string]string{"type": "string", "description": "技能名称"}, "active": map[string]string{"type": "boolean", "description": "true=启用, false=停用"}}, "required": []string{"account", "name"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.GetSkillTemplate", Description: "获取 AI 技能卡的创建模板,了解技能卡的格式。当需要创建技能但不确定格式时使用", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}}},

		// =================================== CodeGen 编码助手工具 =========================================
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.CodegenListProjects",
				Description: "列出所有可用的AI编码项目。当用户问'有哪些项目'、'项目列表'、'编码项目'时使用",
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
				Name:        "Inner_blog.CodegenCreateProject",
				Description: "创建一个新的编码项目目录。可以指定在某个远程agent上创建。当用户说'创建项目'、'新建一个项目'、'在xxx机器上创建项目'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号"},
						"name":    map[string]string{"type": "string", "description": "项目名称（英文，无空格）"},
						"agent":   map[string]string{"type": "string", "description": "可选，远程agent名称。指定后在该agent机器上创建项目"},
					},
					"required": []string{"account", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.CodegenStartSession",
				Description: "启动AI编码会话，让AI在指定项目中编写代码。这是一个异步操作，启动后进度会通过微信推送。当用户说'写个程序'、'帮我写代码'、'在xxx项目里开发'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号（即微信用户ID）"},
						"project": map[string]string{"type": "string", "description": "项目名称"},
						"prompt":  map[string]string{"type": "string", "description": "编码需求描述"},
					},
					"required": []string{"account", "project", "prompt"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.CodegenSendMessage",
				Description: "向当前活跃的编码会话追加消息/指令。当用户说'继续编码'、'修改一下'、'再加个功能'时使用（需要先有活跃会话）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号（即微信用户ID）"},
						"prompt":  map[string]string{"type": "string", "description": "追加的消息/指令"},
					},
					"required": []string{"account", "prompt"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.CodegenGetStatus",
				Description: "查看当前编码会话的运行状态。当用户问'编码进度怎么样'、'完成了吗'、'编码状态'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号（即微信用户ID）"},
					},
					"required": []string{"account"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.CodegenStopSession",
				Description: "停止当前正在运行的编码会话。当用户说'停掉编码'、'取消编码'、'停止开发'时使用",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"account": map[string]string{"type": "string", "description": "账号（即微信用户ID）"},
					},
					"required": []string{"account"},
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
