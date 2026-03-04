package agent

import (
	"config"
	"control"
	"email"
	"encoding/json"
	"fmt"
	"llm"
	"mcp"
	"module"
	log "mylog"
	"statistics"
	"strings"
	"sync"
	"time"
	"codegen"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ============================================================================
// 定时任务调度器 — 使用 robfig/cron 库 + 博客持久化
// ============================================================================

const scheduledTasksBlogPrefix = "scheduled_tasks_"

// Reminder 定时任务
type Reminder struct {
	ID           string    `json:"id"`
	Account      string    `json:"account"`
	Title        string    `json:"title"`
	Message      string    `json:"message"`
	Cron         string    `json:"cron"`     // Cron 表达式: "0 21 * * *" = 每天21:00
	Interval     int       `json:"interval"` // 间隔秒数 (简单模式，与 Cron 二选一)
	NextRunTime  time.Time `json:"next_run_at"`
	LastRunTime  time.Time `json:"last_run_at,omitempty"`
	Enabled      bool      `json:"enabled"`
	RepeatCount  int       `json:"repeat_count"`          // -1 = 无限, 0 = 已完成, >0 = 剩余次数
	RunCount     int       `json:"run_count"`             // 已执行次数
	LinkedTaskID string    `json:"linked_task_id"`        // 关联的任务ID
	SmartMode    bool      `json:"smart_mode"`            // 是否启用 AI 智能消息
	AIQuery      string    `json:"ai_query,omitempty"`    // AI 查询（定时执行 AI 任务）
	SaveResult   bool      `json:"save_result,omitempty"` // 是否保存 AI 结果到博客
	CreatedAt    time.Time `json:"created_at"`

	cronEntryID cron.EntryID `json:"-"` // cron 内部 ID，不序列化
}

// Scheduler 定时调度器 (基于 robfig/cron)
type Scheduler struct {
	cronEngine *cron.Cron
	reminders  map[string]*Reminder
	mu         sync.RWMutex
	hub        *NotificationHub
	running    bool
}

var globalScheduler *Scheduler

// NewScheduler 创建调度器
func NewScheduler(hub *NotificationHub) *Scheduler {
	return &Scheduler{
		cronEngine: cron.New(cron.WithSeconds()), // 支持秒级精度
		reminders:  make(map[string]*Reminder),
		hub:        hub,
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.cronEngine.Start()
	s.running = true
	log.Message(log.ModuleAgent, "Scheduler started (cron engine)")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.running {
		s.cronEngine.Stop()
		s.running = false
		log.Message(log.ModuleAgent, "Scheduler stopped")
	}
}

// ============================================================================
// 持久化：保存/加载提醒到博客
// ============================================================================

// SaveReminders 保存所有提醒到博客
func (s *Scheduler) SaveReminders(account string) {
	s.mu.RLock()
	var reminders []*Reminder
	for _, r := range s.reminders {
		if r.Account == account {
			reminders = append(reminders, r)
		}
	}
	s.mu.RUnlock()

	jsonBytes, err := json.MarshalIndent(reminders, "", "  ")
	if err != nil {
		log.ErrorF(log.ModuleAgent, "Failed to marshal reminders: %v", err)
		return
	}

	blogTitle := scheduledTasksBlogPrefix + account
	existingBlog := control.GetBlog(account, blogTitle)
	if existingBlog != nil {
		blogData := &module.UploadedBlogData{
			Title:    blogTitle,
			Content:  string(jsonBytes),
			Tags:     existingBlog.Tags,
			AuthType: existingBlog.AuthType,
		}
		control.ModifyBlog(account, blogData)
	} else {
		blogData := &module.UploadedBlogData{
			Title:    blogTitle,
			Content:  string(jsonBytes),
			Tags:     "定时任务|自动生成",
			AuthType: module.EAuthType_private,
		}
		control.AddBlog(account, blogData)
	}

	log.DebugF(log.ModuleAgent, "Saved %d reminders for %s", len(reminders), account)
}

// LoadReminders 从博客加载提醒
func (s *Scheduler) LoadReminders(account string) {
	blogTitle := scheduledTasksBlogPrefix + account
	blog := control.GetBlog(account, blogTitle)
	if blog == nil {
		return
	}

	var reminders []*Reminder
	if err := json.Unmarshal([]byte(blog.Content), &reminders); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to parse saved reminders: %v", err)
		return
	}

	// 去重：同一 Title 只保留最新创建的一个（修复历史重复任务问题）
	titleLatest := make(map[string]*Reminder) // title -> latest reminder
	for _, r := range reminders {
		if !r.Enabled {
			continue
		}
		if existing, ok := titleLatest[r.Title]; !ok || r.CreatedAt.After(existing.CreatedAt) {
			titleLatest[r.Title] = r
		}
	}

	loaded := 0
	dedupReminders := make([]*Reminder, 0, len(titleLatest))
	for _, r := range titleLatest {
		s.registerCronJob(r)
		s.mu.Lock()
		s.reminders[r.ID] = r
		s.mu.Unlock()
		dedupReminders = append(dedupReminders, r)
		loaded++
	}

	if len(dedupReminders) < len(reminders) {
		removed := len(reminders) - len(dedupReminders)
		log.MessageF(log.ModuleAgent, "Deduplicated scheduled tasks: removed %d duplicates for %s", removed, account)
		// 保存去重后的结果
		go s.SaveReminders(account)
	}

	log.MessageF(log.ModuleAgent, "Loaded %d scheduled tasks for %s from blog", loaded, account)
}

// ============================================================================
// 核心：添加/删除/触发提醒
// ============================================================================

// registerCronJob 将提醒注册到 cron 引擎
func (s *Scheduler) registerCronJob(r *Reminder) {
	cronSpec := r.Cron
	if cronSpec == "" && r.Interval > 0 {
		// 将间隔秒数转换为 cron 表达式
		cronSpec = fmt.Sprintf("@every %ds", r.Interval)
	}
	if cronSpec == "" {
		log.WarnF(log.ModuleAgent, "Reminder %s has no cron or interval, skipping", r.ID)
		return
	}

	reminderID := r.ID // 闭包捕获
	entryID, err := s.cronEngine.AddFunc(cronSpec, func() {
		s.triggerReminder(reminderID)
	})
	if err != nil {
		log.ErrorF(log.ModuleAgent, "Failed to add cron job for %s (%s): %v", r.ID, cronSpec, err)
		return
	}

	r.cronEntryID = entryID
	log.DebugF(log.ModuleAgent, "Cron job registered: %s [%s] -> entryID %d", r.ID, cronSpec, entryID)
}

// AddReminder 添加提醒
func (s *Scheduler) AddReminder(account, title, message string, intervalSeconds int, repeatCount int) *Reminder {
	return s.AddReminderWithTask(account, title, message, intervalSeconds, repeatCount, "")
}

// AddReminderWithTask 添加提醒并关联任务
func (s *Scheduler) AddReminderWithTask(account, title, message string, intervalSeconds int, repeatCount int, linkedTaskID string) *Reminder {
	id := uuid.New().String()[:8]
	reminder := &Reminder{
		ID:           id,
		Account:      account,
		Title:        title,
		Message:      message,
		Interval:     intervalSeconds,
		Enabled:      true,
		RepeatCount:  repeatCount,
		RunCount:     0,
		LinkedTaskID: linkedTaskID,
		CreatedAt:    time.Now(),
	}

	s.registerCronJob(reminder)

	s.mu.Lock()
	s.reminders[id] = reminder
	s.mu.Unlock()

	// 持久化
	go s.SaveReminders(account)

	log.MessageF(log.ModuleAgent, "Reminder added: %s every %d seconds", id, intervalSeconds)
	return reminder
}

// AddCronReminder 使用 Cron 表达式添加提醒
func (s *Scheduler) AddCronReminder(account, title, message, cronExpr string, repeatCount int) *Reminder {
	id := uuid.New().String()[:8]
	reminder := &Reminder{
		ID:          id,
		Account:     account,
		Title:       title,
		Message:     message,
		Cron:        cronExpr,
		Enabled:     true,
		RepeatCount: repeatCount,
		RunCount:    0,
		CreatedAt:   time.Now(),
	}

	s.registerCronJob(reminder)

	s.mu.Lock()
	s.reminders[id] = reminder
	s.mu.Unlock()

	go s.SaveReminders(account)

	log.MessageF(log.ModuleAgent, "Cron reminder added: %s [%s]", id, cronExpr)
	return reminder
}

// AddAIScheduledTask 添加 AI 定时任务
func (s *Scheduler) AddAIScheduledTask(account, title, cronExpr, aiQuery string, saveResult bool) *Reminder {
	id := uuid.New().String()[:8]
	reminder := &Reminder{
		ID:          id,
		Account:     account,
		Title:       title,
		Message:     "AI 定时任务",
		Cron:        cronExpr,
		AIQuery:     aiQuery,
		SaveResult:  saveResult,
		Enabled:     true,
		RepeatCount: -1,
		RunCount:    0,
		SmartMode:   true,
		CreatedAt:   time.Now(),
	}

	s.registerCronJob(reminder)

	s.mu.Lock()
	s.reminders[id] = reminder
	s.mu.Unlock()

	go s.SaveReminders(account)

	log.MessageF(log.ModuleAgent, "AI scheduled task added: %s [%s] query: %s", id, cronExpr, aiQuery)
	return reminder
}

// RemoveReminder 删除提醒
func (s *Scheduler) RemoveReminder(id string) bool {
	s.mu.Lock()
	r, exists := s.reminders[id]
	if exists {
		// 从 cron 引擎移除
		s.cronEngine.Remove(r.cronEntryID)
		account := r.Account
		delete(s.reminders, id)
		s.mu.Unlock()
		go s.SaveReminders(account)
		log.MessageF(log.ModuleAgent, "Reminder removed: %s", id)
		return true
	}
	s.mu.Unlock()
	return false
}

// triggerReminder 触发提醒
func (s *Scheduler) triggerReminder(reminderID string) {
	s.mu.RLock()
	r, exists := s.reminders[reminderID]
	if !exists || !r.Enabled {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	log.MessageF(log.ModuleAgent, "Triggering reminder: %s - %s", r.ID, r.Title)

	// 决定消息内容
	var finalMessage string

	if r.AIQuery != "" {
		// AI 定时任务：执行 AI 查询
		finalMessage = s.executeAIQuery(r)
	} else if r.SmartMode {
		// 智能模式：AI 生成消息
		if aiMsg := s.generateSmartMessage(r); aiMsg != "" {
			finalMessage = aiMsg
		} else {
			finalMessage = r.Message
		}
	} else {
		finalMessage = r.Message
	}

	// Browser WebSocket 推送
	if s.hub != nil {
		notification := TaskNotification{
			Type:     "smart_reminder",
			TaskID:   r.ID,
			Progress: 100,
			Message:  fmt.Sprintf("[%s] %s", r.Title, finalMessage),
			Data: map[string]interface{}{
				"title":      r.Title,
				"message":    finalMessage,
				"smart_mode": r.SmartMode,
				"ai_query":   r.AIQuery != "",
				"run_count":  r.RunCount + 1,
			},
		}
		s.hub.BroadcastToAccount(r.Account, notification)
	}

	// 始终在服务器日志中输出提醒内容（防止 WS 断连时完全无反馈）
	msgPreview := finalMessage
	if len(msgPreview) > 200 {
		msgPreview = msgPreview[:200] + "..."
	}
	log.MessageF(log.ModuleAgent, "⏰ Reminder [%s]: %s", r.Title, msgPreview)

	// Email 推送
	if email.IsEnabled() {
		go s.sendReminderEmail(r, finalMessage)
	}

	// 企业微信推送（通过 gateway → wechat-agent）
	if codegen.IsGatewayConnected() {
		go func() {
			wechatMsg := fmt.Sprintf("⏰ %s\n%s", r.Title, finalMessage)
			if err := codegen.SendWechatNotify("@all", wechatMsg); err != nil {
				log.WarnF(log.ModuleAgent, "WeChat push via gateway failed: %v", err)
			}
		}()
	}

	// 更新状态
	s.mu.Lock()
	r.LastRunTime = time.Now()
	r.RunCount++
	if r.RepeatCount > 0 {
		r.RepeatCount--
		if r.RepeatCount == 0 {
			r.Enabled = false
			s.cronEngine.Remove(r.cronEntryID)
			log.MessageF(log.ModuleAgent, "Reminder completed: %s", r.ID)
		}
	}
	s.mu.Unlock()

	// 持久化状态
	go s.SaveReminders(r.Account)
}

// executeAIQuery 执行 AI 定时查询
func (s *Scheduler) executeAIQuery(r *Reminder) string {
	log.MessageF(log.ModuleAgent, "Executing AI scheduled query: %s -> %s", r.ID, r.AIQuery)

	messages := []llm.Message{
		{Role: "user", Content: r.AIQuery},
	}

	resp, err := llm.SendSyncLLMRequest(messages, r.Account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "AI scheduled query failed for %s: %v", r.ID, err)
		return fmt.Sprintf("AI 查询执行失败: %v", err)
	}

	// 保存结果到博客
	if r.SaveResult && resp != "" {
		dateStr := time.Now().Format("2006-01-02_15-04")
		title := fmt.Sprintf("ai_task_%s_%s", strings.ReplaceAll(r.Title, " ", "_"), dateStr)
		statistics.RawCreateBlog(r.Account, title, resp, "AI定时任务|自动生成", 2, 0)
		log.MessageF(log.ModuleAgent, "AI task result saved: %s", title)
	}

	return resp
}

// generateSmartMessage 使用 AI 生成智能提醒消息
func (s *Scheduler) generateSmartMessage(r *Reminder) string {
	today := time.Now().Format("2006-01-02")
	todoData := statistics.RawGetTodosByDate(r.Account, today)
	exerciseData := statistics.RawGetExerciseStats(r.Account, 7)

	promptContent := config.SafeSprintf(config.GetPrompt(r.Account, "smart_reminder"), r.Title, r.Message, time.Now().Format("15:04"), todoData, exerciseData)

	messages := []llm.Message{
		{Role: "user", Content: promptContent},
	}

	resp, err := llm.SendSyncLLMRequest(messages, r.Account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Smart message generation failed: %v", err)
		return ""
	}
	return resp
}

// sendReminderEmail 发送提醒邮件
func (s *Scheduler) sendReminderEmail(r *Reminder, message string) {
	subject := fmt.Sprintf("📌 提醒: %s", r.Title)
	htmlBody := fmt.Sprintf(`
<div style="font-family: 'Segoe UI', Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 20px; border-radius: 10px; color: white;">
    <h2 style="margin: 0;">📌 %s</h2>
    <p style="margin: 5px 0 0; opacity: 0.8;">%s</p>
  </div>
  <div style="padding: 20px; background: #f8f9fa; border-radius: 0 0 10px 10px;">
    <p style="font-size: 16px; line-height: 1.6; color: #333;">%s</p>
    <hr style="border: none; border-top: 1px solid #dee2e6; margin: 15px 0;">
    <p style="font-size: 12px; color: #999;">此邮件由 GoBlog 智能提醒系统自动发送</p>
  </div>
</div>`, r.Title, time.Now().Format("2006-01-02 15:04"), message)

	if err := email.SendHTMLEmail("", subject, htmlBody); err != nil {
		log.WarnF(log.ModuleAgent, "Failed to send reminder email for %s: %v", r.ID, err)
	}
}

// GetReminders 获取账户的所有提醒
func (s *Scheduler) GetReminders(account string) []*Reminder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Reminder
	for _, r := range s.reminders {
		if r.Account == account {
			result = append(result, r)
		}
	}
	return result
}

// GetReminderByID 根据ID获取提醒
func (s *Scheduler) GetReminderByID(id string) *Reminder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reminders[id]
}

// PauseReminder 暂停提醒
func (s *Scheduler) PauseReminder(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, exists := s.reminders[id]; exists {
		r.Enabled = false
		s.cronEngine.Remove(r.cronEntryID)
		go s.SaveReminders(r.Account)
		return true
	}
	return false
}

// ResumeReminder 恢复提醒
func (s *Scheduler) ResumeReminder(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, exists := s.reminders[id]; exists {
		r.Enabled = true
		s.registerCronJob(r)
		go s.SaveReminders(r.Account)
		return true
	}
	return false
}

// === 全局函数 ===

// InitScheduler 初始化全局调度器
func InitScheduler(hub *NotificationHub) {
	globalScheduler = NewScheduler(hub)
	globalScheduler.Start()
}

// ShutdownScheduler 关闭调度器
func ShutdownScheduler() {
	if globalScheduler != nil {
		globalScheduler.Stop()
	}
}

// LoadScheduledTasks 加载持久化的定时任务（在 Init 之后调用）
func LoadScheduledTasks(account string) {
	if globalScheduler != nil {
		globalScheduler.LoadReminders(account)
	}
}

// CreateReminder 创建提醒 (MCP 工具接口)
func CreateReminder(account string, args map[string]interface{}) map[string]interface{} {
	return CreateReminderWithTask(account, args, "")
}

// CreateReminderWithTask 创建提醒并关联任务
func CreateReminderWithTask(account string, args map[string]interface{}, linkedTaskID string) map[string]interface{} {
	if globalScheduler == nil {
		return map[string]interface{}{"success": false, "error": "Scheduler not initialized"}
	}

	title, _ := args["title"].(string)
	message, _ := args["message"].(string)
	cronExpr, _ := args["cron"].(string)
	aiQuery, _ := args["ai_query"].(string)
	intervalFloat, _ := args["interval"].(float64)
	repeatFloat, _ := args["repeat"].(float64)
	saveResultBool, _ := args["save_result"].(bool)

	if title == "" {
		title = "提醒"
	}
	if message == "" {
		message = "这是您的定时提醒"
	}

	var reminder *Reminder

	if aiQuery != "" {
		// AI 定时任务
		if cronExpr == "" {
			cronExpr = "@every 1h" // 默认每小时
		}
		reminder = globalScheduler.AddAIScheduledTask(account, title, cronExpr, aiQuery, saveResultBool)
	} else if cronExpr != "" {
		// Cron 表达式模式
		repeat := int(repeatFloat)
		if repeat == 0 {
			repeat = -1
		}
		reminder = globalScheduler.AddCronReminder(account, title, message, cronExpr, repeat)
	} else {
		// 间隔模式（兼容旧接口）
		interval := int(intervalFloat)
		if interval <= 0 {
			interval = 60
		}
		repeat := int(repeatFloat)
		if repeat == 0 {
			repeat = -1
		}
		reminder = globalScheduler.AddReminderWithTask(account, title, message, interval, repeat, linkedTaskID)
	}

	return map[string]interface{}{
		"success":  true,
		"id":       reminder.ID,
		"title":    reminder.Title,
		"cron":     reminder.Cron,
		"interval": reminder.Interval,
		"ai_query": reminder.AIQuery,
	}
}

// ListReminders 列出提醒 (MCP 工具接口)
func ListReminders(account string, args map[string]interface{}) map[string]interface{} {
	if globalScheduler == nil {
		return map[string]interface{}{"success": false, "error": "Scheduler not initialized"}
	}

	reminders := globalScheduler.GetReminders(account)
	data, _ := json.Marshal(reminders)

	return map[string]interface{}{
		"success":   true,
		"count":     len(reminders),
		"reminders": string(data),
	}
}

// DeleteReminder 删除提醒 (MCP 工具接口)
func DeleteReminder(account string, args map[string]interface{}) map[string]interface{} {
	if globalScheduler == nil {
		return map[string]interface{}{"success": false, "error": "Scheduler not initialized"}
	}

	id, _ := args["id"].(string)
	if id == "" {
		return map[string]interface{}{"success": false, "error": "Missing reminder id"}
	}

	success := globalScheduler.RemoveReminder(id)
	return map[string]interface{}{
		"success": success,
		"id":      id,
	}
}

// SendNotification 发送即时通知 (MCP 工具接口)
func SendNotification(account string, args map[string]interface{}) map[string]interface{} {
	if globalHub == nil {
		return map[string]interface{}{"success": false, "error": "Notification hub not initialized"}
	}

	message, _ := args["message"].(string)
	if message == "" {
		message = "通知"
	}

	notification := TaskNotification{
		Type:     "notification",
		TaskID:   fmt.Sprintf("notify_%d", time.Now().UnixNano()),
		Progress: 100,
		Message:  message,
	}

	globalHub.Broadcast(notification)

	return map[string]interface{}{
		"success": true,
		"message": message,
	}
}

// RegisterSchedulerMCPTools 注册调度器相关的 MCP 工具（从 llm.Init 调用避免循环依赖）
func RegisterSchedulerMCPTools() {
	// CreateAIScheduledTask - AI 定时任务
	mcp.RegisterCallBack("CreateAIScheduledTask", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			return `{"error": "缺少 account"}`
		}
		title, _ := args["title"].(string)
		cronExpr, _ := args["cron"].(string)
		aiQuery, _ := args["ai_query"].(string)
		saveResult, _ := args["save_result"].(bool)

		if title == "" || aiQuery == "" {
			return `{"error": "缺少 title 或 ai_query"}`
		}
		if cronExpr == "" {
			cronExpr = "0 0 21 * * *" // 默认每天 21:00
		}

		if globalScheduler == nil {
			return `{"error": "Scheduler not initialized"}`
		}

		reminder := globalScheduler.AddAIScheduledTask(account, title, cronExpr, aiQuery, saveResult)
		data, _ := json.Marshal(reminder)
		return string(data)
	})
	mcp.RegisterCallBackPrompt("CreateAIScheduledTask", "AI 定时任务已创建，将按 cron 表达式定时执行 AI 查询")

	log.Message(log.ModuleAgent, "Scheduler MCP tools registered: CreateAIScheduledTask")
}
