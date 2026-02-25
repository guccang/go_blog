package agent

import (
	"email"
	"encoding/json"
	"fmt"
	"llm"
	log "mylog"
	"statistics"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Reminder 提醒任务
type Reminder struct {
	ID           string    `json:"id"`
	Account      string    `json:"account"`
	Title        string    `json:"title"`
	Message      string    `json:"message"`
	Cron         string    `json:"cron"`     // Cron 表达式: "*/1 * * * *" = 每分钟
	Interval     int       `json:"interval"` // 间隔秒数 (简单模式)
	NextRunTime  time.Time `json:"next_run_at"`
	LastRunTime  time.Time `json:"last_run_at,omitempty"`
	Enabled      bool      `json:"enabled"`
	RepeatCount  int       `json:"repeat_count"`   // -1 = 无限, 0 = 已完成, >0 = 剩余次数
	RunCount     int       `json:"run_count"`      // 已执行次数
	LinkedTaskID string    `json:"linked_task_id"` // 关联的任务ID
	SmartMode    bool      `json:"smart_mode"`     // 是否启用 AI 智能消息
	CreatedAt    time.Time `json:"created_at"`
}

// Scheduler 定时调度器
type Scheduler struct {
	reminders map[string]*Reminder
	mu        sync.RWMutex
	stopCh    chan struct{}
	hub       *NotificationHub
	running   bool
}

var globalScheduler *Scheduler

// NewScheduler 创建调度器
func NewScheduler(hub *NotificationHub) *Scheduler {
	return &Scheduler{
		reminders: make(map[string]*Reminder),
		stopCh:    make(chan struct{}),
		hub:       hub,
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.running = true
	log.Message(log.ModuleAgent, "Scheduler started")
	go s.run()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.running {
		close(s.stopCh)
		s.running = false
		log.Message(log.ModuleAgent, "Scheduler stopped")
	}
}

// run 调度器主循环
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkReminders(now)
		}
	}
}

// checkReminders 检查并触发到期的提醒
func (s *Scheduler) checkReminders(now time.Time) {
	s.mu.RLock()
	remindersToTrigger := make([]*Reminder, 0)
	for _, r := range s.reminders {
		if r.Enabled && now.After(r.NextRunTime) {
			remindersToTrigger = append(remindersToTrigger, r)
		}
	}
	s.mu.RUnlock()

	for _, r := range remindersToTrigger {
		s.triggerReminder(r, now)
	}
}

// triggerReminder 触发提醒（智能版）
func (s *Scheduler) triggerReminder(r *Reminder, now time.Time) {
	log.MessageF(log.ModuleAgent, "Triggering reminder: %s - %s", r.ID, r.Title)

	// 决定消息内容：智能模式用 AI 生成，否则用原始消息
	finalMessage := r.Message
	if r.SmartMode {
		if aiMsg := s.generateSmartMessage(r); aiMsg != "" {
			finalMessage = aiMsg
		}
	}

	// 1. Browser WebSocket 推送（如果在线）
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
				"run_count":  r.RunCount + 1,
			},
		}
		s.hub.BroadcastToAccount(r.Account, notification)
	}

	// 2. Email 推送（异步，不阻塞调度器）
	if email.IsEnabled() {
		go s.sendReminderEmail(r, finalMessage)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	r.LastRunTime = now
	r.RunCount++

	// 计算下次运行时间
	if r.RepeatCount == -1 {
		// 无限重复
		r.NextRunTime = now.Add(time.Duration(r.Interval) * time.Second)
	} else if r.RepeatCount > 0 {
		r.RepeatCount--
		r.NextRunTime = now.Add(time.Duration(r.Interval) * time.Second)
	} else {
		// 已完成所有重复
		r.Enabled = false
		log.MessageF(log.ModuleAgent, "Reminder completed: %s", r.ID)
	}
}

// generateSmartMessage 使用 AI 生成智能提醒消息
func (s *Scheduler) generateSmartMessage(r *Reminder) string {
	// 收集用户上下文
	today := time.Now().Format("2006-01-02")
	todoData := statistics.RawGetTodosByDate(r.Account, today)
	exerciseData := statistics.RawGetExerciseStats(r.Account, 7)

	promptContent := fmt.Sprintf(`你是一个智能提醒助手。请根据以下信息生成一条简洁、有温度的提醒消息。

提醒标题: %s
原始消息: %s
当前时间: %s

用户今日待办: %s
用户近7天运动: %s

要求:
1. 消息简洁，不超过200字
2. 结合用户的待办和运动数据给出个性化提醒
3. 语气温暖友好，带有鼓励
4. 直接输出消息内容，不要加任何前缀`, r.Title, r.Message, time.Now().Format("15:04"), todoData, exerciseData)

	messages := []llm.Message{
		{Role: "user", Content: promptContent},
	}

	resp, err := llm.SendSyncLLMRequest(messages, r.Account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "Smart message generation failed: %v", err)
		return ""
	}

	if resp != "" {
		return resp
	}
	return ""
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

// AddReminder 添加提醒
func (s *Scheduler) AddReminder(account, title, message string, intervalSeconds int, repeatCount int) *Reminder {
	return s.AddReminderWithTask(account, title, message, intervalSeconds, repeatCount, "")
}

// AddReminderWithTask 添加提醒并关联任务
func (s *Scheduler) AddReminderWithTask(account, title, message string, intervalSeconds int, repeatCount int, linkedTaskID string) *Reminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()[:8]
	reminder := &Reminder{
		ID:           id,
		Account:      account,
		Title:        title,
		Message:      message,
		Interval:     intervalSeconds,
		NextRunTime:  time.Now().Add(time.Duration(intervalSeconds) * time.Second),
		Enabled:      true,
		RepeatCount:  repeatCount, // -1 = 无限
		RunCount:     0,
		LinkedTaskID: linkedTaskID,
		CreatedAt:    time.Now(),
	}

	s.reminders[id] = reminder
	log.MessageF(log.ModuleAgent, "Reminder added: %s every %d seconds, linked to task: %s", id, intervalSeconds, linkedTaskID)
	return reminder
}

// RemoveReminder 删除提醒
func (s *Scheduler) RemoveReminder(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.reminders[id]; exists {
		delete(s.reminders, id)
		log.MessageF(log.ModuleAgent, "Reminder removed: %s", id)
		return true
	}
	return false
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
		r.NextRunTime = time.Now().Add(time.Duration(r.Interval) * time.Second)
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
	intervalFloat, _ := args["interval"].(float64)
	repeatFloat, _ := args["repeat"].(float64)

	if title == "" {
		title = "提醒"
	}
	if message == "" {
		message = "这是您的定时提醒"
	}
	interval := int(intervalFloat)
	if interval <= 0 {
		interval = 60 // 默认1分钟
	}
	repeat := int(repeatFloat)
	if repeat == 0 {
		repeat = -1 // 无限重复
	}

	reminder := globalScheduler.AddReminderWithTask(account, title, message, interval, repeat, linkedTaskID)

	return map[string]interface{}{
		"success":  true,
		"id":       reminder.ID,
		"title":    reminder.Title,
		"message":  reminder.Message,
		"interval": reminder.Interval,
		"repeat":   reminder.RepeatCount,
		"next_run": reminder.NextRunTime.Format(time.RFC3339),
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
