package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// TaskExecutor 任务执行器
type TaskExecutor struct {
	cfg     *Config
	storage *Storage
	conn    *Connection
}

// NewTaskExecutor 创建任务执行器
func NewTaskExecutor(cfg *Config, storage *Storage, conn *Connection) *TaskExecutor {
	return &TaskExecutor{
		cfg:     cfg,
		storage: storage,
		conn:    conn,
	}
}

// Execute 执行任务
func (e *TaskExecutor) Execute(task *CronTask) (string, error) {
	// 根据目标agent类型执行不同的逻辑
	switch task.TargetAgent {
	case "llm-agent":
		return e.executeLLMTask(task)
	default:
		return e.executeGenericTask(task)
	}
}

// executeLLMTask 执行llm-agent任务
func (e *TaskExecutor) executeLLMTask(task *CronTask) (string, error) {
	log.Printf("[TaskExecutor] 发送任务到 llm-agent: taskID=%s type=%s", task.ID, task.TaskType)

	// 如果有UAP连接，则通过UAP发送任务
	if e.conn != nil {
		result, err := e.conn.SendTaskAssignViaUAP(task)
		if err != nil {
			log.Printf("[TaskExecutor] 通过UAP发送任务失败: %v", err)
			// 失败时回退到模拟执行
		} else {
			return result, nil
		}
	}

	// 模拟执行（后备方案）
	log.Printf("[TaskExecutor] 模拟执行 llm-agent 任务: %s", task.TaskType)
	time.Sleep(100 * time.Millisecond)

	result := map[string]interface{}{
		"success": true,
		"task_id": task.ID,
		"message": "任务执行成功（模拟）",
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// executeGenericTask 执行通用agent任务
func (e *TaskExecutor) executeGenericTask(task *CronTask) (string, error) {
	log.Printf("[TaskExecutor] 发送任务到 agent %s: taskID=%s type=%s",
		task.TargetAgent, task.ID, task.TaskType)

	// 构建通用任务负载
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(task.Payload, &payloadMap); err != nil {
		// 如果payload不是对象，则作为原始数据
		payloadMap = map[string]interface{}{
			"data": string(task.Payload),
		}
	}

	_ = map[string]interface{}{
		"type":    task.TaskType,
		"payload": payloadMap,
	}

	// 模拟执行
	log.Printf("[TaskExecutor] 模拟执行 %s 任务: %s", task.TargetAgent, task.TaskType)
	time.Sleep(50 * time.Millisecond)

	result := map[string]interface{}{
		"success":     true,
		"target":      task.TargetAgent,
		"task_id":     task.ID,
		"task_type":   task.TaskType,
		"executed_at": time.Now().Format(time.RFC3339),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// sendNotification 发送通知
func (e *TaskExecutor) sendNotification(task *CronTask, success bool, result, errorMsg string) {
	if !e.cfg.Notifications.Enabled {
		return
	}

	var message string
	if success {
		message = e.cfg.Notifications.SuccessTemplate
	} else {
		message = e.cfg.Notifications.FailureTemplate
	}

	// 构建通知内容
	notification := map[string]interface{}{
		"task_id":   task.ID,
		"task_name": task.Name,
		"success":   success,
		"result":    result,
		"error":     errorMsg,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	_, _ = json.Marshal(notification)

	log.Printf("[TaskExecutor] 发送通知: %s", message)

	// 实际实现中，这里应该通过UAP发送通知消息到通知agent（如wechat-agent）
	// 由于我们还没有完整的UAP连接上下文，这里只记录日志
}

// SendTaskAssignViaUAP 通过UAP发送任务分配消息（需要在Connection中实现）
func (c *Connection) SendTaskAssignViaUAP(task *CronTask) (string, error) {
	// 对发往 llm-agent 的任务，规范化 task_type
	taskType := task.TaskType
	if task.TargetAgent == "llm-agent" || task.TargetAgent == "" {
		taskType = normalizeLLMTaskType(taskType)
	}

	// 构建任务负载
	taskPayload := map[string]interface{}{
		"task_type": taskType,
		"payload":   json.RawMessage(task.Payload),
	}

	// 生成任务ID
	taskID := fmt.Sprintf("cron_%s_%d", task.ID, time.Now().UnixNano())

	// 发送任务分配消息
	timeout := time.Duration(c.cfg.LLMAgent.Timeout) * time.Second
	result, err := c.sendTaskAssign(task.TargetAgent, taskID, taskPayload, timeout)
	if err != nil {
		return "", fmt.Errorf("发送任务失败: %v", err)
	}

	if result.Status != "success" {
		return result.Result, fmt.Errorf("任务执行失败: %s", result.Error)
	}

	return result.Result, nil
}

// normalizeLLMTaskType 将常见 task_type 变体映射为 llm-agent 识别的标准类型
func normalizeLLMTaskType(taskType string) string {
	switch taskType {
	case "cron_reminder":
		return "cron_reminder"
	case "reminder", "notify", "notification", "alert":
		return "cron_reminder"
	default:
		// 未知类型默认作为 cron_reminder 发送
		return "cron_reminder"
	}
}