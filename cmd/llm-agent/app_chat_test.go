package main

import (
	"strings"
	"testing"
)

func TestExtractCortanaActionPlanFromTaggedBlock(t *testing.T) {
	input := `当然可以，我来帮你梳理一下。

[CORTANA_ACTION_PLAN]
{
  "speech_text": "当然可以，我来帮你梳理一下。",
  "expression": "happy",
  "fallback_expression": "happy",
  "expression_hold_ms": 1500,
  "actions": [
    {"motion": "IdleWave", "delay": 0, "index": 0},
    {"motion": "Idle", "delay": 1800, "hold_ms": 1200}
  ]
}`

	cleaned, plan := extractCortanaActionPlan(input)
	if cleaned != "当然可以，我来帮你梳理一下。" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if plan == nil {
		t.Fatalf("expected action plan to be extracted")
	}
	if got := plan["expression"]; got != "happy" {
		t.Fatalf("unexpected expression: %#v", got)
	}
	if got := plan["expression_hold_ms"]; got != float64(1500) {
		t.Fatalf("unexpected expression_hold_ms: %#v", got)
	}
	actions, ok := plan["actions"].([]any)
	if !ok || len(actions) != 2 {
		t.Fatalf("unexpected actions: %#v", plan["actions"])
	}
}

func TestExtractCortanaActionPlanFromJsonFence(t *testing.T) {
	input := "收到。\n\n```json\n{\"cortana_action_plan\":{\"speech_text\":\"收到。\",\"expression\":\"sad\",\"actions\":[{\"motion\":\"IdleAlt\",\"delay\":0}]}}\n```"

	cleaned, plan := extractCortanaActionPlan(input)
	if cleaned != "收到。" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if plan == nil {
		t.Fatalf("expected plan from fenced json")
	}
	if got := plan["speech_text"]; got != "收到。" {
		t.Fatalf("unexpected speech_text: %#v", got)
	}
	if got := plan["expression"]; got != "sad" {
		t.Fatalf("unexpected expression: %#v", got)
	}
}

func TestExtractCortanaActionPlanIgnoresInvalidPayload(t *testing.T) {
	input := "普通回复。\n\n[CORTANA_ACTION_PLAN]\n{\"unexpected\":true}"

	cleaned, plan := extractCortanaActionPlan(input)
	if cleaned != input {
		t.Fatalf("expected text to stay unchanged, got %q", cleaned)
	}
	if plan != nil {
		t.Fatalf("expected invalid plan to be ignored, got %#v", plan)
	}
}

func TestBuildCortanaOutputPromptContainsProtocol(t *testing.T) {
	prompt := buildCortanaOutputPrompt()
	expectedSnippets := []string{
		"[CORTANA_ACTION_PLAN]",
		"speech_text",
		"IdleWave",
		"IdleAlt",
		"surprised",
		"fallback_expression",
		"expression_hold_ms",
		"resume_to_idle",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("prompt missing %q: %s", snippet, prompt)
		}
	}
}
