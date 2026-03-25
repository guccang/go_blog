package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// bridgePython 注入到每个 Python 子进程的桥接代码
const bridgePython = `import sys, json

def _auto_parse(data):
    """自动将 JSON 字符串解析为 dict/list，纯文本保持原样"""
    if isinstance(data, str):
        try:
            parsed = json.loads(data)
            if isinstance(parsed, (dict, list)):
                return parsed
        except (json.JSONDecodeError, ValueError):
            pass
    return data

def call_tool(tool_name, arguments=None):
    """调用 MCP 工具 - 通过 stdin/stdout 协议与 agent 通信
    直接返回工具结果值：
    - 工具返回 str   → "2026-03-24"
    - 工具返回 dict  → {"id": "xxx", ...}
    - 工具返回 list  → [...]
    """
    request = json.dumps({"type": "tool_call", "tool": tool_name, "args": arguments or {}})
    print(f"__TOOL_CALL__{request}__END__", flush=True)
    line = sys.stdin.readline().strip()
    if not line:
        raise Exception(f"Tool {tool_name}: no response (agent disconnected?)")
    try:
        result = json.loads(line)
    except (json.JSONDecodeError, ValueError) as e:
        raise Exception(f"Tool {tool_name}: invalid JSON response: {e} raw={line[:200]}")
    if not result.get("success"):
        raise Exception(f"Tool {tool_name} failed: {result.get('error', 'unknown')}")
    return _auto_parse(result.get("data"))

def safe_call_tool(tool_name, arguments=None, default=None):
    """调用 MCP 工具（失败时返回 default 而不是抛异常）"""
    try:
        return call_tool(tool_name, arguments)
    except Exception as e:
        print(f"[WARN] {tool_name} failed: {e}", file=sys.stderr)
        return default

# ===== user code below =====
`

const toolCallPrefix = "__TOOL_CALL__"
const toolCallSuffix = "__END__"

// Executor Python 沙箱执行引擎
type Executor struct {
	cfg      *Config
	callTool func(toolName string, args json.RawMessage) (result string, agentID string, err error)
}

// NewExecutor 创建执行器
func NewExecutor(cfg *Config) *Executor {
	return &Executor{cfg: cfg}
}

// Execute 在 Python 沙箱中执行代码
func (e *Executor) Execute(code string) *ExecutionResult {
	result := &ExecutionResult{ToolCalls: []ToolCallRecord{}}
	start := time.Now()

	// 拼接桥接代码 + 用户代码
	fullCode := bridgePython + "\n" + code

	// 写入临时文件（解决 Windows 命令行编码问题，中文代码通过 -c 传递可能损坏）
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "exec_*.py")
	if err != nil {
		result.Success = false
		result.ErrorType = "runtime"
		result.Stderr = fmt.Sprintf("failed to create temp file: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// 写入 UTF-8 编码的 Python 代码（添加 coding 声明确保 Python 正确解读）
	codingHeader := "# -*- coding: utf-8 -*-\n"
	if _, err := tmpFile.WriteString(codingHeader + fullCode); err != nil {
		tmpFile.Close()
		result.Success = false
		result.ErrorType = "runtime"
		result.Stderr = fmt.Sprintf("failed to write temp file: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}
	tmpFile.Close()

	log.Printf("[Executor] code written to temp file: %s (%d bytes)", filepath.Base(tmpPath), len(codingHeader)+len(fullCode))

	// 启动 Python 子进程（带超时）
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(e.cfg.MaxExecTimeSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.cfg.PythonPath, "-u", tmpPath)
	// 设置 UTF-8 编码环境变量，确保 stdin/stdout/stderr 使用 UTF-8
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		result.Success = false
		result.ErrorType = "runtime"
		result.Stderr = fmt.Sprintf("failed to create stdin pipe: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Success = false
		result.ErrorType = "runtime"
		result.Stderr = fmt.Sprintf("failed to create stdout pipe: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Success = false
		result.ErrorType = "runtime"
		result.Stderr = fmt.Sprintf("failed to create stderr pipe: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if err := cmd.Start(); err != nil {
		result.Success = false
		result.ErrorType = "runtime"
		result.Stderr = fmt.Sprintf("failed to start python: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// 异步读取 stderr
	stderrCh := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(stderr)
		stderrCh <- string(data)
	}()

	// 逐行扫描 stdout
	scanner := bufio.NewScanner(stdout)
	// 增大 scanner buffer 以处理大行（工具调用结果可能很大）
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	var output strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, toolCallPrefix) && strings.HasSuffix(line, toolCallSuffix) {
			// 解析工具调用请求
			jsonStr := line[len(toolCallPrefix) : len(line)-len(toolCallSuffix)]
			var req toolCallRequest
			if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
				log.Printf("[Executor] invalid tool_call json: %v", err)
				// 返回错误给 Python
				resp := toolCallResponse{Success: false, Error: "invalid tool_call json"}
				respData, _ := json.Marshal(resp)
				stdin.Write(append(respData, '\n'))
				continue
			}

			// 拦截虚拟工具（这些工具仅在 LLM Agent 主循环中处理，不可在 ExecuteCode 中调用）
			if isVirtualTool(req.Tool) {
				log.Printf("[Executor] blocked virtual tool call: %s", req.Tool)
				resp := toolCallResponse{
					Success: false,
					Error:   fmt.Sprintf("%s 是虚拟工具，不能在 ExecuteCode 中调用。请直接使用具体工具（如 RawAddTodo, RawGetTodosByDate 等）", req.Tool),
				}
				respData, _ := json.Marshal(resp)
				stdin.Write(append(respData, '\n'))

				// 记录为失败调用
				result.ToolCalls = append(result.ToolCalls, ToolCallRecord{
					Tool:    req.Tool,
					Success: false,
					Error:   resp.Error,
				})
				continue
			}

			// 通过 UAP 调用真正的 MCP 工具
			toolStart := time.Now()
			toolResult, toolAgentID, toolErr := e.callTool(req.Tool, req.Args)
			toolDuration := time.Since(toolStart)

			// 记录工具调用（无论成败）
			record := ToolCallRecord{
				Tool:     req.Tool,
				AgentID:  toolAgentID,
				Success:  toolErr == nil,
				Duration: toolDuration.Milliseconds(),
			}
			if toolErr != nil {
				record.Error = toolErr.Error()
			}
			result.ToolCalls = append(result.ToolCalls, record)

			log.Printf("[Executor] tool_call %s success=%v duration=%dms",
				req.Tool, toolErr == nil, toolDuration.Milliseconds())

			// 构建返回给 Python 的响应
			var resp toolCallResponse
			if toolErr != nil {
				resp = toolCallResponse{
					Success: false,
					Error:   toolErr.Error(),
				}
			} else {
				// 保持原始 JSON — 如果 toolResult 是合法 JSON 则原样传递
				rawData := tryParseRawJSON(toolResult)
				resp = toolCallResponse{
					Success: true,
					Data:    rawData,
				}
			}

			respData, _ := json.Marshal(resp)
			log.Printf("[Executor] tool_response → stdin tool=%s success=%v len=%d",
				req.Tool, resp.Success, len(respData))
			if _, writeErr := stdin.Write(append(respData, '\n')); writeErr != nil {
				log.Printf("[Executor] write to stdin failed: %v", writeErr)
				break
			}
		} else {
			// 普通 print → 收集为最终输出
			if output.Len() >= e.cfg.MaxOutputSize {
				result.Truncated = true
				continue // 不再追加
			}
			output.WriteString(line + "\n")
		}
	}

	// 关闭 stdin，让 Python 可以正常退出
	stdin.Close()

	// 等待进程结束
	cmd.Wait()

	// 收集结果
	result.DurationMs = time.Since(start).Milliseconds()
	result.Stdout = output.String()
	result.Stderr = <-stderrCh

	// 获取退出码
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	} else {
		result.ExitCode = -1
	}

	// === 结果验证与错误分类 ===
	if ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.ErrorType = "timeout"
	} else if result.ExitCode != 0 {
		result.Success = false
		if strings.Contains(result.Stderr, "SyntaxError") {
			result.ErrorType = "syntax"
		} else {
			result.ErrorType = "runtime"
		}
	} else {
		result.Success = true
		if result.Truncated {
			result.ErrorType = "output_truncated"
		}
	}

	return result
}

// isVirtualTool 判断是否为虚拟工具（仅在 LLM Agent 主循环中处理，不可在 ExecuteCode 中调用）
func isVirtualTool(name string) bool {
	switch name {
	case "execute_skill", "plan_and_execute", "set_persona", "set_rule":
		return true
	}
	return false
}

// tryParseRawJSON 尝试将字符串解析为原始 JSON，失败则包装为字符串
func tryParseRawJSON(s string) json.RawMessage {
	// 先检查是否是合法的 JSON
	s = strings.TrimSpace(s)
	if len(s) > 0 && (s[0] == '{' || s[0] == '[' || s[0] == '"') {
		if json.Valid([]byte(s)) {
			return json.RawMessage(s)
		}
	}
	// 不是 JSON，包装为字符串
	data, _ := json.Marshal(s)
	return json.RawMessage(data)
}

// truncate 截断字符串到指定长度（UTF-8 安全，按字符数截断）
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "...[truncated]"
}

// errStr 将 error 转为字符串（nil → ""）
func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
