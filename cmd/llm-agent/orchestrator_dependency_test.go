package main

import (
	"strings"
	"testing"
)

func TestBuildSubTaskResultTextPutsKeyDataBeforeSummary(t *testing.T) {
	keyData := keyToolDataHeader + "\n- AcpStartSession: project_dir=/tmp/webcalc, session_id=acp_123"
	summary := "已完成 go web 计算器编码，并创建了入口文件与监听端口。"

	got := buildSubTaskResultText(summary, keyData)
	if !strings.HasPrefix(got, keyData) {
		t.Fatalf("expected key data to appear first, got: %s", got)
	}
	if !strings.Contains(got, "结果摘要:\n"+summary) {
		t.Fatalf("expected summary section after key data, got: %s", got)
	}
}

func TestSanitizeResumeMessagesDropsWhitespaceAssistantAndOrphanTool(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "system"},
		{Role: "assistant", Content: "   "},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID:   "tc-1",
					Type: "function",
					Function: FunctionCall{
						Name:      "ExecuteCode",
						Arguments: `{"code":"print(1)"}`,
					},
				},
			},
		},
		{Role: "tool", Content: "ok", ToolCallID: "tc-1"},
		{Role: "tool", Content: "orphan", ToolCallID: "tc-x"},
	}

	got := sanitizeResumeMessages(messages)
	if len(got) != 3 {
		t.Fatalf("expected 3 sanitized messages, got %d: %#v", len(got), got)
	}
	if got[1].Role != "assistant" || len(got[1].ToolCalls) != 1 {
		t.Fatalf("expected tool-call assistant to be kept, got %#v", got[1])
	}
	if got[2].ToolCallID != "tc-1" {
		t.Fatalf("expected matching tool result to be kept, got %#v", got[2])
	}
}
