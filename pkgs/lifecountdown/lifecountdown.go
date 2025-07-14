package lifecountdown

import (
	"fmt"
	log "mylog"
	"time"
	"math"
)

func Info() {
	log.Debug("info lifecountdown v1.0")
}

// 人生倒计时数据结构
type LifeCountdownData struct {
	CurrentAge     int     `json:"current_age"`
	ExpectedLifespan int   `json:"expected_lifespan"`
	TotalDays      int     `json:"total_days"`
	PassedDays     int     `json:"passed_days"`
	RemainingDays  int     `json:"remaining_days"`
	PassedPercent  float64 `json:"passed_percent"`
	RemainingPercent float64 `json:"remaining_percent"`
	
	// 时间分配
	SleepDays      int     `json:"sleep_days"`
	StudyDays      int     `json:"study_days"`
	WorkDays       int     `json:"work_days"`
	RestDays       int     `json:"rest_days"`
	GoldenDays     int     `json:"golden_days"`
	
	// 阅读分析
	TotalReadingHours int     `json:"total_reading_hours"`
	BooksCanRead    int     `json:"books_can_read"`
	ReadingSpeed    int     `json:"reading_speed"` // 字/分钟
	AverageBookWords int     `json:"average_book_words"`
	
	// 个性化设置
	DailySleepHours float64 `json:"daily_sleep_hours"`
	DailyStudyHours float64 `json:"daily_study_hours"`
	DailyReadingHours float64 `json:"daily_reading_hours"`
	DailyWorkHours   float64 `json:"daily_work_hours"`
}

// 用户配置
type UserConfig struct {
	CurrentAge      int     `json:"current_age"`
	ExpectedLifespan int    `json:"expected_lifespan"`
	DailySleepHours float64 `json:"daily_sleep_hours"`
	DailyStudyHours float64 `json:"daily_study_hours"`
	DailyReadingHours float64 `json:"daily_reading_hours"`
	DailyWorkHours   float64 `json:"daily_work_hours"`
	ReadingSpeed     int     `json:"reading_speed"`
	AverageBookWords int     `json:"average_book_words"`
}

// 默认配置
var DefaultConfig = UserConfig{
	CurrentAge:        25,
	ExpectedLifespan:  80,
	DailySleepHours:   8.0,
	DailyStudyHours:   2.0,
	DailyReadingHours: 1.0,
	DailyWorkHours:    8.0,
	ReadingSpeed:      300,
	AverageBookWords:  150000,
}

// 计算人生倒计时数据
func CalculateLifeCountdown(config UserConfig) *LifeCountdownData {
	data := &LifeCountdownData{}
	
	// 基本时间计算
	data.CurrentAge = config.CurrentAge
	data.ExpectedLifespan = config.ExpectedLifespan
	data.TotalDays = config.ExpectedLifespan * 365
	data.PassedDays = config.CurrentAge * 365
	data.RemainingDays = data.TotalDays - data.PassedDays
	
	// 百分比计算
	data.PassedPercent = float64(data.PassedDays) / float64(data.TotalDays) * 100
	data.RemainingPercent = 100 - data.PassedPercent
	
	// 时间分配计算
	calculateTimeAllocation(data, config)
	
	// 阅读能力分析
	calculateReadingAnalysis(data, config)
	
	return data
}

// 计算时间分配
func calculateTimeAllocation(data *LifeCountdownData, config UserConfig) {
	// 睡眠时间
	sleepHoursPerDay := config.DailySleepHours
	data.SleepDays = int(float64(data.TotalDays) * sleepHoursPerDay / 24.0)
	
	// 学习时间 (18岁前 + 工作学习)
	studyYears := 18
	if data.CurrentAge < 18 {
		studyYears = data.CurrentAge
	}
	studyDays := studyYears * 365
	workStudyDays := int(float64(data.RemainingDays) * config.DailyStudyHours / 24.0)
	data.StudyDays = studyDays + workStudyDays
	
	// 工作时间 (25-65岁)
	workStartAge := 25
	workEndAge := 65
	workYears := 0
	if data.CurrentAge >= workStartAge && data.CurrentAge <= workEndAge {
		workYears = workEndAge - data.CurrentAge
	} else if data.CurrentAge < workStartAge {
		workYears = workEndAge - workStartAge
	}
	data.WorkDays = workYears * 365
	
	// 休息娱乐时间
	restHoursPerDay := 24.0 - sleepHoursPerDay - config.DailyStudyHours - config.DailyWorkHours
	if restHoursPerDay < 0 {
		restHoursPerDay = 2.0 // 最少2小时休息时间
	}
	data.RestDays = int(float64(data.TotalDays) * restHoursPerDay / 24.0)
	
	// 黄金时间 (18-45岁)
	goldenStartAge := 18
	goldenEndAge := 45
	goldenYears := 0
	if data.CurrentAge >= goldenStartAge && data.CurrentAge <= goldenEndAge {
		goldenYears = goldenEndAge - data.CurrentAge
	} else if data.CurrentAge < goldenStartAge {
		goldenYears = goldenEndAge - goldenStartAge
	}
	data.GoldenDays = goldenYears * 365
}

// 计算阅读能力分析
func calculateReadingAnalysis(data *LifeCountdownData, config UserConfig) {
	// 总阅读时间 (小时)
	data.TotalReadingHours = int(float64(data.RemainingDays) * config.DailyReadingHours)
	
	// 阅读速度设置
	data.ReadingSpeed = config.ReadingSpeed
	data.AverageBookWords = config.AverageBookWords
	
	// 可读书籍数量
	wordsPerMinute := config.ReadingSpeed
	minutesPerBook := config.AverageBookWords / wordsPerMinute
	hoursPerBook := float64(minutesPerBook) / 60.0
	
	if hoursPerBook > 0 {
		data.BooksCanRead = int(float64(data.TotalReadingHours) / hoursPerBook)
	} else {
		data.BooksCanRead = 0
	}
}

// 获取时间分配百分比
func GetTimeAllocationPercentages(data *LifeCountdownData) map[string]float64 {
	totalDays := float64(data.TotalDays)
	
	return map[string]float64{
		"sleep": float64(data.SleepDays) / totalDays * 100,
		"study": float64(data.StudyDays) / totalDays * 100,
		"work":  float64(data.WorkDays) / totalDays * 100,
		"rest":  float64(data.RestDays) / totalDays * 100,
		"golden": float64(data.GoldenDays) / totalDays * 100,
	}
}

// 获取阅读建议
func GetReadingAdvice(data *LifeCountdownData) map[string]interface{} {
	advice := make(map[string]interface{})
	
	// 阅读速度建议
	if data.ReadingSpeed < 200 {
		advice["speed_suggestion"] = "建议通过速读训练提高阅读速度"
	} else if data.ReadingSpeed > 500 {
		advice["speed_suggestion"] = "您的阅读速度很快，可以尝试深度阅读"
	} else {
		advice["speed_suggestion"] = "您的阅读速度适中，保持良好"
	}
	
	// 阅读时间建议
	dailyReadingHours := float64(data.TotalReadingHours) / float64(data.RemainingDays)
	if dailyReadingHours < 0.5 {
		advice["time_suggestion"] = "建议每天至少阅读30分钟"
	} else if dailyReadingHours > 2.0 {
		advice["time_suggestion"] = "您的阅读时间很充足，注意保护眼睛"
	} else {
		advice["time_suggestion"] = "您的阅读时间安排合理"
	}
	
	// 目标建议
	advice["books_target"] = data.BooksCanRead
	advice["daily_target"] = math.Ceil(float64(data.AverageBookWords) / float64(data.ReadingSpeed) / 60.0)
	
	return advice
}

// 格式化时间显示
func FormatTimeDisplay(days int) string {
	years := days / 365
	remainingDays := days % 365
	months := remainingDays / 30
	remainingDays = remainingDays % 30
	
	if years > 0 {
		return fmt.Sprintf("%d年%d个月%d天", years, months, remainingDays)
	} else if months > 0 {
		return fmt.Sprintf("%d个月%d天", months, remainingDays)
	} else {
		return fmt.Sprintf("%d天", remainingDays)
	}
}

// 获取当前时间进度
func GetCurrentProgress(config UserConfig) map[string]interface{} {
	now := time.Now()
	birthYear := now.Year() - config.CurrentAge
	birthDate := time.Date(birthYear, 1, 1, 0, 0, 0, 0, now.Location())
	
	elapsed := now.Sub(birthDate)
	totalLifespan := time.Duration(config.ExpectedLifespan) * 365 * 24 * time.Hour
	
	progress := elapsed.Seconds() / totalLifespan.Seconds() * 100
	
	return map[string]interface{}{
		"elapsed_seconds": elapsed.Seconds(),
		"total_seconds":   totalLifespan.Seconds(),
		"progress_percent": progress,
		"remaining_seconds": totalLifespan.Seconds() - elapsed.Seconds(),
	}
} 