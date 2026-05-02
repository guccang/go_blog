package main

import (
	"strings"
	"testing"
)

func TestUpdateCortanaCompanionStateKeepsGoalAndNextFocus(t *testing.T) {
	state := updateCortanaCompanionState(nil, "今天我想把周报写完，顺便理一下明天计划", "那我们先把周报结构列出来，再补明天的三件事。")
	if state == nil {
		t.Fatalf("expected state")
	}
	if !strings.Contains(state.CurrentGoal, "周报") {
		t.Fatalf("expected goal to mention 周报, got %q", state.CurrentGoal)
	}
	if !strings.Contains(state.NextFocus, "先把周报结构列出来") {
		t.Fatalf("expected next focus, got %q", state.NextFocus)
	}
	if state.Summary == "" {
		t.Fatalf("expected summary")
	}
}

func TestBuildCortanaCompanionPromptIncludesState(t *testing.T) {
	prompt := buildCortanaCompanionPrompt(&CortanaCompanionState{
		CurrentGoal:  "把周报写完",
		LastTopic:    "明天的安排",
		LastUserMood: "比较着急",
		NextFocus:    "先确定三件最重要的事",
		Summary:      "当前目标是把周报写完；下一步可接先确定三件最重要的事",
	})
	for _, want := range []string{"当前主线目标", "把周报写完", "最近话题", "下一步"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got %q", want, prompt)
		}
	}
}
