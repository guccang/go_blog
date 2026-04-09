package main

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type resumeSink struct {
	sendEvent func(event, text string)
}

func (s *resumeSink) OnChunk(text string) {}

func (s *resumeSink) OnEvent(event, text string) {
	if s.sendEvent != nil {
		s.sendEvent(event, text)
	}
}

func (s *resumeSink) Streaming() bool { return false }

func (b *Bridge) resumeQuery(taskID, rootSessionID, account string, sendEvent func(event, text string)) (string, error) {
	store := NewSessionStore(b.cfg.SessionDir)
	rootSession, err := store.Load(rootSessionID, rootSessionID)
	if err != nil {
		return "", fmt.Errorf("load root session: %v", err)
	}

	if strings.TrimSpace(rootSession.Result) != "" && (rootSession.Status == "done" || rootSession.Status == "async") {
		if sendEvent != nil {
			sendEvent("resume_info", "任务已有结果，直接返回已持久化内容")
		}
		return rootSession.Result, nil
	}

	if strings.TrimSpace(account) != "" {
		rootSession.Account = account
	}
	if strings.TrimSpace(rootSession.Account) == "" {
		return "", fmt.Errorf("resume task missing account")
	}
	if strings.TrimSpace(rootSession.Source) == "" {
		rootSession.Source = "resume_task"
	}

	messages := make([]Message, len(rootSession.Messages))
	copy(messages, rootSession.Messages)
	messages = sanitizeResumeMessages(messages)
	if len(messages) == 0 {
		return "", fmt.Errorf("root session has no resumable messages")
	}

	runtimeSnapshot, err := store.LoadRuntimeSnapshot(rootSessionID, rootSessionID)
	if err != nil {
		log.Printf("[Resume] warn: load runtime state failed root=%s err=%v", rootSessionID, err)
	}
	if runtimeSnapshot != nil && strings.TrimSpace(runtimeSnapshot.PromptContext.SystemPrompt) != "" && messages[0].Role == "system" {
		messages[0].Content = runtimeSnapshot.PromptContext.SystemPrompt
	}

	query := ""
	if runtimeSnapshot != nil {
		query = strings.TrimSpace(runtimeSnapshot.Query)
	}
	if query == "" {
		query = strings.TrimSpace(rootSession.Title)
	}
	if query == "" {
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" && strings.TrimSpace(messages[i].Content) != "" {
				query = strings.TrimSpace(messages[i].Content)
				break
			}
		}
	}

	ctx := &TaskContext{
		Ctx:            context.Background(),
		TaskID:         rootSession.ID,
		Account:        rootSession.Account,
		Query:          query,
		Source:         rootSession.Source,
		Messages:       messages,
		Sink:           &resumeSink{sendEvent: sendEvent},
		ResumeSession:  rootSession,
		ResumeSnapshot: runtimeSnapshot,
	}

	if sendEvent != nil {
		sendEvent("resume", fmt.Sprintf("正在恢复任务 %s", rootSessionID))
	}

	if err := b.replayPendingToolCalls(ctx); err != nil {
		if sendEvent != nil {
			sendEvent("resume_info", fmt.Sprintf("恢复未完成工具调用失败: %v", err))
		}
		return "", err
	}

	if sendEvent != nil {
		sendEvent("resume_info", fmt.Sprintf("恢复消息 %d 条，继续执行", len(ctx.Messages)))
	}
	return b.processTask(ctx)
}

func (b *Bridge) resumeRootQuery(taskID, rootSessionID, account string, sendEvent func(event, text string)) (string, error) {
	return b.resumeQuery(taskID, rootSessionID, account, sendEvent)
}

func (b *Bridge) replayPendingToolCalls(ctx *TaskContext) error {
	if ctx == nil || ctx.ResumeSession == nil || len(ctx.Messages) == 0 {
		return nil
	}
	last := ctx.Messages[len(ctx.Messages)-1]
	if last.Role != "assistant" || len(last.ToolCalls) == 0 {
		return nil
	}

	allTools := b.getLLMTools()
	localHandlers := b.buildRuntimeLocalHandlers(ctx, allTools)
	toolExec := NewToolExecutionRuntime(b)
	eventSink := ctx.Sink
	if eventSink == nil {
		eventSink = &BufferSink{}
	}

	for idx, tc := range last.ToolCalls {
		execResult := toolExec.Execute(ToolExecutionCall{
			Mode:           ToolExecutionModeQuery,
			Ctx:            ctx.Ctx,
			Task:           ctx,
			TaskID:         ctx.TaskID,
			Account:        ctx.Account,
			Source:         ctx.Source,
			Session:        ctx.ResumeSession,
			AvailableTools: allTools,
			LocalHandlers:  localHandlers,
			Sink:           eventSink,
			Iteration:      0,
			Index:          idx,
			Total:          len(last.ToolCalls),
			ToolCall:       tc,
		})
		if execResult.Interrupted {
			return context.Canceled
		}
		if execResult.FatalErr != nil {
			return execResult.FatalErr
		}

		toolMsg := Message{
			Role:       "tool",
			Content:    truncateToolResult(execResult.Result, 0),
			ToolCallID: tc.ID,
		}
		ctx.Messages = append(ctx.Messages, toolMsg)
	}

	sessionMessages := make([]Message, len(ctx.Messages))
	copy(sessionMessages, ctx.Messages)
	ctx.ResumeSession.mu.Lock()
	ctx.ResumeSession.Messages = sessionMessages
	ctx.ResumeSession.mu.Unlock()
	store := NewSessionStore(b.cfg.SessionDir)
	if err := store.Save(ctx.ResumeSession); err != nil {
		return fmt.Errorf("save resumed session after replay: %v", err)
	}
	if ctx.ResumeSnapshot != nil {
		ctx.ResumeSnapshot.Status = "running"
		if err := store.SaveRuntimeSnapshot(*ctx.ResumeSnapshot); err != nil {
			return fmt.Errorf("save runtime snapshot after replay: %v", err)
		}
	}
	return nil
}

func (b *Bridge) replayPendingRootToolCalls(ctx *TaskContext) error {
	return b.replayPendingToolCalls(ctx)
}

func sanitizeResumeMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}

	pendingToolCalls := make(map[string]bool)
	sanitized := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
			continue
		}
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				pendingToolCalls[tc.ID] = true
			}
			sanitized = append(sanitized, msg)
			continue
		}
		if msg.Role == "tool" && msg.ToolCallID != "" {
			if !pendingToolCalls[msg.ToolCallID] {
				continue
			}
			delete(pendingToolCalls, msg.ToolCallID)
		}
		sanitized = append(sanitized, msg)
	}

	for i := len(sanitized) - 1; i >= 0; i-- {
		msg := sanitized[i]
		if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
			continue
		}

		resolved := true
		for _, tc := range msg.ToolCalls {
			if pendingToolCalls[tc.ID] {
				resolved = false
				break
			}
		}
		if resolved {
			continue
		}

		// 只保留仍需补执行的最后一个 assistant tool_use 片段，避免重复回放更早的已完成片段。
		for j := i + 1; j < len(sanitized); j++ {
			if sanitized[j].Role == "tool" {
				break
			}
		}
		return sanitized[:i+1]
	}
	return sanitized
}
