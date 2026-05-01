package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"uap"
)

// MonitorResult 单次监控检查的结果
type MonitorResult struct {
	Account       string         `json:"account"`
	CheckedAt     time.Time      `json:"checked_at"`
	DataSources   map[string]any `json:"data_sources"`
	NewBlogs      int            `json:"new_blogs"`
	BlogCount     int            `json:"blog_count"`
	TodoCount     int            `json:"todo_count"`
	OverdueTodo   int            `json:"overdue_todo"`
	ExerciseMin   int            `json:"exercise_min"`
	ExerciseCnt   int            `json:"exercise_cnt"`
	OnlineAgents  []string       `json:"online_agents"`
	OfflineAgents []string       `json:"offline_agents"`
}

// MonitorEngine 定期检查各 agent 数据并评估播报条件
type MonitorEngine struct {
	cfg      *Config
	client   *uap.Client
	decider  *BroadcastDecider
	registry *AccountRegistry

	// OnGenerateTTS 可选：TTS 语音生成回调（由 Connection 注入）
	OnGenerateTTS       func(text string) (audioBase64 string, format string)
	OnPerformCheck      func(account string) *MonitorResult
	OnExecuteBroadcast  func(decision BroadcastDecision)
	OnEvaluateProactive func(account string, result *MonitorResult) (BroadcastDecision, bool)

	mu             sync.Mutex
	running        bool
	stopCh         chan struct{}
	lastCheck      time.Time
	lastBlogCount  int
	lastBlogTitles map[string]bool // 上次检查时的博客标题集合，用于检测新增
	checkCount     int64
}

// NewMonitorEngine 创建监控引擎
func NewMonitorEngine(cfg *Config, client *uap.Client, decider *BroadcastDecider, registry *AccountRegistry) *MonitorEngine {
	return &MonitorEngine{
		cfg:            cfg,
		client:         client,
		decider:        decider,
		registry:       registry,
		stopCh:         make(chan struct{}),
		lastBlogTitles: make(map[string]bool),
	}
}

// Start 启动监控循环
func (e *MonitorEngine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	interval := time.Duration(e.cfg.CheckIntervalSec) * time.Second
	log.Printf("[Monitor] 监控引擎启动 interval=%s broadcasts=%d", interval, len(e.cfg.Broadcasts))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-e.stopCh:
				log.Printf("[Monitor] 监控引擎已停止")
				return
			case <-ticker.C:
				e.runCheckCycle()
			}
		}
	}()
}

// Stop 停止监控循环
func (e *MonitorEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stopCh)
}

// CheckNow 立即执行一次完整检查
func (e *MonitorEngine) CheckNow(account string) *MonitorResult {
	result := e.performCheck(account)
	return result
}

// State 返回监控引擎当前状态
func (e *MonitorEngine) State() map[string]any {
	e.mu.Lock()
	lastCheck := e.lastCheck
	checkCount := e.checkCount
	blogCount := e.lastBlogCount
	e.mu.Unlock()

	return map[string]any{
		"running":             e.IsRunning(),
		"last_check":          lastCheck.Format(time.RFC3339),
		"check_count":         checkCount,
		"blog_count":          blogCount,
		"check_interval":      e.cfg.CheckIntervalSec,
		"registered_accounts": e.ListAccounts(),
	}
}

// IsRunning 是否正在运行
func (e *MonitorEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// runCheckCycle 执行一次检查周期
func (e *MonitorEngine) runCheckCycle() {
	if e.isInQuietHours() {
		log.Printf("[Monitor] 免打扰时段，跳过检查")
		return
	}

	log.Printf("[Monitor] ── 开始检查周期 ──")
	accounts := e.ListAccounts()
	log.Printf("[Monitor] 当前注册账号 accounts=%v", accounts)

	if len(accounts) == 0 {
		result := e.performCheck("")
		e.updateState(result)
		log.Printf("[Monitor] skip broadcast: no registered accounts")
		log.Printf("[Monitor] ── 检查周期完成 blogs=%d todos=%d new_blogs=%d ──",
			result.BlogCount, result.TodoCount, result.NewBlogs)
		return
	}

	for _, account := range accounts {
		log.Printf("[Monitor] 检查账号 account=%s", account)
		result := e.performCheck(account)
		e.updateState(result)

		if e.OnEvaluateProactive != nil {
			if decision, handled := e.OnEvaluateProactive(account, result); handled {
				if decision.ShouldBroadcast {
					log.Printf("[Monitor] 主动编排触发 account=%s broadcast_id=%s text=%q",
						account, decision.BroadcastID, decision.Text)
					e.executeBroadcast(decision)
				} else if decision.SkipReason != "" {
					log.Printf("[Monitor] 主动编排跳过 account=%s reason=%s", account, decision.SkipReason)
				}
				continue
			}
		}

		decision := e.decider.Evaluate(result)
		if decision.ShouldBroadcast {
			log.Printf("[Monitor] 触发播报 account=%s broadcast_id=%s text=%q expression=%s motion=%s",
				account, decision.BroadcastID, decision.Text, decision.Expression, decision.Motion)
			e.executeBroadcast(decision)
		} else if decision.SkipReason != "" {
			log.Printf("[Monitor] 跳过播报 account=%s reason=%s", account, decision.SkipReason)
		}
	}
	log.Printf("[Monitor] ── 检查周期完成 accounts=%d ──", len(accounts))
}

// performCheck 执行实际的数据查询
func (e *MonitorEngine) performCheck(account string) *MonitorResult {
	if e.OnPerformCheck != nil {
		return e.OnPerformCheck(account)
	}

	result := &MonitorResult{
		Account:     account,
		CheckedAt:   time.Now(),
		DataSources: make(map[string]any),
	}

	// 查询博客数据
	e.checkBlogData(result, account)
	// 查询待办数据
	e.checkTodoData(result, account)
	// 查询运动数据
	e.checkExerciseData(result, account)
	// 查询 agent 在线状态
	e.checkAgentStatus(result)

	return result
}

func (e *MonitorEngine) updateState(result *MonitorResult) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.lastCheck = result.CheckedAt
	e.checkCount++
	e.lastBlogCount = result.BlogCount
}

func (e *MonitorEngine) RegisterAccount(account string) bool {
	if e.registry == nil {
		return false
	}
	return e.registry.RegisterAccount(account)
}

func (e *MonitorEngine) UnregisterAccount(account string) bool {
	if e.registry == nil {
		return false
	}
	return e.registry.UnregisterAccount(account)
}

func (e *MonitorEngine) ListAccounts() []string {
	if e.registry == nil {
		return nil
	}
	return e.registry.ListAccounts()
}

func (e *MonitorEngine) HasAccount(account string) bool {
	if e.registry == nil {
		return false
	}
	return e.registry.HasAccount(account)
}

func (e *MonitorEngine) GetSession(account string) *CortanaUserSession {
	if e.registry == nil {
		return nil
	}
	return e.registry.GetSession(account)
}

func (e *MonitorEngine) SyncUserSession(payload CortanaSyncUserPayload) *CortanaUserSession {
	if e.registry == nil {
		return nil
	}
	return e.registry.SyncUserSession(payload)
}

func (e *MonitorEngine) ExecuteBroadcast(decision BroadcastDecision) {
	e.executeBroadcast(decision)
}

// checkBlogData 查询博客相关数据
func (e *MonitorEngine) checkBlogData(result *MonitorResult, account string) {
	// 使用 describe 协议查询 blog-agent 状态，或发送 task_assign 给 llm-agent
	// 这里采用直接查询方式：向 blog-agent 发送工具调用
	taskPayload := map[string]interface{}{
		"task_type": "cortana_monitor",
		"payload": map[string]interface{}{
			"action":  "check_blog_stats",
			"account": account,
		},
	}

	payloadJSON, _ := json.Marshal(taskPayload)

	err := e.client.SendTo(e.cfg.BlogAgentID, uap.MsgTaskAssign, uap.TaskAssignPayload{
		TaskID:  fmt.Sprintf("cortana_monitor_blog_%d", time.Now().UnixMilli()),
		Payload: payloadJSON,
	})
	if err != nil {
		log.Printf("[Monitor] 查询博客数据失败: %v", err)
		result.DataSources["blog_error"] = err.Error()
		return
	}

	// 注意：这里是异步的，实际结果会在 task_complete 中返回
	// 当前版本使用本地数据模拟检测新博客
	result.DataSources["blog_queried"] = true
}

// checkTodoData 查询待办相关数据
func (e *MonitorEngine) checkTodoData(result *MonitorResult, account string) {
	taskPayload := map[string]interface{}{
		"task_type": "cortana_monitor",
		"payload": map[string]interface{}{
			"action":  "check_todo_stats",
			"account": account,
		},
	}

	payloadJSON, _ := json.Marshal(taskPayload)

	err := e.client.SendTo(e.cfg.CronAgentID, uap.MsgTaskAssign, uap.TaskAssignPayload{
		TaskID:  fmt.Sprintf("cortana_monitor_todo_%d", time.Now().UnixMilli()),
		Payload: payloadJSON,
	})
	if err != nil {
		log.Printf("[Monitor] 查询待办数据失败: %v", err)
		result.DataSources["todo_error"] = err.Error()
		return
	}

	result.DataSources["todo_queried"] = true
}

// checkExerciseData 查询运动相关数据
func (e *MonitorEngine) checkExerciseData(result *MonitorResult, account string) {
	result.DataSources["exercise_queried"] = true
}

// checkAgentStatus 查询 agent 在线状态
// 通过向 gateway 发送 describe 请求来获取 agent 列表
func (e *MonitorEngine) checkAgentStatus(result *MonitorResult) {
	// 向 gateway 发送 describe 请求
	err := e.client.SendTo("gateway", uap.MsgDescribe, struct{}{})
	if err != nil {
		log.Printf("[Monitor] 查询 agent 状态失败: %v", err)
		result.DataSources["agent_status_error"] = err.Error()
		return
	}

	result.DataSources["agent_status_queried"] = true
}

// HandleTaskComplete 处理来自 llm-agent 的任务完成
func (e *MonitorEngine) HandleTaskComplete(msg *uap.Message) {
	var payload uap.TaskCompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Monitor] 解析 task_complete 失败: %v", err)
		return
	}

	log.Printf("[Monitor] ← task_complete taskID=%s status=%s resultLen=%d",
		payload.TaskID, payload.Status, len(payload.Result))

	// 解析结果并更新监控数据
	var monitorResult struct {
		BlogCount     int      `json:"blog_count"`
		NewBlogs      int      `json:"new_blogs"`
		TodoCount     int      `json:"todo_count"`
		OverdueTodo   int      `json:"overdue_todo"`
		ExerciseMin   int      `json:"exercise_min"`
		ExerciseCnt   int      `json:"exercise_cnt"`
		OnlineAgents  []string `json:"online_agents"`
		OfflineAgents []string `json:"offline_agents"`
	}

	if payload.Result != "" {
		if err := json.Unmarshal([]byte(payload.Result), &monitorResult); err != nil {
			log.Printf("[Monitor] 解析结果失败: %v", err)
			return
		}
	}

	// 更新本地状态
	e.mu.Lock()
	if monitorResult.BlogCount > 0 {
		e.lastBlogCount = monitorResult.BlogCount
	}
	e.mu.Unlock()

	log.Printf("[Monitor] 监控数据更新 blogs=%d new=%d todos=%d overdue=%d exercise=%dm/%d",
		monitorResult.BlogCount, monitorResult.NewBlogs,
		monitorResult.TodoCount, monitorResult.OverdueTodo,
		monitorResult.ExerciseMin, monitorResult.ExerciseCnt)
}

// DetectNewBlogs 检测新增博客（基于标题集合对比）
func (e *MonitorEngine) DetectNewBlogs(currentTitles []string) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	newCount := 0
	currentSet := make(map[string]bool, len(currentTitles))
	for _, title := range currentTitles {
		currentSet[title] = true
		if !e.lastBlogTitles[title] {
			newCount++
		}
	}

	if newCount > 0 {
		e.lastBlogTitles = currentSet
	}

	return newCount
}

// executeBroadcast 执行播报（含 TTS 语音）
func (e *MonitorEngine) executeBroadcast(decision BroadcastDecision) {
	if e.OnExecuteBroadcast != nil {
		e.OnExecuteBroadcast(decision)
		return
	}

	// 记录播报状态
	e.decider.RecordBroadcast(decision.Account, decision.BroadcastID)

	// 生成 TTS 语音
	var audioBase64, audioFormat string
	if e.OnGenerateTTS != nil {
		audioBase64, audioFormat = e.OnGenerateTTS(decision.Text)
	}

	// 发送播报到 app-agent
	broadcastPayload := map[string]interface{}{
		"kind":         "cortana_broadcast",
		"text":         decision.Text,
		"expression":   decision.Expression,
		"motion":       decision.Motion,
		"origin":       "cortana-agent",
		"broadcast_id": decision.BroadcastID,
		"timestamp":    time.Now().UnixMilli(),
		"audio_base64": audioBase64,
		"audio_format": audioFormat,
	}

	payloadJSON, _ := json.Marshal(broadcastPayload)

	notify := uap.NotifyPayload{
		Channel: "app",
		To:      decision.Account,
		Content: string(payloadJSON),
	}

	if err := e.client.SendTo(e.cfg.AppAgentID, uap.MsgNotify, notify); err != nil {
		log.Printf("[Monitor] 发送播报失败: %v", err)
		return
	}

	log.Printf("[Monitor] ✓ 播报已发送 account=%s notify_to=%s broadcast_id=%s text=%q audio=%dbytes",
		decision.Account, notify.To, decision.BroadcastID, decision.Text, len(audioBase64))
}

// isInQuietHours 判断当前时间是否在免打扰时段
func (e *MonitorEngine) isInQuietHours() bool {
	if e.cfg.QuietHoursStart == "" || e.cfg.QuietHoursEnd == "" {
		return false
	}

	start, ok1 := parseTimeMinutes(e.cfg.QuietHoursStart)
	end, ok2 := parseTimeMinutes(e.cfg.QuietHoursEnd)
	if !ok1 || !ok2 {
		return false
	}

	now := time.Now()
	cur := now.Hour()*60 + now.Minute()

	if start <= end {
		return cur >= start && cur < end
	}
	// 跨午夜
	return cur >= start || cur < end
}

// parseTimeMinutes 解析 "HH:MM" 为分钟数
func parseTimeMinutes(s string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
