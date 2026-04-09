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

// QueryLoopState 保存一次 query runtime 的核心状态。
type QueryLoopState struct {
	Query           string
	AllTools        []LLMTool
	PromptContext   SystemPromptContext
	Session         *QuerySession
	MaxIterations   int
	PermissionMode  PermissionMode
	FinalText       string
	FinalErr        error
	CompletedDirect bool
}

// QueryState 保留旧名称，兼容现有调用。
type QueryState = QueryLoopState

type QueryLoop struct {
	bridge      *Bridge
	task        *TaskContext
	store       *SessionStore
	rootSession *TaskSession
	trace       *RequestTrace
	localTools  map[string]ToolHandler
	toolExec    *ToolExecutionRuntime
	state       QueryLoopState
	taskStart   time.Time
}

// QueryRuntime 保留旧名称，兼容现有调用。
type QueryRuntime = QueryLoop

func (b *Bridge) processTask(ctx *TaskContext) (string, error) {
	rt, err := b.prepareQueryRuntime(ctx)
	if err != nil {
		return "", err
	}
	return rt.Run()
}

func (b *Bridge) prepareQueryRuntime(ctx *TaskContext) (*QueryLoop, error) {
	taskStart := time.Now()
	streaming := ctx.Sink.Streaming()
	log.Printf("[processTask] ▶ 开始处理 taskID=%s source=%s account=%s streaming=%v query=%s",
		ctx.TaskID, ctx.Source, ctx.Account, streaming, truncate(ctx.Query, 100))

	var tools []LLMTool
	var allTools []LLMTool
	if ctx.NoTools {
		log.Printf("[processTask] 工具模式: 禁用")
	} else if len(ctx.AllowedTools) > 0 {
		allTools = b.filterToolsByAllowlist(ctx.AllowedTools)
		log.Printf("[processTask] 工具模式: 系统约束 allow=%d matched=%d", len(ctx.AllowedTools), len(allTools))
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
			rootSession.Source = ctx.Source
			rootSession.AppendMessage(Message{Role: "assistant", Content: directReply})
			rootSession.SetResult(directReply)
			rootSession.SetStatus("done")
			store.Save(rootSession)
			store.SaveIndex(rootSession, nil)
			trace := NewRequestTrace(ctx.TaskID, ctx.Source, "root_query", query, rootSession)
			trace.SetDescription(query)
			trace.RecordPath("direct_reply", "命中 MCP 工具列表直答分支", nil)
			trace.RecordEvent("direct_reply", "直接返回工具目录", directReply, 0, nil)
			trace.Finish(rootSession.Status, directReply, nil)
			store.SaveRequestTrace(trace)
			log.Printf("[processTask] direct MCP tool list reply, toolCount=%d", len(allTools))
			return &QueryLoop{
				bridge:      b,
				task:        ctx,
				store:       store,
				rootSession: rootSession,
				trace:       trace,
				taskStart:   taskStart,
				state: QueryLoopState{
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
	enableToolPrompt := !ctx.NoTools
	if query != "" && isGreeting(query) {
		enableToolPrompt = false
	}
	promptContext := SystemPromptContext{
		Account: strings.TrimSpace(ctx.Account),
		Source:  strings.TrimSpace(ctx.Source),
	}
	if ctx.Messages != nil {
		messages = make([]Message, len(ctx.Messages))
		copy(messages, ctx.Messages)
		if ctx.Source == "web" || ctx.Source == "wechat" || ctx.Source == "app" {
			if len(messages) > 0 && messages[0].Role == "system" {
				freshPrompt, promptSections := b.buildAssistantSystemPromptForQuery(ctx.Account, query, enableToolPrompt)
				messages[0].Content = freshPrompt
				promptContext.Sections = clonePromptSections(promptSections)
				log.Printf("[processTask] 多轮续接：已刷新 system prompt promptLen=%d prompt:\n%s", len(freshPrompt), freshPrompt)
			}
		} else {
			log.Printf("[processTask] 使用预构建消息 count=%d", len(messages))
		}
		if len(messages) > 0 && messages[0].Role == "system" {
			promptContext.SystemPrompt = messages[0].Content
		}
	} else {
		systemPrompt, promptSections := b.buildAssistantSystemPromptForQuery(ctx.Account, query, enableToolPrompt)
		promptContext.SystemPrompt = systemPrompt
		promptContext.Sections = clonePromptSections(promptSections)
		messages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: ctx.Query},
		}
		log.Printf("[processTask] 构建系统提示 promptLen=%d prompt:\n%s", len(systemPrompt), systemPrompt)
	}

	store := NewSessionStore(b.cfg.SessionDir)
	var rootSession *TaskSession
	if ctx.ResumeSession != nil {
		rootSession = ctx.ResumeSession
		rootSession.mu.Lock()
		rootSession.Title = query
		rootSession.Account = ctx.Account
		rootSession.Source = ctx.Source
		rootSession.Messages = nil
		rootSession.mu.Unlock()
		rootSession.SetStatus("running")
	} else {
		rootSession = NewRootSession(ctx.TaskID, query, ctx.Account)
		rootSession.Source = ctx.Source
	}
	for _, msg := range messages {
		rootSession.AppendMessage(msg)
	}
	ctx.CurrentSession = rootSession

	trace := NewRequestTrace(ctx.TaskID, ctx.Source, "root_query", query, rootSession)
	trace.SetDescription(query)
	trace.RecordPath("task_start", "进入根任务 QueryLoop", map[string]string{
		"source":   fallbackText(strings.TrimSpace(ctx.Source), "unknown"),
		"no_tools": fmt.Sprintf("%t", ctx.NoTools),
	})
	trace.RecordPath("prompt_ready", fmt.Sprintf("构建初始消息 %d 条", len(messages)), map[string]string{
		"system_prompt_len": fmt.Sprintf("%d", len(promptContext.SystemPrompt)),
	})
	ctx.Trace = trace
	b.hooks.FireTaskStart(ctx)

	toolView := b.buildRootToolRuntimeView(ctx, query, allTools)
	trace.SetToolView(toolView)
	trace.RecordPath("tool_view_ready", fmt.Sprintf("policy=%s visible=%d all=%d", toolView.Policy, len(toolView.VisibleTools), len(toolView.AllTools)), map[string]string{
		"matched_skills": strings.Join(toolView.MatchedSkills, ","),
	})
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

	if ctx.ResumeSnapshot != nil {
		if strings.TrimSpace(ctx.ResumeSnapshot.Query) != "" {
			query = strings.TrimSpace(ctx.ResumeSnapshot.Query)
		}
		if strings.TrimSpace(ctx.ResumeSnapshot.PromptContext.SystemPrompt) != "" {
			promptContext = ctx.ResumeSnapshot.PromptContext
		}
		trace.RecordPath("resume_snapshot_loaded", "恢复已有 runtime snapshot", map[string]string{
			"attachments":     fmt.Sprintf("%d", len(ctx.ResumeSnapshot.Attachments)),
			"compact_history": fmt.Sprintf("%d", len(ctx.ResumeSnapshot.CompactHistory)),
		})
	}

	sessionRT := NewQuerySession(messages, toolView, defaultQueryCompactConfig())
	if ctx.ResumeSnapshot != nil {
		sessionRT = NewQuerySessionFromSnapshot(messages, ctx.ResumeSnapshot, toolView, defaultQueryCompactConfig())
	}

	queryLoop := &QueryLoop{
		bridge:      b,
		task:        ctx,
		store:       store,
		rootSession: rootSession,
		trace:       trace,
		localTools:  b.buildRuntimeLocalHandlers(ctx, allTools),
		taskStart:   taskStart,
		state: QueryLoopState{
			Query:          query,
			AllTools:       allTools,
			PromptContext:  promptContext,
			Session:        sessionRT,
			MaxIterations:  maxIter,
			PermissionMode: PermissionModeDefault,
		},
		toolExec: NewToolExecutionRuntime(b),
	}
	queryLoop.saveCheckpoint("running")
	return queryLoop, nil
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

	return localHandlers
}

func (rt *QueryLoop) queryLoop() (string, error) {
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

		rt.drainMailboxAttachments(i + 1)

		if meta := rt.state.Session.CompactIfNeeded(i, "query_turn"); meta != nil {
			log.Printf("[processTask] 消息压缩 reason=%s messages=%d→%d chars=%d→%d toolTrim=%d",
				meta.Reason, meta.BeforeMessages, meta.AfterMessages, meta.BeforeChars, meta.AfterChars, meta.ToolResultsTrimed)
			if rt.trace != nil {
				rt.trace.RecordRoundCompaction(i+1, meta)
			}
			rt.saveCheckpoint("running")
		} else {
			rt.saveRuntimeSnapshot("running")
		}

		log.Printf("[processTask] ── 迭代 %d/%d ── messages=%d tools=%d", i+1, rt.state.MaxIterations, len(rt.state.Session.Messages()), len(rt.state.Session.VisibleTools()))

		if i == rt.state.MaxIterations-1 {
			rt.state.Session.DisableTools()
			rt.state.Session.AppendMessage(RuntimeMessage{
				Role:    "user",
				Content: "你已经进行了多轮工具调用。请立即给出最终回复，总结目前已完成的工作和结果。不要再调用任何工具。",
			}.ToMessage(), rt.rootSession)
			rt.saveCheckpoint("running")
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

		var currentRound *TraceRound
		if rt.trace != nil {
			rt.trace.RecordRoundLLM(i+1, llmDuration, text, toolCalls, rt.state.Session.VisibleTools())
			currentRound = rt.trace.EnsureRound(i + 1)
			rt.trace.RecordPath(fmt.Sprintf("round_%d_llm", i+1),
				fmt.Sprintf("LLM返回 text_len=%d tool_calls=%d", len(text), len(toolCalls)),
				map[string]string{"tools": strings.Join(tcNames, ",")})
		} else {
			currentRound = &TraceRound{Index: i + 1}
		}

		if len(toolCalls) > 0 {
			rt.task.Sink.OnEvent("thinking", fmt.Sprintf("LLM 响应完成 (%s)，需要调用 %d 个工具: %s", fmtDuration(llmDuration), len(toolCalls), strings.Join(tcNames, ", ")))
		} else {
			rt.task.Sink.OnEvent("thinking", fmt.Sprintf("LLM 响应完成 (%s)，正在整理结果...", fmtDuration(llmDuration)))
		}

		assistantMsg := RuntimeMessage{Role: "assistant", Content: text, ToolCalls: toolCalls}.ToMessage()
		rt.state.Session.AppendMessage(assistantMsg, rt.rootSession)
		rt.saveCheckpoint("running")

		if len(toolCalls) == 0 {
			log.Printf("[processTask] ✓ 对话结束（无工具调用） resultLen=%d", len(text))
			rt.state.FinalText = text
			rt.rootSession.SetResult(text)
			rt.saveCheckpoint("running")
			rt.task.Sink.OnEvent("task_complete", fmt.Sprintf("处理完成，耗时 %s", fmtDuration(time.Since(rt.taskStart))))
			break
		}

		if rt.task.NoTools {
			log.Printf("[processTask] ✓ 忽略工具调用（NoTools模式） resultLen=%d", len(text))
			rt.state.FinalText = text
			rt.rootSession.SetResult(text)
			rt.saveCheckpoint("running")
			break
		}

		bizFailedTools, bizFailedMsgs, interrupted := rt.executeToolCalls(i, toolCalls, currentRound)
		if interrupted {
			break
		}

		if len(bizFailedTools) > 0 {
			rt.expandSiblingTools(bizFailedTools, bizFailedMsgs)
		}
	}

	return rt.finish()
}

func (rt *QueryLoop) Run() (string, error) {
	return rt.queryLoop()
}

func (rt *QueryLoop) callLLM() (string, []ToolCall, time.Duration, error) {
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

func (rt *QueryLoop) executeToolCalls(iteration int, toolCalls []ToolCall, currentRound *TraceRound) ([]string, []string, bool) {
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
			Trace:          rt.trace,
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
		rt.saveCheckpoint("running")
	}

	return bizFailedTools, bizFailedMsgs, false
}

func (rt *QueryLoop) expandSiblingTools(bizFailedTools, bizFailedMsgs []string) {
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
	attachment := newRuntimeAttachment(
		RuntimeAttachmentSystemHint,
		"工具业务失败恢复",
		hint,
		rt.rootSession.ID,
		map[string]string{"expanded_tools": strings.Join(newToolNames, ",")},
	)
	rt.state.Session.InjectAttachments([]Attachment{attachment}, rt.rootSession)
	rt.saveCheckpoint("running")
	if rt.trace != nil {
		rt.trace.RecordPath("tool_expand", fmt.Sprintf("业务失败后扩展兄弟工具: %s", strings.Join(newToolNames, ", ")), map[string]string{
			"failed_tools": strings.Join(bizFailedTools, ","),
		})
	}
	log.Printf("[processTask] 业务失败扩展: 新增 %d 个兄弟工具: %v", len(newToolNames), newToolNames)
	rt.task.Sink.OnEvent("tool_expand", fmt.Sprintf("工具业务失败，补充兄弟工具: %s", strings.Join(newToolNames, ", ")))
}

func (rt *QueryLoop) finish() (string, error) {
	if rt.state.FinalErr != nil {
		rt.rootSession.SetStatus("failed")
		rt.rootSession.SetError(rt.state.FinalErr.Error())
	} else if rt.rootSession.Status != "cancelled" {
		rt.rootSession.SetStatus("done")
	}

	if rt.state.FinalText == "" && rt.state.FinalErr != nil {
		rt.state.FinalText = fmt.Sprintf("抱歉，AI 服务暂时不可用: %v", rt.state.FinalErr)
	}
	if rt.state.FinalText == "" {
		rt.state.FinalText = "抱歉，未能生成回复。"
	}

	assistantRecord := buildPersistentAssistantRecord(AssistantRecordInput{
		Query:         rt.state.Query,
		DisplayResult: rt.state.FinalText,
		Status:        rt.rootSession.Status,
		RootSession:   rt.rootSession,
		FinalErr:      rt.state.FinalErr,
	})
	if strings.HasPrefix(strings.TrimSpace(assistantRecord), assistantRecordHeader) {
		rt.task.PersistedAssistant = assistantRecord
		appendFinalAssistantRecord(rt.rootSession, assistantRecord)
	} else {
		rt.task.PersistedAssistant = rt.state.FinalText
	}

	rt.store.Save(rt.rootSession)
	rt.store.SaveIndex(rt.rootSession, nil)
	rt.saveRuntimeSnapshot(rt.rootSession.Status)

	totalDuration := time.Since(rt.taskStart)
	status := "done"
	if rt.state.FinalErr != nil {
		status = "failed"
	}
	log.Printf("[processTask] ◀ 处理完成 taskID=%s source=%s status=%s duration=%v resultLen=%d",
		rt.task.TaskID, rt.task.Source, status, totalDuration, len(rt.state.FinalText))

	if rt.trace != nil {
		rt.trace.RecordPath("task_finish", fmt.Sprintf("status=%s result_len=%d", rt.rootSession.Status, len(rt.state.FinalText)), nil)
		rt.trace.Finish(rt.rootSession.Status, rt.state.FinalText, rt.state.FinalErr)
		if err := rt.store.SaveRequestTrace(rt.trace); err != nil {
			log.Printf("[processTask] warn: save trace failed session=%s err=%v", rt.rootSession.ID, err)
		}
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

func (rt *QueryLoop) saveCheckpoint(status string) {
	if rt.store == nil || rt.rootSession == nil || rt.state.Session == nil {
		return
	}
	if rt.trace != nil {
		rt.trace.RefreshFromSession(rt.rootSession)
	}
	if err := rt.store.Save(rt.rootSession); err != nil {
		log.Printf("[processTask] warn: save checkpoint session failed session=%s err=%v", rt.rootSession.ID, err)
	}
	rt.saveRuntimeSnapshot(status)
	if rt.trace != nil {
		if err := rt.store.SaveRequestTrace(rt.trace); err != nil {
			log.Printf("[processTask] warn: save trace failed session=%s err=%v", rt.rootSession.ID, err)
		}
	}
}

func (rt *QueryLoop) drainMailboxAttachments(roundIndex int) {
	if rt.store == nil || rt.rootSession == nil || rt.state.Session == nil {
		return
	}
	msgs, err := rt.store.DrainMailbox(rt.rootSession.RootID, rt.rootSession.ID)
	if err != nil {
		log.Printf("[processTask] warn: drain mailbox failed root=%s session=%s err=%v", rt.rootSession.RootID, rt.rootSession.ID, err)
		return
	}
	attachments := attachmentsFromMailbox(msgs)
	if len(attachments) == 0 {
		return
	}
	injected := rt.state.Session.InjectAttachments(attachments, rt.rootSession)
	if rt.trace != nil {
		rt.trace.RecordRoundMailbox(roundIndex, injected, attachments)
		rt.trace.RecordPath("mailbox_injected", fmt.Sprintf("注入 %d 条运行时上下文", injected), map[string]string{
			"attachments": traceAttachmentKinds(attachments),
		})
	}
	log.Printf("[processTask] injected runtime attachments session=%s count=%d", rt.rootSession.ID, injected)
	rt.task.Sink.OnEvent("runtime_context", fmt.Sprintf("注入 %d 条运行时上下文", injected))
	rt.saveCheckpoint("running")
}

func (rt *QueryLoop) injectMailboxAttachments() {
	rt.drainMailboxAttachments(currentRoundIndex(rt.trace))
}

func (rt *QueryLoop) saveRuntimeSnapshot(status string) {
	if rt.store == nil || rt.rootSession == nil || rt.state.Session == nil {
		return
	}
	if strings.TrimSpace(status) == "" {
		status = rt.rootSession.Status
	}
	snapshot := rt.state.Session.Snapshot(rt.rootSession.RootID, rt.rootSession.ID, rt.state.Query, status, rt.state.PromptContext)
	if err := rt.store.SaveRuntimeSnapshot(snapshot); err != nil {
		log.Printf("[processTask] warn: save runtime state failed session=%s err=%v", rt.rootSession.ID, err)
	}
}

func (rt *QueryLoop) persistRuntimeState(status string) {
	rt.saveRuntimeSnapshot(status)
}
