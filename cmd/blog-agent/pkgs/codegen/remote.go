package codegen

import (
	"config"
	"encoding/json"
	"fmt"
	log "mylog"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DirNode 目录树节点
type DirNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Size     int64      `json:"size,omitempty"`
	Children []*DirNode `json:"children,omitempty"`
}

// MessageSender 消息发送接口（支持直连 WebSocket 和 gateway 路由两种模式）
type MessageSender interface {
	SendAgentMsg(msgType string, payload interface{}) error
}

// WebSocketSender 直连 WebSocket 发送器
type WebSocketSender struct {
	Conn *websocket.Conn
}

// SendAgentMsg 通过 WebSocket 直接发送消息
func (s *WebSocketSender) SendAgentMsg(msgType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := AgentMessage{
		Type:    msgType,
		Payload: json.RawMessage(data),
		Ts:      time.Now().UnixMilli(),
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.Conn.WriteMessage(websocket.TextMessage, msgData)
}

// RemoteAgent 远程 Agent 连接
type RemoteAgent struct {
	ID               string
	Name             string
	Sender           MessageSender   // 统一发送接口
	Conn             *websocket.Conn // 直连模式保留（gateway 模式为 nil）
	Workspaces       []string
	Projects         []string // agent 上报的可用项目
	Models           []string // agent 支持的模型配置列表（兼容旧版）
	ClaudeCodeModels []string // Claude Code 模型配置
	OpenCodeModels   []string // OpenCode 模型配置
	Tools            []string // agent 支持的编码工具列表 (claudecode, opencode)
	DeployTargets    []string // 可用部署目标列表
	HostPlatform     string   // 主机平台
	Pipelines        []string // deploy agent 上报的可用 pipeline 列表
	MaxConcurrent    int
	ActiveSessions   map[string]bool
	LastHeartbeat    time.Time
	Status           string // online, busy, offline
	mu               sync.Mutex
}

// AgentPool Agent 连接池
type AgentPool struct {
	agents  map[string]*RemoteAgent
	mu      sync.RWMutex
	pending map[string]chan json.RawMessage // request_id -> response channel
	pendMu  sync.Mutex
}

// NewAgentPool 创建 Agent 连接池
func NewAgentPool() *AgentPool {
	return &AgentPool{
		agents:  make(map[string]*RemoteAgent),
		pending: make(map[string]chan json.RawMessage),
	}
}

// HandleAgentWebSocket 处理 Agent WebSocket 连接
func (p *AgentPool) HandleAgentWebSocket(conn *websocket.Conn) {
	defer conn.Close()

	var agent *RemoteAgent

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen: agent ws read error: %v", err)
			if agent != nil {
				p.removeAgent(agent.ID)
			}
			return
		}

		var msg AgentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen: agent ws parse error: %v", err)
			continue
		}

		switch msg.Type {
		case MsgRegister:
			var payload RegisterPayload
			json.Unmarshal(msg.Payload, &payload)

			// 验证 token
			if agentToken != "" && payload.AuthToken != agentToken {
				sendAgentMsg(conn, MsgRegisterAck, RegisterAckPayload{
					Success: false, Error: "invalid auth token",
				})
				return
			}

			// 检查是否有同名 agent 已在线
			if existingAgent := p.findOnlineAgentByName(payload.Name); existingAgent != nil {
				sendAgentMsg(conn, MsgRegisterAck, RegisterAckPayload{
					Success: false,
					Error:   fmt.Sprintf("agent '%s' already connected (id=%s), reject duplicate", payload.Name, existingAgent.ID),
				})
				log.WarnF(log.ModuleAgent, "CodeGen: reject duplicate agent name=%s, existing id=%s, new id=%s",
					payload.Name, existingAgent.ID, payload.AgentID)
				return
			}

			agent = &RemoteAgent{
				ID:               payload.AgentID,
				Name:             payload.Name,
				Sender:           &WebSocketSender{Conn: conn},
				Conn:             conn,
				Workspaces:       payload.Workspaces,
				Projects:         payload.Projects,
				Models:           payload.Models,
				ClaudeCodeModels: payload.ClaudeCodeModels,
				OpenCodeModels:   payload.OpenCodeModels,
				Tools:            payload.Tools,
				DeployTargets:    payload.DeployTargets,
				HostPlatform:     payload.HostPlatform,
				Pipelines:        payload.Pipelines,
				MaxConcurrent:    payload.MaxConcurrent,
				ActiveSessions:   make(map[string]bool),
				LastHeartbeat:    time.Now(),
				Status:           "online",
			}
			p.addAgent(agent)

			sendAgentMsg(conn, MsgRegisterAck, RegisterAckPayload{Success: true})
			log.MessageF(log.ModuleAgent, "CodeGen: agent registered: %s (%s), workspaces=%v, claudecode_models=%d, opencode_models=%d",
				agent.ID, agent.Name, agent.Workspaces, len(agent.ClaudeCodeModels), len(agent.OpenCodeModels))

		case MsgHeartbeat:
			var payload HeartbeatPayload
			json.Unmarshal(msg.Payload, &payload)
			if agent != nil {
				agent.mu.Lock()
				agent.LastHeartbeat = time.Now()
				if len(payload.Projects) > 0 {
					agent.Projects = payload.Projects
				}
				if len(payload.Models) > 0 {
					agent.Models = payload.Models
				}
				if len(payload.ClaudeCodeModels) > 0 {
					agent.ClaudeCodeModels = payload.ClaudeCodeModels
				}
				if len(payload.OpenCodeModels) > 0 {
					agent.OpenCodeModels = payload.OpenCodeModels
				}
				if len(payload.Tools) > 0 {
					agent.Tools = payload.Tools
				}
				agent.mu.Unlock()
			}
			sendAgentMsg(conn, MsgHeartbeatAck, struct{}{})

		case MsgTaskAccepted:
			var payload TaskAcceptedPayload
			json.Unmarshal(msg.Payload, &payload)
			if agent != nil {
				agent.mu.Lock()
				agent.ActiveSessions[payload.SessionID] = true
				agent.mu.Unlock()
			}
			log.MessageF(log.ModuleAgent, "CodeGen: agent %s accepted task %s", agent.ID, payload.SessionID)

		case MsgTaskRejected:
			var payload TaskRejectedPayload
			json.Unmarshal(msg.Payload, &payload)
			log.WarnF(log.ModuleAgent, "CodeGen: agent %s rejected task %s: %s",
				agent.ID, payload.SessionID, payload.Reason)
			// 标记 session 错误
			if session := GetSession(payload.SessionID); session != nil {
				session.mu.Lock()
				session.Status = StatusError
				session.Error = "agent rejected: " + payload.Reason
				session.EndTime = time.Now()
				session.mu.Unlock()
				session.broadcast(StreamEvent{
					Type: "error",
					Text: "❌ Agent 拒绝任务: " + payload.Reason,
					Done: true,
				})
			}

		case MsgStreamEvent:
			var payload StreamEventPayload
			json.Unmarshal(msg.Payload, &payload)
			p.handleStreamEvent(&payload)

		case MsgTaskComplete:
			var payload TaskCompletePayload
			json.Unmarshal(msg.Payload, &payload)
			p.handleTaskComplete(agent, &payload)

		case MsgFileReadResp, MsgTreeReadResp, MsgProjectCreateResp:
			// 通用请求-响应：从 payload 提取 request_id，投递到等待的 channel
			var base struct {
				RequestID string `json:"request_id"`
			}
			json.Unmarshal(msg.Payload, &base)
			p.pendMu.Lock()
			if ch, ok := p.pending[base.RequestID]; ok {
				ch <- msg.Payload
				delete(p.pending, base.RequestID)
			}
			p.pendMu.Unlock()
		}
	}
}

// handleStreamEvent 收到 agent 的 stream_event，注入 session.broadcast()
func (p *AgentPool) handleStreamEvent(payload *StreamEventPayload) {
	session := GetSession(payload.SessionID)
	if session == nil {
		return
	}
	// 更新 session 状态（同 processEvent 逻辑）
	processEvent(session, &payload.Event)
	// Done 仅由 handleTaskComplete 触发，防止 result 事件提前关闭 WeChat 通知
	payload.Event.Done = false
	session.broadcast(payload.Event)
}

// handleTaskComplete 处理任务完成
func (p *AgentPool) handleTaskComplete(agent *RemoteAgent, payload *TaskCompletePayload) {
	if agent != nil {
		agent.mu.Lock()
		delete(agent.ActiveSessions, payload.SessionID)
		agent.mu.Unlock()
	}

	session := GetSession(payload.SessionID)
	if session == nil {
		return
	}

	session.mu.Lock()
	session.Status = payload.Status
	session.EndTime = time.Now()
	if payload.Error != "" {
		session.Error = payload.Error
	}
	session.mu.Unlock()

	if payload.Status == StatusError {
		session.broadcast(StreamEvent{
			Type: "error",
			Text: fmt.Sprintf("❌ 编码失败: %s", payload.Error),
			Done: true,
		})
	} else {
		session.broadcast(StreamEvent{
			Type:    "result",
			Text:    "✅ 编码完成",
			CostUSD: session.CostUSD,
			Done:    true,
		})
	}

	log.MessageF(log.ModuleAgent, "CodeGen: remote task %s completed, status=%s",
		payload.SessionID, payload.Status)
}

// SelectAgent 按负载选择可用 agent（支持项目和工具匹配）
func (p *AgentPool) SelectAgent(project, tool string) *RemoteAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *RemoteAgent
	bestLoad := -1

	for _, agent := range p.agents {
		agent.mu.Lock()
		if agent.Status == "offline" {
			agent.mu.Unlock()
			continue
		}
		active := len(agent.ActiveSessions)
		if active >= agent.MaxConcurrent {
			agent.mu.Unlock()
			continue
		}
		// 检查 agent 是否有该项目（project 为空时跳过匹配，适用于 pipeline 模式）
		hasProject := project == ""
		if !hasProject {
			for _, proj := range agent.Projects {
				if proj == project {
					hasProject = true
					break
				}
			}
		}
		// 如果 agent 没有上报项目列表，按 workspace 宽松匹配
		if !hasProject && len(agent.Projects) == 0 {
			hasProject = true
		}
		if !hasProject {
			agent.mu.Unlock()
			continue
		}
		// 检查 agent 是否支持指定的编码工具
		supportsTool := false
		if tool == "" || tool == ToolClaudeCode {
			// 默认工具，所有 agent 都支持
			supportsTool = true
		} else if len(agent.Tools) == 0 {
			// agent 未上报工具列表，仅支持 claudecode
			supportsTool = (tool == ToolClaudeCode)
		} else {
			for _, t := range agent.Tools {
				if t == tool {
					supportsTool = true
					break
				}
			}
		}
		if !supportsTool {
			agent.mu.Unlock()
			continue
		}
		// 选负载最低的
		available := agent.MaxConcurrent - active
		if best == nil || available > bestLoad {
			best = agent
			bestLoad = available
		}
		agent.mu.Unlock()
	}

	return best
}

// Execute 通过远程 agent 执行任务
func (p *AgentPool) Execute(session *CodeSession) error {
	// deploy_only 或 pipeline 模式直接路由到 deploy-agent
	tool := session.Tool
	if session.DeployOnly || session.Pipeline != "" {
		tool = ToolDeploy
	}

	var agent *RemoteAgent

	// 如果指定了 agentID，优先使用该 agent
	if session.AgentID != "" {
		p.mu.RLock()
		candidate := p.agents[session.AgentID]
		p.mu.RUnlock()
		if candidate != nil {
			candidate.mu.Lock()
			isOnline := candidate.Status != "offline"
			// pipeline 模式跳过项目匹配（project 是 pipeline 名称，不是实际项目名）
			hasProject := session.Pipeline != ""
			if !hasProject {
				for _, proj := range candidate.Projects {
					if proj == session.Project {
						hasProject = true
						break
					}
				}
			}
			candidate.mu.Unlock()
			if isOnline && hasProject {
				agent = candidate
			}
		}
	}

	// fallback 到负载均衡选择（pipeline 模式传空 project，匹配任意 deploy agent）
	if agent == nil {
		selectProject := session.Project
		if session.Pipeline != "" {
			selectProject = ""
		}
		agent = p.SelectAgent(selectProject, tool)
	}
	if agent == nil {
		if session.Pipeline != "" {
			return fmt.Errorf("no available deploy agent for pipeline '%s'", session.Pipeline)
		}
		if session.DeployOnly {
			return fmt.Errorf("no available deploy agent for project '%s'", session.Project)
		}
		if session.Tool != "" && session.Tool != ToolClaudeCode {
			return fmt.Errorf("no available agent supporting tool '%s'", session.Tool)
		}
		return fmt.Errorf("no available agent")
	}

	session.mu.Lock()
	session.AgentID = agent.ID
	session.mu.Unlock()

	return p.dispatchTask(agent, session, session.Prompt, "")
}

// ExecuteResume 通过远程 agent 恢复会话
func (p *AgentPool) ExecuteResume(session *CodeSession, prompt string) error {
	// 优先发给同一个 agent
	agentID := session.AgentID
	var agent *RemoteAgent
	if agentID != "" {
		p.mu.RLock()
		agent = p.agents[agentID]
		p.mu.RUnlock()
	}
	if agent == nil {
		agent = p.SelectAgent(session.Project, session.Tool)
	}
	if agent == nil {
		return fmt.Errorf("no available agent")
	}

	session.mu.Lock()
	session.AgentID = agent.ID
	session.mu.Unlock()

	return p.dispatchTask(agent, session, prompt, session.ClaudeSession)
}

// dispatchTask 发送 task_assign 给 agent
func (p *AgentPool) dispatchTask(agent *RemoteAgent, session *CodeSession, prompt, claudeSession string) error {
	tool := session.Tool
	if session.DeployOnly {
		tool = ToolDeploy
	} else if tool == "" {
		tool = ToolClaudeCode
	}

	log.MessageF(log.ModuleAgent, "CodeGen dispatchTask: session=%s, project=%s, tool=%s, model=%s",
		session.ID, session.Project, tool, session.Model)

	// deploy-agent 不需要 system prompt
	var systemPrompt string
	switch tool {
	case ToolDeploy:
		// deploy-agent 无需 system prompt
	case ToolOpenCode:
		systemPrompt = buildOpenCodeSystemPrompt()
	default:
		systemPrompt = buildClaudeCodeSystemPrompt()
	}

	payload := TaskAssignPayload{
		SessionID:     session.ID,
		Project:       session.Project,
		Prompt:        prompt,
		MaxTurns:      maxTurns,
		SystemPrompt:  systemPrompt,
		ClaudeSession: claudeSession,
		Model:         session.Model,
		Tool:          tool,
		AutoDeploy:    session.AutoDeploy,
		DeployOnly:    session.DeployOnly,
		DeployTarget:  session.DeployTarget,
		PackOnly:      session.PackOnly,
		Pipeline:      session.Pipeline,
	}

	return agent.Sender.SendAgentMsg(MsgTaskAssign, payload)
}

// buildClaudeCodeSystemPrompt 构建 Claude Code 系统提示
func buildClaudeCodeSystemPrompt() string {
	return config.GetPrompt(config.GetAdminAccount(), "claude_code_system")
}

// buildOpenCodeSystemPrompt 构建 OpenCode 系统提示
func buildOpenCodeSystemPrompt() string {
	return config.GetPrompt(config.GetAdminAccount(), "opencode_system")
}

// StopRemoteTask 发送 task_stop 给 agent
func (p *AgentPool) StopRemoteTask(session *CodeSession) error {
	if session.AgentID == "" {
		return fmt.Errorf("session has no agent")
	}

	p.mu.RLock()
	agent := p.agents[session.AgentID]
	p.mu.RUnlock()

	if agent == nil {
		return fmt.Errorf("agent not found: %s", session.AgentID)
	}

	return agent.Sender.SendAgentMsg(MsgTaskStop, TaskStopPayload{
		SessionID: session.ID,
	})
}

// GetAgents 获取所有 agent 信息（管理用）
func (p *AgentPool) GetAgents() []map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(p.agents))
	for _, agent := range p.agents {
		agent.mu.Lock()
		result = append(result, map[string]interface{}{
			"id":              agent.ID,
			"name":            agent.Name,
			"workspaces":      agent.Workspaces,
			"projects":        agent.Projects,
			"models":          agent.Models,
			"deploy_targets":  agent.DeployTargets,
			"host_platform":   agent.HostPlatform,
			"max_concurrent":  agent.MaxConcurrent,
			"active_sessions": len(agent.ActiveSessions),
			"last_heartbeat":  agent.LastHeartbeat.Format("2006-01-02 15:04:05"),
			"status":          agent.Status,
		})
		agent.mu.Unlock()
	}
	return result
}

// GetAllModels 聚合所有在线 agent 的 Models（兼容旧版）
func (p *AgentPool) GetAllModels() []string {
	return p.GetAllClaudeCodeModels()
}

// GetAllClaudeCodeModels 聚合所有在线 agent 的 ClaudeCode 模型配置
func (p *AgentPool) GetAllClaudeCodeModels() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	seen := make(map[string]bool)
	for _, agent := range p.agents {
		agent.mu.Lock()
		for _, m := range agent.ClaudeCodeModels {
			seen[m] = true
		}
		agent.mu.Unlock()
	}

	models := make([]string, 0, len(seen))
	for m := range seen {
		models = append(models, m)
	}

	sort.Strings(models)
	return models
}

// GetAllOpenCodeModels 聚合所有在线 agent 的 OpenCode 模型配置
func (p *AgentPool) GetAllOpenCodeModels() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	seen := make(map[string]bool)
	for _, agent := range p.agents {
		agent.mu.Lock()
		for _, m := range agent.OpenCodeModels {
			seen[m] = true
		}
		agent.mu.Unlock()
	}

	models := make([]string, 0, len(seen))
	for m := range seen {
		models = append(models, m)
	}

	sort.Strings(models)
	return models
}

// GetAllTools 聚合所有在线 agent 的编码工具，去重排序
func (p *AgentPool) GetAllTools() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	seen := make(map[string]bool)
	// 默认始终包含 claudecode
	seen[ToolClaudeCode] = true
	for _, agent := range p.agents {
		agent.mu.Lock()
		for _, t := range agent.Tools {
			seen[t] = true
		}
		agent.mu.Unlock()
	}

	tools := make([]string, 0, len(seen))
	for t := range seen {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	return tools
}

// PipelineInfo pipeline 信息（聚合自 deploy agent 上报）
type PipelineInfo struct {
	Name    string
	Agent   string
	AgentID string
}

// ListPipelines 聚合所有在线 deploy agent 的 pipeline 列表
func (p *AgentPool) ListPipelines() []PipelineInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []PipelineInfo
	seen := make(map[string]bool)
	for _, agent := range p.agents {
		agent.mu.Lock()
		for _, pip := range agent.Pipelines {
			if !seen[pip] {
				seen[pip] = true
				result = append(result, PipelineInfo{Name: pip, Agent: agent.Name, AgentID: agent.ID})
			}
		}
		agent.mu.Unlock()
	}
	return result
}

// AgentSupportsTool 检查 agent 是否支持指定工具
func (a *RemoteAgent) AgentSupportsTool(tool string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	// 如果 agent 没有上报 tools 列表，默认支持 claudecode
	if len(a.Tools) == 0 {
		return tool == ToolClaudeCode || tool == ""
	}
	for _, t := range a.Tools {
		if t == tool {
			return true
		}
	}
	return false
}

// ListRemoteProjects 获取所有远程 agent 上报的项目（每个 agent-项目 组合独立输出）
func (p *AgentPool) ListRemoteProjects() []RemoteProjectInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []RemoteProjectInfo

	for _, agent := range p.agents {
		agent.mu.Lock()
		agentTools := agent.Tools
		agentID := agent.ID
		agentName := agent.Name
		projects := agent.Projects
		deployTargets := agent.DeployTargets
		hostPlatform := agent.HostPlatform
		pipelines := agent.Pipelines
		agent.mu.Unlock()

		for _, proj := range projects {
			tools := make([]string, 0)
			if len(agentTools) == 0 {
				tools = append(tools, ToolClaudeCode)
			} else {
				tools = append(tools, agentTools...)
			}
			sort.Strings(tools)
			info := RemoteProjectInfo{
				Name:    proj,
				AgentID: agentID,
				Agent:   agentName,
				Tools:   tools,
			}
			// deploy 类 agent 附加部署信息
			for _, t := range tools {
				if t == ToolDeploy {
					info.DeployTargets = deployTargets
					info.HostPlatform = hostPlatform
					info.Pipelines = pipelines
					break
				}
			}
			result = append(result, info)
		}
	}

	// 按项目名排序，同名按 agent 名排序
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Agent < result[j].Agent
	})

	return result
}

// RemoteProjectInfo 远程项目信息
type RemoteProjectInfo struct {
	Name          string   `json:"name"`
	AgentID       string   `json:"agent_id"`
	Agent         string   `json:"agent"`
	Tools         []string `json:"tools"`                    // 该项目支持的工具列表，如 ["claudecode"], ["deploy"], 或 ["claudecode","deploy"]
	DeployTargets []string `json:"deploy_targets,omitempty"` // deploy 项目的可用部署目标
	HostPlatform  string   `json:"host_platform,omitempty"`  // deploy agent 的主机平台
	Pipelines     []string `json:"pipelines,omitempty"`      // deploy agent 的可用 pipeline
}

// FindAgentForProject 查找拥有指定项目的 agent
func (p *AgentPool) FindAgentForProject(project string) *RemoteAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, agent := range p.agents {
		agent.mu.Lock()
		for _, proj := range agent.Projects {
			if proj == project {
				agent.mu.Unlock()
				return agent
			}
		}
		agent.mu.Unlock()
	}
	return nil
}

// ReadRemoteFile 通过 agent 读取远程项目文件
func (p *AgentPool) ReadRemoteFile(project, path string) (string, error) {
	agent := p.FindAgentForProject(project)
	if agent == nil {
		return "", fmt.Errorf("no agent has project: %s", project)
	}

	reqID := fmt.Sprintf("fr_%d", time.Now().UnixNano())
	ch := make(chan json.RawMessage, 1)
	p.pendMu.Lock()
	p.pending[reqID] = ch
	p.pendMu.Unlock()

	// 超时清理
	defer func() {
		p.pendMu.Lock()
		delete(p.pending, reqID)
		p.pendMu.Unlock()
	}()

	err := agent.Sender.SendAgentMsg(MsgFileRead, FileReadPayload{
		RequestID: reqID,
		Project:   project,
		Path:      path,
	})
	if err != nil {
		return "", err
	}

	select {
	case raw := <-ch:
		var resp FileReadRespPayload
		json.Unmarshal(raw, &resp)
		if resp.Error != "" {
			return "", fmt.Errorf("%s", resp.Error)
		}
		return resp.Content, nil
	case <-time.After(10 * time.Second):
		return "", fmt.Errorf("timeout reading file from agent")
	}
}

// ReadRemoteTree 通过 agent 读取远程项目目录树
func (p *AgentPool) ReadRemoteTree(project string, maxDepth int) (*DirNode, error) {
	agent := p.FindAgentForProject(project)
	if agent == nil {
		return nil, fmt.Errorf("no agent has project: %s", project)
	}

	reqID := fmt.Sprintf("tr_%d", time.Now().UnixNano())
	ch := make(chan json.RawMessage, 1)
	p.pendMu.Lock()
	p.pending[reqID] = ch
	p.pendMu.Unlock()

	defer func() {
		p.pendMu.Lock()
		delete(p.pending, reqID)
		p.pendMu.Unlock()
	}()

	err := agent.Sender.SendAgentMsg(MsgTreeRead, TreeReadPayload{
		RequestID: reqID,
		Project:   project,
		MaxDepth:  maxDepth,
	})
	if err != nil {
		return nil, err
	}

	select {
	case raw := <-ch:
		var resp TreeReadRespPayload
		json.Unmarshal(raw, &resp)
		if resp.Error != "" {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		return resp.Tree, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout reading tree from agent")
	}
}

// HasAgents 是否有可用 agent
func (p *AgentPool) HasAgents() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents) > 0
}

// CreateRemoteProject 通过 agent 在远程机器上创建项目
func (p *AgentPool) CreateRemoteProject(agentName, projectName string) error {
	agent := p.findOnlineAgentByName(agentName)
	if agent == nil {
		return fmt.Errorf("agent '%s' 不在线", agentName)
	}

	reqID := fmt.Sprintf("pc_%d", time.Now().UnixNano())
	ch := make(chan json.RawMessage, 1)
	p.pendMu.Lock()
	p.pending[reqID] = ch
	p.pendMu.Unlock()

	defer func() {
		p.pendMu.Lock()
		delete(p.pending, reqID)
		p.pendMu.Unlock()
	}()

	err := agent.Sender.SendAgentMsg(MsgProjectCreate, ProjectCreatePayload{
		RequestID: reqID,
		Name:      projectName,
	})
	if err != nil {
		return fmt.Errorf("发送创建请求失败: %v", err)
	}

	select {
	case raw := <-ch:
		var resp ProjectCreateRespPayload
		json.Unmarshal(raw, &resp)
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}
		// 创建成功，更新 agent 的项目列表
		agent.mu.Lock()
		agent.Projects = append(agent.Projects, projectName)
		agent.mu.Unlock()
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("等待 agent 响应超时")
	}
}

// FindAgentByName 按名称查找在线 agent（公开方法）
func (p *AgentPool) FindAgentByName(name string) *RemoteAgent {
	return p.findOnlineAgentByName(name)
}

// GetAgentNames 获取所有在线 agent 名称
func (p *AgentPool) GetAgentNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.agents))
	for _, agent := range p.agents {
		agent.mu.Lock()
		if agent.Status != "offline" {
			names = append(names, agent.Name)
		}
		agent.mu.Unlock()
	}
	return names
}

// addAgent 添加 agent
func (p *AgentPool) addAgent(agent *RemoteAgent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 如果已有同 ID 的旧连接，关闭它
	if old, ok := p.agents[agent.ID]; ok {
		if old.Conn != nil {
			old.Conn.Close()
		}
	}
	p.agents[agent.ID] = agent
}

// findOnlineAgentByName 查找同名在线 agent
func (p *AgentPool) findOnlineAgentByName(name string) *RemoteAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, agent := range p.agents {
		agent.mu.Lock()
		if agent.Name == name && agent.Status != "offline" {
			agent.mu.Unlock()
			return agent
		}
		agent.mu.Unlock()
	}
	return nil
}

// removeAgent 移除 agent，并将其活跃 session 标记为 error
func (p *AgentPool) removeAgent(agentID string) {
	p.mu.Lock()
	agent, ok := p.agents[agentID]
	if !ok {
		p.mu.Unlock()
		return
	}

	agent.mu.Lock()
	agent.Status = "offline"
	// 收集活跃 session ID
	activeSessions := make([]string, 0, len(agent.ActiveSessions))
	for sid := range agent.ActiveSessions {
		activeSessions = append(activeSessions, sid)
	}
	agent.mu.Unlock()

	delete(p.agents, agentID)
	p.mu.Unlock()

	log.MessageF(log.ModuleAgent, "CodeGen: agent disconnected: %s, active sessions: %d", agentID, len(activeSessions))

	// 通知所有活跃 session：agent 已离线
	for _, sid := range activeSessions {
		if session := GetSession(sid); session != nil {
			session.mu.Lock()
			if session.Status == StatusRunning {
				session.Status = StatusError
				session.Error = "agent disconnected: " + agentID
				session.EndTime = time.Now()
				session.mu.Unlock()
				session.broadcast(StreamEvent{
					Type: "error",
					Text: "Agent 已离线，任务中断",
					Done: true,
				})
			} else {
				session.mu.Unlock()
			}
		}
	}
}

// CleanupLoop 定时清理超时 agent（45s 无心跳）
func (p *AgentPool) CleanupLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		p.cleanupStaleAgents()
	}
}

func (p *AgentPool) cleanupStaleAgents() {
	// 先收集超时的 agent ID
	p.mu.RLock()
	var staleIDs []string
	now := time.Now()
	for id, agent := range p.agents {
		agent.mu.Lock()
		if now.Sub(agent.LastHeartbeat) > 45*time.Second {
			staleIDs = append(staleIDs, id)
		}
		agent.mu.Unlock()
	}
	p.mu.RUnlock()

	// 逐个移除（removeAgent 内部会加锁并通知 session）
	for _, id := range staleIDs {
		log.WarnF(log.ModuleAgent, "CodeGen: agent %s timed out", id)
		p.removeAgent(id)
	}
}

// sendAgentMsg 直连模式：发送消息给 agent WebSocket（仅 HandleAgentWebSocket 使用）
func sendAgentMsg(conn *websocket.Conn, msgType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := AgentMessage{
		Type:    msgType,
		Payload: json.RawMessage(data),
		Ts:      time.Now().UnixMilli(),
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, msgData)
}
