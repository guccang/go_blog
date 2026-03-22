package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"uap"
)

// ========================= 工具处理函数 =========================

func (c *Connection) handleToolCreateTask(args map[string]interface{}) (string, bool) {
	// 解析参数
	name, _ := args["name"].(string)
	scheduleTypeStr, _ := args["schedule_type"].(string)
	cronExpr, _ := args["cron_expr"].(string)
	intervalSec, _ := args["interval_sec"].(float64)
	delaySec, _ := args["delay_sec"].(float64)
	runAt, _ := args["run_at"].(string)
	targetAgent, _ := args["target_agent"].(string)
	taskType, _ := args["task_type"].(string)
	enabled := true
	if val, ok := args["enabled"].(bool); ok {
		enabled = val
	}

	// 验证参数
	if name == "" {
		return errorResponse("任务名称不能为空"), false
	}
	if taskType == "" {
		return errorResponse("任务类型不能为空"), false
	}

	// 解析payload
	payloadJSON, err := json.Marshal(args["payload"])
	if err != nil {
		return errorResponse(fmt.Sprintf("无效的任务负载: %v", err)), false
	}

	scheduleType := ScheduleType(scheduleTypeStr)
	if scheduleType != ScheduleTypeCron && scheduleType != ScheduleTypeOnce && scheduleType != ScheduleTypeInterval {
		return errorResponse("无效的调度类型，必须是 cron/once/interval"), false
	}

	// 验证调度参数
	if scheduleType == ScheduleTypeCron && cronExpr == "" {
		return errorResponse("cron表达式不能为空"), false
	}
	if scheduleType == ScheduleTypeInterval && intervalSec <= 0 {
		return errorResponse("间隔秒数必须大于0"), false
	}

	// 创建任务请求
	req := &TaskCreateRequest{
		Name:         name,
		ScheduleType: scheduleType,
		CronExpr:     cronExpr,
		IntervalSec:  int64(intervalSec),
		DelaySec:     int64(delaySec),
		RunAt:        runAt,
		TargetAgent:  targetAgent,
		TaskType:     taskType,
		Payload:      json.RawMessage(payloadJSON),
		Enabled:      enabled,
	}

	// 保存任务
	task, err := c.storage.CreateTask(req)
	if err != nil {
		return errorResponse(fmt.Sprintf("创建任务失败: %v", err)), false
	}

	// 注册到调度器
	if err := c.scheduler.AddTask(task); err != nil {
		// 如果调度失败，删除任务
		c.storage.DeleteTask(task.ID)
		return errorResponse(fmt.Sprintf("调度任务失败: %v", err)), false
	}

	log.Printf("[CornAgent] 任务创建成功 ID=%s name=%s", task.ID, task.Name)

	response := map[string]interface{}{
		"success": true,
		"task_id": task.ID,
		"task":    task,
	}
	return jsonResponse(response), true
}

func (c *Connection) handleToolListTasks(args map[string]interface{}) (string, bool) {
	tasks, err := c.storage.ListTasks()
	if err != nil {
		return errorResponse(fmt.Sprintf("获取任务列表失败: %v", err)), false
	}

	response := map[string]interface{}{
		"success": true,
		"tasks":   tasks,
		"total":   len(tasks),
	}
	return jsonResponse(response), true
}

func (c *Connection) handleToolGetTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return errorResponse("任务ID不能为空"), false
	}

	task, err := c.storage.GetTask(taskID)
	if err != nil {
		return errorResponse(fmt.Sprintf("获取任务失败: %v", err)), false
	}

	response := map[string]interface{}{
		"success": true,
		"task":    task,
	}
	return jsonResponse(response), true
}

func (c *Connection) handleToolUpdateTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return errorResponse("任务ID不能为空"), false
	}

	// 构建更新请求
	req := &TaskUpdateRequest{}

	if val, ok := args["name"].(string); ok && val != "" {
		req.Name = &val
	}
	if val, ok := args["schedule_type"].(string); ok && val != "" {
		st := ScheduleType(val)
		req.ScheduleType = &st
	}
	if val, ok := args["cron_expr"].(string); ok && val != "" {
		req.CronExpr = &val
	}
	if val, ok := args["interval_sec"].(float64); ok && val > 0 {
		interval := int64(val)
		req.IntervalSec = &interval
	}
	if val, ok := args["delay_sec"].(float64); ok && val > 0 {
		delay := int64(val)
		req.DelaySec = &delay
	}
	if val, ok := args["run_at"].(string); ok && val != "" {
		req.RunAt = &val
	}
	if val, ok := args["target_agent"].(string); ok && val != "" {
		req.TargetAgent = &val
	}
	if val, ok := args["task_type"].(string); ok && val != "" {
		req.TaskType = &val
	}
	if val, ok := args["enabled"].(bool); ok {
		req.Enabled = &val
	}

	// 处理payload
	if payload, ok := args["payload"]; ok && payload != nil {
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return errorResponse(fmt.Sprintf("无效的任务负载: %v", err)), false
		}
		rawPayload := json.RawMessage(payloadJSON)
		req.Payload = &rawPayload
	}

	// 更新任务
	task, err := c.storage.UpdateTask(taskID, req)
	if err != nil {
		return errorResponse(fmt.Sprintf("更新任务失败: %v", err)), false
	}

	// 更新调度器中的任务
	if err := c.scheduler.UpdateTask(task); err != nil {
		return errorResponse(fmt.Sprintf("更新调度器失败: %v", err)), false
	}

	log.Printf("[CornAgent] 任务更新成功 ID=%s", taskID)

	response := map[string]interface{}{
		"success": true,
		"task":    task,
	}
	return jsonResponse(response), true
}

func (c *Connection) handleToolDeleteTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return errorResponse("任务ID不能为空"), false
	}

	// 从调度器移除
	if err := c.scheduler.RemoveTask(taskID); err != nil {
		log.Printf("[CornAgent] 从调度器移除任务失败: %v", err)
	}

	// 从存储删除
	if err := c.storage.DeleteTask(taskID); err != nil {
		return errorResponse(fmt.Sprintf("删除任务失败: %v", err)), false
	}

	log.Printf("[CornAgent] 任务删除成功 ID=%s", taskID)

	response := map[string]interface{}{
		"success": true,
		"message": "任务删除成功",
	}
	return jsonResponse(response), true
}

func (c *Connection) handleToolTriggerTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return errorResponse("任务ID不能为空"), false
	}

	// 触发任务
	if err := c.scheduler.TriggerTask(taskID); err != nil {
		return errorResponse(fmt.Sprintf("触发任务失败: %v", err)), false
	}

	log.Printf("[CornAgent] 任务触发成功 ID=%s", taskID)

	response := map[string]interface{}{
		"success": true,
		"message": "任务已触发",
	}
	return jsonResponse(response), true
}

// ========================= corn.task.* 消息处理函数 =========================

func (c *Connection) processTaskCreate(msg *uap.Message) {
	var req TaskCreateRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendErrorResponse(msg, "无效的请求负载")
		return
	}

	// 创建任务
	task, err := c.storage.CreateTask(&req)
	if err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("创建任务失败: %v", err))
		return
	}

	// 注册到调度器
	if err := c.scheduler.AddTask(task); err != nil {
		c.storage.DeleteTask(task.ID)
		c.sendErrorResponse(msg, fmt.Sprintf("调度任务失败: %v", err))
		return
	}

	// 发送成功响应
	response := map[string]interface{}{
		"success": true,
		"task_id": task.ID,
		"task":    task,
	}
	c.sendResponse(msg, "corn.task.create.response", response)
}

func (c *Connection) processTaskUpdate(msg *uap.Message) {
	var req struct {
		TaskID string `json:"task_id"`
		TaskUpdateRequest
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendErrorResponse(msg, "无效的请求负载")
		return
	}

	if req.TaskID == "" {
		c.sendErrorResponse(msg, "任务ID不能为空")
		return
	}

	// 更新任务
	task, err := c.storage.UpdateTask(req.TaskID, &req.TaskUpdateRequest)
	if err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("更新任务失败: %v", err))
		return
	}

	// 更新调度器中的任务
	if err := c.scheduler.UpdateTask(task); err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("更新调度器失败: %v", err))
		return
	}

	// 发送成功响应
	response := map[string]interface{}{
		"success": true,
		"task":    task,
	}
	c.sendResponse(msg, "corn.task.update.response", response)
}

func (c *Connection) processTaskDelete(msg *uap.Message) {
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendErrorResponse(msg, "无效的请求负载")
		return
	}

	if req.TaskID == "" {
		c.sendErrorResponse(msg, "任务ID不能为空")
		return
	}

	// 从调度器移除
	if err := c.scheduler.RemoveTask(req.TaskID); err != nil {
		log.Printf("[CornAgent] 从调度器移除任务失败: %v", err)
	}

	// 从存储删除
	if err := c.storage.DeleteTask(req.TaskID); err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("删除任务失败: %v", err))
		return
	}

	// 发送成功响应
	response := map[string]interface{}{
		"success": true,
		"message": "任务删除成功",
	}
	c.sendResponse(msg, "corn.task.delete.response", response)
}

func (c *Connection) processTaskList(msg *uap.Message) {
	tasks, err := c.storage.ListTasks()
	if err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("获取任务列表失败: %v", err))
		return
	}

	// 发送响应
	response := map[string]interface{}{
		"success": true,
		"tasks":   tasks,
		"total":   len(tasks),
	}
	c.sendResponse(msg, "corn.task.list.response", response)
}

func (c *Connection) processTaskStatus(msg *uap.Message) {
	var req struct {
		TaskID string `json:"task_id"`
		IncludeExecutions bool `json:"include_executions"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendErrorResponse(msg, "无效的请求负载")
		return
	}

	if req.TaskID == "" {
		c.sendErrorResponse(msg, "任务ID不能为空")
		return
	}

	// 获取任务
	task, err := c.storage.GetTask(req.TaskID)
	if err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("获取任务失败: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"task":    task,
	}

	// 如果需要执行记录
	if req.IncludeExecutions {
		executions, err := c.storage.GetTaskExecutions(req.TaskID, 10)
		if err != nil {
			log.Printf("[CornAgent] 获取执行记录失败: %v", err)
		} else {
			response["executions"] = executions
		}
	}

	c.sendResponse(msg, "corn.task.status.response", response)
}

func (c *Connection) processTaskTrigger(msg *uap.Message) {
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendErrorResponse(msg, "无效的请求负载")
		return
	}

	if req.TaskID == "" {
		c.sendErrorResponse(msg, "任务ID不能为空")
		return
	}

	// 触发任务
	if err := c.scheduler.TriggerTask(req.TaskID); err != nil {
		c.sendErrorResponse(msg, fmt.Sprintf("触发任务失败: %v", err))
		return
	}

	// 发送成功响应
	response := map[string]interface{}{
		"success": true,
		"message": "任务已触发",
	}
	c.sendResponse(msg, "corn.task.trigger.response", response)
}

// ========================= 辅助函数 =========================

func (c *Connection) sendResponse(originalMsg *uap.Message, responseType string, payload interface{}) {
	responseMsg := &uap.Message{
		Type: responseType,
		ID:   uap.NewMsgID(),
		From: c.AgentID,
		To:   originalMsg.From,
		Ts:   time.Now().UnixMilli(),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[CornAgent] 序列化响应失败: %v", err)
		return
	}
	responseMsg.Payload = payloadJSON

	if err := c.Client.Send(responseMsg); err != nil {
		log.Printf("[CornAgent] 发送响应失败: %v", err)
	}
}

func (c *Connection) sendErrorResponse(originalMsg *uap.Message, errorMsg string) {
	response := map[string]interface{}{
		"success": false,
		"error":   errorMsg,
	}
	c.sendResponse(originalMsg, originalMsg.Type+".response", response)
}

func jsonResponse(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return `{"success":false,"error":"序列化响应失败"}`
	}
	return string(jsonData)
}

func errorResponse(errorMsg string) string {
	return jsonResponse(map[string]interface{}{
		"success": false,
		"error":   errorMsg,
	})
}