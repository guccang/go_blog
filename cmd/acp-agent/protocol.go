package main

// WebSocket 消息类型常量
const (
	MsgRegister     = "register"
	MsgRegisterAck  = "register_ack"
	MsgHeartbeat    = "heartbeat"
	MsgHeartbeatAck = "heartbeat_ack"
	MsgStreamEvent  = "stream_event"
	MsgTaskComplete = "task_complete"
)

// RegisterPayload Agent 注册信息
type RegisterPayload struct {
	AgentID       string   `json:"agent_id"`
	Name          string   `json:"name"`
	AgentType     string   `json:"agent_type"`
	Workspaces    []string `json:"workspaces"`
	Projects      []string `json:"projects"`
	MaxConcurrent int      `json:"max_concurrent"`
	AuthToken     string   `json:"auth_token,omitempty"`
}

// HeartbeatPayload Agent 心跳
type HeartbeatPayload struct {
	AgentID        string   `json:"agent_id"`
	AgentType      string   `json:"agent_type"`
	ActiveSessions int      `json:"active_sessions"`
	Load           float64  `json:"load"`
	Projects       []string `json:"projects,omitempty"`
}

// TaskCompletePayload 任务完成（tool_call 完成通知）
type TaskCompletePayload struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

// StreamEventPayload 流式事件转发
type StreamEventPayload struct {
	SessionID string      `json:"session_id"`
	Event     StreamEvent `json:"event"`
}
