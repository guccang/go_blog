package agent

import (
	"codegen"
	"config"
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
	// 先检查是否已有日报/周报任务（从持久化加载的），避免每次重启重复创建
	existingReminders := rg.scheduler.GetReminders(rg.account)
	hasDailyReport := false
	hasWeeklyReport := false
	for _, r := range existingReminders {
		if r.Title == "📊 日报生成" && r.Enabled {
			hasDailyReport = true
		}
		if r.Title == "📊 周报生成" && r.Enabled {
			hasWeeklyReport = true
		}
	}

	if hasDailyReport && hasWeeklyReport {
		log.Message(log.ModuleAgent, "Report schedules already loaded from persistence, skipping creation")
		return
	}

	if !hasDailyReport {
		// 日报：每天 21:00，使用 cron 表达式避免漂移
		dailyReminder := rg.scheduler.AddCronReminder(rg.account, "📊 日报生成", "自动生成今日日报", "0 0 21 * * *", -1)
		dailyReminder.SmartMode = false
		log.Message(log.ModuleAgent, "Scheduled daily report at 21:00 (cron)")
	}

	if !hasWeeklyReport {
		// 周报：每周日 20:00，使用 cron 表达式
		weeklyReminder := rg.scheduler.AddCronReminder(rg.account, "📊 周报生成", "自动生成本周周报", "0 0 20 * * 0", -1)
		weeklyReminder.SmartMode = false
		log.Message(log.ModuleAgent, "Scheduled weekly report on Sunday 20:00 (cron)")
	}
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

	prompt := config.SafeSprintf(config.GetPrompt(account, "daily_report"), today, todoData, exerciseData, exerciseStats, readingStats, taskStats)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	report, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Daily report generation failed: %v", err)
		return "", err
	}

	// 保存为博客
	title := fmt.Sprintf("agent_report_daily_%s", today)
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

	prompt := config.SafeSprintf(config.GetPrompt(account, "weekly_report"), weekStart, weekEnd, todoData, exerciseStats, exerciseRange, readingStats, taskStats)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	report, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Weekly report generation failed: %v", err)
		return "", err
	}

	title := fmt.Sprintf("agent_report_weekly_%s_%s", weekStart, weekEnd)
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

	prompt := config.SafeSprintf(config.GetPrompt(account, "monthly_report"), year, month, todoData, exerciseStats, readingStats, yearGoal, taskStats)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	report, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		return "", err
	}

	title := fmt.Sprintf("agent_report_monthly_%d-%02d", year, month)
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

	// 企业微信推送（通过 gateway → wechat-agent）
	if codegen.IsGatewayConnected() {
		go func() {
			wechatSummary := content
			if len(wechatSummary) > 500 {
				wechatSummary = wechatSummary[:500] + "\n...(完整报告请查看博客)"
			}
			wechatMsg := fmt.Sprintf("📊 %s已生成\n\n%s", reportType, wechatSummary)
			if err := codegen.SendWechatNotify("@all", wechatMsg); err != nil {
				log.WarnF(log.ModuleAgent, "WeChat report push via gateway failed: %v", err)
			}
		}()
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
