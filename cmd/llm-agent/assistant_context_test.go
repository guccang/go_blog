package main

import (
	"strings"
	"testing"
)

func TestBuildPersistentAssistantRecordForAsyncTask(t *testing.T) {
	root := NewRootSession("task_async", "部署 blog-agent", "ztt")
	root.RecordToolCall(ToolCallRecord{
		ToolName: "DeployProject",
		Result:   `{"success":true,"status":"queued","session_id":"deploy_123","data":{"project":"blog-agent","deploy_target":"ssh-prod"}}`,
		Success:  true,
	})

	child := NewChildSession(root, "部署 blog-agent", "把 blog-agent 部署到 ssh-prod 服务器")
	child.ID = "deploy"
	child.Status = "async"
	child.Result = "已提交部署请求"
	child.RecordToolCall(ToolCallRecord{
		ToolName: "DeployProject",
		Result:   `{"success":true,"status":"queued","session_id":"deploy_123","message":"deploy queued"}`,
		Success:  true,
	})
	child.RecordToolCall(ToolCallRecord{
		ToolName: "DeployProject",
		Result:   `{"success":true,"status":"queued","session_id":"deploy_123","message":"deploy queued"}`,
		Success:  true,
	})

	record := buildPersistentAssistantRecord(AssistantRecordInput{
		Query:         "部署 blog-agent",
		DisplayResult: "📋 任务已派发，进度将通过微信推送",
		Status:        "async",
		RootSession:   root,
		ChildSessions: map[string]*TaskSession{"deploy": child},
		Results: []SubTaskResult{
			{
				SubTaskID: "deploy",
				Title:     "部署 blog-agent",
				Status:    "async",
				Result:    "已提交部署请求",
				AsyncSessions: []AsyncSessionInfo{
					{ToolName: "DeployProject", SessionID: "deploy_123", Message: "deploy queued"},
					{ToolName: "DeployProject", SessionID: "deploy_123", Message: "deploy queued"},
				},
			},
		},
	})

	if !strings.HasPrefix(record, assistantRecordHeader) {
		t.Fatalf("expected structured assistant record, got: %s", record)
	}
	if !strings.Contains(record, "状态: async") {
		t.Fatalf("expected async status in record: %s", record)
	}
	if !strings.Contains(record, "恢复建议: 优先使用 DeployGetStatus 查询 session_id=deploy_123") {
		t.Fatalf("expected recovery hint in record: %s", record)
	}
	if !strings.Contains(record, "DeployProject: session_id=deploy_123, status=queued, project=blog-agent, deploy_target=ssh-prod") {
		t.Fatalf("expected key tool facts in record: %s", record)
	}
}

func TestBuildPersistentAssistantRecordFallsBackForPlainReply(t *testing.T) {
	record := buildPersistentAssistantRecord(AssistantRecordInput{
		Query:         "你好",
		DisplayResult: "你好，有什么可以帮你？",
		Status:        "done",
	})

	if record != "你好，有什么可以帮你？" {
		t.Fatalf("expected plain reply to be kept, got %q", record)
	}
}

func TestPersistedAssistantContentPrefersStructuredRecord(t *testing.T) {
	ctx := &TaskContext{PersistedAssistant: assistantRecordHeader + "\n状态: async"}
	if got := persistedAssistantContent(ctx, "展示文案"); got != ctx.PersistedAssistant {
		t.Fatalf("expected structured content, got %q", got)
	}

	ctx.PersistedAssistant = "普通文案"
	if got := persistedAssistantContent(ctx, "展示文案"); got != "展示文案" {
		t.Fatalf("expected fallback content, got %q", got)
	}
}
