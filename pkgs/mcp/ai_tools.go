package mcp

import (
	"config"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// AI 增强工具 - 跨模块智能、智能待办、运动教练、阅读伴读
// ============================================================================

// Inner_blog_RawSmartDailySummary 智能每日摘要
// 聚合待办、运动、阅读、年度目标数据，生成结构化摘要供 LLM 分析
func Inner_blog_RawSmartDailySummary(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	date, _ := getStringParam(arguments, "date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var sections []string

	// 1. 待办事项
	todoResult := CallInnerTools("RawGetTodosByDate", map[string]interface{}{
		"account": account,
		"date":    date,
	})
	if todoResult != "" && !strings.Contains(todoResult, "error") {
		sections = append(sections, fmt.Sprintf("## 📋 待办事项 (%s)\n%s", date, todoResult))
	}

	// 2. 运动记录
	exerciseResult := CallInnerTools("RawGetExerciseByDate", map[string]interface{}{
		"account": account,
		"date":    date,
	})
	if exerciseResult != "" && !strings.Contains(exerciseResult, "error") {
		sections = append(sections, fmt.Sprintf("## 💪 运动记录 (%s)\n%s", date, exerciseResult))
	}

	// 3. 阅读进度
	readingResult := CallInnerTools("RawGetBooksByStatus", map[string]interface{}{
		"account": account,
		"status":  "reading",
	})
	if readingResult != "" && !strings.Contains(readingResult, "error") {
		sections = append(sections, fmt.Sprintf("## 📖 正在阅读\n%s", readingResult))
	}

	// 4. 年度目标（当月）
	year := time.Now().Year()
	month := int(time.Now().Month())
	monthGoalResult := CallInnerTools("RawGetMonthGoal", map[string]interface{}{
		"account": account,
		"year":    year,
		"month":   month,
	})
	if monthGoalResult != "" && !strings.Contains(monthGoalResult, "error") {
		sections = append(sections, fmt.Sprintf("## 🎯 本月目标 (%d年%d月)\n%s", year, month, monthGoalResult))
	}

	if len(sections) == 0 {
		return fmt.Sprintf(`{"date":"%s","summary":"暂无数据"}`, date)
	}

	result := map[string]interface{}{
		"date":     date,
		"sections": strings.Join(sections, "\n\n"),
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Inner_blog_RawAutoCarryOverTodos 自动延续未完成待办
// 检查昨日未完成的待办，生成延续建议
func Inner_blog_RawAutoCarryOverTodos(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}

	// 计算昨天日期
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	// 获取昨日待办
	yesterdayTodos := CallInnerTools("RawGetTodosByDate", map[string]interface{}{
		"account": account,
		"date":    yesterday,
	})

	// 获取今日待办
	todayTodos := CallInnerTools("RawGetTodosByDate", map[string]interface{}{
		"account": account,
		"date":    today,
	})

	result := map[string]interface{}{
		"yesterday":       yesterday,
		"today":           today,
		"yesterday_todos": yesterdayTodos,
		"today_todos":     todayTodos,
		"instruction":     "请分析昨日未完成的待办事项，告诉用户哪些需要延续到今天，哪些可以取消。如果用户同意，可以调用 RawAddTodo 添加到今日。",
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Inner_blog_RawTodoGoalAlignment 待办-目标对齐检查
// 检查今日待办是否与年度/月度目标对齐
func Inner_blog_RawTodoGoalAlignment(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}

	date, _ := getStringParam(arguments, "date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 获取今日待办
	todos := CallInnerTools("RawGetTodosByDate", map[string]interface{}{
		"account": account,
		"date":    date,
	})

	// 获取本月目标
	year := time.Now().Year()
	month := int(time.Now().Month())
	monthGoals := CallInnerTools("RawGetMonthGoal", map[string]interface{}{
		"account": account,
		"year":    year,
		"month":   month,
	})

	result := map[string]interface{}{
		"date":        date,
		"todos":       todos,
		"month_goals": monthGoals,
		"instruction": "请对比今日待办与本月目标，分析对齐度。指出哪些待办直接支持月度目标，哪些待办与目标无关，以及有没有被忽略的目标。",
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Inner_blog_RawExerciseCoachAdvice 运动教练建议
// 综合近期运动记录，给出个性化建议
func Inner_blog_RawExerciseCoachAdvice(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}

	days := getOptionalIntParam(arguments, "days", 7)

	// 获取近 N 天运动统计
	exerciseStats := CallInnerTools("RawGetExerciseStats", map[string]interface{}{
		"account": account,
		"days":    days,
	})

	// 获取今日运动
	today := time.Now().Format("2006-01-02")
	todayExercise := CallInnerTools("RawGetExerciseByDate", map[string]interface{}{
		"account": account,
		"date":    today,
	})

	// 获取最近运动记录用于分析部位轮换
	recentRecords := CallInnerTools("RawRecentExerciseRecords", map[string]interface{}{
		"account": account,
		"count":   10,
	})

	result := map[string]interface{}{
		"days":           days,
		"exercise_stats": exerciseStats,
		"today_exercise": todayExercise,
		"recent_records": recentRecords,
		"instruction":    config.GetPrompt(account, "exercise_companion"),
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Inner_blog_RawReadingCompanion 阅读伴读建议
// 综合阅读数据给出建议
func Inner_blog_RawReadingCompanion(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}

	// 获取阅读统计
	readingStats := CallInnerTools("RawGetReadingStats", map[string]interface{}{
		"account": account,
	})

	// 获取正在阅读的书
	readingBooks := CallInnerTools("RawGetBooksByStatus", map[string]interface{}{
		"account": account,
		"status":  "reading",
	})

	// 获取全部书籍（用于推荐）
	allBooks := CallInnerTools("RawGetAllBooks", map[string]interface{}{
		"account": account,
	})

	result := map[string]interface{}{
		"reading_stats": readingStats,
		"reading_books": readingBooks,
		"all_books":     allBooks,
		"instruction":   config.GetPrompt(account, "reading_companion"),
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// Inner_blog_RawSmartDecomposeTodo 智能任务拆解
// 参考 Anthropic 文章的增量执行策略：一次只做一件事
// 将一个复杂待办拆解为多个可独立完成的子任务
func Inner_blog_RawSmartDecomposeTodo(arguments map[string]interface{}) string {
	account, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}

	task, err := getStringParam(arguments, "task")
	if err != nil {
		return errorJSON(err.Error())
	}

	date, _ := getStringParam(arguments, "date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 获取用户现有待办，避免重复
	existingTodos := CallInnerTools("RawGetTodosByDate", map[string]interface{}{
		"account": account,
		"date":    date,
	})

	result := map[string]interface{}{
		"account":        account,
		"date":           date,
		"original_task":  task,
		"existing_todos": existingTodos,
		"instruction":    config.GetPrompt(account, "task_decomposition"),
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}
