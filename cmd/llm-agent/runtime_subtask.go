package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	errSubTaskCancelled = errors.New("subtask cancelled")
	errSubTaskTimeout   = errors.New("subtask timeout")
)

func (o *Orchestrator) saveSession(session *TaskSession) {
	if o.store != nil {
		o.store.Save(session)
	}
}

func (o *Orchestrator) runSubTaskLoop(
	ctx context.Context,
	taskID string,
	subtask SubTaskPlan,
	session *TaskSession,
	messages []Message,
	toolView *ToolRuntimeView,
	promptCtx PromptContext,
	sendEvent func(event, text string),
	_ <-chan string,
	deadline time.Time,
	trace *RequestTrace,
) (string, error) {
	maxIter := o.cfg.SubTaskMaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	if toolView == nil {
		toolView = o.bridge.buildSubTaskToolRuntimeView(nil, subtask.ToolsHint)
	}
	sessionRT := NewQuerySession(messages, toolView, defaultSubTaskCompactConfig())
	toolExec := NewToolExecutionRuntime(o.bridge)
	eventSink := &functionEventSink{sendEvent: sendEvent}
	var finalText string
	executionSource := strings.TrimSpace(session.Source)
	if executionSource == "" {
		executionSource = "subtask"
	}
	o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)

	for i := 0; i < maxIter; i++ {
		if ctx != nil && ctx.Err() != nil {
			log.Printf("[Orchestrator] ✗ 子任务取消 id=%s reason=%v", subtask.ID, ctx.Err())
			if trace != nil {
				trace.RecordPath("subtask_cancelled", "子任务被取消", nil)
			}
			o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
			return finalText, errSubTaskCancelled
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			log.Printf("[Orchestrator] ✗ 子任务超时 id=%s", subtask.ID)
			sendEvent("subtask_timeout", fmt.Sprintf("[%s] %s — 执行超时", subtask.ID, subtask.Title))
			if trace != nil {
				trace.RecordPath("subtask_timeout", "子任务执行超时", nil)
			}
			o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
			return finalText, errSubTaskTimeout
		}

		if injected := o.drainMailboxAttachments(sessionRT, session); injected > 0 {
			sendEvent("subtask_context", fmt.Sprintf("[%s] 注入 %d 条运行时上下文", subtask.ID, injected))
			if trace != nil {
				trace.RecordPath("mailbox_injected", fmt.Sprintf("注入 %d 条运行时上下文", injected), nil)
			}
		}
		o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)

		if meta := sessionRT.CompactIfNeeded(i, "subtask_turn"); meta != nil {
			log.Printf("[Orchestrator] subtask=%s 消息压缩 messages=%d→%d chars=%d→%d toolTrim=%d",
				subtask.ID, meta.BeforeMessages, meta.AfterMessages, meta.BeforeChars, meta.AfterChars, meta.ToolResultsTrimed)
			if trace != nil {
				trace.RecordRoundCompaction(i+1, meta)
			}
		}

		log.Printf("[Orchestrator] subtask=%s 迭代 %d/%d messages=%d", subtask.ID, i+1, maxIter, len(sessionRT.Messages()))

		if i == 0 {
			sendEvent("subtask_thinking", fmt.Sprintf("[%s] 正在思考...", subtask.ID))
		} else {
			sendEvent("subtask_thinking", fmt.Sprintf("[%s] 第%d轮分析...", subtask.ID, i+1))
		}

		llmStart := time.Now()
		text, toolCalls, err := o.sendLLMCtx(ctx, sessionRT.Messages(), sessionRT.VisibleTools())
		llmDuration := time.Since(llmStart)
		if err != nil {
			log.Printf("[Orchestrator] ✗ 子任务 %s LLM失败 duration=%v error=%v", subtask.ID, llmDuration, err)
			sendEvent("subtask_llm_error", fmt.Sprintf("[%s] %s — LLM调用失败: %v", subtask.ID, subtask.Title, err))
			if trace != nil {
				trace.RecordEvent("subtask_llm_error", subtask.Title, err.Error(), i+1, nil)
			}
			o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
			return finalText, err
		}
		var currentRound *TraceRound
		if trace != nil {
			trace.RecordRoundLLM(i+1, llmDuration, text, toolCalls, sessionRT.VisibleTools())
			trace.RecordPath(fmt.Sprintf("subtask_round_%d_llm", i+1),
				fmt.Sprintf("LLM返回 text_len=%d tool_calls=%d", len(text), len(toolCalls)),
				map[string]string{"scope": subtask.ID})
			currentRound = trace.EnsureRound(i + 1)
		} else {
			currentRound = &TraceRound{Index: i + 1}
		}

		assistantMsg := Message{Role: "assistant", Content: text, ToolCalls: toolCalls}
		sessionRT.AppendMessage(assistantMsg, session)
		o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)

		if len(toolCalls) == 0 {
			log.Printf("[Orchestrator] ✓ 子任务 %s 对话结束（无工具调用） textLen=%d", subtask.ID, len(text))
			if text != "" {
				sendEvent("subtask_response", fmt.Sprintf("[%s] %s", subtask.ID, truncate(text, 500)))
			}
			finalText = text
			break
		}

		if text != "" {
			sendEvent("subtask_response", fmt.Sprintf("[%s] %s", subtask.ID, truncate(text, 300)))
		}
		var bizFailedTools []string
		var bizFailedMsgs []string

		for tcIdx, tc := range toolCalls {
			execResult := toolExec.Execute(ToolExecutionCall{
				Mode:           ToolExecutionModeSubtask,
				Ctx:            ctx,
				TaskID:         taskID,
				Account:        session.Account,
				Source:         executionSource,
				Session:        session,
				AvailableTools: toolView.AllTools,
				Sink:           eventSink,
				ScopeID:        subtask.ID,
				Iteration:      i,
				Index:          tcIdx,
				Total:          len(toolCalls),
				ToolCall:       tc,
				TraceRound:     currentRound,
				Trace:          trace,
			})
			if execResult.Interrupted {
				o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
				return finalText, errSubTaskCancelled
			}
			if execResult.FatalErr != nil {
				o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
				return finalText, execResult.FatalErr
			}
			if execResult.BusinessErr != "" {
				bizFailedTools = append(bizFailedTools, execResult.ToolName)
				bizFailedMsgs = append(bizFailedMsgs, execResult.BusinessErr)
			}
			sessionRT.AppendToolResult(execResult.Result, tc.ID, i, session)
			o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
			if execResult.Success && execResult.BusinessErr == "" && isTerminalSessionTool(execResult.ToolName) {
				finalText = buildTerminalToolSummary(execResult.ToolName, execResult.Result)
				if finalText == "" {
					finalText = text
				}
				log.Printf("[Orchestrator] subtask=%s terminal tool completed, stop loop: %s", subtask.ID, execResult.ToolName)
				if trace != nil {
					trace.RecordPath("terminal_tool_stop", fmt.Sprintf("terminal tool=%s", execResult.ToolName), nil)
				}
				return finalText, nil
			}
		}

		if len(bizFailedTools) > 0 {
			newToolNames := sessionRT.ExpandSiblingTools(o.bridge, bizFailedTools)
			if len(newToolNames) > 0 {
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
					session.ID,
					map[string]string{"expanded_tools": strings.Join(newToolNames, ",")},
				)
				sessionRT.InjectAttachments([]RuntimeAttachment{attachment}, session)
				log.Printf("[Orchestrator] subtask=%s 业务失败扩展: 新增 %d 个兄弟工具: %v", subtask.ID, len(newToolNames), newToolNames)
				sendEvent("tool_expand", fmt.Sprintf("[%s] 工具业务失败，补充兄弟工具: %s", subtask.ID, strings.Join(newToolNames, ", ")))
				if trace != nil {
					trace.RecordPath("tool_expand", fmt.Sprintf("业务失败后扩展兄弟工具: %s", strings.Join(newToolNames, ", ")), nil)
				}
				o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)
			}
		}

		if i == maxIter-1 {
			finalText = text
		}
	}

	o.saveSessionCheckpoint(sessionRT, session, subtask.Description, promptCtx, trace)

	return finalText, nil
}
