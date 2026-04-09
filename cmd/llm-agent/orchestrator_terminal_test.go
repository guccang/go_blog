package main

import (
	"strings"
	"testing"
)

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
