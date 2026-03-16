package main

import (
	"bufio"
	"encoding/json"
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
	ID           string
	cfg          *AgentConfig
	activeTasks  map[string]*exec.Cmd
	stoppedTasks map[string]bool
	mu           sync.Mutex

	// 会话记录（用于 tool_call 流的 SendMessage/GetStatus/StopSession）
	sessions   map[string]*sessionRecord
	sessionsMu sync.Mutex

	// 完成通知（用于 tool_call 同步等待任务完成）
	completionChs map[string]chan taskResult
	completionMu  sync.Mutex
}

// sessionRecord 记录编码会话状态（用于 tool_call 续接 + 状态查询）
type sessionRecord struct {
	Project       string
	Model         string
	Tool          string
	ClaudeSession string // Claude 内部 session ID（用于 --resume）
	Active        bool
	Status        string // "in_progress", "completed", "failed", "stopped"
	Summary       string // 完成摘要（completed 时有值）
}

// taskResult 任务完成结果
type taskResult struct {
	Status     string // done / error / stopped
	Error      string
	Summary    string // 任务总结报告
	ProjectDir string // 项目目录绝对路径
}

// NewAgent 创建 Agent
func NewAgent(id string, cfg *AgentConfig) *Agent {
	return &Agent{
		ID:            id,
		cfg:           cfg,
		activeTasks:   make(map[string]*exec.Cmd),
		stoppedTasks:  make(map[string]bool),
		sessions:      make(map[string]*sessionRecord),
		completionChs: make(map[string]chan taskResult),
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

// ScanSettings 扫描 ClaudeCode 和 OpenCode 配置目录，返回合并后的配置名列表
func (a *Agent) ScanSettings() []string {
	seen := make(map[string]bool)
	var models []string

	// 扫描 Claude Code 配置目录
	if a.cfg.ClaudeCodeSettingsDir != "" {
		entries, err := os.ReadDir(a.cfg.ClaudeCodeSettingsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(strings.ToLower(name), ".json") {
					modelName := strings.TrimSuffix(name, filepath.Ext(name))
					if !seen[modelName] {
						seen[modelName] = true
						models = append(models, modelName)
					}
				}
			}
		}
	}

	// 扫描 OpenCode 配置目录
	if a.cfg.OpenCodeSettingsDir != "" {
		entries, err := os.ReadDir(a.cfg.OpenCodeSettingsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(strings.ToLower(name), ".json") {
					modelName := strings.TrimSuffix(name, filepath.Ext(name))
					if !seen[modelName] {
						seen[modelName] = true
						models = append(models, modelName)
					}
				}
			}
		}
	}

	sort.Strings(models)
	return models
}

// ScanClaudeCodeSettings 扫描 Claude Code 配置目录
func (a *Agent) ScanClaudeCodeSettings() []string {
	if a.cfg.ClaudeCodeSettingsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(a.cfg.ClaudeCodeSettingsDir)
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

// ScanOpenCodeSettings 扫描 OpenCode 配置目录
func (a *Agent) ScanOpenCodeSettings() []string {
	if a.cfg.OpenCodeSettingsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(a.cfg.OpenCodeSettingsDir)
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

// ScanTools 检测本机安装的编码工具
func (a *Agent) ScanTools() []string {
	var tools []string
	if _, err := exec.LookPath(a.cfg.ClaudePath); err == nil {
		tools = append(tools, "claudecode")
	}
	if _, err := exec.LookPath(a.cfg.OpenCodePath); err == nil {
		tools = append(tools, "opencode")
	}
	return tools
}

// StopTask 停止指定任务
func (a *Agent) StopTask(sessionID string) {
	a.mu.Lock()
	cmd, ok := a.activeTasks[sessionID]
	if ok {
		a.stoppedTasks[sessionID] = true
	}
	a.mu.Unlock()

	if ok && cmd.Process != nil {
		log.Printf("[INFO] killing task %s", sessionID)
		// Windows 需要杀死整个进程组
		if cmd.Process.Pid > 0 {
			killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
			killCmd.Run()
		}
		cmd.Process.Kill()
	}
}

// IsTaskStopped 检查任务是否被停止
func (a *Agent) IsTaskStopped(sessionID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stoppedTasks[sessionID]
}

// ClearStopped 清除停止标记
func (a *Agent) ClearStopped(sessionID string) {
	a.mu.Lock()
	delete(a.stoppedTasks, sessionID)
	a.mu.Unlock()
}

// ExecuteTask 执行编码任务
func (a *Agent) ExecuteTask(conn *Connection, task *TaskAssignPayload) {
	sessionID := task.SessionID

	// 调试：打印收到的任务参数
	log.Printf("[DEBUG] ExecuteTask: session=%s, project=%s, tool=%s, model=%s",
		sessionID, task.Project, task.Tool, task.Model)

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

	// 根据工具类型选择可执行文件和参数
	var cmdPath string
	var args []string
	toolName := "Claude Code"
	if task.Tool == "opencode" {
		cmdPath = a.cfg.OpenCodePath
		args = a.buildOpenCodeArgs(task)
		toolName = "OpenCode"
	} else {
		cmdPath = a.cfg.ClaudePath
		var buildErr error
		args, buildErr = a.buildArgs(task)
		if buildErr != nil {
			conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
				SessionID: sessionID, Status: "error", Error: buildErr.Error(),
			})
			return
		}
	}

	log.Printf("[INFO] executing: %s %s (dir=%s, tool=%s, prompt_len=%d)", cmdPath, strings.Join(args, " "), projectPath, toolName, len(task.Prompt))

	cmd := exec.Command(cmdPath, args...)
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
			Text: fmt.Sprintf("🔧 %s 开始编码... (项目: %s, Agent: %s)", toolName, task.Project, a.cfg.AgentName),
		},
	})

	// 标记是否使用 OpenCode（stderr/stdout 解析策略不同）
	useOpenCode := task.Tool == "opencode"

	// 任务总结收集器
	var summary TaskSummary

	// 异步读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// 过滤 Windows libuv 已知断言错误（进程退出时竞态，不影响结果）
			if strings.Contains(line, "UV_HANDLE_CLOSING") {
				log.Printf("[STDERR] (ignored libuv assertion) %s", line)
				continue
			}
			log.Printf("[STDERR] %s", line)
			if useOpenCode {
				// OpenCode 的进度输出（工具调用、命令执行）走 stderr
				event := parseOpenCodeStderr(line)
				if event != nil {
					conn.SendMsg(MsgStreamEvent, StreamEventPayload{
						SessionID: sessionID,
						Event:     *event,
					})
				}
			} else {
				conn.SendMsg(MsgStreamEvent, StreamEventPayload{
					SessionID: sessionID,
					Event:     StreamEvent{Type: "error", Text: "⚠️ " + line},
				})
			}
		}
	}()

	// 逐行读取输出并转发
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event *StreamEvent
		if useOpenCode {
			event = parseOpenCodeLine(line)
		} else {
			event = parseStreamLine(line)
		}
		if event == nil {
			continue
		}

		// 收集事件用于总结
		summary.UpdateFromEvent(event)

		// 捕获 Claude 内部 session ID（用于 --resume 续接）
		if event.SessionID != "" {
			a.UpdateSessionClaudeID(sessionID, event.SessionID)
		}

		// Done 仅由 TaskComplete 统一触发，防止 result 事件提前关闭 WeChat 通知
		evt := *event
		evt.Done = false
		conn.SendMsg(MsgStreamEvent, StreamEventPayload{
			SessionID: sessionID,
			Event:     evt,
		})
	}

	// 等待进程完成
	err = cmd.Wait()

	// 清除停止标记
	defer a.ClearStopped(sessionID)

	status := "done"
	errMsg := ""
	if err != nil {
		// 检查是否是被用户停止
		if a.IsTaskStopped(sessionID) {
			status = "stopped"
		} else if summary.ResultText != "" {
			// 已收到 result 事件说明编码完成，忽略进程退出码错误
			// （Windows 上 Claude CLI/libuv 退出时可能触发断言失败）
			log.Printf("[WARN] task %s process exited with error but result received, treating as done: %v", sessionID, err)
		} else {
			status = "error"
			errMsg = err.Error()
		}
	}

	// 发送任务总结报告
	if status == "done" {
		report := summary.GenerateReport()
		conn.SendMsg(MsgStreamEvent, StreamEventPayload{
			SessionID: sessionID,
			Event: StreamEvent{
				Type: "summary",
				Text: report,
				Done: false, // 不在这里标记完成，由 TaskComplete 统一触发
			},
		})
	}

	conn.SendMsg(MsgTaskComplete, TaskCompletePayload{
		SessionID: sessionID,
		Status:    SessionStatus(status),
		Error:     errMsg,
	})

	// 标记会话完成（含最终状态和摘要）
	taskSummary := ""
	if status == "done" {
		taskSummary = summary.GenerateReport()
	}
	a.CompleteSession(sessionID, status, taskSummary)

	// 通知同步等待者（tool_call 流）
	completionResult := taskResult{Status: status, Error: errMsg, ProjectDir: projectPath}
	if status == "done" {
		completionResult.Summary = taskSummary
	}
	a.SignalCompletion(sessionID, completionResult)

	log.Printf("[INFO] task %s completed, status=%s", sessionID, status)
}

// buildArgs 构建 Claude CLI 参数
// prompt 通过 -p 参数传递，换行符已被 sanitize 为空格
// 因为 Windows CreateProcess 无法正确处理参数中的换行符
func (a *Agent) buildArgs(task *TaskAssignPayload) ([]string, error) {
	prompt := sanitizePromptForCLI(task.Prompt)
	args := []string{
		"-p", prompt,
		"--verbose",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
	}

	if task.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", task.SystemPrompt)
	}

	// 支持 resume 的模型使用 --resume 续接会话，减少 tokens 浪费
	if task.ClaudeSession != "" && a.isResumableModel(task.Model) {
		args = append(args, "--resume", task.ClaudeSession)
	}

	maxTurns := task.MaxTurns
	if maxTurns <= 0 {
		maxTurns = a.cfg.MaxTurns
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}

	// 查找对应的 settings 文件：指定 model 用 <model>.json，未指定用 default.json
	if a.cfg.ClaudeCodeSettingsDir != "" {
		modelName := task.Model
		if modelName == "" {
			modelName = "default"
		}
		settingsFile := filepath.Join(a.cfg.ClaudeCodeSettingsDir, modelName+".json")
		if _, err := os.Stat(settingsFile); err != nil {
			return nil, fmt.Errorf("settings file not found: %s (请确保 %s.json 存在)", settingsFile, modelName)
		}
		args = append(args, "--settings", settingsFile)
	}

	return args, nil
}

// isResumableModel 检查模型是否支持 --resume 续接
// 空字符串视为 "default"（未指定模型时使用 Claude Code 默认模型）
func (a *Agent) isResumableModel(model string) bool {
	checkModel := model
	if checkModel == "" {
		checkModel = "default"
	}
	for _, m := range a.cfg.ResumeModels {
		if m == checkModel {
			return true
		}
	}
	return false
}

// buildOpenCodeArgs 构建 OpenCode CLI 参数
// OpenCode 使用 --model "provider/model" 格式
// OpenCode 不支持 --append-system-prompt，系统指令注入 prompt 前缀
func (a *Agent) buildOpenCodeArgs(task *TaskAssignPayload) []string {
	args := []string{"run", "--format", "json"}

	// 模型选择：OpenCode 使用 provider/model 格式
	if task.Model != "" {
		modelID := a.resolveOpenCodeModel(task.Model)
		if modelID != "" {
			args = append(args, "--model", modelID)
		}
	}

	// OpenCode 不支持 system prompt flag，注入到 prompt 前缀
	prompt := task.Prompt
	if task.SystemPrompt != "" {
		prompt = "[系统指令] " + task.SystemPrompt + "\n\n[用户需求] " + prompt
	}

	// 清理 prompt 中的换行符，避免 Windows 命令行参数截断
	prompt = sanitizePromptForCLI(prompt)

	// prompt 放最后
	args = append(args, prompt)

	return args
}

// sanitizePromptForCLI 清理 prompt 中的换行符用于命令行参数传递
// Windows CreateProcess 无法正确处理命令行参数中的换行符
func sanitizePromptForCLI(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// resolveOpenCodeModel 将配置名解析为 OpenCode 可用的 model ID
// 从 opencode 配置目录读取 model 字段
func (a *Agent) resolveOpenCodeModel(model string) string {
	if a.cfg.OpenCodeSettingsDir == "" {
		return model
	}

	settingsFile := filepath.Join(a.cfg.OpenCodeSettingsDir, model+".json")
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return model
	}

	var settings struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return model
	}
	if settings.Model != "" {
		return settings.Model
	}
	return model
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

// RecordSession 记录新会话
func (a *Agent) RecordSession(sessionID, project, model, tool string) {
	a.sessionsMu.Lock()
	a.sessions[sessionID] = &sessionRecord{
		Project: project,
		Model:   model,
		Tool:    tool,
		Active:  true,
		Status:  "in_progress",
	}
	a.sessionsMu.Unlock()
}

// CompleteSession 标记会话完成（含最终状态和摘要）
func (a *Agent) CompleteSession(sessionID, status, summary string) {
	a.sessionsMu.Lock()
	if rec, ok := a.sessions[sessionID]; ok {
		rec.Active = false
		switch status {
		case "done":
			rec.Status = "completed"
		case "error":
			rec.Status = "failed"
		case "stopped":
			rec.Status = "stopped"
		default:
			rec.Status = status
		}
		rec.Summary = summary
	}
	a.sessionsMu.Unlock()
}

// RegisterCompletion 注册完成通知 channel（tool_call 同步等待用）
func (a *Agent) RegisterCompletion(sessionID string) chan taskResult {
	ch := make(chan taskResult, 1)
	a.completionMu.Lock()
	a.completionChs[sessionID] = ch
	a.completionMu.Unlock()
	return ch
}

// SignalCompletion 发送完成信号（ExecuteTask 完成时调用）
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

// UpdateSessionClaudeID 更新会话的 Claude 内部 session ID
func (a *Agent) UpdateSessionClaudeID(sessionID, claudeSession string) {
	a.sessionsMu.Lock()
	if rec, ok := a.sessions[sessionID]; ok && claudeSession != "" {
		rec.ClaudeSession = claudeSession
	}
	a.sessionsMu.Unlock()
}

// GetSession 获取会话记录
func (a *Agent) GetSession(sessionID string) *sessionRecord {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	if rec, ok := a.sessions[sessionID]; ok {
		// 返回副本
		copy := *rec
		return &copy
	}
	return nil
}

// GetLastSession 获取最近的会话记录（不论是否活跃）
func (a *Agent) GetLastSession() (string, *sessionRecord) {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	var lastID string
	var lastRec *sessionRecord
	for id, rec := range a.sessions {
		if lastID == "" || id > lastID {
			lastID = id
			copy := *rec
			lastRec = &copy
		}
	}
	return lastID, lastRec
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
