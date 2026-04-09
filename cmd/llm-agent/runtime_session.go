package main

type QuerySession struct {
	messages       []Message
	toolView       *ToolRuntimeView
	compactConfig  RuntimeCompactConfig
	compactHistory []RuntimeCompactMetadata
	attachments    []Attachment
}

// SessionRuntime 保留旧名称，兼容现有调用。
type SessionRuntime = QuerySession

func NewQuerySession(messages []Message, toolView *ToolRuntimeView, compactConfig RuntimeCompactConfig) *QuerySession {
	cloned := make([]Message, len(messages))
	copy(cloned, messages)
	return &QuerySession{
		messages:      cloned,
		toolView:      toolView,
		compactConfig: compactConfig,
	}
}

func NewSessionRuntime(messages []Message, toolView *ToolRuntimeView, compactConfig RuntimeCompactConfig) *QuerySession {
	return NewQuerySession(messages, toolView, compactConfig)
}

func NewQuerySessionFromSnapshot(messages []Message, snapshot *RuntimeSnapshot, toolView *ToolRuntimeView, compactConfig RuntimeCompactConfig) *QuerySession {
	session := NewQuerySession(messages, toolView, compactConfig)
	if snapshot == nil {
		return session
	}
	session.attachments = cloneAttachments(snapshot.Attachments)
	if len(snapshot.CompactHistory) > 0 {
		session.compactHistory = make([]RuntimeCompactMetadata, len(snapshot.CompactHistory))
		copy(session.compactHistory, snapshot.CompactHistory)
	}
	return session
}

func NewSessionRuntimeFromSnapshot(messages []Message, snapshot *RuntimeSnapshot, toolView *ToolRuntimeView, compactConfig RuntimeCompactConfig) *QuerySession {
	return NewQuerySessionFromSnapshot(messages, snapshot, toolView, compactConfig)
}

func (sr *QuerySession) Messages() []Message {
	cloned := make([]Message, len(sr.messages))
	copy(cloned, sr.messages)
	return cloned
}

func (sr *QuerySession) VisibleTools() []LLMTool {
	if sr.toolView == nil {
		return nil
	}
	return sr.toolView.Visible()
}

func (sr *QuerySession) DisableTools() {
	if sr.toolView != nil {
		sr.toolView.VisibleTools = nil
	}
}

func (sr *QuerySession) CompactHistory() []RuntimeCompactMetadata {
	cloned := make([]RuntimeCompactMetadata, len(sr.compactHistory))
	copy(cloned, sr.compactHistory)
	return cloned
}

func (sr *QuerySession) Attachments() []Attachment {
	return cloneAttachments(sr.attachments)
}

func (sr *QuerySession) CompactIfNeeded(iteration int, reason string) *RuntimeCompactMetadata {
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

func (sr *QuerySession) AppendMessage(msg Message, session *TaskSession) {
	sr.messages = append(sr.messages, msg)
	if session != nil {
		session.AppendMessage(msg)
	}
}

func (sr *QuerySession) AppendAttachmentMessages(attachments []Attachment, session *TaskSession) int {
	if len(attachments) == 0 {
		return 0
	}
	for _, att := range attachments {
		sr.attachments = append(sr.attachments, att)
		sr.AppendMessage(createAttachmentMessage(att), session)
	}
	return len(attachments)
}

func (sr *QuerySession) InjectAttachments(attachments []Attachment, session *TaskSession) int {
	return sr.AppendAttachmentMessages(attachments, session)
}

func (sr *QuerySession) AppendToolResult(result string, toolCallID string, iteration int, session *TaskSession) Message {
	msg := RuntimeMessage{
		Role:       "tool",
		Content:    truncateToolResult(result, iteration),
		ToolCallID: toolCallID,
	}.ToMessage()
	sr.AppendMessage(msg, session)
	return msg
}

func (sr *QuerySession) ExpandSiblingTools(bridge *Bridge, failedTools []string) []string {
	if sr.toolView == nil {
		return nil
	}
	return bridge.expandSiblingToolsInView(sr.toolView, failedTools)
}

func (sr *QuerySession) Snapshot(rootID, sessionID, query, status string, promptCtx SystemPromptContext) RuntimeSnapshot {
	return RuntimeSnapshot{
		RootID:         rootID,
		SessionID:      sessionID,
		Query:          query,
		Status:         status,
		PromptContext:  promptCtx,
		Attachments:    cloneAttachments(sr.attachments),
		CompactHistory: sr.CompactHistory(),
	}
}

func (sr *QuerySession) SnapshotState(rootID, sessionID, query, status string, promptCtx PromptContext) RuntimeSnapshot {
	return sr.Snapshot(rootID, sessionID, query, status, promptCtx)
}
