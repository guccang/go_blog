package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildStatusInfo(t *testing.T) {
	sessionMgr := NewChatSessionManager(time.Hour, 40, 15, "")
	session, _ := sessionMgr.GetOrCreate("wechat", "alice", "alice")
	session.LastActiveAt = time.Date(2026, 5, 2, 14, 30, 0, 0, time.Local)
	session.TurnCount = 3
	session.Messages = []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
	}
	session.ClaudeMode = true
	session.ClaudeCurrentMode = "code"

	bridge := &Bridge{
		sessionMgr: sessionMgr,
		agentInfo: map[string]AgentInfo{
			"test-agent-1": {
				ID: "test-agent-1", Name: "Test Agent", Description: "A test agent",
				ToolNames: []string{"bash", "read"}, HostPlatform: "macOS", HostIP: "10.0.0.1",
			},
			"deploy-agent": {
				ID: "deploy-agent", Name: "Deploy Agent", Description: "Handles deployments",
				ToolNames: []string{"deploy", "status"}, HostPlatform: "linux",
			},
		},
		activeLLM: NewActiveLLMState(LLMConfig{
			Provider:    "deepseek",
			Model:       "deepseek-chat",
			ModelID:     "deepseek-v4-pro",
			MaxTokens:   131072,
			Temperature: 0.50,
		}),
	}

	statusInfo := bridge.buildStatusInfo("wechat", "alice")

	// 验证包含关键信息
	checks := []string{
		"📊 Status",
		"当前模型: deepseek/deepseek-chat",
		"deepseek-v4-pro",
		"max_tokens=131072",
		"temperature=0.50",
		"活跃会话",
		"当前会话:",
		"轮次: 3/15",
		"消息: 2",
		"claude/code",
		// Agent 状态
		"Agent 状态",
		"总数: 2",
		"Test Agent (test-agent-1)",
		"A test agent",
		"工具数: 2 | 平台: macOS | IP: 10.0.0.1",
		"Deploy Agent (deploy-agent)",
		"Handles deployments",
		"工具数: 2 | 平台: linux",
	}
	for _, needle := range checks {
		if !strings.Contains(statusInfo, needle) {
			t.Fatalf("status info missing %q:\n%s", needle, statusInfo)
		}
	}
}

func TestBuildStatusInfoNoSession(t *testing.T) {
	sessionMgr := NewChatSessionManager(time.Hour, 40, 15, "")

	bridge := &Bridge{
		sessionMgr: sessionMgr,
		activeLLM: NewActiveLLMState(LLMConfig{
			Provider:    "test",
			Model:       "chat",
			ModelID:     "test-model",
			MaxTokens:   2048,
			Temperature: 0.10,
		}),
	}

	statusInfo := bridge.buildStatusInfo("wechat", "alice")

	if !strings.Contains(statusInfo, "无活跃会话") {
		t.Fatalf("expected '无活跃会话' for no session, got:\n%s", statusInfo)
	}
	if !strings.Contains(statusInfo, "无已注册 Agent") {
		t.Fatalf("expected '无已注册 Agent' for no agents, got:\n%s", statusInfo)
	}
}
