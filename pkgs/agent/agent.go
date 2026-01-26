package agent

import (
	"encoding/json"
	"mcp"
	log "mylog"
	"sync"
	"time"
)

// 全局变量
var (
	globalHub     *NotificationHub
	globalPool    *WorkerPool
	globalStorage *TaskStorage
	initOnce      sync.Once
	globalAccount string
)

// Info 模块信息
func Info() {
	log.Message(log.ModuleAgent, "info agent v1.0")
}

// Init 初始化 Agent 模块
func Init(account string) {
	initOnce.Do(func() {
		log.Message(log.ModuleAgent, "Initializing agent module")
		globalAccount = account

		// 创建通知中心
		globalHub = NewNotificationHub()
		globalHub.Start()

		// 创建存储
		globalStorage = NewTaskStorage(account)

		// 创建规划器
		planner := NewTaskPlanner(account)

		// 创建工作池（4个 Worker）
		globalPool = NewWorkerPool(4, globalHub, planner, globalStorage)
		globalPool.Start()

		// 初始化调度器
		InitScheduler(globalHub)

		// 注册 MCP 回调
		registerMCPCallbacks()

		// 恢复未完成的任务（仅加载，不自动执行）
		pendingGraphs := globalStorage.GetPendingTaskGraphs()
		// for _, graph := range pendingGraphs {
		// 	globalPool.taskQueue <- graph
		// }

		log.MessageF(log.ModuleAgent, "Agent module initialized, %d pending tasks recovered (awaiting manual start)", len(pendingGraphs))
	})
}

// registerMCPCallbacks 注册 Agent 工具到 MCP 系统
func registerMCPCallbacks() {
	// 创建提醒工具
	mcp.RegisterCallBack("CreateReminder", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		// 获取关联的任务ID（如果有）
		linkedTaskID, _ := args["linked_task_id"].(string)
		result := CreateReminderWithTask(account, args, linkedTaskID)
		data, _ := json.Marshal(result)
		return string(data)
	})

	// 列出提醒工具
	mcp.RegisterCallBack("ListReminders", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		result := ListReminders(account, args)
		data, _ := json.Marshal(result)
		return string(data)
	})

	// 删除提醒工具
	mcp.RegisterCallBack("DeleteReminder", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		result := DeleteReminder(account, args)
		data, _ := json.Marshal(result)
		return string(data)
	})

	// 发送通知工具
	mcp.RegisterCallBack("SendNotification", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		result := SendNotification(account, args)
		data, _ := json.Marshal(result)
		return string(data)
	})

	log.Message(log.ModuleAgent, "Agent MCP callbacks registered: CreateReminder, ListReminders, DeleteReminder, SendNotification")
}

// GetHub 获取通知中心
func GetHub() *NotificationHub {
	return globalHub
}

// GetPool 获取工作池
func GetPool() *WorkerPool {
	return globalPool
}

// GetStorage 获取存储
func GetStorage() *TaskStorage {
	return globalStorage
}

// GetReminderInfo 获取提醒信息
func GetReminderInfo(reminderID string) *Reminder {
	if globalScheduler != nil {
		return globalScheduler.GetReminderByID(reminderID)
	}
	return nil
}

// CreateTask 创建并提交任务
func CreateTask(account, title, description string, priority int) *TaskGraph {
	if globalPool != nil {
		return globalPool.Submit(account, title, description)
	}
	return nil
}

// GetTaskGraph 获取任务图
func GetTaskGraph(taskID string) *TaskGraph {
	if globalPool != nil {
		return globalPool.GetTaskGraphByID(taskID)
	}
	return nil
}

// GetTaskGraphs 获取账户的所有任务图
func GetTaskGraphs(account string) []*TaskGraph {
	if globalPool != nil {
		return globalPool.GetAllTaskGraphs(account)
	}
	return nil
}

// TaskSummary 任务摘要（轻量级，用于列表显示）
type TaskSummary struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Status    NodeStatus `json:"status"`
	Progress  float64    `json:"progress"`
	CreatedAt time.Time  `json:"created_at"`
}

// GetTaskSummaries 获取账户的任务摘要列表（轻量级）
func GetTaskSummaries(account string) []TaskSummary {
	graphs := GetTaskGraphs(account)
	summaries := make([]TaskSummary, 0, len(graphs))
	for _, g := range graphs {
		if g.Root != nil {
			summaries = append(summaries, TaskSummary{
				ID:        g.RootID,
				Title:     g.Root.Title,
				Status:    g.Root.Status,
				Progress:  g.CalculateProgress(),
				CreatedAt: g.StartTime,
			})
		}
	}
	return summaries
}

// GetActiveTaskIDs 获取当前正在执行的任务 ID 列表
func GetActiveTaskIDs() []string {
	if globalPool != nil {
		return globalPool.GetActiveTaskIDs()
	}
	return []string{}
}

// IsTaskActive 检查任务是否正在执行
func IsTaskActive(taskID string) bool {
	if globalPool != nil {
		return globalPool.IsTaskActive(taskID)
	}
	return false
}

// PauseTask 暂停任务
func PauseTask(taskID string) bool {
	if globalPool != nil {
		return globalPool.PauseTask(taskID)
	}
	return false
}

// ResumeTask 恢复任务
func ResumeTask(taskID string) bool {
	if globalPool != nil {
		return globalPool.ResumeTask(taskID)
	}
	return false
}

// CancelTask 取消任务
func CancelTask(taskID string) bool {
	if globalPool != nil {
		return globalPool.CancelTask(taskID)
	}
	return false
}

// DeleteTask 删除任务
func DeleteTask(taskID string) bool {
	if globalStorage != nil {
		// 先取消任务（如果运行中）
		if globalPool != nil {
			globalPool.CancelTask(taskID)
		}
		// 删除 TaskGraph
		return globalStorage.DeleteTaskGraph(taskID) == nil
	}
	return false
}

// ============================================================================
// 新版 TaskNode API（支持递归拆解、串行/并行执行）
// ============================================================================

// CreateTaskNode 创建 TaskNode 任务（新版）
func CreateTaskNode(account, title, description string) *TaskNode {
	node := NewTaskNode(account, title, description)
	node.Goal = description
	return node
}

// SubmitTaskNode 提交 TaskNode 任务（异步执行）
func SubmitTaskNode(node *TaskNode, config *ExecutionConfig) *TaskGraph {
	if globalPool == nil {
		return nil
	}
	return globalPool.SubmitTaskNode(node, config)
}

// ExecuteTaskNodeSync 同步执行 TaskNode 任务
func ExecuteTaskNodeSync(node *TaskNode, config *ExecutionConfig) (*TaskGraph, error) {
	if globalPool == nil {
		return nil, nil
	}
	return globalPool.ExecuteTaskNodeSync(node, config)
}

// GetGraphVisualization 获取任务图可视化数据
func GetGraphVisualization(graph *TaskGraph) *GraphVisualization {
	if graph == nil {
		return nil
	}
	return graph.ToVisualization()
}

// GetGraphJSON 获取任务图 JSON 数据
func GetGraphJSON(graph *TaskGraph) string {
	if graph == nil {
		return "{}"
	}
	return graph.ToJSON()
}

// GetDefaultExecutionConfig 获取默认执行配置
func GetDefaultExecutionConfig() *ExecutionConfig {
	return DefaultExecutionConfig()
}

// NewExecutionConfig 创建自定义执行配置
func NewExecutionConfig(maxDepth, maxContextLen, maxRetries int) *ExecutionConfig {
	return &ExecutionConfig{
		MaxDepth:         maxDepth,
		MaxContextLen:    maxContextLen,
		MaxRetries:       maxRetries,
		ExecutionTimeout: 60 * time.Minute, // 1小时
		EnableLogging:    true,
	}
}

// ============================================================================
// 任务重试 API
// ============================================================================

// RetryTask 重试失败的任务（从失败节点继续执行）
// 保留已完成节点的结果，仅重新执行失败/取消的节点
func RetryTask(taskID string) bool {
	if globalStorage == nil || globalPool == nil {
		log.ErrorF(log.ModuleAgent, "RetryTask: storage or pool not initialized")
		return false
	}

	// 从存储加载任务图
	graph := globalStorage.GetTaskGraph(taskID)
	if graph == nil {
		log.ErrorF(log.ModuleAgent, "RetryTask: task not found: %s", taskID)
		return false
	}

	// 重置失败节点
	resetCount := graph.ResetFailedNodes()
	if resetCount == 0 {
		log.WarnF(log.ModuleAgent, "RetryTask: no failed nodes to retry in task: %s", taskID)
		return false
	}

	log.MessageF(log.ModuleAgent, "RetryTask: reset %d failed nodes, resubmitting task: %s", resetCount, taskID)

	// 保存重置后的状态
	globalStorage.SaveTaskGraph(graph)

	// 重新提交到工作池
	if !globalPool.ResubmitGraph(graph) {
		log.ErrorF(log.ModuleAgent, "RetryTask: failed to resubmit task: %s", taskID)
		return false
	}
	return true
}

// ============================================================================
// 用户输入 API
// ============================================================================

// SubmitTaskInput 提交任务输入响应
func SubmitTaskInput(taskID, nodeID string, resp *InputResponse) error {
	if globalStorage == nil {
		return nil
	}

	// 优先从图缓存中查找节点
	graph := globalStorage.GetTaskGraph(taskID)
	if graph != nil {
		node := graph.GetNode(nodeID)
		if node != nil && node.IsWaitingInput() {
			node.ReceiveInput(resp)
			return nil
		}
	}

	return nil
}

// GetPendingInputs 获取某任务的所有待处理输入请求
func GetPendingInputs(taskID string) []*InputRequest {
	if globalStorage == nil {
		return nil
	}

	var requests []*InputRequest

	// 从图缓存查找
	graph := globalStorage.GetTaskGraph(taskID)
	if graph != nil {
		for _, node := range graph.Nodes {
			if node.HasPendingInput() {
				requests = append(requests, node.GetPendingInput())
			}
		}
	}

	return requests
}

// GetWaitingNode 获取等待输入的节点
func GetWaitingNode(taskID, nodeID string) *TaskNode {
	if globalStorage == nil {
		return nil
	}

	graph := globalStorage.GetTaskGraph(taskID)
	if graph != nil {
		node := graph.GetNode(nodeID)
		if node != nil && node.IsWaitingInput() {
			return node
		}
	}

	return nil
}

// GetTaskGraphData 获取任务图数据（用于 API）
func GetTaskGraphData(rootID string) map[string]interface{} {
	if globalStorage == nil {
		return map[string]interface{}{"success": false, "error": "Storage not initialized"}
	}

	// 尝试获取新版 TaskGraph
	graph := globalStorage.GetTaskGraph(rootID)
	if graph != nil {
		// 新版任务图
		vis := graph.ToVisualization()
		logs := graph.GetAllLogs()
		return map[string]interface{}{
			"success": true,
			"graph":   vis,
			"logs":    logs,
		}
	}

	return map[string]interface{}{"success": false, "error": "Task not found"}
}

// SaveTaskGraph 保存任务图（用于执行器）
func SaveTaskGraph(graph *TaskGraph) error {
	if globalStorage == nil {
		return nil
	}
	return globalStorage.SaveTaskGraph(graph)
}

// Shutdown 关闭 Agent 模块
func Shutdown() {
	ShutdownScheduler()
	if globalPool != nil {
		globalPool.Shutdown()
	}
	log.Message(log.ModuleAgent, "Agent module shutdown")
}
