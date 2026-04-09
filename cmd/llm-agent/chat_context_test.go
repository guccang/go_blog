package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildContextDebugInfoIncludesRuntimeSnapshot(t *testing.T) {
	sessionMgr := NewChatSessionManager(time.Hour, 40, 15, "")
	session, _ := sessionMgr.GetOrCreate("wechat", "alice", "alice")
	session.LastActiveAt = time.Date(2026, 4, 9, 10, 30, 0, 0, time.Local)
	session.TurnCount = 1
	session.Messages = []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "帮我查一下部署状态"},
		{Role: "assistant", Content: "好的，我来检查。"},
	}
	session.PromptSections = []PromptSection{
		{Name: "基础", Chars: 12},
	}

	sessionDir := t.TempDir()
	store := NewSessionStore(sessionDir)
	rootID := fmt.Sprintf("%s_%d", session.SessionID, 0)
	root := NewRootSession(rootID, "帮我查一下部署状态", "alice")
	root.Source = "wechat"
	root.AppendMessage(Message{Role: "system", Content: "system prompt"})
	root.AppendMessage(Message{Role: "user", Content: "帮我查一下部署状态"})
	root.AppendMessage(Message{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID:   "tc_1",
				Type: "function",
				Function: FunctionCall{
					Name:      "ExecuteCode",
					Arguments: `{"script":"print('ok')"}`,
				},
			},
		},
	})
	if err := store.Save(root); err != nil {
		t.Fatalf("save root: %v", err)
	}

	child := NewChildSession(root, "skill:deploy", "检查部署状态")
	child.SetStatus("done")
	if err := store.Save(child); err != nil {
		t.Fatalf("save child: %v", err)
	}

	snapshot := RuntimeSnapshot{
		RootID:    rootID,
		SessionID: rootID,
		Query:     "帮我查一下部署状态",
		Status:    "running",
		PromptContext: SystemPromptContext{
			SystemPrompt: "system prompt",
			Sections: []PromptSection{
				{Name: "基础", Chars: 12},
			},
		},
		Attachments: []Attachment{
			newAttachment(AttachmentKindTaskNotification, "子任务状态通知", "done", child.ID, nil),
		},
		CompactHistory: []RuntimeCompactMetadata{
			{
				Reason:         "query_turn",
				BeforeMessages: 10,
				AfterMessages:  7,
				BeforeChars:    5000,
				AfterChars:     3200,
				At:             time.Now(),
			},
		},
	}
	if err := store.SaveRuntimeSnapshot(snapshot); err != nil {
		t.Fatalf("save runtime snapshot: %v", err)
	}
	if err := store.WriteToMailbox(rootID, rootID, newMailboxEntry(rootID, rootID, child.ID, string(AttachmentKindTaskNotification), "子任务状态通知", "done", nil)); err != nil {
		t.Fatalf("write mailbox: %v", err)
	}

	bridge := &Bridge{
		cfg:        &Config{SessionDir: sessionDir},
		sessionMgr: sessionMgr,
		activeLLM: NewActiveLLMState(LLMConfig{
			Provider:    "test",
			Model:       "chat",
			ModelID:     "test-model",
			MaxTokens:   2048,
			Temperature: 0.1,
		}),
	}

	debugInfo := bridge.buildContextDebugInfo("wechat", "alice")
	containsAll := []string{
		"🧠 最新任务运行时",
		rootID,
		"attachments=1 (task_notification=1)",
		"mailbox(root=1,total=1)",
		"child_sessions: 1 (done=1)",
		"pending_tool_calls: ExecuteCode",
	}
	for _, needle := range containsAll {
		if !strings.Contains(debugInfo, needle) {
			t.Fatalf("context debug info missing %q:\n%s", needle, debugInfo)
		}
	}
}
