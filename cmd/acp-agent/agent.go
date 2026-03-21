package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"uap"
)

// ProjectInfo 项目信息
type ProjectInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// sessionRecord ACP 会话记录（支持多轮 Prompt）
type sessionRecord struct {
	Project    string
	Active     bool
	Status     string // "in_progress", "completed", "failed", "stopped"
	Summary    string
	ACPSession *ACPSession
	ACPClient  *ACPClientImpl
}

// taskResult 任务完成结果
type taskResult struct {
	Status       string
	Error        string
	Summary      string
	ProjectDir   string
	FilesWritten int
	FilesEdited  int
}

// Agent 纯 ACP 模式 Agent
type Agent struct {
	ID  string
	cfg *AgentConfig

	// ACP 会话记录（支持多轮）
	sessions   map[string]*sessionRecord
	sessionsMu sync.Mutex

	// 完成通知（用于 tool_call 同步等待）
	completionChs map[string]chan taskResult
	completionMu  sync.Mutex

	// 交互式权限：sessionID → ACPClientImpl（供权限回复和模式切换）
	permissionWaiters map[string]*ACPClientImpl
	permWaitersMu     sync.Mutex

	mu sync.Mutex
}

// NewAgent 创建 Agent
func NewAgent(id string, cfg *AgentConfig) *Agent {
	return &Agent{
		ID:                id,
		cfg:               cfg,
		sessions:          make(map[string]*sessionRecord),
		completionChs:     make(map[string]chan taskResult),
		permissionWaiters: make(map[string]*ACPClientImpl),
	}
}

// ActiveCount 当前活跃会话数
func (a *Agent) ActiveCount() int {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	count := 0
	for _, s := range a.sessions {
		if s.Active {
			count++
		}
	}
	return count
}

// LoadFactor 负载因子
func (a *Agent) LoadFactor() float64 {
	if a.cfg.MaxConcurrent <= 0 {
		return 1.0
	}
	return float64(a.ActiveCount()) / float64(a.cfg.MaxConcurrent)
}

// CanAccept 是否可以接受新任务
func (a *Agent) CanAccept() bool {
	return a.ActiveCount() < a.cfg.MaxConcurrent
}

// ScanProjects 扫描所有 workspace 下的项目目录
func (a *Agent) ScanProjects() []ProjectInfo {
	var projects []ProjectInfo
	seen := make(map[string]bool)
	for _, ws := range a.cfg.Workspaces {
		entries, err := os.ReadDir(ws)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if !seen[entry.Name()] {
				seen[entry.Name()] = true
				projects = append(projects, ProjectInfo{
					Name: entry.Name(),
					Path: filepath.Join(ws, entry.Name()),
				})
			}
		}
	}
	return projects
}

// resolveProject 在 workspaces 中查找项目，不存在则在第一个 workspace 创建
func (a *Agent) resolveProject(project string) string {
	if strings.Contains(project, "..") || strings.Contains(project, "/") || strings.Contains(project, "\\") {
		return ""
	}
	for _, ws := range a.cfg.Workspaces {
		p := filepath.Join(ws, project)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}

	// 不存在，在第一个 workspace 创建
	if len(a.cfg.Workspaces) == 0 {
		return ""
	}
	p := filepath.Join(a.cfg.Workspaces[0], project)
	if err := os.MkdirAll(p, 0755); err != nil {
		return ""
	}
	return p
}

// ========================= ACP 执行 =========================

// ExecuteACP 执行 ACP 会话（统一入口：分析 + 编码）
// extraArgs: 动态 CLI 参数（如 --dangerously-skip-permissions, --settings 等）
// interactive: 是否交互式权限模式
// callerAgentID: 调用方 agent ID（交互模式下权限请求发给该 agent）
func (a *Agent) ExecuteACP(conn *Connection, sessionID, project, prompt string, extraArgs []string, interactive bool, callerAgentID string) (taskResult, error) {
	projectPath := a.resolveProject(project)
	if projectPath == "" {
		return taskResult{Status: "error"}, fmt.Errorf("project not found in workspaces: %s", project)
	}

	// 记录会话
	a.sessionsMu.Lock()
	a.sessions[sessionID] = &sessionRecord{
		Project: project,
		Active:  true,
		Status:  "in_progress",
	}
	a.sessionsMu.Unlock()

	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: fmt.Sprintf("🔍 ACP 会话开始... (项目: %s, Agent: %s)", project, a.cfg.AgentName),
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.AnalysisTimeout)*time.Second)
	defer cancel()

	acpSession, acpClient, err := StartACPSession(ctx, a.cfg, projectPath, extraArgs)
	if err != nil {
		a.completeSession(sessionID, "failed", "")
		return taskResult{Status: "error"}, fmt.Errorf("start acp session: %v", err)
	}

	// 交互模式：配置权限回调 + channel
	if interactive {
		acpClient.interactive = true
		acpClient.permissionCh = make(chan permissionResponse, 1)
		acpClient.onPermission = func(req acp.RequestPermissionRequest) {
			// 将 ACP SDK 权限请求转为 UAP PermissionRequestPayload
			var options []uap.PermissionOptionDTO
			for i, opt := range req.Options {
				options = append(options, uap.PermissionOptionDTO{
					Index:    i + 1,
					OptionID: string(opt.OptionId),
					Name:     opt.Name,
					Kind:     string(opt.Kind),
				})
			}
			// 提取标题和内容
			title := ""
			if req.ToolCall.Title != nil {
				title = *req.ToolCall.Title
			}
			contentStr := ""
			for _, c := range req.ToolCall.Content {
				if c.Content != nil && c.Content.Content.Text != nil {
					contentStr += c.Content.Content.Text.Text
				}
			}

			payload := uap.PermissionRequestPayload{
				SessionID: sessionID,
				RequestID: fmt.Sprintf("perm_%d", time.Now().UnixNano()),
				Title:     title,
				Content:   contentStr,
				Options:   options,
			}
			// 发给调用方 agent（llm-agent）
			target := callerAgentID
			if target == "" {
				target = a.cfg.GoBackendAgentID
			}
			conn.Client.SendTo(target, uap.MsgPermissionRequest, payload)
		}

		// 注册到 permissionWaiters 供外部回复
		a.permWaitersMu.Lock()
		a.permissionWaiters[sessionID] = acpClient
		a.permWaitersMu.Unlock()
	}

	// 设置 stream 回调（callerAgentID 非空时发给调用方，否则发给 GoBackendAgentID）
	streamTarget := callerAgentID
	acpClient.SetStreamCallback(func(evt StreamEvent) {
		evt.SessionID = sessionID
		payload := StreamEventPayload{
			SessionID: sessionID,
			Event:     evt,
		}
		if streamTarget != "" {
			// Claude Mode: 通过 notify(acp_stream) 发给调用方
			conn.Client.SendTo(streamTarget, uap.MsgNotify, uap.NotifyPayload{
				Channel: "acp_stream",
				To:      sessionID,
				Content: mustMarshalStr(payload),
			})
		} else {
			conn.SendMsg(MsgStreamEvent, payload)
		}
	})

	// 保存 ACPSession 到记录（供 SendMessage 复用）
	a.sessionsMu.Lock()
	if rec, ok := a.sessions[sessionID]; ok {
		rec.ACPSession = acpSession
		rec.ACPClient = acpClient
	}
	a.sessionsMu.Unlock()

	log.Printf("[ACP] sending prompt: session=%s project=%s prompt_len=%d", sessionID, project, len(prompt))

	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: "📝 正在处理...",
		},
	})

	_, err = acpSession.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acpSession.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		a.completeSession(sessionID, "failed", "")
		acpSession.Close()
		return taskResult{Status: "error"}, fmt.Errorf("acp prompt: %v", err)
	}

	resultText := acpClient.GetResult()
	filesWritten := acpClient.GetFilesWritten()
	filesEdited := acpClient.GetFilesEdited()

	summary := resultText
	if len(summary) > 3000 {
		summary = summary[:3000] + "\n..."
	}

	a.completeSession(sessionID, "completed", summary)

	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: "✅ ACP 会话完成",
			Done: true,
		},
	})

	return taskResult{
		Status:       "done",
		Summary:      summary,
		ProjectDir:   projectPath,
		FilesWritten: len(uniqueStrings(filesWritten)),
		FilesEdited:  len(uniqueStrings(filesEdited)),
	}, nil
}

// SendMessage 向已有 ACP 会话追加消息（多轮 Prompt）
// interactive 和 callerAgentID 用于 Claude Mode 流式路由
func (a *Agent) SendMessage(conn *Connection, sessionID, prompt string, interactive bool, callerAgentID string) (taskResult, error) {
	a.sessionsMu.Lock()
	rec, ok := a.sessions[sessionID]
	if !ok {
		a.sessionsMu.Unlock()
		return taskResult{Status: "error"}, fmt.Errorf("session not found: %s", sessionID)
	}
	if rec.ACPSession == nil {
		a.sessionsMu.Unlock()
		return taskResult{Status: "error"}, fmt.Errorf("session has no active ACP connection: %s", sessionID)
	}
	acpSession := rec.ACPSession
	acpClient := rec.ACPClient
	project := rec.Project
	rec.Active = true
	rec.Status = "in_progress"
	a.sessionsMu.Unlock()

	// 设置 stream 回调（可能已切换 conn）
	smStreamTarget := callerAgentID
	acpClient.SetStreamCallback(func(evt StreamEvent) {
		evt.SessionID = sessionID
		payload := StreamEventPayload{
			SessionID: sessionID,
			Event:     evt,
		}
		if smStreamTarget != "" {
			conn.Client.SendTo(smStreamTarget, uap.MsgNotify, uap.NotifyPayload{
				Channel: "acp_stream",
				To:      sessionID,
				Content: mustMarshalStr(payload),
			})
		} else {
			conn.SendMsg(MsgStreamEvent, payload)
		}
	})

	projectPath := a.resolveProject(project)

	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: "📝 继续对话...",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.AnalysisTimeout)*time.Second)
	defer cancel()

	log.Printf("[ACP] send_message: session=%s prompt_len=%d", sessionID, len(prompt))

	_, err := acpSession.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acpSession.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		a.completeSession(sessionID, "failed", "")
		return taskResult{Status: "error"}, fmt.Errorf("acp prompt: %v", err)
	}

	resultText := acpClient.GetResult()
	filesWritten := acpClient.GetFilesWritten()
	filesEdited := acpClient.GetFilesEdited()

	summary := resultText
	if len(summary) > 3000 {
		summary = summary[:3000] + "\n..."
	}

	// 标记为非活跃但不关闭（保留会话供后续多轮）
	a.sessionsMu.Lock()
	if r, ok := a.sessions[sessionID]; ok {
		r.Active = false
		r.Status = "completed"
		r.Summary = summary
	}
	a.sessionsMu.Unlock()

	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: "✅ 对话完成",
			Done: true,
		},
	})

	return taskResult{
		Status:       "done",
		Summary:      summary,
		ProjectDir:   projectPath,
		FilesWritten: len(uniqueStrings(filesWritten)),
		FilesEdited:  len(uniqueStrings(filesEdited)),
	}, nil
}

// StopTask 停止 ACP 会话
func (a *Agent) StopTask(sessionID string) {
	a.sessionsMu.Lock()
	rec, ok := a.sessions[sessionID]
	a.sessionsMu.Unlock()

	if !ok {
		return
	}

	if rec.ACPSession != nil {
		log.Printf("[ACP] stopping session: %s", sessionID)
		ctx := context.Background()
		rec.ACPSession.conn.Cancel(ctx, acp.CancelNotification{
			SessionId: rec.ACPSession.sessionID,
		})
		rec.ACPSession.Close()
	}

	a.completeSession(sessionID, "stopped", "")
}

// ========================= 会话管理 =========================

// completeSession 标记会话完成
func (a *Agent) completeSession(sessionID, status, summary string) {
	a.sessionsMu.Lock()
	if rec, ok := a.sessions[sessionID]; ok {
		rec.Active = false
		rec.Status = status
		if summary != "" {
			rec.Summary = summary
		}
	}
	a.sessionsMu.Unlock()

	// 清理权限等待器
	a.cleanupPermissionWaiter(sessionID)
}

// GetSession 获取会话记录
func (a *Agent) GetSession(sessionID string) *sessionRecord {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	if rec, ok := a.sessions[sessionID]; ok {
		return &sessionRecord{
			Project: rec.Project,
			Active:  rec.Active,
			Status:  rec.Status,
			Summary: rec.Summary,
		}
	}
	return nil
}

// GetLastSession 获取最近的会话记录
func (a *Agent) GetLastSession() (string, *sessionRecord) {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	var lastID string
	for id := range a.sessions {
		if lastID == "" || id > lastID {
			lastID = id
		}
	}
	if lastID == "" {
		return "", nil
	}
	rec := a.sessions[lastID]
	return lastID, &sessionRecord{
		Project: rec.Project,
		Active:  rec.Active,
		Status:  rec.Status,
		Summary: rec.Summary,
	}
}

// RegisterCompletion 注册完成通知 channel
func (a *Agent) RegisterCompletion(sessionID string) chan taskResult {
	ch := make(chan taskResult, 1)
	a.completionMu.Lock()
	a.completionChs[sessionID] = ch
	a.completionMu.Unlock()
	return ch
}

// SignalCompletion 发送完成信号
func (a *Agent) SignalCompletion(sessionID string, result taskResult) {
	a.completionMu.Lock()
	ch, ok := a.completionChs[sessionID]
	if ok {
		delete(a.completionChs, sessionID)
	}
	a.completionMu.Unlock()
	if ok {
		select {
		case ch <- result:
		default:
		}
	}
}

func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ========================= Claude Mode: 权限/模式管理 =========================

// deliverPermissionResponse 将用户的权限回复发送到对应 ACPClient 的 channel
func (a *Agent) deliverPermissionResponse(sessionID, optionID string, cancelled bool) error {
	a.permWaitersMu.Lock()
	client, ok := a.permissionWaiters[sessionID]
	a.permWaitersMu.Unlock()

	if !ok {
		return fmt.Errorf("no permission waiter for session: %s", sessionID)
	}
	client.RespondPermission(optionID, cancelled)
	return nil
}

// setSessionMode 切换 ACP 会话模式
func (a *Agent) setSessionMode(sessionID, modeID string) error {
	a.sessionsMu.Lock()
	rec, ok := a.sessions[sessionID]
	a.sessionsMu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if rec.ACPSession == nil {
		return fmt.Errorf("session has no active ACP connection: %s", sessionID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := rec.ACPSession.conn.SetSessionMode(ctx, acp.SetSessionModeRequest{
		SessionId: rec.ACPSession.sessionID,
		ModeId:    acp.SessionModeId(modeID),
	})
	if err != nil {
		return fmt.Errorf("set session mode: %v", err)
	}
	log.Printf("[ACP] mode switched: session=%s mode=%s", sessionID, modeID)
	return nil
}

// cleanupPermissionWaiter 清理权限等待器
func (a *Agent) cleanupPermissionWaiter(sessionID string) {
	a.permWaitersMu.Lock()
	delete(a.permissionWaiters, sessionID)
	a.permWaitersMu.Unlock()
}

// mustMarshalStr JSON 序列化为字符串
func mustMarshalStr(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
