package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

// ACPSession 一次 ACP 会话
type ACPSession struct {
	cmd       *exec.Cmd
	conn      *acp.ClientSideConnection
	sessionID acp.SessionId
	cancel    context.CancelFunc
}

// Close 关闭 ACP 会话
func (s *ACPSession) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
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
}

// NewACPClientImpl 创建 ACP Client 实现
func NewACPClientImpl(projectPath string) *ACPClientImpl {
	return &ACPClientImpl{
		projectPath: projectPath,
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

// SessionUpdate 处理 session/update 通知（核心：收集 agent 输出 + 推送事件）
func (c *ACPClientImpl) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	update := params.Update

	if update.AgentMessageChunk != nil {
		if update.AgentMessageChunk.Content.Text != nil {
			text := update.AgentMessageChunk.Content.Text.Text
			c.mu.Lock()
			c.chunks = append(c.chunks, text)
			cb := c.streamCb
			c.mu.Unlock()

			if cb != nil {
				cb(StreamEvent{Type: "assistant", Text: text})
			}
		}
	}

	if update.ToolCall != nil {
		log.Printf("[ACP] tool_call: %s (status=%s)", update.ToolCall.Title, update.ToolCall.Status)

		c.mu.Lock()
		cb := c.streamCb
		c.mu.Unlock()

		if cb != nil {
			cb(StreamEvent{
				Type:     "tool",
				ToolName: update.ToolCall.Title,
				Text:     fmt.Sprintf("🔧 %s", update.ToolCall.Title),
			})
		}
	}

	if update.ToolCallUpdate != nil {
		if update.ToolCallUpdate.Status != nil {
			log.Printf("[ACP] tool_call_update: status=%s", *update.ToolCallUpdate.Status)
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

	log.Printf("[ACP] write_file: %s (new=%v)", filePath, isNew)

	return acp.WriteTextFileResponse{}, nil
}

// RequestPermission 自动允许所有操作
func (c *ACPClientImpl) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// 查找 allow_once 选项
	for _, opt := range params.Options {
		if opt.Kind == acp.PermissionOptionKindAllowOnce {
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
func StartACPSession(ctx context.Context, cfg *AgentConfig, projectPath string) (*ACPSession, *ACPClientImpl, error) {
	ctx, cancel := context.WithCancel(ctx)

	// 启动 claude-agent-acp 子进程
	cmd := exec.CommandContext(ctx, cfg.ACPAgentCmd, cfg.ACPAgentArgs...)
	cmd.Dir = projectPath
	cmd.Stderr = os.Stderr

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

	log.Printf("[ACP] started %s %s (pid=%d, dir=%s)", cfg.ACPAgentCmd, strings.Join(cfg.ACPAgentArgs, " "), cmd.Process.Pid, projectPath)

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

	session := &ACPSession{
		cmd:       cmd,
		conn:      conn,
		sessionID: sessResp.SessionId,
		cancel:    cancel,
	}

	return session, client, nil
}
