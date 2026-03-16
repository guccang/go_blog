package agentbase

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"uap"
)

// LogToolKit 日志查询工具包
// 通过 UAP 工具机制暴露给 llm-mcp-agent 远程查询 agent 运行日志
type LogToolKit struct {
	prefix  string // 工具名前缀，如 "Codegen"
	logPath string // 日志文件路径，如 "codegen-agent.log"
}

// NewLogToolKit 创建 LogToolKit 实例
func NewLogToolKit(prefix, logPath string) *LogToolKit {
	return &LogToolKit{
		prefix:  prefix,
		logPath: logPath,
	}
}

// ToolDefs 返回 1 个 UAP 工具定义：{Prefix}ReadLog
func (lt *LogToolKit) ToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        lt.prefix + "ReadLog",
			Description: fmt.Sprintf("查询 %s agent 的运行日志，支持按行数、关键词、时间范围过滤", lt.prefix),
			Parameters: MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"lines": map[string]interface{}{
						"type":        "integer",
						"description": "返回最近 N 行（过滤后），默认 200，上限 2000",
					},
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "关键词过滤，只返回包含该字符串的行",
					},
					"start_time": map[string]interface{}{
						"type":        "string",
						"description": "起始时间，格式 \"2006-01-02 15:04:05\" 或 \"15:04:05\"（今天）",
					},
					"end_time": map[string]interface{}{
						"type":        "string",
						"description": "结束时间，格式同上",
					},
				},
			}),
		},
	}
}

// HandleTool 处理工具调用，返回 (result_json, handled)
func (lt *LogToolKit) HandleTool(toolName string, args map[string]interface{}) (string, bool) {
	if toolName != lt.prefix+"ReadLog" {
		return "", false
	}
	return lt.toolReadLog(args), true
}

// toolReadLog 查询日志文件（委托给公共函数 ReadLogFile）
func (lt *LogToolKit) toolReadLog(args map[string]interface{}) string {
	// 解析参数
	lines := 200
	if v, ok := args["lines"].(float64); ok && v > 0 {
		lines = int(v)
	}
	if lines > 2000 {
		lines = 2000
	}

	keyword, _ := args["keyword"].(string)
	startTimeStr, _ := args["start_time"].(string)
	endTimeStr, _ := args["end_time"].(string)

	// 解析时间范围
	var startTime, endTime time.Time
	if startTimeStr != "" {
		t, err := parseLogTime(startTimeStr)
		if err != nil {
			return marshalResult(false, fmt.Sprintf("start_time 格式错误: %v", err), nil)
		}
		startTime = t
	}
	if endTimeStr != "" {
		t, err := parseLogTime(endTimeStr)
		if err != nil {
			return marshalResult(false, fmt.Sprintf("end_time 格式错误: %v", err), nil)
		}
		endTime = t
	}

	return ReadLogFile(lt.logPath, lines, keyword, startTime, endTime)
}

// ReadLogFile 通用日志查询（公共函数）
// 从指定文件读取最后 lines 行，支持关键词和时间范围过滤
// 返回 JSON 格式结果字符串
func ReadLogFile(filePath string, lines int, keyword string, startTime, endTime time.Time) string {
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}

	hasTimeFilter := !startTime.IsZero() || !endTime.IsZero()

	// 打开日志文件
	f, err := os.Open(filePath)
	if err != nil {
		return marshalResult(false, fmt.Sprintf("打开日志文件失败: %v", err), nil)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return marshalResult(false, fmt.Sprintf("获取文件信息失败: %v", err), nil)
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return marshalResult(true, "", map[string]interface{}{
			"content":       "",
			"matched_lines": 0,
			"truncated":     false,
			"log_path":      filePath,
		})
	}

	// 从文件末尾反向读取候选行（最多读 lines*10 行或 50000 行，确保过滤后有足够结果）
	maxCandidates := lines * 10
	if maxCandidates > 50000 {
		maxCandidates = 50000
	}
	candidateLines := tailReadLines(f, fileSize, maxCandidates)

	// 过滤
	var matched []string
	for _, line := range candidateLines {
		// 关键词过滤
		if keyword != "" && !strings.Contains(line, keyword) {
			continue
		}
		// 时间范围过滤
		if hasTimeFilter {
			lineTime, ok := extractLineTime(line)
			if ok {
				if !startTime.IsZero() && lineTime.Before(startTime) {
					continue
				}
				if !endTime.IsZero() && lineTime.After(endTime) {
					continue
				}
			}
			// 无法解析时间的行：如果设置了时间过滤则跳过
		}
		matched = append(matched, line)
	}

	// 只取最后 lines 行
	if len(matched) > lines {
		matched = matched[len(matched)-lines:]
	}

	content := strings.Join(matched, "\n")
	truncated := false

	// 响应截断到 100KB
	const maxResponseBytes = 100 * 1024
	if len(content) > maxResponseBytes {
		content = content[len(content)-maxResponseBytes:]
		truncated = true
	}

	return marshalResult(true, "", map[string]interface{}{
		"content":       content,
		"matched_lines": len(matched),
		"truncated":     truncated,
		"log_path":      filePath,
	})
}

// tailReadLines 从文件末尾反向分块读取，返回最后 maxLines 行
// 使用 32KB 分块读取，不会将整个文件加载到内存
func tailReadLines(f *os.File, fileSize int64, maxLines int) []string {
	const chunkSize = 32 * 1024 // 32KB

	var buf []byte
	offset := fileSize

	for offset > 0 {
		readSize := int64(chunkSize)
		if readSize > offset {
			readSize = offset
		}
		offset -= readSize

		chunk := make([]byte, readSize)
		n, err := f.ReadAt(chunk, offset)
		if err != nil && err != io.EOF {
			break
		}
		chunk = chunk[:n]

		// 拼接到 buf 前面
		buf = append(chunk, buf...)

		// 检查是否已有足够行数
		lineCount := 0
		for _, b := range buf {
			if b == '\n' {
				lineCount++
			}
		}
		if lineCount >= maxLines+1 {
			break
		}
	}

	// 按行分割
	text := string(buf)
	allLines := strings.Split(text, "\n")

	// 去掉首尾空行
	for len(allLines) > 0 && allLines[0] == "" {
		allLines = allLines[1:]
	}
	for len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	// 只取最后 maxLines 行
	if len(allLines) > maxLines {
		allLines = allLines[len(allLines)-maxLines:]
	}

	return allLines
}

// extractLineTime 从行首解析 Go 标准日志时间格式 "2006/01/02 15:04:05"
func extractLineTime(line string) (time.Time, bool) {
	// Go 标准 log 包格式: "2006/01/02 15:04:05 ..."
	if len(line) < 19 {
		return time.Time{}, false
	}
	timeStr := line[:19]
	t, err := time.ParseInLocation("2006/01/02 15:04:05", timeStr, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// parseLogTime 解析用户输入的时间参数
// 支持完整格式 "2006-01-02 15:04:05" 和仅时间 "15:04:05"（补今天日期）
func parseLogTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	// 完整格式
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}

	// 仅时间，补今天日期
	if t, err := time.ParseInLocation("15:04:05", s, time.Local); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}

	return time.Time{}, fmt.Errorf("不支持的时间格式 %q，请使用 \"2006-01-02 15:04:05\" 或 \"15:04:05\"", s)
}

// mustMarshalJSONLog 内部序列化（避免与 filetools.go 的同名函数冲突）
func mustMarshalJSONLog(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
