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
		originalName := unsanitizeToolName(tool.Function.Name)
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

// ========================= 统一处理函数 =========================

// processTask 统一消息处理核心：构建消息 → 创建会话 → 获取工具 → LLM 循环 → 保存会话
func (b *Bridge) processTask(ctx *TaskContext) (string, error) {
	taskStart := time.Now()
	streaming := ctx.Sink.Streaming()
	log.Printf("[processTask] ▶ 开始处理 taskID=%s source=%s account=%s streaming=%v query=%s",
		ctx.TaskID, ctx.Source, ctx.Account, streaming, truncate(ctx.Query, 100))

	// 1. 获取工具
	var messages []Message
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

	// 提取 query（构建消息前就需要，用于工具路由和 skill 匹配）
	query := ctx.Query
	if query == "" && ctx.Messages != nil {
		for i := len(ctx.Messages) - 1; i >= 0; i-- {
			if ctx.Messages[i].Role == "user" {
				query = ctx.Messages[i].Content
				break
			}
		}
	}

	// 2. MCP 工具列表查询拦截（在路由之前，使用全量工具展示）
	if !ctx.NoTools {
		if directReply, ok := b.buildMCPToolListReply(query, tools); ok {
			store := NewSessionStore(b.cfg.SessionDir)
			rootSession := NewRootSession(ctx.TaskID, query, ctx.Account)
			rootSession.AppendMessage(Message{Role: "assistant", Content: directReply})
			rootSession.SetResult(directReply)
			rootSession.SetStatus("done")
			store.Save(rootSession)
			store.SaveIndex(rootSession, nil)
			log.Printf("[processTask] direct MCP tool list reply, toolCount=%d", len(tools))
			return directReply, nil
		}
	}

	// 2.5 人设未设置时注入 set_persona 工具（让 LLM 从自然语言中提取人设信息）
	if b.persona != nil && !b.persona.IsConfigured() && !ctx.NoTools {
		tools = append(tools, setPersonaTool)
		log.Printf("[processTask] 人设未设置，注入 set_persona 工具")
	}

	// 3. 静态工具策略管道（替代 LLM 路由）
	var selectedSkills []SkillEntry

	if !ctx.NoTools && query != "" {
		beforeCount := len(tools)
		ctx.Sink.OnEvent("route_info", "正在匹配工具策略...")

		policyResult := b.ApplyPolicyPipeline(query, tools)
		tools = policyResult.Tools
		selectedSkills = policyResult.SelectedSkills

		var routedToolNames []string
		for _, t := range tools {
			routedToolNames = append(routedToolNames, unsanitizeToolName(t.Function.Name))
		}
		ctx.Sink.OnEvent("route_info", fmt.Sprintf("工具策略: %d → %d\n%s",
			beforeCount, len(tools), strings.Join(routedToolNames, ", ")))
		log.Printf("[processTask] 策略管道: %d → %d", beforeCount, len(tools))
	}

	// 4. 构建消息（使用路由后的 tools 做 skill 匹配）
	if ctx.Messages != nil {
		// llm_request: 直接使用预构建消息
		messages = make([]Message, len(ctx.Messages))
		copy(messages, ctx.Messages)
		log.Printf("[processTask] 使用预构建消息 count=%d", len(messages))
	} else {
		// web / wechat: 构建 system prompt + user query（传入预选 skill）
		systemPrompt := b.buildAssistantSystemPrompt(ctx.Account, ctx.Query, tools, selectedSkills)
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

	// 触发任务开始 hook
	b.hooks.FireTaskStart(ctx)

	// 发送工具数量信息（在注入虚拟工具之前，只展示真实工具）
	if !ctx.NoTools && len(tools) > 0 {
		var toolNames []string
		for _, t := range tools {
			toolNames = append(toolNames, unsanitizeToolName(t.Function.Name))
		}
		ctx.Sink.OnEvent("tool_info", fmt.Sprintf("[🔧 本次加载 %d 个工具]\n%s", len(tools), strings.Join(toolNames, ", ")))
	}

	// 注入 plan_and_execute（除非 NoTools）
	if !ctx.NoTools {
		tools = append(tools, planAndExecuteTool)
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

		log.Printf("[processTask] ── 迭代 %d/%d ── messages=%d tools=%d", i+1, maxIter, len(messages), len(tools))

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
			tcNames = append(tcNames, unsanitizeToolName(tc.Function.Name))
		}
		log.Printf("[processTask] ← LLM 响应 duration=%v textLen=%d toolCalls=%d tools=%v",
			llmDuration, len(text), len(toolCalls), tcNames)

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
			result := b.handleComplexTask(ctx, rootSession, store, tools, completedWork, selectedSkills)
			ctx.Sink.OnChunk(result)
			finalText = result
			complexTaskHandled = true
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

			originalName := unsanitizeToolName(tc.Function.Name)

			// set_persona 内置处理（不走远程 agent）
			if tc.Function.Name == "set_persona" && b.persona != nil {
				reply, ok := b.persona.HandleSetPersona(tc.Function.Arguments)
				log.Printf("[processTask] set_persona: success=%v result=%s", ok, reply)
				toolMsg := Message{
					Role:       "tool",
					Content:    reply,
					ToolCallID: tc.ID,
				}
				rootSession.AppendMessage(toolMsg)
				messages = append(messages, toolMsg)
				continue
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

			// 异步执行工具调用，主协程发送心跳进度
			type toolCallResultPair struct {
				result *ToolCallResult
				err    error
			}
			resultCh := make(chan toolCallResultPair, 1)
			go func() {
				r, e := b.CallToolCtxWithProgress(callCtx, originalName, json.RawMessage(tc.Function.Arguments), ctx.Sink)
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
						newToolNames = append(newToolNames, unsanitizeToolName(s.Function.Name))
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
	selectedSkills []SkillEntry, // Pass 1 预选的 skill（可为空）
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

	// 使用 Pass 1 预选的 skill 构建规划指引
	var skillBlock string
	if b.skillMgr != nil && len(selectedSkills) > 0 {
		skillBlock = b.skillMgr.BuildSkillBlock(selectedSkills)
	}

	planStart := time.Now()
	plan, err := PlanTask(&b.cfg.LLM, query, tools, ctx.Account, maxSubTasks, completedWork, skillBlock, b.cfg.Fallbacks, b.fallbackCooldown())
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
	review, err := ReviewPlan(&b.cfg.LLM, query, plan, tools, ctx.Account, agentCapabilities, b.cfg.Fallbacks, b.fallbackCooldown())
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
