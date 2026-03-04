package llm

import (
	"config"
	"fmt"
	"mcp"
	log "mylog"
	"strings"
	"time"
)

// ============================================================================
// 用户上下文采集 - 自动注入 System Prompt
// ============================================================================

// UserContextSummary 用户上下文摘要
type UserContextSummary struct {
	Date     string
	Todos    string
	Exercise string
	Reading  string
	YearPlan string
}

// CollectUserContext 采集用户的各模块数据摘要
// 用于注入到 System Prompt 中，让 AI 了解用户当前状态
func CollectUserContext(account string) UserContextSummary {
	today := time.Now().Format("2006-01-02")
	summary := UserContextSummary{Date: today}

	// 并发采集各模块数据，使用 channel 汇总
	type result struct {
		module string
		data   string
	}
	ch := make(chan result, 4)

	// 采集今日待办
	go func() {
		data := collectTodoSummary(account, today)
		ch <- result{"todo", data}
	}()

	// 采集运动数据
	go func() {
		data := collectExerciseSummary(account, today)
		ch <- result{"exercise", data}
	}()

	// 采集阅读进度
	go func() {
		data := collectReadingSummary(account)
		ch <- result{"reading", data}
	}()

	// 采集年度目标
	go func() {
		data := collectYearPlanSummary(account)
		ch <- result{"yearplan", data}
	}()

	// 汇总结果（超时 3 秒）
	timeout := time.After(3 * time.Second)
collectLoop:
	for i := 0; i < 4; i++ {
		select {
		case r := <-ch:
			switch r.module {
			case "todo":
				summary.Todos = r.data
			case "exercise":
				summary.Exercise = r.data
			case "reading":
				summary.Reading = r.data
			case "yearplan":
				summary.YearPlan = r.data
			}
		case <-timeout:
			log.WarnF(log.ModuleLLM, "User context collection timed out after 3s")
			break collectLoop
		}
	}

	return summary
}

// BuildEnhancedSystemPrompt 构建增强版 System Prompt
func BuildEnhancedSystemPrompt(account string) string {
	ctx := CollectUserContext(account)

	var contextParts []string

	if ctx.Todos != "" {
		contextParts = append(contextParts, fmt.Sprintf("📋 今日待办: %s", ctx.Todos))
	}
	if ctx.Exercise != "" {
		contextParts = append(contextParts, fmt.Sprintf("💪 运动情况: %s", ctx.Exercise))
	}
	if ctx.Reading != "" {
		contextParts = append(contextParts, fmt.Sprintf("📖 阅读进度: %s", ctx.Reading))
	}
	if ctx.YearPlan != "" {
		contextParts = append(contextParts, fmt.Sprintf("🎯 年度目标: %s", ctx.YearPlan))
	}

	// 注入近期会话记忆（跨上下文窗口连续性）
	recentSessions := LoadRecentSessions(account, 3)
	if len(recentSessions) > 0 {
		contextParts = append(contextParts, fmt.Sprintf("💬 近期对话记忆:\n%s", FormatSessionHistory(recentSessions)))
	}

	// 健康检查：主动发现需要关注的问题
	healthCheck := collectHealthCheck(account)
	if healthCheck != "" {
		contextParts = append(contextParts, healthCheck)
	}

	// 加载可插拔 AI 技能
	skills := LoadActiveSkills(account)
	if len(skills) > 0 {
		contextParts = append(contextParts, BuildSkillsPrompt(skills))
	}

	// 构建用户上下文块
	var userContext string
	if len(contextParts) > 0 {
		userContext = config.SafeSprintf(config.GetPrompt(account, "ai_assistant_context"), ctx.Date, strings.Join(contextParts, "\n"))
	}

	sysPrompt := config.SafeSprintf(config.GetPrompt(account, "ai_assistant_system"), account, userContext)

	return sysPrompt
}

// ============================================================================
// 各模块数据采集器
// ============================================================================

// collectTodoSummary 采集今日待办摘要
func collectTodoSummary(account, date string) string {
	defer func() {
		if r := recover(); r != nil {
			log.WarnF(log.ModuleLLM, "collectTodoSummary panic: %v", r)
		}
	}()

	args := map[string]interface{}{
		"account": account,
		"date":    date,
	}
	result := mcp.CallInnerTools("Inner_blog.RawGetTodosByDate", args)
	if result == "" || result == "null" || strings.Contains(result, "error") {
		return ""
	}

	// 简单解析：从 JSON 结果中提取关键信息
	return truncateContextData(result, 300)
}

// collectExerciseSummary 采集近期运动摘要
func collectExerciseSummary(account, date string) string {
	defer func() {
		if r := recover(); r != nil {
			log.WarnF(log.ModuleLLM, "collectExerciseSummary panic: %v", r)
		}
	}()

	args := map[string]interface{}{
		"account": account,
		"date":    date,
	}
	result := mcp.CallInnerTools("Inner_blog.RawGetExerciseByDate", args)
	if result == "" || result == "null" || strings.Contains(result, "error") {
		// 尝试获取运动统计
		statsArgs := map[string]interface{}{
			"account": account,
		}
		statsResult := mcp.CallInnerTools("Inner_blog.RawGetExerciseStats", statsArgs)
		if statsResult != "" && statsResult != "null" {
			return truncateContextData(statsResult, 200)
		}
		return ""
	}

	return truncateContextData(result, 200)
}

// collectReadingSummary 采集阅读进度摘要
func collectReadingSummary(account string) string {
	defer func() {
		if r := recover(); r != nil {
			log.WarnF(log.ModuleLLM, "collectReadingSummary panic: %v", r)
		}
	}()

	args := map[string]interface{}{
		"account": account,
		"status":  "reading",
	}
	result := mcp.CallInnerTools("Inner_blog.RawGetBooksByStatus", args)
	if result == "" || result == "null" || strings.Contains(result, "error") {
		return ""
	}

	return truncateContextData(result, 300)
}

// collectYearPlanSummary 采集年度目标摘要
func collectYearPlanSummary(account string) string {
	defer func() {
		if r := recover(); r != nil {
			log.WarnF(log.ModuleLLM, "collectYearPlanSummary panic: %v", r)
		}
	}()

	year := time.Now().Year()
	args := map[string]interface{}{
		"account": account,
		"year":    year,
	}
	result := mcp.CallInnerTools("Inner_blog.RawGetYearGoals", args)
	if result == "" || result == "null" || strings.Contains(result, "error") {
		return ""
	}

	return truncateContextData(result, 300)
}

// truncateContextData 截断上下文数据到指定长度
func truncateContextData(data string, maxLen int) string {
	runes := []rune(data)
	if len(runes) <= maxLen {
		return data
	}
	return string(runes[:maxLen]) + "..."
}

// collectHealthCheck 自验证：检查用户各模块的健康状态
// 参考 Anthropic 文章：Agent 在开始新工作前应先检查当前状态
func collectHealthCheck(account string) string {
	defer func() {
		if r := recover(); r != nil {
			log.WarnF(log.ModuleLLM, "collectHealthCheck panic: %v", r)
		}
	}()

	var issues []string

	// 检查运动数据：是否连续多天没有运动
	today := time.Now().Format("2006-01-02")
	exerciseResult := mcp.CallInnerTools("RawGetExerciseByDate", map[string]interface{}{
		"account": account,
		"date":    today,
	})
	if exerciseResult == "" || exerciseResult == "null" || strings.Contains(exerciseResult, "[]") {
		// 今天没运动，检查昨天
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		yesterdayResult := mcp.CallInnerTools("RawGetExerciseByDate", map[string]interface{}{
			"account": account,
			"date":    yesterday,
		})
		if yesterdayResult == "" || yesterdayResult == "null" || strings.Contains(yesterdayResult, "[]") {
			issues = append(issues, "已连续2天未记录运动")
		}
	}

	if len(issues) == 0 {
		return ""
	}

	return fmt.Sprintf("⚠️ 需要关注: %s", strings.Join(issues, "; "))
}
