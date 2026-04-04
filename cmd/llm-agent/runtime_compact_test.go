package main

import (
	"strings"
	"testing"
)

func TestCompactRuntimeMessagesTrimsToolResults(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "user"},
		{Role: "tool", Content: strings.Repeat("a", 4000)},
		{Role: "assistant", Content: "next"},
		{Role: "tool", Content: strings.Repeat("b", 4000)},
	}

	cfg := RuntimeCompactConfig{
		MaxMessages:      10,
		MaxChars:         10000,
		TriggerMessages:  2,
		TriggerChars:     2000,
		ToolResultBudget: 1200,
		RecentToolKeep:   1,
	}
	compacted, meta := compactRuntimeMessages(messages, cfg, 0, "test")
	if meta == nil {
		t.Fatalf("expected compact metadata")
	}
	if len(compacted) != len(messages) {
		t.Fatalf("expected tool result trimming without dropping messages, got %d", len(compacted))
	}
	if !strings.Contains(compacted[2].Content, "上下文压缩") && !strings.Contains(compacted[4].Content, "上下文压缩") {
		t.Fatalf("expected tool results to be compacted")
	}
}

func TestSanitizeMessagesWithBudgetKeepsSystemAndTail(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
	}

	compacted := sanitizeMessagesWithBudget(messages, 3, 1024)
	if len(compacted) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(compacted))
	}
	if compacted[0].Role != "system" {
		t.Fatalf("expected first message to remain system")
	}
	if compacted[1].Content != "u2" || compacted[2].Content != "a2" {
		t.Fatalf("expected tail messages preserved, got %#v", compacted)
	}
}
