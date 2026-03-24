package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// ========================= EventSink 接口与实现 =========================

// EventSink 抽象不同来源的输出差异
type EventSink interface {
	OnChunk(text string)        // LLM 文本片段
	OnEvent(event, text string) // 结构化事件 (tool_info, plan_start, plan_done, subtask_*, etc.)
	Streaming() bool            // 是否使用流式 LLM 调用
}

// StreamingSink Web 前端流式输出
type StreamingSink struct {
	bridge *Bridge
	taskID string
}

func (s *StreamingSink) OnChunk(text string)        { s.bridge.sendTaskEvent(s.taskID, "chunk", text) }
func (s *StreamingSink) OnEvent(event, text string) { s.bridge.sendTaskEvent(s.taskID, event, text) }
func (s *StreamingSink) Streaming() bool            { return true }

// BufferSink 缓冲输出（llm_request）
type BufferSink struct {
	buf strings.Builder
}

func (s *BufferSink) OnChunk(text string)        { s.buf.WriteString(text) }
func (s *BufferSink) OnEvent(event, text string) {}
func (s *BufferSink) Streaming() bool            { return false }
func (s *BufferSink) Result() string             { return s.buf.String() }

// LLMRequestSink 缓冲文本 + 转发事件（用于 llm_request 任务，支持 Path 2 进度推送）
type LLMRequestSink struct {
	buf    strings.Builder
	bridge *Bridge
	taskID string
}

func (s *LLMRequestSink) OnChunk(text string)        { s.buf.WriteString(text) }
func (s *LLMRequestSink) OnEvent(event, text string) { s.bridge.sendTaskEvent(s.taskID, event, text) }
func (s *LLMRequestSink) Streaming() bool            { return false }
func (s *LLMRequestSink) Result() string             { return s.buf.String() }

// ========================= RequestTrace 请求级追踪 =========================

// RequestTrace 请求级追踪：记录 LLM 轮次、工具调用路径，方便定位问题
type RequestTrace struct {
	TaskID    string
	Source    string
	Query     string
	StartTime time.Time
	Rounds    []TraceRound // 每轮 LLM 调用
}

// TraceRound 单轮 LLM 调用记录
type TraceRound struct {
	Index         int             // 第几轮（从1开始）
	LLMDurationMs int64          // LLM 响应耗时（毫秒）
	TextLen       int             // LLM 返回文本长度
	ToolCalls     []TraceToolCall // 本轮工具调用
}

// TraceToolCall 单次工具调用记录
type TraceToolCall struct {
	ToolName   string
	Arguments  string // 截断后的参数摘要
	Success    bool
	DurationMs int64
	ResultLen  int
}

// Summary 输出结构化追踪摘要
func (t *RequestTrace) Summary() string {
	if t == nil || len(t.Rounds) == 0 {
		return ""
	}
	totalDuration := time.Since(t.StartTime)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Trace] taskID=%s source=%s 共%d轮 总耗时%s query=%s\n",
		t.TaskID, t.Source, len(t.Rounds), fmtDuration(totalDuration), truncate(t.Query, 80)))

	for _, r := range t.Rounds {
		llmDur := fmtDuration(time.Duration(r.LLMDurationMs) * time.Millisecond)
		if len(r.ToolCalls) == 0 {
			sb.WriteString(fmt.Sprintf("  Round[%d] LLM=%s textLen=%d → 无工具调用（最终回复）\n", r.Index, llmDur, r.TextLen))
		} else {
			var tcParts []string
			for _, tc := range r.ToolCalls {
				status := "✅"
				if !tc.Success {
					status = "❌"
				}
				tcDur := fmtDuration(time.Duration(tc.DurationMs) * time.Millisecond)
				part := fmt.Sprintf("%s(%s %s", tc.ToolName, status, tcDur)
				if tc.Arguments != "" {
					part += " " + tc.Arguments
				}
				part += ")"
				tcParts = append(tcParts, part)
			}
			sb.WriteString(fmt.Sprintf("  Round[%d] LLM=%s → %d个工具: %s\n",
				r.Index, llmDur, len(r.ToolCalls), strings.Join(tcParts, ", ")))
		}
	}
	return sb.String()
}

// ========================= TaskContext =========================

// TaskContext 统一任务输入
type TaskContext struct {
	Ctx           context.Context // 可取消的 context（nil 表示不可取消）
	TaskID        string
	Account       string
	Query         string    // 用户问题（用于 plan_and_execute）
	Source        string    // "web" | "wechat" | "llm_request"
	Messages      []Message // 预构建消息（nil 则自动构建）
	SelectedTools []string
	NoTools       bool
	Sink          EventSink
	Trace         *RequestTrace // 请求追踪（可选）
}

func isMCPToolListQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}

	hasMCP := strings.Contains(q, "mcp")
	hasTool := strings.Contains(q, "tool") ||
		strings.Contains(q, "tools") ||
		strings.Contains(q, "??")
	if !hasMCP || !hasTool {
		return false
	}

	intents := []string{
		"??", "??", "??", "??", "??", "??", "??", "???", "??", "??", "??", "??",
		"all", "list", "show", "catalog", "inventory", "enumerate",
	}
	for _, intent := range intents {
		if strings.Contains(q, intent) {
			return true
		}
	}
	return false
}

func (b *Bridge) buildMCPToolListReply(query string, tools []LLMTool) (string, bool) {
	if !isMCPToolListQuery(query) {
		return "", false
	}

	if len(tools) == 0 {
		return "??????? MCP ???", true
	}

	// 按 agent 分组
	b.catalogMu.RLock()
	agentInfoCopy := make(map[string]AgentInfo, len(b.agentInfo))
	for k, v := range b.agentInfo {
		agentInfoCopy[k] = v
	}
	toolCatalogCopy := make(map[string]string, len(b.toolCatalog))
	for k, v := range b.toolCatalog {
		toolCatalogCopy[k] = v
	}
	b.catalogMu.RUnlock()

	// 构建 tool → agent 映射（使用 sanitized name）
	type groupedTool struct {
		Name string
		Desc string
	}
	agentGroups := make(map[string][]groupedTool)
	ungrouped := make([]groupedTool, 0)

	sorted := make([]LLMTool, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Function.Name < sorted[j].Function.Name
	})

	for _, tool := range sorted {
		originalName := b.resolveToolName(tool.Function.Name)
		desc := strings.TrimSpace(tool.Function.Description)
		if desc == "" {
			desc = "???"
		}
		gt := groupedTool{Name: originalName, Desc: desc}

		agentID, ok := toolCatalogCopy[originalName]
		if !ok {
			agentID, ok = toolCatalogCopy[tool.Function.Name]
		}
		if ok {
			agentGroups[agentID] = append(agentGroups[agentID], gt)
		} else {
			ungrouped = append(ungrouped, gt)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("????? %d ? MCP ???\n\n", len(sorted)))

	idx := 1
	for agentID, groupTools := range agentGroups {
		info, hasInfo := agentInfoCopy[agentID]
		if hasInfo && info.Description != "" {
			sb.WriteString(fmt.Sprintf("?? %s (%s)\n", info.Name, info.Description))
		} else if hasInfo {
			sb.WriteString(fmt.Sprintf("?? %s\n", info.Name))
		} else {
			sb.WriteString(fmt.Sprintf("?? %s\n", agentID))
		}
		for _, gt := range groupTools {
			sb.WriteString(fmt.Sprintf("  %d. %s - %s\n", idx, gt.Name, gt.Desc))
			idx++
		}
		sb.WriteString("\n")
	}
	if len(ungrouped) > 0 {
		sb.WriteString("?? ???\n")
		for _, gt := range ungrouped {
			sb.WriteString(fmt.Sprintf("  %d. %s - %s\n", idx, gt.Name, gt.Desc))
			idx++
		}
	}
	return strings.TrimSpace(sb.String()), true
}

// executeSkillTool 虚拟工具定义（技能子任务执行）
var executeSkillTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "execute_skill",
		Description: "执行一个技能。这是处理用户任务的首选方式——技能封装了完整的工具集和执行策略，在独立子任务中运行并返回结果。匹配到可用技能时必须优先使用。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"skill_name": {
					"type": "string",
					"description": "技能名称（来自可用技能列表）"
				},
				"query": {
					"type": "string",
					"description": "具体任务描述"
				}
			},
			"required": ["skill_name", "query"]
		}`),
	},
}

// getAgentToolsTool 虚拟工具定义：获取指定 agent 的工具列表（渐进式发现）
var getAgentToolsTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "get_agent_tools",
		Description: "获取指定 agent 的完整工具列表和参数说明。调用后该 agent 的工具将在后续轮次可用。在需要使用某个 agent 的能力时，先调用此工具加载其工具。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"agent_id": {
					"type": "string",
					"description": "Agent ID（来自系统提示词中的可用 Agent 列表）"
				}
			},
			"required": ["agent_id"]
		}`),
	},
}

// getSkillDetailTool 虚拟工具定义：获取技能详细文档
var getSkillDetailTool = LLMTool{
	Type: "function",
	Function: LLMFunction{
		Name:        "get_skill_detail",
		Description: "获取指定技能的详细文档，包括工具列表、执行策略和历史经验。在决定是否使用某个技能前，可以先查看其详细说明。",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"skill_name": {
					"type": "string",
					"description": "技能名称（来自系统提示词中的 Skill 目录）"
				}
			},
			"required": ["skill_name"]
		}`),
	},
}

// ========================= 统一处理函数 =========================

// processTask 统一消息处理核心：构建消息 → 创建会话 → 获取工具 → LLM 循环 → 保存会话
func (b *Bridge) processTask(ctx *TaskContext) (string, error) {
	taskStart := time.Now()
	streaming := ctx.Sink.Streaming()
	log.Printf("[processTask] ▶ 开始处理 taskID=%s source=%s account=%s streaming=%v query=%s",
		ctx.TaskID, ctx.Source, ctx.Account, streaming, truncate(ctx.Query, 100))

	// 1. 获取工具 + 渐进式发现初始化
	var messages []Message
	var tools []LLMTool

	// 渐进式发现状态：追踪已加载的 agent
	loadedAgents := make(map[string]bool)
	allAgentToolsSnapshot := b.getAgentToolsMap()

	if ctx.NoTools {
		tools = nil
		log.Printf("[processTask] 工具模式: 禁用")
	} else if len(ctx.SelectedTools) > 0 {
		tools = b.filterToolsBySelection(ctx.SelectedTools)
		log.Printf("[processTask] 工具模式: 用户选择 selected=%d matched=%d", len(ctx.SelectedTools), len(tools))
	} else if ctx.Source == "cron_query" || ctx.Source == "llm_request" {
		// 后向兼容：自动加载全部工具（无需渐进式发现）
		tools = b.getLLMTools()
		log.Printf("[processTask] 工具模式: 全量加载（%s） count=%d", ctx.Source, len(tools))
	} else {
		// 渐进式发现：只加载基础工具，LLM 通过 get_agent_tools 按需加载
		tools = b.getBaseToolSet()
		log.Printf("[processTask] 工具模式: 基础工具（渐进式发现） count=%d", len(tools))
	}

	// 提取 query（构建消息前就需要）
	query := ctx.Query
	if query == "" && ctx.Messages != nil {
		for i := len(ctx.Messages) - 1; i >= 0; i-- {
			if ctx.Messages[i].Role == "user" {
				query = ctx.Messages[i].Content
				break
			}
		}
	}

	// 闲聊检测：短问候不需要工具
	if !ctx.NoTools && query != "" && isGreeting(query) {
		tools = nil
		log.Printf("[processTask] 闲聊检测命中，禁用工具 query=%s", truncate(query, 50))
	}

	// 2. MCP 工具列表查询拦截（使用全量工具展示）
	if !ctx.NoTools {
		allTools := b.getLLMTools()
		if directReply, ok := b.buildMCPToolListReply(query, allTools); ok {
			store := NewSessionStore(b.cfg.SessionDir)
			rootSession := NewRootSession(ctx.TaskID, query, ctx.Account)
			rootSession.AppendMessage(Message{Role: "assistant", Content: directReply})
			rootSession.SetResult(directReply)
			rootSession.SetStatus("done")
			store.Save(rootSession)
			store.SaveIndex(rootSession, nil)
			log.Printf("[processTask] direct MCP tool list reply, toolCount=%d", len(allTools))
			return directReply, nil
		}
	}

	// 3. 构建消息（固定系统提示词）
	if ctx.Messages != nil {
		// 有预构建消息（多轮续接或 llm_request）
		messages = make([]Message, len(ctx.Messages))
		copy(messages, ctx.Messages)

		// web/wechat 多轮续接时，刷新 system prompt
		if ctx.Source == "web" || ctx.Source == "wechat" {
			if len(messages) > 0 && messages[0].Role == "system" {
				freshPrompt, _ := b.buildAssistantSystemPrompt(ctx.Account)
				messages[0].Content = freshPrompt
				log.Printf("[processTask] 多轮续接：已刷新 system prompt promptLen=%d", len(freshPrompt))
			}
		} else {
			log.Printf("[processTask] 使用预构建消息 count=%d", len(messages))
		}
	} else {
		// 新对话：构建固定 system prompt + user query
		systemPrompt, _ := b.buildAssistantSystemPrompt(ctx.Account)
		messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: ctx.Query},
		}
		log.Printf("[processTask] 构建系统提示 promptLen=%d", len(systemPrompt))
	}

	// 5. 创建会话（所有来源都持久化）
	store := NewSessionStore(b.cfg.SessionDir)
	rootSession := NewRootSession(ctx.TaskID, query, ctx.Account)
	for _, msg := range messages {
		rootSession.AppendMessage(msg)
	}

	// 创建请求追踪
	trace := &RequestTrace{
		TaskID:    ctx.TaskID,
		Source:    ctx.Source,
		Query:     query,
		StartTime: taskStart,
	}
	ctx.Trace = trace

	// 触发任务开始 hook
	b.hooks.FireTaskStart(ctx)

	// 注入虚拟工具（集中管理）
	tools = b.injectVirtualTools(tools, ctx.NoTools)

	// 发送初始工具数量信息（分类展示）
	if !ctx.NoTools && len(tools) > 0 {
		var localNames  []string // 本地工具：本进程内执行
		var remoteNames []string // 远程工具：通过 UAP 发送到远程 agent 执行

		for _, t := range tools {
			name := t.Function.Name
			canonical := b.resolveToolName(name)
			// 远程工具：canonical 包含 "." 且不以本 agent 前缀开头
			if strings.Contains(canonical, ".") && !strings.HasPrefix(canonical, b.cfg.AgentID+".") {
				remoteNames = append(remoteNames, canonical)
			} else {
				localNames = append(localNames, name)
			}
		}

		var parts []string
		parts = append(parts, fmt.Sprintf("本地工具: %d", len(localNames)))
		if len(remoteNames) > 0 {
			parts = append(parts, fmt.Sprintf("远程工具: %d [%s]", len(remoteNames), strings.Join(remoteNames, ", ")))
		} else {
			parts = append(parts, "远程工具: 0")
		}

		ctx.Sink.OnEvent("tool_info", fmt.Sprintf("[🔧 初始加载 %d 个工具] %s\n本地: %s",
			len(tools), strings.Join(parts, " | "),
			strings.Join(localNames, ", ")))
	}

	// 4. LLM 循环
	maxIter := b.cfg.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	// 构建本次 processTask 专用的 localHandlers
	localHandlers := make(map[string]ToolHandler)

	// get_agent_tools handler：加载指定 agent 的工具到后续轮次
	if !ctx.NoTools {
		localHandlers["get_agent_tools"] = func(callCtx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
			var a struct {
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return &ToolCallResult{Result: fmt.Sprintf("参数解析失败: %v", err), AgentID: "builtin"}, nil
			}
			resolved := b.resolveAgentByName(a.AgentID)
			if resolved == "" {
				return &ToolCallResult{Result: fmt.Sprintf("agent '%s' 不存在。可用 agent:\n%s", a.AgentID, b.listAgentNames()), AgentID: "builtin"}, nil
			}
			loadedAgents[resolved] = true
			desc := b.getAgentToolDescriptions(resolved)
			log.Printf("[processTask] get_agent_tools: loaded agent=%s", resolved)
			return &ToolCallResult{Result: desc, AgentID: "builtin"}, nil
		}

		// get_skill_detail handler：返回技能详细文档
		localHandlers["get_skill_detail"] = func(callCtx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
			var a struct {
				SkillName string `json:"skill_name"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return &ToolCallResult{Result: fmt.Sprintf("参数解析失败: %v", err), AgentID: "builtin"}, nil
			}
			if b.skillMgr == nil {
				return &ToolCallResult{Result: "技能系统未启用", AgentID: "builtin"}, nil
			}
			skill := b.skillMgr.GetSkill(a.SkillName)
			if skill == nil {
				return &ToolCallResult{Result: fmt.Sprintf("技能 '%s' 不存在", a.SkillName), AgentID: "builtin"}, nil
			}
			detail := b.skillMgr.BuildSkillBlock([]SkillEntry{*skill})
			return &ToolCallResult{Result: detail, AgentID: "builtin"}, nil
		}
	}

	// execute_skill handler
	if !ctx.NoTools && b.skillMgr != nil && len(b.skillMgr.GetAllSkills()) > 0 {
		capturedCtx := ctx
		capturedTools := tools
		localHandlers["execute_skill"] = func(callCtx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
			var skillArgs struct {
				SkillName string `json:"skill_name"`
				Query     string `json:"query"`
			}
			if err := json.Unmarshal(args, &skillArgs); err != nil {
				return &ToolCallResult{Result: fmt.Sprintf("参数解析失败: %v", err), AgentID: "builtin"}, nil
			}
			sink.OnEvent("skill_start", fmt.Sprintf("正在执行技能: %s\n任务: %s", skillArgs.SkillName, skillArgs.Query))
			log.Printf("[processTask] execute_skill: skill=%s query=%s", skillArgs.SkillName, truncate(skillArgs.Query, 200))
			result := b.executeSkillSubTask(capturedCtx, skillArgs.SkillName, skillArgs.Query, capturedTools)
			return &ToolCallResult{Result: result, AgentID: "builtin"}, nil
		}
	}

	var finalText string
	var finalErr error
	complexTaskHandled := false

	for i := 0; i < maxIter; i++ {
		// 检查是否被用户取消
		if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
			log.Printf("[processTask] ✗ 任务被取消 taskID=%s", ctx.TaskID)
			finalText = "任务已停止。"
			rootSession.SetStatus("cancelled")
			ctx.Sink.OnEvent("task_cancelled", "任务已被用户停止")
			break
		}

		// 消息历史压缩：防止上下文溢出（字符预算 + 消息数双重检查）
		if len(messages) > 20 || estimateChars(messages) > processMaxTotalChars*80/100 {
			before := len(messages)
			messages = sanitizeProcessMessages(messages)
			if len(messages) != before {
				log.Printf("[processTask] 消息压缩: %d → %d", before, len(messages))
			}
		}

		log.Printf("[processTask] ── 迭代 %d/%d ── messages=%d tools=%d loadedAgents=%d", i+1, maxIter, len(messages), len(tools), len(loadedAgents))

		// 渐进式工具重建：如果有新 agent 被加载，重建工具列表
		if len(loadedAgents) > 0 && !ctx.NoTools {
			rebuiltTools := b.getBaseToolSet()
			for agentID := range loadedAgents {
				if agentTools, ok := allAgentToolsSnapshot[agentID]; ok {
					rebuiltTools = append(rebuiltTools, agentTools...)
				}
			}
			rebuiltTools = b.injectVirtualTools(rebuiltTools, false)
			if len(rebuiltTools) != len(tools) {
				log.Printf("[processTask] 工具重建: %d → %d (已加载 %d 个 agent)", len(tools), len(rebuiltTools), len(loadedAgents))
			}
			tools = rebuiltTools
		}

		// 接近迭代上限：强制 LLM 收敛
		if i == maxIter-1 {
			// 最后一轮：移除所有工具，强制 LLM 用文本总结
			tools = nil
			messages = append(messages, Message{
				Role:    "system",
				Content: "你已经进行了多轮工具调用。请立即给出最终回复，总结目前已完成的工作和结果。不要再调用任何工具。",
			})
			ctx.Sink.OnEvent("task_forced_summary", fmt.Sprintf("已执行 %d 轮工具调用，正在强制总结结果...", i))
			log.Printf("[processTask] ⚠ 达到迭代上限，移除工具强制总结")
		}

		// 发射 thinking 事件
		if i == 0 {
			ctx.Sink.OnEvent("thinking", "正在思考...")
		} else {
			ctx.Sink.OnEvent("thinking", fmt.Sprintf("正在分析工具结果（第%d轮）...", i+1))
		}

		var text string
		var toolCalls []ToolCall
		var err error

		// 获取来源渠道的 LLM 配置
		llmCfg, llmFallbacks := b.GetLLMConfigForSource(ctx.Source)

		llmStart := time.Now()
		if ctx.Sink.Streaming() {
			log.Printf("[processTask] → 发送流式 LLM 请求...")
			text, toolCalls, err = b.sendStreamingLLMWithConfig(llmCfg, llmFallbacks, messages, tools, func(chunk string) {
				ctx.Sink.OnChunk(chunk)
			})
		} else {
			log.Printf("[processTask] → 发送同步 LLM 请求...")
			text, toolCalls, err = b.sendLLMWithConfig(llmCfg, llmFallbacks, messages, tools)
		}
		llmDuration := time.Since(llmStart)

		if err != nil {
			log.Printf("[processTask] ✗ LLM 请求失败 duration=%v error=%v", llmDuration, err)
			finalErr = err
			ctx.Sink.OnChunk(fmt.Sprintf("\n\n抱歉，AI 服务暂时不可用: %v", err))
			break
		}

		// 构建工具调用名称列表用于日志
		var tcNames []string
		for _, tc := range toolCalls {
			tcNames = append(tcNames, b.resolveToolName(tc.Function.Name))
		}
		log.Printf("[processTask] ← LLM 响应 duration=%v textLen=%d toolCalls=%d tools=%v",
			llmDuration, len(text), len(toolCalls), tcNames)

		// 记录 Trace 轮次
		currentRound := TraceRound{
			Index:         i + 1,
			LLMDurationMs: llmDuration.Milliseconds(),
			TextLen:       len(text),
		}

		// LLM 响应反馈
		if len(toolCalls) > 0 {
			ctx.Sink.OnEvent("thinking", fmt.Sprintf("LLM 响应完成 (%s)，需要调用 %d 个工具: %s", fmtDuration(llmDuration), len(toolCalls), strings.Join(tcNames, ", ")))
		} else {
			ctx.Sink.OnEvent("thinking", fmt.Sprintf("LLM 响应完成 (%s)，正在整理结果...", fmtDuration(llmDuration)))
		}

		// 记录 assistant 消息到 session
		assistantMsg := Message{Role: "assistant", Content: text, ToolCalls: toolCalls}
		rootSession.AppendMessage(assistantMsg)

		// 无工具调用 → 对话结束
		if len(toolCalls) == 0 {
			log.Printf("[processTask] ✓ 对话结束（无工具调用） resultLen=%d", len(text))
			trace.Rounds = append(trace.Rounds, currentRound)
			finalText = text
			rootSession.SetResult(text)
			ctx.Sink.OnEvent("task_complete", fmt.Sprintf("处理完成，耗时 %s", fmtDuration(time.Since(taskStart))))
			break
		}

		// NoTools 但 LLM 仍返回工具调用，忽略并取文本
		if ctx.NoTools {
			log.Printf("[processTask] ✓ 忽略工具调用（NoTools模式） resultLen=%d", len(text))
			finalText = text
			rootSession.SetResult(text)
			break
		}

		// 检查 plan_and_execute
		planCallIdx := -1
		for idx, tc := range toolCalls {
			if tc.Function.Name == "plan_and_execute" {
				planCallIdx = idx
				break
			}
		}

		if planCallIdx >= 0 {
			var reasoning string
			var args struct {
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal([]byte(toolCalls[planCallIdx].Function.Arguments), &args); err == nil {
				reasoning = args.Reasoning
			}
			log.Printf("[processTask] plan_and_execute triggered at iteration %d: %s", i, reasoning)

			// 收集简单路径中已完成的工具调用历史（避免子任务重复执行）
			var completedWork string
			rootSession.mu.Lock()
			existingCalls := make([]ToolCallRecord, len(rootSession.ToolCalls))
			copy(existingCalls, rootSession.ToolCalls)
			rootSession.mu.Unlock()

			if len(existingCalls) > 0 {
				var workSummary strings.Builder
				for _, rec := range existingCalls {
					status := "✅ 成功"
					if !rec.Success {
						status = "❌ 失败"
					}
					workSummary.WriteString(fmt.Sprintf("- %s(%s) → %s: %s\n",
						rec.ToolName, truncate(rec.Arguments, 100), status, truncate(rec.Result, 200)))
				}
				completedWork = workSummary.String()
				log.Printf("[processTask] passing %d completed tool calls to planner", len(existingCalls))
			}

			// 进入复杂任务处理流程（内部处理会话保存）
			result := b.handleComplexTask(ctx, rootSession, store, tools, completedWork)
			ctx.Sink.OnChunk(result)
			finalText = result
			complexTaskHandled = true
			trace.Rounds = append(trace.Rounds, currentRound)
			break
		}

		// 普通工具调用：有工具调用时，思考过程不进入 LLM 上下文（完整记录已保存到 rootSession）
		msgText := text
		if len(toolCalls) > 0 {
			msgText = "" // 思考过程对后续 LLM 调用无价值，清除以节省上下文
		}
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   msgText,
			ToolCalls: toolCalls,
		})

		// 本轮业务失败记录（用于循环后扩展兄弟工具）
		var bizFailedTools []string
		var bizFailedMsgs []string

		for tcIdx, tc := range toolCalls {
			// 检查是否被用户取消
			if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
				log.Printf("[processTask] ✗ 工具调用期间任务被取消 taskID=%s", ctx.TaskID)
				finalText = "任务已停止。"
				rootSession.SetStatus("cancelled")
				ctx.Sink.OnEvent("task_cancelled", "任务已被用户停止")
				break
			}

			originalName := b.resolveToolName(tc.Function.Name)

			// 统一 handler 查找：localHandlers → 全局 toolHandlers
			handler, hasHandler := localHandlers[originalName]
			if !hasHandler {
				b.catalogMu.RLock()
				handler, hasHandler = b.toolHandlers[originalName]
				b.catalogMu.RUnlock()
			}

			// ExecuteCode 特殊展示：提取 description + code
			toolCallEvent := fmt.Sprintf("调用 %s (%d/%d)\n参数: %s", originalName, tcIdx+1, len(toolCalls), tc.Function.Arguments)
			if originalName == "ExecuteCode" {
				toolCallEvent = formatExecuteCodeEvent(tc.Function.Arguments, tcIdx+1, len(toolCalls))
			}
			ctx.Sink.OnEvent("tool_call", toolCallEvent)
			log.Printf("[processTask] → 调用工具: %s args=%s", originalName, truncate(tc.Function.Arguments, 500))

			start := time.Now()
			callCtx := ctx.Ctx
			if callCtx == nil {
				callCtx = context.Background()
			}

			// 统一调度：通过 handler 执行（包括 Bash 和远程工具）
			if !hasHandler {
				log.Printf("[processTask] ✗ 工具未找到: %s", originalName)
				toolMsg := Message{Role: "tool", Content: fmt.Sprintf("工具 %s 未找到", originalName), ToolCallID: tc.ID}
				rootSession.AppendMessage(toolMsg)
				messages = append(messages, toolMsg)
				continue
			}

			// 异步执行工具调用，主协程发送心跳进度
			type toolCallResultPair struct {
				result *ToolCallResult
				err    error
			}
			resultCh := make(chan toolCallResultPair, 1)
			capturedHandler := handler
			go func() {
				r, e := capturedHandler(callCtx, json.RawMessage(tc.Function.Arguments), ctx.Sink)
				resultCh <- toolCallResultPair{r, e}
			}()

			// 等待工具返回，每 10 秒推送心跳
			var tcResult *ToolCallResult
			var err error
			heartbeatTicker := time.NewTicker(10 * time.Second)
		waitLoop:
			for {
				select {
				case pair := <-resultCh:
					tcResult, err = pair.result, pair.err
					break waitLoop
				case <-heartbeatTicker.C:
					elapsed := time.Since(start)
					ctx.Sink.OnEvent("tool_progress", fmt.Sprintf("⏳ %s 执行中 (%s)...", originalName, fmtDuration(elapsed)))
				}
			}
			heartbeatTicker.Stop()
			duration := time.Since(start)

			var result string
			var toAgent, fromAgent string
			if tcResult != nil {
				result = tcResult.Result
				toAgent = tcResult.AgentID
				fromAgent = tcResult.FromID
			}

			// 工具调用期间被取消：立即中断，不记录为失败
			if err != nil && ctx.Ctx != nil && ctx.Ctx.Err() != nil {
				log.Printf("[processTask] ✗ 工具调用期间任务被取消 tool=%s taskID=%s", originalName, ctx.TaskID)
				finalText = "任务已停止。"
				rootSession.SetStatus("cancelled")
				ctx.Sink.OnEvent("task_cancelled", "任务已被用户停止")
				break
			}

			// ExecuteCode 特殊处理：解析结构化 JSON，提取 stdout 给 LLM，tool_calls 展示给用户
			var execToolCallsSummary string
			if originalName == "ExecuteCode" && result != "" {
				stdout, summary := parseExecuteCodeResult(result)
				if stdout != "" {
					result = stdout // LLM 只看到 stdout
				}
				execToolCallsSummary = summary
			}

			success := true
			if err != nil {
				log.Printf("[processTask] ✗ 工具调用失败: %s →agent=%s duration=%v error=%v", originalName, toAgent, duration, err)
				success = false

				// ExecuteCode 特殊处理：从结构化结果中提取 stderr 详情，让 LLM 能看到具体错误并修正代码
				if originalName == "ExecuteCode" && result != "" {
					stderr := extractExecuteCodeStderr(result)
					if stderr != "" {
						result = fmt.Sprintf("ExecuteCode 执行失败: %v\n错误详情:\n%s", err, stderr)
					} else {
						result = fmt.Sprintf("ExecuteCode 执行失败: %v\n原始结果: %s", err, truncate(result, 1000))
					}
				} else {
					result = fmt.Sprintf("工具调用失败: %v", err)
				}

				ctx.Sink.OnEvent("tool_result", fmt.Sprintf("❌ %s 失败 →%s (%.1fs): %v", originalName, toAgent, duration.Seconds(), err))
			} else if originalName == "ExecuteCode" {
				log.Printf("[processTask] ← ExecuteCode返回: →agent=%s duration=%v stdoutLen=%d",
					toAgent, duration, len(result))
				eventText := fmt.Sprintf("✅ ExecuteCode (%.1fs)", duration.Seconds())
				if execToolCallsSummary != "" {
					eventText += "\n" + execToolCallsSummary
				}
				eventText += fmt.Sprintf("\n输出: %s", truncate(result, 300))
				ctx.Sink.OnEvent("tool_result", eventText)
			} else {
				log.Printf("[processTask] ← 工具返回: %s →agent=%s ←from=%s duration=%v resultLen=%d result=%s",
					originalName, toAgent, fromAgent, duration, len(result), truncate(result, 200))
				// 尝试提取标准格式的 message
				var stdResult struct {
					Data    any    `json:"data"`
					Message string `json:"message"`
				}
				if json.Unmarshal([]byte(result), &stdResult) == nil && stdResult.Message != "" {
					ctx.Sink.OnEvent("tool_result", fmt.Sprintf("✅ %s: %s", originalName, stdResult.Message))
				} else {
					ctx.Sink.OnEvent("tool_result", fmt.Sprintf("✅ %s [%s→%s] (%.1fs)\n结果: %s", originalName, toAgent, fromAgent, duration.Seconds(), truncate(result, 300)))
				}
			}

			// 记录工具调用到 session
			toolRecord := ToolCallRecord{
				ID:         tc.ID,
				ToolName:   originalName,
				Arguments:  tc.Function.Arguments,
				Result:     result,
				Success:    success,
				DurationMs: duration.Milliseconds(),
				Timestamp:  time.Now(),
				Iteration:  i,
			}
			rootSession.RecordToolCall(toolRecord)

			// 记录到 Trace
			currentRound.ToolCalls = append(currentRound.ToolCalls, TraceToolCall{
				ToolName:   originalName,
				Arguments:  truncate(tc.Function.Arguments, 100),
				Success:    success,
				DurationMs: duration.Milliseconds(),
				ResultLen:  len(result),
			})

			// 检测业务失败（transport 成功但 result JSON 中 success:false）
			if err == nil && tcResult != nil && tcResult.Result != "" {
				var bizResult struct {
					Success bool   `json:"success"`
					Error   string `json:"error"`
				}
				if json.Unmarshal([]byte(tcResult.Result), &bizResult) == nil && !bizResult.Success && bizResult.Error != "" {
					bizFailedTools = append(bizFailedTools, originalName)
					bizFailedMsgs = append(bizFailedMsgs, bizResult.Error)
					log.Printf("[processTask] 业务失败检测: %s → %s", originalName, bizResult.Error)
				}
			}

			// 触发工具调用 hook
			b.hooks.FireToolCall(ctx, toolRecord)

			toolMsg := Message{
				Role:       "tool",
				Content:    truncateToolResult(result, i),
				ToolCallID: tc.ID,
			}
			rootSession.AppendMessage(toolMsg)
			messages = append(messages, toolMsg)
		}

		// 工具业务失败 → 扩展同 agent 兄弟工具，让 LLM 自行决策修复参数或切换工具
		if len(bizFailedTools) > 0 {
			existingSet := make(map[string]bool, len(tools))
			for _, t := range tools {
				existingSet[t.Function.Name] = true
			}
			var newToolNames []string
			for _, failedTool := range bizFailedTools {
				siblings := b.getSiblingTools(failedTool)
				for _, s := range siblings {
					if !existingSet[s.Function.Name] {
						existingSet[s.Function.Name] = true
						tools = append(tools, s)
						newToolNames = append(newToolNames, b.resolveToolName(s.Function.Name))
					}
				}
			}
			if len(newToolNames) > 0 {
				var failInfo strings.Builder
				for idx, name := range bizFailedTools {
					failInfo.WriteString(fmt.Sprintf("- %s: %s\n", name, bizFailedMsgs[idx]))
				}
				hint := fmt.Sprintf("以下工具返回业务失败:\n%s已补充同 Agent 的替代工具: %s\n你可以选择修复参数重试原工具，或使用替代工具完成任务。",
					failInfo.String(), strings.Join(newToolNames, ", "))
				messages = append(messages, Message{Role: "system", Content: hint})
				log.Printf("[processTask] 业务失败扩展: 新增 %d 个兄弟工具: %v", len(newToolNames), newToolNames)
				ctx.Sink.OnEvent("tool_expand", fmt.Sprintf("工具业务失败，补充兄弟工具: %s", strings.Join(newToolNames, ", ")))
			}
		}

		// 本轮结束，记录到 Trace
		trace.Rounds = append(trace.Rounds, currentRound)
	}

	// 5. 保存会话（handleComplexTask 内部已处理时跳过）
	if !complexTaskHandled {
		if finalErr != nil {
			rootSession.SetStatus("failed")
			rootSession.SetError(finalErr.Error())
		} else {
			rootSession.SetStatus("done")
		}
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
	}

	// 确保返回值非空
	if finalText == "" && finalErr != nil {
		finalText = fmt.Sprintf("抱歉，AI 服务暂时不可用: %v", finalErr)
	}
	if finalText == "" {
		finalText = "抱歉，未能生成回复。"
	}

	totalDuration := time.Since(taskStart)
	status := "done"
	if finalErr != nil {
		status = "failed"
	}
	log.Printf("[processTask] ◀ 处理完成 taskID=%s source=%s status=%s duration=%v resultLen=%d",
		ctx.TaskID, ctx.Source, status, totalDuration, len(finalText))

	// 输出 Trace 摘要日志
	if traceSummary := trace.Summary(); traceSummary != "" {
		log.Print(traceSummary)
	}

	// 触发任务结束 hook（收集所有工具调用记录）
	rootSession.mu.Lock()
	allToolCalls := make([]ToolCallRecord, len(rootSession.ToolCalls))
	copy(allToolCalls, rootSession.ToolCalls)
	rootSession.mu.Unlock()
	b.hooks.FireTaskEnd(ctx, finalText, allToolCalls, finalErr)

	// 记忆系统：自动收集工具调用错误
	if b.memoryCollector != nil && len(allToolCalls) > 0 {
		go b.memoryCollector.CollectAfterTask(allToolCalls)
	}

	return finalText, finalErr
}

// handleComplexTask 处理复杂任务：规划 → 编排 → 汇总
func (b *Bridge) handleComplexTask(
	ctx *TaskContext,
	rootSession *TaskSession,
	store *SessionStore,
	tools []LLMTool,
	completedWork string, // 简单路径中已完成的工具调用摘要（可为空）
) string {
	complexStart := time.Now()
	sendEvent := func(event, text string) {
		ctx.Sink.OnEvent(event, text)
	}

	query := ctx.Query
	if query == "" {
		query = rootSession.Title
	}

	// ① 规划阶段
	log.Printf("[ComplexTask] ▶ 开始复杂任务处理 taskID=%s query=%s", ctx.TaskID, truncate(query, 100))

	// 检查是否被用户取消
	if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
		log.Printf("[ComplexTask] ✗ 规划前任务被取消 taskID=%s", ctx.TaskID)
		rootSession.SetStatus("cancelled")
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
		return "任务已停止。"
	}

	sendEvent("plan_start", "正在分析任务...")

	maxSubTasks := b.cfg.MaxSubTasks
	if maxSubTasks <= 0 {
		maxSubTasks = 10
	}

	// 使用全部可用 skill 构建规划指引
	var skillBlock string
	if b.skillMgr != nil {
		allSkills := b.skillMgr.GetAllSkills()
		if len(allSkills) > 0 {
			skillBlock = b.skillMgr.BuildSkillBlock(allSkills)
		}
	}

	planStart := time.Now()
	activeCfg := b.activeLLM.Get()
	plan, err := PlanTask(&activeCfg, query, tools, ctx.Account, maxSubTasks, completedWork, skillBlock, b.cfg.Fallbacks, b.fallbackCooldown())
	planDuration := time.Since(planStart)

	if err != nil {
		log.Printf("[ComplexTask] ✗ 任务规划失败 duration=%v error=%v", planDuration, err)
		sendEvent("plan_done", fmt.Sprintf("任务规划失败: %v", err))
		// 保存失败状态
		rootSession.SetStatus("failed")
		rootSession.SetError(err.Error())
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
		return fmt.Sprintf("抱歉，任务规划失败: %v", err)
	}

	// 打印计划详情
	log.Printf("[ComplexTask] ✓ 任务规划完成 duration=%v subtasks=%d mode=%s", planDuration, len(plan.SubTasks), plan.ExecutionMode)
	for i, st := range plan.SubTasks {
		log.Printf("[ComplexTask]   子任务[%d] id=%s title=%s depends=%v tools_hint=%v",
			i+1, st.ID, st.Title, st.DependsOn, st.ToolsHint)
	}

	rootSession.Plan = plan
	store.Save(rootSession)

	// 触发计划创建 hook
	b.hooks.FirePlanCreated(ctx, plan)

	// 发送规划耗时
	sendEvent("plan_timing", fmt.Sprintf("任务规划完成，耗时 %s，拆解为 %d 个子任务", fmtDuration(planDuration), len(plan.SubTasks)))

	// 发送计划摘要
	var planSummary strings.Builder
	planSummary.WriteString(fmt.Sprintf("拆解为 %d 个子任务: ", len(plan.SubTasks)))
	for i, st := range plan.SubTasks {
		if i > 0 {
			planSummary.WriteString(" → ")
		}
		planSummary.WriteString(fmt.Sprintf("(%d)%s", i+1, st.Title))
	}
	sendEvent("plan_done", planSummary.String())

	// 发送每个子任务的详细信息（含 tool_params）
	sendPlanDetails := func(subtasks []SubTaskPlan) {
		for i, st := range subtasks {
			var detail strings.Builder
			detail.WriteString(fmt.Sprintf("📌 子任务[%d/%d] %s\n", i+1, len(subtasks), st.Title))
			detail.WriteString(fmt.Sprintf("  描述: %s\n", st.Description))
			if len(st.DependsOn) > 0 {
				detail.WriteString(fmt.Sprintf("  依赖: %s\n", strings.Join(st.DependsOn, ", ")))
			}
			if len(st.ToolsHint) > 0 {
				detail.WriteString(fmt.Sprintf("  工具: %s\n", strings.Join(st.ToolsHint, ", ")))
			}
			if len(st.ToolParams) > 0 {
				detail.WriteString("  参数:\n")
				for k, v := range st.ToolParams {
					detail.WriteString(fmt.Sprintf("    - %s: %v\n", k, v))
				}
			}
			sendEvent("plan_detail", detail.String())
		}
	}
	sendPlanDetails(plan.SubTasks)

	// 检查是否被用户取消
	if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
		log.Printf("[ComplexTask] ✗ 审查前任务被取消 taskID=%s", ctx.TaskID)
		rootSession.SetStatus("cancelled")
		store.Save(rootSession)
		store.SaveIndex(rootSession, nil)
		return "任务已停止。"
	}

	// ② LLM审查计划（含 agent 能力信息，确保工具参数完整性）
	sendEvent("plan_review_start", "正在审查计划参数...")
	agentCapabilities := b.getAgentDescriptionBlock()
	reviewStart := time.Now()
	reviewCfg := b.activeLLM.Get()
	review, err := ReviewPlan(&reviewCfg, query, plan, tools, ctx.Account, agentCapabilities, b.cfg.Fallbacks, b.fallbackCooldown())
	reviewDuration := time.Since(reviewStart)
	if err != nil {
		log.Printf("[ComplexTask] ⚠ 计划审查失败 error=%v，继续执行原计划", err)
		sendEvent("plan_review_result", fmt.Sprintf("计划审查跳过: %v", err))
	} else if review.Action == "optimize" && review.Plan != nil {
		log.Printf("[ComplexTask] 计划已优化: %s", review.Reason)
		plan = review.Plan
		rootSession.Plan = plan
		store.Save(rootSession)
		sendEvent("plan_review_result", fmt.Sprintf("计划已优化: %s", review.Reason))
		// 重新展示优化后的计划摘要
		var optimizedSummary strings.Builder
		optimizedSummary.WriteString(fmt.Sprintf("优化后 %d 个子任务: ", len(plan.SubTasks)))
		for i, st := range plan.SubTasks {
			if i > 0 {
				optimizedSummary.WriteString(" → ")
			}
			optimizedSummary.WriteString(fmt.Sprintf("(%d)%s", i+1, st.Title))
		}
		sendEvent("plan_done", optimizedSummary.String())
		// 重新展示优化后的子任务详情
		sendPlanDetails(plan.SubTasks)
	} else {
		reason := "审查通过"
		if review != nil && review.Reason != "" {
			reason = review.Reason
		}
		log.Printf("[ComplexTask] 计划审查通过: %s", reason)
		sendEvent("plan_review_result", fmt.Sprintf("计划审查通过: %s", reason))
	}
	sendEvent("review_timing", fmt.Sprintf("审查完成，耗时 %s", fmtDuration(reviewDuration)))

	// ③ 为每个子任务创建 ChildSession
	childSessions := make(map[string]*TaskSession)
	for _, st := range plan.SubTasks {
		child := NewChildSession(rootSession, st.Title, st.Description)
		child.ID = st.ID
		childSessions[st.ID] = child
		rootSession.AddChildID(st.ID)
		store.Save(child)
	}
	store.Save(rootSession)

	// ④ 编排执行
	log.Printf("[ComplexTask] ── 开始编排执行 ──")
	execStart := time.Now()
	orchestrator := NewOrchestrator(b, store)
	results := orchestrator.Execute(ctx.Ctx, ctx.TaskID, rootSession, childSessions, tools, sendEvent)
	execDuration := time.Since(execStart)

	// 检查是否被用户取消
	cancelled := ctx.Ctx != nil && ctx.Ctx.Err() != nil
	if cancelled {
		log.Printf("[ComplexTask] ✗ 任务被取消 taskID=%s", ctx.TaskID)
		rootSession.SetStatus("cancelled")
		store.Save(rootSession)
		var childList []*TaskSession
		for _, c := range childSessions {
			childList = append(childList, c)
		}
		store.SaveIndex(rootSession, childList)
		return "任务已停止。"
	}

	// 触发子任务完成 hook + 汇总子任务工具调用到 rootSession
	for _, r := range results {
		b.hooks.FireSubTaskDone(ctx, r)

		// 将子任务的成功 ToolCalls 汇总到 rootSession（供 FireTaskEnd 统计使用）
		if child, ok := childSessions[r.SubTaskID]; ok {
			child.mu.Lock()
			childCalls := make([]ToolCallRecord, len(child.ToolCalls))
			copy(childCalls, child.ToolCalls)
			child.mu.Unlock()
			for _, tc := range childCalls {
				if tc.Success {
					rootSession.RecordToolCall(tc)
				}
			}
		}
	}

	// 统计结果
	var doneCount, failCount, skipCount, asyncCount, deferCount int
	var asyncInfos []AsyncSessionInfo
	for _, r := range results {
		switch r.Status {
		case "done":
			doneCount++
		case "failed":
			failCount++
		case "skipped":
			skipCount++
		case "async":
			asyncCount++
			asyncInfos = append(asyncInfos, r.AsyncSessions...)
		case "deferred":
			deferCount++
		}
	}
	log.Printf("[ComplexTask] ✓ 编排执行完成 duration=%v total=%d done=%d failed=%d skipped=%d async=%d deferred=%d",
		execDuration, len(results), doneCount, failCount, skipCount, asyncCount, deferCount)

	// 发送编排执行完成统计
	sendEvent("progress", fmt.Sprintf("编排执行完成，耗时 %s — 成功:%d 失败:%d 跳过:%d",
		fmtDuration(execDuration), doneCount, failCount, skipCount))

	// 检测异步子任务 → 跳过 Synthesize，返回即时确认
	if asyncCount > 0 {
		log.Printf("[ComplexTask] async subtasks detected: async=%d deferred=%d, skip synthesis", asyncCount, deferCount)
		_ = asyncInfos // asyncInfos 已包含在 results 中

		summary := buildAsyncAcknowledgment(results)
		rootSession.SetStatus("async")
		rootSession.SetResult(summary)
		rootSession.Summary = summary
		store.Save(rootSession)

		var childList []*TaskSession
		for _, c := range childSessions {
			childList = append(childList, c)
		}
		store.SaveIndex(rootSession, childList)

		totalDuration := time.Since(complexStart)
		log.Printf("[ComplexTask] ◀ 异步任务确认 taskID=%s duration=%v async=%d deferred=%d",
			ctx.TaskID, totalDuration, asyncCount, deferCount)
		return summary
	}

	// ⑤ 汇总（仅在无异步子任务时执行）
	log.Printf("[ComplexTask] ── 开始汇总 ──")
	synthStart := time.Now()
	summary := orchestrator.Synthesize(rootSession, childSessions, results, query, sendEvent)
	synthDuration := time.Since(synthStart)
	log.Printf("[ComplexTask] ✓ 汇总完成 duration=%v summaryLen=%d", synthDuration, len(summary))

	rootSession.SetStatus("done")
	rootSession.SetResult(summary)
	rootSession.Summary = summary
	store.Save(rootSession)

	// 保存索引（含子会话）
	var childList []*TaskSession
	for _, c := range childSessions {
		childList = append(childList, c)
	}
	store.SaveIndex(rootSession, childList)

	totalDuration := time.Since(complexStart)
	log.Printf("[ComplexTask] ◀ 复杂任务完成 taskID=%s duration=%v (plan=%v exec=%v synth=%v)",
		ctx.TaskID, totalDuration, planDuration, execDuration, synthDuration)

	return summary
}

// truncate 截断字符串用于日志显示（UTF-8 安全，按字符数截断）
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// buildAsyncAcknowledgment 构建异步任务即时确认消息
func buildAsyncAcknowledgment(results []SubTaskResult) string {
	var sb strings.Builder
	sb.WriteString("📋 任务已派发，进度将通过微信推送\n\n")

	for _, r := range results {
		switch r.Status {
		case "done":
			sb.WriteString(fmt.Sprintf("✅ %s\n", r.Title))
		case "failed":
			sb.WriteString(fmt.Sprintf("❌ %s: %s\n", r.Title, r.Error))
		case "skipped":
			sb.WriteString(fmt.Sprintf("⏭ %s\n", r.Title))
		case "async":
			var sids []string
			for _, a := range r.AsyncSessions {
				sids = append(sids, a.SessionID)
			}
			sb.WriteString(fmt.Sprintf("⏳ %s (后台执行中: %s)\n", r.Title, strings.Join(sids, ", ")))
		case "deferred":
			sb.WriteString(fmt.Sprintf("⏸ %s (等待前置任务完成)\n", r.Title))
		}
	}
	return sb.String()
}

// formatExecuteCodeEvent 格式化 ExecuteCode 工具调用的展示
func formatExecuteCodeEvent(argsJSON string, idx, total int) string {
	var args struct {
		Code        string   `json:"code"`
		Description string   `json:"description"`
		ToolsHint   []string `json:"tools_hint"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("调用 ExecuteCode (%d/%d)\n参数: %s", idx, total, argsJSON)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🐍 ExecuteCode (%d/%d)", idx, total))
	if args.Description != "" {
		sb.WriteString(fmt.Sprintf("\n说明: %s", args.Description))
	}
	if len(args.ToolsHint) > 0 {
		sb.WriteString(fmt.Sprintf("\n工具: %s", strings.Join(args.ToolsHint, ", ")))
	}

	// 限制代码显示长度，防止超出微信消息长度限制
	const maxCodeDisplay = 190000 // 字符数（预留头部空间，微信限制约204800）
	code := args.Code
	codeRunes := []rune(code)
	if len(codeRunes) > maxCodeDisplay {
		code = string(codeRunes[:maxCodeDisplay]) + "\n# ... [代码已截断，共" + fmt.Sprintf("%d", len(codeRunes)) + "字符]"
	}

	sb.WriteString(fmt.Sprintf("\n```python\n%s\n```", code))
	return sb.String()
}

// truncateToolResult 截断工具结果，防止上下文溢出
// 前 3 轮 max 3000 字符，之后 max 1500 字符
func truncateToolResult(result string, iteration int) string {
	maxLen := 3000
	if iteration >= 3 {
		maxLen = 1500
	}
	runes := []rune(result)
	if len(runes) <= maxLen {
		return result
	}
	return string(runes[:maxLen]) + "\n...[结果已截断]"
}

// 上下文字符预算常量
const (
	processMaxTotalChars = 150000 // 总字符预算
	processMaxMessages   = 40     // 最大消息数
)

// estimateChars 估算消息列表的总字符数
func estimateChars(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Arguments)
		}
	}
	return total
}

// sanitizeProcessMessages 参考 blog-agent SanitizeMessages 模式
// 从末尾向前保留消息，超出字符预算或消息数上限时停止
// 始终保留 system prompt（messages[0]）
func sanitizeProcessMessages(messages []Message) []Message {
	if len(messages) <= 2 {
		return messages
	}

	// 始终保留 system prompt
	systemMsg := messages[0]
	rest := messages[1:]

	// 从末尾向前遍历，累计字符数
	systemChars := len(systemMsg.Content)
	charBudget := processMaxTotalChars - systemChars
	msgBudget := processMaxMessages - 1 // 减去 system prompt

	var kept []Message
	totalChars := 0
	for i := len(rest) - 1; i >= 0; i-- {
		msg := rest[i]
		msgChars := len(msg.Content)
		for _, tc := range msg.ToolCalls {
			msgChars += len(tc.Function.Arguments)
		}

		if len(kept) >= msgBudget || totalChars+msgChars > charBudget {
			break
		}
		kept = append(kept, msg)
		totalChars += msgChars
	}

	// 反转 kept（因为是从末尾向前收集的）
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}

	result := make([]Message, 0, 1+len(kept))
	result = append(result, systemMsg)
	result = append(result, kept...)
	return result
}

// parseExecuteCodeResult 解析 execute-code-agent 返回的结构化 JSON
// 返回 (stdout 给 LLM, tool_calls 摘要给用户展示)
func parseExecuteCodeResult(resultJSON string) (string, string) {
	var execResult struct {
		Success    bool   `json:"success"`
		Stdout     string `json:"stdout"`
		Stderr     string `json:"stderr"`
		DurationMs int64  `json:"duration_ms"`
		ToolCalls  []struct {
			Tool     string `json:"tool"`
			AgentID  string `json:"agent_id"`
			Success  bool   `json:"success"`
			Duration int64  `json:"duration_ms"`
			Error    string `json:"error"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &execResult); err != nil {
		// 不是结构化 JSON，原样返回
		return resultJSON, ""
	}

	// 构建工具调用链摘要
	var summary string
	if len(execResult.ToolCalls) > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📡 内部调用 %d 个工具:", len(execResult.ToolCalls)))
		for _, tc := range execResult.ToolCalls {
			status := "✅"
			if !tc.Success {
				status = "❌"
			}
			agent := tc.AgentID
			if agent == "" {
				agent = "?"
			}
			sb.WriteString(fmt.Sprintf("\n  %s %s →%s (%dms)", status, tc.Tool, agent, tc.Duration))
			if tc.Error != "" {
				sb.WriteString(fmt.Sprintf(" %s", truncate(tc.Error, 80)))
			}
		}
		summary = sb.String()
	}

	return execResult.Stdout, summary
}

// extractExecuteCodeStderr 从 ExecuteCode 的结构化结果中提取 stderr 错误详情
// 用于 ExecuteCode 失败时将具体错误信息传递给 LLM，使其能修正代码
func extractExecuteCodeStderr(resultJSON string) string {
	// 尝试从 BuildToolResult 包装的 JSON 中提取
	var wrapper struct {
		Data struct {
			Stderr    string `json:"stderr"`
			ErrorType string `json:"error_type"`
			Stdout    string `json:"stdout"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &wrapper); err == nil && wrapper.Data.Stderr != "" {
		return wrapper.Data.Stderr
	}

	// 直接作为 execResult 解析
	var execResult struct {
		Stderr    string `json:"stderr"`
		ErrorType string `json:"error_type"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &execResult); err == nil && execResult.Stderr != "" {
		return execResult.Stderr
	}

	return ""
}

// executeSkillSubTask 在独立子任务中执行技能
func (b *Bridge) executeSkillSubTask(ctx *TaskContext, skillName, query string, parentTools []LLMTool) string {
	start := time.Now()

	// 1. 查找 skill
	skill := b.skillMgr.GetSkill(skillName)
	if skill == nil {
		log.Printf("[SkillSubTask] 技能不存在: %s", skillName)
		return fmt.Sprintf("技能 '%s' 不存在，可用技能请参考 Skill 目录。", skillName)
	}

	// 1.5 检查所需 agent 是否在线
	if len(skill.Agents) > 0 {
		b.catalogMu.RLock()
		for _, requiredPrefix := range skill.Agents {
			found := false
			for agentID := range b.agentInfo {
				if strings.HasPrefix(agentID, requiredPrefix) {
					found = true
					break
				}
			}
			if !found {
				b.catalogMu.RUnlock()
				log.Printf("[SkillSubTask] ✗ 技能 %s 所需 agent %s 不在线", skillName, requiredPrefix)
				return fmt.Sprintf("技能 %s 无法执行：所需 agent %s offline", skillName, requiredPrefix)
			}
		}
		b.catalogMu.RUnlock()
	}

	// 2. 构建子任务 system prompt
	var sb strings.Builder
	now := time.Now()
	sb.WriteString(fmt.Sprintf("account: %s\n当前时间: %s %s\n\n",
		ctx.Account, now.Format("2006-01-02 15:04"), chineseWeekday(now.Weekday())))

	// 注入 agent 能力描述
	agentBlock := b.getAgentDescriptionBlock()
	if agentBlock != "" {
		sb.WriteString(agentBlock)
		sb.WriteString("\n")
	}

	// 注入 skill 详情（含历史经验）
	skillBlock := b.skillMgr.BuildSkillBlock([]SkillEntry{*skill})
	sb.WriteString(skillBlock)

	// 注入执行策略：优先 ExecuteCode 批量调用
	sb.WriteString("\n## 执行策略\n")
	sb.WriteString("**必须优先使用 ExecuteCode 工具**批量调用多个工具并整合数据。\n")
	sb.WriteString("- 将多个工具调用组合到一段代码中一次性执行，避免逐个调用工具进行多轮交互\n")
	sb.WriteString("- 在 ExecuteCode 代码中，直接使用 call_tool 调用具体工具（如 RawAddTodo, RawGetTodosByDate），不要调用 execute_skill\n")
	sb.WriteString("- call_tool 返回值已自动解析为 dict/list，无需再 json.loads\n")
	sb.WriteString("- 只在 ExecuteCode 无法覆盖的场景才单独调用工具\n")
	sb.WriteString("- 最终回复要简洁，直接给出用户需要的数据结果\n")

	// 3. 过滤工具（从全量工具列表中筛选，因为主列表已隐藏 skill 工具）
	allTools := b.getLLMTools()
	filteredTools := b.filterToolsForSkill(skill, allTools)
	log.Printf("[SkillSubTask] skill=%s tools=%d query=%s", skillName, len(filteredTools), truncate(query, 200))

	// 注入工具参数参考（让 LLM 在 ExecuteCode 中写 call_tool 时有直接参考）
	toolRef := b.buildToolParamReference(filteredTools)
	if toolRef != "" {
		sb.WriteString(toolRef)
	}

	systemPrompt := sb.String()

	// 4. Mini agentic loop（使用 ExecuteCode 批量调用时通常 2-3 轮即可完成）
	maxIter := 5
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	llmCfg, llmFallbacks := b.GetLLMConfigForSource(ctx.Source)
	var finalText string

	// 记录子任务中的工具调用摘要（用于返回给主 LLM 提供充分上下文）
	type skillToolCall struct {
		Name    string
		Args    string
		Success bool
		Result  string
	}
	var skillCalls []skillToolCall

	for i := 0; i < maxIter; i++ {
		// 检查取消
		if ctx.Ctx != nil && ctx.Ctx.Err() != nil {
			log.Printf("[SkillSubTask] 取消 skill=%s", skillName)
			return "技能执行已取消。"
		}

		// 消息压缩
		if len(messages) > 15 || estimateChars(messages) > processMaxTotalChars*80/100 {
			before := len(messages)
			messages = sanitizeProcessMessages(messages)
			if len(messages) != before {
				log.Printf("[SkillSubTask] skill=%s 消息压缩: %d → %d", skillName, before, len(messages))
			}
		}

		log.Printf("[SkillSubTask] skill=%s 迭代 %d/%d messages=%d", skillName, i+1, maxIter, len(messages))

		// 最后一轮强制收敛
		iterTools := filteredTools
		if i == maxIter-1 {
			iterTools = nil
			messages = append(messages, Message{
				Role:    "system",
				Content: "请立即给出最终回复，总结已完成的工作和结果。不要再调用任何工具。",
			})
		}

		text, toolCalls, err := b.sendLLMWithConfig(llmCfg, llmFallbacks, messages, iterTools)
		if err != nil {
			log.Printf("[SkillSubTask] LLM 失败 skill=%s error=%v", skillName, err)
			return fmt.Sprintf("技能 %s 执行失败: %v", skillName, err)
		}

		// 无工具调用 → 完成
		if len(toolCalls) == 0 {
			finalText = text
			break
		}

		messages = append(messages, Message{Role: "assistant", Content: "", ToolCalls: toolCalls})

		// 执行工具调用
		for _, tc := range toolCalls {
			originalName := b.resolveToolName(tc.Function.Name)
			log.Printf("[SkillSubTask] skill=%s → 调用工具: %s args=%s",
				skillName, originalName, truncate(tc.Function.Arguments, 500))

			ctx.Sink.OnEvent("skill_tool_call", fmt.Sprintf("[%s] 调用 %s\n参数: %s",
				skillName, originalName, truncate(tc.Function.Arguments, 300)))

			// 检查工具是否在可用列表中（防止 LLM 编造工具名）
			if !isToolInList(originalName, filteredTools) {
				var availNames []string
				for _, ft := range filteredTools {
					availNames = append(availNames, ft.Function.Name)
				}
				result := fmt.Sprintf("工具 %s 不存在。可用工具: %s\n请使用正确的工具名重试。",
					originalName, strings.Join(availNames, ", "))
				log.Printf("[SkillSubTask] skill=%s 工具不存在: %s", skillName, originalName)
				ctx.Sink.OnEvent("skill_tool_result", fmt.Sprintf("[%s] ❌ %s 不存在，已提示可用工具列表",
					skillName, originalName))
				messages = append(messages, Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				})
				continue
			}

			callCtx := ctx.Ctx
			if callCtx == nil {
				callCtx = context.Background()
			}
			toolStart := time.Now()
			tcResult, err := b.CallToolCtx(callCtx, originalName, json.RawMessage(tc.Function.Arguments))

			var result string
			if tcResult != nil {
				result = tcResult.Result
			}
			if err != nil {
				result = fmt.Sprintf("工具调用失败: %v", err)
				log.Printf("[SkillSubTask] skill=%s 工具失败: %s error=%v", skillName, originalName, err)
				ctx.Sink.OnEvent("skill_tool_result", fmt.Sprintf("[%s] ❌ %s 失败 (%s)\n%s",
					skillName, originalName, fmtDuration(time.Since(toolStart)), truncate(result, 500)))
				skillCalls = append(skillCalls, skillToolCall{Name: originalName, Args: truncate(tc.Function.Arguments, 150), Success: false, Result: truncate(result, 200)})
			} else {
				log.Printf("[SkillSubTask] skill=%s ← 工具返回: %s resultLen=%d",
					skillName, originalName, len(result))
				ctx.Sink.OnEvent("skill_tool_result", fmt.Sprintf("[%s] ✅ %s 成功 (%s, %d字符)\n%s",
					skillName, originalName, fmtDuration(time.Since(toolStart)), len(result), truncate(result, 500)))
				skillCalls = append(skillCalls, skillToolCall{Name: originalName, Args: truncate(tc.Function.Arguments, 150), Success: true, Result: truncate(result, 300)})
			}

			messages = append(messages, Message{
				Role:       "tool",
				Content:    truncateToolResult(result, i),
				ToolCallID: tc.ID,
			})
		}
	}

	duration := time.Since(start)
	ctx.Sink.OnEvent("skill_done", fmt.Sprintf("技能 %s 执行完成 (%s)", skillName, fmtDuration(duration)))
	log.Printf("[SkillSubTask] ✓ skill=%s 完成 duration=%v resultLen=%d calls=%d", skillName, duration, len(finalText), len(skillCalls))

	// 构建结构化返回：执行日志 + LLM 总结
	// 让主 LLM 清楚知道子任务做了什么、调了哪些工具、结果如何
	var result strings.Builder
	if len(skillCalls) > 0 {
		result.WriteString(fmt.Sprintf("技能 %s 执行日志（%s，%d次工具调用）:\n", skillName, fmtDuration(duration), len(skillCalls)))
		for _, sc := range skillCalls {
			status := "✅"
			if !sc.Success {
				status = "❌"
			}
			result.WriteString(fmt.Sprintf("  %s %s(%s) → %s\n", status, sc.Name, sc.Args, sc.Result))
		}
		result.WriteString("\n")
	}
	if finalText != "" {
		result.WriteString(finalText)
	} else {
		result.WriteString(fmt.Sprintf("技能 %s 已执行但未产生总结。", skillName))
	}
	return result.String()
}

// filterToolsForSkill 按 skill 声明过滤工具
func (b *Bridge) filterToolsForSkill(skill *SkillEntry, parentTools []LLMTool) []LLMTool {
	// skill.Tools 为空 → 使用全量 parentTools（排除虚拟工具）
	if len(skill.Tools) == 0 {
		var filtered []LLMTool
		for _, t := range parentTools {
			name := t.Function.Name
			if name == "plan_and_execute" || name == "execute_skill" || name == "set_persona" || name == "set_rule" {
				continue
			}
			filtered = append(filtered, t)
		}
		return filtered
	}

	// skill.Tools 非空 → 只保留声明的工具 + ExecuteCode 基础工具
	hintSet := make(map[string]bool, len(skill.Tools))
	for _, t := range skill.Tools {
		hintSet[t] = true
		hintSet[sanitizeToolName(t)] = true
	}
	// 始终保留基础工具
	hintSet["ExecuteCode"] = true

	var filtered []LLMTool
	for _, t := range parentTools {
		name := t.Function.Name
		originalName := b.resolveToolName(name)
		if name == "plan_and_execute" || name == "execute_skill" || name == "set_persona" || name == "set_rule" {
			continue
		}
		if hintSet[name] || hintSet[originalName] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// isToolInList 检查工具名是否在工具列表中（考虑 sanitize）
func isToolInList(toolName string, tools []LLMTool) bool {
	for _, t := range tools {
		if t.Function.Name == toolName || unsanitizeToolName(t.Function.Name) == toolName {
			return true
		}
	}
	return false
}

// buildToolParamReference 从工具列表生成参数参考摘要，供 ExecuteCode 中 call_tool 使用
// 利用 toolCatalog 标注工具所属 agent
func (b *Bridge) buildToolParamReference(tools []LLMTool) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## call_tool 参数参考\n")
	sb.WriteString("在 ExecuteCode 中使用 call_tool(tool_name, {参数}) 调用，参数为 dict。\n\n")

	for _, t := range tools {
		name := unsanitizeToolName(t.Function.Name)
		desc := t.Function.Description

		// 查找工具所属 agent
		agentID := ""
		b.catalogMu.RLock()
		if id, ok := b.toolCatalog[name]; ok {
			agentID = id
		}
		b.catalogMu.RUnlock()

		// 解析 parameters JSON Schema 提取字段摘要
		paramSummary := extractParamSummary(t.Function.Parameters)

		// 格式化：有 agentID 时标注
		var toolLine string
		if agentID != "" {
			toolLine = fmt.Sprintf("- **%s** [%s]: %s\n", name, agentID, desc)
		} else {
			toolLine = fmt.Sprintf("- **%s**: %s\n", name, desc)
		}

		sb.WriteString(toolLine)
		if paramSummary != "" {
			sb.WriteString(fmt.Sprintf("  参数: %s\n", paramSummary))
		}
	}
	return sb.String()
}

// extractParamSummary 从 JSON Schema 提取参数摘要（如 "account*(str), date*(str,2025-01-01), id*(str)"）
func extractParamSummary(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}

	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil || len(schema.Properties) == 0 {
		return ""
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var parts []string
	for name, prop := range schema.Properties {
		entry := name
		if requiredSet[name] {
			entry += "*"
		}
		detail := prop.Type
		if prop.Description != "" {
			detail = prop.Description
		}
		entry += "(" + detail + ")"
		parts = append(parts, entry)
	}
	return strings.Join(parts, ", ")
}

