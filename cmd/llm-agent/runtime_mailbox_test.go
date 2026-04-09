package main

import (
	"testing"
	"time"
)

func TestSessionStoreMailboxDrainAndRuntimeState(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	root := NewRootSession("root-test", "root", "acct")
	if err := store.Save(root); err != nil {
		t.Fatalf("save root: %v", err)
	}

	msg1 := newMailboxMessage(root.RootID, "child-1", root.ID, string(RuntimeAttachmentDependencyResult), "前置任务结果", "dep result", map[string]string{"k": "v"})
	msg2 := newMailboxMessage(root.RootID, "child-1", root.ID, string(RuntimeAttachmentResume), "恢复提示", "resume", nil)
	if err := store.EnqueueMailbox(root.RootID, "child-1", msg1); err != nil {
		t.Fatalf("enqueue msg1: %v", err)
	}
	if err := store.EnqueueMailbox(root.RootID, "child-1", msg2); err != nil {
		t.Fatalf("enqueue msg2: %v", err)
	}

	peeked, err := store.PeekMailbox(root.RootID, "child-1")
	if err != nil {
		t.Fatalf("peek mailbox: %v", err)
	}
	if len(peeked) != 2 {
		t.Fatalf("expected 2 mailbox messages, got %d", len(peeked))
	}

	drained, err := store.DrainMailbox(root.RootID, "child-1")
	if err != nil {
		t.Fatalf("drain mailbox: %v", err)
	}
	if len(drained) != 2 {
		t.Fatalf("expected 2 drained messages, got %d", len(drained))
	}
	drainedAgain, err := store.DrainMailbox(root.RootID, "child-1")
	if err != nil {
		t.Fatalf("drain mailbox again: %v", err)
	}
	if len(drainedAgain) != 0 {
		t.Fatalf("expected mailbox to be empty after drain, got %d", len(drainedAgain))
	}

	state := RuntimeStateSnapshot{
		RootID:    root.RootID,
		SessionID: "child-1",
		Query:     "do work",
		Status:    "running",
		PromptContext: PromptContext{
			Account:      "acct",
			SystemPrompt: "system",
		},
		Attachments: []RuntimeAttachment{
			newRuntimeAttachment(RuntimeAttachmentDependencyResult, "前置任务结果", "dep result", root.ID, nil),
		},
	}
	if err := store.SaveRuntimeState(state); err != nil {
		t.Fatalf("save runtime state: %v", err)
	}
	loaded, err := store.LoadRuntimeState(root.RootID, "child-1")
	if err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected runtime state")
	}
	if loaded.PromptContext.SystemPrompt != "system" {
		t.Fatalf("unexpected prompt context: %+v", loaded.PromptContext)
	}
	if len(loaded.Attachments) != 1 || loaded.Attachments[0].Content != "dep result" {
		t.Fatalf("unexpected attachments: %+v", loaded.Attachments)
	}
}

func TestSessionRuntimeInjectAttachmentsAndSnapshot(t *testing.T) {
	session := NewRootSession("root-test", "root", "acct")
	rt := NewSessionRuntime([]Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "hello"},
	}, nil, defaultQueryCompactConfig())

	count := rt.InjectAttachments([]RuntimeAttachment{
		newRuntimeAttachment(RuntimeAttachmentTaskNotification, "子任务状态通知", "done", "child-1", map[string]string{"status": "done"}),
	}, session)
	if count != 1 {
		t.Fatalf("expected 1 injected attachment, got %d", count)
	}
	if got := len(rt.Messages()); got != 3 {
		t.Fatalf("expected 3 runtime messages, got %d", got)
	}
	snapshot := rt.SnapshotState(session.RootID, session.ID, "hello", "running", PromptContext{SystemPrompt: "system"})
	if len(snapshot.Attachments) != 1 {
		t.Fatalf("expected 1 attachment in snapshot, got %d", len(snapshot.Attachments))
	}
	if snapshot.Attachments[0].Kind != RuntimeAttachmentTaskNotification {
		t.Fatalf("unexpected attachment kind: %+v", snapshot.Attachments[0])
	}
}

func TestNewQuerySessionFromSnapshotRestoresStateWithoutDuplicatingMessages(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "hello"},
	}
	snapshot := &RuntimeSnapshot{
		Attachments: []Attachment{
			newAttachment(AttachmentKindTaskNotification, "子任务状态通知", "done", "child-1", map[string]string{"status": "done"}),
		},
		CompactHistory: []RuntimeCompactMetadata{
			{Reason: "query_turn", BeforeMessages: 12, AfterMessages: 8, At: time.Now()},
		},
	}

	session := NewQuerySessionFromSnapshot(messages, snapshot, nil, defaultQueryCompactConfig())
	if got := len(session.Messages()); got != len(messages) {
		t.Fatalf("expected messages to stay at %d, got %d", len(messages), got)
	}
	if got := len(session.Attachments()); got != 1 {
		t.Fatalf("expected 1 restored attachment, got %d", got)
	}
	if got := len(session.CompactHistory()); got != 1 {
		t.Fatalf("expected 1 compact history entry, got %d", got)
	}
}

func TestQueryLoopSaveCheckpointPersistsSessionAndRuntimeSnapshot(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	root := NewRootSession("root-test", "root", "acct")
	root.Source = "web"
	root.AppendMessage(Message{Role: "system", Content: "system"})
	root.AppendMessage(Message{Role: "user", Content: "hello"})

	session := NewQuerySession(root.Messages, nil, defaultQueryCompactConfig())
	session.compactHistory = []RuntimeCompactMetadata{
		{Reason: "query_turn", BeforeMessages: 10, AfterMessages: 6, At: time.Now()},
	}
	session.InjectAttachments([]Attachment{
		newAttachment(AttachmentKindResume, "恢复提示", "resume", root.ID, nil),
	}, root)

	loop := &QueryLoop{
		store:       store,
		rootSession: root,
		state: QueryLoopState{
			Query:         "hello",
			PromptContext: SystemPromptContext{Account: "acct", Source: "web", SystemPrompt: "system"},
			Session:       session,
		},
	}

	loop.saveCheckpoint("running")

	savedSession, err := store.Load(root.RootID, root.ID)
	if err != nil {
		t.Fatalf("load saved session: %v", err)
	}
	if got := len(savedSession.Messages); got != len(root.Messages) {
		t.Fatalf("expected %d saved messages, got %d", len(root.Messages), got)
	}

	snapshot, err := store.LoadRuntimeSnapshot(root.RootID, root.ID)
	if err != nil {
		t.Fatalf("load runtime snapshot: %v", err)
	}
	if snapshot == nil {
		t.Fatalf("expected runtime snapshot")
	}
	if snapshot.Query != "hello" || snapshot.Status != "running" {
		t.Fatalf("unexpected snapshot identity: %+v", snapshot)
	}
	if snapshot.PromptContext.SystemPrompt != "system" {
		t.Fatalf("unexpected prompt context: %+v", snapshot.PromptContext)
	}
	if got := len(snapshot.Attachments); got != 1 {
		t.Fatalf("expected 1 attachment in snapshot, got %d", got)
	}
	if got := len(snapshot.CompactHistory); got != 1 {
		t.Fatalf("expected 1 compact history entry, got %d", got)
	}
}
