package main

import "encoding/json"

// WebSocket 消息类型常量
const (
	MsgRegister     = "register"
	MsgRegisterAck  = "register_ack"
	MsgHeartbeat    = "heartbeat"
	MsgHeartbeatAck = "heartbeat_ack"
	MsgTaskAssign   = "task_assign"
	MsgTaskAccepted = "task_accepted"
	MsgTaskRejected = "task_rejected"
	MsgTaskStop     = "task_stop"
	MsgStreamEvent  = "stream_event"
	MsgTaskComplete = "task_complete"
)

// SessionStatus 会话状态
type SessionStatus string

// AgentMessage WebSocket 统一消息信封
type AgentMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Ts      int64           `json:"ts"`
}

// RegisterPayload Agent 注册信息
type RegisterPayload struct {
	AgentID       string   `json:"agent_id"`
	Name          string   `json:"name"`
	Workspaces    []string `json:"workspaces"`
	Projects      []string `json:"projects"`
	Tools         []string `json:"tools,omitempty"`
	MaxConcurrent int      `json:"max_concurrent"`
	AuthToken     string   `json:"auth_token,omitempty"`
}

// RegisterAckPayload 注册确认
type RegisterAckPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// HeartbeatPayload Agent 心跳
type HeartbeatPayload struct {
	AgentID        string   `json:"agent_id"`
	ActiveSessions int      `json:"active_sessions"`
	Load           float64  `json:"load"`
	Tools          []string `json:"tools,omitempty"`
}

// TaskAssignPayload 任务分派
type TaskAssignPayload struct {
	SessionID  string `json:"session_id"`
	Project    string `json:"project"`
	Prompt     string `json:"prompt"`
	AutoDeploy bool   `json:"auto_deploy,omitempty"`
	DeployOnly bool   `json:"deploy_only,omitempty"`
}

// TaskAcceptedPayload 任务接受确认
type TaskAcceptedPayload struct {
	SessionID string `json:"session_id"`
}

// TaskRejectedPayload 任务拒绝
type TaskRejectedPayload struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

// TaskStopPayload 停止任务
type TaskStopPayload struct {
	SessionID string `json:"session_id"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Done bool   `json:"done,omitempty"`
}

// StreamEventPayload 流式事件转发
type StreamEventPayload struct {
	SessionID string      `json:"session_id"`
	Event     StreamEvent `json:"event"`
}

// TaskCompletePayload 任务完成
type TaskCompletePayload struct {
	SessionID string        `json:"session_id"`
	Status    SessionStatus `json:"status"`
	Error     string        `json:"error,omitempty"`
}
