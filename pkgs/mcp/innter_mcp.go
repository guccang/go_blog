package mcp

import (
	"statistics"
	"strconv"
)

// 提供内部mcp接口,接口名称为Inner_blog.xxx
var callBacks = make(map[string]func(arguments map[string]interface{}) string)

func Inner_blog_RawAllBlogData(arguments map[string]interface{}) string {
	return statistics.RawAllBlogData()
}

func Inner_blog_RawGetBlogData(arguments map[string]interface{}) string {
	title := arguments["title"].(string)
	return statistics.RawGetBlogData(title)
}

func Inner_blog_RawAllCommentData(arguments map[string]interface{}) string {
	return statistics.RawAllCommentData()
}

func Inner_blog_RawCommentData(arguments map[string]interface{}) string {
	title := arguments["title"].(string)
	return statistics.RawCommentData(title)
}

func Inner_blog_RawAllCooperationData(arguments map[string]interface{}) string {
	return statistics.RawAllCooperationData()
}

func Inner_blog_RawAllBlogDataByDate(arguments map[string]interface{}) string {
	date := arguments["date"].(string)
	return statistics.RawAllBlogDataByDate(date)
}

func Inner_blog_RawAllBlogDataByDateRange(arguments map[string]interface{}) string {
	startDate := arguments["startDate"].(string)
	endDate := arguments["endDate"].(string)
	return statistics.RawAllBlogDataByDateRange(startDate, endDate)
}

func Inner_blog_RawAllBlogDataByDateRangeCount(arguments map[string]interface{}) string {
	startDate := arguments["startDate"].(string)
	endDate := arguments["endDate"].(string)
	return string(statistics.RawAllBlogDataByDateRangeCount(startDate, endDate))
}

func Inner_blog_RawGetBlogDataByDate(arguments map[string]interface{}) string {
	date := arguments["date"].(string)
	return statistics.RawGetBlogDataByDate(date)
}

func Inner_blog_RawCurrentDate(arguments map[string]interface{}) string {
	return statistics.RawCurrentDate()
}

func Inner_blog_RawCurrentTime(arguments map[string]interface{}) string {
	return statistics.RawCurrentTime()
}

func Inner_blog_RawAllBlogCount(arguments map[string]interface{}) string {
	// int to string
	return strconv.Itoa(statistics.RawAllBlogCount())
}

func Inner_blog_RawAllDiaryCount(arguments map[string]interface{}) string {
	return strconv.Itoa(statistics.RawAllDiaryCount())
}

func Inner_blog_RawAllExerciseCount(arguments map[string]interface{}) string {
	return strconv.Itoa(statistics.RawAllExerciseCount())
}

func Inner_blog_RawAllExerciseTotalMinutes(arguments map[string]interface{}) string {
	return strconv.Itoa(statistics.RawAllExerciseTotalMinutes())
}

func Inner_blog_RawAllExerciseDistance(arguments map[string]interface{}) string {
	return strconv.Itoa(statistics.RawAllExerciseDistance())
}

func Inner_blog_RawAllExerciseCalories(arguments map[string]interface{}) string {
	return strconv.Itoa(statistics.RawAllExerciseCalories())
}

func Inner_blog_RawAllDiaryContent(arguments map[string]interface{}) string {
	return statistics.RawAllDiaryContent()
}

func Inner_blog_RawGetBlogByTitleMatch(arguments map[string]interface{}) string {
	match := arguments["match"].(string)
	return statistics.RawGetBlogByTitleMatch(match)
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
	RegisterCallBack("RawAllBlogData", Inner_blog_RawAllBlogData)
	RegisterCallBack("RawGetBlogData", Inner_blog_RawGetBlogData)
	RegisterCallBack("RawAllCommentData", Inner_blog_RawAllCommentData)
	RegisterCallBack("RawCommentData", Inner_blog_RawCommentData)
	RegisterCallBack("RawAllCooperationData", Inner_blog_RawAllCooperationData)
	RegisterCallBack("RawAllBlogDataByDate", Inner_blog_RawAllBlogDataByDate)
	RegisterCallBack("RawAllBlogDataByDateRange", Inner_blog_RawAllBlogDataByDateRange)
	RegisterCallBack("RawAllBlogDataByDateRangeCount", Inner_blog_RawAllBlogDataByDateRangeCount)
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
	RegisterCallBack("RawGetBlogByTitleMatch", Inner_blog_RawGetBlogByTitleMatch)
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
				Name:        "Inner_blog.RawAllDiaryContent",
				Description: "获取所有日志内容",
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
						"match": map[string]string{"type": "string", "description": "博客名称匹配字符串，如日记_,匹配日记_开头的博客"},
					},
					"required": []string{"match"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseCalories",
				Description: "获取锻炼总卡路里,单位千卡",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseDistance",
				Description: "获取锻炼总距离,单位公里",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseTotalMinutes",
				Description: "获取锻炼总时长,单位分钟",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllDiaryCount",
				Description: "获取日记数量",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllExerciseCount",
				Description: "获取锻炼次数",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogData",
				Description: "获取所有blog名称,以空格分割",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
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
						"title": map[string]string{"type": "string", "description": "blog名称"},
					},
					"required": []string{"title"},
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
						"date": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"date"},
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
						"title": map[string]string{"type": "string", "description": "comment名称"},
					},
					"required": []string{"title"},
				},
			},
		},

		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllCooperationData",
				Description: "通过名称获取cooperation内容",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]string{"type": "string", "description": "cooperation名称"},
					},
					"required": []string{"title"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawGetBlogDataByDate",
				Description: "通过日期获取blog内容,如2025-01-01的所有博客",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"date"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogDataByDateRange",
				Description: "通过日期范围获取blog内容,如2025-01-01到2025-02-01之间的博客",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"startDate": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
						"endDate":   map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"startDate", "endDate"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogDataByDateRangeCount",
				Description: "通过日期范围获取blog数量,如2025-01-01到2025-02-01之间的博客数量",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"startDate": map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
						"endDate":   map[string]string{"type": "string", "description": "日期格式为2025-01-01"},
					},
					"required": []string{"startDate", "endDate"},
				},
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawAllBlogCount",
				Description: "获取blog数量",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCurrentDate",
				Description: "获取当前日期",
			},
		},
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Inner_blog.RawCurrentTime",
				Description: "获取当前时间",
			},
		},
	}

	return tools
}
