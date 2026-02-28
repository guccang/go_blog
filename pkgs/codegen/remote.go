package codegen

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RemoteAgent 远程 Agent 连接
type RemoteAgent struct {
	ID               string
	Name             string
	Conn             *websocket.Conn
	Workspaces       []string
	Projects         []string // agent 上报的可用项目
	Models           []string // agent 支持的模型配置列表（兼容旧版）
	ClaudeCodeModels []string // Claude Code 模型配置
	OpenCodeModels   []string // OpenCode 模型配置
	Tools            []string // agent 支持的编码工具列表 (claudecode, opencode)
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
				Conn:             conn,
				Workspaces:       payload.Workspaces,
				Projects:         payload.Projects,
				Models:           payload.Models,
				ClaudeCodeModels: payload.ClaudeCodeModels,
				OpenCodeModels:   payload.OpenCodeModels,
				Tools:            payload.Tools,
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
		// 检查 agent 是否有该项目
		hasProject := false
		for _, proj := range agent.Projects {
			if proj == project {
				hasProject = true
				break
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
	agent := p.SelectAgent(session.Project, session.Tool)
	if agent == nil {
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
	if tool == "" {
		tool = ToolClaudeCode
	}

	log.MessageF(log.ModuleAgent, "CodeGen dispatchTask: session=%s, project=%s, tool=%s, model=%s",
		session.ID, session.Project, tool, session.Model)

	var systemPrompt string
	if tool == ToolOpenCode {
		systemPrompt = buildOpenCodeSystemPrompt()
	} else {
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
	}

	return sendAgentMsg(agent.Conn, MsgTaskAssign, payload)
}

// buildClaudeCodeSystemPrompt 构建 Claude Code 系统提示
func buildClaudeCodeSystemPrompt() string {
	return "重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，" +
		"禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。" +
		"你必须完成完整的开发流程：" +
		"1. 编写代码；" +
		"2. 构建/编译项目（如 go build、npm run build 等），确认无编译错误；" +
		"3. 运行程序并验证输出正确；" +
		"4. 如有测试则运行测试；" +
		"5. 最后汇报结果：创建了哪些文件、构建是否成功、运行输出是什么。" +
		"不要只写代码就停止，必须验证代码能正常工作。" +
		"绝对禁止使用 AskUserQuestion 工具或任何需要用户交互的操作。" +
		"你在无人值守的自动化环境中运行，没有人可以回答你的问题。" +
		"遇到不确定的地方自己做出最合理的决定，不要询问用户。" +
		"不要进入 plan mode，不要使用 EnterPlanMode，直接执行任务。"
}

// buildOpenCodeSystemPrompt 构建 OpenCode 系统提示
func buildOpenCodeSystemPrompt() string {
	return "重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，" +
		"禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。" +
		"你必须完成完整的开发流程：" +
		"1. 编写代码；" +
		"2. 构建/编译项目（如 go build、npm run build 等），确认无编译错误；" +
		"3. 运行程序并验证输出正确；" +
		"4. 如有测试则运行测试；" +
		"5. 最后汇报结果：创建了哪些文件、构建是否成功、运行输出是什么。" +
		"不要只写代码就停止，必须验证代码能正常工作。" +
		"你在无人值守的自动化环境中运行，没有人可以回答你的问题。" +
		"遇到不确定的地方自己做出最合理的决定，不要询问用户。" +
		"直接执行任务，不要进行多余的交互式确认。"
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

	return sendAgentMsg(agent.Conn, MsgTaskStop, TaskStopPayload{
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

// ListRemoteProjects 获取所有远程 agent 上报的项目（去重，附带 agent 信息）
func (p *AgentPool) ListRemoteProjects() []RemoteProjectInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	seen := make(map[string]bool)
	var result []RemoteProjectInfo
	for _, agent := range p.agents {
		agent.mu.Lock()
		for _, proj := range agent.Projects {
			if !seen[proj] {
				seen[proj] = true
				result = append(result, RemoteProjectInfo{
					Name:    proj,
					AgentID: agent.ID,
					Agent:   agent.Name,
				})
			}
		}
		agent.mu.Unlock()
	}
	return result
}

// RemoteProjectInfo 远程项目信息
type RemoteProjectInfo struct {
	Name    string `json:"name"`
	AgentID string `json:"agent_id"`
	Agent   string `json:"agent"`
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

	err := sendAgentMsg(agent.Conn, MsgFileRead, FileReadPayload{
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

	err := sendAgentMsg(agent.Conn, MsgTreeRead, TreeReadPayload{
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

	err := sendAgentMsg(agent.Conn, MsgProjectCreate, ProjectCreatePayload{
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
		old.Conn.Close()
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

// removeAgent 移除 agent
func (p *AgentPool) removeAgent(agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if agent, ok := p.agents[agentID]; ok {
		agent.mu.Lock()
		agent.Status = "offline"
		agent.mu.Unlock()
		delete(p.agents, agentID)
		log.MessageF(log.ModuleAgent, "CodeGen: agent disconnected: %s", agentID)
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
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for id, agent := range p.agents {
		agent.mu.Lock()
		if now.Sub(agent.LastHeartbeat) > 45*time.Second {
			agent.Status = "offline"
			agent.Conn.Close()
			delete(p.agents, id)
			log.WarnF(log.ModuleAgent, "CodeGen: agent %s timed out, removed", id)
		}
		agent.mu.Unlock()
	}
}

// sendAgentMsg 发送消息给 agent
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
