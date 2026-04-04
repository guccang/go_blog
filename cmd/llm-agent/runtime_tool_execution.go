package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type functionEventSink struct {
	sendEvent func(event, text string)
}

func (s *functionEventSink) OnChunk(text string) {}
func (s *functionEventSink) OnEvent(event, text string) {
	if s.sendEvent != nil {
		s.sendEvent(event, text)
	}
}
func (s *functionEventSink) Streaming() bool { return false }

type ToolExecutionMode string

const (
	ToolExecutionModeQuery   ToolExecutionMode = "query"
	ToolExecutionModeSubtask ToolExecutionMode = "subtask"
)

type ToolExecutionCall struct {
	Mode           ToolExecutionMode
	Ctx            context.Context
	Task           *TaskContext
	TaskID         string
	Account        string
	Source         string
	Session        *TaskSession
	AvailableTools []LLMTool
	LocalHandlers  map[string]ToolHandler
	Sink           EventSink
	ScopeID        string
	Iteration      int
	Index          int
	Total          int
	ToolCall       ToolCall
	TraceRound     *TraceRound
}

type ToolExecutionResult struct {
	ToolName    string
	Result      string
	BusinessErr string
	Interrupted bool
	FatalErr    error
}

type ToolExecutionRuntime struct {
	bridge *Bridge
}

func NewToolExecutionRuntime(bridge *Bridge) *ToolExecutionRuntime {
	return &ToolExecutionRuntime{bridge: bridge}
}

func (rt *ToolExecutionRuntime) Execute(call ToolExecutionCall) ToolExecutionResult {
	originalName := rt.bridge.resolveToolName(call.ToolCall.Function.Name)
	toolCallEvent := rt.formatToolCallEvent(call.Mode, call.ScopeID, originalName, call.ToolCall.Function.Arguments, call.Index, call.Total)
	call.Sink.OnEvent("tool_call", toolCallEvent)
	log.Printf("[ToolExecution] mode=%s scope=%s → 调用工具: %s args=%s", call.Mode, call.ScopeID, originalName, call.ToolCall.Function.Arguments)
	startedAt := time.Now()

	callCtx := call.Ctx
	if callCtx == nil {
		callCtx = context.Background()
	}

	tcResult, err, found := rt.dispatch(call, callCtx, originalName)
	if !found {
		result := fmt.Sprintf("工具 %s 未找到", originalName)
		call.Sink.OnEvent("tool_result", result)
		return rt.finish(call, originalName, result, nil, fmt.Errorf("tool not found: %s", originalName), time.Since(startedAt))
	}

	var result string
	if tcResult != nil {
		result = tcResult.Result
	}
	if err != nil && call.Ctx != nil && call.Ctx.Err() != nil {
		log.Printf("[ToolExecution] mode=%s scope=%s 工具调用中断: %s err=%v", call.Mode, call.ScopeID, originalName, call.Ctx.Err())
		return ToolExecutionResult{
			ToolName:    originalName,
			Result:      result,
			Interrupted: true,
			FatalErr:    call.Ctx.Err(),
		}
	}

	return rt.finish(call, originalName, result, tcResult, err, time.Since(startedAt))
}

func (rt *ToolExecutionRuntime) dispatch(call ToolExecutionCall, callCtx context.Context, originalName string) (*ToolCallResult, error, bool) {
	if handler, ok := call.LocalHandlers[originalName]; ok {
		return rt.dispatchLocal(callCtx, originalName, call.ToolCall.Function.Arguments, handler, call.Sink)
	}

	if originalName == "execute_skill" {
		return rt.dispatchSkill(call, callCtx)
	}

	progressSink := call.Sink
	result, err := rt.bridge.DispatchTool(callCtx, originalName, json.RawMessage(call.ToolCall.Function.Arguments), progressSink)
	if err == nil {
		return result, nil, true
	}
	result, err = rt.bridge.CallToolCtxWithProgress(callCtx, originalName, json.RawMessage(call.ToolCall.Function.Arguments), progressSink)
	return result, err, true
}

func (rt *ToolExecutionRuntime) dispatchLocal(callCtx context.Context, originalName, args string, handler ToolHandler, sink EventSink) (*ToolCallResult, error, bool) {
	type toolCallResultPair struct {
		result *ToolCallResult
		err    error
	}

	resultCh := make(chan toolCallResultPair, 1)
	start := time.Now()
	go func() {
		r, e := handler(callCtx, json.RawMessage(args), sink)
		resultCh <- toolCallResultPair{result: r, err: e}
	}()

	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case pair := <-resultCh:
			return pair.result, pair.err, true
		case <-heartbeatTicker.C:
			sink.OnEvent("tool_progress", fmt.Sprintf("⏳ %s 执行中 (%s)...", originalName, fmtDuration(time.Since(start))))
		}
	}
}

func (rt *ToolExecutionRuntime) dispatchSkill(call ToolExecutionCall, callCtx context.Context) (*ToolCallResult, error, bool) {
	var skillArgs struct {
		SkillName string `json:"skill_name"`
		Query     string `json:"query"`
	}
	if jsonErr := json.Unmarshal([]byte(call.ToolCall.Function.Arguments), &skillArgs); jsonErr != nil {
		return &ToolCallResult{Result: fmt.Sprintf("参数解析失败: %v", jsonErr), AgentID: "builtin"}, nil, true
	}

	call.Sink.OnEvent("skill_start", rt.formatSkillStart(call.Mode, call.ScopeID, skillArgs.SkillName, skillArgs.Query))
	log.Printf("[ToolExecution] mode=%s scope=%s → execute_skill: skill=%s query=%s", call.Mode, call.ScopeID, skillArgs.SkillName, truncate(skillArgs.Query, 200))
	result := rt.bridge.executeSkillSubTask(&TaskContext{
		Ctx:     callCtx,
		TaskID:  call.TaskID,
		Account: call.Account,
		Source:  call.Source,
		Sink:    call.Sink,
	}, skillArgs.SkillName, skillArgs.Query, call.AvailableTools)
	return &ToolCallResult{Result: result, AgentID: "builtin"}, nil, true
}

func (rt *ToolExecutionRuntime) finish(call ToolExecutionCall, originalName, result string, tcResult *ToolCallResult, err error, duration time.Duration) ToolExecutionResult {
	var toAgent, fromAgent string
	if tcResult != nil {
		toAgent = tcResult.AgentID
		fromAgent = tcResult.FromID
	}

	success := err == nil
	if originalName == "ExecuteCode" && result != "" {
		stdout, execSummary := parseExecuteCodeResult(result)
		if stdout != "" {
			result = stdout
		}
		call.Sink.OnEvent("tool_result", rt.formatToolResultEvent(call.Mode, call.ScopeID, originalName, result, toAgent, fromAgent, execSummary, err, duration))
	} else {
		call.Sink.OnEvent("tool_result", rt.formatToolResultEvent(call.Mode, call.ScopeID, originalName, result, toAgent, fromAgent, "", err, duration))
	}

	record := ToolCallRecord{
		ID:         call.ToolCall.ID,
		ToolName:   originalName,
		Arguments:  call.ToolCall.Function.Arguments,
		Result:     result,
		Success:    success,
		DurationMs: duration.Milliseconds(),
		Timestamp:  time.Now(),
		Iteration:  call.Iteration,
	}
	call.Session.RecordToolCall(record)

	if call.TraceRound != nil {
		call.TraceRound.ToolCalls = append(call.TraceRound.ToolCalls, TraceToolCall{
			ToolName:   originalName,
			Arguments:  truncate(call.ToolCall.Function.Arguments, 100),
			Success:    success,
			DurationMs: duration.Milliseconds(),
			ResultLen:  len(result),
		})
	}

	var bizErr string
	if err == nil && result != "" {
		var bizResult struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if json.Unmarshal([]byte(result), &bizResult) == nil && !bizResult.Success && bizResult.Error != "" {
			bizErr = bizResult.Error
		}
	}

	if call.Task != nil {
		rt.bridge.hooks.FireToolCall(call.Task, record)
	}

	return ToolExecutionResult{
		ToolName:    originalName,
		Result:      result,
		BusinessErr: bizErr,
	}
}

func (rt *ToolExecutionRuntime) formatToolCallEvent(mode ToolExecutionMode, scopeID, originalName, args string, idx, total int) string {
	if originalName == "ExecuteCode" {
		event := formatExecuteCodeEvent(args, idx+1, total)
		if mode == ToolExecutionModeSubtask {
			return fmt.Sprintf("[%s] %s", scopeID, event)
		}
		return event
	}
	if mode == ToolExecutionModeSubtask {
		return fmt.Sprintf("[%s] 调用 %s (%d/%d)\n参数: %s", scopeID, originalName, idx+1, total, args)
	}
	return fmt.Sprintf("调用 %s (%d/%d)\n参数: %s", originalName, idx+1, total, args)
}

func (rt *ToolExecutionRuntime) formatSkillStart(mode ToolExecutionMode, scopeID, skillName, query string) string {
	if mode == ToolExecutionModeSubtask {
		return fmt.Sprintf("[%s] 执行技能: %s\n任务: %s", scopeID, skillName, query)
	}
	return fmt.Sprintf("正在执行技能: %s\n任务: %s", skillName, query)
}

func (rt *ToolExecutionRuntime) formatToolResultEvent(mode ToolExecutionMode, scopeID, originalName, result, toAgent, fromAgent, execSummary string, err error, duration time.Duration) string {
	scopePrefix := ""
	if mode == ToolExecutionModeSubtask {
		scopePrefix = "[" + scopeID + "] "
	}

	if err != nil {
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
		return fmt.Sprintf("❌ %s%s 失败 →%s (%.1fs): %s", scopePrefix, originalName, toAgent, duration.Seconds(), truncate(result, 300))
	}

	if originalName == "ExecuteCode" {
		eventText := fmt.Sprintf("✅ %sExecuteCode (%.1fs)", scopePrefix, duration.Seconds())
		if execSummary != "" {
			eventText += "\n" + execSummary
		}
		eventText += fmt.Sprintf("\n输出: %s", truncate(result, 300))
		return eventText
	}

	if mode == ToolExecutionModeQuery {
		var stdResult struct {
			Data    any    `json:"data"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(result), &stdResult) == nil && stdResult.Message != "" {
			return fmt.Sprintf("✅ %s%s: %s", scopePrefix, originalName, stdResult.Message)
		}
	}
	return fmt.Sprintf("✅ %s%s [%s→%s] (%.1fs)\n结果: %s", scopePrefix, originalName, toAgent, fromAgent, duration.Seconds(), truncate(result, 300))
}
