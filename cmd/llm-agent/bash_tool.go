package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// 危险命令黑名单
var bashBlacklist = []string{
	"rm -rf /",
	"rm -rf /*",
	"shutdown",
	"reboot",
	"mkfs",
	"dd if=",
	":(){:|:&};:",
	"> /dev/sda",
	"chmod -R 777 /",
}

// BashToolManager 内置 Bash 工具管理器
type BashToolManager struct {
	Timeout   time.Duration // 命令超时（默认 30s）
	MaxOutput int           // 输出截断字节数（默认 100KB）
}

// Exec 执行 bash 命令
func (m *BashToolManager) Exec(command, workDir string) (string, error) {
	// 安全检查
	cmdLower := strings.ToLower(strings.TrimSpace(command))
	for _, blocked := range bashBlacklist {
		if strings.Contains(cmdLower, blocked) {
			return "", fmt.Errorf("命令被安全策略拦截: 包含危险操作 '%s'", blocked)
		}
	}

	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxOutput := m.MaxOutput
	if maxOutput <= 0 {
		maxOutput = 102400
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// 合并 stdout + stderr
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	log.Printf("[BashTool] 执行: %s (workDir=%s timeout=%v)", command, workDir, timeout)
	start := time.Now()

	err := cmd.Run()
	duration := time.Since(start)
	output := buf.String()

	// 输出截断
	if len(output) > maxOutput {
		output = output[:maxOutput] + fmt.Sprintf("\n...[输出已截断，共 %d 字节]", len(output))
	}

	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[BashTool] 超时: %s (%.1fs)", command, duration.Seconds())
		return output, fmt.Errorf("命令执行超时（%v）", timeout)
	}

	if err != nil {
		log.Printf("[BashTool] 失败: %s (%.1fs) error=%v output=%s", command, duration.Seconds(), err, truncate(output, 200))
		// 返回 output + error，让 LLM 看到 stderr 信息
		return output, fmt.Errorf("exit: %v", err)
	}

	log.Printf("[BashTool] 完成: %s (%.1fs) outputLen=%d", command, duration.Seconds(), len(output))
	return output, nil
}

// ToolDefs 返回 LLM 工具定义
func (m *BashToolManager) ToolDefs() []LLMTool {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "要执行的 bash 命令"
			},
			"work_dir": {
				"type": "string",
				"description": "工作目录（可选，默认当前目录）"
			}
		},
		"required": ["command"]
	}`)

	return []LLMTool{
		{
			Type: "function",
			Function: LLMFunction{
				Name:        "Bash",
				Description: "执行 bash 命令。本地命令直接执行，远程命令通过 ssh 执行（如 ssh root@1.2.3.4 'ls -la'）",
				Parameters:  params,
			},
		},
	}
}

// HandleTool 处理工具调用，返回 (result, handled)
func (m *BashToolManager) HandleTool(toolName string, args map[string]interface{}) (string, bool) {
	if toolName != "Bash" {
		return "", false
	}

	command, _ := args["command"].(string)
	if command == "" {
		return "错误: command 参数不能为空", true
	}

	workDir, _ := args["work_dir"].(string)

	output, err := m.Exec(command, workDir)
	if err != nil {
		if output != "" {
			return fmt.Sprintf("%s\n[错误] %v", output, err), true
		}
		return fmt.Sprintf("[错误] %v", err), true
	}

	if output == "" {
		return "(无输出)", true
	}
	return output, true
}
