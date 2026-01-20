package agent

import (
	"context"
	log "mylog"
	"sync"
)

// WorkerPool 工作池
type WorkerPool struct {
	taskQueue    chan *TaskGraph // 改为 TaskGraph 队列
	workers      []*Worker
	maxWorkers   int
	notification *NotificationHub
	planner      *TaskPlanner
	storage      *TaskStorage
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
	// 活跃任务追踪
	activeGraphs map[string]bool // 当前正在执行的 TaskGraph ID
	activeMu     sync.RWMutex
}

// Worker 单个工作者
type Worker struct {
	id           int
	pool         *WorkerPool
	currentGraph *TaskGraph // 改为跟踪 TaskGraph
	mu           sync.RWMutex
}

// NewWorkerPool 创建工作池
func NewWorkerPool(maxWorkers int, notification *NotificationHub, planner *TaskPlanner, storage *TaskStorage) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		taskQueue:    make(chan *TaskGraph, 100),
		workers:      make([]*Worker, 0, maxWorkers),
		maxWorkers:   maxWorkers,
		notification: notification,
		planner:      planner,
		storage:      storage,
		ctx:          ctx,
		cancel:       cancel,
		activeGraphs: make(map[string]bool), // 初始化活跃任务映射
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

// Submit 提交任务（创建 TaskNode 和 TaskGraph）
func (p *WorkerPool) Submit(account, title, description string) *TaskGraph {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 创建根节点
	root := NewTaskNode(account, title, description)
	root.Goal = description

	// 创建任务图
	graph := NewTaskGraph(root, DefaultExecutionConfig())

	// 保存到存储
	p.storage.SaveTaskGraph(graph)

	// 发送到队列
	select {
	case p.taskQueue <- graph:
		log.MessageF(log.ModuleAgent, "Task graph submitted: %s", graph.RootID)
		// 通知客户端
		p.notification.Broadcast(TaskNotification{
			TaskID:   graph.RootID,
			Type:     "submitted",
			Progress: 0,
			Message:  "任务已提交到队列",
		})
	default:
		log.WarnF(log.ModuleAgent, "Task queue full, task %s rejected", graph.RootID)
		root.SetStatus(NodeFailed)
		root.Result = NewTaskResultError("任务队列已满")
		p.storage.SaveTaskGraph(graph)
	}

	return graph
}

// ResubmitGraph 重新提交已有的任务图（用于重试）
func (p *WorkerPool) ResubmitGraph(graph *TaskGraph) bool {
	if graph == nil {
		return false
	}

	// 发送到队列
	select {
	case p.taskQueue <- graph:
		log.MessageF(log.ModuleAgent, "Task graph resubmitted for retry: %s", graph.RootID)
		p.notification.Broadcast(TaskNotification{
			TaskID:   graph.RootID,
			Type:     "retrying",
			Progress: graph.CalculateProgress(),
			Message:  "任务已重新提交执行",
		})
		return true
	default:
		log.WarnF(log.ModuleAgent, "Task queue full, retry for %s rejected", graph.RootID)
		return false
	}
}

// GetTaskGraphByID 获取任务图
func (p *WorkerPool) GetTaskGraphByID(taskID string) *TaskGraph {
	return p.storage.GetTaskGraph(taskID)
}

// GetAllTaskGraphs 获取所有任务图
func (p *WorkerPool) GetAllTaskGraphs(account string) []*TaskGraph {
	return p.storage.GetTaskGraphsByAccount(account)
}

// PauseTask 暂停任务
func (p *WorkerPool) PauseTask(taskID string) bool {
	graph := p.storage.GetTaskGraph(taskID)
	if graph != nil && graph.Root != nil {
		status := graph.Root.GetStatus()
		if status == NodeRunning {
			graph.Root.Pause()
			p.storage.SaveTaskGraph(graph)
			p.notification.Broadcast(TaskNotification{
				TaskID:  taskID,
				Type:    "paused",
				Message: "任务已暂停",
			})
			return true
		}
	}
	return false
}

// ResumeTask 恢复任务
func (p *WorkerPool) ResumeTask(taskID string) bool {
	// 如果任务已经在执行中，无需恢复
	if p.IsTaskActive(taskID) {
		log.MessageF(log.ModuleAgent, "Task %s is already active, ignore resume", taskID)
		return true
	}

	graph := p.storage.GetTaskGraph(taskID)
	if graph != nil && graph.Root != nil {
		status := graph.Root.GetStatus()
		// 允许恢复 Paused 状态任务，以及因重启等原因处于 Pending/Running 但未实际执行的任务
		canResume := status == NodePaused || status == NodeRunning || status == NodePending

		if canResume {
			// 如果是暂停状态，切换回运行状态
			if status == NodePaused {
				graph.Root.Resume()
				p.storage.SaveTaskGraph(graph)
			}

			// 重新提交到队列
			select {
			case p.taskQueue <- graph:
				log.MessageF(log.ModuleAgent, "Task %s resumed (re-queued)", taskID)
				p.notification.Broadcast(TaskNotification{
					TaskID:  taskID,
					Type:    "resumed",
					Message: "任务已恢复",
				})
				return true
			default:
				log.WarnF(log.ModuleAgent, "Task queue full, failed to resume task %s", taskID)
				return false
			}
		}
	}
	return false
}

// CancelTask 取消任务
func (p *WorkerPool) CancelTask(taskID string) bool {
	graph := p.storage.GetTaskGraph(taskID)
	if graph != nil && graph.Root != nil {
		status := graph.Root.GetStatus()
		if status == NodePending || status == NodeRunning || status == NodePaused {
			graph.Root.Cancel()
			p.storage.SaveTaskGraph(graph)
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

// IsTaskActive 检查任务是否正在执行
func (p *WorkerPool) IsTaskActive(taskID string) bool {
	p.activeMu.RLock()
	defer p.activeMu.RUnlock()
	return p.activeGraphs[taskID]
}

// GetActiveTaskIDs 获取所有正在执行的任务 ID
func (p *WorkerPool) GetActiveTaskIDs() []string {
	p.activeMu.RLock()
	defer p.activeMu.RUnlock()
	ids := make([]string, 0, len(p.activeGraphs))
	for id := range p.activeGraphs {
		ids = append(ids, id)
	}
	return ids
}

// ============================================================================
// 新版 TaskNode 执行接口
// ============================================================================

// SubmitTaskNode 提交 TaskNode 任务（新版）
func (p *WorkerPool) SubmitTaskNode(node *TaskNode, config *ExecutionConfig) *TaskGraph {
	if config == nil {
		config = DefaultExecutionConfig()
	}

	// 创建任务图
	graph := NewTaskGraph(node, config)

	// 异步执行
	go p.executeTaskNode(graph)

	return graph
}

// executeTaskNode 执行 TaskNode 任务
func (p *WorkerPool) executeTaskNode(graph *TaskGraph) {
	log.MessageF(log.ModuleAgent, "Executing TaskNode: %s", graph.Root.Title)

	// 创建执行器
	executor := NewTaskExecutor(graph, p.planner, p.notification, p.storage)

	// 执行
	err := executor.Execute()

	// 输出最终结果
	if err != nil {
		log.WarnF(log.ModuleAgent, "TaskNode execution failed: %v", err)
	} else {
		log.MessageF(log.ModuleAgent, "TaskNode execution completed: %s", graph.Root.Title)
	}
}

// CreateTaskNode 创建 TaskNode 任务（便捷方法）
func (p *WorkerPool) CreateTaskNode(account, title, description string) *TaskNode {
	node := NewTaskNode(account, title, description)
	node.Goal = description
	return node
}

// ExecuteTaskNodeSync 同步执行 TaskNode 任务
func (p *WorkerPool) ExecuteTaskNodeSync(node *TaskNode, config *ExecutionConfig) (*TaskGraph, error) {
	if config == nil {
		config = DefaultExecutionConfig()
	}

	// 创建任务图
	graph := NewTaskGraph(node, config)

	// 创建执行器
	executor := NewTaskExecutor(graph, p.planner, p.notification, p.storage)

	// 同步执行
	err := executor.Execute()

	return graph, err
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
		case graph, ok := <-w.pool.taskQueue:
			if !ok {
				return
			}
			w.executeGraph(graph)
		}
	}
}

// Worker.executeGraph 执行任务图
func (w *Worker) executeGraph(graph *TaskGraph) {
	w.mu.Lock()
	w.currentGraph = graph
	w.mu.Unlock()

	// 注册活跃任务
	w.pool.activeMu.Lock()
	w.pool.activeGraphs[graph.RootID] = true
	w.pool.activeMu.Unlock()

	defer func() {
		// 取消注册活跃任务
		w.pool.activeMu.Lock()
		delete(w.pool.activeGraphs, graph.RootID)
		w.pool.activeMu.Unlock()

		w.mu.Lock()
		w.currentGraph = nil
		w.mu.Unlock()
	}()

	log.MessageF(log.ModuleAgent, "Worker %d executing graph: %s", w.id, graph.RootID)

	// 创建执行器
	executor := NewTaskExecutor(graph, w.pool.planner, w.pool.notification, w.pool.storage)

	// 执行任务图
	err := executor.Execute()
	if err != nil {
		log.WarnF(log.ModuleAgent, "Graph execution error: %v", err)
	}

	// 保存最终状态
	w.pool.storage.SaveTaskGraph(graph)
	log.MessageF(log.ModuleAgent, "Worker %d completed graph: %s", w.id, graph.RootID)
}
