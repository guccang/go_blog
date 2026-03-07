package codegen

import (
	"config"
	"fmt"
	log "mylog"
	"sort"
	"strings"
	"sync"
	"time"
)

// processEvent 根据事件更新会话状态
func processEvent(session *CodeSession, event *StreamEvent) {
	if event.SessionID != "" {
		session.mu.Lock()
		session.ClaudeSession = event.SessionID
		session.mu.Unlock()
	}

	if event.CostUSD > 0 {
		session.mu.Lock()
		session.CostUSD = event.CostUSD
		session.mu.Unlock()
	}

	// 记录到消息历史
	switch event.Type {
	case "thinking":
		session.addMessage(SessionMessage{
			Role:    "thinking",
			Content: event.Text,
			Time:    time.Now(),
		})
	case "assistant":
		session.addMessage(SessionMessage{
			Role:    "assistant",
			Content: event.Text,
			Time:    time.Now(),
		})
	case "tool":
		session.addMessage(SessionMessage{
			Role:      "tool",
			Content:   event.Text,
			ToolName:  event.ToolName,
			ToolInput: event.ToolInput,
			Time:      time.Now(),
		})
	case "result":
		if event.Text != "" {
			session.addMessage(SessionMessage{
				Role:    "result",
				Content: event.Text,
				Time:    time.Now(),
			})
		}
	case "summary":
		if event.Text != "" {
			session.addMessage(SessionMessage{
				Role:    "summary",
				Content: event.Text,
				Time:    time.Now(),
			})
		}
	}
}

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
	ClaudeSession string           `json:"claude_session"` // claude --session-id / opencode --session
	Project       string           `json:"project"`
	Prompt        string           `json:"prompt"`
	Model         string           `json:"model,omitempty"`         // 指定模型配置名称
	Tool          string           `json:"tool,omitempty"`          // 编码工具: claudecode, opencode（默认 claudecode）
	AutoDeploy    bool             `json:"auto_deploy,omitempty"`   // 编码完成后自动部署+验证
	DeployOnly    bool             `json:"deploy_only,omitempty"`   // 跳过编码，直接部署+验证
	DeployTarget  string           `json:"deploy_target,omitempty"`  // 部署目标: local/ssh-prod/all
	DeployPort    string           `json:"deploy_port,omitempty"`    // 部署端口号
	PackOnly      bool             `json:"pack_only,omitempty"`      // 仅打包不部署
	Pipeline      string           `json:"pipeline,omitempty"`       // deploy pipeline 名称
	Status        SessionStatus    `json:"status"`
	Messages      []SessionMessage `json:"messages"`
	StartTime     time.Time        `json:"start_time"`
	EndTime       time.Time        `json:"end_time,omitempty"`
	CostUSD       float64          `json:"cost_usd"`
	Error         string           `json:"error,omitempty"`
	AgentID       string           `json:"agent_id,omitempty"` // 执行此任务的远程 agent

	mu          sync.Mutex
	subscribers []chan StreamEvent
	subMu       sync.Mutex
}

// StreamEvent 流式事件（推送给前端）
type StreamEvent struct {
	Type      string  `json:"type"` // system, assistant, thinking, tool, result, error
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
	sessions   = make(map[string]*CodeSession)
	sessionsMu sync.RWMutex
	maxTurns   = 20       // 默认最大轮数
	agentPool  *AgentPool // 远程 agent 连接池
	agentToken string     // agent 认证 token
)

// Init 初始化 CodeGen 模块
func Init() {
	adminAccount := config.GetAdminAccount()

	// agent 认证 token：由 InitGatewayBridge 从 gateway_token 统一设置
	// 兼容旧模式（无 gateway 直连）：回退到 codegen_agent_token
	agentToken = config.GetConfigWithAccount(adminAccount, "codegen_agent_token")

	// 始终初始化 agent 连接池
	agentPool = NewAgentPool()
	go agentPool.CleanupLoop()

	// 定期清理已完成的旧会话，防止内存泄漏
	go sessionCleanupLoop()

	log.MessageF(log.ModuleAgent, "CodeGen initialized (remote-only mode)")
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

// ToolClaudeCode Claude Code 编码工具
const ToolClaudeCode = "claudecode"

// ToolOpenCode OpenCode 编码工具
const ToolOpenCode = "opencode"

// ToolDeploy 部署工具（deploy-agent）
const ToolDeploy = "deploy"

// NormalizeTool 规范化工具名称，返回合法工具名
func NormalizeTool(tool string) string {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "opencode", "oc":
		return ToolOpenCode
	case "claudecode", "cc", "claude", "":
		return ToolClaudeCode
	default:
		return ToolClaudeCode
	}
}

// StartSession 启动编码会话，agentID 可选指定目标 agent
func StartSession(project, prompt, model, tool, agentID string, autoDeploy, deployOnly bool, deployTarget string, packOnly bool, pipeline string, port ...string) (*CodeSession, error) {
	// 项目目录由远程 agent 管理，服务端不需要创建本地目录
	// 仅验证项目名合法性
	if project == "" {
		return nil, fmt.Errorf("project name is empty")
	}

	normalizedTool := NormalizeTool(tool)

	deployPort := ""
	if len(port) > 0 {
		deployPort = port[0]
	}

	session := &CodeSession{
		ID:           fmt.Sprintf("cg_%d", time.Now().UnixMilli()),
		Project:      project,
		Prompt:       prompt,
		Model:        model,
		Tool:         normalizedTool,
		AutoDeploy:   autoDeploy,
		DeployOnly:   deployOnly,
		DeployTarget:  deployTarget,
		DeployPort:    deployPort,
		PackOnly:      packOnly,
		Pipeline:      pipeline,
		AgentID:      agentID,
		Status:       StatusRunning,
		Messages:     make([]SessionMessage, 0),
		StartTime:    time.Now(),
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

	// 异步执行 — 统一走远程 agent
	go func() {
		err := agentPool.Execute(session)
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

	// 检查上一个任务是否还在运行
	session.mu.Lock()
	if session.Status == StatusRunning {
		session.mu.Unlock()
		return fmt.Errorf("session is still running, please wait for it to finish")
	}
	session.Status = StatusRunning
	session.mu.Unlock()

	session.addMessage(SessionMessage{
		Role:    "user",
		Content: prompt,
		Time:    time.Now(),
	})

	go func() {
		err := agentPool.ExecuteResume(session, prompt)
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

	// 统一走远程 agent 停止
	if session.AgentID != "" {
		agentPool.StopRemoteTask(session)
	}

	session.Status = StatusStopped
	session.EndTime = time.Now()

	session.broadcast(StreamEvent{Type: "system", Text: "会话已停止", Done: true})
	return nil
}

// StopAllSessions 停止所有运行中的会话（本地 + 远程 agent）
// 返回停止的会话数量
func StopAllSessions() int {
	stopped := 0

	// 停止 blog-agent 跟踪的所有运行中会话
	sessionsMu.RLock()
	var runningSessions []*CodeSession
	for _, s := range sessions {
		s.mu.Lock()
		if s.Status == StatusRunning {
			runningSessions = append(runningSessions, s)
		}
		s.mu.Unlock()
	}
	sessionsMu.RUnlock()

	for _, s := range runningSessions {
		s.mu.Lock()
		if s.AgentID != "" {
			agentPool.StopRemoteTask(s)
		}
		s.Status = StatusStopped
		s.EndTime = time.Now()
		s.broadcast(StreamEvent{Type: "system", Text: "会话已停止", Done: true})
		s.mu.Unlock()
		stopped++
	}

	// 停止所有远程 agent 上的活跃任务（包括 llm-mcp-agent 直接派发的）
	if agentPool != nil {
		stopped += agentPool.StopAllRunningTasks()
	}

	return stopped
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

// sessionCleanupLoop 定期清理已完成的旧会话，防止 sessions map 无限增长
func sessionCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cleanupSessions()
	}
}

// cleanupSessions 清理已完成且超过 1 小时的会话，保留最近 50 个
func cleanupSessions() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	now := time.Now()
	maxAge := 1 * time.Hour
	maxKeep := 50

	// 如果总数没超限，只清理超时的
	if len(sessions) <= maxKeep {
		for id, s := range sessions {
			s.mu.Lock()
			status := s.Status
			endTime := s.EndTime
			s.mu.Unlock()
			if status != StatusRunning && !endTime.IsZero() && now.Sub(endTime) > maxAge {
				delete(sessions, id)
			}
		}
		return
	}

	// 超限时，删除所有已完成且超时的；如果仍超限，删除最老的已完成会话
	type sessionEntry struct {
		id      string
		endTime time.Time
		running bool
	}
	var entries []sessionEntry
	for id, s := range sessions {
		s.mu.Lock()
		entries = append(entries, sessionEntry{
			id:      id,
			endTime: s.EndTime,
			running: s.Status == StatusRunning,
		})
		s.mu.Unlock()
	}

	// 先删除超时的已完成会话
	for _, e := range entries {
		if !e.running && !e.endTime.IsZero() && now.Sub(e.endTime) > maxAge {
			delete(sessions, e.id)
		}
	}

	// 如果仍超限，按 endTime 升序删除最旧的已完成会话
	if len(sessions) > maxKeep {
		// 重新收集剩余的非运行会话
		var removable []sessionEntry
		for id, s := range sessions {
			s.mu.Lock()
			if s.Status != StatusRunning {
				removable = append(removable, sessionEntry{id: id, endTime: s.EndTime})
			}
			s.mu.Unlock()
		}
		sort.Slice(removable, func(i, j int) bool {
			return removable[i].endTime.Before(removable[j].endTime)
		})
		for _, e := range removable {
			if len(sessions) <= maxKeep {
				break
			}
			delete(sessions, e.id)
		}
	}

	log.DebugF(log.ModuleAgent, "Session cleanup: %d sessions remaining", len(sessions))
}
