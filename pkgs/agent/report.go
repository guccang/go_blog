package agent

import (
	"email"
	"fmt"
	"llm"
	log "mylog"
	"statistics"
	"time"
)

// ReportGenerator 自动报告生成器
type ReportGenerator struct {
	scheduler *Scheduler
	hub       *NotificationHub
	account   string
}

var globalReportGen *ReportGenerator

// NewReportGenerator 创建报告生成器
func NewReportGenerator(scheduler *Scheduler, hub *NotificationHub, account string) *ReportGenerator {
	return &ReportGenerator{
		scheduler: scheduler,
		hub:       hub,
		account:   account,
	}
}

// InitReportGenerator 初始化全局报告生成器并注册定时任务
func InitReportGenerator(account string) {
	if globalScheduler == nil || globalHub == nil {
		log.Warn(log.ModuleAgent, "Cannot init ReportGenerator: scheduler or hub not ready")
		return
	}

	globalReportGen = NewReportGenerator(globalScheduler, globalHub, account)
	globalReportGen.ScheduleReports()
	log.Message(log.ModuleAgent, "ReportGenerator initialized with scheduled reports")
}

// ScheduleReports 注册定时报告任务
func (rg *ReportGenerator) ScheduleReports() {
	// 日报：每天 21:00（每天触发一次，间隔 86400 秒）
	// 计算到今天 21:00 的秒数
	now := time.Now()
	dailyTarget := time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location())
	if now.After(dailyTarget) {
		dailyTarget = dailyTarget.Add(24 * time.Hour)
	}
	dailyDelay := int(dailyTarget.Sub(now).Seconds())

	dailyReminder := rg.scheduler.AddReminder(rg.account, "📊 日报生成", "自动生成今日日报", dailyDelay, -1)
	dailyReminder.SmartMode = false // 报告用专门逻辑，不用SmartMode
	dailyReminder.Interval = 86400  // 之后每24小时触发一次

	// 周报：每周日 20:00
	weekdayDiff := (7 - int(now.Weekday())) % 7
	if weekdayDiff == 0 && now.Hour() >= 20 {
		weekdayDiff = 7
	}
	weeklyTarget := time.Date(now.Year(), now.Month(), now.Day()+weekdayDiff, 20, 0, 0, 0, now.Location())
	weeklyDelay := int(weeklyTarget.Sub(now).Seconds())

	weeklyReminder := rg.scheduler.AddReminder(rg.account, "📊 周报生成", "自动生成本周周报", weeklyDelay, -1)
	weeklyReminder.SmartMode = false
	weeklyReminder.Interval = 604800 // 7天

	log.MessageF(log.ModuleAgent, "Scheduled daily report at 21:00 (in %ds), weekly report on Sunday 20:00 (in %ds)", dailyDelay, weeklyDelay)
}

// GenerateDailyReport 生成日报
func (rg *ReportGenerator) GenerateDailyReport(account string) (string, error) {
	today := time.Now().Format("2006-01-02")
	log.MessageF(log.ModuleAgent, "Generating daily report for %s on %s", account, today)

	// 收集各模块数据
	todoData := statistics.RawGetTodosByDate(account, today)
	exerciseData := statistics.RawGetExerciseByDate(account, today)
	exerciseStats := statistics.RawGetExerciseStats(account, 1)
	readingStats := statistics.RawGetReadingStats(account)
	taskStats := statistics.RawGetComplexTaskStats(account)

	prompt := fmt.Sprintf(`你是一个智能报告助手。请根据以下数据生成一份简洁的日报。

日期: %s

## 今日数据

### 待办事项
%s

### 运动记录
%s

### 运动统计
%s

### 阅读情况
%s

### 任务进度
%s

## 报告要求
1. 用 Markdown 格式输出
2. 包含以下部分：今日总结、完成情况、运动数据、阅读进展、明日建议
3. 语气专业但友好
4. 如果某部分没有数据，简要说明即可
5. 在末尾给出1-2条针对性的改进建议`, today, todoData, exerciseData, exerciseStats, readingStats, taskStats)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	report, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Daily report generation failed: %v", err)
		return "", err
	}

	// 保存为博客
	title := fmt.Sprintf("日报-%s", today)
	saveResult := statistics.RawCreateBlog(account, title, report, "日报,自动生成", 2, 0)
	log.MessageF(log.ModuleAgent, "Daily report saved: %s, result: %s", title, saveResult)

	// 推送通知
	rg.notifyReport(account, "日报", title, report)

	return report, nil
}

// GenerateWeeklyReport 生成周报
func (rg *ReportGenerator) GenerateWeeklyReport(account string) (string, error) {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -6).Format("2006-01-02")
	weekEnd := now.Format("2006-01-02")
	log.MessageF(log.ModuleAgent, "Generating weekly report for %s: %s to %s", account, weekStart, weekEnd)

	// 收集一周数据
	todoData := statistics.RawGetTodosRange(account, weekStart, weekEnd)
	exerciseStats := statistics.RawGetExerciseStats(account, 7)
	exerciseRange := statistics.RawGetExerciseRange(account, weekStart, weekEnd)
	readingStats := statistics.RawGetReadingStats(account)
	taskStats := statistics.RawGetComplexTaskStats(account)

	prompt := fmt.Sprintf(`你是一个智能报告助手。请根据以下数据生成一份详细的周报。

周期: %s 至 %s

## 本周数据

### 待办事项（本周所有）
%s

### 运动统计（7天）
%s

### 运动详情
%s

### 阅读情况
%s

### 任务进度
%s

## 报告要求
1. 用 Markdown 格式输出
2. 包含：本周总结、待办完成率分析、运动趋势、阅读进展、任务推进、下周计划建议
3. 对比上周数据给出趋势分析（如果有的话）
4. 给出2-3条具体可执行的改进建议
5. 语气专业、有洞察力`, weekStart, weekEnd, todoData, exerciseStats, exerciseRange, readingStats, taskStats)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	report, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Weekly report generation failed: %v", err)
		return "", err
	}

	title := fmt.Sprintf("周报-%s至%s", weekStart, weekEnd)
	saveResult := statistics.RawCreateBlog(account, title, report, "周报,自动生成", 2, 0)
	log.MessageF(log.ModuleAgent, "Weekly report saved: %s, result: %s", title, saveResult)

	rg.notifyReport(account, "周报", title, report)
	return report, nil
}

// GenerateMonthlyReport 生成月报
func (rg *ReportGenerator) GenerateMonthlyReport(account string) (string, error) {
	now := time.Now()
	year, month := now.Year(), int(now.Month())
	monthStart := fmt.Sprintf("%d-%02d-01", year, month)
	monthEnd := now.Format("2006-01-02")
	log.MessageF(log.ModuleAgent, "Generating monthly report for %s: %d-%02d", account, year, month)

	todoData := statistics.RawGetTodosRange(account, monthStart, monthEnd)
	exerciseStats := statistics.RawGetExerciseStats(account, 30)
	readingStats := statistics.RawGetReadingStats(account)
	yearGoal := statistics.RawGetMonthGoal(account, year, month)
	taskStats := statistics.RawGetComplexTaskStats(account)

	prompt := fmt.Sprintf(`你是一个智能报告助手。请根据以下数据生成一份全面的月报。

月份: %d年%d月

### 待办数据
%s

### 运动统计（30天）
%s

### 阅读情况
%s

### 本月目标
%s

### 任务进度
%s

## 报告要求
1. Markdown 格式
2. 包含：月度总结、目标达成率、运动/阅读分析、关键成就、不足与改进
3. 给出下月目标调整建议
4. 数据驱动，有具体数字`, year, month, todoData, exerciseStats, readingStats, yearGoal, taskStats)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	report, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		return "", err
	}

	title := fmt.Sprintf("月报-%d年%02d月", year, month)
	statistics.RawCreateBlog(account, title, report, "月报,自动生成", 2, 0)
	rg.notifyReport(account, "月报", title, report)
	return report, nil
}

// notifyReport 推送报告通知
func (rg *ReportGenerator) notifyReport(account, reportType, title, content string) {
	// Browser 推送
	if rg.hub != nil {
		notification := TaskNotification{
			Type:    "report_generated",
			Message: fmt.Sprintf("📊 %s已生成: %s", reportType, title),
			Data: map[string]interface{}{
				"type":  reportType,
				"title": title,
				"link":  fmt.Sprintf("/get?blogname=%s", title),
			},
		}
		rg.hub.BroadcastToAccount(account, notification)
	}

	// Email 推送
	if email.IsEnabled() {
		subject := fmt.Sprintf("📊 %s: %s", reportType, title)
		// 截取前500字作为邮件摘要
		summary := content
		if len(summary) > 500 {
			summary = summary[:500] + "\n\n...(完整报告请查看博客)"
		}
		htmlBody := fmt.Sprintf(`
<div style="font-family: 'Segoe UI', Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <div style="background: linear-gradient(135deg, #11998e 0%%, #38ef7d 100%%); padding: 20px; border-radius: 10px; color: white;">
    <h2 style="margin: 0;">📊 %s</h2>
    <p style="margin: 5px 0 0; opacity: 0.8;">%s</p>
  </div>
  <div style="padding: 20px; background: #f8f9fa; border-radius: 0 0 10px 10px;">
    <pre style="font-size: 14px; line-height: 1.6; color: #333; white-space: pre-wrap;">%s</pre>
    <hr style="border: none; border-top: 1px solid #dee2e6; margin: 15px 0;">
    <p style="font-size: 12px; color: #999;">此邮件由 GoBlog 智能报告系统自动发送</p>
  </div>
</div>`, title, time.Now().Format("2006-01-02 15:04"), summary)
		go email.SendHTMLEmail("", subject, htmlBody)
	}
}

// === 全局函数（MCP 工具接口） ===

// GenerateReport 手动触发报告生成
func GenerateReport(account string, reportType string) (string, error) {
	if globalReportGen == nil {
		return "", fmt.Errorf("ReportGenerator not initialized")
	}

	switch reportType {
	case "daily":
		return globalReportGen.GenerateDailyReport(account)
	case "weekly":
		return globalReportGen.GenerateWeeklyReport(account)
	case "monthly":
		return globalReportGen.GenerateMonthlyReport(account)
	default:
		return "", fmt.Errorf("unknown report type: %s, use: daily/weekly/monthly", reportType)
	}
}
