package http

import (
	"control"
	"encoding/json"
	"exercise"
	"fmt"
	"module"
	log "mylog"
	h "net/http"
	"reading"
	"sort"
	"strings"
	"time"
	"todolist"
)

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
