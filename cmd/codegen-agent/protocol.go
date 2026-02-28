package main

import "encoding/json"

// WebSocket 消息类型常量
const (
	MsgRegister          = "register"
	MsgRegisterAck       = "register_ack"
	MsgHeartbeat         = "heartbeat"
	MsgHeartbeatAck      = "heartbeat_ack"
	MsgTaskAssign        = "task_assign"
	MsgTaskAccepted      = "task_accepted"
	MsgTaskRejected      = "task_rejected"
	MsgTaskStop          = "task_stop"
	MsgStreamEvent       = "stream_event"
	MsgTaskComplete      = "task_complete"
	MsgFileRead          = "file_read"
	MsgFileReadResp      = "file_read_resp"
	MsgTreeRead          = "tree_read"
	MsgTreeReadResp      = "tree_read_resp"
	MsgProjectCreate     = "project_create"
	MsgProjectCreateResp = "project_create_resp"
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
	AgentID          string   `json:"agent_id"`
	Name             string   `json:"name"`
	Workspaces       []string `json:"workspaces"`
	Projects         []string `json:"projects"`
	Models           []string `json:"models,omitempty"`            // 兼容旧版
	ClaudeCodeModels []string `json:"claudecode_models,omitempty"` // Claude Code 模型配置
	OpenCodeModels   []string `json:"opencode_models,omitempty"`   // OpenCode 模型配置
	Tools            []string `json:"tools,omitempty"`
	MaxConcurrent    int      `json:"max_concurrent"`
	AuthToken        string   `json:"auth_token,omitempty"`
}

// RegisterAckPayload 注册确认
type RegisterAckPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// HeartbeatPayload Agent 心跳
type HeartbeatPayload struct {
	AgentID          string   `json:"agent_id"`
	ActiveSessions   int      `json:"active_sessions"`
	Load             float64  `json:"load"`
	Projects         []string `json:"projects,omitempty"`
	Models           []string `json:"models,omitempty"`            // 兼容旧版
	ClaudeCodeModels []string `json:"claudecode_models,omitempty"` // Claude Code 模型配置
	OpenCodeModels   []string `json:"opencode_models,omitempty"`   // OpenCode 模型配置
	Tools            []string `json:"tools,omitempty"`
}

// TaskAssignPayload 任务分派
type TaskAssignPayload struct {
	SessionID     string `json:"session_id"`
	Project       string `json:"project"`
	Prompt        string `json:"prompt"`
	MaxTurns      int    `json:"max_turns"`
	SystemPrompt  string `json:"system_prompt"`
	ClaudeSession string `json:"claude_session,omitempty"`
	Model         string `json:"model,omitempty"`
	Tool          string `json:"tool,omitempty"`
	AutoDeploy    bool   `json:"auto_deploy,omitempty"`  // 编码完成后自动部署+验证
	DeployOnly    bool   `json:"deploy_only,omitempty"` // 跳过编码，直接部署+验证
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
	Type      string  `json:"type"`
	Text      string  `json:"text,omitempty"`
	ToolName  string  `json:"tool_name,omitempty"`
	ToolInput string  `json:"tool_input,omitempty"`
	SessionID string  `json:"session_id,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	TokensIn  int     `json:"tokens_in,omitempty"`
	TokensOut int     `json:"tokens_out,omitempty"`
	Duration  float64 `json:"duration_ms,omitempty"`
	NumTurns  int     `json:"num_turns,omitempty"`
	Done      bool    `json:"done,omitempty"`
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

// FileReadPayload 请求读取文件
type FileReadPayload struct {
	RequestID string `json:"request_id"`
	Project   string `json:"project"`
	Path      string `json:"path"`
}

// FileReadRespPayload 文件读取响应
type FileReadRespPayload struct {
	RequestID string `json:"request_id"`
	Content   string `json:"content,omitempty"`
	Error     string `json:"error,omitempty"`
}

// TreeReadPayload 请求读取目录树
type TreeReadPayload struct {
	RequestID string `json:"request_id"`
	Project   string `json:"project"`
	MaxDepth  int    `json:"max_depth"`
}

// DirNode 目录树节点
type DirNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Size     int64      `json:"size,omitempty"`
	Children []*DirNode `json:"children,omitempty"`
}

// TreeReadRespPayload 目录树响应
type TreeReadRespPayload struct {
	RequestID string   `json:"request_id"`
	Tree      *DirNode `json:"tree,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// ProjectCreatePayload 请求创建项目
type ProjectCreatePayload struct {
	RequestID string `json:"request_id"`
	Name      string `json:"name"`
}

// ProjectCreateRespPayload 项目创建响应
type ProjectCreateRespPayload struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}
