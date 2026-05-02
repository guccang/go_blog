package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"agentbase"
	"uap"
)

// Connection UAP 客户端连接管理
type Connection struct {
	*agentbase.AgentBase

	cfg         *Config
	logToolKit  *agentbase.LogToolKit
	activeCount int32 // 活跃任务原子计数
}

// NewConnection 创建连接管理器
func NewConnection(cfg *Config, agentID string) *Connection {
	logToolKit := agentbase.NewLogToolKit("Log", "log-agent.log")

	// 动态生成 ReadLog 描述，嵌入所有源名
	sourceNames := make([]string, 0, len(cfg.LogSources))
	for name := range cfg.LogSources {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	sourcesStr := strings.Join(sourceNames, ", ")

	readLogDesc := fmt.Sprintf("查询指定日志源的日志文件。可用源: %s。用 ListLogSources 查看详情", sourcesStr)

	tools := append(buildLogToolDefs(readLogDesc, sourcesStr), logToolKit.ToolDefs()...)

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "log_query",
		AgentName:   cfg.AgentName,
		Description: "通用日志查询代理，通过命名日志源查询服务器上的日志文件",
		AuthToken:   cfg.AuthToken,
		Capacity:    10,
		Tools:       tools,
		Meta: map[string]any{
			"log_sources": buildLogSourceSummary(cfg.LogSources),
		},
	}

	c := &Connection{
		AgentBase:  agentbase.NewAgentBase(baseCfg),
		cfg:        cfg,
		logToolKit: logToolKit,
	}

	c.RegisterToolCallHandler(c.handleToolCallMsg)
	c.RegisterHandler(uap.MsgError, c.handleError)

	return c
}

// buildLogToolDefs 构建 log-agent 的工具定义
func buildLogToolDefs(readLogDesc, sourcesStr string) []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "ListLogSources",
			Description: "列出所有可查询日志源的名称、路径和描述。仅用于发现日志源与确认参数，不读取日志内容。",
			Parameters:  agentbase.MustMarshalJSON(map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}),
		},
		{
			Name:        "ReadLog",
			Description: readLogDesc,
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "日志源名称，如 gameserver、blog-agent、nginx",
					},
					"file": map[string]interface{}{
						"type":        "string",
						"description": "日志文件名，如 error.log。不填则读取目录下最新的 .log 文件",
					},
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
					"regex": map[string]interface{}{
						"type":        "string",
						"description": "正则表达式过滤，如 \"ERROR|FATAL\"，与 keyword 为 AND 关系",
					},
					"case_sensitive": map[string]interface{}{
						"type":        "boolean",
						"description": "关键词/正则是否区分大小写，默认 true。设为 false 则不区分",
					},
				},
				"required": []string{"source"},
			}),
		},
		{
			Name:        "AnalyzeLog",
			Description: fmt.Sprintf("对日志进行统计分析。可用源: %s。analysis 可选: top_errors(Top-N错误统计), error_timeline(时序分布), top_values(字段Top-N统计,配合 field), rate(频率/匹配率), summary(日志概览)。无需手写 grep/awk，按预设模板自动汇总", sourcesStr),
			Parameters: agentbase.MustMarshalJSON(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "日志源名称",
					},
					"analysis": map[string]interface{}{
						"type":        "string",
						"description": "分析类型: top_errors, error_timeline, top_values, rate, summary",
					},
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "预过滤关键词，如 ERROR、WARN、指定 IP",
					},
					"field": map[string]interface{}{
						"type":        "integer",
						"description": "字段索引(1-based)，用于 top_values 提取指定列，如第1列填1",
					},
					"top_n": map[string]interface{}{
						"type":        "integer",
						"description": "返回 Top-N 条结果，默认 10，上限 100",
					},
					"interval": map[string]interface{}{
						"type":        "string",
						"description": "时间聚合粒度，用于 error_timeline: \"1m\"/\"5m\"/\"15m\"/\"1h\"，默认 \"5m\"",
					},
					"start_time": map[string]interface{}{
						"type":        "string",
						"description": "起始时间，格式 \"2006-01-02 15:04:05\" 或 \"15:04:05\"",
					},
					"end_time": map[string]interface{}{
						"type":        "string",
						"description": "结束时间，格式同上",
					},
					"file": map[string]interface{}{
						"type":        "string",
						"description": "日志文件名，不填则使用最新 .log 文件",
					},
					"lines": map[string]interface{}{
						"type":        "integer",
						"description": "扫描行数，默认 5000，上限 100000",
					},
				},
				"required": []string{"source", "analysis"},
			}),
		},
	}
}

// handleToolCallMsg 处理工具调用请求
func (c *Connection) handleToolCallMsg(msg *uap.Message) {
	atomic.AddInt32(&c.activeCount, 1)
	defer atomic.AddInt32(&c.activeCount, -1)

	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Connection] parse tool_call payload failed: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid payload"))
		return
	}

	log.Printf("[Connection] tool_call from=%s tool=%s", msg.From, payload.ToolName)

	// 解析参数
	var args map[string]interface{}
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[Connection] parse arguments failed: %v", err)
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, "invalid arguments"))
			return
		}
	} else {
		args = make(map[string]interface{})
	}

	// 先尝试 LogToolKit 处理（自身日志）
	if result, handled := c.logToolKit.HandleTool(payload.ToolName, args); handled {
		log.Printf("[Connection] tool %s handled by logToolKit", payload.ToolName)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolResult(msg.ID, result, ""))
		return
	}

	// 处理 log-agent 自有工具
	var result string
	switch payload.ToolName {
	case "ListLogSources":
		result = c.toolListLogSources()
	case "ReadLog":
		result = c.toolReadLog(args)
	case "AnalyzeLog":
		result = c.toolAnalyzeLog(args)
	default:
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolError(msg.ID, fmt.Sprintf("unknown tool: %s", payload.ToolName)))
		return
	}

	c.Client.SendTo(msg.From, uap.MsgToolResult, uap.BuildToolResult(msg.ID, result, ""))
}

// handleError 处理错误消息
func (c *Connection) handleError(msg *uap.Message) {
	var payload uap.ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[Connection] parse error payload failed: %v", err)
		return
	}
	log.Printf("[Connection] error from=%s code=%s msg=%s", msg.From, payload.Code, payload.Message)
}

// ========================= 工具实现 =========================

// toolListLogSources 列出所有可查日志源（含文件统计）
func (c *Connection) toolListLogSources() string {
	type sourceInfo struct {
		Name         string `json:"name"`
		Path         string `json:"path"`
		Description  string `json:"description"`
		FileCount    int    `json:"file_count"`
		NewestFile   string `json:"newest_file,omitempty"`
		LastModified string `json:"last_modified,omitempty"`
		TotalSizeMB  int64  `json:"total_size_mb,omitempty"`
	}

	sources := make([]sourceInfo, 0, len(c.cfg.LogSources))
	for name, src := range c.cfg.LogSources {
		info := sourceInfo{
			Name:        name,
			Path:        src.Path,
			Description: src.Description,
		}

		entries, err := os.ReadDir(src.Path)
		if err == nil {
			var latestTime time.Time
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
					continue
				}
				info.FileCount++
				fi, fiErr := e.Info()
				if fiErr != nil {
					continue
				}
				info.TotalSizeMB += fi.Size()
				if fi.ModTime().After(latestTime) {
					latestTime = fi.ModTime()
					info.NewestFile = e.Name()
				}
			}
			if info.TotalSizeMB > 0 {
				info.TotalSizeMB = info.TotalSizeMB / (1024 * 1024)
			}
			if !latestTime.IsZero() {
				info.LastModified = latestTime.Format("2006-01-02 15:04:05")
			}
		}

		sources = append(sources, info)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })

	data, _ := json.Marshal(map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	})
	return string(data)
}

// resolveLogFile 根据 source+file 参数解析日志文件路径
func (c *Connection) resolveLogFile(args map[string]interface{}) (string, string) {
	source, _ := args["source"].(string)
	if source == "" {
		return "", agentbase.ErrorJSON("缺少 source 参数")
	}

	logSource, ok := c.cfg.LogSources[source]
	if !ok {
		names := make([]string, 0, len(c.cfg.LogSources))
		for name := range c.cfg.LogSources {
			names = append(names, name)
		}
		sort.Strings(names)
		return "", agentbase.ErrorJSON(fmt.Sprintf("未知日志源 %q，可用源: %s", source, strings.Join(names, ", ")))
	}

	file, _ := args["file"].(string)
	if file != "" {
		filePath := filepath.Join(logSource.Path, file)
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return "", agentbase.ErrorJSON(fmt.Sprintf("路径解析失败: %v", err))
		}
		absBase, err := filepath.Abs(logSource.Path)
		if err != nil {
			return "", agentbase.ErrorJSON(fmt.Sprintf("基础路径解析失败: %v", err))
		}
		if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
			return "", agentbase.ErrorJSON("路径不合法，禁止访问日志源目录之外的文件")
		}
		return filePath, ""
	}

	latest, err := findLatestLogFile(logSource.Path)
	if err != nil {
		return "", agentbase.ErrorJSON(fmt.Sprintf("扫描日志目录失败: %v", err))
	}
	return latest, ""
}

// toolReadLog 查询指定日志源的日志
func (c *Connection) toolReadLog(args map[string]interface{}) string {
	filePath, errMsg := c.resolveLogFile(args)
	if errMsg != "" {
		return errMsg
	}

	opts := agentbase.ParseReadLogArgs(args)

	if v, ok := args["start_time"].(string); ok && v != "" {
		t, err := agentbase.ParseLogTime(v)
		if err != nil {
			return agentbase.ErrorJSON(fmt.Sprintf("start_time 格式错误: %v", err))
		}
		opts.StartTime = t
	}
	if v, ok := args["end_time"].(string); ok && v != "" {
		t, err := agentbase.ParseLogTime(v)
		if err != nil {
			return agentbase.ErrorJSON(fmt.Sprintf("end_time 格式错误: %v", err))
		}
		opts.EndTime = t
	}

	log.Printf("[ReadLog] source=%s file=%s lines=%d keyword=%q regex=%q",
		args["source"], filePath, opts.Lines, opts.Keyword, opts.Regex)

	return agentbase.ReadLogFile(filePath, opts)
}

// toolAnalyzeLog 对日志执行预设分析
func (c *Connection) toolAnalyzeLog(args map[string]interface{}) string {
	filePath, errMsg := c.resolveLogFile(args)
	if errMsg != "" {
		return errMsg
	}

	analysis, _ := args["analysis"].(string)
	if analysis == "" {
		return agentbase.ErrorJSON("缺少 analysis 参数，可选: top_errors, error_timeline, top_values, rate, summary")
	}

	keyword, _ := args["keyword"].(string)
	topN := agentbase.GetOptionalIntParam(args, "top_n", 10)
	field := agentbase.GetOptionalIntParam(args, "field", 1)
	interval, _ := args["interval"].(string)
	lines := agentbase.GetOptionalIntParam(args, "lines", 5000)

	log.Printf("[AnalyzeLog] source=%s file=%s analysis=%s keyword=%q topN=%d lines=%d",
		args["source"], filePath, analysis, keyword, topN, lines)

	return analyzeLogFile(filePath, analysis, keyword, field, topN, interval, lines)
}

// findLatestLogFile 在目录中找到最新修改的 .log 文件
func findLatestLogFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("读取目录失败: %w", err)
	}

	var latestPath string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestPath = filepath.Join(dir, entry.Name())
		}
	}

	if latestPath == "" {
		return "", fmt.Errorf("目录 %s 下没有 .log 文件", dir)
	}
	return latestPath, nil
}

// buildLogSourceSummary 将日志源配置转为 源名→描述 映射，暴露给 LLM
func buildLogSourceSummary(sources map[string]LogSource) map[string]string {
	summary := make(map[string]string, len(sources))
	for name, src := range sources {
		summary[name] = src.Description
	}
	return summary
}
