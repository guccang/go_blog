package mcp

import "strings"

// blog-agent 对外公开的工具只保留 5 个业务域：
// 博客、锻炼、待办、项目、读书。其余工具即使内部仍保留回调，也不再对外暴露。
var publicToolNames = map[string]struct{}{
	// 博客
	"RawAllBlogName":                 {},
	"RawGetBlogData":                 {},
	"RawAllBlogNameByDate":           {},
	"RawAllBlogNameByDateRange":      {},
	"RawAllBlogNameByDateRangeCount": {},
	"RawGetBlogDataByDate":           {},
	"RawGetBlogByTitleMatch":         {},
	"RawCreateBlog":                  {},
	"RawSearchBlogContent":           {},
	"RawBlogsByAuthType":             {},
	"RawBlogsByTag":                  {},

	// TodoList
	"RawGetTodosByDate": {},
	"RawGetTodosRange":  {},
	"RawAddTodo":        {},
	"RawToggleTodo":     {},
	"RawDeleteTodo":     {},
	"RawUpdateTodo":     {},

	// Exercise
	"RawGetExerciseByDate":     {},
	"RawGetExerciseRange":      {},
	"RawAddExercise":           {},
	"RawGetExerciseStats":      {},
	"RawToggleExercise":        {},
	"RawDeleteExercise":        {},
	"RawUpdateExercise":        {},
	"RawRecentExerciseRecords": {},

	// Reading
	"RawGetAllBooks":           {},
	"RawGetBooksByStatus":      {},
	"RawGetReadingStats":       {},
	"RawUpdateReadingProgress": {},
	"RawGetBookNotes":          {},
	"RawAddBook":               {},

	// Project
	"RawCreateProject":          {},
	"RawGetProject":             {},
	"RawListProjects":           {},
	"RawUpdateProject":          {},
	"RawDeleteProject":          {},
	"RawAddProjectGoal":         {},
	"RawUpdateProjectGoal":      {},
	"RawDeleteProjectGoal":      {},
	"RawAddProjectOKR":          {},
	"RawUpdateProjectOKR":       {},
	"RawDeleteProjectOKR":       {},
	"RawUpdateProjectKeyResult": {},
	"RawGetProjectSummary":      {},
}

func normalizePublicToolName(toolName string) string {
	if idx := strings.LastIndex(toolName, "."); idx >= 0 {
		return toolName[idx+1:]
	}
	return toolName
}

func isPublicToolName(toolName string) bool {
	_, ok := publicToolNames[normalizePublicToolName(toolName)]
	return ok
}

func filterPublicInnerTools(tools []LLMTool) []LLMTool {
	filtered := make([]LLMTool, 0, len(tools))
	for _, tool := range tools {
		if isPublicToolName(tool.Function.Name) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
