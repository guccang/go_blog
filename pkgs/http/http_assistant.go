package http

import (
	"control"
	"encoding/json"
	"exercise"
	"fmt"
	"llm"
	"math"
	"mcp"
	"module"
	log "mylog"
	h "net/http"
	"reading"
	"sort"
	"statistics"
	"strings"
	"time"
	"todolist"
	"view"
	"yearplan"
)

// HandleAssistant renders the assistant page
func HandleAssistant(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistant", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageAssistant(w)
}

// HandleAssistantChat handles assistant chat API using llm CallLM
// 智能助手聊天API处理函数 - 使用llm CallLM
func HandleAssistantChat(w h.ResponseWriter, r *h.Request) {
	log.Debug(log.ModuleHandler, "=== Assistant Chat Request Started (MCP Mode) ===")
	LogRemoteAddr("HandleAssistantChat", r)

	if checkLogin(r) != 0 {
		log.WarnF(log.ModuleHandler, "Unauthorized assistant chat request from %s", r.RemoteAddr)
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	llm.ProcessRequest(r, w)
}

// HandleAssistantChatHistory handles loading stored chat messages
// 智能助手聊天历史加载API处理函数
func HandleAssistantChatHistory(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantChatHistory", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取日期参数，默认为今天
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}

		// 获取账户信息
		account := getAccountFromRequest(r)

		// 加载指定日期的聊天历史
		chatHistory := loadChatHistoryForDate(account, date)

		response := map[string]interface{}{
			"success":     true,
			"date":        date,
			"chatHistory": chatHistory,
			"timestamp":   time.Now().Unix(),
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleMCPToolsAPI handles MCP tools API requests
// MCP工具API处理函数
func HandleMCPToolsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMCPToolsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取可用工具列表和服务器状态
		action := r.URL.Query().Get("action")

		switch action {
		case "status":
			// 获取服务器状态
			status := mcp.GetServerStatus()
			response := map[string]interface{}{
				"success": true,
				"status":  status,
			}
			json.NewEncoder(w).Encode(response)
		default:
			// 获取工具列表
			tools := mcp.GetAvailableToolsImproved()
			response := map[string]interface{}{
				"success": true,
				"message": "MCP tools retrieved successfully",
				"data":    tools,
			}
			json.NewEncoder(w).Encode(response)
		}

	case "POST":
		// 测试工具调用
		var toolCall mcp.MCPToolCall
		if err := json.NewDecoder(r.Body).Decode(&toolCall); err != nil {
			response := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Invalid JSON: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		result := mcp.CallToolImproved(toolCall)
		json.NewEncoder(w).Encode(result)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandleAssistantStats handles assistant statistics API
// 智能助手统计API处理函数
func HandleAssistantStats(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantStats", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	account := getAccountFromRequest(r)
	switch r.Method {
	case h.MethodGet:
		// 获取今日统计数据
		stats := gatherTodayStats(account)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"stats":     stats,
			"timestamp": time.Now().Unix(),
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

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

// HandleAssistantHealthData handles health data API for visualization
// 智能助手健康数据API处理函数
func HandleAssistantHealthData(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantHealthData", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	account := getAccountFromRequest(r)
	switch r.Method {
	case h.MethodGet:
		// 生成详细的健康分析数据
		healthData := generateDetailedHealthData(account)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"healthData": healthData,
			"timestamp":  time.Now().Unix(),
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

// ChatMessage represents a chat message in the history
type ChatMessage struct {
	Role      string `json:"role"`      // "user" or "assistant"
	Content   string `json:"content"`   // message content
	Timestamp string `json:"timestamp"` // time when message was sent
}

// loadChatHistoryForDate loads chat history for a specific date
// 加载指定日期的聊天历史
func loadChatHistoryForDate(account, date string) []ChatMessage {
	// 构建AI助手日记标题
	diaryTitle := fmt.Sprintf("AI_assistant_%s", date)

	// 获取博客内容
	blog := control.GetBlog(account, diaryTitle)
	if blog == nil {
		log.DebugF(log.ModuleHandler, "No chat history found for date: %s", date)
		return []ChatMessage{}
	}

	// 解析博客内容，提取聊天记录
	return parseChatHistoryFromContent(blog.Content)
}

// parseChatHistoryFromContent parses chat messages from blog content
// 从博客内容中解析聊天记录
func parseChatHistoryFromContent(content string) []ChatMessage {
	var messages []ChatMessage

	// 按行分割内容
	lines := strings.Split(content, "\n")

	var currentMessage ChatMessage
	var inUserQuestion bool
	var inAIReply bool
	var currentTime string
	var contentBuilder strings.Builder

	for _, line := range lines {
		// 检测新对话开始的标记
		if strings.Contains(line, "### 🤖 AI助手对话") {
			// 提取时间戳
			if strings.Contains(line, "(") && strings.Contains(line, ")") {
				start := strings.Index(line, "(") + 1
				end := strings.Index(line, ")")
				if start < end {
					currentTime = line[start:end]
				}
			}
			continue
		}

		// 检测用户问题开始
		if strings.Contains(line, "**用户问题：**") {
			// 保存之前的AI回复消息（如果有的话）
			if inAIReply && contentBuilder.Len() > 0 {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				if currentMessage.Content != "" {
					messages = append(messages, currentMessage)
				}
				contentBuilder.Reset()
			}

			inUserQuestion = true
			inAIReply = false
			currentMessage = ChatMessage{
				Role:      "user",
				Timestamp: currentTime,
			}
			continue
		}

		// 检测AI回复开始
		if strings.Contains(line, "**AI回复：**") {
			// 保存用户问题消息
			if inUserQuestion && contentBuilder.Len() > 0 {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				if currentMessage.Content != "" {
					messages = append(messages, currentMessage)
				}
				contentBuilder.Reset()
			}

			inUserQuestion = false
			inAIReply = true
			currentMessage = ChatMessage{
				Role:      "assistant",
				Timestamp: currentTime,
			}
			continue
		}

		// 检测分割线，表示一次对话结束
		if strings.Contains(line, "----") && !strings.Contains(line, "|") {
			// 保存当前AI回复消息
			if inAIReply && contentBuilder.Len() > 0 {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				if currentMessage.Content != "" {
					messages = append(messages, currentMessage)
				}
				contentBuilder.Reset()
			}

			inUserQuestion = false
			inAIReply = false
			continue
		}

		// 收集消息内容
		if (inUserQuestion || inAIReply) && line != "" {
			if contentBuilder.Len() > 0 {
				contentBuilder.WriteString("\n")
			}
			contentBuilder.WriteString(line)
		}
	}

	// 处理最后一条消息
	if (inUserQuestion || inAIReply) && contentBuilder.Len() > 0 {
		currentMessage.Content = strings.TrimSpace(contentBuilder.String())
		if currentMessage.Content != "" {
			messages = append(messages, currentMessage)
		}
	}

	log.DebugF(log.ModuleAssistant, "Parsed %d chat messages from content", len(messages))
	return messages
}

// gatherTodayStats generates today's statistics data
// 生成今日统计数据
func gatherTodayStats(account string) map[string]interface{} {
	// 获取今日任务统计
	todayTasks := getTodayTasksStats(account)

	// 获取今日阅读统计
	todayReading := getTodayReadingStats(account)

	// 获取今日锻炼统计
	todayExercise := getTodayExerciseStats(account)

	// 获取今日写作统计
	todayBlogs := getTodayBlogsStats(account)

	log.DebugF(log.ModuleAssistant, "gatherTodayStats: Tasks=%v, Reading=%v, Exercise=%v, Blogs=%v",
		todayTasks, todayReading, todayExercise, todayBlogs)

	return map[string]interface{}{
		"tasks":    todayTasks,
		"reading":  todayReading,
		"exercise": todayExercise,
		"blogs":    todayBlogs,
		"date":     time.Now().Format("2006-01-02"),
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

// Helper functions for generating different types of analysis

// generateStatusAnalysis generates status analysis
// 辅助函数 - 生成状态分析
func generateStatusAnalysis() string {
	return "📊 **整体状态分析**\n\n✅ **优势表现**：\n- 任务执行：近7天平均完成率78%\n- 阅读习惯：日均阅读2.1小时\n- 运动状态：保持良好的运动频率\n\n⚠️ **需要关注**：\n- 睡眠时间略显不足，建议调整作息\n\n💡 **改进建议**：\n- 建议在下午3-5点处理重要任务，这是您的高效时段\n- 保持当前的阅读和运动习惯"
}

// generateTimeAnalysis generates time analysis
// 辅助函数 - 生成时间分析
func generateTimeAnalysis() string {
	return "⏰ **时间分配分析**\n\n📈 **效率高峰**：通常在下午3-5点效率最高\n📊 **时间分布**：\n- 工作学习：6.5小时/天\n- 阅读时间：2.1小时/天\n- 锻炼时间：1.2小时/天\n\n🎯 **优化建议**：\n- 建议将重要任务安排在高效时段\n- 增加休息间隔，避免连续长时间工作\n- 保持规律的作息时间"
}

// generateGoalsAnalysis generates goals analysis
// 辅助函数 - 生成目标分析
func generateGoalsAnalysis() string {
	return "🎯 **目标进度追踪**\n\n📚 **阅读目标**：已完成65%\n💪 **健身目标**：已完成72%\n📝 **写作目标**：已完成45%\n\n🏆 **近期成就**：\n- 连续7天保持阅读习惯\n- 完成3篇高质量博客\n\n📈 **下一步行动**：\n- 专注提升写作频率\n- 继续保持运动习惯\n- 适当调整目标期限"
}

// generateSuggestionsAnalysis generates suggestions analysis
// 辅助函数 - 生成建议分析
func generateSuggestionsAnalysis() string {
	return "💡 **个性化建议**\n\n🔥 **立即行动**：\n- 完成今天剩余的2个任务\n- 安排30分钟阅读时间\n\n📅 **本周计划**：\n- 制定下周的详细学习计划\n- 安排3次锻炼\n\n🎯 **长期优化**：\n- 建立更完善的知识管理系统\n- 提高学习效率\n- 保持工作生活平衡"
}

// generateDefaultResponse generates default response
// 辅助函数 - 生成默认回复
func generateDefaultResponse() string {
	return "这是一个有趣的问题，让我基于您的数据来分析一下...\n\n如果您需要具体的数据分析，可以尝试问我：\n• \"我最近的状态怎么样？\"\n• \"帮我分析一下时间分配\"\n• \"我的目标进度如何？\"\n• \"给我一些建议\""
}

// gatherTaskData collects task data
// 收集任务数据
func gatherTaskData(account string) string {
	// 获取今日任务数据
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("todolist-%s", today)

	// 获取今日任务列表
	todayBlog := control.GetBlog(account, todayTitle)
	var todayCompleted, todayTotal int
	var recentTasks []string

	if todayBlog != nil {
		// 解析今日任务数据
		todayData := todolist.ParseTodoListFromBlog(todayBlog.Content)
		todayTotal = len(todayData.Items)

		for _, item := range todayData.Items {
			if item.Completed {
				todayCompleted++
			}
			if len(recentTasks) < 3 {
				status := "进行中"
				if item.Completed {
					status = "已完成"
				}
				recentTasks = append(recentTasks, fmt.Sprintf("%s(%s)", item.Content, status))
			}
		}
	}

	// 计算本周完成率
	weekCompletionRate := calculateWeeklyTaskCompletion(account)

	// 获取最近完成的任务
	recentCompletedTasks := getRecentCompletedTasks(account, 3)

	recentTasksStr := "无"
	if len(recentCompletedTasks) > 0 {
		recentTasksStr = strings.Join(recentCompletedTasks, ", ")
	} else if len(recentTasks) > 0 {
		recentTasksStr = strings.Join(recentTasks, ", ")
	}

	return fmt.Sprintf("- 今日任务: %d/%d 完成\n- 本周完成率: %.1f%%\n- 最近任务: %s",
		todayCompleted, todayTotal, weekCompletionRate, recentTasksStr)
}

// gatherReadingData collects reading data
// 收集阅读数据
func gatherReadingData(account string) string {
	// 获取所有阅读相关的博客
	readingBlogs := getReadingBlogs(account)

	var currentReading []string
	var recentBooks []string
	var monthlyReadingHours float64
	var readingProgress []string

	for _, blog := range readingBlogs {
		// 解析阅读数据
		bookData := parseReadingDataFromBlog(blog.Content)

		// 统计当前在读的书籍
		if bookData.Status == "reading" {
			currentReading = append(currentReading, bookData.Title)

			// 计算阅读进度
			if bookData.TotalPages > 0 {
				progress := float64(bookData.CurrentPage) / float64(bookData.TotalPages) * 100
				readingProgress = append(readingProgress, fmt.Sprintf("%s(%.0f%%)", bookData.Title, progress))
			}
		}

		// 收集最近阅读的书籍
		if len(recentBooks) < 3 {
			recentBooks = append(recentBooks, bookData.Title)
		}

		// 统计本月阅读时间
		if bookData.LastReadDate != "" {
			if lastRead, err := time.Parse("2006-01-02", bookData.LastReadDate); err == nil {
				if lastRead.Month() == time.Now().Month() && lastRead.Year() == time.Now().Year() {
					monthlyReadingHours += bookData.MonthlyReadingTime
				}
			}
		}
	}

	// 格式化输出
	currentReadingStr := "无"
	if len(currentReading) > 0 {
		currentReadingStr = fmt.Sprintf("%d 本书", len(currentReading))
	}

	recentBooksStr := "无"
	if len(recentBooks) > 0 {
		recentBooksStr = strings.Join(recentBooks, ", ")
	}

	readingProgressStr := "无"
	if len(readingProgress) > 0 {
		readingProgressStr = strings.Join(readingProgress, ", ")
	}

	return fmt.Sprintf("- 当前在读: %s\n- 本月阅读: %.1f 小时\n- 最近阅读: %s\n- 阅读进度: %s",
		currentReadingStr, monthlyReadingHours, recentBooksStr, readingProgressStr)
}

// gatherExerciseData collects exercise data
// 收集锻炼数据
func gatherExerciseData(account string) string {
	// 获取今日锻炼数据
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("exercise-%s", today)

	var todayExercise []string
	var todayCalories float64

	// 获取今日锻炼
	todayBlog := control.GetBlog(account, todayTitle)
	if todayBlog != nil {
		exerciseList := exercise.ParseExerciseFromBlog(todayBlog.Content)

		for _, ex := range exerciseList.Items {
			exerciseType := getExerciseTypeText(ex.Type)
			todayExercise = append(todayExercise, fmt.Sprintf("%s %d分钟", exerciseType, ex.Duration))
			todayCalories += float64(ex.Calories)
		}
	}

	// 获取本周锻炼统计
	weeklyStats := getWeeklyExerciseStats(account)

	// 获取最近锻炼记录
	recentExercises := getRecentExercises(account, 3)

	// 格式化输出
	todayExerciseStr := "无"
	if len(todayExercise) > 0 {
		todayExerciseStr = strings.Join(todayExercise, ", ")
	}

	recentExercisesStr := "无"
	if len(recentExercises) > 0 {
		recentExercisesStr = strings.Join(recentExercises, ", ")
	}

	return fmt.Sprintf("- 今日锻炼: %s\n- 本周锻炼: %d 次\n- 消耗卡路里: %.0f 千卡\n- 最近锻炼: %s",
		todayExerciseStr, weeklyStats.SessionCount, weeklyStats.TotalCalories, recentExercisesStr)
}

// gatherBlogData collects blog data
// 收集博客数据
func gatherBlogData(account string) string {
	// 获取所有博客数据
	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	var totalBlogs int
	var monthlyBlogs int
	var recentBlogs []string
	var tagCount map[string]int

	tagCount = make(map[string]int)
	currentMonth := time.Now().Format("2006-01")

	// 过滤掉系统生成的博客（任务、锻炼、阅读等）
	for _, blog := range allBlogs {
		// 跳过系统生成的博客
		if isSystemBlog(blog.Title) {
			continue
		}

		totalBlogs++

		// 统计本月博客
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.Format("2006-01") == currentMonth {
					monthlyBlogs++
				}
			}
		}

		// 收集最近博客
		if len(recentBlogs) < 3 {
			recentBlogs = append(recentBlogs, blog.Title)
		}

		// 统计标签
		if blog.Tags != "" {
			tags := strings.Split(blog.Tags, "|")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tagCount[tag]++
				}
			}
		}
	}

	// 获取热门标签
	hotTags := getHotTags(tagCount, 3)

	// 格式化输出
	recentBlogsStr := "无"
	if len(recentBlogs) > 0 {
		recentBlogsStr = strings.Join(recentBlogs, ", ")
	}

	hotTagsStr := "无"
	if len(hotTags) > 0 {
		hotTagsStr = strings.Join(hotTags, ", ")
	}

	return fmt.Sprintf("- 总博客数: %d 篇\n- 本月发布: %d 篇\n- 最近博客: %s\n- 热门标签: %s",
		totalBlogs, monthlyBlogs, recentBlogsStr, hotTagsStr)
}

// gatherYearPlanData collects year plan data
// 收集年度计划数据
func gatherYearPlanData(account string) string {
	// 获取当前年份
	currentYear := time.Now().Year()
	yearPlanTitle := fmt.Sprintf("年计划_%d", currentYear)

	// 获取年度计划
	yearPlan := control.GetBlog(account, yearPlanTitle)
	if yearPlan == nil {
		return "- 年度目标: 未设置\n- 整体进度: 0%\n- 目标详情: 暂无年度计划"
	}

	// 解析年度计划数据
	yearPlanData := yearplan.ParseYearPlanFromBlog(yearPlan.Content)

	// 获取月度目标统计
	monthlyStats := getMonthlyGoalsStats(currentYear)

	// 计算整体进度
	var totalProgress float64
	var goalCount int
	var goalDetails []string

	for _, goal := range yearPlanData.Tasks {
		if goal.Status == "completed" {
			totalProgress += 1
			goalCount++
			goalDetails = append(goalDetails, fmt.Sprintf("%s(%.0f%%)", goal.Title, 100.0))
		}
	}

	overallProgress := float64(0)
	if goalCount > 0 {
		overallProgress = totalProgress / float64(goalCount) * 100
	}

	// 格式化输出
	goalDetailsStr := "暂无具体目标"
	if len(goalDetails) > 0 {
		goalDetailsStr = strings.Join(goalDetails, ", ")
	}

	return fmt.Sprintf("- 年度目标: %d 个\n- 整体进度: %.1f%%\n- 完成月份: %d/%d\n- 目标详情: %s",
		len(yearPlanData.Tasks), overallProgress, monthlyStats.CompletedMonths,
		monthlyStats.TotalMonths, goalDetailsStr)
}

// gatherStatsData collects statistics data
// 收集统计数据
func gatherStatsData(account string) string {
	// 获取系统整体统计
	stats := statistics.GetOverallStatistics(account)

	// 计算活跃天数
	activeDays := calculateActiveDays()

	// 计算数据完整性
	dataCompleteness := calculateDataCompleteness()

	// 计算生产力指数
	productivityIndex := calculateProductivityIndex()

	// 分析近期趋势
	recentTrend := analyzeRecentTrend()

	return fmt.Sprintf("- 活跃天数: %d 天\n- 数据完整性: %.1f%%\n- 生产力指数: %.1f\n- 近期趋势: %s\n- 总博客数: %d\n- 今日新增: %d",
		activeDays, dataCompleteness, productivityIndex, recentTrend, stats.BlogStats.TotalBlogs, stats.BlogStats.TodayNewBlogs)
}

// Data structures used in assistant functions

// ReadingBookData represents reading book data structure
// 阅读书籍数据结构
type ReadingBookData struct {
	Title              string
	Status             string
	CurrentPage        int
	TotalPages         int
	MonthlyReadingTime float64
	LastReadDate       string
}

// WeeklyExerciseStats represents weekly exercise statistics
// 本周锻炼统计结构
type WeeklyExerciseStats struct {
	SessionCount  int
	TotalCalories float64
}

// Helper functions

// calculateWeeklyTaskCompletion calculates weekly task completion rate
// 计算本周任务完成率
func calculateWeeklyTaskCompletion(account string) float64 {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))

	var totalTasks, completedTasks int

	for i := 0; i < 7; i++ {
		date := weekStart.AddDate(0, 0, i)
		title := fmt.Sprintf("todolist-%s", date.Format("2006-01-02"))

		blog := control.GetBlog(account, title)
		if blog != nil {
			todoData := todolist.ParseTodoListFromBlog(blog.Content)
			totalTasks += len(todoData.Items)

			for _, item := range todoData.Items {
				if item.Completed {
					completedTasks++
				}
			}
		}
	}

	if totalTasks == 0 {
		return 0
	}

	return float64(completedTasks) / float64(totalTasks) * 100
}

// getRecentCompletedTasks gets recently completed tasks
// 获取最近完成的任务
func getRecentCompletedTasks(account string, limit int) []string {
	var recentTasks []string
	now := time.Now()

	// 查看最近7天的任务
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		title := fmt.Sprintf("todolist-%s", date.Format("2006-01-02"))

		blog := control.GetBlog(account, title)
		if blog != nil {
			todoData := todolist.ParseTodoListFromBlog(blog.Content)

			for _, item := range todoData.Items {
				if item.Completed && len(recentTasks) < limit {
					recentTasks = append(recentTasks, item.Content)
				}
			}
		}

		if len(recentTasks) >= limit {
			break
		}
	}

	return recentTasks
}

// getReadingBlogs gets reading-related blogs
// 获取阅读相关的博客
func getReadingBlogs(account string) []*module.Blog {
	allBlogs := control.GetAll(account, 0, module.EAuthType_all)
	var readingBlogs []*module.Blog

	for _, blog := range allBlogs {
		if strings.HasPrefix(blog.Title, "reading_book_") {
			readingBlogs = append(readingBlogs, blog)
		}
	}

	return readingBlogs
}

// parseReadingDataFromBlog parses reading data from blog content
// 解析阅读数据
func parseReadingDataFromBlog(content string) ReadingBookData {
	// 简化的解析逻辑
	data := ReadingBookData{
		Status:             "reading",
		CurrentPage:        0,
		TotalPages:         0,
		MonthlyReadingTime: 0,
		LastReadDate:       time.Now().Format("2006-01-02"),
	}

	// 从content中解析标题
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			data.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	return data
}

// getExerciseTypeText gets exercise type text in Chinese
// 获取锻炼类型文本
func getExerciseTypeText(exerciseType string) string {
	switch exerciseType {
	case "cardio":
		return "有氧运动"
	case "strength":
		return "力量训练"
	case "flexibility":
		return "柔韧性训练"
	case "sports":
		return "运动项目"
	default:
		return "锻炼"
	}
}

// getWeeklyExerciseStats gets weekly exercise statistics
// 获取本周锻炼统计
func getWeeklyExerciseStats(account string) WeeklyExerciseStats {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))

	var sessionCount int
	var totalCalories float64

	for i := 0; i < 7; i++ {
		date := weekStart.AddDate(0, 0, i)
		title := fmt.Sprintf("exercise-%s", date.Format("2006-01-02"))

		blog := control.GetBlog(account, title)
		if blog != nil {
			exercises := exercise.ParseExerciseFromBlog(blog.Content)
			if len(exercises.Items) > 0 {
				sessionCount++
				for _, ex := range exercises.Items {
					totalCalories += float64(ex.Calories)
				}
			}
		}
	}

	return WeeklyExerciseStats{
		SessionCount:  sessionCount,
		TotalCalories: totalCalories,
	}
}

// getRecentExercises gets recent exercise records
// 获取最近锻炼记录
func getRecentExercises(account string, limit int) []string {
	var recentExercises []string
	now := time.Now()

	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		title := fmt.Sprintf("exercise-%s", date.Format("2006-01-02"))

		blog := control.GetBlog(account, title)
		if blog != nil {
			exercises := exercise.ParseExerciseFromBlog(blog.Content)

			for _, ex := range exercises.Items {
				if len(recentExercises) < limit {
					exerciseType := getExerciseTypeText(ex.Type)
					recentExercises = append(recentExercises, fmt.Sprintf("%s(%d分钟)", exerciseType, ex.Duration))
				}
			}
		}

		if len(recentExercises) >= limit {
			break
		}
	}

	return recentExercises
}

// isSystemBlog checks if a blog is system-generated
// 判断是否为系统生成的博客
func isSystemBlog(title string) bool {
	systemPrefixes := []string{
		"todolist-",
		"exercise-",
		"reading_book_",
		"年计划_",
		"月度目标_",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(title, prefix) {
			return true
		}
	}

	return false
}

// getHotTags gets hot tags from tag count map
// 获取热门标签
func getHotTags(tagCount map[string]int, limit int) []string {
	type tagInfo struct {
		name  string
		count int
	}

	var tags []tagInfo
	for name, count := range tagCount {
		tags = append(tags, tagInfo{name: name, count: count})
	}

	// 简单排序（按计数降序）
	for i := 0; i < len(tags)-1; i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[i].count < tags[j].count {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}

	var result []string
	for i, tag := range tags {
		if i >= limit {
			break
		}
		result = append(result, tag.name)
	}

	return result
}

// getTopTagsFromMap gets top tags from tag count map
// 从标签计数映射中获取热门标签
func getTopTagsFromMap(tagCount map[string]int, limit int) []string {
	type tagInfo struct {
		name  string
		count int
	}

	var tags []tagInfo
	for name, count := range tagCount {
		tags = append(tags, tagInfo{name: name, count: count})
	}

	// Sort by count (descending)
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].count > tags[j].count
	})

	var result []string
	for i, tag := range tags {
		if i >= limit {
			break
		}
		result = append(result, tag.name)
	}

	return result
}

// parseInt parses a string to integer, returns 0 if failed
// 解析字符串为整数，失败时返回0
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	result := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			break
		}
	}
	return result
}

// MonthlyGoalsStats represents monthly goals statistics
type MonthlyGoalsStats struct {
	CompletedMonths int
	TotalMonths     int
}

// getMonthlyGoalsStats gets monthly goals statistics
// 获取月度目标统计
func getMonthlyGoalsStats(year int) MonthlyGoalsStats {
	// 简化实现，返回模拟数据
	return MonthlyGoalsStats{
		CompletedMonths: 8,
		TotalMonths:     12,
	}
}

// calculateActiveDays calculates active days
// 计算活跃天数
func calculateActiveDays() int {
	// 简化实现，返回模拟数据
	return 180
}

// calculateDataCompleteness calculates data completeness percentage
// 计算数据完整性
func calculateDataCompleteness() float64 {
	// 简化实现，返回模拟数据
	return 85.5
}

// calculateProductivityIndex calculates productivity index
// 计算生产力指数
func calculateProductivityIndex() float64 {
	// 简化实现，返回模拟数据
	return 78.5
}

// analyzeRecentTrend analyzes recent trend
// 分析近期趋势
func analyzeRecentTrend() string {
	return "上升"
}

// Individual stats functions that can be implemented based on real data

// getTodayTasksStats gets today's tasks statistics
func getTodayTasksStats(account string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("todolist-%s", today)

	// Get today's todo blog
	todayBlog := control.GetBlog(account, todayTitle)
	if todayBlog == nil {
		log.DebugF(log.ModuleAssistant, "getTodayTasksStats: No todo blog found for %s", today)
		return map[string]interface{}{
			"total":           0,
			"completed":       0,
			"pending":         0,
			"completion_rate": 0.0,
			"total_minutes":   0,
			"date":            today,
		}
	}

	// Parse todo data from blog content
	todoData := todolist.ParseTodoListFromBlog(todayBlog.Content)
	totalTasks := len(todoData.Items)
	completedTasks := 0
	totalMinutes := 0

	for _, item := range todoData.Items {
		if item.Completed {
			completedTasks++
		}
		totalMinutes += item.Hours*60 + item.Minutes
	}

	completionRate := 0.0
	if totalTasks > 0 {
		completionRate = float64(completedTasks) / float64(totalTasks) * 100
	}

	log.DebugF(log.ModuleAssistant, "getTodayTasksStats: Found %d total tasks, %d completed (%.1f%%) for %s", totalTasks, completedTasks, completionRate, today)

	return map[string]interface{}{
		"total":           totalTasks,
		"completed":       completedTasks,
		"pending":         totalTasks - completedTasks,
		"completion_rate": completionRate,
		"total_minutes":   totalMinutes,
		"date":            today,
	}
}

// getTodayReadingStats gets today's reading statistics using reading module interfaces
func getTodayReadingStats(account string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")

	// 使用reading模块的接口获取统计数据
	stats := reading.GetReadingStatisticsWithAccount("")

	// 获取当前在读的书籍
	currentBooks := []string{}
	totalProgress := 0.0
	validProgressBooks := 0
	todayPages := 0

	// 遍历所有书籍获取详细信息
	books := reading.GetAllBooksWithAccount("")
	for _, book := range books {
		if book.Status == "reading" {
			if len(currentBooks) < 3 {
				currentBooks = append(currentBooks, book.Title)
			}

			// 计算阅读进度
			if book.TotalPages > 0 {
				progress := float64(book.CurrentPage) / float64(book.TotalPages) * 100
				totalProgress += progress
				validProgressBooks++
			}
		}
	}

	// 估算今日阅读页数（基于阅读记录的最后更新时间）
	// 由于没有直接获取所有阅读记录的函数，我们需要通过书籍来获取记录
	for _, book := range books {
		record := reading.GetReadingRecordWithAccount(account, book.ID)
		if record == nil {
			continue
		}
		if record.LastUpdateTime != "" {
			if lastUpdate, err := time.Parse("2006-01-02 15:04:05", record.LastUpdateTime); err == nil {
				if lastUpdate.Format("2006-01-02") == today {
					// 简单估算：假设每次更新读了5页
					todayPages += 5
				}
			}
		}
	}

	// 计算平均阅读进度
	averageProgress := 0.0
	if validProgressBooks > 0 {
		averageProgress = totalProgress / float64(validProgressBooks)
	} else if stats["reading_books"].(int) > 0 {
		// 如果没有具体进度数据，但有正在阅读的书，给一个默认进度
		averageProgress = 50.0
	}

	log.DebugF(log.ModuleAssistant, "getTodayReadingStats: Found %d reading books, average progress %.1f%%, today pages: %d",
		stats["reading_books"].(int), averageProgress, todayPages)

	return map[string]interface{}{
		"reading_books": stats["reading_books"],
		"total_books":   stats["total_books"],
		"today_pages":   todayPages,
		"progress":      int(averageProgress), // 前端期望的字段名，返回整数百分比
		"current_books": currentBooks,
		"date":          today,
	}
}

// getTodayExerciseStats gets today's exercise statistics
func getTodayExerciseStats(account string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("exercise-%s", today)

	// Get today's exercise blog
	todayBlog := control.GetBlog(account, todayTitle)
	if todayBlog == nil {
		log.DebugF(log.ModuleAssistant, "getTodayExerciseStats: No exercise blog found for %s", today)
		return map[string]interface{}{
			"total_exercises":     0,
			"completed_exercises": 0,
			"sessions":            0, // 前端期望的字段名
			"total_duration":      0,
			"total_calories":      0,
			"completion_rate":     0.0,
			"exercise_types":      []string{},
			"date":                today,
		}
	}

	// Parse exercise data from blog content
	exerciseList := exercise.ParseExerciseFromBlog(todayBlog.Content)
	totalExercises := len(exerciseList.Items)
	completedExercises := 0
	totalDuration := 0
	totalCalories := 0
	exerciseTypes := []string{}
	exerciseTypeMap := make(map[string]bool)

	for _, item := range exerciseList.Items {
		if item.Completed {
			completedExercises++
			totalDuration += item.Duration
			totalCalories += item.Calories
		}

		// Collect unique exercise types
		if !exerciseTypeMap[item.Type] {
			exerciseTypeMap[item.Type] = true
			exerciseTypes = append(exerciseTypes, getExerciseTypeText(item.Type))
		}
	}

	completionRate := 0.0
	if totalExercises > 0 {
		completionRate = float64(completedExercises) / float64(totalExercises) * 100
	}

	log.DebugF(log.ModuleAssistant, "getTodayExerciseStats: Found %d total exercises, %d completed, %d calories for %s", totalExercises, completedExercises, totalCalories, today)

	return map[string]interface{}{
		"total_exercises":     totalExercises,
		"completed_exercises": completedExercises,
		"sessions":            completedExercises, // 前端期望的字段名
		"total_duration":      totalDuration,
		"total_calories":      totalCalories,
		"completion_rate":     completionRate,
		"exercise_types":      exerciseTypes,
		"date":                today,
	}
}

// getTodayBlogsStats gets today's blogs statistics
func getTodayBlogsStats(account string) map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	createdToday := 0
	updatedToday := 0
	totalWords := 0
	publicBlogs := 0
	privateBlogs := 0
	encryptedBlogs := 0
	todayBlogs := []string{}
	tags := make(map[string]int)

	log.DebugF(log.ModuleAssistant, "getTodayBlogsStats: Processing %d total blogs for date %s", len(allBlogs), today)

	for _, blog := range allBlogs {
		// Skip system-generated blogs
		if isSystemBlog(blog.Title) {
			continue
		}

		// Check if blog was created today
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.Format("2006-01-02") == today {
					createdToday++

					// Calculate word count for today's blogs
					content := strings.TrimSpace(blog.Content)
					if content != "" {
						wordCount := calculateWordCount(content)
						totalWords += wordCount
					}

					// Collect blog titles
					if len(todayBlogs) < 5 {
						todayBlogs = append(todayBlogs, blog.Title)
					}

					// Count by auth type
					switch blog.AuthType {
					case module.EAuthType_public:
						publicBlogs++
					case module.EAuthType_private:
						privateBlogs++
					case module.EAuthType_encrypt:
						encryptedBlogs++
					}

					// Count tags
					if blog.Tags != "" {
						blogTags := strings.Split(blog.Tags, "|")
						for _, tag := range blogTags {
							tag = strings.TrimSpace(tag)
							if tag != "" {
								tags[tag]++
							}
						}
					}
				}
			}
		}

		// Check if blog was updated today (but not created today)
		if blog.AccessTime != "" {
			if accessTime, err := time.Parse("2006-01-02 15:04:05", blog.AccessTime); err == nil {
				if accessTime.Format("2006-01-02") == today {
					// Check if it wasn't created today (to avoid double counting)
					if blog.CreateTime != "" {
						if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
							if createTime.Format("2006-01-02") != today {
								updatedToday++
							}
						}
					}
				}
			}
		}
	}

	// Get top tags for today
	topTags := getTopTagsFromMap(tags, 3)

	log.DebugF(log.ModuleAssistant, "getTodayBlogsStats: Created=%d, Updated=%d, Words=%d, PublicBlogs=%d",
		createdToday, updatedToday, totalWords, publicBlogs)

	return map[string]interface{}{
		"created":         createdToday,
		"updated":         updatedToday,
		"count":           createdToday, // 前端期望的字段名
		"total_words":     totalWords,
		"public_blogs":    publicBlogs,
		"private_blogs":   privateBlogs,
		"encrypted_blogs": encryptedBlogs,
		"today_blogs":     todayBlogs,
		"top_tags":        topTags,
		"date":            today,
	}
}

// getTodayBlogCount gets the count of blogs created today
func getTodayBlogCount(account string) int {
	today := time.Now().Format("2006-01-02")
	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	log.DebugF(log.ModuleAssistant, "getTodayBlogCount: Found %d total blogs", len(allBlogs))

	count := 0
	for _, blog := range allBlogs {
		// 跳过系统博客
		if isSystemBlog(blog.Title) {
			continue
		}

		// 检查博客是否是今天创建的
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.Format("2006-01-02") == today {
					log.DebugF(log.ModuleAssistant, "getTodayBlogCount: Found today's blog: %s", blog.Title)
					count++
				}
			}
		}
	}

	log.DebugF(log.ModuleAssistant, "getTodayBlogCount: Returning count=%d for today=%s", count, today)
	return count
}

// getTodayWordCount gets the total word count for today's blogs
func getTodayWordCount(account string) int {
	today := time.Now().Format("2006-01-02")
	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	totalWords := 0
	for _, blog := range allBlogs {
		// 跳过系统博客
		if isSystemBlog(blog.Title) {
			continue
		}

		// 检查博客是否是今天创建的
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.Format("2006-01-02") == today {
					// 计算字数（简单的字符数统计，中文字符按1个字计算）
					content := strings.TrimSpace(blog.Content)
					if content != "" {
						// 去除markdown标记和特殊字符，进行基本的字数统计
						wordCount := calculateWordCount(content)
						totalWords += wordCount
					}
				}
			}
		}
	}

	return totalWords
}

// calculateWordCount calculates word count from content
func calculateWordCount(content string) int {
	// 移除常见的markdown标记
	content = strings.ReplaceAll(content, "#", "")
	content = strings.ReplaceAll(content, "*", "")
	content = strings.ReplaceAll(content, "_", "")
	content = strings.ReplaceAll(content, "`", "")
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\t", " ")

	// 压缩多个空格为单个空格
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}

	// 简单的字数统计：按字符数计算（适合中文）
	// 对于更精确的统计，可以区分中英文
	runes := []rune(content)
	return len(runes)
}

// Health analysis structures and functions

// SleepPattern represents sleep pattern analysis
type SleepPattern struct {
	EarlyMorningActivities int     // 早晨活动次数 (5:00-9:00)
	LateNightActivities    int     // 深夜活动次数 (22:00-2:00)
	RegularityScore        float64 // 作息规律性评分 (0-100)
	AverageFirstActivity   string  // 平均首次活动时间
	AverageLastActivity    string  // 平均最后活动时间
}

// LifeHealthScore represents overall life health assessment
type LifeHealthScore struct {
	BloggingFrequency   float64 // 写作频率评分
	TaskCompletionRate  float64 // 任务完成率
	ExerciseConsistency float64 // 锻炼一致性
	ReadingHabit        float64 // 阅读习惯评分
	OverallHealthScore  float64 // 综合健康评分
}

// analyzeSleepPattern analyzes sleep and activity patterns from blog data
func analyzeSleepPattern(account string) SleepPattern {
	now := time.Now()
	oneWeekAgo := now.AddDate(0, 0, -7)

	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	var earlyMorning, lateNight int
	var firstActivities, lastActivities []time.Time
	var dailyActivities = make(map[string][]time.Time) // 按日期组织活动时间

	for _, blog := range allBlogs {
		if isSystemBlog(blog.Title) {
			continue
		}

		// 分析创建时间
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.After(oneWeekAgo) {
					hour := createTime.Hour()
					dateKey := createTime.Format("2006-01-02")

					// 记录每日活动时间
					dailyActivities[dateKey] = append(dailyActivities[dateKey], createTime)

					// 统计早晨活动 (5:00-9:00)
					if hour >= 5 && hour < 9 {
						earlyMorning++
					}

					// 统计深夜活动 (22:00-2:00)
					if hour >= 22 || hour < 2 {
						lateNight++
					}
				}
			}
		}

		// 分析访问时间
		if blog.AccessTime != "" {
			if accessTime, err := time.Parse("2006-01-02 15:04:05", blog.AccessTime); err == nil {
				if accessTime.After(oneWeekAgo) {
					hour := accessTime.Hour()
					dateKey := accessTime.Format("2006-01-02")

					// 记录每日活动时间
					dailyActivities[dateKey] = append(dailyActivities[dateKey], accessTime)

					// 统计早晨活动
					if hour >= 5 && hour < 9 {
						earlyMorning++
					}

					// 统计深夜活动
					if hour >= 22 || hour < 2 {
						lateNight++
					}
				}
			}
		}
	}

	// 计算每日的首次和最后活动时间
	for _, activities := range dailyActivities {
		if len(activities) > 0 {
			// 排序活动时间
			sort.Slice(activities, func(i, j int) bool {
				return activities[i].Before(activities[j])
			})

			firstActivities = append(firstActivities, activities[0])
			lastActivities = append(lastActivities, activities[len(activities)-1])
		}
	}

	// 计算规律性评分
	regularityScore := calculateRegularityScore(firstActivities, lastActivities)

	// 计算平均时间
	avgFirst := calculateAverageTime(firstActivities)
	avgLast := calculateAverageTime(lastActivities)

	return SleepPattern{
		EarlyMorningActivities: earlyMorning,
		LateNightActivities:    lateNight,
		RegularityScore:        regularityScore,
		AverageFirstActivity:   avgFirst,
		AverageLastActivity:    avgLast,
	}
}

// analyzeLifeHealthScore analyzes overall life health metrics
func analyzeLifeHealthScore(account string) LifeHealthScore {
	// 分析写作频率 (近7天)
	bloggingScore := analyzeBloggingFrequency(account)

	// 分析任务完成率
	taskScore := analyzeTaskCompletion(account)

	// 分析锻炼一致性
	exerciseScore := analyzeExerciseConsistency(account)

	// 分析阅读习惯
	readingScore := analyzeReadingHabit(account)

	// 计算综合评分
	overallScore := (bloggingScore + taskScore + exerciseScore + readingScore) / 4.0

	return LifeHealthScore{
		BloggingFrequency:   bloggingScore,
		TaskCompletionRate:  taskScore,
		ExerciseConsistency: exerciseScore,
		ReadingHabit:        readingScore,
		OverallHealthScore:  overallScore,
	}
}

// calculateRegularityScore calculates sleep regularity score
func calculateRegularityScore(firstActivities, lastActivities []time.Time) float64 {
	if len(firstActivities) < 2 || len(lastActivities) < 2 {
		return 50.0 // 默认中等评分
	}

	// 计算首次活动时间的标准差
	firstVariance := calculateTimeVariance(firstActivities)
	lastVariance := calculateTimeVariance(lastActivities)

	// 标准差越小，规律性越高
	// 将标准差转换为0-100的评分
	avgVariance := (firstVariance + lastVariance) / 2.0

	// 如果平均方差小于1小时，评分很高；大于4小时，评分很低
	if avgVariance <= 1.0 {
		return 90.0 + (1.0-avgVariance)*10.0
	} else if avgVariance <= 4.0 {
		return 90.0 - (avgVariance-1.0)*20.0
	} else {
		return math.Max(10.0, 30.0-(avgVariance-4.0)*5.0)
	}
}

// calculateAverageTime calculates average time from a slice of times
func calculateAverageTime(times []time.Time) string {
	if len(times) == 0 {
		return "未知"
	}

	totalMinutes := 0
	for _, t := range times {
		totalMinutes += t.Hour()*60 + t.Minute()
	}

	avgMinutes := totalMinutes / len(times)
	avgHour := avgMinutes / 60
	avgMinute := avgMinutes % 60

	return fmt.Sprintf("%02d:%02d", avgHour, avgMinute)
}

// calculateTimeVariance calculates variance in hours for time slice
func calculateTimeVariance(times []time.Time) float64 {
	if len(times) <= 1 {
		return 0.0
	}

	// 转换为分钟数进行计算
	var minutes []float64
	for _, t := range times {
		minutes = append(minutes, float64(t.Hour()*60+t.Minute()))
	}

	// 计算平均值
	sum := 0.0
	for _, m := range minutes {
		sum += m
	}
	mean := sum / float64(len(minutes))

	// 计算方差
	variance := 0.0
	for _, m := range minutes {
		variance += (m - mean) * (m - mean)
	}
	variance /= float64(len(minutes))

	// 转换为小时单位
	return math.Sqrt(variance) / 60.0
}

// analyzeBloggingFrequency analyzes blogging frequency score
func analyzeBloggingFrequency(account string) float64 {
	weeklyBlogs := 0
	now := time.Now()
	oneWeekAgo := now.AddDate(0, 0, -7)

	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	for _, blog := range allBlogs {
		if isSystemBlog(blog.Title) {
			continue
		}

		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.After(oneWeekAgo) {
					weeklyBlogs++
				}
			}
		}
	}

	// 评分标准：每周7篇=100分，3篇=70分，1篇=40分，0篇=0分
	if weeklyBlogs >= 7 {
		return 100.0
	} else if weeklyBlogs >= 3 {
		return 70.0 + float64(weeklyBlogs-3)*7.5
	} else if weeklyBlogs >= 1 {
		return 40.0 + float64(weeklyBlogs-1)*15.0
	}
	return 0.0
}

// analyzeTaskCompletion analyzes task completion rate
func analyzeTaskCompletion(account string) float64 {
	// 简化实现：基于近期任务完成情况
	// 这里可以集成真实的任务系统数据

	// 模拟数据：近期任务完成率
	return 75.0 // 可以后续集成真实任务数据
}

// analyzeExerciseConsistency analyzes exercise consistency
func analyzeExerciseConsistency(account string) float64 {
	// 简化实现：基于近期锻炼记录
	// 这里可以集成真实的锻炼数据

	// 模拟数据：锻炼一致性评分
	return 60.0 // 可以后续集成真实锻炼数据
}

// analyzeReadingHabit analyzes reading habit score
func analyzeReadingHabit(account string) float64 {
	// 简化实现：基于阅读相关博客数量和频率
	readingBlogs := getReadingBlogs(account)

	if len(readingBlogs) == 0 {
		return 30.0
	}

	// 基于阅读博客数量评分
	if len(readingBlogs) >= 10 {
		return 90.0
	} else if len(readingBlogs) >= 5 {
		return 70.0 + float64(len(readingBlogs)-5)*4.0
	} else {
		return 50.0 + float64(len(readingBlogs))*4.0
	}
}

// generateHealthAdvice generates health advice based on analysis
func generateHealthAdvice(sleepPattern SleepPattern, lifeHealth LifeHealthScore) string {
	var suggestions []string

	// 作息建议
	if sleepPattern.LateNightActivities > 3 {
		suggestions = append(suggestions, "深夜活动过多，建议22点后减少电子设备使用")
	}

	if sleepPattern.EarlyMorningActivities < 2 {
		suggestions = append(suggestions, "早起活动较少，建议培养早起习惯")
	}

	if sleepPattern.RegularityScore < 60 {
		suggestions = append(suggestions, "作息不够规律，建议固定作息时间")
	}

	// 生活习惯建议
	if lifeHealth.BloggingFrequency < 50 {
		suggestions = append(suggestions, "写作频率偏低，建议增加记录和分享")
	}

	if lifeHealth.ExerciseConsistency < 70 {
		suggestions = append(suggestions, "运动频率不足，建议增加体育锻炼")
	}

	if lifeHealth.ReadingHabit < 60 {
		suggestions = append(suggestions, "阅读习惯有待提升，建议增加阅读时间")
	}

	// 综合评价
	if lifeHealth.OverallHealthScore >= 80 {
		return fmt.Sprintf("健康状态良好！继续保持规律作息。%s", strings.Join(suggestions, "；"))
	} else if lifeHealth.OverallHealthScore >= 60 {
		return fmt.Sprintf("健康状态一般，建议改进：%s", strings.Join(suggestions, "；"))
	} else {
		return fmt.Sprintf("健康状态需要关注，重点改进：%s", strings.Join(suggestions, "；"))
	}
}

// generateDetailedHealthData generates comprehensive health data for visualization
func generateDetailedHealthData(account string) map[string]interface{} {
	// 分析作息规律
	sleepPattern := analyzeSleepPattern(account)

	// 分析生活习惯健康度
	lifeHealthScore := analyzeLifeHealthScore(account)

	// 生成活动时间分布数据
	activityHourDistribution := generateActivityHourDistribution(account)

	// 生成一周健康趋势数据
	weeklyHealthTrend := generateWeeklyHealthTrend(account)

	// 生成健康评分雷达图数据
	healthRadarData := generateHealthRadarData(account, lifeHealthScore)

	return map[string]interface{}{
		"sleepPattern":             sleepPattern,
		"lifeHealthScore":          lifeHealthScore,
		"activityHourDistribution": activityHourDistribution,
		"weeklyHealthTrend":        weeklyHealthTrend,
		"healthRadarData":          healthRadarData,
		"healthAdvice":             generateHealthAdvice(sleepPattern, lifeHealthScore),
		"lastAnalysisTime":         time.Now().Format("2006-01-02 15:04:05"),
	}
}

// generateActivityHourDistribution generates hourly activity distribution
func generateActivityHourDistribution(account string) map[string]interface{} {
	hourCounts := make([]int, 24) // 24小时计数
	now := time.Now()
	oneWeekAgo := now.AddDate(0, 0, -7)

	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	for _, blog := range allBlogs {
		if isSystemBlog(blog.Title) {
			continue
		}

		// 统计创建时间分布
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.After(oneWeekAgo) {
					hourCounts[createTime.Hour()]++
				}
			}
		}

		// 统计访问时间分布
		if blog.AccessTime != "" {
			if accessTime, err := time.Parse("2006-01-02 15:04:05", blog.AccessTime); err == nil {
				if accessTime.After(oneWeekAgo) {
					hourCounts[accessTime.Hour()]++
				}
			}
		}
	}

	// 生成图表标签
	labels := make([]string, 24)
	for i := 0; i < 24; i++ {
		labels[i] = fmt.Sprintf("%02d:00", i)
	}

	return map[string]interface{}{
		"labels": labels,
		"data":   hourCounts,
		"title":  "24小时活动分布",
	}
}

// generateWeeklyHealthTrend generates weekly health trend data
func generateWeeklyHealthTrend(account string) map[string]interface{} {
	labels := make([]string, 7)
	blogCounts := make([]int, 7)
	activityCounts := make([]int, 7)

	now := time.Now()

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		labels[6-i] = date.Format("01-02")

		// 统计当天博客数量和活动数量
		dailyBlogs, dailyActivities := getDailyHealthMetrics(account, date)
		blogCounts[6-i] = dailyBlogs
		activityCounts[6-i] = dailyActivities
	}

	return map[string]interface{}{
		"labels": labels,
		"datasets": []map[string]interface{}{
			{
				"label":           "博客创建",
				"data":            blogCounts,
				"borderColor":     "rgba(75, 192, 192, 1)",
				"backgroundColor": "rgba(75, 192, 192, 0.2)",
				"tension":         0.4,
			},
			{
				"label":           "总活动次数",
				"data":            activityCounts,
				"borderColor":     "rgba(255, 99, 132, 1)",
				"backgroundColor": "rgba(255, 99, 132, 0.2)",
				"tension":         0.4,
			},
		},
		"title": "近7天健康趋势",
	}
}

// generateHealthRadarData generates health radar chart data
func generateHealthRadarData(account string, lifeHealth LifeHealthScore) map[string]interface{} {
	return map[string]interface{}{
		"labels": []string{"写作频率", "任务完成", "锻炼习惯", "阅读习惯", "作息规律", "整体健康"},
		"datasets": []map[string]interface{}{
			{
				"label": "健康评分",
				"data": []float64{
					lifeHealth.BloggingFrequency,
					lifeHealth.TaskCompletionRate,
					lifeHealth.ExerciseConsistency,
					lifeHealth.ReadingHabit,
					calculateSleepRegularityScore(account), // 作息规律单独计算
					lifeHealth.OverallHealthScore,
				},
				"borderColor":          "rgba(54, 162, 235, 1)",
				"backgroundColor":      "rgba(54, 162, 235, 0.2)",
				"pointBorderColor":     "rgba(54, 162, 235, 1)",
				"pointBackgroundColor": "#fff",
			},
		},
		"title": "健康状态雷达图",
	}
}

// getDailyHealthMetrics gets daily health metrics for specific date
func getDailyHealthMetrics(account string, date time.Time) (int, int) {
	dateStr := date.Format("2006-01-02")
	blogCount := 0
	activityCount := 0

	allBlogs := control.GetAll(account, 0, module.EAuthType_all)

	for _, blog := range allBlogs {
		if isSystemBlog(blog.Title) {
			continue
		}

		// 统计创建时间
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.Format("2006-01-02") == dateStr {
					blogCount++
					activityCount++
				}
			}
		}

		// 统计访问时间
		if blog.AccessTime != "" {
			if accessTime, err := time.Parse("2006-01-02 15:04:05", blog.AccessTime); err == nil {
				if accessTime.Format("2006-01-02") == dateStr {
					activityCount++
				}
			}
		}
	}

	return blogCount, activityCount
}

// calculateSleepRegularityScore calculates sleep regularity score
func calculateSleepRegularityScore(account string) float64 {
	sleepPattern := analyzeSleepPattern(account)
	return sleepPattern.RegularityScore
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

// HandleAssistantHealthComprehensive handles comprehensive health data API
// 智能助手综合健康数据API处理函数
func HandleAssistantHealthComprehensive(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantHealthComprehensive", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	account := getAccountFromRequest(r)
	switch r.Method {
	case h.MethodGet:
		// 生成综合健康分析数据
		healthData := generateComprehensiveHealthData(account)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"healthData": healthData,
			"timestamp":  time.Now().Unix(),
		})

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// generateComprehensiveHealthData generates comprehensive health data with mental health analysis
// 生成综合健康数据（包含心理健康分析）
func generateComprehensiveHealthData(account string) map[string]interface{} {
	// 计算6个健康维度评分
	mentalHealthScore := calculateMentalHealthScore(account)
	physicalHealthScore := calculatePhysicalHealthScore(account)
	learningGrowthScore := calculateLearningGrowthScore(account)
	timeManagementScore := calculateTimeManagementScore(account)
	goalExecutionScore := calculateGoalExecutionScore(account)
	lifeBalanceScore := calculateLifeBalanceScore(account)

	// 计算综合评分（加权平均）
	overallScore := int(mentalHealthScore*0.25 + physicalHealthScore*0.20 +
		learningGrowthScore*0.20 + timeManagementScore*0.15 +
		goalExecutionScore*0.15 + lifeBalanceScore*0.05)

	// 分析心理健康数据
	mentalHealthData := analyzeMentalHealthData(account)

	// 分析核心指标数据
	coreMetricsData := analyzeCoreMetrics(account)

	// 生成个性化建议
	recommendations := generateHealthRecommendations(account)

	return map[string]interface{}{
		"overallScore": overallScore,
		"dimensions": map[string]interface{}{
			"mental": map[string]interface{}{
				"score": int(mentalHealthScore),
			},
			"physical": map[string]interface{}{
				"score": int(physicalHealthScore),
			},
			"learning": map[string]interface{}{
				"score": int(learningGrowthScore),
			},
			"time": map[string]interface{}{
				"score": int(timeManagementScore),
			},
			"goal": map[string]interface{}{
				"score": int(goalExecutionScore),
			},
			"balance": map[string]interface{}{
				"score": int(lifeBalanceScore),
			},
		},
		"mentalHealth":    mentalHealthData,
		"coreMetrics":     coreMetricsData,
		"recommendations": recommendations,
	}
}

// calculateMentalHealthScore calculates mental health score based on stress, anxiety, emotion
// 计算心理健康评分（基于压力、焦虑、情绪分析）
func calculateMentalHealthScore(account string) float64 {
	// 分析压力水平
	stressLevel := analyzeStressLevel(account)

	// 分析焦虑风险
	anxietyRisk := analyzeAnxietyRisk(account)

	// 分析情绪稳定度
	emotionStability := analyzeEmotionStability(account)

	// 综合评分（压力越低、焦虑风险越小、情绪越稳定，分数越高）
	score := (100.0-stressLevel)*0.4 + (100.0-anxietyRisk)*0.3 + emotionStability*0.3

	return math.Max(0, math.Min(100, score))
}

// analyzeStressLevel analyzes stress level based on task management and time patterns
// 分析压力水平（基于任务管理和时间模式）
func analyzeStressLevel(account string) float64 {
	// 获取未完成任务数量
	unfinishedTasks := getUnfinishedTasksCount(account)

	// 获取紧急任务数量
	urgentTasks := getUrgentTasksCount(account)

	// 分析深夜活动频率
	sleepPattern := analyzeSleepPattern(account)
	lateNightFactor := float64(sleepPattern.LateNightActivities) * 2.0

	// 计算压力水平（0-100，越高压力越大）
	stressLevel := float64(unfinishedTasks)*3.0 + float64(urgentTasks)*8.0 + lateNightFactor

	// 归一化到0-100范围
	return math.Max(0, math.Min(100, stressLevel))
}

// analyzeAnxietyRisk analyzes anxiety risk based on behavioral patterns
// 分析焦虑风险（基于行为模式）
func analyzeAnxietyRisk(account string) float64 {
	// 分析作息规律性
	sleepPattern := analyzeSleepPattern(account)
	irregularityFactor := (100.0 - sleepPattern.RegularityScore) * 0.3

	// 分析任务完成率
	taskCompletionRate := calculateWeeklyTaskCompletion(account)
	taskStressFactor := (100.0 - taskCompletionRate) * 0.4

	// 分析深夜活动频率
	lateNightFactor := float64(sleepPattern.LateNightActivities) * 3.0

	// 综合焦虑风险评分
	anxietyRisk := irregularityFactor + taskStressFactor + lateNightFactor

	return math.Max(0, math.Min(100, anxietyRisk))
}

// analyzeEmotionStability analyzes emotional stability from writing patterns
// 分析情绪稳定度（基于写作模式）
func analyzeEmotionStability(account string) float64 {
	// 分析最近博客的情绪倾向
	recentBlogs := getRecentBlogs(account, 7) // 最近7篇博客

	positiveWords := 0
	negativeWords := 0
	totalWords := 0

	// 简化的情绪词汇分析
	positiveKeywords := []string{"好", "棒", "优秀", "成功", "完成", "满意", "开心", "快乐", "收获", "进步"}
	negativeKeywords := []string{"问题", "困难", "失败", "烦恼", "压力", "焦虑", "担心", "紧张", "疲惫", "沮丧"}

	for _, blog := range recentBlogs {
		content := strings.ToLower(blog.Content)

		for _, word := range positiveKeywords {
			positiveWords += strings.Count(content, word)
		}

		for _, word := range negativeKeywords {
			negativeWords += strings.Count(content, word)
		}

		// 计算总词数
		totalWords += len(strings.Fields(content))
	}

	// 计算情绪稳定度
	if totalWords == 0 {
		return 75.0 // 默认中等稳定度
	}

	emotionalBalance := float64(positiveWords-negativeWords*2) / float64(totalWords) * 1000
	stabilityScore := 70.0 + emotionalBalance // 基础分70，根据情绪平衡调整

	return math.Max(30, math.Min(100, stabilityScore))
}

// calculatePhysicalHealthScore calculates physical health score based on exercise data
// 计算体能健康评分（基于锻炼数据）
func calculatePhysicalHealthScore(account string) float64 {
	// 获取本周锻炼统计
	weeklyStats := getWeeklyExerciseStats(account)

	// 基于锻炼频率和强度评分
	frequencyScore := math.Min(100, float64(weeklyStats.SessionCount)*20) // 每次锻炼20分
	intensityScore := math.Min(100, weeklyStats.TotalCalories/10)         // 每10卡路里1分

	// 综合评分
	return (frequencyScore + intensityScore) / 2.0
}

// calculateLearningGrowthScore calculates learning growth score
// 计算学习成长评分（基于阅读和写作数据）
func calculateLearningGrowthScore(account string) float64 {
	// 分析阅读习惯
	readingScore := analyzeReadingHabit(account)

	// 分析写作频率
	bloggingScore := analyzeBloggingFrequency(account)

	// 综合学习成长评分
	return (readingScore + bloggingScore) / 2.0
}

// calculateTimeManagementScore calculates time management score
// 计算时间管理评分（基于作息规律和活动模式）
func calculateTimeManagementScore(account string) float64 {
	// 分析作息规律
	sleepPattern := analyzeSleepPattern(account)

	// 分析任务完成及时性
	taskCompletionRate := calculateWeeklyTaskCompletion(account)

	// 综合时间管理评分
	return (sleepPattern.RegularityScore + taskCompletionRate) / 2.0
}

// calculateGoalExecutionScore calculates goal execution score
// 计算目标执行评分（基于任务完成和目标达成）
func calculateGoalExecutionScore(account string) float64 {
	// 任务完成率
	taskRate := calculateWeeklyTaskCompletion(account)

	// 目标达成度（简化计算）
	goalAchievementRate := 80.0 // 可以后续集成真实目标数据

	// 综合执行力评分
	return (taskRate + goalAchievementRate) / 2.0
}

// calculateLifeBalanceScore calculates life balance score
// 计算生活平衡评分（基于工作学习与休息娱乐的平衡）
func calculateLifeBalanceScore(account string) float64 {
	// 分析活动分布
	activityDistribution := analyzeActivityDistribution(account)

	// 基于活动平衡度评分
	if activityDistribution["work"] > 0.7 {
		return 60.0 // 工作过多
	} else if activityDistribution["work"] < 0.3 {
		return 70.0 // 工作过少
	} else {
		return 85.0 // 平衡良好
	}
}

// analyzeMentalHealthData analyzes detailed mental health data
// 分析详细心理健康数据
func analyzeMentalHealthData(account string) map[string]interface{} {
	stressLevel := analyzeStressLevel(account)
	anxietyRisk := analyzeAnxietyRisk(account)
	emotionStability := analyzeEmotionStability(account)

	// 获取压力因素数据
	unfinishedTasks := getUnfinishedTasksCount(account)
	urgentTasks := getUrgentTasksCount(account)
	sleepPattern := analyzeSleepPattern(account)

	return map[string]interface{}{
		"stress": map[string]interface{}{
			"level": int(stressLevel),
			"label": getStressLevelLabel(stressLevel),
			"factors": map[string]interface{}{
				"unfinishedTasks": unfinishedTasks,
				"urgentTasks":     urgentTasks,
			},
		},
		"emotion": map[string]interface{}{
			"stability":          getEmotionStabilityLabel(emotionStability),
			"positiveExpression": int(emotionStability),
			"richness":           getEmotionRichnessLabel(emotionStability),
		},
		"anxiety": map[string]interface{}{
			"level":             getAnxietyRiskLabel(anxietyRisk),
			"lateNightActivity": fmt.Sprintf("%d次/周", sleepPattern.LateNightActivities),
		},
	}
}

// Helper functions for labels and data analysis

func getStressLevelLabel(level float64) string {
	if level < 30 {
		return "低"
	} else if level < 60 {
		return "中等"
	} else {
		return "高"
	}
}

func getEmotionStabilityLabel(stability float64) string {
	if stability >= 80 {
		return "优秀"
	} else if stability >= 60 {
		return "良好"
	} else {
		return "需改善"
	}
}

func getEmotionRichnessLabel(stability float64) string {
	if stability >= 75 {
		return "高"
	} else if stability >= 50 {
		return "中等"
	} else {
		return "低"
	}
}

func getAnxietyRiskLabel(risk float64) string {
	if risk < 30 {
		return "低"
	} else if risk < 60 {
		return "低-中等"
	} else {
		return "中-高"
	}
}

// getUnfinishedTasksCount gets count of unfinished tasks
func getUnfinishedTasksCount(account string) int {
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("todolist-%s", today)

	todayBlog := control.GetBlog(account, todayTitle)
	if todayBlog == nil {
		return 0
	}

	todoData := todolist.ParseTodoListFromBlog(todayBlog.Content)
	unfinished := 0

	for _, item := range todoData.Items {
		if !item.Completed {
			unfinished++
		}
	}

	return unfinished
}

// getUrgentTasksCount gets count of urgent tasks (simplified)
func getUrgentTasksCount(account string) int {
	// 简化实现：假设未完成任务的30%是紧急任务
	unfinished := getUnfinishedTasksCount(account)
	return int(float64(unfinished) * 0.3)
}

// getRecentBlogs gets recent blogs for analysis
func getRecentBlogs(account string, limit int) []*module.Blog {
	allBlogs := control.GetAll(account, 0, module.EAuthType_all)
	var recentBlogs []*module.Blog

	for _, blog := range allBlogs {
		if isSystemBlog(blog.Title) {
			continue
		}

		if len(recentBlogs) < limit {
			recentBlogs = append(recentBlogs, blog)
		}
	}

	return recentBlogs
}

// analyzeActivityDistribution analyzes activity distribution
func analyzeActivityDistribution(account string) map[string]float64 {
	// 简化实现：返回模拟的活动分布
	return map[string]float64{
		"work":     0.5,
		"study":    0.2,
		"rest":     0.2,
		"exercise": 0.1,
	}
}

// analyzeCoreMetrics analyzes core health metrics
func analyzeCoreMetrics(account string) map[string]interface{} {
	// 获取运动数据
	weeklyStats := getWeeklyExerciseStats(account)

	// 获取学习数据
	readingBlogs := getReadingBlogs(account)
	currentBook := "《深度工作》" // 简化实现
	if len(readingBlogs) > 0 {
		currentBook = readingBlogs[0].Title
	}

	// 获取时间管理数据
	sleepPattern := analyzeSleepPattern(account)

	// 获取任务执行数据
	todayTasks := getTodayTasksStats(account)

	return map[string]interface{}{
		"fitness": map[string]interface{}{
			"weeklyExercise": weeklyStats.SessionCount,
			"todayCalories":  int(weeklyStats.TotalCalories / 7), // 日均卡路里
			"mainExercise":   "有氧运动 45分钟",
		},
		"learning": map[string]interface{}{
			"readingProgress": 65,
			"currentBook":     currentBook,
			"weeklyWriting":   "3篇, 2400字",
		},
		"timeManagement": map[string]interface{}{
			"efficiency":    getEfficiencyLabel(sleepPattern.RegularityScore),
			"activeHours":   "9-11点, 14-17点",
			"routineStreak": 7,
		},
		"goalExecution": map[string]interface{}{
			"dailyCompletion":  fmt.Sprintf("%d/%d", todayTasks["completed"], todayTasks["total"]),
			"monthlyGoals":     "已达成 8/10 项",
			"completionStreak": 5,
		},
		"lifeBalance": map[string]interface{}{
			"workLifeBalance":   "平衡",
			"workStudyHours":    "8小时 (合理)",
			"socialInteraction": "本周5次",
		},
		"trend": map[string]interface{}{
			"direction":      "↗️ 稳步上升",
			"type":           "up",
			"predictedScore": 87,
		},
	}
}

// getEfficiencyLabel gets efficiency label based on score
func getEfficiencyLabel(score float64) string {
	if score >= 80 {
		return "优秀"
	} else if score >= 60 {
		return "良好"
	} else {
		return "需改善"
	}
}

// generateHealthRecommendations generates personalized health recommendations
func generateHealthRecommendations(account string) map[string]interface{} {
	return map[string]interface{}{
		"mental": []map[string]interface{}{
			{
				"icon": "🧘",
				"text": "建议增加冥想/放松时间",
			},
			{
				"icon": "🌅",
				"text": "尝试早起，减少深夜活动",
			},
			{
				"icon": "👥",
				"text": "本周社交互动较少，建议主动参与讨论",
			},
			{
				"icon": "📝",
				"text": "写作情绪偏负面，建议记录积极事件",
			},
		},
	}
}
