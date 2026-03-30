package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"agentbase"
	"uap"
)

// Connection UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg         *Config
	engine      *CronEngine
	activeCount int32
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	tools := buildToolDefs()

	log.Printf("[CronAgent] 创建连接 agentID=%s type=cron_agent tools=%d", agentID, len(tools))
	for _, t := range tools {
		log.Printf("[CronAgent]   ├─ tool: %s", t.Name)
	}

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "cron_agent",
		AgentName:   cfg.AgentName,
		Description: "定时任务调度代理，支持 cron 表达式、间隔、延迟一次性任务，发送到 llm-agent 执行并推送微信",
		AuthToken:   cfg.AuthToken,
		Capacity:    10,
		Tools:       tools,
		Meta: map[string]any{
			"version": "2.0.0",
		},
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
	}

	// 创建引擎（加载任务 + 启动调度器）
	c.engine = NewCronEngine(cfg, c.AgentBase)

	// 注册消息处理器
	c.RegisterToolCallHandler(c.handleToolCall)
	c.RegisterHandler(uap.MsgTaskComplete, c.handleTaskComplete)
	c.RegisterHandler(uap.MsgError, c.handleError)

	log.Printf("[CronAgent] ✓ 连接管理器创建完成，已注册 3 个消息处理器 (tool_call, task_complete, error)")

	return c
}

// ========================= 工具定义 =========================

func buildToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "cronCreateTask",
			Description: "创建定时任务。支持三种调度模式：(1) delay_sec=N 延迟 N 秒后执行一次；(2) schedule 使用 cron 表达式如 '0 20 * * *' 每天20点执行，或间隔如 '@every 20m' 每20分钟执行；(3) schedule + one_shot=true 在下一个匹配时间执行一次后自动删除。task_type 为 'cron_reminder'（提醒通知，需要 message）或 'cron_query'（LLM 查询执行，需要 query）",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":         map[string]interface{}{"type": "string", "description": "任务名称"},
					"task_type":    map[string]interface{}{"type": "string", "enum": []string{"cron_reminder", "cron_query"}, "description": "任务类型"},
					"schedule":     map[string]interface{}{"type": "string", "description": "cron 表达式或间隔，如 '0 20 * * *' 或 '@every 20m'"},
					"delay_sec":    map[string]interface{}{"type": "integer", "description": "延迟秒数（一次性延迟任务，与 schedule 互斥）"},
					"account":      map[string]interface{}{"type": "string", "description": "用户账号"},
					"wechat_user":  map[string]interface{}{"type": "string", "description": "微信用户标识；仅微信场景需要，app/group 场景可留空"},
					"message":      map[string]interface{}{"type": "string", "description": "提醒内容（task_type=cron_reminder 时必填）"},
					"query":        map[string]interface{}{"type": "string", "description": "查询问题（task_type=cron_query 时必填）"},
					"one_shot":     map[string]interface{}{"type": "boolean", "description": "是否一次性任务（配合 schedule 使用，执行一次后自动删除）"},
					"ignore_quiet": map[string]interface{}{"type": "boolean", "description": "是否忽略免打扰时段（默认 false 受免打扰控制；服务器监控等重要任务设为 true）"},
				},
				"required": []string{"name", "task_type", "account"},
			}),
		},
		{
			Name:        "cronListTasks",
			Description: "列出所有定时任务",
			Parameters:  agentbase.MustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "cronDeleteTask",
			Description: "删除指定定时任务",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string", "description": "任务 ID"},
				},
				"required": []string{"task_id"},
			}),
		},
		{
			Name:        "cronTriggerTask",
			Description: "立即触发指定定时任务执行一次（不影响正常调度）",
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{"type": "string", "description": "任务 ID"},
				},
				"required": []string{"task_id"},
			}),
		},
		{
			Name:        "cronListPending",
			Description: "[debug] 列出正在执行中的任务（已发送到 llm-agent 尚未返回 task_complete 的执行）",
			Parameters:  agentbase.MustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
	}
}

// ========================= 消息处理 =========================

func (c *Connection) handleToolCall(msg *uap.Message) {
	atomic.AddInt32(&c.activeCount, 1)
	defer atomic.AddInt32(&c.activeCount, -1)

	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CronAgent] ✗ 解析 tool_call 失败: %v raw=%s", err, string(msg.Payload))
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid payload"))
		return
	}

	log.Printf("[CronAgent] ← tool_call 收到 from=%s msgID=%s tool=%s", msg.From, msg.ID, payload.ToolName)

	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[CronAgent] ✗ 解析 arguments 失败: %v raw=%s", err, string(payload.Arguments))
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid arguments"))
			return
		}
		argsJSON, _ := json.Marshal(args)
		log.Printf("[CronAgent]   args=%s", string(argsJSON))
	} else {
		args = make(map[string]interface{})
		log.Printf("[CronAgent]   args=(empty)")
	}

	var result string
	var success bool

	switch payload.ToolName {
	case "cronCreateTask":
		result, success = c.toolCreateTask(args)
	case "cronListTasks":
		result, success = c.toolListTasks()
	case "cronDeleteTask":
		result, success = c.toolDeleteTask(args)
	case "cronTriggerTask":
		result, success = c.toolTriggerTask(args)
	case "cronListPending":
		result, success = c.toolListPending()
	default:
		log.Printf("[CronAgent] ✗ 未知工具: %s", payload.ToolName)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName)))
		return
	}

	if success {
		log.Printf("[CronAgent] → tool_result 成功 to=%s tool=%s resultLen=%d", msg.From, payload.ToolName, len(result))
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolResult(msg.ID, result, ""))
	} else {
		log.Printf("[CronAgent] → tool_result 失败 to=%s tool=%s error=%s", msg.From, payload.ToolName, result)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, result))
	}
}

func (c *Connection) handleTaskComplete(msg *uap.Message) {
	var payload uap.TaskCompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CronAgent] ✗ 解析 task_complete 失败: %v raw=%s", err, string(msg.Payload))
		return
	}

	log.Printf("[CronAgent] ← task_complete 收到 from=%s taskID=%s status=%s resultLen=%d",
		msg.From, payload.TaskID, payload.Status, len(payload.Result))
	if payload.Error != "" {
		log.Printf("[CronAgent]   error=%s", payload.Error)
	}

	c.engine.HandleTaskComplete(payload.TaskID, payload.Status, payload.Error, payload.Result)
}

func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[CronAgent] ✗ 解析 error 失败: %v raw=%s", err, string(msg.Payload))
		return
	}
	log.Printf("[CronAgent] ← error 收到 from=%s msgID=%s code=%s msg=%s",
		msg.From, msg.ID, payload.Code, payload.Message)
}

// ========================= 工具实现 =========================

func (c *Connection) toolCreateTask(args map[string]interface{}) (string, bool) {
	name, _ := args["name"].(string)
	taskType, _ := args["task_type"].(string)
	schedule, _ := args["schedule"].(string)
	delaySec, _ := args["delay_sec"].(float64)
	account, _ := args["account"].(string)
	wechatUser, _ := args["wechat_user"].(string)
	message, _ := args["message"].(string)
	query, _ := args["query"].(string)
	oneShot, _ := args["one_shot"].(bool)
	ignoreQuiet, _ := args["ignore_quiet"].(bool)

	log.Printf("[CronAgent] toolCreateTask name=%q type=%s schedule=%q delay=%.0f account=%s wechat=%s oneShot=%v ignoreQuiet=%v",
		name, taskType, schedule, delaySec, account, wechatUser, oneShot, ignoreQuiet)

	// 参数校验
	if name == "" {
		log.Printf("[CronAgent] ✗ toolCreateTask 校验失败: 任务名称为空")
		return "任务名称不能为空", false
	}
	if taskType != "cron_reminder" && taskType != "cron_query" {
		log.Printf("[CronAgent] ✗ toolCreateTask 校验失败: 无效 task_type=%s", taskType)
		return "task_type 必须是 cron_reminder 或 cron_query", false
	}
	if schedule == "" && delaySec <= 0 {
		log.Printf("[CronAgent] ✗ toolCreateTask 校验失败: schedule 和 delay_sec 都为空")
		return "必须指定 schedule 或 delay_sec", false
	}
	if taskType == "cron_reminder" && message == "" {
		log.Printf("[CronAgent] ✗ toolCreateTask 校验失败: cron_reminder 缺少 message")
		return "cron_reminder 类型必须指定 message", false
	}
	if taskType == "cron_query" && query == "" {
		log.Printf("[CronAgent] ✗ toolCreateTask 校验失败: cron_query 缺少 query")
		return "cron_query 类型必须指定 query", false
	}

	task := &CronTask{
		ID:          newTaskID(),
		Name:        name,
		TaskType:    taskType,
		Schedule:    schedule,
		DelaySec:    int(delaySec),
		Account:     account,
		WechatUser:  wechatUser,
		Message:     message,
		Query:       query,
		OneShot:     oneShot || (schedule == "" && delaySec > 0), // 纯延迟任务默认 one_shot
		IgnoreQuiet: ignoreQuiet,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	if err := c.engine.AddTask(task); err != nil {
		log.Printf("[CronAgent] ✗ toolCreateTask 添加失败: %v", err)
		return fmt.Sprintf("创建任务失败: %v", err), false
	}

	log.Printf("[CronAgent] ✓ toolCreateTask 成功 ID=%s", task.ID)

	resp, _ := json.Marshal(map[string]interface{}{
		"task_id": task.ID,
		"name":    task.Name,
		"message": "任务创建成功",
	})
	return string(resp), true
}

func (c *Connection) toolListTasks() (string, bool) {
	tasks := c.engine.ListTasks()
	log.Printf("[CronAgent] toolListTasks 返回 %d 个任务", len(tasks))
	for _, t := range tasks {
		log.Printf("[CronAgent]   ├─ ID=%s name=%s type=%s schedule=%q", t.ID, t.Name, t.TaskType, t.Schedule)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
	return string(resp), true
}

func (c *Connection) toolDeleteTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		log.Printf("[CronAgent] ✗ toolDeleteTask: task_id 为空")
		return "task_id 不能为空", false
	}

	log.Printf("[CronAgent] toolDeleteTask ID=%s", taskID)
	if err := c.engine.RemoveTask(taskID); err != nil {
		log.Printf("[CronAgent] ✗ toolDeleteTask 失败: %v", err)
		return err.Error(), false
	}

	log.Printf("[CronAgent] ✓ toolDeleteTask 成功 ID=%s", taskID)
	return "任务删除成功", true
}

func (c *Connection) toolTriggerTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		log.Printf("[CronAgent] ✗ toolTriggerTask: task_id 为空")
		return "task_id 不能为空", false
	}

	log.Printf("[CronAgent] toolTriggerTask ID=%s", taskID)
	if err := c.engine.TriggerTask(taskID); err != nil {
		log.Printf("[CronAgent] ✗ toolTriggerTask 失败: %v", err)
		return err.Error(), false
	}

	log.Printf("[CronAgent] ✓ toolTriggerTask 已触发 ID=%s", taskID)
	return "任务已触发", true
}

func (c *Connection) toolListPending() (string, bool) {
	pending := c.engine.ListPending()
	log.Printf("[CronAgent] toolListPending 返回 %d 个执行中任务", len(pending))
	for _, p := range pending {
		log.Printf("[CronAgent]   ├─ executionID=%s cronTaskID=%s", p["execution_id"], p["task_id"])
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"pending": pending,
		"total":   len(pending),
	})
	return string(resp), true
}
