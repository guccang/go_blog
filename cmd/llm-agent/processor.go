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
		strings.Contains(q, "工具")
	if !hasMCP || !hasTool {
		return false
	}

	intents := []string{
		"有哪些", "列出", "查看", "显示", "全部", "所有", "清单", "目录列表", "盘点", "列表", "都有", "哪些",
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
		return "当前没有可用的 MCP 工具。", true
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
			desc = "无描述"
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
	sb.WriteString(fmt.Sprintf("当前共有 %d 个 MCP 工具：\n\n", len(sorted)))

	idx := 1
	for agentID, groupTools := range agentGroups {
		info, hasInfo := agentInfoCopy[agentID]
		if hasInfo && info.Description != "" {
			sb.WriteString(fmt.Sprintf("📦 %s (%s)\n", info.Name, info.Description))
		} else if hasInfo {
			sb.WriteString(fmt.Sprintf("📦 %s\n", info.Name))
		} else {
			sb.WriteString(fmt.Sprintf("📦 %s\n", agentID))
		}
		for _, gt := range groupTools {
			sb.WriteString(fmt.Sprintf("  %d. %s - %s\n", idx, gt.Name, gt.Desc))
			idx++
		}
		sb.WriteString("\n")
	}
	if len(ungrouped) > 0 {
		sb.WriteString("📦 其他\n")
		for _, gt := range ungrouped {
			sb.WriteString(fmt.Sprintf("  %d. %s - %s\n", idx, gt.Name, gt.Desc))
			idx++
		}
	}
	return strings.TrimSpace(sb.String()), true
}

// ========================= 虚拟工具定义 =========================

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

// executeToolCall 执行单个工具调用（供内部使用）
func (b *Bridge) executeToolCall(ctx context.Context, toolName, arguments string, sink EventSink) (*ToolCallResult, error) {
	originalName := b.resolveToolName(toolName)

	b.catalogMu.RLock()
	handler, hasHandler := b.toolHandlers[originalName]
	b.catalogMu.RUnlock()

	if !hasHandler {
		return nil, fmt.Errorf("工具 %s 未找到", originalName)
	}

	return handler(ctx, json.RawMessage(arguments), sink)
}

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
				log.Printf("[processTask] 多轮续接：已刷新 system prompt promptLen=%d prompt:\n%s", len(freshPrompt), freshPrompt)
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
		log.Printf("[processTask] 构建系统提示 promptLen=%d prompt:\n%s", len(systemPrompt), systemPrompt)
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
		var localNames []string  // 本地工具：本进程内执行
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
			// 检查所需 agent 是否在线
			if offline := b.skillMgr.offlineAgents(skill); len(offline) > 0 {
				return &ToolCallResult{
					Result:  fmt.Sprintf("技能 '%s' 当前不可用：所需 agent %s offline。请告知用户该功能暂时无法使用。", a.SkillName, strings.Join(offline, ", ")),
					AgentID: "builtin",
				}, nil
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
			log.Printf("[processTask] → 调用工具: %s args=%s", originalName, tc.Function.Arguments)

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
					originalName, toAgent, fromAgent, duration, len(result), result)
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
