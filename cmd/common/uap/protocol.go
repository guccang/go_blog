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

	// Claude Mode: 权限交互
	MsgPermissionRequest  = "permission_request"
	MsgPermissionResponse = "permission_response"

	// Claude Mode: 模式切换
	MsgSetMode = "set_mode"

	// 控制协议（ctrl_* 前缀，AgentBase 内置处理）
	MsgCtrlShutdown     = "ctrl_shutdown"
	MsgCtrlShutdownAck  = "ctrl_shutdown_ack"
	MsgCtrlStatus       = "ctrl_status"
	MsgCtrlStatusReport = "ctrl_status_report"
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
	AgentID      string         `json:"agent_id"`
	AgentType    string         `json:"agent_type"`    // "wechat", "go_blog", "llm_mcp", "codegen", "deploy"
	Name         string         `json:"name"`          // 人类可读名称
	Description  string         `json:"description"`   // agent 能力简述
	HostPlatform string         `json:"host_platform"` // 运行平台（macOS/Linux/Windows）
	HostIP       string         `json:"host_ip"`       // 主机 IP 地址
	Workspace    string         `json:"workspace"`     // 工作目录
	Tools        []ToolDef      `json:"tools"`         // 注册的工具列表
	Capacity     int            `json:"capacity"`      // 最大并发
	Meta         map[string]any `json:"meta"`          // 扩展字段
	AuthToken    string         `json:"auth_token"`
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
// 约定:
// - Success: 操作是否成功（唯一判断标准）
// - Result: 成功时为业务数据（JSON 字符串），标准格式:
//   {"data": <具体数据>, "message": "可选的人类可读摘要"}
// - Error: 失败时的错误描述
// - Result 中不再重复放 success/status 字段
type ToolResultPayload struct {
	RequestID string `json:"request_id"` // 对应 Message.ID
	Success   bool   `json:"success"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// BuildToolResult 构建标准化工具返回结果
func BuildToolResult(requestID string, data any, message string) ToolResultPayload {
	result, _ := json.Marshal(map[string]any{
		"data":    data,
		"message": message,
	})
	return ToolResultPayload{
		RequestID: requestID,
		Success:   true,
		Result:    string(result),
	}
}

// BuildToolError 构建标准化工具错误返回
func BuildToolError(requestID string, err string) ToolResultPayload {
	return ToolResultPayload{
		RequestID: requestID,
		Success:   false,
		Error:     err,
	}
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

// ========================= 权限交互载荷 =========================

// PermissionOptionDTO 权限选项
type PermissionOptionDTO struct {
	Index    int    `json:"index"`     // 序号（1-based，供用户输入）
	OptionID string `json:"option_id"` // ACP SDK 中的 optionId
	Name     string `json:"name"`      // 人类可读名称
	Kind     string `json:"kind"`      // allow_once, allow_always, deny 等
}

// PermissionRequestPayload 权限请求（acp-agent → llm-agent）
type PermissionRequestPayload struct {
	SessionID string              `json:"session_id"`
	RequestID string              `json:"request_id"` // 关联回复
	Title     string              `json:"title"`      // 工具名/操作名
	Content   string              `json:"content"`    // 详细描述
	Options   []PermissionOptionDTO `json:"options"`
}

// PermissionResponsePayload 权限回复（llm-agent → acp-agent）
type PermissionResponsePayload struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	OptionID  string `json:"option_id"`  // 用户选择的选项 ID
	Cancelled bool   `json:"cancelled"`  // 是否取消
}

// ========================= 模式切换载荷 =========================

// SetModePayload 模式切换请求（llm-agent → acp-agent）
type SetModePayload struct {
	SessionID string `json:"session_id"`
	ModeID    string `json:"mode_id"`
}

// ========================= 控制协议载荷 =========================

// CtrlShutdownPayload 控制协议：关闭请求
type CtrlShutdownPayload struct {
	Reason     string `json:"reason,omitempty"`
	TimeoutSec int    `json:"timeout_sec,omitempty"` // drain 超时（0=默认 30s）
	Force      bool   `json:"force,omitempty"`       // true=立即退出，跳过 drain
}

// CtrlShutdownAckPayload 控制协议：关闭确认
type CtrlShutdownAckPayload struct {
	AgentID      string `json:"agent_id"`
	Accepted     bool   `json:"accepted"`
	CurrentState string `json:"current_state"`
	ActiveTasks  int    `json:"active_tasks"`
	Error        string `json:"error,omitempty"`
}

// CtrlStatusPayload 控制协议：状态查询
type CtrlStatusPayload struct{}

// CtrlStatusReportPayload 控制协议：状态报告
type CtrlStatusReportPayload struct {
	AgentID     string         `json:"agent_id"`
	AgentType   string         `json:"agent_type"`
	AgentName   string         `json:"agent_name"`
	State       string         `json:"state"`
	ActiveTasks int            `json:"active_tasks"`
	Capacity    int            `json:"capacity"`
	Uptime      int64          `json:"uptime_sec"`
	Meta        map[string]any `json:"meta,omitempty"`
}
