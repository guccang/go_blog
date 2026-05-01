package main

import (
	"testing"
	"time"
)

func TestShouldKeepSessionAlive(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		keepSession bool
		want        bool
	}{
		{
			name:        "one shot prompt closes by default",
			prompt:      "修复一个 bug",
			keepSession: false,
			want:        false,
		},
		{
			name:        "claude mode prompt keeps session",
			prompt:      "继续修改这个文件",
			keepSession: true,
			want:        true,
		},
		{
			name:        "empty prompt keeps idle session",
			prompt:      "",
			keepSession: false,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldKeepSessionAlive(tt.prompt, tt.keepSession); got != tt.want {
				t.Fatalf("shouldKeepSessionAlive(%q, %v) = %v, want %v", tt.prompt, tt.keepSession, got, tt.want)
			}
		})
	}
}

func TestPromptProgressTimeoutUsesAnalysisTimeout(t *testing.T) {
	agent := NewAgent("test", &AgentConfig{AnalysisTimeout: 1800})
	if got := agent.promptProgressTimeout(); got != 30*time.Minute {
		t.Fatalf("promptProgressTimeout() = %s, want %s", got, 30*time.Minute)
	}
}

func TestPromptProgressTimeoutFallsBackToThirtyMinutes(t *testing.T) {
	agent := NewAgent("test", &AgentConfig{})
	if got := agent.promptProgressTimeout(); got != 30*time.Minute {
		t.Fatalf("promptProgressTimeout() = %s, want %s", got, 30*time.Minute)
	}
}

func TestPromptProgressWarnAfterUsesHalfTimeout(t *testing.T) {
	agent := NewAgent("test", &AgentConfig{AnalysisTimeout: 1800})
	if got := agent.promptProgressWarnAfter(); got != 15*time.Minute {
		t.Fatalf("promptProgressWarnAfter() = %s, want %s", got, 15*time.Minute)
	}
}

func TestPromptProgressWarnAfterKeepsMinimumThreshold(t *testing.T) {
	agent := NewAgent("test", &AgentConfig{AnalysisTimeout: 60})
	if got := agent.promptProgressWarnAfter(); got != 45*time.Second {
		t.Fatalf("promptProgressWarnAfter() = %s, want %s", got, 45*time.Second)
	}
}
