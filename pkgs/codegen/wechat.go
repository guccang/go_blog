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

// InitWeChatBridge 初始化微信桥接，注入发送函数
func InitWeChatBridge(sender SendFunc) {
	wechatBridge = &WeChatBridge{
		sendMsg:      sender,
		userSessions: make(map[string]*UserSessionState),
	}
	log.Message(log.ModuleAgent, "CodeGen WeChat bridge initialized")
}

// StartSessionForWeChat 启动编码会话并订阅通知
func StartSessionForWeChat(userID, project, prompt string) (string, error) {
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
	session, err := StartSession(project, prompt)
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

// StopSessionForWeChat 停止用户当前编码会话
func StopSessionForWeChat(userID string) (string, error) {
	if wechatBridge == nil {
		return "", fmt.Errorf("WeChat bridge not initialized")
	}

	wechatBridge.mu.RLock()
	state := wechatBridge.userSessions[userID]
	wechatBridge.mu.RUnlock()

	if state == nil {
		return "", fmt.Errorf("当前没有活跃的编码会话")
	}

	// 停止通知 goroutine
	select {
	case <-state.stopCh:
		// already closed
	default:
		close(state.stopCh)
	}

	// 停止会话
	if err := StopSession(state.SessionID); err != nil {
		return "", err
	}

	// 清理用户状态
	wechatBridge.mu.Lock()
	delete(wechatBridge.userSessions, userID)
	wechatBridge.mu.Unlock()

	return state.SessionID, nil
}

// subscribeAndRelay 后台 goroutine：订阅会话事件并中继到微信
func subscribeAndRelay(state *UserSessionState, session *CodeSession) {
	ch := session.Subscribe()
	defer session.Unsubscribe(ch)

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
				// 清理用户状态
				if wechatBridge != nil {
					wechatBridge.mu.Lock()
					delete(wechatBridge.userSessions, state.UserID)
					wechatBridge.mu.Unlock()
				}
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

// sendCompletionSummary 发送完成摘要（精简版）
func sendCompletionSummary(state *UserSessionState, session *CodeSession, event StreamEvent) {
	session.mu.Lock()
	status := session.Status
	cost := session.CostUSD
	startTime := session.StartTime
	endTime := session.EndTime
	errMsg := session.Error
	session.mu.Unlock()

	state.mu.Lock()
	totalSteps := state.StepCount
	state.mu.Unlock()

	elapsed := endTime.Sub(startTime).Round(time.Second)

	var summary string
	switch status {
	case StatusError:
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		summary = fmt.Sprintf("❌ %s 编码失败\n%s · %d步 · $%.4f\n\n%s",
			state.Project, elapsed, totalSteps, cost, errMsg)
	case StatusStopped:
		summary = fmt.Sprintf("⏹ %s 已停止\n%s · %d步 · $%.4f",
			state.Project, elapsed, totalSteps, cost)
	default:
		summary = fmt.Sprintf("✅ %s 编码完成\n%s · %d步 · $%.4f",
			state.Project, elapsed, totalSteps, cost)
	}

	if wechatBridge != nil && wechatBridge.sendMsg != nil {
		if err := wechatBridge.sendMsg(state.UserID, summary); err != nil {
			log.WarnF(log.ModuleAgent, "CodeGen WeChat completion notify failed: %v", err)
		}
	}
}

// formatEventForWeChat 格式化流式事件为微信可读文本
// 只保留工具操作步骤，丢弃 assistant 思考文本和 system 噪音
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

	// 本地项目
	projects, err := ListProjects()
	if err != nil {
		result["local_projects"] = []interface{}{}
		result["local_error"] = err.Error()
	} else {
		result["local_projects"] = projects
	}

	// 远程 agent 项目
	if agentPool != nil {
		remoteProjects := agentPool.ListRemoteProjects()
		result["remote_projects"] = remoteProjects
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// CreateProjectJSON 创建项目并返回 JSON
func CreateProjectJSON(name string) string {
	if err := CreateProject(name); err != nil {
		return fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())
	}
	return fmt.Sprintf(`{"success":true,"message":"项目 %s 创建成功"}`, name)
}
