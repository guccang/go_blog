package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// PermissionMode 对齐 Claude Code 风格的运行时权限模式。
type PermissionMode string

const (
	PermissionModeDefault PermissionMode = "default"
	PermissionModePlan    PermissionMode = "plan"
	PermissionModeAuto    PermissionMode = "auto"
	PermissionModeBypass  PermissionMode = "bypass"
)

// PermissionAction 表示一次权限判定的结果。
type PermissionAction string

const (
	PermissionActionAllow PermissionAction = "allow"
	PermissionActionDeny  PermissionAction = "deny"
	PermissionActionAsk   PermissionAction = "ask"
)

// PermissionDecision 为后续接入 ask/deny runtime 预留结构化结果。
type PermissionDecision struct {
	Action       PermissionAction `json:"action"`
	Reason       string           `json:"reason"`
	UpdatedInput json.RawMessage  `json:"updated_input,omitempty"`
}

// RuntimeMessage 统一 runtime 内部消息视图，当前先复用到现有 Message 映射。
type RuntimeMessage struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// ToMessage 转回现有 LLM 消息结构。
func (m RuntimeMessage) ToMessage() Message {
	return Message{
		Role:       m.Role,
		Content:    m.Content,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
	}
}

// QueryState 保存一次 query runtime 的核心状态。
type QueryState struct {
	Query           string
	AllTools        []LLMTool
	Session         *SessionRuntime
	MaxIterations   int
	PermissionMode  PermissionMode
	FinalText       string
	FinalErr        error
	CompletedDirect bool
	ComplexHandled  bool
}

type QueryRuntime struct {
	bridge      *Bridge
	task        *TaskContext
	store       *SessionStore
	rootSession *TaskSession
	trace       *RequestTrace
	localTools  map[string]ToolHandler
	toolExec    *ToolExecutionRuntime
	state       QueryState
	taskStart   time.Time
}

func (b *Bridge) processTask(ctx *TaskContext) (string, error) {
	rt, err := b.prepareQueryRuntime(ctx)
	if err != nil {
		return "", err
	}
	return rt.Run()
}

func (b *Bridge) prepareQueryRuntime(ctx *TaskContext) (*QueryRuntime, error) {
	taskStart := time.Now()
	streaming := ctx.Sink.Streaming()
	log.Printf("[processTask] ▶ 开始处理 taskID=%s source=%s account=%s streaming=%v query=%s",
		ctx.TaskID, ctx.Source, ctx.Account, streaming, truncate(ctx.Query, 100))

	var tools []LLMTool
	var allTools []LLMTool
	if ctx.NoTools {
		log.Printf("[processTask] 工具模式: 禁用")
	} else if len(ctx.SelectedTools) > 0 {
		allTools = b.filterToolsBySelection(ctx.SelectedTools)
		log.Printf("[processTask] 工具模式: 用户选择 selected=%d matched=%d", len(ctx.SelectedTools), len(allTools))
	} else {
		allTools = b.getLLMTools()
		log.Printf("[processTask] 工具模式: skill+agent（%d 个 agent 工具 + 虚拟工具）", len(allTools))
	}

	query := ctx.Query
	if query == "" && ctx.Messages != nil {
		for i := len(ctx.Messages) - 1; i >= 0; i-- {
			if ctx.Messages[i].Role == "user" {
				query = ctx.Messages[i].Content
				break
			}
		}
	}

	if !ctx.NoTools && query != "" && isGreeting(query) {
		log.Printf("[processTask] 闲聊检测命中，禁用工具 query=%s", truncate(query, 50))
	}

	if !ctx.NoTools {
		if directReply, ok := b.buildMCPToolListReply(query, b.getLLMTools()); ok {
			store := NewSessionStore(b.cfg.SessionDir)
			rootSession := NewRootSession(ctx.TaskID, query, ctx.Account)
			rootSession.AppendMessage(Message{Role: "assistant", Content: directReply})
			rootSession.SetResult(directReply)
			rootSession.SetStatus("done")
			store.Save(rootSession)
			store.SaveIndex(rootSession, nil)
			log.Printf("[processTask] direct MCP tool list reply, toolCount=%d", len(allTools))
			return &QueryRuntime{
				bridge:      b,
				task:        ctx,
				store:       store,
				rootSession: rootSession,
				taskStart:   taskStart,
				state: QueryState{
					Query:           query,
					AllTools:        allTools,
					FinalText:       directReply,
					PermissionMode:  PermissionModeDefault,
					CompletedDirect: true,
				},
			}, nil
		}
	}

	var messages []Message
	if ctx.Messages != nil {
		messages = make([]Message, len(ctx.Messages))
		copy(messages, ctx.Messages)
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
		systemPrompt, _ := b.buildAssistantSystemPrompt(ctx.Account)
		messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: ctx.Query},
		}
		log.Printf("[processTask] 构建系统提示 promptLen=%d prompt:\n%s", len(systemPrompt), systemPrompt)
	}

	store := NewSessionStore(b.cfg.SessionDir)
	rootSession := NewRootSession(ctx.TaskID, query, ctx.Account)
	for _, msg := range messages {
		rootSession.AppendMessage(msg)
	}

	trace := &RequestTrace{
		TaskID:    ctx.TaskID,
		Source:    ctx.Source,
		Query:     query,
		StartTime: taskStart,
	}
	ctx.Trace = trace
	b.hooks.FireTaskStart(ctx)

	toolView := b.buildRootToolRuntimeView(ctx, query, allTools)
	tools = toolView.Visible()
	if !ctx.NoTools && len(tools) > 0 {
		skillCount := 0
		if b.skillMgr != nil {
			skillCount = len(b.skillMgr.GetAvailableSkills())
		}
		ctx.Sink.OnEvent("tool_info", fmt.Sprintf("[🔧 加载 %d 个工具]\n  %d 个技能, %d 个 agent 工具, %d 个虚拟工具",
			len(tools), skillCount, len(allTools), len(tools)-len(allTools)))
	}

	maxIter := b.cfg.MaxToolIterations
	if maxIter <= 0 {
		maxIter = 15
	}

	return &QueryRuntime{
		bridge:      b,
		task:        ctx,
		store:       store,
		rootSession: rootSession,
		trace:       trace,
		localTools:  b.buildRuntimeLocalHandlers(ctx, allTools),
		taskStart:   taskStart,
		state: QueryState{
			Query:          query,
			AllTools:       allTools,
			Session:        NewSessionRuntime(messages, toolView, defaultQueryCompactConfig()),
			MaxIterations:  maxIter,
			PermissionMode: PermissionModeDefault,
		},
		toolExec: NewToolExecutionRuntime(b),
	}, nil
}

func (b *Bridge) buildRuntimeLocalHandlers(ctx *TaskContext, allTools []LLMTool) map[string]ToolHandler {
	localHandlers := make(map[string]ToolHandler)
	if ctx.NoTools {
		return localHandlers
	}

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
		if offline := b.skillMgr.offlineAgents(skill); len(offline) > 0 {
			return &ToolCallResult{
				Result:  fmt.Sprintf("技能 '%s' 当前不可用：所需 agent %s offline。请告知用户该功能暂时无法使用。", a.SkillName, strings.Join(offline, ", ")),
				AgentID: "builtin",
			}, nil
		}
		detail := b.skillMgr.BuildSkillBlock([]SkillEntry{*skill})
		return &ToolCallResult{Result: detail, AgentID: "builtin"}, nil
	}

	localHandlers["get_tool_detail"] = func(callCtx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		var a struct {
			ToolName string `json:"tool_name"`
			AgentID  string `json:"agent_id"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return &ToolCallResult{Result: fmt.Sprintf("参数解析失败: %v", err), AgentID: "builtin"}, nil
		}
		return b.handleGetToolDetail(a.ToolName, a.AgentID), nil
	}

	localHandlers["get_agent_detail"] = func(callCtx context.Context, args json.RawMessage, sink EventSink) (*ToolCallResult, error) {
		var a struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return &ToolCallResult{Result: fmt.Sprintf("参数解析失败: %v", err), AgentID: "builtin"}, nil
		}
		return b.handleGetAgentDetail(a.AgentID), nil
	}

	if b.skillMgr != nil && len(b.skillMgr.GetAllSkills()) > 0 {
		capturedCtx := ctx
		capturedTools := allTools
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

	return localHandlers
}

func (rt *QueryRuntime) Run() (string, error) {
	if rt.state.CompletedDirect {
		return rt.state.FinalText, rt.state.FinalErr
	}

	for i := 0; i < rt.state.MaxIterations; i++ {
		if rt.task.Ctx != nil && rt.task.Ctx.Err() != nil {
			log.Printf("[processTask] ✗ 任务被取消 taskID=%s", rt.task.TaskID)
			rt.state.FinalText = "任务已停止。"
			rt.rootSession.SetStatus("cancelled")
			rt.task.Sink.OnEvent("task_cancelled", "任务已被用户停止")
			break
		}

		if meta := rt.state.Session.CompactIfNeeded(i, "query_turn"); meta != nil {
			log.Printf("[processTask] 消息压缩 reason=%s messages=%d→%d chars=%d→%d toolTrim=%d",
				meta.Reason, meta.BeforeMessages, meta.AfterMessages, meta.BeforeChars, meta.AfterChars, meta.ToolResultsTrimed)
		}

		log.Printf("[processTask] ── 迭代 %d/%d ── messages=%d tools=%d", i+1, rt.state.MaxIterations, len(rt.state.Session.Messages()), len(rt.state.Session.VisibleTools()))

		if i == rt.state.MaxIterations-1 {
			rt.state.Session.DisableTools()
			rt.state.Session.AppendMessage(RuntimeMessage{
				Role:    "user",
				Content: "你已经进行了多轮工具调用。请立即给出最终回复，总结目前已完成的工作和结果。不要再调用任何工具。",
			}.ToMessage(), nil)
			rt.task.Sink.OnEvent("task_forced_summary", fmt.Sprintf("已执行 %d 轮工具调用，正在强制总结结果...", i))
			log.Printf("[processTask] ⚠ 达到迭代上限，移除工具强制总结")
		}

		if i == 0 {
			rt.task.Sink.OnEvent("thinking", "正在思考...")
		} else {
			rt.task.Sink.OnEvent("thinking", fmt.Sprintf("正在分析工具结果（第%d轮）...", i+1))
		}

		text, toolCalls, llmDuration, err := rt.callLLM()
		if err != nil {
			log.Printf("[processTask] ✗ LLM 请求失败 duration=%v error=%v", llmDuration, err)
			rt.state.FinalErr = err
			rt.task.Sink.OnChunk(fmt.Sprintf("\n\n抱歉，AI 服务暂时不可用: %v", err))
			break
		}

		var tcNames []string
		for _, tc := range toolCalls {
			tcNames = append(tcNames, rt.bridge.resolveToolName(tc.Function.Name))
		}
		log.Printf("[processTask] ← LLM 响应 duration=%v textLen=%d toolCalls=%d tools=%v",
			llmDuration, len(text), len(toolCalls), tcNames)

		currentRound := TraceRound{
			Index:         i + 1,
			LLMDurationMs: llmDuration.Milliseconds(),
			TextLen:       len(text),
		}

		if len(toolCalls) > 0 {
			rt.task.Sink.OnEvent("thinking", fmt.Sprintf("LLM 响应完成 (%s)，需要调用 %d 个工具: %s", fmtDuration(llmDuration), len(toolCalls), strings.Join(tcNames, ", ")))
		} else {
			rt.task.Sink.OnEvent("thinking", fmt.Sprintf("LLM 响应完成 (%s)，正在整理结果...", fmtDuration(llmDuration)))
		}

		assistantMsg := RuntimeMessage{Role: "assistant", Content: text, ToolCalls: toolCalls}.ToMessage()
		rt.rootSession.AppendMessage(assistantMsg)

		if len(toolCalls) == 0 {
			log.Printf("[processTask] ✓ 对话结束（无工具调用） resultLen=%d", len(text))
			rt.trace.Rounds = append(rt.trace.Rounds, currentRound)
			rt.state.FinalText = text
			rt.rootSession.SetResult(text)
			rt.task.Sink.OnEvent("task_complete", fmt.Sprintf("处理完成，耗时 %s", fmtDuration(time.Since(rt.taskStart))))
			break
		}

		if rt.task.NoTools {
			log.Printf("[processTask] ✓ 忽略工具调用（NoTools模式） resultLen=%d", len(text))
			rt.state.FinalText = text
			rt.rootSession.SetResult(text)
			break
		}

		planCallIdx := -1
		for idx, tc := range toolCalls {
			if tc.Function.Name == "plan_and_execute" {
				planCallIdx = idx
				break
			}
		}
		if planCallIdx >= 0 {
			var args struct {
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal([]byte(toolCalls[planCallIdx].Function.Arguments), &args); err == nil {
				log.Printf("[processTask] plan_and_execute triggered at iteration %d: %s", i, args.Reasoning)
			} else {
				log.Printf("[processTask] plan_and_execute triggered at iteration %d", i)
			}

			var completedWork string
			rt.rootSession.mu.Lock()
			existingCalls := make([]ToolCallRecord, len(rt.rootSession.ToolCalls))
			copy(existingCalls, rt.rootSession.ToolCalls)
			rt.rootSession.mu.Unlock()
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

			result := rt.bridge.handleComplexTask(rt.task, rt.rootSession, rt.store, rt.state.AllTools, completedWork)
			rt.task.Sink.OnChunk(result)
			rt.state.FinalText = result
			rt.state.ComplexHandled = true
			rt.trace.Rounds = append(rt.trace.Rounds, currentRound)
			break
		}

		msgText := text
		if len(toolCalls) > 0 {
			msgText = ""
		}
		rt.state.Session.AppendMessage(RuntimeMessage{
			Role:      "assistant",
			Content:   msgText,
			ToolCalls: toolCalls,
		}.ToMessage(), nil)

		bizFailedTools, bizFailedMsgs, interrupted := rt.executeToolCalls(i, toolCalls, &currentRound)
		if interrupted {
			break
		}

		if len(bizFailedTools) > 0 {
			rt.expandSiblingTools(bizFailedTools, bizFailedMsgs)
		}

		rt.trace.Rounds = append(rt.trace.Rounds, currentRound)
	}

	return rt.finish()
}

func (rt *QueryRuntime) callLLM() (string, []ToolCall, time.Duration, error) {
	llmCfg, llmFallbacks := rt.bridge.GetLLMConfigForSource(rt.task.Source)
	llmStart := time.Now()
	var (
		text      string
		toolCalls []ToolCall
		err       error
	)
	if rt.task.Sink.Streaming() {
		log.Printf("[processTask] → 发送流式 LLM 请求...")
		text, toolCalls, err = rt.bridge.sendStreamingLLMWithConfig(llmCfg, llmFallbacks, rt.state.Session.Messages(), rt.state.Session.VisibleTools(), func(chunk string) {
			rt.task.Sink.OnChunk(chunk)
		})
	} else {
		log.Printf("[processTask] → 发送同步 LLM 请求...")
		text, toolCalls, err = rt.bridge.sendLLMWithConfig(llmCfg, llmFallbacks, rt.state.Session.Messages(), rt.state.Session.VisibleTools())
	}
	return text, toolCalls, time.Since(llmStart), err
}

func (rt *QueryRuntime) executeToolCalls(iteration int, toolCalls []ToolCall, currentRound *TraceRound) ([]string, []string, bool) {
	var bizFailedTools []string
	var bizFailedMsgs []string

	for tcIdx, tc := range toolCalls {
		if rt.task.Ctx != nil && rt.task.Ctx.Err() != nil {
			log.Printf("[processTask] ✗ 工具调用期间任务被取消 taskID=%s", rt.task.TaskID)
			rt.state.FinalText = "任务已停止。"
			rt.rootSession.SetStatus("cancelled")
			rt.task.Sink.OnEvent("task_cancelled", "任务已被用户停止")
			return bizFailedTools, bizFailedMsgs, true
		}

		execResult := rt.toolExec.Execute(ToolExecutionCall{
			Mode:           ToolExecutionModeQuery,
			Ctx:            rt.task.Ctx,
			Task:           rt.task,
			TaskID:         rt.task.TaskID,
			Account:        rt.task.Account,
			Source:         rt.task.Source,
			Session:        rt.rootSession,
			AvailableTools: rt.state.AllTools,
			LocalHandlers:  rt.localTools,
			Sink:           rt.task.Sink,
			Iteration:      iteration,
			Index:          tcIdx,
			Total:          len(toolCalls),
			ToolCall:       tc,
			TraceRound:     currentRound,
		})
		if execResult.Interrupted {
			log.Printf("[processTask] ✗ 工具调用期间任务被取消 tool=%s taskID=%s", execResult.ToolName, rt.task.TaskID)
			rt.state.FinalText = "任务已停止。"
			rt.rootSession.SetStatus("cancelled")
			rt.task.Sink.OnEvent("task_cancelled", "任务已被用户停止")
			return bizFailedTools, bizFailedMsgs, true
		}
		if execResult.BusinessErr != "" {
			bizFailedTools = append(bizFailedTools, execResult.ToolName)
			bizFailedMsgs = append(bizFailedMsgs, execResult.BusinessErr)
			log.Printf("[processTask] 业务失败检测: %s → %s", execResult.ToolName, execResult.BusinessErr)
		}
		rt.state.Session.AppendToolResult(execResult.Result, tc.ID, iteration, rt.rootSession)
	}

	return bizFailedTools, bizFailedMsgs, false
}

func (rt *QueryRuntime) expandSiblingTools(bizFailedTools, bizFailedMsgs []string) {
	newToolNames := rt.state.Session.ExpandSiblingTools(rt.bridge, bizFailedTools)

	if len(newToolNames) == 0 {
		return
	}

	var failInfo strings.Builder
	for idx, name := range bizFailedTools {
		failInfo.WriteString(fmt.Sprintf("- %s: %s\n", name, bizFailedMsgs[idx]))
	}
	hint := fmt.Sprintf("以下工具返回业务失败:\n%s已补充同 Agent 的替代工具: %s\n你可以选择修复参数重试原工具，或使用替代工具完成任务。",
		failInfo.String(), strings.Join(newToolNames, ", "))
	rt.state.Session.AppendMessage(RuntimeMessage{Role: "user", Content: hint}.ToMessage(), nil)
	log.Printf("[processTask] 业务失败扩展: 新增 %d 个兄弟工具: %v", len(newToolNames), newToolNames)
	rt.task.Sink.OnEvent("tool_expand", fmt.Sprintf("工具业务失败，补充兄弟工具: %s", strings.Join(newToolNames, ", ")))
}

func (rt *QueryRuntime) finish() (string, error) {
	if !rt.state.ComplexHandled {
		if rt.state.FinalErr != nil {
			rt.rootSession.SetStatus("failed")
			rt.rootSession.SetError(rt.state.FinalErr.Error())
		} else if rt.rootSession.Status != "cancelled" {
			rt.rootSession.SetStatus("done")
		}
		rt.store.Save(rt.rootSession)
		rt.store.SaveIndex(rt.rootSession, nil)
	}

	if rt.state.FinalText == "" && rt.state.FinalErr != nil {
		rt.state.FinalText = fmt.Sprintf("抱歉，AI 服务暂时不可用: %v", rt.state.FinalErr)
	}
	if rt.state.FinalText == "" {
		rt.state.FinalText = "抱歉，未能生成回复。"
	}

	totalDuration := time.Since(rt.taskStart)
	status := "done"
	if rt.state.FinalErr != nil {
		status = "failed"
	}
	log.Printf("[processTask] ◀ 处理完成 taskID=%s source=%s status=%s duration=%v resultLen=%d",
		rt.task.TaskID, rt.task.Source, status, totalDuration, len(rt.state.FinalText))

	if rt.trace != nil {
		if traceSummary := rt.trace.Summary(); traceSummary != "" {
			log.Print(traceSummary)
		}
	}

	rt.rootSession.mu.Lock()
	allToolCalls := make([]ToolCallRecord, len(rt.rootSession.ToolCalls))
	copy(allToolCalls, rt.rootSession.ToolCalls)
	rt.rootSession.mu.Unlock()

	rt.bridge.hooks.FireTaskEnd(rt.task, rt.state.FinalText, allToolCalls, rt.state.FinalErr)
	if rt.bridge.memoryCollector != nil && len(allToolCalls) > 0 {
		go rt.bridge.memoryCollector.CollectAfterTask(allToolCalls)
	}

	return rt.state.FinalText, rt.state.FinalErr
}
