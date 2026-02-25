package http

import (
	"encoding/json"
	"fmt"
	log "mylog"
	h "net/http"
	"time"
)

// HandleAssistantSuggestions handles assistant suggestions API
// 智能助手建议API处理函数
func HandleAssistantSuggestions(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantSuggestions", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	account := getAccountFromRequest(r)
	switch r.Method {
	case h.MethodGet:
		// 生成智能建议
		suggestions := generateAssistantSuggestions(account)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"suggestions": suggestions,
			"timestamp":   time.Now().Unix(),
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleAssistantTrends handles assistant trends data API
// 智能助手趋势数据API处理函数
func HandleAssistantTrends(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantTrends", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		// 生成趋势数据
		trendData := generateTrendData()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"trendData": trendData,
			"timestamp": time.Now().Unix(),
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// generateAssistantSuggestions generates intelligent suggestions
// 生成智能建议
func generateAssistantSuggestions(account string) []map[string]interface{} {
	suggestions := []map[string]interface{}{}

	// 基于任务完成情况生成建议
	taskSuggestion := generateTaskSuggestion(account)
	if taskSuggestion != nil {
		suggestions = append(suggestions, taskSuggestion)
	}

	// 基于阅读习惯生成建议
	readingSuggestion := generateReadingSuggestion(account)
	if readingSuggestion != nil {
		suggestions = append(suggestions, readingSuggestion)
	}

	// 基于锻炼情况生成建议
	exerciseSuggestion := generateExerciseSuggestion(account)
	if exerciseSuggestion != nil {
		suggestions = append(suggestions, exerciseSuggestion)
	}

	// 基于时间模式生成建议
	timeSuggestion := generateTimeSuggestion(account)
	if timeSuggestion != nil {
		suggestions = append(suggestions, timeSuggestion)
	}

	// 基于学习习惯生成建议
	studySuggestion := generateStudySuggestion(account)
	if studySuggestion != nil {
		suggestions = append(suggestions, studySuggestion)
	}

	// 基于健康状况生成建议
	healthSuggestion := generateHealthSuggestion(account)
	if healthSuggestion != nil {
		suggestions = append(suggestions, healthSuggestion)
	}

	// 基于目标进度生成建议
	goalSuggestion := generateGoalSuggestion(account)
	if goalSuggestion != nil {
		suggestions = append(suggestions, goalSuggestion)
	}

	// 基于写作习惯生成建议
	writingSuggestion := generateWritingSuggestion(account)
	if writingSuggestion != nil {
		suggestions = append(suggestions, writingSuggestion)
	}

	// 基于数据分析生成建议
	analyticsSuggestion := generateAnalyticsSuggestion(account)
	if analyticsSuggestion != nil {
		suggestions = append(suggestions, analyticsSuggestion)
	}

	return suggestions
}

// generateTrendData generates trend data for visualization
// 生成趋势数据
func generateTrendData() map[string]interface{} {
	// 获取过去7天的数据
	labels := []string{"7天前", "6天前", "5天前", "4天前", "3天前", "2天前", "昨天", "今天"}

	// 获取任务完成率趋势
	taskCompletionRates := getTaskCompletionTrend()

	// 获取阅读时间趋势
	readingTimeTrend := getReadingTimeTrend()

	// 获取锻炼频率趋势
	exerciseFrequencyTrend := getExerciseFrequencyTrend()

	return map[string]interface{}{
		"labels": labels,
		"datasets": []map[string]interface{}{
			{
				"label":           "任务完成率",
				"data":            taskCompletionRates,
				"borderColor":     "rgba(0, 212, 170, 1)",
				"backgroundColor": "rgba(0, 212, 170, 0.1)",
				"tension":         0.4,
			},
			{
				"label":           "阅读时间(小时)",
				"data":            readingTimeTrend,
				"borderColor":     "rgba(161, 196, 253, 1)",
				"backgroundColor": "rgba(161, 196, 253, 0.1)",
				"tension":         0.4,
			},
			{
				"label":           "锻炼次数",
				"data":            exerciseFrequencyTrend,
				"borderColor":     "rgba(244, 162, 97, 1)",
				"backgroundColor": "rgba(244, 162, 97, 0.1)",
				"tension":         0.4,
			},
		},
	}
}

// getTaskCompletionTrend gets task completion trend for the last 7 days
// 获取任务完成率趋势（近7天）
func getTaskCompletionTrend() []int {
	// 这里应该从真实数据源获取，暂时返回模拟数据
	return []int{80, 75, 90, 85, 70, 95, 85, 60}
}

// getReadingTimeTrend gets reading time trend for the last 7 days
// 获取阅读时间趋势（近7天）
func getReadingTimeTrend() []float64 {
	// 这里应该从真实数据源获取，暂时返回模拟数据
	return []float64{2.0, 1.5, 3.0, 2.5, 1.0, 2.0, 3.0, 2.5}
}

// getExerciseFrequencyTrend gets exercise frequency trend for the last 7 days
// 获取锻炼频率趋势（近7天）
func getExerciseFrequencyTrend() []int {
	// 这里应该从真实数据源获取，暂时返回模拟数据
	return []int{1, 1, 0, 2, 1, 1, 2, 1}
}

// Suggestion generation functions

// generateTaskSuggestion generates task-related suggestions
func generateTaskSuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "📝",
		"text":   "您今天的任务完成率为60%，建议优先处理剩余的重要任务",
		"type":   "task",
		"action": "查看任务列表",
	}
}

// generateReadingSuggestion generates reading-related suggestions
func generateReadingSuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "📚",
		"text":   "今日阅读时间2.5小时，建议继续保持良好的阅读习惯",
		"type":   "reading",
		"action": "查看阅读进度",
	}
}

// generateExerciseSuggestion generates exercise-related suggestions
func generateExerciseSuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "💪",
		"text":   "本周已完成3次锻炼，运动习惯保持良好，继续加油！",
		"type":   "exercise",
		"action": "制定运动计划",
	}
}

// generateTimeSuggestion generates time management suggestions
func generateTimeSuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "⏰",
		"text":   "分析显示您在下午2-4点效率最高，建议安排重要工作",
		"type":   "time",
		"action": "查看时间统计",
	}
}

// generateStudySuggestion generates study-related suggestions
func generateStudySuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "🎓",
		"text":   "您的学习进度保持稳定，建议增加深度学习时间",
		"type":   "study",
		"action": "制定学习计划",
	}
}

// generateHealthSuggestion generates health-related suggestions
func generateHealthSuggestion(account string) map[string]interface{} {
	// 分析作息规律
	sleepPattern := analyzeSleepPattern(account)
	log.DebugF(log.ModuleAssistant, "Health Analysis - Sleep Pattern: EarlyMorning=%d, LateNight=%d, Regularity=%.1f",
		sleepPattern.EarlyMorningActivities, sleepPattern.LateNightActivities, sleepPattern.RegularityScore)

	// 分析生活习惯健康度
	lifeHealthScore := analyzeLifeHealthScore(account)
	log.DebugF(log.ModuleAssistant, "Health Analysis - Life Health Score: Overall=%.1f, Blogging=%.1f, Exercise=%.1f",
		lifeHealthScore.OverallHealthScore, lifeHealthScore.BloggingFrequency, lifeHealthScore.ExerciseConsistency)

	// 根据分析结果生成建议
	suggestion := generateHealthAdvice(sleepPattern, lifeHealthScore)

	return map[string]interface{}{
		"icon":   "❤️",
		"text":   suggestion,
		"type":   "health",
		"action": "查看健康报告",
	}
}

// generateGoalSuggestion generates goal-related suggestions
func generateGoalSuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "🎯",
		"text":   "本月目标完成度75%，距离达成还有5天，加油冲刺！",
		"type":   "goal",
		"action": "查看目标详情",
	}
}

// generateWritingSuggestion generates writing-related suggestions
func generateWritingSuggestion(account string) map[string]interface{} {
	todayCount := getTodayBlogCount(account)
	todayWords := getTodayWordCount(account)

	var text string
	if todayCount == 0 {
		text = "今日还未写作，建议记录一篇日记或博客分享"
	} else if todayWords < 500 {
		text = fmt.Sprintf("今日已写作%d篇，字数偏少(%d字)，建议增加内容深度", todayCount, todayWords)
	} else {
		text = fmt.Sprintf("今日写作状态良好：%d篇博客，共%d字，保持这个习惯！", todayCount, todayWords)
	}

	return map[string]interface{}{
		"icon":   "✍️",
		"text":   text,
		"type":   "writing",
		"action": "开始写作",
	}
}

// generateAnalyticsSuggestion generates analytics-related suggestions
func generateAnalyticsSuggestion(account string) map[string]interface{} {
	return map[string]interface{}{
		"icon":   "📊",
		"text":   "数据完整性85%，持续记录可获得更精准的个人分析",
		"type":   "analytics",
		"action": "查看分析报告",
	}
}
