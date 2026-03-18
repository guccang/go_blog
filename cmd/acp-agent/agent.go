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

// AnalysisSession 分析会话
type AnalysisSession struct {
	SessionID  string
	Project    string
	Status     string // "in_progress", "completed", "failed"
	Result     string
	ACPSession *ACPSession
}

// Agent ACP 分析 Agent
type Agent struct {
	ID       string
	cfg      *AgentConfig
	sessions map[string]*AnalysisSession
	mu       sync.Mutex
}

// NewAgent 创建 Agent
func NewAgent(id string, cfg *AgentConfig) *Agent {
	return &Agent{
		ID:       id,
		cfg:      cfg,
		sessions: make(map[string]*AnalysisSession),
	}
}

// ActiveCount 当前活跃分析数
func (a *Agent) ActiveCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := 0
	for _, s := range a.sessions {
		if s.Status == "in_progress" {
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

// resolveProject 在 workspaces 中查找项目
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
	return ""
}

// ExecuteAnalysis 执行项目分析（同步阻塞，返回分析报告）
func (a *Agent) ExecuteAnalysis(conn *Connection, sessionID, project, prompt string) (string, error) {
	// 解析项目路径
	projectPath := a.resolveProject(project)
	if projectPath == "" {
		return "", fmt.Errorf("project not found in workspaces: %s", project)
	}

	// 注册会话
	a.mu.Lock()
	a.sessions[sessionID] = &AnalysisSession{
		SessionID: sessionID,
		Project:   project,
		Status:    "in_progress",
	}
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		if s, ok := a.sessions[sessionID]; ok && s.Status == "in_progress" {
			s.Status = "failed"
		}
		a.mu.Unlock()
	}()

	// 发送开始事件
	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: fmt.Sprintf("🔍 ACP 分析开始... (项目: %s, Agent: %s)", project, a.cfg.AgentName),
		},
	})

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.AnalysisTimeout)*time.Second)
	defer cancel()

	// 启动 ACP 会话
	acpSession, acpClient, err := StartACPSession(ctx, a.cfg, projectPath)
	if err != nil {
		return "", fmt.Errorf("start acp session: %v", err)
	}
	defer acpSession.Close()

	// 记录 ACP 会话
	a.mu.Lock()
	if s, ok := a.sessions[sessionID]; ok {
		s.ACPSession = acpSession
	}
	a.mu.Unlock()

	// 发送分析 prompt
	log.Printf("[ACP] sending prompt: session=%s project=%s prompt_len=%d", sessionID, project, len(prompt))

	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: "📝 正在分析项目代码...",
		},
	})

	promptResp, err := acpSession.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acpSession.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		return "", fmt.Errorf("acp prompt: %v", err)
	}

	log.Printf("[ACP] prompt completed: session=%s stop_reason=%s", sessionID, promptResp.StopReason)

	// 收集分析结果
	result := acpClient.GetResult()

	// 更新会话状态
	a.mu.Lock()
	if s, ok := a.sessions[sessionID]; ok {
		s.Status = "completed"
		s.Result = result
	}
	a.mu.Unlock()

	// 发送完成事件
	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: "✅ 项目分析完成",
			Done: true,
		},
	})

	return result, nil
}

// CancelAnalysis 取消分析
func (a *Agent) CancelAnalysis(sessionID string) {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	a.mu.Unlock()

	if !ok {
		return
	}

	if s.ACPSession != nil {
		log.Printf("[ACP] cancelling analysis: session=%s", sessionID)
		// 发送 ACP cancel
		ctx := context.Background()
		s.ACPSession.conn.Cancel(ctx, acp.CancelNotification{
			SessionId: s.ACPSession.sessionID,
		})
		s.ACPSession.Close()
	}

	a.mu.Lock()
	s.Status = "failed"
	a.mu.Unlock()
}
