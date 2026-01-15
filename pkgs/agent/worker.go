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
		taskQueue:    make(chan *TaskGraph, 100), // 使用 TaskGraph
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
	graph := p.storage.GetTaskGraph(taskID)
	if graph != nil && graph.Root != nil {
		if graph.Root.GetStatus() == NodePaused {
			graph.Root.Resume()
			p.storage.SaveTaskGraph(graph)
			// 重新提交到队列
			p.taskQueue <- graph
			p.notification.Broadcast(TaskNotification{
				TaskID:  taskID,
				Type:    "resumed",
				Message: "任务已恢复",
			})
			return true
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

	defer func() {
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
