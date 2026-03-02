package agent

import (
	"codegen"
	"email"
	"encoding/json"
	"fmt"
	"llm"
	"mcp"
	log "mylog"
	"strings"
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

		// 初始化编码助手模块
		codegen.Init()

		// 初始化 CodeGen 微信桥接
		codegen.InitWeChatBridge(func(toUser, content string) error {
			return wechat.SendAppMessage(toUser, content)
		})

		// 初始化报告生成器
		InitReportGenerator(account)

		// 注册 MCP 回调
		registerMCPCallbacks()

		// 恢复未完成的任务（仅加载，不自动执行）
		pendingGraphs := globalStorage.GetPendingTaskGraphs()

		// 启动 graphCache 定期清理（每 30 分钟清理 7 天前已完成的图，缓存上限 200）
		go func() {
			ticker := time.NewTicker(30 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				GraphCacheCleanup(7*24*time.Hour, 200)
			}
		}()
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

	// ============================================================================
	// CodeGen 编码助手工具
	// ============================================================================

	// 列出所有编码项目
	mcp.RegisterCallBack("CodegenListProjects", func(args map[string]interface{}) string {
		return codegen.ListProjectsJSON()
	})

	// 创建新编码项目（支持本地或指定 agent）
	mcp.RegisterCallBack("CodegenCreateProject", func(args map[string]interface{}) string {
		name, _ := args["name"].(string)
		if name == "" {
			return `{"success":false,"error":"缺少项目名称"}`
		}
		agentName, _ := args["agent"].(string)
		if agentName != "" {
			pool := codegen.GetAgentPool()
			if pool == nil {
				return `{"success":false,"error":"远程 agent 模式未启用"}`
			}
			if err := pool.CreateRemoteProject(agentName, name); err != nil {
				return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
			}
			return fmt.Sprintf(`{"success":true,"message":"项目 %s 已在 agent %s 上创建"}`, name, agentName)
		}
		return codegen.CreateProjectJSON(name)
	})

	// 启动 AI 编码会话（异步，后台推送进度）
	mcp.RegisterCallBack("CodegenStartSession", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		project, _ := args["project"].(string)
		prompt, _ := args["prompt"].(string)
		model, _ := args["model"].(string)
		tool, _ := args["tool"].(string)
		if project == "" || prompt == "" {
			return `{"success":false,"error":"缺少 project 或 prompt 参数"}`
		}
		sessionID, err := codegen.StartSessionForWeChat(account, project, prompt, model, tool, "")
		if err != nil {
			return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
		}
		return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"编码会话已启动，进度将通过微信推送"}`, sessionID)
	})

	// 向活跃编码会话追加消息
	mcp.RegisterCallBack("CodegenSendMessage", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		prompt, _ := args["prompt"].(string)
		if prompt == "" {
			return `{"success":false,"error":"缺少 prompt 参数"}`
		}
		sessionID, err := codegen.SendMessageForWeChat(account, prompt)
		if err != nil {
			return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
		}
		return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"消息已发送，后续进度将通过微信推送"}`, sessionID)
	})

	// 查看编码会话运行状态
	mcp.RegisterCallBack("CodegenGetStatus", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		status := codegen.GetStatusForWeChat(account)
		return fmt.Sprintf(`{"success":true,"status":"%s"}`, status)
	})

	// 停止当前编码会话
	mcp.RegisterCallBack("CodegenStopSession", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		sessionID, err := codegen.StopSessionForWeChat(account)
		if err != nil {
			return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
		}
		return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"编码会话已停止"}`, sessionID)
	})

	// 列出所有支持部署的项目
	mcp.RegisterCallBack("CodegenListDeployProjects", func(args map[string]interface{}) string {
		return codegen.ListDeployProjectsJSON()
	})

	// 启动部署会话（跳过编码，直接部署）
	mcp.RegisterCallBack("CodegenStartDeploy", func(args map[string]interface{}) string {
		account, _ := args["account"].(string)
		if account == "" {
			account = globalAccount
		}
		project, _ := args["project"].(string)
		if project == "" {
			return `{"success":false,"error":"缺少 project 参数"}`
		}
		return codegen.StartDeployJSON(account, project)
	})

	log.Message(log.ModuleAgent, "Agent MCP callbacks registered: CreateReminder, ListReminders, DeleteReminder, SendNotification, GenerateReport, SwitchModel, GetCurrentModel, CodegenListProjects, CodegenCreateProject, CodegenStartSession, CodegenSendMessage, CodegenGetStatus, CodegenStopSession, CodegenListDeployProjects, CodegenStartDeploy")
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

// toolNameMap 工具名称中文映射
var toolNameMap = map[string]string{
	"ListReminders":   "查询定时任务列表",
	"CreateReminder":  "创建定时任务",
	"DeleteReminder":  "删除定时任务",
	"SendNotification": "发送通知",
	"GenerateReport":  "生成报告",
	"SwitchModel":     "切换模型",
	"GetCurrentModel": "获取当前模型",
}

// getToolDisplayName 获取工具的中文显示名称
func getToolDisplayName(toolName string) string {
	if name, ok := toolNameMap[toolName]; ok {
		return name
	}
	return toolName
}

// handleWechatCommand 处理企业微信指令（通过 AI 路由）
func handleWechatCommand(wechatUser, message string) string {
	// 优先使用微信传过来的账户，没有则使用管理员账号
	account := wechatUser
	if account == "" {
		account = globalAccount
	}

	log.MessageF(log.ModuleAgent, "WeChat command from %s (account: %s): %s", wechatUser, account, message)

	// 方案A：拦截 cg 命令，直接处理，不经过 LLM
	if strings.HasPrefix(message, "cg ") || message == "cg" {
		return handleCodegenCommand(account, message)
	}

	// 发送即时确认
	wechat.SendAppMessage(wechatUser, "⏳ 收到指令，正在处理...")

	// 构建 LLM 请求（注入 system prompt 告知账号，限制回复长度）
	messages := []llm.Message{
		{Role: "system", Content: fmt.Sprintf(
			"你是 Go Blog 智能助手，通过企业微信与用户对话。当前用户账号是 \"%s\"，请直接使用此账号调用工具查询数据，不要询问用户账号。"+
				"重要：回复必须精简，控制在500字以内，只输出关键数据和结论，不要冗余解释。适合手机屏幕阅读。", account)},
		{Role: "user", Content: message},
	}

	// 进度回调：只在 tool_call 事件时发送进度消息
	progressCallback := func(eventType string, detail string) {
		if eventType == "tool_call" {
			displayName := getToolDisplayName(detail)
			wechat.SendAppMessage(wechatUser, fmt.Sprintf("🔧 正在执行: %s...", displayName))
		}
	}

	result, err := llm.SendSyncLLMRequestWithProgress(messages, account, progressCallback)
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

// parseProjectAgent 从 "myapp@win" 解析出 (project, agentName)
// 从 "myapp" 解析出 ("myapp", "")
func parseProjectAgent(s string) (project, agentName string) {
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

// resolveAgentID 根据 project 和 agentName 解析出目标 agentID
// agentName 非空：通过 FindAgentByName 查找
// agentName 为空：遍历远程项目，若只有一个 agent 持有该项目则自动使用，多个则报错
func resolveAgentID(project, agentName string) (string, error) {
	pool := codegen.GetAgentPool()
	if pool == nil {
		if agentName != "" {
			return "", fmt.Errorf("远程 agent 模式未启用")
		}
		return "", nil // 本地模式
	}

	if agentName != "" {
		agent := pool.FindAgentByName(agentName)
		if agent == nil {
			return "", fmt.Errorf("未找到在线 agent: %s", agentName)
		}
		return agent.ID, nil
	}

	// agentName 为空：检查远程项目中是否有同名项目
	remoteProjects := pool.ListRemoteProjects()
	var matched []codegen.RemoteProjectInfo
	for _, p := range remoteProjects {
		if p.Name == project {
			matched = append(matched, p)
		}
	}
	if len(matched) == 1 {
		return matched[0].AgentID, nil
	}
	if len(matched) > 1 {
		var agents []string
		for _, m := range matched {
			agents = append(agents, m.Agent)
		}
		return "", fmt.Errorf("多个 agent 都有项目 %s，请用 %s@<agent> 指定\n可选: %s",
			project, project, strings.Join(agents, ", "))
	}

	return "", nil // 没有远程匹配，走本地
}

// handleCodegenCommand 处理 cg 快捷命令（方案A：确定性命令，不经过 LLM）
func handleCodegenCommand(userID, message string) string {
	// 去掉 "cg " 前缀，解析子命令
	args := strings.TrimPrefix(message, "cg")
	args = strings.TrimSpace(args)

	if args == "" {
		return getCodegenHelpText()
	}

	parts := strings.SplitN(args, " ", 2)
	subCmd := parts[0]
	var param string
	if len(parts) > 1 {
		param = strings.TrimSpace(parts[1])
	}

	switch subCmd {
	case "help", "h":
		return getCodegenHelpText()

	case "list", "ls":
		var sb strings.Builder

		// 本地项目
		projects, err := codegen.ListProjects()
		if err != nil {
			return fmt.Sprintf("❌ %v", err)
		}

		// 远程 agent 项目
		var remoteProjects []codegen.RemoteProjectInfo
		pool := codegen.GetAgentPool()
		if pool != nil {
			remoteProjects = pool.ListRemoteProjects()
		}

		totalCount := len(projects) + len(remoteProjects)
		if totalCount == 0 {
			return fmt.Sprintf("📂 暂无编码项目\n工作区: %s\n\n使用 cg create <名称> 创建项目", codegen.GetWorkspace())
		}

		sb.WriteString(fmt.Sprintf("📂 编码项目 (%d个)\n\n", totalCount))

		if len(projects) > 0 {
			sb.WriteString(fmt.Sprintf("**本地** [%s]\n", codegen.GetWorkspace()))
			for i, p := range projects {
				sb.WriteString(fmt.Sprintf("%d. %s — %d文件 (%s)\n", i+1, p.Name, p.FileCount, p.ModTime))
			}
		}

		if len(remoteProjects) > 0 {
			if len(projects) > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("**远程Agent**\n")
			for i, p := range remoteProjects {
				sb.WriteString(fmt.Sprintf("%d. %s@%s\n", len(projects)+i+1, p.Name, p.Agent))
			}
		}

		return sb.String()

	case "create", "new":
		if param == "" {
			return "⚠️ 请指定项目名称\n用法: cg create <名称[@agent]>\n远程: cg create <名称>@<agent名>"
		}
		parts := strings.Fields(param)
		projectName, agentTarget := parseProjectAgent(parts[0])

		// 兼容旧语法: cg create myapp @agent
		if agentTarget == "" {
			for _, p := range parts[1:] {
				if strings.HasPrefix(p, "@") {
					agentTarget = strings.TrimPrefix(p, "@")
				}
			}
		}

		if agentTarget != "" {
			// 在远程 agent 上创建
			pool := codegen.GetAgentPool()
			if pool == nil {
				return "❌ 远程 agent 模式未启用"
			}
			if err := pool.CreateRemoteProject(agentTarget, projectName); err != nil {
				return fmt.Sprintf("❌ 远程创建失败: %v", err)
			}
			return fmt.Sprintf("✅ 项目 **%s** 已在 agent **%s** 上创建", projectName, agentTarget)
		}

		// 本地创建
		if err := codegen.CreateProject(projectName); err != nil {
			return fmt.Sprintf("❌ 创建失败: %v", err)
		}
		return fmt.Sprintf("✅ 项目 **%s** 创建成功（本地）", projectName)

	case "start", "run":
		// cg start <project[@agent]> [#model] [@tool] [!deploy] <prompt>
		if param == "" {
			return "⚠️ 请指定项目和需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>\n示例: cg start myapp #sonnet 写个HTTP服务\n示例: cg start myapp@win 用指定agent编码\n示例: cg start myapp !deploy 增加健康检查接口"
		}
		startParts := strings.SplitN(param, " ", 2)
		project, agentName := parseProjectAgent(startParts[0])
		rest := ""
		if len(startParts) > 1 {
			rest = strings.TrimSpace(startParts[1])
		}
		if rest == "" {
			return "⚠️ 请提供编码需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>"
		}
		// 解析可选的 #model、@tool、!deploy（顺序不限）
		model := ""
		tool := ""
		autoDeploy := false
		for strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "@") || strings.HasPrefix(rest, "!") {
			optParts := strings.SplitN(rest, " ", 2)
			opt := optParts[0]
			if strings.HasPrefix(opt, "#") {
				model = strings.TrimPrefix(opt, "#")
			} else if strings.HasPrefix(opt, "@") {
				toolAlias := strings.TrimPrefix(opt, "@")
				tool = codegen.NormalizeTool(toolAlias)
			} else if strings.EqualFold(opt, "!deploy") {
				autoDeploy = true
			}
			if len(optParts) > 1 {
				rest = strings.TrimSpace(optParts[1])
			} else {
				rest = ""
				break
			}
		}
		if rest == "" {
			return "⚠️ 请提供编码需求\n用法: cg start <项目[@agent]> [#模型] [@工具] [!deploy] <编码需求>"
		}
		agentID, err := resolveAgentID(project, agentName)
		if err != nil {
			return fmt.Sprintf("❌ %v", err)
		}
		sessionID, err := codegen.StartSessionForWeChat(userID, project, rest, model, tool, agentID, autoDeploy)
		if err != nil {
			return fmt.Sprintf("❌ 启动失败: %v", err)
		}
		modelInfo := ""
		if model != "" {
			modelInfo = fmt.Sprintf("\n模型: %s", model)
		}
		toolInfo := ""
		if tool != "" && tool != "claudecode" {
			toolInfo = fmt.Sprintf("\n工具: %s", tool)
		}
		deployInfo := ""
		if autoDeploy {
			deployInfo = "\n部署: 编码完成后自动部署"
		}
		agentInfo := ""
		if agentName != "" {
			agentInfo = fmt.Sprintf("\nAgent: %s", agentName)
		}
		return fmt.Sprintf("🚀 编码会话已启动\n\n项目: %s%s%s%s%s\n会话: %s\n\n进度将通过微信推送", project, agentInfo, modelInfo, toolInfo, deployInfo, sessionID)

	case "deploy", "dp":
		// cg deploy <project[@agent]> — 仅部署，不编码
		if param == "" {
			return "⚠️ 请指定项目名称\n用法: cg deploy <项目[@agent]>\n示例: cg deploy myapp\n示例: cg deploy myapp@mac"
		}
		project, agentName := parseProjectAgent(strings.Fields(param)[0])
		agentID, err := resolveAgentID(project, agentName)
		if err != nil {
			return fmt.Sprintf("❌ %v", err)
		}
		sessionID, err := codegen.StartSessionForWeChat(userID, project, "", "", "", agentID, false, true)
		if err != nil {
			return fmt.Sprintf("❌ 部署启动失败: %v", err)
		}
		agentInfo := ""
		if agentName != "" {
			agentInfo = fmt.Sprintf(" (agent: %s)", agentName)
		}
		return fmt.Sprintf("🚀 部署已启动\n\n项目: %s%s\n会话: %s\n\n进度将通过微信推送", project, agentInfo, sessionID)

	case "send", "msg":
		// cg send <prompt>
		if param == "" {
			return "⚠️ 请提供消息内容\n用法: cg send <消息>"
		}
		sessionID, err := codegen.SendMessageForWeChat(userID, param)
		if err != nil {
			return fmt.Sprintf("❌ 发送失败: %v", err)
		}
		return fmt.Sprintf("📨 消息已发送到会话 %s", sessionID)

	case "status", "st":
		return codegen.GetStatusForWeChat(userID)

	case "stop":
		sessionID, err := codegen.StopSessionForWeChat(userID)
		if err != nil {
			return fmt.Sprintf("❌ 停止失败: %v", err)
		}
		return fmt.Sprintf("⏹ 编码会话 %s 已停止", sessionID)

	case "agents":
		pool := codegen.GetAgentPool()
		if pool == nil {
			return "远程 agent 模式未启用"
		}
		agents := pool.GetAgents()
		if len(agents) == 0 {
			return "当前无在线 agent"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🖥 在线 Agent (%d个)\n\n", len(agents)))
		for i, a := range agents {
			name, _ := a["name"].(string)
			status, _ := a["status"].(string)
			active, _ := a["active_sessions"].(int)
			projects, _ := a["projects"].([]string)
			sb.WriteString(fmt.Sprintf("%d. **%s** [%s] 活跃:%d 项目:%d\n",
				i+1, name, status, active, len(projects)))
		}
		return sb.String()

	case "models":
		pool := codegen.GetAgentPool()
		if pool == nil {
			return "远程 agent 模式未启用"
		}
		models := pool.GetAllModels()
		if len(models) == 0 {
			return "当前无可用模型配置\n\n在 agent 的 settings/ 目录下放置 .json 配置文件即可"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🤖 可用模型配置 (%d个)\n\n", len(models)))
		for i, m := range models {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, m))
		}
		sb.WriteString("\n用法: cg start <项目> #模型名 <需求>")
		return sb.String()

	case "tools":
		pool := codegen.GetAgentPool()
		if pool == nil {
			return "远程 agent 模式未启用"
		}
		tools := pool.GetAllTools()
		if len(tools) == 0 {
			return "当前无可用编码工具"
		}
		toolLabels := map[string]string{
			"claudecode": "Claude Code (默认)",
			"opencode":   "OpenCode",
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🔧 可用编码工具 (%d个)\n\n", len(tools)))
		for i, t := range tools {
			label := toolLabels[t]
			if label == "" {
				label = t
			}
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, label))
		}
		sb.WriteString("\n用法: cg start <项目> @oc <需求>")
		sb.WriteString("\n别名: @oc/@opencode=OpenCode, @cc/@claude=ClaudeCode")
		return sb.String()

	default:
		return fmt.Sprintf("⚠️ 未知命令: cg %s\n\n%s", subCmd, getCodegenHelpText())
	}
}

// getCodegenHelpText 返回 cg 命令帮助
func getCodegenHelpText() string {
	return "💻 CodeGen 编码助手命令\n\n" +
		"cg list — 列出所有项目（本地+远程）\n" +
		"cg create <名称[@agent]> — 创建项目\n" +
		"cg start <项目[@agent]> <需求> — 启动编码\n" +
		"cg start <项目[@agent]> #<模型> <需求> — 指定模型\n" +
		"cg start <项目[@agent]> @oc <需求> — 用OpenCode\n" +
		"cg start <项目[@agent]> !deploy <需求> — 编码后自动部署\n" +
		"cg deploy <项目[@agent]> — 仅部署（不编码）\n" +
		"cg send <消息> — 追加指令\n" +
		"cg status — 查看进度\n" +
		"cg stop — 停止编码\n" +
		"cg models — 查看可用模型配置\n" +
		"cg tools — 查看可用编码工具\n" +
		"cg agents — 查看在线agent\n\n" +
		"@agent 语法: 多agent同名项目时用 项目@agent 指定目标\n" +
		"工具别名: @oc/@opencode=OpenCode, @cc/@claude=ClaudeCode\n" +
		"示例: cg start myapp@win #sonnet !deploy 写个HTTP服务"
}

// Shutdown 关闭 Agent 模块
func Shutdown() {
	ShutdownScheduler()
	if globalPool != nil {
		globalPool.Shutdown()
	}
	log.Message(log.ModuleAgent, "Agent module shutdown")
}
