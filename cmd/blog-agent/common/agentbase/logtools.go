package agentbase

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"uap"
)

// ============================================================================
// LogToolKit — agent 自身日志查询
// ============================================================================

// LogToolKit 日志查询工具包
// 通过 UAP 工具机制暴露给 llm-agent 远程查询 agent 运行日志
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
	opts := ParseReadLogArgs(args)

	var err error
	if v, ok := args["start_time"].(string); ok && v != "" {
		opts.StartTime, err = ParseLogTime(v)
		if err != nil {
			return marshalResult(false, fmt.Sprintf("start_time 格式错误: %v", err), nil)
		}
	}
	if v, ok := args["end_time"].(string); ok && v != "" {
		opts.EndTime, err = ParseLogTime(v)
		if err != nil {
			return marshalResult(false, fmt.Sprintf("end_time 格式错误: %v", err), nil)
		}
	}

	return ReadLogFile(lt.logPath, opts)
}

// ============================================================================
// ReadLogOptions — ReadLogFile 参数
// ============================================================================

// ReadLogOptions 日志查询选项
type ReadLogOptions struct {
	Lines           int       // 返回行数，默认 200，上限 2000
	Keyword         string    // 关键词过滤
	Regex           string    // 正则过滤
	CaseInsensitive bool      // 不区分大小写
	StartTime       time.Time // 起始时间
	EndTime         time.Time // 结束时间
}

// ParseReadLogArgs 从 tool args 中解析 ReadLogOptions
func ParseReadLogArgs(args map[string]interface{}) ReadLogOptions {
	opts := ReadLogOptions{Lines: 200}

	if v, ok := args["lines"].(float64); ok && v > 0 {
		opts.Lines = int(v)
	}
	if opts.Lines > 2000 {
		opts.Lines = 2000
	}

	opts.Keyword, _ = args["keyword"].(string)
	opts.Regex, _ = args["regex"].(string)

	if v, ok := args["case_sensitive"].(bool); ok {
		opts.CaseInsensitive = !v
	}

	return opts
}

// ============================================================================
// ReadLogFile — 核心日志查询
// ============================================================================

// ReadLogFile 通用日志查询（公共函数）
// 从指定文件读取最后 lines 行，支持关键词/正则/时间范围过滤
func ReadLogFile(filePath string, opts ReadLogOptions) string {
	if opts.Lines <= 0 {
		opts.Lines = 200
	}
	if opts.Lines > 2000 {
		opts.Lines = 2000
	}

	hasTimeFilter := !opts.StartTime.IsZero() || !opts.EndTime.IsZero()

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

	// 编译正则
	var re *regexp.Regexp
	if opts.Regex != "" {
		pattern := opts.Regex
		if opts.CaseInsensitive {
			pattern = "(?i)" + pattern
		}
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return marshalResult(false, fmt.Sprintf("正则表达式无效: %v", err), nil)
		}
	}

	// 从文件末尾反向读取候选行
	maxCandidates := opts.Lines * 10
	if maxCandidates > 50000 {
		maxCandidates = 50000
	}
	candidateLines := tailReadLines(f, fileSize, maxCandidates)

	// 过滤
	var matched []string
	for _, line := range candidateLines {
		// 关键词过滤
		if opts.Keyword != "" {
			if opts.CaseInsensitive {
				if !strings.Contains(strings.ToLower(line), strings.ToLower(opts.Keyword)) {
					continue
				}
			} else {
				if !strings.Contains(line, opts.Keyword) {
					continue
				}
			}
		}

		// 正则过滤
		if re != nil && !re.MatchString(line) {
			continue
		}

		// 时间范围过滤
		if hasTimeFilter {
			lineTime, ok := extractLineTime(line)
			if ok {
				if !opts.StartTime.IsZero() && lineTime.Before(opts.StartTime) {
					continue
				}
				if !opts.EndTime.IsZero() && lineTime.After(opts.EndTime) {
					continue
				}
			}
		}
		matched = append(matched, line)
	}

	if len(matched) > opts.Lines {
		matched = matched[len(matched)-opts.Lines:]
	}

	content := strings.Join(matched, "\n")
	truncated := false

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

// ============================================================================
// tailReadLines — 高效分块反向读取
// ============================================================================

// tailReadLines 从文件末尾反向分块读取，返回最后 maxLines 行
func tailReadLines(f *os.File, fileSize int64, maxLines int) []string {
	const chunkSize = 32 * 1024

	var buf []byte
	offset := fileSize
	totalNewlines := 0

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

		for _, b := range chunk {
			if b == '\n' {
				totalNewlines++
			}
		}

		buf = append(chunk, buf...)

		if totalNewlines >= maxLines+1 {
			break
		}
	}

	text := string(buf)
	allLines := strings.Split(text, "\n")

	for len(allLines) > 0 && allLines[0] == "" {
		allLines = allLines[1:]
	}
	for len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	if len(allLines) > maxLines {
		allLines = allLines[len(allLines)-maxLines:]
	}

	return allLines
}

// ============================================================================
// 时间解析
// ============================================================================

// 常见日志时间格式，按长度降序排列以优先匹配
var logTimeFormats = []string{
	time.RFC3339Nano,                 // "2006-01-02T15:04:05.999999999Z07:00"
	"2006-01-02T15:04:05.000-07:00", // ISO 8601 with ms and tz
	"2006-01-02T15:04:05.000Z",      // ISO 8601 with ms and Z
	"2006-01-02T15:04:05-07:00",     // ISO 8601 with tz
	"2006-01-02T15:04:05Z",          // ISO 8601 with Z
	time.RFC3339,                      // "2006-01-02T15:04:05Z07:00"
	"2006-01-02T15:04:05.000",       // ISO 8601 with ms, no tz
	"2006-01-02T15:04:05",           // ISO 8601, no tz
	"2006/01/02 15:04:05",           // Go standard log
	"2006-01-02 15:04:05",           // ISO date + space-separated time
	"2006/01/02 15:04:05.000",       // Go log with ms
	"Jan  2 15:04:05",               // syslog (space-padded day)
	"Jan 2 15:04:05",                // syslog (single-space day)
	"2006-01-02",                    // date only
}

// extractLineTime 从行首解析日志时间戳，支持多种常见格式
func extractLineTime(line string) (time.Time, bool) {
	for _, format := range logTimeFormats {
		if len(line) < len(format) {
			continue
		}
		candidate := line[:len(format)]
		t, err := time.ParseInLocation(format, candidate, time.Local)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ParseLogTime 解析用户输入的时间参数
// 支持完整格式 "2006-01-02 15:04:05" 和仅时间 "15:04:05"（补今天日期）
func ParseLogTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}

	if t, err := time.ParseInLocation("15:04:05", s, time.Local); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}

	return time.Time{}, fmt.Errorf("不支持的时间格式 %q，请使用 \"2006-01-02 15:04:05\" 或 \"15:04:05\"", s)
}

// TailReadLines 从文件末尾反向分块读取，返回最后 maxLines 行（导出供 log-agent 等使用）
func TailReadLines(f *os.File, fileSize int64, maxLines int) []string {
	return tailReadLines(f, fileSize, maxLines)
}

