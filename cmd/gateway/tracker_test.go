package main

import (
	"encoding/json"
	"testing"
	"time"

	"uap"
)

func TestTrackerBindsTaskLifecycleToOriginalTrace(t *testing.T) {
	tracker, err := NewTracker(&TrackerConfig{
		BufferSize: 16,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewTracker error: %v", err)
	}
	defer tracker.Close()

	assign := &uap.Message{
		Type:    uap.MsgTaskAssign,
		ID:      "trace-1",
		Payload: mustTrackerPayload(uap.TaskAssignPayload{TaskID: "task-1"}),
		Ts:      time.Now().UnixMilli(),
	}
	complete := &uap.Message{
		Type:    uap.MsgTaskComplete,
		ID:      "msg-complete",
		Payload: mustTrackerPayload(uap.TaskCompletePayload{TaskID: "task-1", Status: "success"}),
		Ts:      time.Now().UnixMilli(),
	}

	tracker.RecordMessage(EventKindMsgIn, nil, nil, assign)
	tracker.RecordMessage(EventKindMsgOut, nil, nil, complete)

	events := tracker.GetTrace("trace-1")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].TraceID != "trace-1" {
		t.Fatalf("expected task_complete to use original trace_id, got %s", events[1].TraceID)
	}
}

func TestTrackerUsesRequestIDForToolResultTrace(t *testing.T) {
	tracker, err := NewTracker(&TrackerConfig{
		BufferSize: 16,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewTracker error: %v", err)
	}
	defer tracker.Close()

	call := &uap.Message{
		Type:    uap.MsgToolCall,
		ID:      "trace-tool-1",
		Payload: mustTrackerPayload(uap.ToolCallPayload{ToolName: "Echo"}),
		Ts:      time.Now().UnixMilli(),
	}
	result := &uap.Message{
		Type:    uap.MsgToolResult,
		ID:      "tool-result-id",
		Payload: mustTrackerPayload(uap.ToolResultPayload{RequestID: "trace-tool-1", Success: true}),
		Ts:      time.Now().UnixMilli(),
	}

	tracker.RecordMessage(EventKindMsgIn, nil, nil, call)
	tracker.RecordMessage(EventKindMsgOut, nil, nil, result)

	events := tracker.GetTrace("trace-tool-1")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].TraceID != "trace-tool-1" {
		t.Fatalf("expected tool_result to reuse request trace id, got %s", events[1].TraceID)
	}
}

func mustTrackerPayload(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
