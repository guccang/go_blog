package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Agent 远程执行器
type Agent struct {
	ID            string
	cfg           *AgentConfig
	activeTasks   map[string]*exec.Cmd
	mu            sync.Mutex
}

// NewAgent 创建 Agent
func NewAgent(id string, cfg *AgentConfig) *Agent {
	return &Agent{
		ID:          id,
		cfg:         cfg,
		activeTasks: make(map[string]*exec.Cmd),
	}
}

// CanAccept 是否可以接受新任务
func (a *Agent) CanAccept() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.activeTasks) < a.cfg.MaxConcurrent
}

// ActiveCount 当前活跃任务数
func (a *Agent) ActiveCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.activeTasks)
}

// LoadFactor 负载因子 (0.0 ~ 1.0)
func (a *Agent) LoadFactor() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.MaxConcurrent <= 0 {
		return 1.0
	}
	return float64(len(a.activeTasks)) / float64(a.cfg.MaxConcurrent)
}

// ScanProjects 扫描所有 workspace 下的项目目录
func (a *Agent) ScanProjects() []string {
	var projects []string
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
				projects = append(projects, entry.Name())
			}
		}
	}
	return projects
}

// ScanSettings 扫描 SettingsDir 下所有 .json 文件，返回不含后缀的文件名列表
func (a *Agent) ScanSettings() []string {
	if a.cfg.SettingsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(a.cfg.SettingsDir)
	if err != nil {
		return nil
	}
	var models []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".json") {
			models = append(models, strings.TrimSuffix(name, filepath.Ext(name)))
		}
	}
	sort.Strings(models)
	return models
}

// StopTask 停止指定任务
func (a *Agent) StopTask(sessionID string) {
	a.mu.Lock()
	cmd, ok := a.activeTasks[sessionID]
	a.mu.Unlock()

	if ok && cmd.Process != nil {
		log.Printf("[INFO] killing task %s", sessionID)
		cmd.Process.Kill()
	}
}

// ExecuteTask 执行编码任务
func (a *Agent) ExecuteTask(conn *Connection, task *TaskAssignPayload) {
	sessionID := task.SessionID

	// 解析项目路径
	projectPath := a.resolveProject(task.Project)
	if projectPath == "" {
		conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID,
			Status:    "error",
			Error:     fmt.Sprintf("project not found in workspaces: %s", task.Project),
		})
		return
	}

	// 确保 .git 存在
	ensureGitInit(projectPath)

	// 构建 Claude CLI 参数
	args := a.buildArgs(task)

	log.Printf("[INFO] executing: %s %s (dir=%s)", a.cfg.ClaudePath, strings.Join(args, " "), projectPath)

	cmd := exec.Command(a.cfg.ClaudePath, args...)
	cmd.Dir = projectPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID, Status: "error", Error: fmt.Sprintf("stdout pipe: %v", err),
		})
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID, Status: "error", Error: fmt.Sprintf("stderr pipe: %v", err),
		})
		return
	}

	if err := cmd.Start(); err != nil {
		conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
			SessionID: sessionID, Status: "error", Error: fmt.Sprintf("start claude: %v", err),
		})
		return
	}

	// 注册活跃任务
	a.mu.Lock()
	a.activeTasks[sessionID] = cmd
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.activeTasks, sessionID)
		a.mu.Unlock()
	}()

	// 发送开始事件
	conn.SendMsg(MsgStreamEvent, StreamEventPayload{
		SessionID: sessionID,
		Event: StreamEvent{
			Type: "system",
			Text: fmt.Sprintf("🔧 Claude Code 开始编码... (项目: %s, Agent: %s)", task.Project, a.cfg.AgentName),
		},
	})

	// 异步读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[STDERR] %s", line)
			conn.SendMsg(MsgStreamEvent, StreamEventPayload{
				SessionID: sessionID,
				Event:     StreamEvent{Type: "error", Text: "⚠️ " + line},
			})
		}
	}()

	// 逐行读取 stream-json 并转发
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		event := parseStreamLine(line)
		if event == nil {
			continue
		}

		conn.SendMsg(MsgStreamEvent, StreamEventPayload{
			SessionID: sessionID,
			Event:     *event,
		})
	}

	// 等待进程完成
	err = cmd.Wait()

	status := "done"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
	}

	conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
		SessionID: sessionID,
		Status:    SessionStatus(status),
		Error:     errMsg,
	})

	log.Printf("[INFO] task %s completed, status=%s", sessionID, status)
}

// buildArgs 构建 Claude CLI 参数
func (a *Agent) buildArgs(task *TaskAssignPayload) []string {
	args := []string{
		"-p", task.Prompt,
		"--verbose",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
	}

	if task.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", task.SystemPrompt)
	}

	// 不使用 -c/--continue 续接会话：
	// 第三方模型（如 DeepSeek）续接时 thinking block signature 校验失败
	// 每次交互独立启动 Claude CLI，前端 UI 已维护对话历史展示

	maxTurns := task.MaxTurns
	if maxTurns <= 0 {
		maxTurns = a.cfg.MaxTurns
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}

	// 如果指定了模型配置，查找对应的 settings 文件
	if task.Model != "" && a.cfg.SettingsDir != "" {
		settingsFile := filepath.Join(a.cfg.SettingsDir, task.Model+".json")
		if _, err := os.Stat(settingsFile); err == nil {
			args = append(args, "--settings", settingsFile)
		}
	}

	return args
}

// resolveProject 在 workspaces 中查找项目
func (a *Agent) resolveProject(project string) string {
	// 安全检查
	if strings.Contains(project, "..") || strings.Contains(project, "/") || strings.Contains(project, "\\") {
		return ""
	}

	for _, ws := range a.cfg.Workspaces {
		p := filepath.Join(ws, project)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}

	// 如果不存在，在第一个 workspace 创建
	p := filepath.Join(a.cfg.Workspaces[0], project)
	if err := os.MkdirAll(p, 0755); err != nil {
		return ""
	}
	return p
}

// ensureGitInit 确保项目有独立 .git
func ensureGitInit(projectPath string) {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = projectPath
	cmd.Run()
}

// HandleFileRead 处理服务端的文件读取请求
func (a *Agent) HandleFileRead(conn *Connection, req *FileReadPayload) {
	projectPath := a.findProjectPath(req.Project)
	if projectPath == "" {
		conn.SendMsg(MsgFileReadResp, FileReadRespPayload{
			RequestID: req.RequestID,
			Error:     "project not found: " + req.Project,
		})
		return
	}

	fullPath := filepath.Join(projectPath, req.Path)
	// 安全检查：防止路径穿越
	absProject, _ := filepath.Abs(projectPath)
	absFile, _ := filepath.Abs(fullPath)
	rel, err := filepath.Rel(absProject, absFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		conn.SendMsg(MsgFileReadResp, FileReadRespPayload{
			RequestID: req.RequestID,
			Error:     "invalid file path",
		})
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		conn.SendMsg(MsgFileReadResp, FileReadRespPayload{
			RequestID: req.RequestID,
			Error:     err.Error(),
		})
		return
	}

	conn.SendMsg(MsgFileReadResp, FileReadRespPayload{
		RequestID: req.RequestID,
		Content:   string(data),
	})
}

// HandleTreeRead 处理服务端的目录树读取请求
func (a *Agent) HandleTreeRead(conn *Connection, req *TreeReadPayload) {
	projectPath := a.findProjectPath(req.Project)
	if projectPath == "" {
		conn.SendMsg(MsgTreeReadResp, TreeReadRespPayload{
			RequestID: req.RequestID,
			Error:     "project not found: " + req.Project,
		})
		return
	}

	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 5
	}

	tree := buildTree(projectPath, req.Project, 0, maxDepth)
	conn.SendMsg(MsgTreeReadResp, TreeReadRespPayload{
		RequestID: req.RequestID,
		Tree:      tree,
	})
}

// HandleProjectCreate 处理服务端的项目创建请求
func (a *Agent) HandleProjectCreate(conn *Connection, req *ProjectCreatePayload) {
	name := req.Name
	if name == "" {
		conn.SendMsg(MsgProjectCreateResp, ProjectCreateRespPayload{
			RequestID: req.RequestID,
			Error:     "project name is empty",
		})
		return
	}

	// 安全检查
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		conn.SendMsg(MsgProjectCreateResp, ProjectCreateRespPayload{
			RequestID: req.RequestID,
			Error:     "invalid project name: " + name,
		})
		return
	}

	// 检查是否已存在
	if existing := a.findProjectPath(name); existing != "" {
		conn.SendMsg(MsgProjectCreateResp, ProjectCreateRespPayload{
			RequestID: req.RequestID,
			Error:     "project already exists: " + name,
		})
		return
	}

	// 在第一个 workspace 创建
	if len(a.cfg.Workspaces) == 0 {
		conn.SendMsg(MsgProjectCreateResp, ProjectCreateRespPayload{
			RequestID: req.RequestID,
			Error:     "no workspace configured",
		})
		return
	}

	projectPath := filepath.Join(a.cfg.Workspaces[0], name)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		conn.SendMsg(MsgProjectCreateResp, ProjectCreateRespPayload{
			RequestID: req.RequestID,
			Error:     fmt.Sprintf("create dir failed: %v", err),
		})
		return
	}

	ensureGitInit(projectPath)
	log.Printf("[INFO] project created: %s at %s", name, projectPath)

	conn.SendMsg(MsgProjectCreateResp, ProjectCreateRespPayload{
		RequestID: req.RequestID,
		Success:   true,
	})
}

// findProjectPath 在 workspaces 中查找已存在的项目（不自动创建）
func (a *Agent) findProjectPath(project string) string {
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

// buildTree 递归构建目录树
func buildTree(absPath, relPath string, depth, maxDepth int) *DirNode {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil
	}

	node := &DirNode{
		Name:  filepath.Base(absPath),
		Path:  relPath,
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		node.Size = info.Size()
		return node
	}

	if depth >= maxDepth {
		return node
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return node
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" || name == "vendor" {
			continue
		}
		childAbs := filepath.Join(absPath, name)
		childRel := filepath.Join(relPath, name)
		child := buildTree(childAbs, childRel, depth+1, maxDepth)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	return node
}
