package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// jobWrapper 包装函数以实现 cron.Job 接口
type jobWrapper struct {
	f func()
}

func (j jobWrapper) Run() {
	j.f()
}

// Scheduler 任务调度器
type Scheduler struct {
	cfg       *Config
	storage   *Storage
	cron      *cron.Cron
	executor  *TaskExecutor                 // 任务执行器
	mu        sync.RWMutex
	tasks     map[string]*CronTask          // 任务映射
	entries   map[string]cron.EntryID       // 任务ID到cron条目ID的映射
	onceTimers map[string]*time.Timer       // 一次性任务timer
	running   map[string]bool               // 正在执行的任务
	semaphore chan struct{}                 // 并发控制信号量
	stopCh    chan struct{}                 // 停止通道
}

// NewScheduler 创建调度器
func NewScheduler(cfg *Config, storage *Storage) *Scheduler {
	loc := cfg.GetLocation()
	c := cron.New(cron.WithLocation(loc), cron.WithSeconds())

	return &Scheduler{
		cfg:        cfg,
		storage:    storage,
		cron:       c,
		executor:   NewTaskExecutor(cfg, storage, nil), // conn稍后设置
		tasks:      make(map[string]*CronTask),
		entries:    make(map[string]cron.EntryID),
		onceTimers: make(map[string]*time.Timer),
		running:    make(map[string]bool),
		semaphore:  make(chan struct{}, cfg.Scheduler.MaxConcurrent),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	log.Printf("[Scheduler] 启动调度器，最大并发=%d", s.cfg.Scheduler.MaxConcurrent)
	s.cron.Start()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	log.Printf("[Scheduler] 停止调度器")
	close(s.stopCh)
	s.cron.Stop()
	// 停止所有一次性任务定时器
	for taskID, timer := range s.onceTimers {
		timer.Stop()
		delete(s.onceTimers, taskID)
	}
}

// AddTask 添加任务到调度器
func (s *Scheduler) AddTask(task *CronTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果任务已存在，先移除
	if _, exists := s.entries[task.ID]; exists {
		s.removeTaskInternal(task.ID)
	}

	// 如果任务未启用，只存储在内存中不调度
	if !task.Enabled {
		s.tasks[task.ID] = task
		return nil
	}

	// 根据调度类型添加任务
	var entryID cron.EntryID
	var err error

	switch task.ScheduleType {
	case ScheduleTypeCron:
		entryID, err = s.addCronTask(task)
	case ScheduleTypeOnce:
		// 一次性任务使用time.Timer，不通过cron调度
		entryID = 0 // 虚拟ID
		err = s.addOnceTask(task)
	case ScheduleTypeInterval:
		entryID, err = s.addIntervalTask(task)
	default:
		return fmt.Errorf("未知的调度类型: %s", task.ScheduleType)
	}

	if err != nil {
		return err
	}

	s.tasks[task.ID] = task
	s.entries[task.ID] = entryID
	log.Printf("[Scheduler] 任务已添加 ID=%s name=%s type=%s", task.ID, task.Name, task.ScheduleType)
	return nil
}

// addCronTask 添加cron任务
func (s *Scheduler) addCronTask(task *CronTask) (cron.EntryID, error) {
	if task.CronExpr == "" {
		return 0, fmt.Errorf("cron表达式不能为空")
	}

	// 验证cron表达式
	if _, err := cron.ParseStandard(task.CronExpr); err != nil {
		return 0, fmt.Errorf("无效的cron表达式: %v", err)
	}

	// 创建jobWrapper
	job := jobWrapper{f: s.createJobFunc(task)}
	return s.cron.AddJob(task.CronExpr, job)
}

// addOnceTask 添加一次性任务
func (s *Scheduler) addOnceTask(task *CronTask) error {
	// 一次性任务：立即执行或指定时间执行
	// 如果NextRunAt未设置，则立即执行
	now := time.Now()
	if task.NextRunAt.IsZero() || task.NextRunAt.Before(now) {
		task.NextRunAt = now.Add(1 * time.Second) // 1秒后执行
	}

	// 计算延迟
	delay := task.NextRunAt.Sub(now)
	if delay < 0 {
		delay = 0
	}

	// 创建并启动定时器
	timer := time.AfterFunc(delay, func() {
		s.executeTask(task)
	})

	// 存储定时器以便后续管理
	s.onceTimers[task.ID] = timer
	log.Printf("[Scheduler] 一次性任务定时器已设置 ID=%s delay=%v", task.ID, delay)
	return nil
}

// addIntervalTask 添加间隔任务
func (s *Scheduler) addIntervalTask(task *CronTask) (cron.EntryID, error) {
	if task.IntervalSec <= 0 {
		return 0, fmt.Errorf("间隔秒数必须大于0")
	}

	// 使用cron表达式实现间隔任务
	// 例如每5秒: "*/5 * * * * *"
	cronExpr := fmt.Sprintf("*/%d * * * * *", task.IntervalSec)
	if task.IntervalSec < 1 {
		cronExpr = fmt.Sprintf("@every %ds", task.IntervalSec)
	}

	// 创建jobWrapper
	job := jobWrapper{f: s.createJobFunc(task)}
	return s.cron.AddJob(cronExpr, job)
}

// RemoveTask 从调度器移除任务
func (s *Scheduler) RemoveTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeTaskInternal(taskID)
}

// removeTaskInternal 内部移除任务实现
func (s *Scheduler) removeTaskInternal(taskID string) error {
	if entryID, exists := s.entries[taskID]; exists && entryID != 0 {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}
	// 停止并删除一次性任务定时器
	if timer, exists := s.onceTimers[taskID]; exists {
		timer.Stop()
		delete(s.onceTimers, taskID)
	}
	delete(s.tasks, taskID)
	delete(s.running, taskID)
	log.Printf("[Scheduler] 任务已移除 ID=%s", taskID)
	return nil
}

// UpdateTask 更新任务
func (s *Scheduler) UpdateTask(task *CronTask) error {
	// 先移除旧任务
	if err := s.RemoveTask(task.ID); err != nil {
		// 如果任务不存在，继续添加
		log.Printf("[Scheduler] 更新时移除任务失败（可能不存在）: %v", err)
	}

	// 添加新任务
	return s.AddTask(task)
}

// GetTask 获取任务
func (s *Scheduler) GetTask(taskID string) (*CronTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, exists := s.tasks[taskID]
	return task, exists
}

// ListTasks 列出所有任务
func (s *Scheduler) ListTasks() []*CronTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]*CronTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// TriggerTask 手动触发任务
func (s *Scheduler) TriggerTask(taskID string) error {
	s.mu.RLock()
	task, exists := s.tasks[taskID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	// 异步执行任务
	go s.executeTask(task)
	return nil
}

// createJobFunc 创建任务执行函数
func (s *Scheduler) createJobFunc(task *CronTask) func() {
	return func() {
		s.executeTask(task)
	}
}

// executeTask 执行任务
func (s *Scheduler) executeTask(task *CronTask) {
	// 并发控制
	select {
	case s.semaphore <- struct{}{}:
		// 获取到信号量
	default:
		log.Printf("[Scheduler] 并发限制，任务等待中 ID=%s", task.ID)
		<-s.semaphore // 等待一个槽位
		s.semaphore <- struct{}{}
	}
	defer func() { <-s.semaphore }()

	// 检查任务是否仍在运行列表中
	s.mu.Lock()
	if s.running[task.ID] {
		s.mu.Unlock()
		log.Printf("[Scheduler] 任务已在执行中，跳过 ID=%s", task.ID)
		return
	}
	s.running[task.ID] = true
	s.mu.Unlock()

	// 执行完成后清理running状态
	defer func() {
		s.mu.Lock()
		delete(s.running, task.ID)
		s.mu.Unlock()
	}()

	log.Printf("[Scheduler] 开始执行任务 ID=%s name=%s", task.ID, task.Name)

	// 更新任务状态为运行中
	s.storage.UpdateTaskStatus(task.ID, TaskStatusRunning)

	// 创建执行记录
	executionID := fmt.Sprintf("exec_%d", time.Now().UnixNano())
	startedAt := time.Now()

	// 调用任务执行器
	result, err := s.executor.Execute(task)

	endedAt := time.Now()
	duration := endedAt.Sub(startedAt).Milliseconds()

	// 保存执行记录
	execution := &TaskExecution{
		ID:        executionID,
		TaskID:    task.ID,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Duration:  duration,
	}

	if err != nil {
		execution.Status = ExecutionStatusFailed
		execution.Error = err.Error()
		execution.Result = "任务执行失败"
		log.Printf("[Scheduler] 任务执行失败 ID=%s: %v", task.ID, err)
		s.storage.UpdateTaskStatus(task.ID, TaskStatusFailed)
	} else {
		execution.Status = ExecutionStatusSuccess
		execution.Result = result
		log.Printf("[Scheduler] 任务执行成功 ID=%s 耗时=%dms", task.ID, duration)
		s.storage.UpdateTaskStatus(task.ID, TaskStatusPending)
	}

	// 保存执行记录
	if err := s.storage.SaveExecution(execution); err != nil {
		log.Printf("[Scheduler] 保存执行记录失败: %v", err)
	}

	// 如果是one-off任务，执行后禁用
	if task.ScheduleType == ScheduleTypeOnce {
		task.Enabled = false
		task.Status = TaskStatusCompleted
		s.storage.UpdateTask(task.ID, &TaskUpdateRequest{Enabled: &task.Enabled})
		s.RemoveTask(task.ID)
	}
}

// UpdateTaskNextRun 更新任务下次执行时间（供外部调用）
func (s *Scheduler) UpdateTaskNextRun(taskID string, nextRunAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.tasks[taskID]; exists {
		task.NextRunAt = nextRunAt
		s.tasks[taskID] = task
	}
}

// SetConnection 设置UAP连接，用于任务执行器
func (s *Scheduler) SetConnection(conn *Connection) {
	s.executor.conn = conn
}