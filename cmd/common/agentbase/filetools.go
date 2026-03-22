package agentbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"uap"
)

// ProjectResolver 根据项目名称返回项目根目录绝对路径
// 返回空字符串表示项目不存在
type ProjectResolver func(project string) string

// FileToolKit 文件读写 + Bash 执行工具包
// 通过 UAP 工具机制暴露给 llm-agent 调用
type FileToolKit struct {
	prefix      string          // 工具名前缀，如 "Codegen" / "Deploy"
	agentLabel  string          // agent 标识，如 "codegen-agent"，用于工具描述
	resolver    ProjectResolver // 项目路径解析器
	bashTimeout time.Duration   // bash 默认超时（默认 60s）
}

// NewFileToolKit 创建 FileToolKit 实例
func NewFileToolKit(prefix string, agentLabel string, resolver ProjectResolver) *FileToolKit {
	return &FileToolKit{
		prefix:      prefix,
		agentLabel:  agentLabel,
		resolver:    resolver,
		bashTimeout: 60 * time.Second,
	}
}

// SetBashTimeout 设置 bash 默认超时时间
func (ft *FileToolKit) SetBashTimeout(d time.Duration) {
	ft.bashTimeout = d
}

// ToolDefs 返回 4 个 UAP 工具定义
func (ft *FileToolKit) ToolDefs() []uap.ToolDef {
	// 根据 agentLabel 生成带 agent 上下文的描述
	readFileDesc := "读取项目中的文件内容"
	writeFileDesc := "写入文件到项目中（自动创建父目录）"
	execBashDesc := "在项目目录中执行 bash/cmd 命令"
	execEnvBashDesc := "执行环境检测/安装命令（不限项目目录，用于 env-agent 远程调用）"
	if ft.agentLabel != "" {
		readFileDesc = fmt.Sprintf("在 %s 上读取项目中的文件内容", ft.agentLabel)
		writeFileDesc = fmt.Sprintf("在 %s 上写入文件到项目中（自动创建父目录）", ft.agentLabel)
		execBashDesc = fmt.Sprintf("在 %s 上的项目目录中执行 bash/cmd 命令", ft.agentLabel)
		execEnvBashDesc = fmt.Sprintf("在 %s 上执行环境检测/安装命令（不限项目目录，用于 env-agent 远程调用）", ft.agentLabel)
	}

	return []uap.ToolDef{
		{
			Name:        ft.prefix + "ReadFile",
			Description: readFileDesc,
			Parameters: MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "项目名称"},
					"path":    map[string]interface{}{"type": "string", "description": "文件相对路径（相对于项目根目录）"},
				},
				"required": []string{"project", "path"},
			}),
		},
		{
			Name:        ft.prefix + "WriteFile",
			Description: writeFileDesc,
			Parameters: MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "项目名称"},
					"path":    map[string]interface{}{"type": "string", "description": "文件相对路径（相对于项目根目录）"},
					"content": map[string]interface{}{"type": "string", "description": "文件内容"},
				},
				"required": []string{"project", "path", "content"},
			}),
		},
		{
			Name:        ft.prefix + "ExecBash",
			Description: execBashDesc,
			Parameters: MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "项目名称"},
					"command": map[string]interface{}{"type": "string", "description": "要执行的命令"},
					"timeout": map[string]interface{}{"type": "integer", "description": "超时秒数（可选，默认60，上限300）"},
				},
				"required": []string{"project", "command"},
			}),
		},
		{
			Name:        ft.prefix + "ExecEnvBash",
			Description: execEnvBashDesc,
			Parameters: MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string", "description": "要执行的命令"},
					"timeout": map[string]interface{}{"type": "integer", "description": "超时秒数（可选，默认120，上限600）"},
					"workdir": map[string]interface{}{"type": "string", "description": "工作目录（可选，默认 /tmp）"},
				},
				"required": []string{"command"},
			}),
		},
	}
}

// HandleTool 处理工具调用，返回 (result_json, handled)
// handled=false 表示该 toolName 不属于 FileToolKit
func (ft *FileToolKit) HandleTool(toolName string, args map[string]interface{}) (string, bool) {
	switch toolName {
	case ft.prefix + "ReadFile":
		return ft.toolReadFile(args), true
	case ft.prefix + "WriteFile":
		return ft.toolWriteFile(args), true
	case ft.prefix + "ExecBash":
		return ft.toolExecBash(args), true
	case ft.prefix + "ExecEnvBash":
		return ft.toolExecEnvBash(args), true
	default:
		return "", false
	}
}

// validatePath 安全校验：确保路径在项目目录内
// 返回 (绝对路径, 错误信息)
func (ft *FileToolKit) validatePath(project, path string) (string, string) {
	if project == "" {
		return "", "缺少 project 参数"
	}
	if path == "" {
		return "", "缺少 path 参数"
	}

	projectPath := ft.resolver(project)
	if projectPath == "" {
		return "", fmt.Sprintf("项目 %q 不存在", project)
	}

	fullPath := filepath.Join(projectPath, path)
	absProject, _ := filepath.Abs(projectPath)
	absFile, _ := filepath.Abs(fullPath)
	rel, err := filepath.Rel(absProject, absFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "无效的文件路径（不允许访问项目目录之外）"
	}

	return fullPath, ""
}

// toolReadFile 读取文件内容
func (ft *FileToolKit) toolReadFile(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	path, _ := args["path"].(string)

	fullPath, errMsg := ft.validatePath(project, path)
	if errMsg != "" {
		return marshalResult(false, errMsg, nil)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return marshalResult(false, err.Error(), nil)
	}

	return marshalResult(true, "", map[string]interface{}{
		"content": string(data),
		"size":    len(data),
	})
}

// toolWriteFile 写入文件（自动创建父目录）
func (ft *FileToolKit) toolWriteFile(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	fullPath, errMsg := ft.validatePath(project, path)
	if errMsg != "" {
		return marshalResult(false, errMsg, nil)
	}

	// 自动创建父目录
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return marshalResult(false, fmt.Sprintf("创建目录失败: %v", err), nil)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return marshalResult(false, fmt.Sprintf("写入失败: %v", err), nil)
	}

	return marshalResult(true, "", map[string]interface{}{
		"path": path,
		"size": len(content),
	})
}

// toolExecBash 在项目目录中执行命令
func (ft *FileToolKit) toolExecBash(args map[string]interface{}) string {
	project, _ := args["project"].(string)
	command, _ := args["command"].(string)

	if project == "" {
		return marshalResult(false, "缺少 project 参数", nil)
	}
	if command == "" {
		return marshalResult(false, "缺少 command 参数", nil)
	}

	projectPath := ft.resolver(project)
	if projectPath == "" {
		return marshalResult(false, fmt.Sprintf("项目 %q 不存在", project), nil)
	}

	// 超时处理
	timeout := ft.bashTimeout
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}
	// 硬上限 300s
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if bashPath, err := exec.LookPath("bash"); err == nil {
			cmd = exec.CommandContext(ctx, bashPath, "-c", command)
		} else {
			cmd = exec.CommandContext(ctx, "cmd", "/C", command)
		}
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}
	cmd.Dir = projectPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]interface{}{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": 0,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return marshalResult(false, fmt.Sprintf("命令超时（%ds）", int(timeout.Seconds())), result)
		} else {
			return marshalResult(false, err.Error(), result)
		}
	}

	return marshalResult(true, "", result)
}

// toolExecEnvBash 执行环境检测/安装命令（不限项目目录）
func (ft *FileToolKit) toolExecEnvBash(args map[string]interface{}) string {
	command, _ := args["command"].(string)
	if command == "" {
		return marshalResult(false, "缺少 command 参数", nil)
	}

	// 工作目录：默认 /tmp（Windows 用 os.TempDir()）
	workdir, _ := args["workdir"].(string)
	if workdir == "" {
		if runtime.GOOS == "windows" {
			workdir = os.TempDir()
		} else {
			workdir = "/tmp"
		}
	}

	// 超时处理：默认 120s，上限 600s
	timeout := 120 * time.Second
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}
	if timeout > 600*time.Second {
		timeout = 600 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if bashPath, err := exec.LookPath("bash"); err == nil {
			cmd = exec.CommandContext(ctx, bashPath, "-c", command)
		} else {
			cmd = exec.CommandContext(ctx, "cmd", "/C", command)
		}
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]interface{}{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": 0,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return marshalResult(false, fmt.Sprintf("命令超时（%ds）", int(timeout.Seconds())), result)
		} else {
			return marshalResult(false, err.Error(), result)
		}
	}

	return marshalResult(true, "", result)
}

// marshalResult 构建标准返回 JSON
func marshalResult(success bool, errMsg string, data map[string]interface{}) string {
	result := make(map[string]interface{})
	result["success"] = success
	if errMsg != "" {
		result["error"] = errMsg
	}
	if data != nil {
		for k, v := range data {
			result[k] = v
		}
	}
	b, err := json.Marshal(result)
	if err != nil {
		return `{"success":false,"error":"internal marshal error"}`
	}
	return string(b)
}

// MustMarshalJSON 将值序列化为 JSON RawMessage，失败时返回空对象
// 公共版本，供各 agent 复用
func MustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
