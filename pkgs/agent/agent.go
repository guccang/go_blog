package agent

import (
	"encoding/json"
	"mcp"
	log "mylog"
	"sync"
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

		// 恢复未完成的任务
		pendingTasks := globalStorage.GetPendingTasks()
		for _, task := range pendingTasks {
			globalPool.Submit(task)
		}

		log.MessageF(log.ModuleAgent, "Agent module initialized, %d pending tasks recovered", len(pendingTasks))
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
func CreateTask(account, title, description string, priority int) *AgentTask {
	task := NewTask(account, title, description, priority)
	if globalPool != nil {
		globalPool.Submit(task)
	}
	return task
}

// GetTask 获取任务
func GetTask(taskID string) *AgentTask {
	if globalPool != nil {
		return globalPool.GetTaskByID(taskID)
	}
	return nil
}

// GetTasks 获取账户的所有任务
func GetTasks(account string) []*AgentTask {
	if globalPool != nil {
		return globalPool.GetAllTasks(account)
	}
	return nil
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
		// 删除关联的提醒
		task := globalStorage.GetTask(taskID)
		if task != nil && task.LinkedReminderID != "" {
			if globalScheduler != nil {
				globalScheduler.RemoveReminder(task.LinkedReminderID)
			}
		}
		// 删除任务
		return globalStorage.DeleteTask(taskID) == nil
	}
	return false
}

// Shutdown 关闭 Agent 模块
func Shutdown() {
	ShutdownScheduler()
	if globalPool != nil {
		globalPool.Shutdown()
	}
	log.Message(log.ModuleAgent, "Agent module shutdown")
}
