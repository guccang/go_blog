package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCortanaProfileDefaults(t *testing.T) {
	profile := loadCortanaProfile(t.TempDir(), "alice")
	if profile == nil {
		t.Fatalf("expected default profile")
	}
	if profile.Name != "Cortana" {
		t.Fatalf("profile.Name=%q want Cortana", profile.Name)
	}
}

func TestBuildCortanaAppSystemPromptUsesCortanaPersona(t *testing.T) {
	workspace := t.TempDir()
	accountDir := filepath.Join(workspace, "users", "alice", "memory")
	if err := os.MkdirAll(accountDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(accountDir, "cortana_profile.json"),
		[]byte("{\"name\":\"Cortana Prime\",\"owner_title\":\"主人\",\"description\":\"语气冷静、连续接话、把自己当成用户唯一数字助手\"}\n"),
		0644,
	); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	bridge := &Bridge{
		cfg:        &Config{WorkspaceDir: workspace},
		activeLLM:  NewActiveLLMState(LLMConfig{ModelID: "test", MaxTokens: 2048}),
		memoryMgrs: make(map[string]*MemoryManager),
	}

	profile := loadCortanaProfile(workspace, "alice")
	prompt, _ := bridge.buildCortanaAppSystemPrompt("alice", "你好", profile, nil)

	if !strings.Contains(prompt, "你是 Cortana Prime") {
		t.Fatalf("expected Cortana persona in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "固定称呼是「主人」") {
		t.Fatalf("expected owner title instruction in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "`account` 仅用于权限、工作区和数据隔离标识") {
		t.Fatalf("expected account boundary in prompt, got: %s", prompt)
	}
	if strings.Contains(prompt, "你是一个可执行任务的工程型智能体，不是陪聊助手") {
		t.Fatalf("unexpected legacy engineering persona in prompt: %s", prompt)
	}
}

func TestLoadCortanaProfileInfersOwnerTitleFromDescription(t *testing.T) {
	workspace := t.TempDir()
	accountDir := filepath.Join(workspace, "users", "alice", "memory")
	if err := os.MkdirAll(accountDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(accountDir, "cortana_profile.json"),
		[]byte("{\"name\":\"Cortana Prime\",\"description\":\"语气冷静，称呼我为主人，不要叫我账号名。\"}\n"),
		0644,
	); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	profile := loadCortanaProfile(workspace, "alice")
	if profile.OwnerTitle != "主人" {
		t.Fatalf("OwnerTitle=%q want 主人", profile.OwnerTitle)
	}
}
