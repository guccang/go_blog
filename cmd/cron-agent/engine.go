package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"agentbase"
	"uap"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// CronTask 定时任务
type CronTask struct {
	ID         string `json:"id"`          // 短 UUID
	Name       string `json:"name"`        // 任务名称
	TaskType   string `json:"task_type"`   // "cron_reminder" | "cron_query"
	Schedule   string `json:"schedule"`    // cron 表达式或 "@every 20m"，空=延迟任务
	DelaySec   int    `json:"delay_sec"`   // 延迟秒数（Schedule 为空时生效）
	Account    string `json:"account"`     // 用户账号
	WechatUser string `json:"wechat_user"` // 微信用户标识
	Message    string `json:"message"`     // cron_reminder 提醒内容
	Query      string `json:"query"`       // cron_query 查询问题
	OneShot    bool   `json:"one_shot"`    // 执行后自动删除
	CreatedAt  string `json:"created_at"`  // RFC3339 创建时间
}

// CronEngine 定时任务引擎：调度 + 存储 + 执行
type CronEngine struct {
	mu      sync.RWMutex
	tasks   map[string]*CronTask            // taskID → task
	entries map[string]cron.EntryID          // taskID → cron entryID
	timers  map[string]context.CancelFunc    // taskID → 延迟任务取消函数
	cron    *cron.Cron
	ab      *agentbase.AgentBase
	cfg     *Config
	pending sync.Map // executionID → cronTaskID
}

// NewCronEngine 创建引擎，加载持久化任务，启动调度器
func NewCronEngine(cfg *Config, ab *agentbase.AgentBase) *CronEngine {
	log.Printf("[CronEngine] 初始化引擎 task_file=%s llm_agent=%s", cfg.TaskFile, cfg.LLMAgentID)

	e := &CronEngine{
		tasks:   make(map[string]*CronTask),
		entries: make(map[string]cron.EntryID),
		timers:  make(map[string]context.CancelFunc),
		cron:    cron.New(),
		ab:      ab,
		cfg:     cfg,
	}

	// 加载持久化任务
	if err := e.loadFromFile(); err != nil {
		log.Printf("[CronEngine] 加载任务文件失败: %v", err)
	} else {
		log.Printf("[CronEngine] 从 %s 加载了 %d 个任务", cfg.TaskFile, len(e.tasks))
		for _, task := range e.tasks {
			log.Printf("[CronEngine]   ├─ ID=%s name=%s type=%s schedule=%s",
				task.ID, task.Name, task.TaskType, task.Schedule)
		}
	}

	// 将已加载的任务注册到调度器
	for _, task := range e.tasks {
		if err := e.scheduleTask(task); err != nil {
			log.Printf("[CronEngine] 调度任务失败 ID=%s name=%s: %v", task.ID, task.Name, err)
		} else {
			log.Printf("[CronEngine] 调度任务成功 ID=%s name=%s", task.ID, task.Name)
		}
	}

	e.cron.Start()
	log.Printf("[CronEngine] ✓ 引擎启动完成，%d 个任务已调度", len(e.tasks))
	return e
}

// AddTask 添加任务到引擎
func (e *CronEngine) AddTask(task *CronTask) error {
	log.Printf("[CronEngine] AddTask 开始 ID=%s name=%s type=%s schedule=%q delay=%d account=%s wechat=%s",
		task.ID, task.Name, task.TaskType, task.Schedule, task.DelaySec, task.Account, task.WechatUser)

	e.mu.Lock()
	defer e.mu.Unlock()

	// 调度
	if err := e.scheduleTask(task); err != nil {
		log.Printf("[CronEngine] AddTask 调度失败 ID=%s: %v", task.ID, err)
		return err
	}

	e.tasks[task.ID] = task
	e.saveToFile()

	log.Printf("[CronEngine] ✓ AddTask 成功 ID=%s name=%s, 当前任务总数=%d",
		task.ID, task.Name, len(e.tasks))
	return nil
}

// RemoveTask 移除任务
func (e *CronEngine) RemoveTask(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[id]
	if !exists {
		log.Printf("[CronEngine] RemoveTask 失败，任务不存在: %s", id)
		return fmt.Errorf("任务不存在: %s", id)
	}

	log.Printf("[CronEngine] RemoveTask 开始 ID=%s name=%s", id, task.Name)

	// 取消 cron 调度
	if entryID, ok := e.entries[id]; ok {
		e.cron.Remove(entryID)
		delete(e.entries, id)
		log.Printf("[CronEngine]   ├─ cron entry 已移除 entryID=%d", entryID)
	}

	// 取消延迟 timer
	if cancel, ok := e.timers[id]; ok {
		cancel()
		delete(e.timers, id)
		log.Printf("[CronEngine]   ├─ 延迟 timer 已取消")
	}

	delete(e.tasks, id)
	e.saveToFile()

	log.Printf("[CronEngine] ✓ RemoveTask 成功 ID=%s, 剩余任务数=%d", id, len(e.tasks))
	return nil
}

// ListTasks 返回所有任务
func (e *CronEngine) ListTasks() []*CronTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]*CronTask, 0, len(e.tasks))
	for _, t := range e.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// TriggerTask 立即触发任务执行一次
func (e *CronEngine) TriggerTask(id string) error {
	e.mu.RLock()
	task, exists := e.tasks[id]
	e.mu.RUnlock()

	if !exists {
		log.Printf("[CronEngine] TriggerTask 失败，任务不存在: %s", id)
		return fmt.Errorf("任务不存在: %s", id)
	}

	log.Printf("[CronEngine] TriggerTask 手动触发 ID=%s name=%s type=%s",
		task.ID, task.Name, task.TaskType)
	go e.executeTask(task)
	return nil
}

// Stop 停止引擎
func (e *CronEngine) Stop() {
	log.Printf("[CronEngine] 停止引擎，取消所有定时器...")
	e.cron.Stop()

	e.mu.Lock()
	timerCount := len(e.timers)
	for id, cancel := range e.timers {
		cancel()
		delete(e.timers, id)
	}
	e.mu.Unlock()

	log.Printf("[CronEngine] ✓ 引擎已停止，取消了 %d 个延迟定时器", timerCount)
}

// PendingCount 返回正在执行中的任务数
func (e *CronEngine) PendingCount() int {
	count := 0
	e.pending.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// ListPending 返回所有正在执行中的任务信息（debug 用）
func (e *CronEngine) ListPending() []map[string]string {
	var result []map[string]string
	e.pending.Range(func(key, value interface{}) bool {
		execID, _ := key.(string)
		taskID, _ := value.(string)
		entry := map[string]string{
			"execution_id": execID,
			"task_id":      taskID,
		}
		// 补充任务名称
		e.mu.RLock()
		if task, ok := e.tasks[taskID]; ok {
			entry["task_name"] = task.Name
			entry["task_type"] = task.TaskType
		}
		e.mu.RUnlock()
		result = append(result, entry)
		return true
	})
	return result
}

// HandleTaskComplete 处理 llm-agent 返回的任务完成消息
func (e *CronEngine) HandleTaskComplete(executionID, status, errMsg, result string) {
	cronTaskID, ok := e.pending.LoadAndDelete(executionID)
	if !ok {
		log.Printf("[CronEngine] ⚠ task_complete 未知 executionID=%s (可能已过期或重复)", executionID)
		return
	}

	pendingCount := e.PendingCount()
	if status == "success" {
		log.Printf("[CronEngine] ✓ task_complete executionID=%s cronTask=%s status=%s resultLen=%d pendingLeft=%d",
			executionID, cronTaskID, status, len(result), pendingCount)
		if result != "" {
			// 截取前200字符预览
			preview := result
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			log.Printf("[CronEngine]   └─ result preview: %s", preview)
		}
	} else {
		log.Printf("[CronEngine] ✗ task_complete executionID=%s cronTask=%s status=%s error=%s pendingLeft=%d",
			executionID, cronTaskID, status, errMsg, pendingCount)
	}
}

// ========================= 内部方法 =========================

// scheduleTask 将任务注册到调度系统（调用前需持有 mu 锁）
func (e *CronEngine) scheduleTask(task *CronTask) error {
	taskID := task.ID // 捕获闭包变量

	if task.Schedule != "" {
		log.Printf("[CronEngine] scheduleTask ID=%s 注册 cron/interval schedule=%q oneShot=%v",
			taskID, task.Schedule, task.OneShot)

		// cron 表达式或 @every 间隔
		entryID, err := e.cron.AddFunc(task.Schedule, func() {
			log.Printf("[CronEngine] ⏰ 调度触发 ID=%s name=%s schedule=%s",
				taskID, task.Name, task.Schedule)

			e.mu.RLock()
			t, exists := e.tasks[taskID]
			e.mu.RUnlock()
			if !exists {
				log.Printf("[CronEngine] ⚠ 触发时任务已不存在 ID=%s", taskID)
				return
			}
			e.executeTask(t)
			if t.OneShot {
				log.Printf("[CronEngine] oneShot 任务执行完毕，自动移除 ID=%s", taskID)
				e.RemoveTask(taskID)
			}
		})
		if err != nil {
			return fmt.Errorf("无效的调度表达式 %q: %v", task.Schedule, err)
		}
		e.entries[taskID] = entryID
		log.Printf("[CronEngine]   └─ cron entry 注册成功 entryID=%d", entryID)
		return nil
	}

	if task.DelaySec > 0 {
		log.Printf("[CronEngine] scheduleTask ID=%s 注册延迟任务 delay=%ds",
			taskID, task.DelaySec)

		// 延迟一次性任务
		ctx, cancel := context.WithCancel(context.Background())
		e.timers[taskID] = cancel
		go func() {
			log.Printf("[CronEngine] 延迟定时器启动 ID=%s 将在 %ds 后执行", taskID, task.DelaySec)
			select {
			case <-time.After(time.Duration(task.DelaySec) * time.Second):
				log.Printf("[CronEngine] ⏰ 延迟触发 ID=%s name=%s delay=%ds 已到期",
					taskID, task.Name, task.DelaySec)
				e.executeTask(task)
				e.RemoveTask(taskID)
			case <-ctx.Done():
				log.Printf("[CronEngine] 延迟任务已取消 ID=%s", taskID)
			}
		}()
		return nil
	}

	return fmt.Errorf("任务必须指定 schedule 或 delay_sec")
}

// executeTask 发送 task_assign 到 llm-agent
func (e *CronEngine) executeTask(task *CronTask) {
	executionID := fmt.Sprintf("cron_%s_%d", task.ID, time.Now().UnixMilli())

	log.Printf("[CronEngine] ── executeTask 开始 ──")
	log.Printf("[CronEngine]   任务: ID=%s name=%s type=%s", task.ID, task.Name, task.TaskType)
	log.Printf("[CronEngine]   账号: account=%s wechat_user=%s", task.Account, task.WechatUser)
	log.Printf("[CronEngine]   executionID=%s", executionID)
	log.Printf("[CronEngine]   目标: %s", e.cfg.LLMAgentID)

	// 构建 llm-agent 期望的 payload 格式：{task_type, payload: {inner}}
	var innerPayload interface{}
	switch task.TaskType {
	case "cron_reminder":
		innerPayload = map[string]string{
			"message":     task.Message,
			"account":     task.Account,
			"wechat_user": task.WechatUser,
		}
		log.Printf("[CronEngine]   payload: cron_reminder message=%q", task.Message)
	case "cron_query":
		innerPayload = map[string]string{
			"query":       task.Query,
			"account":     task.Account,
			"wechat_user": task.WechatUser,
		}
		log.Printf("[CronEngine]   payload: cron_query query=%q", task.Query)
	default:
		log.Printf("[CronEngine] ✗ 未知 task_type=%s, 跳过执行", task.TaskType)
		return
	}

	taskPayload := map[string]interface{}{
		"task_type": task.TaskType,
		"payload":   innerPayload,
	}

	// 记录 pending
	e.pending.Store(executionID, task.ID)

	// 发送 task_assign
	payloadJSON, _ := json.Marshal(taskPayload)
	log.Printf("[CronEngine]   发送 task_assign payload=%s", string(payloadJSON))

	err := e.ab.SendMsg(e.cfg.LLMAgentID, uap.MsgTaskAssign, uap.TaskAssignPayload{
		TaskID:  executionID,
		Payload: json.RawMessage(payloadJSON),
	})
	if err != nil {
		e.pending.Delete(executionID)
		log.Printf("[CronEngine] ✗ 发送 task_assign 失败: %v", err)
		log.Printf("[CronEngine]   gateway 是否连接: %v", e.ab.IsConnected())
		return
	}

	log.Printf("[CronEngine] ✓ task_assign 已发送 executionID=%s → %s, 等待 task_complete...",
		executionID, e.cfg.LLMAgentID)
	log.Printf("[CronEngine] ── executeTask 结束 ──")
}

// loadFromFile 从 JSON 文件加载任务
func (e *CronEngine) loadFromFile() error {
	log.Printf("[CronEngine] 读取任务文件: %s", e.cfg.TaskFile)

	data, err := os.ReadFile(e.cfg.TaskFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[CronEngine] 任务文件不存在，跳过加载")
			return nil
		}
		return err
	}

	var tasks []*CronTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("解析任务文件失败: %v", err)
	}

	for _, t := range tasks {
		e.tasks[t.ID] = t
	}
	return nil
}

// saveToFile 持久化任务到 JSON 文件（调用前需持有 mu 锁）
func (e *CronEngine) saveToFile() {
	// 只持久化周期性任务，纯延迟一次性任务不持久化
	tasks := make([]*CronTask, 0)
	for _, t := range e.tasks {
		if t.Schedule != "" {
			tasks = append(tasks, t)
		}
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		log.Printf("[CronEngine] 序列化任务失败: %v", err)
		return
	}

	if err := os.WriteFile(e.cfg.TaskFile, data, 0644); err != nil {
		log.Printf("[CronEngine] 写入任务文件失败: %v", err)
		return
	}

	log.Printf("[CronEngine] 任务文件已保存: %s (%d 个周期任务)", e.cfg.TaskFile, len(tasks))
}

// newTaskID 生成短 UUID
func newTaskID() string {
	return uuid.New().String()[:8]
}
