package codegen

import (
	"bufio"
	"encoding/json"
	"fmt"
	log "mylog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// claudeStreamMsg Claude Code stream-json 输出的消息格式
type claudeStreamMsg struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Result    string          `json:"result,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	CostUSD   float64         `json:"cost_usd,omitempty"`
	Duration  float64         `json:"duration_ms,omitempty"`
	TokensIn  int             `json:"input_tokens,omitempty"`
	TokensOut int             `json:"output_tokens,omitempty"`
	NumTurns  int             `json:"num_turns,omitempty"`
}

type claudeContent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Name  string `json:"name,omitempty"`  // tool_use 的工具名
	Input string `json:"input,omitempty"` // tool_use 的参数（JSON string）
	ID    string `json:"id,omitempty"`
}

type claudeAssistantMessage struct {
	Role    string          `json:"role"`
	Content []claudeContent `json:"content"`
}

// RunClaude 启动 claude 子进程执行编码任务
func RunClaude(session *CodeSession) error {
	systemPrompt := "重要：你的工作目录就是当前项目目录，只能在当前目录（.）下操作，" +
		"禁止访问上级目录或其他项目的文件。所有文件操作必须在当前目录内。" +
		"你必须完成完整的开发流程：" +
		"1. 编写代码；" +
		"2. 构建/编译项目（如 go build、npm run build 等），确认无编译错误；" +
		"3. 运行程序并验证输出正确；" +
		"4. 如有测试则运行测试；" +
		"5. 最后汇报结果：创建了哪些文件、构建是否成功、运行输出是什么。" +
		"不要只写代码就停止，必须验证代码能正常工作。"

	args := []string{
		"-p", session.Prompt,
		"--verbose",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
		"--append-system-prompt", systemPrompt,
	}

	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}

	return runClaudeProcess(session, args)
}

// RunClaudeResume 恢复已有会话发送新消息
func RunClaudeResume(session *CodeSession, prompt string) error {
	args := []string{
		"-p", prompt,
		"--verbose",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
	}

	if session.ClaudeSession != "" {
		args = append(args, "--session-id", session.ClaudeSession)
	}

	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}

	return runClaudeProcess(session, args)
}

// runClaudeProcess 执行 claude 命令并解析流式输出
func runClaudeProcess(session *CodeSession, args []string) error {
	projectPath, err := ResolveProjectPath(session.Project)
	if err != nil {
		projectPath = filepath.Join(GetDefaultWorkspace(), session.Project)
	}
	log.MessageF(log.ModuleAgent, "CodeGen: running %s %s (dir=%s)", claudePath, strings.Join(args, " "), projectPath)

	cmd := exec.Command(claudePath, args...)
	cmd.Dir = projectPath // 通过工作目录设置项目路径

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %v", err)
	}

	// 保存进程引用以便停止
	session.mu.Lock()
	session.process = cmd.Process
	session.mu.Unlock()

	session.broadcast(StreamEvent{
		Type: "system",
		Text: fmt.Sprintf("🔧 Claude Code 开始编码... (项目: %s)", session.Project),
	})

	// 异步读取 stderr，收集错误信息
	var stderrLines []string
	var stderrMu sync.Mutex
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.WarnF(log.ModuleAgent, "CodeGen stderr: %s", line)
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()
			// 实时推送 stderr 给用户
			session.broadcast(StreamEvent{
				Type: "error",
				Text: "⚠️ " + line,
			})
		}
	}()

	// 逐行读取 stream-json
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		event := parseStreamLine(line)
		if event == nil {
			continue
		}

		processEvent(session, event)
		session.broadcast(*event)
	}

	// 等待进程完成
	err = cmd.Wait()

	session.mu.Lock()
	session.EndTime = time.Now()
	session.process = nil
	if err != nil && session.Status == StatusRunning {
		session.Status = StatusError
		stderrMu.Lock()
		if len(stderrLines) > 0 {
			session.Error = strings.Join(stderrLines, "; ")
		} else {
			session.Error = err.Error()
		}
		stderrMu.Unlock()
	} else if session.Status == StatusRunning {
		session.Status = StatusDone
	}
	finalStatus := session.Status
	finalError := session.Error
	session.mu.Unlock()

	// 根据状态发送不同事件
	if finalStatus == StatusError {
		session.broadcast(StreamEvent{
			Type: "error",
			Text: fmt.Sprintf("❌ 编码失败: %s", finalError),
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

	log.MessageF(log.ModuleAgent, "CodeGen session %s finished, status=%s, cost=$%.4f",
		session.ID, session.Status, session.CostUSD)

	return nil
}

// parseStreamLine 解析一行 stream-json 输出
func parseStreamLine(line string) *StreamEvent {
	var msg claudeStreamMsg
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		log.DebugF(log.ModuleAgent, "CodeGen: skip unparseable line: %s", line)
		return nil
	}

	switch msg.Type {
	case "system":
		return &StreamEvent{
			Type:      "system",
			Text:      "系统初始化",
			SessionID: msg.SessionID,
		}

	case "assistant":
		// 解析 assistant message
		var assistMsg claudeAssistantMessage
		if msg.Message != nil {
			json.Unmarshal(msg.Message, &assistMsg)
		}

		// 遍历 content blocks
		for _, block := range assistMsg.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					return &StreamEvent{
						Type: "assistant",
						Text: block.Text,
					}
				}
			case "tool_use":
				inputStr := block.Input
				// 如果 input 是 JSON 对象，格式化显示
				return &StreamEvent{
					Type:      "tool",
					ToolName:  block.Name,
					ToolInput: inputStr,
					Text:      formatToolAction(block.Name, inputStr),
				}
			}
		}

		// 如果没有 content blocks，尝试用 subtype
		if msg.Subtype != "" {
			return &StreamEvent{
				Type: "system",
				Text: msg.Subtype,
			}
		}

		return nil

	case "result":
		return &StreamEvent{
			Type:      "result",
			Text:      msg.Result,
			SessionID: msg.SessionID,
			CostUSD:   msg.CostUSD,
			TokensIn:  msg.TokensIn,
			TokensOut: msg.TokensOut,
			Duration:  msg.Duration,
			NumTurns:  msg.NumTurns,
			Done:      true,
		}

	case "user":
		// tool_result 消息：解析工具执行结果
		var userMsg struct {
			Role    string `json:"role"`
			Content []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
				IsError bool   `json:"is_error"`
			} `json:"content"`
		}
		if msg.Message != nil {
			json.Unmarshal(msg.Message, &userMsg)
		}
		for _, block := range userMsg.Content {
			if block.Type == "tool_result" && block.Content != "" {
				// 截断过长的结果
				text := block.Content
				if len(text) > 500 {
					text = text[:500] + "..."
				}
				eventType := "system"
				if block.IsError {
					eventType = "error"
					text = "⚠️ " + text
				}
				return &StreamEvent{
					Type: eventType,
					Text: text,
				}
			}
		}
		// 也检查 tool_use_result 字段
		// 尝试从顶层解析
		raw, _ := json.Marshal(msg)
		var topLevel map[string]interface{}
		json.Unmarshal(raw, &topLevel)
		if tr, ok := topLevel["tool_use_result"]; ok {
			if trMap, ok := tr.(map[string]interface{}); ok {
				if stdout, ok := trMap["stdout"].(string); ok && stdout != "" {
					text := stdout
					if len(text) > 500 {
						text = text[:500] + "..."
					}
					return &StreamEvent{Type: "system", Text: text}
				}
			}
			if trStr, ok := tr.(string); ok && trStr != "" {
				text := trStr
				if len(text) > 500 {
					text = text[:500] + "..."
				}
				return &StreamEvent{Type: "error", Text: trStr}
			}
		}
		return nil // 忽略无内容的 user 事件

	default:
		// 忽略其他事件类型（不显示原始 JSON）
		log.DebugF(log.ModuleAgent, "CodeGen: skip event type=%s", msg.Type)
		return nil
	}
}

// processEvent 根据事件更新会话状态
func processEvent(session *CodeSession, event *StreamEvent) {
	if event.SessionID != "" {
		session.mu.Lock()
		session.ClaudeSession = event.SessionID
		session.mu.Unlock()
	}

	if event.CostUSD > 0 {
		session.mu.Lock()
		session.CostUSD = event.CostUSD
		session.mu.Unlock()
	}

	// 记录到消息历史
	switch event.Type {
	case "assistant":
		session.addMessage(SessionMessage{
			Role:    "assistant",
			Content: event.Text,
			Time:    time.Now(),
		})
	case "tool":
		session.addMessage(SessionMessage{
			Role:      "tool",
			Content:   event.Text,
			ToolName:  event.ToolName,
			ToolInput: event.ToolInput,
			Time:      time.Now(),
		})
	}
}

// formatToolAction 格式化工具操作为可读文本
func formatToolAction(toolName, input string) string {
	// 尝试提取关键参数
	var args map[string]interface{}
	json.Unmarshal([]byte(input), &args)

	switch toolName {
	case "Write", "write_file", "write":
		if path, ok := args["file_path"].(string); ok {
			return fmt.Sprintf("✏️ 写入 %s", path)
		}
		return "✏️ 写入文件"
	case "Read", "read_file", "read":
		if path, ok := args["file_path"].(string); ok {
			return fmt.Sprintf("📖 读取 %s", path)
		}
		return "📖 读取文件"
	case "Edit", "edit_file":
		if path, ok := args["file_path"].(string); ok {
			return fmt.Sprintf("✏️ 编辑 %s", path)
		}
		return "✏️ 编辑文件"
	case "Bash", "bash", "run_command":
		if cmd, ok := args["command"].(string); ok {
			if len(cmd) > 80 {
				cmd = cmd[:80] + "..."
			}
			return fmt.Sprintf("💻 执行: %s", cmd)
		}
		return "💻 执行命令"
	case "list_dir", "ListDir":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📁 列出 %s", path)
		}
		return "📁 列出目录"
	case "Search", "search", "Grep":
		if pattern, ok := args["pattern"].(string); ok {
			return fmt.Sprintf("🔍 搜索: %s", pattern)
		}
		return "🔍 搜索文件"
	default:
		return fmt.Sprintf("🔧 %s", toolName)
	}
}
