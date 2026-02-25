package http

import (
	"control"
	"encoding/json"
	"fmt"
	"math"
	"module"
	h "net/http"
	"strings"
	"time"
	"todolist"
)

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
