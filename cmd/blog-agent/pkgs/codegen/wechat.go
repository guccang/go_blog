package codegen

import (
	"encoding/json"
	"fmt"
	log "mylog"
	"strings"
	"sync"
	"time"
)

// SendFunc 微信消息发送函数（由外部注入，避免循环依赖）
type SendFunc func(toUser, content string) error

// WeChatBridge 微信桥接层：将 codegen 流式事件中继为微信消息
type WeChatBridge struct {
	sendMsg      SendFunc
	userSessions map[string]*UserSessionState // wechatUser → state
	mu           sync.RWMutex
}

// UserSessionState 用户会话状态
type UserSessionState struct {
	UserID      string
	SessionID   string
	Project     string
	LastNotify  time.Time
	EventBuffer []string // 只存工具步骤摘要
	StepCount   int      // 已执行步骤总数
	stopCh      chan struct{}
	mu          sync.Mutex
}

// 全局桥接实例
var wechatBridge *WeChatBridge

// GetSessionUser 根据 sessionID 查找关联的微信用户
func GetSessionUser(sessionID string) string {
	if wechatBridge == nil {
		return ""
	}
	wechatBridge.mu.RLock()
	defer wechatBridge.mu.RUnlock()
	for _, state := range wechatBridge.userSessions {
		if state.SessionID == sessionID {
			return state.UserID
		}
	}
	return ""
}

// GetUserSessionID 根据用户查找当前关联的会话。
func GetUserSessionID(userID string) string {
	if wechatBridge == nil {
		return ""
	}
	wechatBridge.mu.RLock()
	defer wechatBridge.mu.RUnlock()
	if state := wechatBridge.userSessions[userID]; state != nil {
		return state.SessionID
	}
	return ""
}

// InitWeChatBridge 初始化微信桥接，注入发送函数
func InitWeChatBridge(sender SendFunc) {
	wechatBridge = &WeChatBridge{
		sendMsg:      sender,
		userSessions: make(map[string]*UserSessionState),
	}
	log.Message(log.ModuleAgent, "CodeGen WeChat bridge initialized")
}

// StartSessionForWeChat 启动编码会话并订阅通知
func StartSessionForWeChat(userID, project, prompt, model, tool, agentID string, deployOpts ...bool) (string, error) {
	if wechatBridge == nil {
		return "", fmt.Errorf("WeChat bridge not initialized")
	}

	// 解析部署选项: deployOpts[0]=autoDeploy, deployOpts[1]=deployOnly
	autoDeploy := false
	deployOnly := false
	if len(deployOpts) > 0 {
		autoDeploy = deployOpts[0]
	}
	if len(deployOpts) > 1 {
		deployOnly = deployOpts[1]
	}

	// 检查用户是否已有运行中的会话
	wechatBridge.mu.RLock()
	existing := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()
	if existing != nil {
		s := GetSession(existing.SessionID)
		if s != nil && s.Status == StatusRunning {
			return "", fmt.Errorf("你已有运行中的编码会话（项目: %s），请等待完成或先停止", existing.Project)
		}
	}

	// 启动会话
	session, err := StartSession(project, prompt, model, tool, agentID, autoDeploy, deployOnly, "", false, "")
	if err != nil {
		return "", err
	}

	// 创建用户状态
	state := &UserSessionState{
		UserID:      userID,
		SessionID:   session.ID,
		Project:     project,
		LastNotify:  time.Now(),
		EventBuffer: make([]string, 0),
		stopCh:      make(chan struct{}),
	}

	wechatBridge.mu.Lock()
	wechatBridge.userSessions[userID] = state
	wechatBridge.mu.Unlock()

	// 启动后台通知 goroutine
	go subscribeAndRelay(state, session)

	return session.ID, nil
}

// SendMessageForWeChat 向活跃编码会话追加消息
func SendMessageForWeChat(userID, prompt string) (string, error) {
	if wechatBridge == nil {
		return "", fmt.Errorf("WeChat bridge not initialized")
	}

	wechatBridge.mu.RLock()
	state := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()

	if state == nil {
		return "", fmt.Errorf("没有活跃的编码会话，请先启动一个会话")
	}

	session := GetSession(state.SessionID)
	if session == nil {
		return "", fmt.Errorf("会话已过期，请重新启动")
	}

	// 停掉旧的通知 goroutine
	close(state.stopCh)

	// 追加消息
	if err := SendMessage(state.SessionID, prompt); err != nil {
		return "", err
	}

	// 重新创建状态并订阅
	newState := &UserSessionState{
		UserID:      userID,
		SessionID:   state.SessionID,
		Project:     state.Project,
		LastNotify:  time.Now(),
		EventBuffer: make([]string, 0),
		stopCh:      make(chan struct{}),
	}
	wechatBridge.mu.Lock()
	wechatBridge.userSessions[userID] = newState
	wechatBridge.mu.Unlock()

	go subscribeAndRelay(newState, session)

	return state.SessionID, nil
}

// GetStatusForWeChat 获取用户当前编码会话状态
func GetStatusForWeChat(userID string) string {
	if wechatBridge == nil {
		return "编码桥接未初始化"
	}

	wechatBridge.mu.RLock()
	state := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()

	if state == nil {
		return "当前没有活跃的编码会话"
	}

	session := GetSession(state.SessionID)
	if session == nil {
		return "会话已过期"
	}

	session.mu.Lock()
	status := session.Status
	project := session.Project
	startTime := session.StartTime
	cost := session.CostUSD
	session.mu.Unlock()

	elapsed := time.Since(startTime).Round(time.Second)
	statusText := "未知"
	switch status {
	case StatusRunning:
		statusText = "运行中"
	case StatusDone:
		statusText = "已完成"
	case StatusError:
		statusText = "出错"
	case StatusStopped:
		statusText = "已停止"
	}

	return fmt.Sprintf("项目: %s\n状态: %s\n耗时: %s\n费用: $%.4f\n会话ID: %s",
		project, statusText, elapsed, cost, state.SessionID)
}

// StopSessionForWeChat 停止用户当前编码会话 + 所有远程 agent 上的运行中任务
func StopSessionForWeChat(userID string) (string, error) {
	if wechatBridge == nil {
		return "", fmt.Errorf("WeChat bridge not initialized")
	}

	var stoppedSessionID string

	// 停止用户自己的跟踪会话（通过 cg start 启动的）
	wechatBridge.mu.RLock()
	state := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()

	if state != nil {
		stoppedSessionID = state.SessionID

		// 停止通知 goroutine
		select {
		case <-state.stopCh:
		default:
			close(state.stopCh)
		}

		// 停止该会话
		StopSession(state.SessionID)

		// 清理用户状态
		wechatBridge.mu.Lock()
		delete(wechatBridge.userSessions, userID)
		wechatBridge.mu.Unlock()
	}

	// 停止所有运行中的会话（包括 llm-agent 直接派发到 codegen-agent 的任务）
	stopped := StopAllSessions()

	if stoppedSessionID != "" {
		return stoppedSessionID, nil
	}
	if stopped > 0 {
		return fmt.Sprintf("all(%d)", stopped), nil
	}
	return "", fmt.Errorf("当前没有运行中的编码会话")
}

// subscribeAndRelay 后台 goroutine：订阅会话事件并中继到微信
func subscribeAndRelay(state *UserSessionState, session *CodeSession) {
	ch := session.Subscribe()
	defer session.Unsubscribe(ch)

	// 订阅后立即检查会话是否已结束（防止订阅前事件已广播的竞态）
	session.mu.Lock()
	status := session.Status
	session.mu.Unlock()
	if status == StatusError || status == StatusDone || status == StatusStopped {
		sendCompletionSummary(state, session, StreamEvent{Done: true})
		// 不删除 userSessions 映射，保留供 cg send 续接使用
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-state.stopCh:
			// 被外部停止
			return

		case event, ok := <-ch:
			if !ok {
				// 通道关闭
				return
			}

			if event.Done {
				// 会话结束，先刷新缓冲区再发送完成摘要
				flushBuffer(state)
				sendCompletionSummary(state, session, event)
				// 不删除 userSessions 映射，保留供 cg send 续接使用
				return
			}

			// 缓冲事件（只保留工具步骤）
			text := formatEventForWeChat(event)
			if text != "" {
				state.mu.Lock()
				state.StepCount++
				state.EventBuffer = append(state.EventBuffer, text)
				state.mu.Unlock()
			}

		case <-ticker.C:
			// 定期刷新缓冲区
			elapsed := time.Since(startTime)
			if elapsed > 5*time.Minute {
				// 超过 5 分钟降频到 30 秒
				ticker.Reset(30 * time.Second)
			}
			flushBuffer(state)
		}
	}
}

// flushBuffer 合并缓冲区的工具步骤，发送精简进度消息
func flushBuffer(state *UserSessionState) {
	state.mu.Lock()
	if len(state.EventBuffer) == 0 {
		state.mu.Unlock()
		return
	}
	steps := make([]string, len(state.EventBuffer))
	copy(steps, state.EventBuffer)
	state.EventBuffer = state.EventBuffer[:0]
	totalSteps := state.StepCount
	state.mu.Unlock()

	// 只保留最近 8 条步骤，避免消息过长
	if len(steps) > 8 {
		steps = steps[len(steps)-8:]
	}

	elapsed := time.Since(state.LastNotify).Round(time.Second)
	content := fmt.Sprintf("⚙️ %s · 第%d步 · %s\n\n%s",
		state.Project, totalSteps, elapsed, strings.Join(steps, "\n"))

	if wechatBridge != nil && wechatBridge.sendMsg != nil {
		if err := wechatBridge.sendMsg(state.UserID, content); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen WeChat notify failed: %v", err)
		}
	}

	state.mu.Lock()
	state.LastNotify = time.Now()
	state.mu.Unlock()
}

// sendCompletionSummary 发送完成摘要（精简版，含修改内容简介）
func sendCompletionSummary(state *UserSessionState, session *CodeSession, event StreamEvent) {
	session.mu.Lock()
	status := session.Status
	cost := session.CostUSD
	startTime := session.StartTime
	endTime := session.EndTime
	errMsg := session.Error
	msgs := make([]SessionMessage, len(session.Messages))
	copy(msgs, session.Messages)
	session.mu.Unlock()

	state.mu.Lock()
	totalSteps := state.StepCount
	state.mu.Unlock()

	elapsed := formatDuration(startTime, endTime)

	// 提取最后一条 result/assistant 消息作为修改内容摘要
	changeSummary := extractChangeSummary(msgs)

	// 构建状态行：耗时 · 步数 [· 费用]
	statsLine := fmt.Sprintf("%s · %d步", elapsed, totalSteps)
	if cost > 0 {
		statsLine += fmt.Sprintf(" · $%.4f", cost)
	}

	var summary string
	switch status {
	case StatusError:
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		summary = fmt.Sprintf("❌ %s 编码失败\n%s\n\n%s",
			state.Project, statsLine, errMsg)
	case StatusStopped:
		summary = fmt.Sprintf("⏹ %s 已停止\n%s",
			state.Project, statsLine)
	default:
		summary = fmt.Sprintf("✅ %s 编码完成\n%s",
			state.Project, statsLine)
	}

	if changeSummary != "" {
		summary += "\n\n" + changeSummary
	}

	if wechatBridge != nil && wechatBridge.sendMsg != nil {
		if err := wechatBridge.sendMsg(state.UserID, summary); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen WeChat completion notify failed: %v", err)
		}
	}
}

// extractChangeSummary 从会话消息中提取修改内容摘要
// 只在最后一次 user 消息之后的范围内查找，避免 cg send 续接时使用上一轮的结果
// 优先级: result（Claude Code 最终结果）> summary（agent 报告）> assistant
func extractChangeSummary(msgs []SessionMessage) string {
	// 找到最后一条 user 消息的位置，只在其之后搜索
	lastUserIdx := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}

	// 优先从 result 消息提取（Claude Code 的最终结果文本）
	for i := len(msgs) - 1; i >= lastUserIdx; i-- {
		if msgs[i].Role == "result" && strings.TrimSpace(msgs[i].Content) != "" {
			return truncateForWeChat(strings.TrimSpace(msgs[i].Content))
		}
	}
	// 再尝试 summary（agent 生成的任务报告）
	for i := len(msgs) - 1; i >= lastUserIdx; i-- {
		if msgs[i].Role == "summary" && strings.TrimSpace(msgs[i].Content) != "" {
			return truncateForWeChat(strings.TrimSpace(msgs[i].Content))
		}
	}
	// 最后回退到 assistant 消息
	for i := len(msgs) - 1; i >= lastUserIdx; i-- {
		if msgs[i].Role == "assistant" && strings.TrimSpace(msgs[i].Content) != "" {
			return truncateForWeChat(strings.TrimSpace(msgs[i].Content))
		}
	}
	return ""
}

// truncateForWeChat 为微信消息截断文本
// 保留换行（微信支持），折叠多余空行，限制 800 rune
func truncateForWeChat(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// 折叠连续多个空行为两个换行
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	runes := []rune(s)
	if len(runes) <= 800 {
		return string(runes)
	}
	return string(runes[:800]) + "\n..."
}

// formatDuration 格式化耗时，处理零值和负值
func formatDuration(startTime, endTime time.Time) string {
	if endTime.IsZero() {
		endTime = time.Now()
	}
	d := endTime.Sub(startTime)
	if d < 0 {
		d = time.Since(startTime)
	}
	d = d.Round(time.Second)

	if d >= time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if d >= time.Minute {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// formatEventForWeChat 格式化流式事件为微信可读文本
// 只保留工具操作步骤，丢弃 assistant/thinking 思考文本和 system 噪音
func formatEventForWeChat(event StreamEvent) string {
	switch event.Type {
	case "tool":
		// 工具操作步骤（已由 formatToolAction 格式化为简短描述）
		return event.Text
	case "error":
		return "⚠️ " + event.Text
	default:
		// assistant / system 等文本不推送
		return ""
	}
}

// ListProjectsJSON 列出项目并返回 JSON（供 MCP 工具调用）
func ListProjectsJSON() string {
	result := map[string]interface{}{
		"success": true,
	}

	// 远程 agent 项目
	if agentPool != nil {
		remoteProjects := agentPool.ListRemoteProjects()
		result["remote_projects"] = remoteProjects
	} else {
		result["remote_projects"] = []interface{}{}
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// ListDeployProjectsJSON 列出支持部署的项目并返回 JSON（供 MCP 工具调用）
func ListDeployProjectsJSON() string {
	result := map[string]interface{}{
		"success": true,
	}

	if agentPool != nil {
		allProjects := agentPool.ListRemoteProjects()
		deployProjects := make([]RemoteProjectInfo, 0)
		for _, p := range allProjects {
			for _, t := range p.Tools {
				if t == ToolDeploy {
					deployProjects = append(deployProjects, p)
					break
				}
			}
		}
		result["deploy_projects"] = deployProjects
	} else {
		result["deploy_projects"] = []interface{}{}
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// StartDeployJSON 启动部署会话并返回 JSON（供 MCP 工具调用）
func StartDeployJSON(account, project, deployTarget, port string) string {
	if wechatBridge == nil {
		return `{"success":false,"error":"WeChat bridge not initialized"}`
	}

	// 如果指定了 port，将其附加到 deployTarget 信息中
	// deploy-agent 通过 deployTarget 和项目配置确定端口
	effectiveTarget := deployTarget

	sessionID, err := StartDeployForWeChat(account, project, "", effectiveTarget, false, port)
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
	}

	result := map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"message":    "部署会话已启动，进度将通过当前客户端推送",
	}
	if deployTarget != "" {
		result["deploy_target"] = deployTarget
	}
	if port != "" {
		result["port"] = port
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// StartDeployForWeChat 启动部署会话并订阅通知（支持 target/packOnly/port）
func StartDeployForWeChat(userID, project, agentID, deployTarget string, packOnly bool, port ...string) (string, error) {
	if wechatBridge == nil {
		return "", fmt.Errorf("WeChat bridge not initialized")
	}

	// 检查用户是否已有运行中的会话
	wechatBridge.mu.RLock()
	existing := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()
	if existing != nil {
		s := GetSession(existing.SessionID)
		if s != nil && s.Status == StatusRunning {
			return "", fmt.Errorf("你已有运行中的编码会话（项目: %s），请等待完成或先停止", existing.Project)
		}
	}

	// 启动会话
	deployPort := ""
	if len(port) > 0 {
		deployPort = port[0]
	}
	session, err := StartSession(project, "", "", "", agentID, false, true, deployTarget, packOnly, "", deployPort)
	if err != nil {
		return "", err
	}

	// 创建用户状态
	state := &UserSessionState{
		UserID:      userID,
		SessionID:   session.ID,
		Project:     project,
		LastNotify:  time.Now(),
		EventBuffer: make([]string, 0),
		stopCh:      make(chan struct{}),
	}

	wechatBridge.mu.Lock()
	wechatBridge.userSessions[userID] = state
	wechatBridge.mu.Unlock()

	go subscribeAndRelay(state, session)

	return session.ID, nil
}

// StartPipelineForWeChat 启动 pipeline 会话并订阅通知
func StartPipelineForWeChat(userID, pipeline, agentID string) (string, error) {
	if wechatBridge == nil {
		return "", fmt.Errorf("WeChat bridge not initialized")
	}

	// 检查用户是否已有运行中的会话
	wechatBridge.mu.RLock()
	existing := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()
	if existing != nil {
		s := GetSession(existing.SessionID)
		if s != nil && s.Status == StatusRunning {
			return "", fmt.Errorf("你已有运行中的编码会话（项目: %s），请等待完成或先停止", existing.Project)
		}
	}

	// pipeline 模式：project 用 pipeline 名称标识，prompt 留空
	session, err := StartSession(pipeline, "pipeline", "", "", agentID, false, true, "", false, pipeline)
	if err != nil {
		return "", err
	}

	// 创建用户状态
	state := &UserSessionState{
		UserID:      userID,
		SessionID:   session.ID,
		Project:     "pipeline:" + pipeline,
		LastNotify:  time.Now(),
		EventBuffer: make([]string, 0),
		stopCh:      make(chan struct{}),
	}

	wechatBridge.mu.Lock()
	wechatBridge.userSessions[userID] = state
	wechatBridge.mu.Unlock()

	go subscribeAndRelay(state, session)

	return session.ID, nil
}

// StartPipelineJSON 启动 pipeline 会话并返回 JSON（供 MCP 工具调用）
func StartPipelineJSON(account, pipeline string) string {
	if wechatBridge == nil {
		return `{"success":false,"error":"WeChat bridge not initialized"}`
	}

	sessionID, err := StartPipelineForWeChat(account, pipeline, "")
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
	}
	return fmt.Sprintf(`{"success":true,"session_id":"%s","message":"Pipeline %s 已启动，进度将通过当前客户端推送"}`, sessionID, pipeline)
}

// CreateProjectJSON 在远程 agent 上创建项目并返回 JSON
func CreateProjectJSON(agentName, name string) string {
	pool := agentPool
	if pool == nil {
		return `{"success":false,"error":"远程 agent 模式未启用"}`
	}

	// 若未指定 agent，自动选择第一个在线 agent
	if agentName == "" {
		names := pool.GetAgentNames()
		if len(names) == 0 {
			return `{"success":false,"error":"无在线编码agent。请确保编码agent已连接，或用 agent 参数指定编码机器名称（如 win、mac）。注意：编码agent不是部署目标服务器"}`
		}
		agentName = names[0]
	}

	if err := pool.CreateRemoteProject(agentName, name); err != nil {
		return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
	}
	return fmt.Sprintf(`{"success":true,"message":"项目 %s 已在 agent %s 上创建"}`, name, agentName)
}
