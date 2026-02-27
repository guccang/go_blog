package agent

import (
	"email"
	"encoding/json"
	"fmt"
	"llm"
	"mcp"
	log "mylog"
	"sync"
	"time"
	"wechat"
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
		LoadScheduledTasks(account) // 从博客加载持久化的定时任务
		RegisterSchedulerMCPTools() // 注册 AI 定时任务 MCP 工具

		// 初始化邮件模块
		email.InitEmailConfig()

		// 初始化企业微信模块
		wechat.InitWechatConfig()
		wechat.SetCommandHandler(handleWechatCommand)

		// 初始化报告生成器
		InitReportGenerator(account)

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

	// 生成报告工具
	mcp.RegisterCallBack("GenerateReport", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		reportType, _ := args["type"].(string)
		if reportType == "" {
			reportType = "daily"
		}
		report, err := GenerateReport(account, reportType)
		if err != nil {
			return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
		}
		return fmt.Sprintf(`{"success":true,"type":"%s","length":%d}`, reportType, len(report))
	})

	// 切换模型工具
	mcp.RegisterCallBack("SwitchModel", func(args map[string]interface{}) string {
		provider, _ := args["provider"].(string)
		if provider == "" {
			return `{"success":false,"error":"missing provider name"}`
		}
		if err := llm.SwitchModel(provider); err != nil {
			return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
		}
		data, _ := json.Marshal(llm.GetModelInfo())
		return string(data)
	})

	// 获取当前模型信息工具
	mcp.RegisterCallBack("GetCurrentModel", func(args map[string]interface{}) string {
		data, _ := json.Marshal(llm.GetModelInfo())
		return string(data)
	})

	log.Message(log.ModuleAgent, "Agent MCP callbacks registered: CreateReminder, ListReminders, DeleteReminder, SendNotification, GenerateReport, SwitchModel, GetCurrentModel")
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

// handleWechatCommand 处理企业微信指令（通过 AI 路由）
func handleWechatCommand(wechatUser, message string) string {
	// 优先使用微信传过来的账户，没有则使用管理员账号
	account := wechatUser
	if account == "" {
		account = globalAccount
	}

	log.MessageF(log.ModuleAgent, "WeChat command from %s (account: %s): %s", wechatUser, account, message)

	// 构建 LLM 请求（注入 system prompt 告知账号，限制回复长度）
	messages := []llm.Message{
		{Role: "system", Content: fmt.Sprintf(
			"你是 Go Blog 智能助手，通过企业微信与用户对话。当前用户账号是 \"%s\"，请直接使用此账号调用工具查询数据，不要询问用户账号。"+
				"重要：回复必须精简，控制在500字以内，只输出关键数据和结论，不要冗余解释。适合手机屏幕阅读。", account)},
		{Role: "user", Content: message},
	}

	result, err := llm.SendSyncLLMRequest(messages, account)
	if err != nil {
		log.WarnF(log.ModuleAgent, "WeChat AI processing failed: %v", err)
		return fmt.Sprintf("⚠️ AI 处理出错: %v", err)
	}

	// 截断过长回复（企业微信应用消息限制 2048 字符）
	if len(result) > 2000 {
		result = result[:2000] + "\n..."
	}

	return result
}

// Shutdown 关闭 Agent 模块
func Shutdown() {
	ShutdownScheduler()
	if globalPool != nil {
		globalPool.Shutdown()
	}
	log.Message(log.ModuleAgent, "Agent module shutdown")
}
