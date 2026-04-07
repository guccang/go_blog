package main

import (
	"strings"
	"testing"
)

func TestFindSuccessfulTerminalToolCallFindsLatestSessionTool(t *testing.T) {
	session := &TaskSession{
		ToolCalls: []ToolCallRecord{
			{ToolName: "ExecuteCode", Success: true},
			{ToolName: "AcpStartSession", Success: true, Result: `{"success":true,"status":"in_progress","message":"已创建会话","data":{"session_id":"acp_123","project_dir":"E:/workspace/demo"}}`},
			{ToolName: "ReadFile", Success: true},
		},
	}

	rec, ok := findSuccessfulTerminalToolCall(session)
	if !ok {
		t.Fatal("expected to find terminal tool call")
	}
	if rec.ToolName != "AcpStartSession" {
		t.Fatalf("expected AcpStartSession, got %s", rec.ToolName)
	}
}

func TestBuildTerminalToolSummaryIncludesSessionAndProjectDir(t *testing.T) {
	got := buildTerminalToolSummary("AcpStartSession", `{"success":true,"status":"in_progress","message":"编码已开始","data":{"session_id":"acp_123","project_dir":"E:/workspace/demo"}}`)
	if !strings.Contains(got, "AcpStartSession 完成") {
		t.Fatalf("expected tool name in summary, got %s", got)
	}
	if !strings.Contains(got, "session_id=acp_123") {
		t.Fatalf("expected session_id in summary, got %s", got)
	}
	if !strings.Contains(got, "project_dir=E:/workspace/demo") {
		t.Fatalf("expected project_dir in summary, got %s", got)
	}
}

func TestBuildSubTaskHandleKeySeparatesRootTasks(t *testing.T) {
	if got := buildSubTaskHandleKey("root-a", "t1"); got != "root-a/t1" {
		t.Fatalf("unexpected handle key: %s", got)
	}
	if buildSubTaskHandleKey("root-a", "t1") == buildSubTaskHandleKey("root-b", "t1") {
		t.Fatal("expected different root tasks to have different handle keys")
	}
}
