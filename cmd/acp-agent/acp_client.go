package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// ACPSession 一次 ACP 会话
type ACPSession struct {
	cmd       *exec.Cmd
	conn      *acp.ClientSideConnection
	sessionID acp.SessionId
	cancel    context.CancelFunc
}

// Close 关闭 ACP 会话（带超时保护，防止 Wait 挂起）
func (s *ACPSession) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		// Wait with timeout: Windows 上 Kill 后 Wait 可能因管道未关闭而挂起
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()
		select {
		case <-done:
			// 进程已正常退出
		case <-time.After(5 * time.Second):
			log.Printf("[ACP] warning: process pid=%d didn't exit within 5s after kill", s.cmd.Process.Pid)
		}
	}
}

// ACPClientImpl 实现 acp.Client 接口
// 处理 Agent 的反向请求（读文件、写文件、权限等）
type ACPClientImpl struct {
	projectPath  string
	mu           sync.Mutex
	chunks       []string // 收集 agent_message_chunk
	streamCb     func(StreamEvent)
	filesWritten []string
	filesEdited  []string
	resultText   string
	lastEventAt  time.Time
	lastEvent    string

	// 交互式权限模式
	interactive  bool
	permissionCh chan permissionResponse
	onPermission func(acp.RequestPermissionRequest) // 权限请求外发回调

	// 模式信息
	availableModes []acp.SessionMode
	currentModeID  string
	modelID        string
}

// permissionResponse 权限回复
type permissionResponse struct {
	OptionID  string
	Cancelled bool
}

// NewACPClientImpl 创建 ACP Client 实现
func NewACPClientImpl(projectPath string) *ACPClientImpl {
	return &ACPClientImpl{
		projectPath: projectPath,
		lastEventAt: time.Now(),
		lastEvent:   "session initialized",
	}
}

// SetStreamCallback 设置事件推送回调
func (c *ACPClientImpl) SetStreamCallback(cb func(StreamEvent)) {
	c.mu.Lock()
	c.streamCb = cb
	c.mu.Unlock()
}

// GetResult 获取收集到的结果文本
func (c *ACPClientImpl) GetResult() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.resultText != "" {
		return c.resultText
	}
	return strings.Join(c.chunks, "")
}

// GetFilesWritten 获取写入的文件列表
func (c *ACPClientImpl) GetFilesWritten() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]string, len(c.filesWritten))
	copy(result, c.filesWritten)
	return result
}

// GetFilesEdited 获取编辑的文件列表
func (c *ACPClientImpl) GetFilesEdited() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]string, len(c.filesEdited))
	copy(result, c.filesEdited)
	return result
}

// GetAvailableModes 获取可用模式列表
func (c *ACPClientImpl) GetAvailableModes() []acp.SessionMode {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]acp.SessionMode, len(c.availableModes))
	copy(result, c.availableModes)
	return result
}

// GetCurrentModeID 获取当前模式 ID
func (c *ACPClientImpl) GetCurrentModeID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentModeID
}

// GetModelID 获取当前模型 ID
func (c *ACPClientImpl) GetModelID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.modelID
}

func (c *ACPClientImpl) markActivity(event string) {
	c.mu.Lock()
	c.lastEventAt = time.Now()
	c.lastEvent = event
	c.mu.Unlock()
}

func (c *ACPClientImpl) ActivitySnapshot() (time.Time, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastEventAt, c.lastEvent
}

// SessionUpdate 处理 session/update 通知（核心：收集 agent 输出 + 推送事件）
func (c *ACPClientImpl) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	update := params.Update

	if update.AgentMessageChunk != nil {
		if update.AgentMessageChunk.Content.Text != nil {
			text := update.AgentMessageChunk.Content.Text.Text
			preview := previewText(text, 160)
			c.mu.Lock()
			c.chunks = append(c.chunks, text)
			cb := c.streamCb
			c.mu.Unlock()
			c.markActivity("assistant: " + preview)
			log.Printf("[ACP] assistant_chunk: %s", preview)

			if cb != nil {
				cb(StreamEvent{Type: "assistant", Text: text})
			}
		}
	}

	// AgentThoughtChunk → thought 事件
	if update.AgentThoughtChunk != nil {
		if update.AgentThoughtChunk.Content.Text != nil {
			text := update.AgentThoughtChunk.Content.Text.Text
			preview := previewText(text, 160)
			c.mu.Lock()
			cb := c.streamCb
			c.mu.Unlock()
			c.markActivity("thought: " + preview)
			log.Printf("[ACP] thought_chunk: %s", preview)

			if cb != nil {
				cb(StreamEvent{Type: "thought", Text: text})
			}
		}
	}

	if update.ToolCall != nil {
		title := strings.TrimSpace(update.ToolCall.Title)
		detail := buildToolDetail(update.ToolCall)
		c.markActivity("tool_call: " + title)
		if detail != "" {
			log.Printf("[ACP] tool_call: %s (status=%s)%s", title, update.ToolCall.Status, detail)
		} else {
			log.Printf("[ACP] tool_call: %s (status=%s)", title, update.ToolCall.Status)
		}

		c.mu.Lock()
		cb := c.streamCb
		c.mu.Unlock()

		if cb != nil {
			cb(StreamEvent{
				Type:     "tool",
				ToolName: update.ToolCall.Title,
				Text:     fmt.Sprintf("🔧 %s", update.ToolCall.Title),
			})

			// 额外发送 tool_detail 事件（包含工具详情）
			if detail != "" {
				cb(StreamEvent{Type: "tool_detail", Text: detail})
			}
		}
	}

	if update.ToolCallUpdate != nil {
		c.mu.Lock()
		cb := c.streamCb
		c.mu.Unlock()

		if cb != nil {
			text := buildToolUpdateText(update.ToolCallUpdate)
			if text != "" {
				cb(StreamEvent{Type: "tool_update", Text: text})
			}
		}

		if update.ToolCallUpdate.Status != nil {
			statusText := string(*update.ToolCallUpdate.Status)
			c.markActivity("tool_update: " + statusText)
			log.Printf("[ACP] tool_call_update: %s", strings.TrimSpace(buildToolUpdateText(update.ToolCallUpdate)))
		}
	}

	// Plan 事件：格式化执行计划推送
	if update.Plan != nil {
		planText := formatPlan(update.Plan.Entries)
		c.markActivity(fmt.Sprintf("plan update: %d entries", len(update.Plan.Entries)))
		log.Printf("[ACP] plan update: %d entries", len(update.Plan.Entries))

		c.mu.Lock()
		cb := c.streamCb
		c.mu.Unlock()

		if cb != nil {
			cb(StreamEvent{Type: "plan", Text: planText})
		}
	}

	// Mode 切换事件
	if update.CurrentModeUpdate != nil {
		modeID := string(update.CurrentModeUpdate.CurrentModeId)
		c.markActivity("mode update: " + modeID)
		log.Printf("[ACP] mode update: %s", modeID)

		c.mu.Lock()
		c.currentModeID = modeID
		cb := c.streamCb
		c.mu.Unlock()

		if cb != nil {
			cb(StreamEvent{Type: "mode", Text: fmt.Sprintf("🔄 模式: %s", modeID)})
		}
	}

	return nil
}

// ReadTextFile 读取项目文件（Agent 反向请求）
func (c *ACPClientImpl) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	filePath := params.Path

	// 安全检查：禁止路径穿越
	if strings.Contains(filePath, "..") {
		return acp.ReadTextFileResponse{}, fmt.Errorf("path traversal not allowed")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read file: %v", err)
	}
	c.markActivity("read file: " + filePath)
	log.Printf("[ACP] read_file: %s (%d bytes)", filePath, len(data))

	return acp.ReadTextFileResponse{
		Content: string(data),
	}, nil
}

// WriteTextFile 写入文件（含项目目录安全检查）
func (c *ACPClientImpl) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	filePath := params.Path

	// 安全检查：禁止路径穿越
	if strings.Contains(filePath, "..") {
		return acp.WriteTextFileResponse{}, fmt.Errorf("path traversal not allowed")
	}

	// 安全检查：必须在项目目录下
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("resolve path: %v", err)
	}
	absProject, _ := filepath.Abs(c.projectPath)
	if !strings.HasPrefix(absPath, absProject+string(filepath.Separator)) && absPath != absProject {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write outside project directory not allowed")
	}

	// 确保父目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("create dir: %v", err)
	}

	// 判断是新建还是编辑
	isNew := true
	if _, err := os.Stat(filePath); err == nil {
		isNew = false
	}

	if err := os.WriteFile(filePath, []byte(params.Content), 0644); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write file: %v", err)
	}

	// 记录文件变更
	c.mu.Lock()
	if isNew {
		c.filesWritten = append(c.filesWritten, filePath)
	} else {
		c.filesEdited = append(c.filesEdited, filePath)
	}
	c.mu.Unlock()

	c.markActivity(fmt.Sprintf("write file: %s", filePath))
	log.Printf("[ACP] write_file: %s (new=%v)", filePath, isNew)

	return acp.WriteTextFileResponse{}, nil
}

// RequestPermission 处理权限请求（自动模式：自动批准 / 交互模式：转发等待用户回复）
func (c *ACPClientImpl) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// 交互模式：通过回调发给 llm-agent，阻塞等待用户回复
	if c.interactive && c.onPermission != nil {
		log.Printf("[ACP] permission request (interactive): tool=%v options=%d", params.ToolCall.Title, len(params.Options))
		c.markActivity("permission request")
		c.onPermission(params)

		// 阻塞等待用户回复
		select {
		case resp := <-c.permissionCh:
			if resp.Cancelled {
				return acp.RequestPermissionResponse{
					Outcome: acp.RequestPermissionOutcome{
						Cancelled: &acp.RequestPermissionOutcomeCancelled{
							Outcome: "cancelled",
						},
					},
				}, nil
			}
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{
						OptionId: acp.PermissionOptionId(resp.OptionID),
						Outcome:  "selected",
					},
				},
			}, nil
		case <-ctx.Done():
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Cancelled: &acp.RequestPermissionOutcomeCancelled{
						Outcome: "cancelled",
					},
				},
			}, nil
		}
	}

	// 自动模式：查找 allow_once 选项
	for _, opt := range params.Options {
		if opt.Kind == acp.PermissionOptionKindAllowOnce {
			c.markActivity("permission auto-approved")
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{
						OptionId: opt.OptionId,
						Outcome:  "selected",
					},
				},
			}, nil
		}
	}
	// 默认选第一个选项
	if len(params.Options) > 0 {
		c.markActivity("permission default-first-option")
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Selected: &acp.RequestPermissionOutcomeSelected{
					OptionId: params.Options[0].OptionId,
					Outcome:  "selected",
				},
			},
		}, nil
	}
	return acp.RequestPermissionResponse{}, nil
}

// RespondPermission 从外部注入权限回复（由 agent.go 调用）
func (c *ACPClientImpl) RespondPermission(optionID string, cancelled bool) {
	if c.permissionCh != nil {
		c.permissionCh <- permissionResponse{OptionID: optionID, Cancelled: cancelled}
	}
}

// CreateTerminal 创建终端
func (c *ACPClientImpl) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, fmt.Errorf("terminal not supported")
}

// KillTerminalCommand 终止终端命令
func (c *ACPClientImpl) KillTerminalCommand(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, fmt.Errorf("terminal not supported")
}

// TerminalOutput 终端输出
func (c *ACPClientImpl) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, fmt.Errorf("terminal not supported")
}

// ReleaseTerminal 释放终端
func (c *ACPClientImpl) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, fmt.Errorf("terminal not supported")
}

// WaitForTerminalExit 等待终端退出
func (c *ACPClientImpl) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, fmt.Errorf("terminal not supported")
}

// StartACPSession 启动 ACP 会话（WriteTextFile 始终启用）
// extraArgs 追加到 cfg.ACPAgentArgs 后面，用于传递动态 CLI 参数
func StartACPSession(ctx context.Context, cfg *AgentConfig, projectPath string, extraArgs []string) (*ACPSession, *ACPClientImpl, error) {
	ctx, cancel := context.WithCancel(ctx)

	// 拼接基础参数 + 动态参数
	allArgs := append([]string{}, cfg.ACPAgentArgs...)

	// 解析 --settings <name>: 转为绝对路径 settings/claudecode/<name>.json
	resolvedExtra := resolveSettingsArgs(extraArgs, cfg.ClaudeCodeSettingsDir)

	// 如果 extraArgs 中没有 --settings，且配置了 default_settings，自动补充
	if cfg.DefaultSettings != "" && !hasSettingsArg(resolvedExtra) {
		name := cfg.DefaultSettings
		if !strings.HasSuffix(name, ".json") {
			name = name + ".json"
		}
		settingsFile := filepath.Join(cfg.ClaudeCodeSettingsDir, name)
		if _, err := os.Stat(settingsFile); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("default settings file not found: %s", settingsFile)
		}
		resolvedExtra = append(resolvedExtra, "--settings", settingsFile)
	}

	allArgs = append(allArgs, resolvedExtra...)

	// 启动 claude-agent-acp 子进程
	cmd := exec.CommandContext(ctx, cfg.ACPAgentCmd, allArgs...)
	cmd.Dir = projectPath
	cmd.Stderr = os.Stderr

	// 读取 settings 文件：打印 model + 自动映射 ANTHROPIC_AUTH_TOKEN → ANTHROPIC_API_KEY
	if settingsPath := extractSettingsPath(allArgs); settingsPath != "" {
		applySettingsEnv(cmd, settingsPath)
	} else {
		log.Printf("[ACP] warning: no --settings provided, ACP child process may have no authentication env")
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("start acp agent: %v", err)
	}

	log.Printf("[ACP] started %s %s (pid=%d, dir=%s)", cfg.ACPAgentCmd, strings.Join(allArgs, " "), cmd.Process.Pid, projectPath)

	// 创建 ACP Client 实现
	client := NewACPClientImpl(projectPath)

	// 建立 ACP 连接
	conn := acp.NewClientSideConnection(client, io.Writer(stdin), io.Reader(stdout))

	// Initialize 握手（WriteTextFile 始终为 true）
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientInfo: &acp.Implementation{
			Name:    "acp-agent",
			Version: "1.0.0",
		},
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: false,
		},
	})
	if err != nil {
		cancel()
		cmd.Process.Kill()
		cmd.Wait()
		return nil, nil, fmt.Errorf("acp initialize: %v", err)
	}

	log.Printf("[ACP] initialized: agent=%s version=%s protocol=%d",
		initResp.AgentInfo.Name, initResp.AgentInfo.Version, initResp.ProtocolVersion)

	// 创建会话
	sessResp, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        projectPath,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		cancel()
		cmd.Process.Kill()
		cmd.Wait()
		return nil, nil, fmt.Errorf("acp new session: %v", err)
	}

	log.Printf("[ACP] session created: id=%s", sessResp.SessionId)

	// 保存可用模式列表 + 当前模式/模型
	if sessResp.Modes != nil {
		client.mu.Lock()
		client.availableModes = sessResp.Modes.AvailableModes
		if sessResp.Modes.CurrentModeId != "" {
			client.currentModeID = string(sessResp.Modes.CurrentModeId)
		}
		client.mu.Unlock()
		log.Printf("[ACP] available modes: %d, current mode: %s", len(sessResp.Modes.AvailableModes), sessResp.Modes.CurrentModeId)
		for _, m := range sessResp.Modes.AvailableModes {
			log.Printf("[ACP]   mode: id=%s name=%s", m.Id, m.Name)
		}
	}
	if sessResp.Models != nil && sessResp.Models.CurrentModelId != "" {
		client.mu.Lock()
		client.modelID = string(sessResp.Models.CurrentModelId)
		client.mu.Unlock()
		log.Printf("[ACP] current model: %s", sessResp.Models.CurrentModelId)
	}

	session := &ACPSession{
		cmd:       cmd,
		conn:      conn,
		sessionID: sessResp.SessionId,
		cancel:    cancel,
	}

	return session, client, nil
}

func previewText(text string, limit int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r", " "))
	text = strings.ReplaceAll(text, "\n", " ")
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

// buildToolDetail 构建 tool_detail 文本（工具调用的详细信息）
func buildToolDetail(tc *acp.SessionUpdateToolCall) string {
	var parts []string
	if tc.Kind != "" {
		parts = append(parts, fmt.Sprintf("kind=%s", tc.Kind))
	}
	// 提取文件路径
	if len(tc.Locations) > 0 {
		var files []string
		for _, loc := range tc.Locations {
			files = append(files, loc.Path)
		}
		parts = append(parts, fmt.Sprintf("files=%s", strings.Join(files, ",")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "   └─ " + strings.Join(parts, " ")
}

// buildToolUpdateText 构建 tool_update 文本
func buildToolUpdateText(tu *acp.SessionToolCallUpdate) string {
	title := ""
	if tu.Title != nil {
		title = *tu.Title
	}
	status := ""
	if tu.Status != nil {
		status = string(*tu.Status)
	}
	if title == "" && status == "" {
		return ""
	}
	// 提取文件路径
	var files []string
	for _, loc := range tu.Locations {
		files = append(files, loc.Path)
	}
	result := fmt.Sprintf("🔧 %s [%s]", title, status)
	if len(files) > 0 {
		result += fmt.Sprintf(" (%s)", strings.Join(files, ", "))
	}
	return result
}

// formatPlan 格式化 PlanEntry[] 为可读文本
func formatPlan(entries []acp.PlanEntry) string {
	if len(entries) == 0 {
		return "📋 执行计划: (空)"
	}
	var sb strings.Builder
	sb.WriteString("📋 执行计划:\n")
	for i, entry := range entries {
		var icon string
		switch entry.Status {
		case acp.PlanEntryStatusCompleted:
			icon = "✅"
		case acp.PlanEntryStatusInProgress:
			icon = "▶"
		default:
			icon = "⏳"
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, icon, entry.Content))
	}
	return sb.String()
}

// resolveSettingsArgs 解析 extraArgs 中的 --settings <name>，转为绝对路径
func resolveSettingsArgs(args []string, settingsDir string) []string {
	if len(args) == 0 {
		return args
	}
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--settings" && i+1 < len(args) {
			name := args[i+1]
			// 如果已经是文件路径（含 / 或 .json 后缀），直接使用
			if strings.Contains(name, "/") || strings.HasSuffix(name, ".json") {
				result = append(result, "--settings", name)
			} else {
				// 短名称 → settings/claudecode/<name>.json
				settingsFile := filepath.Join(settingsDir, name+".json")
				result = append(result, "--settings", settingsFile)
			}
			i++ // 跳过下一个参数
		} else {
			result = append(result, args[i])
		}
	}
	return result
}

// hasSettingsArg 检查参数列表中是否已包含 --settings
func hasSettingsArg(args []string) bool {
	for _, a := range args {
		if a == "--settings" {
			return true
		}
	}
	return false
}

// extractSettingsPath 从参数列表中提取 --settings 文件路径
func extractSettingsPath(args []string) string {
	for i, a := range args {
		if a == "--settings" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// applySettingsEnv 读取 settings 文件，打印 model，并将 env 注入 cmd 环境
// 自动将 ANTHROPIC_AUTH_TOKEN 映射为 ANTHROPIC_API_KEY（兼容第三方代理）
func applySettingsEnv(cmd *exec.Cmd, settingsFile string) {
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		log.Printf("[ACP] warning: cannot read settings file %s: %v", settingsFile, err)
		return
	}

	var settings struct {
		Env   map[string]interface{} `json:"env"`
		Model string                 `json:"model"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("[ACP] warning: cannot parse settings file: %v", err)
		return
	}

	if settings.Model != "" {
		log.Printf("[ACP] settings model: %s", settings.Model)
	}

	if len(settings.Env) == 0 {
		return
	}

	// 将 settings 中的 env 注入 cmd 环境
	cmd.Env = os.Environ()
	authToken := ""
	hasAPIKey := false
	for k, v := range settings.Env {
		val := fmt.Sprintf("%v", v)
		cmd.Env = append(cmd.Env, k+"="+val)
		if k == "ANTHROPIC_AUTH_TOKEN" {
			authToken = val
		}
		if k == "ANTHROPIC_API_KEY" {
			hasAPIKey = true
		}
	}

	// 自动映射：ANTHROPIC_AUTH_TOKEN → ANTHROPIC_API_KEY
	if !hasAPIKey && authToken != "" {
		cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+authToken)
		log.Printf("[ACP] auto-mapped ANTHROPIC_AUTH_TOKEN → ANTHROPIC_API_KEY")
	}
}
