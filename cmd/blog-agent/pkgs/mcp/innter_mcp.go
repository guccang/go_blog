package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 提供内部mcp接口,接口名称为Inner_blog.xxx
var callBacks = make(map[string]func(arguments map[string]interface{}) string)
var callBacksPrompt = make(map[string]string)

// 当前请求 ID（用于 delegation token 上下文）
// 在 HTTP API 调用时设置为实际请求 ID，LLM 调用时为 "llm-request"
var currentRequestID = "llm-request"

// SetCurrentRequestID 设置当前请求 ID
func SetCurrentRequestID(requestID string) {
	currentRequestID = requestID
}

// GetCurrentRequestID 获取当前请求 ID
func GetCurrentRequestID() string {
	return currentRequestID
}

// ValidateAccountParam 验证 account 参数
// 如果存在有效的 delegation token，返回授权的账户；否则返回请求的账户
// 工具函数应该使用此函数来验证 account 参数
func ValidateAccountParam(requestedAccount string) (string, error) {
	return ValidateAccountAccess(currentRequestID, requestedAccount)
}

// ============================================================================
// 安全参数提取辅助函数
// ============================================================================

// getStringParam 安全提取字符串参数
func getStringParam(arguments map[string]interface{}, key string) (string, error) {
	val, ok := arguments[key]
	if !ok {
		return "", fmt.Errorf("missing param: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("param %s should be string", key)
	}
	return str, nil
}

// getIntParam ???????????? (JSON????????loat64)
func getIntParam(arguments map[string]interface{}, key string) (int, error) {
	val, ok := arguments[key]
	if !ok {
		return 0, fmt.Errorf("missing param: %s", key)
	}
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("param %s should be number", key)
	}
}

// getOptionalIntParam ????????????????
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

func getFloatParam(arguments map[string]interface{}, key string) (float64, error) {
	val, ok := arguments[key]
	if !ok {
		return 0, fmt.Errorf("缂哄皯鍙傛暟: %s", key)
	}
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("鍙傛暟绫诲瀷閿欒: %s 搴斾负鏁板瓧", key)
	}
}

func getOptionalFloatParam(arguments map[string]interface{}, key string, defaultVal float64) float64 {
	val, ok := arguments[key]
	if !ok {
		return defaultVal
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return defaultVal
	}
}

// errorJSON 返回统一信封格式的错误消息
func errorJSON(msg string) string {
	escaped, _ := json.Marshal(msg)
	return `{"ok":false,"error":` + string(escaped) + `}`
}

// wrapResult 将工具原始返回值包装为统一信封格式
// 自动检测 statistics 层返回的 {"error":"..."} 并转为错误信封
func wrapResult(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return `{"ok":true,"data":""}`
	}
	// 检测 statistics 层返回的 {"error":"..."}
	if strings.HasPrefix(trimmed, `{"error"`) {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(trimmed), &e) == nil && e.Error != "" {
			return errorJSON(e.Error)
		}
	}
	// 检测 "Error " 前缀的旧式错误
	if strings.HasPrefix(trimmed, "Error ") {
		return errorJSON(trimmed)
	}
	// 成功：如果是合法 JSON 则内嵌，否则包装为字符串
	if json.Valid([]byte(trimmed)) {
		return `{"ok":true,"data":` + trimmed + `}`
	}
	escaped, _ := json.Marshal(trimmed)
	return `{"ok":true,"data":` + string(escaped) + `}`
}

// ============================================================================
// Tool function implementations are in separate domain files:
//   inner_blog_tools.go      - Blog core operations and statistics
//   inner_todo_tools.go      - TodoList module
//   inner_exercise_tools.go  - Exercise module
//   inner_reading_tools.go   - Reading module
//   inner_yearplan_tasks_tools.go - YearPlan and TaskBreakdown modules
// ============================================================================

func RegisterCallBack(name string, callback func(arguments map[string]interface{}) string) {
	callBacks[name] = callback
}

func CallInnerTools(name string, arguments map[string]interface{}) string {
	return CallInnerToolsWithRequestID(name, arguments, currentRequestID)
}

func CallInnerToolsWithRequestID(name string, arguments map[string]interface{}, requestID string) string {
	callback, ok := callBacks[name]
	if !ok {
		return errorJSON("NOT find callback: " + name)
	}

	result := callback(arguments)
	// 注入 prompt 到信封内（不再用 /n/n 拼接破坏 JSON）
	prompt, hasPrompt := getInnerToolsPrompt(name)
	if hasPrompt {
		return injectHint(result, prompt)
	}
	return result
}

// injectHint 将提示信息注入到信封 JSON 中的 hint 字段
func injectHint(envelopeJSON, hint string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(envelopeJSON), &m) == nil {
		m["hint"] = hint
		data, _ := json.Marshal(m)
		return string(data)
	}
	return envelopeJSON
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

	// 新增模块工具 - Web 搜索与抓取
	RegisterCallBack("WebSearch", Inner_blog_WebSearch)
	RegisterCallBack("WebFetch", Inner_blog_WebFetch)

	// 新增模块工具 - TodoList
	RegisterCallBack("RawGetTodosByDate", Inner_blog_RawGetTodosByDate)
	RegisterCallBack("RawGetTodosRange", Inner_blog_RawGetTodosRange)
	RegisterCallBack("RawAddTodo", Inner_blog_RawAddTodo)
	RegisterCallBack("RawToggleTodo", Inner_blog_RawToggleTodo)
	RegisterCallBack("RawDeleteTodo", Inner_blog_RawDeleteTodo)
	RegisterCallBack("RawUpdateTodo", Inner_blog_RawUpdateTodo)

	// 新增模块工具 - Exercise
	RegisterCallBack("RawGetExerciseByDate", Inner_blog_RawGetExerciseByDate)
	RegisterCallBack("RawGetExerciseRange", Inner_blog_RawGetExerciseRange)
	RegisterCallBack("RawAddExercise", Inner_blog_RawAddExercise)
	RegisterCallBack("RawGetExerciseStats", Inner_blog_RawGetExerciseStats)
	RegisterCallBack("RawToggleExercise", Inner_blog_RawToggleExercise)
	RegisterCallBack("RawDeleteExercise", Inner_blog_RawDeleteExercise)
	RegisterCallBack("RawUpdateExercise", Inner_blog_RawUpdateExercise)

	// 新增模块工具 - Reading
	RegisterCallBack("RawGetAllBooks", Inner_blog_RawGetAllBooks)
	RegisterCallBack("RawGetBooksByStatus", Inner_blog_RawGetBooksByStatus)
	RegisterCallBack("RawGetReadingStats", Inner_blog_RawGetReadingStats)
	RegisterCallBack("RawUpdateReadingProgress", Inner_blog_RawUpdateReadingProgress)
	RegisterCallBack("RawGetBookNotes", Inner_blog_RawGetBookNotes)
	RegisterCallBack("RawAddBook", Inner_blog_RawAddBook)

	// 新增模块工具 - Project Management
	RegisterCallBack("RawCreateProject", Inner_blog_RawCreateProject)
	RegisterCallBack("RawGetProject", Inner_blog_RawGetProject)
	RegisterCallBack("RawListProjects", Inner_blog_RawListProjects)
	RegisterCallBack("RawUpdateProject", Inner_blog_RawUpdateProject)
	RegisterCallBack("RawDeleteProject", Inner_blog_RawDeleteProject)
	RegisterCallBack("RawAddProjectGoal", Inner_blog_RawAddProjectGoal)
	RegisterCallBack("RawUpdateProjectGoal", Inner_blog_RawUpdateProjectGoal)
	RegisterCallBack("RawDeleteProjectGoal", Inner_blog_RawDeleteProjectGoal)
	RegisterCallBack("RawAddProjectOKR", Inner_blog_RawAddProjectOKR)
	RegisterCallBack("RawUpdateProjectOKR", Inner_blog_RawUpdateProjectOKR)
	RegisterCallBack("RawDeleteProjectOKR", Inner_blog_RawDeleteProjectOKR)
	RegisterCallBack("RawUpdateProjectKeyResult", Inner_blog_RawUpdateProjectKeyResult)
	RegisterCallBack("RawGetProjectSummary", Inner_blog_RawGetProjectSummary)

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
				Description: "获取当天日记数据。返回str(markdown纯文本)",
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
				Description: "获取当天todolist数据。返回JSON(list,每项含id/content/done字段)",
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
				Description: "获取指定日期的todolist数据。返回JSON(list)",
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
				Description: "获取指定日期范围的todolist数据。返回JSON(dict,key为日期)",
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
				Description: "获取所有日记内容。返回str(纯文本,每篇以'日记_日期:'开头,不是JSON,不可调用.get())",
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
				Description: "通过名称匹配获取blog内容。返回str(markdown纯文本)",
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
				Description: "获取锻炼总卡路里,单位千卡。返回str(数字)",
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
				Description: "获取锻炼总距离,单位公里。返回str(数字)",
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
				Description: "获取锻炼总时长,单位分钟。返回str(数字)",
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
				Description: "获取日记数量。返回str(数字)",
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
				Description: "获取锻炼次数。返回str(数字)",
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
				Description: "获取所有blog名称,以空格分割。返回str",
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
				Description: "通过名称获取blog内容。返回str(markdown纯文本)",
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
				Description: "根据日期获取blog内容,如2025-01-01的所有博客。返回str(空格分隔的标题列表)",
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
				Description: "通过名称获取comment内容。返回str",
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
				Description: "通过日期范围获取blog内容,如2025-01-01到2025-02-01之间的博客。返回str(空格分隔的标题列表)",
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
				Description: "通过日期范围获取blog数量,如2025-01-01到2025-02-01之间的博客数量。返回str(数字)",
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
				Description: "获取blog数量。返回str(数字)",
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
				Description: "获取当前日期。返回str(YYYY-MM-DD格式)",
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
				Description: "获取当前时间。返回str",
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
				Description: "获取博客详细统计信息,包括总数、权限分布、时间统计等。返回str(格式化文本)",
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
				Description: "获取博客访问统计信息,包括总访问量、今日/周/月访问等。返回str(格式化文本)",
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
				Description: "获取热门博客列表(前10名),按访问量排序。返回str(格式化列表)",
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
				Description: "获取最近访问的博客列表,按访问时间排序。返回str(格式化列表)",
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
				Description: "获取博客编辑统计信息,包括编辑次数、频率等。返回str(格式化文本)",
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
				Description: "获取标签统计信息,包括标签总数和热门标签排行。返回str(格式化文本)",
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
				Description: "获取评论统计信息,包括评论总数、活跃度等。返回str(格式化文本)",
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
				Description: "获取内容统计信息,包括字符数、文章长度分布等。返回str(格式化文本)",
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
				Description: "创建新博客。返回str(操作结果)",
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
				Description: "按权限类型获取博客列表。权限类型:1=私有,2=公开,4=加密,8=协作,16=日记。返回str(空格分隔标题)",
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
				Description: "按标签获取博客列表。返回str(空格分隔标题)",
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
				Description: "获取指定博客的元数据信息(不包含内容),如创建时间、访问次数等。返回str(格式化文本)",
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
				Description: "获取近期活跃博客列表(近7天有访问或修改的博客)。返回str(格式化列表)",
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
				Description: "获取博客月度创建趋势统计,显示每月创建的博客数量。返回str(格式化文本)",
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
				Description: "在博客标题和内容中搜索关键词,返回匹配的博客列表。返回str(格式化结果)",
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
				Description: "获取锻炼详细统计信息,包括总次数、时长、卡路里、类型分布等。返回str(格式化文本)",
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
				Description: "获取近期锻炼记录,可指定天数范围。返回str(格式化列表)",
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

		// =================================== Exercise 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetExerciseByDate", Description: "获取指定日期的运动记录。返回JSON(list)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}}, "required": []string{"account", "date"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetExerciseRange", Description: "获取日期范围内的运动记录。返回JSON(list)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "startDate": map[string]string{"type": "string", "description": "起始日期,格式2025-01-01"}, "endDate": map[string]string{"type": "string", "description": "结束日期,格式2025-01-01"}}, "required": []string{"account", "startDate", "endDate"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddExercise", Description: "添加运动记录。返回str(操作结果)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "name": map[string]string{"type": "string", "description": "运动名称"}, "exerciseType": map[string]string{"type": "string", "description": "运动类型如跑步/游泳/力量训练"}, "duration": map[string]interface{}{"type": "number", "description": "时长(分钟)"}, "intensity": map[string]string{"type": "string", "description": "强度:low/medium/high"}, "calories": map[string]interface{}{"type": "number", "description": "卡路里"}, "notes": map[string]string{"type": "string", "description": "备注"}}, "required": []string{"account", "date", "name", "exerciseType", "duration"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetExerciseStats", Description: "获取运动统计数据,可指定天数。返回str(格式化文本)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "days": map[string]interface{}{"type": "number", "description": "统计天数,默认7天"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawToggleExercise", Description: "切换运动记录的完成状态(完成/未完成)。返回JSON({success:true})", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "id": map[string]string{"type": "string", "description": "运动记录ID"}}, "required": []string{"account", "date", "id"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawDeleteExercise", Description: "删除指定的运动记录。返回JSON({success:true})", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "id": map[string]string{"type": "string", "description": "运动记录ID"}}, "required": []string{"account", "date", "id"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateExercise", Description: "修改运动记录信息。返回JSON({success:true})", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "id": map[string]string{"type": "string", "description": "运动记录ID"}, "name": map[string]string{"type": "string", "description": "运动名称"}, "exerciseType": map[string]string{"type": "string", "description": "运动类型如跑步/游泳/力量训练"}, "duration": map[string]interface{}{"type": "number", "description": "时长(分钟)"}, "intensity": map[string]string{"type": "string", "description": "强度:low/medium/high"}, "calories": map[string]interface{}{"type": "number", "description": "卡路里"}, "notes": map[string]string{"type": "string", "description": "备注"}}, "required": []string{"account", "date", "id", "name", "exerciseType", "duration"}}}},

		// =================================== Web 搜索与抓取工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.WebSearch", Description: "搜索互联网(Bing)，返回搜索结果列表(标题/URL/摘要)。返回JSON", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]string{"type": "string", "description": "搜索关键词"}, "count": map[string]interface{}{"type": "number", "description": "结果数量,默认5,最大10"}}, "required": []string{"query"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.WebFetch", Description: "抓取指定URL网页内容，返回纯文本。返回JSON(含url/content/length/truncated)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"url": map[string]string{"type": "string", "description": "要抓取的网页URL"}, "maxLength": map[string]interface{}{"type": "number", "description": "最大返回字符数,默认5000"}}, "required": []string{"url"}}}},

		// =================================== TodoList 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetTodosByDate", Description: "获取指定日期的待办列表。返回JSON(list,每项含id/content/done字段)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}}, "required": []string{"account", "date"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetTodosRange", Description: "获取日期范围内的待办列表。返回JSON(dict,key为日期)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "startDate": map[string]string{"type": "string", "description": "起始日期,格式2025-01-01"}, "endDate": map[string]string{"type": "string", "description": "结束日期,格式2025-01-01"}}, "required": []string{"account", "startDate", "endDate"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddTodo", Description: "添加待办事项。返回JSON(新建的待办项)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "content": map[string]string{"type": "string", "description": "待办内容"}, "hours": map[string]interface{}{"type": "number", "description": "预计小时数"}, "minutes": map[string]interface{}{"type": "number", "description": "预计分钟数"}, "urgency": map[string]interface{}{"type": "number", "description": "紧急度1-4(1最高)"}, "importance": map[string]interface{}{"type": "number", "description": "重要度1-4(1最高)"}}, "required": []string{"account", "date", "content"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawToggleTodo", Description: "切换待办完成状态(完成/未完成)。返回JSON({success:true})", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "id": map[string]string{"type": "string", "description": "待办ID"}}, "required": []string{"account", "date", "id"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawDeleteTodo", Description: "删除待办事项。返回JSON({success:true})", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "id": map[string]string{"type": "string", "description": "待办ID"}}, "required": []string{"account", "date", "id"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateTodo", Description: "修改待办的预计时间。返回JSON({success:true})", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "date": map[string]string{"type": "string", "description": "日期,格式2025-01-01"}, "id": map[string]string{"type": "string", "description": "待办ID"}, "hours": map[string]interface{}{"type": "number", "description": "预计小时数"}, "minutes": map[string]interface{}{"type": "number", "description": "预计分钟数"}}, "required": []string{"account", "date", "id", "hours", "minutes"}}}},

		// =================================== Reading 模块工具 =========================================
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetAllBooks", Description: "获取所有书籍列表(含状态、作者、页数)。返回JSON(list)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetBooksByStatus", Description: "按状态筛选书籍。status: reading/completed/want-to-read/paused。返回JSON(list)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "status": map[string]string{"type": "string", "description": "状态:reading/completed/want-to-read/paused"}}, "required": []string{"account", "status"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetReadingStats", Description: "获取阅读统计信息(总数、各状态数量等)。返回str(格式化文本)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateReadingProgress", Description: "更新阅读进度(当前页数和笔记)。返回str(操作结果)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "bookID": map[string]string{"type": "string", "description": "书籍ID"}, "currentPage": map[string]interface{}{"type": "number", "description": "当前页数"}, "notes": map[string]string{"type": "string", "description": "阅读笔记"}}, "required": []string{"account", "bookID", "currentPage"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetBookNotes", Description: "获取指定书籍的读书笔记。返回str(纯文本)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "bookID": map[string]string{"type": "string", "description": "书籍ID"}}, "required": []string{"account", "bookID"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddBook", Description: "添加新书籍到阅读列表。返回str(操作结果)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "账号"}, "title": map[string]string{"type": "string", "description": "书名"}, "author": map[string]string{"type": "string", "description": "作者"}, "isbn": map[string]string{"type": "string", "description": "ISBN号"}, "publisher": map[string]string{"type": "string", "description": "出版社"}, "publishDate": map[string]string{"type": "string", "description": "出版日期,格式2025-01-01"}, "coverUrl": map[string]string{"type": "string", "description": "封面URL"}, "description": map[string]string{"type": "string", "description": "书籍简介"}, "sourceUrl": map[string]string{"type": "string", "description": "来源URL"}, "totalPages": map[string]interface{}{"type": "number", "description": "总页数"}, "category": map[string]string{"type": "string", "description": "分类,多个用逗号分隔"}, "tags": map[string]string{"type": "string", "description": "标签,多个用逗号分隔"}}, "required": []string{"account", "title"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawCreateProject", Description: "???????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "name": map[string]string{"type": "string", "description": "????"}, "description": map[string]string{"type": "string", "description": "????"}, "status": map[string]string{"type": "string", "description": "?? planning/active/on_hold/completed/cancelled"}, "priority": map[string]string{"type": "string", "description": "??? low/medium/high/urgent"}, "owner": map[string]string{"type": "string", "description": "???"}, "startDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}, "endDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}, "tags": map[string]string{"type": "string", "description": "???????"}}, "required": []string{"account", "name"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetProject", Description: "?????????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}}, "required": []string{"account", "projectID"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawListProjects", Description: "??????????????JSON(list)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "status": map[string]string{"type": "string", "description": "??????"}}, "required": []string{"account"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateProject", Description: "???????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "name": map[string]string{"type": "string", "description": "????"}, "description": map[string]string{"type": "string", "description": "????"}, "status": map[string]string{"type": "string", "description": "??"}, "priority": map[string]string{"type": "string", "description": "???"}, "owner": map[string]string{"type": "string", "description": "???"}, "startDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}, "endDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}, "tags": map[string]string{"type": "string", "description": "???????"}}, "required": []string{"account", "projectID"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawDeleteProject", Description: "???????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}}, "required": []string{"account", "projectID"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddProjectGoal", Description: "?????????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "title": map[string]string{"type": "string", "description": "????"}, "description": map[string]string{"type": "string", "description": "????"}, "status": map[string]string{"type": "string", "description": "?? pending/in_progress/completed/cancelled"}, "priority": map[string]string{"type": "string", "description": "???"}, "progress": map[string]interface{}{"type": "number", "description": "?? 0-100"}, "startDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}, "endDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}}, "required": []string{"account", "projectID", "title"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateProjectGoal", Description: "?????????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "goalID": map[string]string{"type": "string", "description": "??ID"}, "title": map[string]string{"type": "string", "description": "????"}, "description": map[string]string{"type": "string", "description": "????"}, "status": map[string]string{"type": "string", "description": "??"}, "priority": map[string]string{"type": "string", "description": "???"}, "progress": map[string]interface{}{"type": "number", "description": "?? 0-100"}, "startDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}, "endDate": map[string]string{"type": "string", "description": "???? YYYY-MM-DD"}}, "required": []string{"account", "projectID", "goalID", "title", "status", "priority"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawDeleteProjectGoal", Description: "?????????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "goalID": map[string]string{"type": "string", "description": "??ID"}}, "required": []string{"account", "projectID", "goalID"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawAddProjectOKR", Description: "????OKR???JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "objective": map[string]string{"type": "string", "description": "Objective"}, "status": map[string]string{"type": "string", "description": "?? draft/active/at_risk/completed/cancelled"}, "period": map[string]string{"type": "string", "description": "??"}, "progress": map[string]interface{}{"type": "number", "description": "?? 0-100"}}, "required": []string{"account", "projectID", "objective"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateProjectOKR", Description: "????OKR???JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "okrID": map[string]string{"type": "string", "description": "OKR ID"}, "objective": map[string]string{"type": "string", "description": "Objective"}, "status": map[string]string{"type": "string", "description": "??"}, "period": map[string]string{"type": "string", "description": "??"}, "progress": map[string]interface{}{"type": "number", "description": "?? 0-100"}}, "required": []string{"account", "projectID", "okrID", "objective", "status"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawDeleteProjectOKR", Description: "????OKR???JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "okrID": map[string]string{"type": "string", "description": "OKR ID"}}, "required": []string{"account", "projectID", "okrID"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawUpdateProjectKeyResult", Description: "????????????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}, "projectID": map[string]string{"type": "string", "description": "??ID"}, "okrID": map[string]string{"type": "string", "description": "OKR ID"}, "keyResultID": map[string]string{"type": "string", "description": "????ID???????"}, "title": map[string]string{"type": "string", "description": "??????"}, "metricType": map[string]string{"type": "string", "description": "????"}, "targetValue": map[string]interface{}{"type": "number", "description": "???"}, "currentValue": map[string]interface{}{"type": "number", "description": "???"}, "unit": map[string]string{"type": "string", "description": "??"}, "status": map[string]string{"type": "string", "description": "?? pending/in_progress/completed/cancelled"}}, "required": []string{"account", "projectID", "okrID", "title", "metricType", "targetValue"}}}},
		{Type: "function", Function: LLMFunction{Name: "Inner_blog.RawGetProjectSummary", Description: "?????????????JSON(dict)", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"account": map[string]string{"type": "string", "description": "??"}}, "required": []string{"account"}}}},
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
