package http

import (
	"control"
	"fmt"
	"math"
	"module"
	"sort"
	"strings"
	"time"
)

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
