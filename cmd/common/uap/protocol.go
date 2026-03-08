package uap

import "encoding/json"

// ========================= 消息类型常量 =========================

const (
	// 生命周期
	MsgRegister    = "register"
	MsgRegisterAck = "register_ack"
	MsgHeartbeat   = "heartbeat"
	MsgHeartbeatAck = "heartbeat_ack"

	// 工具调用（跨 agent）
	MsgToolCall   = "tool_call"
	MsgToolResult = "tool_result"

	// 长任务
	MsgTaskAssign   = "task_assign"
	MsgTaskAccepted = "task_accepted"
	MsgTaskRejected = "task_rejected"
	MsgTaskEvent    = "task_event"
	MsgTaskComplete = "task_complete"
	MsgTaskStop     = "task_stop"

	// 通知
	MsgNotify = "notify"

	// 错误
	MsgError = "error"
)

// ========================= 消息信封 =========================

// Message UAP 统一消息信封
type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`      // 唯一消息 ID（请求-响应关联）
	From    string          `json:"from"`    // 源 agent ID
	To      string          `json:"to"`      // 目标 agent ID
	Payload json.RawMessage `json:"payload"`
	Ts      int64           `json:"ts"`
}

// ========================= 注册载荷 =========================

// RegisterPayload agent 注册信息
type RegisterPayload struct {
	AgentID   string         `json:"agent_id"`
	AgentType string         `json:"agent_type"` // "wechat", "go_blog", "llm_mcp", "codegen", "deploy"
	Name      string         `json:"name"`       // 人类可读名称
	Tools     []ToolDef      `json:"tools"`      // 注册的工具列表
	Capacity  int            `json:"capacity"`   // 最大并发
	Meta      map[string]any `json:"meta"`       // 扩展字段
	AuthToken string         `json:"auth_token"`
}

// ToolDef 工具定义
type ToolDef struct {
	Name        string          `json:"name"`        // 命名空间: "blog.GetTodos"
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`  // JSON Schema
}

// RegisterAckPayload 注册确认
type RegisterAckPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ========================= 心跳载荷 =========================

// HeartbeatPayload 心跳数据
type HeartbeatPayload struct {
	AgentID string `json:"agent_id"`
	Load    float64 `json:"load"`
}

// ========================= 工具调用载荷 =========================

// ToolCallPayload 跨 agent 工具调用请求
type ToolCallPayload struct {
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResultPayload 跨 agent 工具调用结果
type ToolResultPayload struct {
	RequestID string `json:"request_id"` // 对应 Message.ID
	Success   bool   `json:"success"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ========================= 长任务载荷 =========================

// TaskAssignPayload 任务分派
type TaskAssignPayload struct {
	TaskID  string          `json:"task_id"`
	Payload json.RawMessage `json:"payload"` // 任务专属数据
}

// TaskAcceptedPayload 任务接受
type TaskAcceptedPayload struct {
	TaskID string `json:"task_id"`
}

// TaskRejectedPayload 任务拒绝
type TaskRejectedPayload struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// TaskEventPayload 任务进度事件
type TaskEventPayload struct {
	TaskID string          `json:"task_id"`
	Event  json.RawMessage `json:"event"`
}

// TaskCompletePayload 任务完成
type TaskCompletePayload struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"` // "success", "failed", "cancelled"
	Error  string `json:"error,omitempty"`
	Result string `json:"result,omitempty"` // LLM 结果文本
}

// TaskStopPayload 停止任务
type TaskStopPayload struct {
	TaskID string `json:"task_id"`
}

// ========================= 通知载荷 =========================

// NotifyPayload 单向通知
type NotifyPayload struct {
	Channel string `json:"channel"` // 通知渠道: "wechat", "email"
	To      string `json:"to"`      // 接收人
	Content string `json:"content"`
}

// ========================= 错误载荷 =========================

// ErrorPayload 错误消息
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
