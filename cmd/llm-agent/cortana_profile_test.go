package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uap"
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
	if !strings.Contains(prompt, "已经签注完成") {
		t.Fatalf("expected completed residence permit instruction in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "除非本轮确实调用工具并拿到结果") {
		t.Fatalf("expected anti-hallucinated-task-check instruction in prompt, got: %s", prompt)
	}
	if strings.Contains(prompt, "你是一个可执行任务的工程型智能体，不是陪聊助手") {
		t.Fatalf("unexpected legacy engineering persona in prompt: %s", prompt)
	}
}

func TestBuildCortanaAppSystemPromptSkipsToolsForCompanionContinuation(t *testing.T) {
	bridge := &Bridge{
		cfg:        &Config{AgentName: "LLM Agent", AgentID: "llm-agent"},
		client:     &uap.Client{},
		activeLLM:  NewActiveLLMState(LLMConfig{ModelID: "test", MaxTokens: 2048}),
		memoryMgrs: make(map[string]*MemoryManager),
		agentInfo: map[string]AgentInfo{
			"blog-agent": {ID: "blog-agent", Name: "Go Blog Server", Description: "博客CRUD"},
		},
		agentTools: map[string][]LLMTool{
			"blog-agent": {
				{Type: "function", Function: LLMFunction{Name: "RawAddTodo", Description: "添加待办"}},
			},
		},
	}

	prompt, sections := bridge.buildCortanaAppSystemPrompt("alice", "已完成了", defaultCortanaProfile(), nil)
	if hasPromptSection(sections, "Agent能力") || hasPromptSection(sections, "工具目录") {
		t.Fatalf("companion continuation should skip tooling sections: %+v", sections)
	}
	if strings.Contains(prompt, "RawAddTodo") {
		t.Fatalf("tool catalog leaked into companion prompt: %s", prompt)
	}

	toolPrompt, toolSections := bridge.buildCortanaAppSystemPrompt("alice", "添加一个待办", defaultCortanaProfile(), nil)
	if !hasPromptSection(toolSections, "Agent能力") || !hasPromptSection(toolSections, "工具目录") {
		t.Fatalf("actionable query should keep tooling sections: %+v", toolSections)
	}
	if !strings.Contains(toolPrompt, "RawAddTodo") {
		t.Fatalf("tool catalog missing from actionable prompt: %s", toolPrompt)
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
