package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
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

	mu sync.Mutex
}

// NewAgent 创建 Agent
func NewAgent(id string, cfg *AgentConfig) *Agent {
	return &Agent{
		ID:            id,
		cfg:           cfg,
		sessions:      make(map[string]*sessionRecord),
		completionChs: make(map[string]chan taskResult),
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
func (a *Agent) ExecuteACP(conn *Connection, sessionID, project, prompt string) (taskResult, error) {
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

	acpSession, acpClient, err := StartACPSession(ctx, a.cfg, projectPath)
	if err != nil {
		a.completeSession(sessionID, "failed", "")
		return taskResult{Status: "error"}, fmt.Errorf("start acp session: %v", err)
	}

	// 设置 stream 回调
	acpClient.SetStreamCallback(func(evt StreamEvent) {
		evt.SessionID = sessionID
		conn.SendMsg(MsgStreamEvent, StreamEventPayload{
			SessionID: sessionID,
			Event:     evt,
		})
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
func (a *Agent) SendMessage(conn *Connection, sessionID, prompt string) (taskResult, error) {
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
	acpClient.SetStreamCallback(func(evt StreamEvent) {
		evt.SessionID = sessionID
		conn.SendMsg(MsgStreamEvent, StreamEventPayload{
			SessionID: sessionID,
			Event:     evt,
		})
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
