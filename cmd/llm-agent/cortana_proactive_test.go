package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCortanaProactiveSystemPromptIncludesProfile(t *testing.T) {
	prompt := buildCortanaProactiveSystemPrompt(&CortanaProfile{
		Name:        "Cortana Prime",
		OwnerTitle:  "主人",
		Description: "冷静、主动、会接住用户情绪",
	})

	for _, want := range []string{
		"当前 Cortana 名称: Cortana Prime",
		"对用户固定称呼: 主人",
		"冷静、主动、会接住用户情绪",
		"语气、人设和称呼必须与以上 Cortana 设定保持一致",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got: %s", want, prompt)
		}
	}
}

func TestBuildCortanaProactiveSystemPromptIncludesLocalTimeContext(t *testing.T) {
	prompt := buildCortanaProactiveSystemPrompt(&CortanaProfile{Name: "Cortana"}, &CortanaProactivePayload{
		Snapshot: map[string]any{
			"local_datetime":  "2026-05-07 21:30:00",
			"weekday":         "星期四",
			"timezone":        "CST",
			"timezone_offset": "+08:00",
			"collected_at":    int64(1778151000000),
			"device_context": map[string]any{
				"location": map[string]any{"available": true},
			},
		},
	})

	for _, want := range []string{
		"当前本地时间上下文",
		"2026-05-07 21:30:00",
		"+08:00",
		"判断当前时段时优先级最高",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got: %s", want, prompt)
		}
	}
}

func TestCortanaCurrentTimeQueryShortCircuitHelpers(t *testing.T) {
	for _, input := range []string{"现在几点了", "你是不是又把时间当成凌晨了", "what time is it"} {
		if !isCortanaCurrentTimeQuery(input) {
			t.Fatalf("expected %q to be treated as current time query", input)
		}
	}
	if isCortanaCurrentTimeQuery("继续刚才的话题") {
		t.Fatalf("expected unrelated text not to be treated as current time query")
	}
}

func TestRecordCortanaProactiveContextUpdatesStateWithoutChangingHistory(t *testing.T) {
	tmp := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceDir = filepath.Join(tmp, "workspace")
	cfg.ChatSessionDir = filepath.Join(tmp, "chat_sessions")
	cfg.SessionDir = filepath.Join(tmp, "sessions")
	cfg.MemoryDir = filepath.Join(tmp, "memory")

	bridge := NewBridge(cfg)
	bridge.recordCortanaProactiveContext(&CortanaProactivePayload{
		Account:       "alice",
		TriggerReason: "monitor_cycle",
	}, &CortanaProactiveDecision{
		ShouldInteract: true,
		SpeechText:     "我刚看到一段很适合今晚听的历史，要不要听？",
		Expression:     "happy",
	})

	session := bridge.sessionMgr.Get("app", "alice")
	if session == nil {
		t.Fatalf("expected app session to be created")
	}
	if len(session.Messages) != 0 {
		t.Fatalf("proactive context should not write app history, got %#v", session.Messages)
	}
	if session.CortanaState == nil || !strings.Contains(session.CortanaState.LastAssistantMsg, "历史") {
		t.Fatalf("expected cortana state to record proactive speech, got %#v", session.CortanaState)
	}

	loaded, err := bridge.sessionMgr.LoadSession("app_alice")
	if err != nil {
		t.Fatalf("expected persisted app session: %v", err)
	}
	if len(loaded.Messages) != 0 {
		t.Fatalf("expected persisted app history to stay empty, got %#v", loaded.Messages)
	}
	if loaded.CortanaState == nil || !strings.Contains(loaded.CortanaState.LastAssistantMsg, "要不要听") {
		t.Fatalf("expected persisted cortana state, got %#v", loaded.CortanaState)
	}
}
