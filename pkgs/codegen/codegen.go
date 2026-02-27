package codegen

import (
	"config"
	"fmt"
	log "mylog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	StatusRunning SessionStatus = "running"
	StatusDone    SessionStatus = "done"
	StatusError   SessionStatus = "error"
	StatusStopped SessionStatus = "stopped"
)

// SessionMessage 会话消息
type SessionMessage struct {
	Role      string    `json:"role"` // user, assistant, system, tool
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolInput string    `json:"tool_input,omitempty"`
	Time      time.Time `json:"time"`
}

// CodeSession 编码会话
type CodeSession struct {
	ID            string           `json:"id"`
	ClaudeSession string           `json:"claude_session"` // claude --session-id
	Project       string           `json:"project"`
	Prompt        string           `json:"prompt"`
	Status        SessionStatus    `json:"status"`
	Messages      []SessionMessage `json:"messages"`
	StartTime     time.Time        `json:"start_time"`
	EndTime       time.Time        `json:"end_time,omitempty"`
	CostUSD       float64          `json:"cost_usd"`
	Error         string           `json:"error,omitempty"`
	AgentID       string           `json:"agent_id,omitempty"` // 执行此任务的远程 agent

	mu          sync.Mutex
	process     *os.Process
	subscribers []chan StreamEvent
	subMu       sync.Mutex
}

// StreamEvent 流式事件（推送给前端）
type StreamEvent struct {
	Type      string  `json:"type"` // system, assistant, tool, result, error
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

// 全局状态
var (
	sessions      = make(map[string]*CodeSession)
	sessionsMu    sync.RWMutex
	workspaces    []string // 多个工作区路径
	claudePath    string
	maxTurns      int
	executionMode string     // "local" | "remote" | "auto"
	agentPool     *AgentPool // 远程 agent 连接池
	agentToken    string     // agent 认证 token
)

// Init 初始化 CodeGen 模块
func Init() {
	adminAccount := config.GetAdminAccount()
	wsConfig := config.GetConfigWithAccount(adminAccount, "codegen_workspace")

	claudePath = config.GetConfigWithAccount(adminAccount, "codegen_claude_path")
	if claudePath == "" {
		claudePath = "claude"
	}

	maxTurnsStr := config.GetConfigWithAccount(adminAccount, "codegen_max_turns")
	maxTurns = 20
	if maxTurnsStr != "" {
		fmt.Sscanf(maxTurnsStr, "%d", &maxTurns)
	}

	// 解析多个工作区路径（逗号分隔）
	workspaces = make([]string, 0)
	if wsConfig == "" {
		wsConfig = "./codegen"
	}
	for _, ws := range strings.Split(wsConfig, ",") {
		ws = strings.TrimSpace(ws)
		if ws == "" {
			continue
		}
		absWs, _ := filepath.Abs(ws)
		if err := os.MkdirAll(absWs, 0755); err != nil {
			log.ErrorF(log.ModuleAgent, "CodeGen: failed to create workspace %s: %v", absWs, err)
			continue
		}
		workspaces = append(workspaces, absWs)
	}

	// 远程 agent 模式配置
	executionMode = config.GetConfigWithAccount(adminAccount, "codegen_mode")
	if executionMode == "" {
		executionMode = "local"
	}
	agentToken = config.GetConfigWithAccount(adminAccount, "codegen_agent_token")

	if executionMode != "local" {
		agentPool = NewAgentPool()
		go agentPool.CleanupLoop()
		log.MessageF(log.ModuleAgent, "CodeGen: remote agent pool enabled (mode=%s)", executionMode)
	}

	log.MessageF(log.ModuleAgent, "CodeGen initialized: workspaces=%v, claude=%s, maxTurns=%d, mode=%s",
		workspaces, claudePath, maxTurns, executionMode)
}

// GetWorkspace 获取所有工作区路径（展示用）
func GetWorkspace() string {
	return strings.Join(workspaces, " ; ")
}

// GetWorkspaces 获取所有工作区路径
func GetWorkspaces() []string {
	return workspaces
}

// GetDefaultWorkspace 获取默认工作区（第一个）
func GetDefaultWorkspace() string {
	if len(workspaces) > 0 {
		return workspaces[0]
	}
	return ""
}

// ResolveProjectPath 根据项目名查找所在的工作区绝对路径
func ResolveProjectPath(project string) (string, error) {
	for _, ws := range workspaces {
		p := filepath.Join(ws, project)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("project not found: %s", project)
}

// Subscribe 订阅会话事件
func (s *CodeSession) Subscribe() chan StreamEvent {
	ch := make(chan StreamEvent, 100)
	s.subMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (s *CodeSession) Unsubscribe(ch chan StreamEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// broadcast 广播事件给所有订阅者
func (s *CodeSession) broadcast(event StreamEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// 丢弃慢消费者的消息
		}
	}
}

// addMessage 添加消息到历史
func (s *CodeSession) addMessage(msg SessionMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, msg)
}

// StartSession 启动编码会话
func StartSession(project, prompt string) (*CodeSession, error) {
	// 查找项目所在工作区
	projectPath, err := ResolveProjectPath(project)
	if err != nil {
		// 如果项目不存在，在默认工作区创建
		projectPath = filepath.Join(GetDefaultWorkspace(), project)
	}

	// 确保项目目录存在
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return nil, fmt.Errorf("create project dir: %v", err)
	}

	session := &CodeSession{
		ID:        fmt.Sprintf("cg_%d", time.Now().UnixMilli()),
		Project:   project,
		Prompt:    prompt,
		Status:    StatusRunning,
		Messages:  make([]SessionMessage, 0),
		StartTime: time.Now(),
	}

	sessionsMu.Lock()
	sessions[session.ID] = session
	sessionsMu.Unlock()

	// 添加用户消息
	session.addMessage(SessionMessage{
		Role:    "user",
		Content: prompt,
		Time:    time.Now(),
	})

	// 异步执行 Claude
	go func() {
		var err error
		// 按执行模式分发
		if executionMode == "remote" || (executionMode == "auto" && agentPool != nil && agentPool.HasAgents()) {
			err = agentPool.Execute(session)
		}
		if executionMode == "local" || (executionMode == "auto" && err != nil) {
			err = RunClaude(session)
		}
		if err != nil {
			session.mu.Lock()
			session.Status = StatusError
			session.Error = err.Error()
			session.EndTime = time.Now()
			session.mu.Unlock()

			session.broadcast(StreamEvent{
				Type: "error",
				Text: err.Error(),
				Done: true,
			})
		}
	}()

	log.MessageF(log.ModuleAgent, "CodeGen session started: %s, project=%s", session.ID, project)
	return session, nil
}

// SendMessage 向已有会话追加消息
func SendMessage(sessionID, prompt string) error {
	session := GetSession(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 检查上一个进程是否还在运行
	session.mu.Lock()
	if session.Status == StatusRunning {
		session.mu.Unlock()
		return fmt.Errorf("session is still running, please wait for it to finish")
	}
	if session.process != nil {
		// 上一个进程刚结束但引用还在，等待清理
		session.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
		session.mu.Lock()
	}
	session.Status = StatusRunning
	session.mu.Unlock()

	session.addMessage(SessionMessage{
		Role:    "user",
		Content: prompt,
		Time:    time.Now(),
	})

	go func() {
		var err error
		useRemote := false

		// 如果之前由远程 agent 执行，继续用远程
		if session.AgentID != "" && agentPool != nil {
			useRemote = true
		} else if executionMode == "remote" || (executionMode == "auto" && agentPool != nil && agentPool.HasAgents()) {
			useRemote = true
		}

		if useRemote {
			err = agentPool.ExecuteResume(session, prompt)
		}
		// local 模式，或 auto 模式远程失败时回退本地
		if !useRemote || (executionMode == "auto" && err != nil) {
			err = RunClaudeResume(session, prompt)
		}
		if err != nil {
			session.mu.Lock()
			session.Status = StatusError
			session.Error = err.Error()
			session.mu.Unlock()
			session.broadcast(StreamEvent{Type: "error", Text: err.Error(), Done: true})
		}
	}()

	return nil
}

// StopSession 停止运行中的会话
func StopSession(sessionID string) error {
	session := GetSession(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 远程模式：发送 task_stop 给 agent
	if session.AgentID != "" && agentPool != nil {
		agentPool.StopRemoteTask(session)
	} else if session.process != nil {
		session.process.Kill()
	}

	session.Status = StatusStopped
	session.EndTime = time.Now()

	session.broadcast(StreamEvent{Type: "system", Text: "会话已停止", Done: true})
	return nil
}

// GetSession 获取会话
func GetSession(sessionID string) *CodeSession {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	return sessions[sessionID]
}

// GetSessions 获取所有会话（按时间倒序）
func GetSessions() []*CodeSession {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	result := make([]*CodeSession, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, s)
	}
	return result
}

// GetAgentPool 获取 agent 连接池（供 HTTP 层使用）
func GetAgentPool() *AgentPool {
	return agentPool
}

// GetAgentToken 获取 agent 认证 token
func GetAgentToken() string {
	return agentToken
}

// isSubPath 检查 child 是否在 parent 下（防止路径穿越）
func isSubPath(parent, child string) bool {
	absParent, _ := filepath.Abs(parent)
	absChild, _ := filepath.Abs(child)
	rel, err := filepath.Rel(absParent, absChild)
	if err != nil {
		return false
	}
	return rel != ".." && len(rel) > 0 && rel[0] != '.'
}
