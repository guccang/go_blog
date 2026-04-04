package main

type SessionRuntime struct {
	messages       []Message
	toolView       *ToolRuntimeView
	compactConfig  RuntimeCompactConfig
	compactHistory []RuntimeCompactMetadata
}

func NewSessionRuntime(messages []Message, toolView *ToolRuntimeView, compactConfig RuntimeCompactConfig) *SessionRuntime {
	cloned := make([]Message, len(messages))
	copy(cloned, messages)
	return &SessionRuntime{
		messages:      cloned,
		toolView:      toolView,
		compactConfig: compactConfig,
	}
}

func (sr *SessionRuntime) Messages() []Message {
	cloned := make([]Message, len(sr.messages))
	copy(cloned, sr.messages)
	return cloned
}

func (sr *SessionRuntime) VisibleTools() []LLMTool {
	if sr.toolView == nil {
		return nil
	}
	return sr.toolView.Visible()
}

func (sr *SessionRuntime) DisableTools() {
	if sr.toolView != nil {
		sr.toolView.VisibleTools = nil
	}
}

func (sr *SessionRuntime) CompactHistory() []RuntimeCompactMetadata {
	cloned := make([]RuntimeCompactMetadata, len(sr.compactHistory))
	copy(cloned, sr.compactHistory)
	return cloned
}

func (sr *SessionRuntime) CompactIfNeeded(iteration int, reason string) *RuntimeCompactMetadata {
	if !shouldCompactMessages(sr.messages, sr.compactConfig) {
		return nil
	}
	compacted, meta := compactRuntimeMessages(sr.messages, sr.compactConfig, iteration, reason)
	if meta == nil {
		return nil
	}
	sr.messages = compacted
	sr.compactHistory = append(sr.compactHistory, *meta)
	return meta
}

func (sr *SessionRuntime) AppendMessage(msg Message, session *TaskSession) {
	sr.messages = append(sr.messages, msg)
	if session != nil {
		session.AppendMessage(msg)
	}
}

func (sr *SessionRuntime) AppendToolResult(result string, toolCallID string, iteration int, session *TaskSession) Message {
	msg := RuntimeMessage{
		Role:       "tool",
		Content:    truncateToolResult(result, iteration),
		ToolCallID: toolCallID,
	}.ToMessage()
	sr.AppendMessage(msg, session)
	return msg
}

func (sr *SessionRuntime) ExpandSiblingTools(bridge *Bridge, failedTools []string) []string {
	if sr.toolView == nil {
		return nil
	}
	return bridge.expandSiblingToolsInView(sr.toolView, failedTools)
}
