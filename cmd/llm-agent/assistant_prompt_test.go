package main

import (
	"strings"
	"testing"

	"uap"
)

func newPromptTestBridge() *Bridge {
	return &Bridge{
		cfg: &Config{
			AgentID:            "llm-agent",
			AgentName:          "LLM Agent",
			SystemPromptPrefix: "你是测试助手。",
		},
		client:     &uap.Client{},
		activeLLM:  NewActiveLLMState(LLMConfig{Provider: "test", Model: "chat", ModelID: "test-model", MaxTokens: 4096, Temperature: 0.1}),
		memoryMgrs: make(map[string]*MemoryManager),
		agentInfo: map[string]AgentInfo{
			"deploy-agent": {
				ID:          "deploy-agent",
				Name:        "Deploy Agent",
				Description: "部署代理",
				Models:      []string{"default"},
			},
		},
		agentTools: map[string][]LLMTool{
			"deploy-agent": {
				{
					Type: "function",
					Function: LLMFunction{
						Name:        "DeployProject",
						Description: "部署项目",
					},
				},
			},
		},
	}
}

func hasPromptSection(sections []PromptSection, name string) bool {
	for _, sec := range sections {
		if sec.Name == name {
			return true
		}
	}
	return false
}

func TestBuildAssistantSystemPromptForGreetingKeepsStableToolingSections(t *testing.T) {
	bridge := newPromptTestBridge()

	prompt, sections := bridge.buildAssistantSystemPromptForQuery("alice", "你好", true)
	if !hasPromptSection(sections, "Agent能力") {
		t.Fatalf("greeting prompt should include stable Agent能力: %+v", sections)
	}
	if !hasPromptSection(sections, "工具目录") {
		t.Fatalf("greeting prompt should include stable 工具目录: %+v", sections)
	}
	if hasPromptSection(sections, "Git快照") {
		t.Fatalf("stable prompt should not include Git快照: %+v", sections)
	}
	if hasPromptSection(sections, "长期记忆") {
		t.Fatalf("stable prompt should not include 长期记忆 by default: %+v", sections)
	}
	if strings.Contains(prompt, "当前时间:") {
		t.Fatalf("stable prompt should not include current time: %s", prompt)
	}
	if !hasPromptSection(sections, "人设/基础") {
		t.Fatalf("greeting prompt should keep 人设/基础: %+v", sections)
	}
	if len(prompt) == 0 {
		t.Fatalf("expected greeting prompt to be non-empty")
	}
}

func TestBuildAssistantSystemPromptDoesNotDependOnQuery(t *testing.T) {
	bridge := newPromptTestBridge()

	greetingPrompt, greetingSections := bridge.buildAssistantSystemPromptForQuery("alice", "你好", true)
	toolPrompt, toolSections := bridge.buildAssistantSystemPromptForQuery("alice", "帮我部署 blog-agent", true)
	if greetingPrompt != toolPrompt {
		t.Fatalf("system prompt should be stable across query text")
	}
	if len(greetingSections) != len(toolSections) {
		t.Fatalf("section count should be stable: greeting=%+v tool=%+v", greetingSections, toolSections)
	}
	for i := range greetingSections {
		if greetingSections[i] != toolSections[i] {
			t.Fatalf("section[%d] should be stable: greeting=%+v tool=%+v", i, greetingSections[i], toolSections[i])
		}
	}
}

func TestBuildAssistantSystemPromptForToolTaskKeepsToolingSections(t *testing.T) {
	bridge := newPromptTestBridge()

	_, sections := bridge.buildAssistantSystemPromptForQuery("alice", "帮我部署 blog-agent", true)
	if !hasPromptSection(sections, "Agent能力") {
		t.Fatalf("tool task prompt should include Agent能力: %+v", sections)
	}
	if !hasPromptSection(sections, "工具目录") {
		t.Fatalf("tool task prompt should include 工具目录: %+v", sections)
	}
}

func TestBuildAssistantSystemPromptWithoutToolsSkipsToolingSections(t *testing.T) {
	bridge := newPromptTestBridge()

	_, sections := bridge.buildAssistantSystemPromptForQuery("alice", "请总结一下今天的工作", false)
	if hasPromptSection(sections, "Agent能力") {
		t.Fatalf("no-tools prompt should not include Agent能力: %+v", sections)
	}
	if hasPromptSection(sections, "工具目录") {
		t.Fatalf("no-tools prompt should not include 工具目录: %+v", sections)
	}
}
