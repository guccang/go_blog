package mcp

import (
	"statistics"
)

// 提供内部mcp接口,接口名称为Inner_blog.xxx

func Inner_blog_RawAllBlogData() string {
	return statistics.RawAllBlogData()
}

func Inner_blog_RawBlogData(title string) string {
	return statistics.RawBlogData(title)
}

func Inner_blog_RawAllCommentData() string {
	return statistics.RawAllCommentData()
}

func Inner_blog_RawCommentData(title string) string {
	return statistics.RawCommentData(title)
}

func Inner_blog_RawAllCooperationData() string {
	return statistics.RawAllCooperationData()
}	

func Inner_blog_RawAllBlogDataByDate(date string) string {
	return statistics.RawAllBlogDataByDate(date)
}

func Inner_blog_RawAllBlogDataByDateRange(startDate, endDate string) string {
	return statistics.RawAllBlogDataByDateRange(startDate, endDate)
}

func Inner_blog_RawAllBlogDataByDateRangeCount(startDate, endDate string) int {	
	return statistics.RawAllBlogDataByDateRangeCount(startDate, endDate)
}

func Inner_blog_RawBlogDataByDate(date string) string {
	return statistics.RawBlogDataByDate(date)
}

func getTitle(arguments map[string]interface{}) string {
	title := ""
	for _, v := range arguments {
		title = v.(string)
		break
	}
	return title
}

func CallInnerTools(toolName string, arguments map[string]interface{}) string {
	switch toolName {
	case "RawAllBlogData":
		return Inner_blog_RawAllBlogData()
	case "RawBlogData":
		title := getTitle(arguments)
		data := Inner_blog_RawBlogData(title)
		if  data == ""{
			data = "Error NOT find blog: " + title
		}
		return data
	case "RawAllCommentData":
		return Inner_blog_RawAllCommentData()
	case "RawCommentData":
		title := getTitle(arguments)
		data := Inner_blog_RawCommentData(title)
		if  data == ""{
			data = "Error NOT find comment: " + title
		}
		return data
	case "RawAllCooperationData":
		return Inner_blog_RawAllCooperationData()
	case "RawAllBlogDataByDate":
		return Inner_blog_RawAllBlogDataByDate(arguments["date"].(string))
	case "RawAllBlogDataByDateRange":
		return Inner_blog_RawAllBlogDataByDateRange(arguments["startDate"].(string), arguments["endDate"].(string))
	case "RawAllBlogDataByDateRangeCount":
		return string(Inner_blog_RawAllBlogDataByDateRangeCount(arguments["startDate"].(string), arguments["endDate"].(string)))
	case "RawBlogDataByDate":
		return Inner_blog_RawBlogDataByDate(arguments["date"].(string))
	default:
		return "Error NOT find tool: " + toolName
	}
}


func createParameters(properties map[string]interface{}, required []string) map[string]interface{} {
	return map[string]interface{}{
		"properties": properties,
		"required": required,
		"type": "object",
	}
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
			Type : "function",
			Function: LLMFunction{
				Name: "Inner_blog.RawAllBlogData",
				Description: "获取所有blog名称,以空格分割",
				InputSchema: createParameters(map[string]interface{}{}, []string{}),
			},
		},
		{
			Type : "function",
			Function: LLMFunction{
				Name: "Inner_blog.RawBlogData",
				Description: "通过名称获取blog内容",
				InputSchema: createParameters(map[string]interface{}{
					"title": map[string]string{"type": "string"},
				}, []string{"title"}),
			},
		},
		{
			Type : "function",
			Function: LLMFunction{
				Name:"Inner_blog.RawGetBlogData",
				Description: "通过名称获取blog内容",
				InputSchema: createParameters(map[string]interface{}{
					"title": map[string]string{"type": "string"},
				}, []string{"title"}),
			},
		},
		{
			Type : "function",
			Function: LLMFunction{
				Name:"Inner_blog.RawGetCommentData",
				Description: "通过名称获取comment内容",
				InputSchema: createParameters(map[string]interface{}{
					"title": map[string]string{"type": "string"},
				}, []string{"title"}),
			},
		},
		
		{
			Type : "function",
			Function: LLMFunction{
				Name:"Inner_blog.RawGetCooperationData",
				Description: "通过名称获取cooperation内容",
				InputSchema: createParameters(map[string]interface{}{
					"title": map[string]string{"type": "string"},
				}, []string{"title"}),
			},
		},
		{
			Type : "function",
			Function: LLMFunction{
				Name:"Inner_blog.RawGetBlogDataByDate",
				Description: "通过日期获取blog内容",
				InputSchema: createParameters(map[string]interface{}{
					"date": map[string]string{"type": "string"},
				}, []string{"date"}),
			},
		},
		{
			Type : "function",
			Function: LLMFunction{
				Name:"Inner_blog.RawGetBlogDataByDateRange",
				Description: "通过日期范围获取blog内容",
				InputSchema: createParameters(map[string]interface{}{
					"startDate": map[string]string{"type": "date"},
					"endDate": map[string]string{"type": "date"},
				}, []string{"startDate", "endDate"}),
			},
		},
	}

	return tools
}


