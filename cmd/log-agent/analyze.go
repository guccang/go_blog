package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"agentbase"
)

// 日志管道允许的命令白名单
var allowedLogCommands = map[string]bool{
	"grep": true, "egrep": true, "fgrep": true,
	"awk": true, "sed": true,
	"sort": true, "uniq": true,
	"wc": true, "head": true, "tail": true,
	"cut": true, "tr": true, "cat": true,
	"column": true, "paste": true,
	"echo": true, "printf": true, "tee": true, "xargs": true,
}

// 禁止的 shell 控制模式
var blockedPatterns = []string{
	";", "&&", "||", "`", "$(", "${", ">>", "<<", "&>",
}

// validateLogPipeline 校验管道命令是否安全
func validateLogPipeline(command string) error {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return fmt.Errorf("command 不能为空")
	}

	for _, p := range blockedPatterns {
		if strings.Contains(cmd, p) {
			return fmt.Errorf("管道命令包含禁止的控制模式: %q", p)
		}
	}

	segments := strings.Split(cmd, "|")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		cmdName := fields[0]
		if strings.Contains(cmdName, "=") {
			continue
		}
		if !allowedLogCommands[cmdName] {
			return fmt.Errorf("不允许的命令: %q，只支持: grep, awk, sed, sort, uniq, wc, head, tail, cut, tr, cat, column, paste, echo, printf, tee, xargs", cmdName)
		}
	}

	return nil
}

// executeLogPipeline 执行日志管道命令
func executeLogPipeline(content, command string) string {
	if err := validateLogPipeline(command); err != nil {
		return marshalPipelineResult(false, err.Error(), nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	cmd.Stdin = strings.NewReader(content)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return marshalPipelineResult(false, "管道命令执行超时(30s)", map[string]interface{}{
				"stderr":  stderr.String(),
				"command": command,
			})
		} else {
			return marshalPipelineResult(false, fmt.Sprintf("管道命令执行失败: %v", err), map[string]interface{}{
				"stderr":  stderr.String(),
				"command": command,
			})
		}
	}

	out := stdout.String()
	truncated := false
	const maxOutputBytes = 100 * 1024
	if len(out) > maxOutputBytes {
		out = out[:maxOutputBytes]
		truncated = true
	}

	return marshalPipelineResult(true, "", map[string]interface{}{
		"stdout":     out,
		"stderr":     stderr.String(),
		"exit_code":  exitCode,
		"truncated":  truncated,
		"output_len": len(out),
	})
}

// buildAnalyzeCommand 根据预设分析模板构造管道命令
func buildAnalyzeCommand(analysis string, keyword string, field int, topN int, interval string) (string, error) {
	switch analysis {
	case "top_errors":
		grep := "grep -i"
		if keyword != "" {
			grep = fmt.Sprintf("grep -i '%s'", strings.ReplaceAll(keyword, "'", "'\\''"))
		} else {
			grep = "grep -iE 'error|fatal|panic|fail'"
		}
		return fmt.Sprintf("cat | %s | sed 's/^\\[[^]]*\\] //' | sed 's/[0-9]\\{4\\}[/][0-9]\\{2\\}[/][0-9]\\{2\\}[ ][0-9]\\{2\\}:[0-9]\\{2\\}:[0-9]\\{2\\}[.0-9]* //' | sort | uniq -c | sort -rn | head -%d", grep, topN), nil

	case "error_timeline":
		bucketMinutes := 5
		switch interval {
		case "1m":
			bucketMinutes = 1
		case "5m":
			bucketMinutes = 5
		case "15m":
			bucketMinutes = 15
		case "1h":
			bucketMinutes = 60
		}
		timeLen := 13 + bucketMinutes/10
		awkCmd := fmt.Sprintf("cat | awk '{t=$1\" \"$2; sub(/:[0-9]{2}$/, \":00\", t); print substr(t,1,%d)}' | sort | uniq -c | head -200", timeLen)
		if keyword != "" {
			q := strings.ReplaceAll(keyword, "'", "'\\''")
			awkCmd = fmt.Sprintf("cat | grep -i '%s' | awk '{t=$1\" \"$2; sub(/:[0-9]{2}$/, \":00\", t); print substr(t,1,%d)}' | sort | uniq -c | head -200", q, timeLen)
		}
		return awkCmd, nil

	case "top_values":
		f := field
		if f <= 0 {
			f = 1
		}
		awkExpr := fmt.Sprintf("awk '{print $%d}'", f)
		if keyword != "" {
			return fmt.Sprintf("cat | grep -i '%s' | %s | sort | uniq -c | sort -rn | head -%d",
				strings.ReplaceAll(keyword, "'", "'\\''"), awkExpr, topN), nil
		}
		return fmt.Sprintf("cat | %s | sort | uniq -c | sort -rn | head -%d", awkExpr, topN), nil

	case "rate":
		if keyword != "" {
			q := strings.ReplaceAll(keyword, "'", "'\\''")
			return fmt.Sprintf("cat | awk 'BEGIN{t=0;m=0} {t++; if(tolower($0) ~ tolower(\"%s\")) m++} END{printf \"total_lines: %%d\\nmatched_lines: %%d\\nmatch_rate: %%.2f%%%%\\n\", t, m, (t>0?m*100.0/t:0)}'", q), nil
		}
		return "cat | wc -l | awk '{printf \"total_lines: %s\\n\", $1}'", nil

	case "summary":
		return `cat | awk '
BEGIN { total=0; errors=0; warns=0; infos=0; first=""; last="" }
{
  total++
  if ($0 ~ /[Ee][Rr][Rr][Oo][Rr]/) errors++
  else if ($0 ~ /[Ww][Aa][Rr][Nn]/) warns++
  else if ($0 ~ /[Ii][Nn][Ff][Oo]/) infos++
  if (first == "") first = $0
  last = $0
}
END {
  printf "total_lines: %d\n", total
  printf "errors: %d (%.1f%%)\n", errors, (total>0?errors*100.0/total:0)
  printf "warns:  %d (%.1f%%)\n", warns, (total>0?warns*100.0/total:0)
  printf "infos:  %d (%.1f%%)\n", infos, (total>0?infos*100.0/total:0)
  printf "first_line: %s\n", first
  printf "last_line:  %s\n", last
}' | head -20`, nil

	default:
		return "", fmt.Errorf("未知分析类型: %q，可选: top_errors, error_timeline, top_values, rate, summary", analysis)
	}
}

// analyzeLogFile 对日志文件执行预设分析
func analyzeLogFile(filePath string, analysis, keyword string, field, topN int, interval string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 5000
	}
	if maxLines > 100000 {
		maxLines = 100000
	}
	if topN <= 0 {
		topN = 10
	}
	if topN > 100 {
		topN = 100
	}
	if field <= 0 {
		field = 1
	}
	if interval == "" {
		interval = "5m"
	}

	command, err := buildAnalyzeCommand(analysis, keyword, field, topN, interval)
	if err != nil {
		return marshalPipelineResult(false, err.Error(), nil)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return marshalPipelineResult(false, fmt.Sprintf("打开日志文件失败: %v", err), nil)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return marshalPipelineResult(false, fmt.Sprintf("获取文件信息失败: %v", err), nil)
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return marshalPipelineResult(true, "", map[string]interface{}{
			"stdout":      "",
			"exit_code":   0,
			"input_lines": 0,
			"analysis":    analysis,
		})
	}

	lines := agentbase.TailReadLines(f, fileSize, maxLines)
	content := strings.Join(lines, "\n")

	resultJSON := executeLogPipeline(content, command)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return resultJSON
	}
	result["log_path"] = filePath
	result["input_lines"] = len(lines)
	result["analysis"] = analysis
	result["command"] = command

	meta, _ := json.Marshal(result)
	return string(meta)
}

// marshalPipelineResult 构建管道结果 JSON
func marshalPipelineResult(success bool, errMsg string, data map[string]interface{}) string {
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
