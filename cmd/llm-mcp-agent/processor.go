package main

import (
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
	OnChunk(text string)          // LLM 文本片段
	OnEvent(event, text string)   // 结构化事件 (tool_info, plan_start, plan_done, subtask_*, etc.)
	Streaming() bool              // 是否使用流式 LLM 调用
}

// StreamingSink Web 前端流式输出
type StreamingSink struct {
	bridge *Bridge
	taskID string
}

func (s *StreamingSink) OnChunk(text string)       { s.bridge.sendTaskEvent(s.taskID, "chunk", text) }
func (s *StreamingSink) OnEvent(event, text string) { s.bridge.sendTaskEvent(s.taskID, event, text) }
func (s *StreamingSink) Streaming() bool            { return true }

// BufferSink 缓冲输出（llm_request）
type BufferSink struct {
	buf strings.Builder
}

func (s *BufferSink) OnChunk(text string)       { s.buf.WriteString(text) }
func (s *BufferSink) OnEvent(event, text string) {}
func (s *BufferSink) Streaming() bool            { return false }
func (s *BufferSink) Result() string             { return s.buf.String() }

// LLMRequestSink 缓冲文本 + 转发事件（用于 llm_request 任务，支持 Path 2 进度推送）
type LLMRequestSink struct {
	buf    strings.Builder
	bridge *Bridge
	taskID string
}

func (s *LLMRequestSink) OnChunk(text string)       { s.buf.WriteString(text) }
func (s *LLMRequestSink) OnEvent(event, text string) { s.bridge.sendTaskEvent(s.taskID, event, text) }
func (s *LLMRequestSink) Streaming() bool            { return false }
func (s *LLMRequestSink) Result() string             { return s.buf.String() }

// ========================= TaskContext =========================

// TaskContext 统一任务输入
type TaskContext struct {
	TaskID        string
	Account       string
	Query         string    // 用户问题（用于 plan_and_execute）
	Source        string    // "web" | "wechat" | "llm_request"
	Messages      []Message // 预构建消息（nil 则自动构建）
	SelectedTools []string
	NoTools       bool
	Sink          EventSink
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

func buildMCPToolListReply(query string, tools []LLMTool) (string, bool) {
	if !isMCPToolListQuery(query) {
		return "", false
	}

	if len(tools) == 0 {
		return "??????? MCP ???", true
	}

	sorted := make([]LLMTool, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Function.Name < sorted[j].Function.Name
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("????? %d ? MCP ???\n", len(sorted)))
	for i, tool := range sorted {
		name := unsanitizeToolName(tool.Function.Name)
		desc := strings.TrimSpace(tool.Function.Description)
		if desc == "" {
			desc = "???"
		}
		sb.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, name, desc))
	}
	return strings.TrimSpace(sb.String()), true
}

// ========================= 统一处理函数 =========================

// processTask 统一消息处理核心：构建消息 → 创建会话 → 获取工具 → LLM 循环 → 保存会话
func (b *Bridge) processTask(ctx *TaskContext) (string, error) {
	taskStart := time.Now()
	streaming := ctx.Sink.Streaming()
	log.Printf("[processTask] ▶ 开始处理 taskID=%s source=%s account=%s streaming=%v query=%s",
		ctx.TaskID, ctx.Source, ctx.Account, streaming, truncate(ctx.Query, 100))

	// 1. 构建消息
	var messages []Message
	if ctx.Messages != nil {
		// llm_request: 直接使用预构建消息
		messages = make([]Message, len(ctx.Messages))
		copy(messages, ctx.Messages)
		log.Printf("[processTask] 使用预构建消息 count=%d", len(messages))
	} else {
		// web / wechat: 构建 system prompt + user query
		systemPrompt := b.buildAssistantSystemPrompt(ctx.Account)
		messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: ctx.Query},
		}
		log.Printf("[processTask] 构建系统提示 promptLen=%d", len(systemPrompt))
	}

	// 2. 创建会话（所有来源都持久化）
	store := NewSessionStore(b.cfg.SessionDir)
	query := ctx.Query
	if query == "" && len(messages) > 0 {
		// 对 llm_request，从消息中提取最后一条 user 消息
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				query = messages[i].Content
				break
			}
		}
	}
	rootSession := NewRootSession(ctx.TaskID, query, ctx.Account)
	for _, msg := range messages {
		rootSession.AppendMessage(msg)
	}

	// 3. 获取工具
	var tools []LLMTool
	if ctx.NoTools {
		tools = nil
		log.Printf("[processTask] 工具模式: 禁用")
	} else if len(ctx.SelectedTools) > 0 {
		tools = b.filterToolsBySelection(ctx.SelectedTools)
		log.Printf("[processTask] 工具模式: 用户选择 selected=%d matched=%d", len(ctx.SelectedTools), len(tools))
	} else {
		tools = b.getLLMTools()
		log.Printf("[processTask] 工具模式: 默认加载全部 count=%d", len(tools))
	}

	// 工具路由：>15 时智能筛选

	// ????? MCP ?????????????????? tools schema ??? LLM
	if !ctx.NoTools {
		if directReply, ok := buildMCPToolListReply(query, tools); ok {
			rootSession.AppendMessage(Message{Role: "assistant", Content: directReply})
			rootSession.SetResult(directReply)
			rootSession.SetStatus("done")
			store.Save(rootSession)
			store.SaveIndex(rootSession, nil)
			log.Printf("[processTask] ? direct MCP tool list reply, toolCount=%d", len(tools))
			return directReply, nil
		}
	}

	if !ctx.NoTools && len(tools) > 15 && query != "" {
		beforeCount := len(tools)
		tools = b.routeTools(query, tools)
		log.Printf("[processTask] 工具路由: %d → %d", beforeCount, len(tools))
	}

	// 注入 plan_and_execute（除非 NoTools）
	if !ctx.NoTools {
		tools = append(tools, planAndExecuteTool)
	}

	// 发送工具数量信息
	if !ctx.NoTools && len(tools) > 0 {
		ctx.Sink.OnEvent("tool_info", fmt.Sprintf("[🔧 本次加载 %d 个工具]", len(tools)))
	}

	// 4. LLM 循环
	maxIter := b.cfg.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	var finalText string
	var finalErr error
	complexTaskHandled := false

	for i := 0; i < maxIter; i++ {
		log.Printf("[processTask] ── 迭代 %d/%d ── messages=%d tools=%d", i+1, maxIter, len(messages), len(tools))

		// 发射 thinking 事件
		if i == 0 {
			ctx.Sink.OnEvent("thinking", "正在思考...")
		} else {
			ctx.Sink.OnEvent("thinking", fmt.Sprintf("正在分析工具结果（第%d轮）...", i+1))
		}

		var text string
		var toolCalls []ToolCall
		var err error

		llmStart := time.Now()
		if ctx.Sink.Streaming() {
			log.Printf("[processTask] → 发送流式 LLM 请求...")
			text, toolCalls, err = SendStreamingLLMRequest(&b.cfg.LLM, messages, tools, func(chunk string) {
				ctx.Sink.OnChunk(chunk)
			})
		} else {
			log.Printf("[processTask] → 发送同步 LLM 请求...")
			text, toolCalls, err = SendLLMRequest(&b.cfg.LLM, messages, tools)
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
			tcNames = append(tcNames, unsanitizeToolName(tc.Function.Name))
		}
		log.Printf("[processTask] ← LLM 响应 duration=%v textLen=%d toolCalls=%d tools=%v",
			llmDuration, len(text), len(toolCalls), tcNames)

		// 记录 assistant 消息到 session
		assistantMsg := Message{Role: "assistant", Content: text, ToolCalls: toolCalls}
		rootSession.AppendMessage(assistantMsg)

		// 无工具调用 → 对话结束
		if len(toolCalls) == 0 {
			log.Printf("[processTask] ✓ 对话结束（无工具调用） resultLen=%d", len(text))
			finalText = text
			rootSession.SetResult(text)
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
			log.Printf("[processTask] plan_and_execute triggered: %s", reasoning)

			// 进入复杂任务处理流程（内部处理会话保存）
			result := b.handleComplexTask(ctx, rootSession, store, tools)
			ctx.Sink.OnChunk(result)
			finalText = result
			complexTaskHandled = true
			break
		}

		// 普通工具调用
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   text,
			ToolCalls: toolCalls,
		})

		for _, tc := range toolCalls {
			originalName := unsanitizeToolName(tc.Function.Name)

			ctx.Sink.OnEvent("tool_info", fmt.Sprintf("[Calling tool %s with args %s]", originalName, tc.Function.Arguments))
			log.Printf("[processTask] → 调用工具: %s args=%s", originalName, truncate(tc.Function.Arguments, 200))

			start := time.Now()
			result, err := b.CallTool(originalName, json.RawMessage(tc.Function.Arguments))
			duration := time.Since(start)

			success := true
			if err != nil {
				log.Printf("[processTask] ✗ 工具调用失败: %s duration=%v error=%v", originalName, duration, err)
				result = fmt.Sprintf("工具调用失败: %v", err)
				success = false
			} else {
				log.Printf("[processTask] ← 工具返回: %s duration=%v resultLen=%d result=%s",
					originalName, duration, len(result), truncate(result, 200))
			}

			// 记录工具调用到 session
			rootSession.RecordToolCall(ToolCallRecord{
				ID:         tc.ID,
				ToolName:   originalName,
				Arguments:  tc.Function.Arguments,
				Result:     result,
				Success:    success,
				DurationMs: duration.Milliseconds(),
				Timestamp:  time.Now(),
				Iteration:  i,
			})

			toolMsg := Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			}
			rootSession.AppendMessage(toolMsg)
			messages = append(messages, toolMsg)
		}

		// 最后一次迭代
		if i == maxIter-1 {
			finalText = "抱歉，处理过程过于复杂，请尝试简化您的请求。"
			ctx.Sink.OnChunk("\n\n抱歉，处理过程过于复杂，请尝试简化您的请求。")
		}
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

	return finalText, finalErr
}

// handleComplexTask 处理复杂任务：规划 → 编排 → 汇总
func (b *Bridge) handleComplexTask(
	ctx *TaskContext,
	rootSession *TaskSession,
	store *SessionStore,
	tools []LLMTool,
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
	sendEvent("plan_start", "正在分析任务...")

	maxSubTasks := b.cfg.MaxSubTasks
	if maxSubTasks <= 0 {
		maxSubTasks = 10
	}

	planStart := time.Now()
	plan, err := PlanTask(&b.cfg.LLM, query, tools, ctx.Account, maxSubTasks)
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

	// ② 为每个子任务创建 ChildSession
	childSessions := make(map[string]*TaskSession)
	for _, st := range plan.SubTasks {
		child := NewChildSession(rootSession, st.Title, st.Description)
		child.ID = st.ID
		childSessions[st.ID] = child
		rootSession.AddChildID(st.ID)
		store.Save(child)
	}
	store.Save(rootSession)

	// ③ 编排执行
	log.Printf("[ComplexTask] ── 开始编排执行 ──")
	execStart := time.Now()
	orchestrator := NewOrchestrator(b, store)
	results := orchestrator.Execute(ctx.TaskID, rootSession, childSessions, tools, sendEvent)
	execDuration := time.Since(execStart)

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

	// ④ 汇总（仅在无异步子任务时执行）
	log.Printf("[ComplexTask] ── 开始汇总 ──")
	synthStart := time.Now()
	summary := orchestrator.Synthesize(rootSession, results, query, sendEvent)
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

// truncate 截断字符串用于日志显示
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
