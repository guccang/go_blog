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

	tools := append(buildLogToolDefs(readLogDesc), logToolKit.ToolDefs()...)

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
func buildLogToolDefs(readLogDesc string) []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "ListLogSources",
			Description: "列出所有可查询的日志源（名称、路径、描述）",
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
				},
				"required": []string{"source"},
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

// toolListLogSources 列出所有可查日志源
func (c *Connection) toolListLogSources() string {
	type sourceInfo struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Description string `json:"description"`
	}

	sources := make([]sourceInfo, 0, len(c.cfg.LogSources))
	for name, src := range c.cfg.LogSources {
		sources = append(sources, sourceInfo{
			Name:        name,
			Path:        src.Path,
			Description: src.Description,
		})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })

	data, _ := json.Marshal(map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	})
	return string(data)
}

// toolReadLog 查询指定日志源的日志
func (c *Connection) toolReadLog(args map[string]interface{}) string {
	source, _ := args["source"].(string)
	if source == "" {
		return agentbase.ErrorJSON("缺少 source 参数")
	}

	// 查找日志源
	logSource, ok := c.cfg.LogSources[source]
	if !ok {
		names := make([]string, 0, len(c.cfg.LogSources))
		for name := range c.cfg.LogSources {
			names = append(names, name)
		}
		sort.Strings(names)
		return agentbase.ErrorJSON(fmt.Sprintf("未知日志源 %q，可用源: %s", source, strings.Join(names, ", ")))
	}

	// 获取文件名
	file, _ := args["file"].(string)
	var filePath string

	if file != "" {
		// 指定了文件名 → 拼接路径并校验不逃逸
		filePath = filepath.Join(logSource.Path, file)
		// 路径逃逸检查：确保结果路径在 source.Path 下
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return agentbase.ErrorJSON(fmt.Sprintf("路径解析失败: %v", err))
		}
		absBase, err := filepath.Abs(logSource.Path)
		if err != nil {
			return agentbase.ErrorJSON(fmt.Sprintf("基础路径解析失败: %v", err))
		}
		if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
			return agentbase.ErrorJSON("路径不合法，禁止访问日志源目录之外的文件")
		}
	} else {
		// 未指定文件名 → 扫描目录找最新 .log 文件
		latest, err := findLatestLogFile(logSource.Path)
		if err != nil {
			return agentbase.ErrorJSON(fmt.Sprintf("扫描日志目录失败: %v", err))
		}
		filePath = latest
	}

	// 解析查询参数
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

	var startTime, endTime time.Time
	if startTimeStr != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(startTimeStr), time.Local)
		if err != nil {
			// 尝试仅时间格式
			t2, err2 := time.ParseInLocation("15:04:05", strings.TrimSpace(startTimeStr), time.Local)
			if err2 != nil {
				return agentbase.ErrorJSON(fmt.Sprintf("start_time 格式错误: %v", err))
			}
			now := time.Now()
			t = time.Date(now.Year(), now.Month(), now.Day(), t2.Hour(), t2.Minute(), t2.Second(), 0, time.Local)
		}
		startTime = t
	}
	if endTimeStr != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(endTimeStr), time.Local)
		if err != nil {
			t2, err2 := time.ParseInLocation("15:04:05", strings.TrimSpace(endTimeStr), time.Local)
			if err2 != nil {
				return agentbase.ErrorJSON(fmt.Sprintf("end_time 格式错误: %v", err))
			}
			now := time.Now()
			t = time.Date(now.Year(), now.Month(), now.Day(), t2.Hour(), t2.Minute(), t2.Second(), 0, time.Local)
		}
		endTime = t
	}

	log.Printf("[ReadLog] source=%s file=%s lines=%d keyword=%q", source, filePath, lines, keyword)

	return agentbase.ReadLogFile(filePath, lines, keyword, startTime, endTime)
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
