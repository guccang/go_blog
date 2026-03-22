package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"agentbase"
	"uap"
)

// Connection corn-agent UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg      *Config
	scheduler *Scheduler
	storage  *Storage

	// 请求-响应关联
	pending     map[string]chan *uap.ToolResultPayload
	pendMu      sync.Mutex
	taskResults map[string]chan *uap.TaskCompletePayload
	taskMu      sync.Mutex
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	// 初始化存储
	storage := NewStorage(cfg)

	// 初始化调度器
	scheduler := NewScheduler(cfg, storage)

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.GatewayURL,
		AgentID:     agentID,
		AgentType:   "corn_agent",
		AgentName:   "corn-agent",
		Description: "定时任务调度器：基于cron表达式的任务调度，支持一次性、周期性任务，与llm-agent集成",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.Scheduler.MaxConcurrent,
		Tools:       buildToolDefs(),
		Meta: map[string]any{
			"version": "1.0.0",
		},
	}

	c := &Connection{
		AgentBase:   agentbase.NewAgentBase(baseCfg),
		cfg:         cfg,
		scheduler:   scheduler,
		storage:     storage,
		pending:     make(map[string]chan *uap.ToolResultPayload),
		taskResults: make(map[string]chan *uap.TaskCompletePayload),
	}
	// 设置调度器的UAP连接，用于任务执行
	scheduler.SetConnection(c)

	// 注册消息处理器
	c.RegisterHandler(uap.MsgToolCall, c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgToolResult, c.handleToolResult)
	c.RegisterHandler(uap.MsgTaskComplete, c.handleTaskComplete)
	c.RegisterHandler(uap.MsgTaskEvent, c.handleTaskEvent)
	c.RegisterHandler(uap.MsgError, c.handleError)

	// 注册corn.task.*消息处理器
	c.RegisterHandler("corn.task.create", c.handleTaskCreate)
	c.RegisterHandler("corn.task.update", c.handleTaskUpdate)
	c.RegisterHandler("corn.task.delete", c.handleTaskDelete)
	c.RegisterHandler("corn.task.list", c.handleTaskList)
	c.RegisterHandler("corn.task.status", c.handleTaskStatus)
	c.RegisterHandler("corn.task.trigger", c.handleTaskTrigger)

	// 启动时加载任务
	c.loadTasks()

	// 启动调度器
	c.scheduler.Start()

	return c
}

// loadTasks 从存储加载任务并注册到调度器
func (c *Connection) loadTasks() {
	tasks, err := c.storage.LoadTasks()
	if err != nil {
		log.Printf("[CornAgent] 加载任务失败: %v", err)
		return
	}

	enabledCount := 0
	for _, task := range tasks {
		if task.Enabled {
			if err := c.scheduler.AddTask(task); err != nil {
				log.Printf("[CornAgent] 注册任务失败 ID=%s: %v", task.ID, err)
			} else {
				enabledCount++
			}
		}
	}

	log.Printf("[CornAgent] 加载了 %d 个任务 (%d 个已启用)", len(tasks), enabledCount)
}

// ========================= 工具注册 =========================

func buildToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "CornCreateTask",
			Description: "创建定时任务。支持三种调度类型：cron（cron表达式周期任务）、once（一次性延迟任务，用delay_sec指定延迟秒数或run_at指定绝对时间）、interval（固定间隔重复任务）。例如：'5分钟后提醒我喝水'应使用 schedule_type=once, delay_sec=300。\n\n提醒类任务规则：当用户请求定时提醒时，task_type 必须填 \"cron_reminder\"，payload 必须包含 message（提醒内容）、account（用户账号，取系统prompt中的account值）、wechat_user（微信用户标识）。示例：task_type=\"cron_reminder\", payload={\"message\":\"喝水\",\"account\":\"xxx\",\"wechat_user\":\"wxid_xxx\"}。",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":          map[string]interface{}{"type": "string", "description": "任务名称"},
					"schedule_type": map[string]interface{}{"type": "string", "description": "调度类型: cron（周期cron表达式）/ once（一次性延迟）/ interval（固定间隔重复）", "enum": []string{"cron", "once", "interval"}},
					"cron_expr":     map[string]interface{}{"type": "string", "description": "cron表达式（schedule_type=cron时必填，如 '0 30 9 * * *' 表示每天9:30）"},
					"interval_sec":  map[string]interface{}{"type": "integer", "description": "间隔秒数（schedule_type=interval时必填）"},
					"delay_sec":     map[string]interface{}{"type": "integer", "description": "延迟秒数（schedule_type=once时使用，如300表示5分钟后执行）"},
					"run_at":        map[string]interface{}{"type": "string", "description": "执行时间，ISO8601格式（schedule_type=once时使用，如 '2025-01-01T09:00:00+08:00'）"},
					"target_agent":  map[string]interface{}{"type": "string", "description": "目标agent ID，默认llm-agent"},
					"task_type":     map[string]interface{}{"type": "string", "description": "任务类型（llm-agent任务类型）"},
					"payload":       map[string]interface{}{"type": "object", "description": "任务负载数据"},
					"enabled":       map[string]interface{}{"type": "boolean", "description": "是否启用，默认true"},
				},
				"required": []string{"name", "schedule_type", "task_type", "payload"},
			}),
		},
		{
			Name:        "CornListTasks",
			Description: "列出所有定时任务",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			}),
		},
		{
			Name:        "CornGetTask",
			Description: "获取任务详情",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		},
		{
			Name:        "CornUpdateTask",
			Description: "更新定时任务",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id":       map[string]interface{}{"type": "string", "description": "任务ID"},
					"name":          map[string]interface{}{"type": "string", "description": "任务名称"},
					"schedule_type": map[string]interface{}{"type": "string", "description": "调度类型: cron/once/interval", "enum": []string{"cron", "once", "interval"}},
					"cron_expr":     map[string]interface{}{"type": "string", "description": "cron表达式"},
					"interval_sec":  map[string]interface{}{"type": "integer", "description": "间隔秒数"},
					"delay_sec":     map[string]interface{}{"type": "integer", "description": "延迟秒数（schedule_type=once时）"},
					"run_at":        map[string]interface{}{"type": "string", "description": "执行时间ISO8601（schedule_type=once时）"},
					"target_agent":  map[string]interface{}{"type": "string", "description": "目标agent ID"},
					"task_type":     map[string]interface{}{"type": "string", "description": "任务类型"},
					"payload":       map[string]interface{}{"type": "object", "description": "任务负载数据"},
					"enabled":       map[string]interface{}{"type": "boolean", "description": "是否启用"},
				},
				"required": []string{"task_id"},
			}),
		},
		{
			Name:        "CornDeleteTask",
			Description: "删除定时任务",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		},
		{
			Name:        "CornTriggerTask",
			Description: "手动触发任务",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string", "description": "任务ID"},
				},
				"required": []string{"task_id"},
			}),
		},
	}
}

// ========================= 消息处理 =========================

func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	go c.handleToolCall(msg)
}

func (c *Connection) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CornAgent] invalid tool_call payload: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid tool_call payload",
		})
		return
	}

	var args map[string]interface{}
	if err := json.Unmarshal(payload.Arguments, &args); err != nil {
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid arguments: " + err.Error(),
		})
		return
	}

	log.Printf("[CornAgent] tool_call from=%s tool=%s", msg.From, payload.ToolName)

	var resultJSON string
	var success bool

	switch payload.ToolName {
	case "CornCreateTask":
		resultJSON, success = c.handleToolCreateTask(args)
	case "CornListTasks":
		resultJSON, success = c.handleToolListTasks(args)
	case "CornGetTask":
		resultJSON, success = c.handleToolGetTask(args)
	case "CornUpdateTask":
		resultJSON, success = c.handleToolUpdateTask(args)
	case "CornDeleteTask":
		resultJSON, success = c.handleToolDeleteTask(args)
	case "CornTriggerTask":
		resultJSON, success = c.handleToolTriggerTask(args)
	default:
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
		})
		return
	}

	resp := uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   success,
		Result:    resultJSON,
	}
	if !success {
		// 从 result 中提取 error
		var res struct {
			Error string `json:"error"`
		}
		json.Unmarshal([]byte(resultJSON), &res)
		resp.Error = res.Error
	}
	c.Client.SendTo(msg.From, uap.MsgToolResult, resp)
}

// handleToolResult 处理远程工具调用结果
func (c *Connection) handleToolResult(msg *uap.Message) {
	var payload uap.ToolResultPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CornAgent] invalid tool_result payload: %v", err)
		return
	}
	c.pendMu.Lock()
	ch, ok := c.pending[payload.RequestID]
	c.pendMu.Unlock()
	if ok {
		ch <- &payload
	} else {
		log.Printf("[CornAgent] tool_result requestID=%s has no pending channel", payload.RequestID)
	}
}

// handleTaskComplete 处理 task_complete 消息（目标agent返回）
func (c *Connection) handleTaskComplete(msg *uap.Message) {
	var payload uap.TaskCompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CornAgent] invalid task_complete payload: %v", err)
		return
	}
	log.Printf("[CornAgent] task_complete taskID=%s status=%s", payload.TaskID, payload.Status)

	c.taskMu.Lock()
	ch, ok := c.taskResults[payload.TaskID]
	c.taskMu.Unlock()
	if ok {
		ch <- &payload
	} else {
		log.Printf("[CornAgent] task_complete taskID=%s has no pending channel", payload.TaskID)
	}
}

// handleTaskEvent 处理 task_event 消息（目标agent进度事件）
func (c *Connection) handleTaskEvent(msg *uap.Message) {
	var payload uap.TaskEventPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CornAgent] invalid task_event payload: %v", err)
		return
	}
	log.Printf("[CornAgent] task_event taskID=%s", payload.TaskID)
	// 记录任务执行进度（可扩展：存储进度到数据库）
}

// handleError 处理 gateway 错误消息
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CornAgent] invalid error payload: %v", err)
		return
	}
	log.Printf("[CornAgent] error from=%s code=%s msg=%s", msg.From, payload.Code, payload.Message)

	// 释放 pending channel
	c.pendMu.Lock()
	ch, ok := c.pending[msg.ID]
	c.pendMu.Unlock()
	if ok {
		ch <- &uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     payload.Message,
		}
	}
}

// ========================= corn.task.* 消息处理 =========================

func (c *Connection) handleTaskCreate(msg *uap.Message) {
	go c.processTaskCreate(msg)
}

func (c *Connection) handleTaskUpdate(msg *uap.Message) {
	go c.processTaskUpdate(msg)
}

func (c *Connection) handleTaskDelete(msg *uap.Message) {
	go c.processTaskDelete(msg)
}

func (c *Connection) handleTaskList(msg *uap.Message) {
	go c.processTaskList(msg)
}

func (c *Connection) handleTaskStatus(msg *uap.Message) {
	go c.processTaskStatus(msg)
}

func (c *Connection) handleTaskTrigger(msg *uap.Message) {
	go c.processTaskTrigger(msg)
}

// ========================= 远程调用 =========================

// callRemoteTool 调用远程 agent 的工具（通过 tool_call 消息）
func (c *Connection) callRemoteTool(toolName string, args map[string]interface{}, timeout time.Duration) (string, error) {
	// 注意：corn-agent 不依赖 toolCatalog，我们直接发送到目标agent
	// 这里需要先找到目标agent的ID，暂时简化实现
	return "", fmt.Errorf("callRemoteTool not implemented yet")
}

// sendTaskAssign 发送 task_assign 到目标 agent，等待 task_complete
func (c *Connection) sendTaskAssign(targetAgentID string, taskID string, payload interface{}, timeout time.Duration) (*uap.TaskCompletePayload, error) {
	ch := make(chan *uap.TaskCompletePayload, 1)

	c.taskMu.Lock()
	c.taskResults[taskID] = ch
	c.taskMu.Unlock()

	defer func() {
		c.taskMu.Lock()
		delete(c.taskResults, taskID)
		c.taskMu.Unlock()
	}()

	taskPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal task payload: %v", err)
	}

	log.Printf("[CornAgent] task_assign → agent=%s taskID=%s", targetAgentID, taskID)

	err = c.Client.Send(&uap.Message{
		Type: uap.MsgTaskAssign,
		ID:   uap.NewMsgID(),
		From: c.AgentID,
		To:   targetAgentID,
		Payload: mustMarshalJSON(uap.TaskAssignPayload{
			TaskID:  taskID,
			Payload: json.RawMessage(taskPayload),
		}),
		Ts: time.Now().UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("send task_assign: %v", err)
	}

	select {
	case result := <-ch:
		return result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("task timeout after %ds", int(timeout.Seconds()))
	}
}

// ========================= 工具函数 =========================

func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}