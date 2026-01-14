package agent

import (
	"context"
	log "mylog"
	"sync"
	"time"
)

// WorkerPool 工作池
type WorkerPool struct {
	taskQueue    chan *AgentTask
	workers      []*Worker
	maxWorkers   int
	notification *NotificationHub
	planner      *TaskPlanner
	storage      *TaskStorage
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// Worker 单个工作者
type Worker struct {
	id          int
	pool        *WorkerPool
	currentTask *AgentTask
	mu          sync.RWMutex
}

// NewWorkerPool 创建工作池
func NewWorkerPool(maxWorkers int, notification *NotificationHub, planner *TaskPlanner, storage *TaskStorage) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		taskQueue:    make(chan *AgentTask, 100),
		workers:      make([]*Worker, 0, maxWorkers),
		maxWorkers:   maxWorkers,
		notification: notification,
		planner:      planner,
		storage:      storage,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start 启动工作池
func (p *WorkerPool) Start() {
	log.MessageF(log.ModuleAgent, "Starting worker pool with %d workers", p.maxWorkers)
	for i := 0; i < p.maxWorkers; i++ {
		worker := &Worker{
			id:   i,
			pool: p,
		}
		p.workers = append(p.workers, worker)
		p.wg.Add(1)
		go worker.run()
	}
}

// Shutdown 关闭工作池
func (p *WorkerPool) Shutdown() {
	log.Message(log.ModuleAgent, "Shutting down worker pool")
	p.cancel()
	close(p.taskQueue)
	p.wg.Wait()
	log.Message(log.ModuleAgent, "Worker pool shutdown complete")
}

// Submit 提交任务
func (p *WorkerPool) Submit(task *AgentTask) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 保存任务到存储
	p.storage.SaveTask(task)

	// 发送到队列
	select {
	case p.taskQueue <- task:
		log.MessageF(log.ModuleAgent, "Task submitted: %s", task.ID)
		// 通知客户端
		p.notification.Broadcast(TaskNotification{
			TaskID:   task.ID,
			Type:     "submitted",
			Progress: 0,
			Message:  "任务已提交到队列",
		})
	default:
		log.WarnF(log.ModuleAgent, "Task queue full, task %s rejected", task.ID)
		task.SetStatus(StatusFailed)
		task.Error = "任务队列已满"
		p.storage.SaveTask(task)
	}
}

// GetTaskByID 获取任务
func (p *WorkerPool) GetTaskByID(taskID string) *AgentTask {
	return p.storage.GetTask(taskID)
}

// GetAllTasks 获取所有任务
func (p *WorkerPool) GetAllTasks(account string) []*AgentTask {
	return p.storage.GetTasksByAccount(account)
}

// PauseTask 暂停任务
func (p *WorkerPool) PauseTask(taskID string) bool {
	task := p.storage.GetTask(taskID)
	if task != nil && task.GetStatus() == StatusRunning {
		task.Pause()
		p.storage.SaveTask(task)
		p.notification.Broadcast(TaskNotification{
			TaskID:  taskID,
			Type:    "paused",
			Message: "任务已暂停",
		})
		return true
	}
	return false
}

// ResumeTask 恢复任务
func (p *WorkerPool) ResumeTask(taskID string) bool {
	task := p.storage.GetTask(taskID)
	if task != nil && task.GetStatus() == StatusPaused {
		task.Resume()
		p.storage.SaveTask(task)
		// 重新提交到队列
		p.taskQueue <- task
		p.notification.Broadcast(TaskNotification{
			TaskID:  taskID,
			Type:    "resumed",
			Message: "任务已恢复",
		})
		return true
	}
	return false
}

// CancelTask 取消任务
func (p *WorkerPool) CancelTask(taskID string) bool {
	task := p.storage.GetTask(taskID)
	if task != nil {
		status := task.GetStatus()
		if status == StatusPending || status == StatusRunning || status == StatusPaused {
			task.Cancel()
			p.storage.SaveTask(task)
			p.notification.Broadcast(TaskNotification{
				TaskID:  taskID,
				Type:    "canceled",
				Message: "任务已取消",
			})
			return true
		}
	}
	return false
}

// Worker.run 工作者主循环
func (w *Worker) run() {
	defer w.pool.wg.Done()
	log.MessageF(log.ModuleAgent, "Worker %d started", w.id)

	for {
		select {
		case <-w.pool.ctx.Done():
			log.MessageF(log.ModuleAgent, "Worker %d stopping", w.id)
			return
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				return
			}
			w.executeTask(task)
		}
	}
}

// Worker.executeTask 执行任务
func (w *Worker) executeTask(task *AgentTask) {
	w.mu.Lock()
	w.currentTask = task
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.currentTask = nil
		w.mu.Unlock()
	}()

	log.MessageF(log.ModuleAgent, "Worker %d executing task: %s", w.id, task.ID)

	// 设置为运行中
	task.SetStatus(StatusRunning)
	task.AddLog("info", "任务开始执行")
	w.pool.storage.SaveTask(task)
	w.pool.notification.Broadcast(TaskNotification{
		TaskID:   task.ID,
		Type:     "started",
		Progress: 0,
		Message:  "任务开始执行",
	})

	// 如果没有子任务，先进行任务分解
	if len(task.SubTasks) == 0 {
		subtasks, err := w.pool.planner.PlanTask(task.Description)
		if err != nil {
			task.SetStatus(StatusFailed)
			task.Error = err.Error()
			task.AddLog("error", "任务分解失败: "+err.Error())
			w.pool.storage.SaveTask(task)
			w.pool.notification.Broadcast(TaskNotification{
				TaskID:  task.ID,
				Type:    "error",
				Message: "任务分解失败: " + err.Error(),
			})
			return
		}
		task.SubTasks = subtasks
		task.AddLog("info", "任务分解完成，共 "+string(rune(len(subtasks)+'0'))+" 个子任务")
		w.pool.storage.SaveTask(task)
	}

	// 执行子任务
	totalSubTasks := len(task.SubTasks)
	for i := range task.SubTasks {
		// 检查取消
		if task.IsCanceled() {
			task.AddLog("info", "任务已取消")
			w.pool.storage.SaveTask(task)
			return
		}

		// 检查暂停
		for task.IsPaused() {
			time.Sleep(500 * time.Millisecond)
			if task.IsCanceled() {
				return
			}
		}

		task.CurrentStep = i
		task.SubTasks[i].Status = "running"
		w.pool.storage.SaveTask(task)

		// 执行子任务
		result, err := w.pool.planner.ExecuteSubTask(task, &task.SubTasks[i])
		if err != nil {
			task.SubTasks[i].Status = "failed"
			task.SubTasks[i].Error = err.Error()
			task.AddLog("error", "子任务执行失败: "+err.Error())
		} else {
			task.SubTasks[i].Status = "done"
			task.SubTasks[i].Result = result
			task.AddLog("info", "子任务完成: "+task.SubTasks[i].Description)
		}

		// 更新进度
		progress := float64(i+1) / float64(totalSubTasks) * 100
		task.SetProgress(progress)
		w.pool.storage.SaveTask(task)
		w.pool.notification.Broadcast(TaskNotification{
			TaskID:   task.ID,
			Type:     "progress",
			Progress: progress,
			Message:  task.SubTasks[i].Description,
		})
	}

	// 任务完成
	task.SetStatus(StatusDone)
	task.AddLog("info", "任务执行完成")
	w.pool.storage.SaveTask(task)
	w.pool.notification.Broadcast(TaskNotification{
		TaskID:   task.ID,
		Type:     "done",
		Progress: 100,
		Message:  "任务执行完成",
	})
}
