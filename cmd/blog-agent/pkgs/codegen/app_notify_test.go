package codegen

import (
	"strings"
	"testing"
)

func TestNormalizeCodegenCommand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "/cg", want: "cg"},
		{input: "/cg agents", want: "cg agents"},
		{input: " /cg   status  ", want: "cg status"},
		{input: "cg list", want: "cg list"},
		{input: "hello", want: "hello"},
	}

	for _, tt := range tests {
		if got := normalizeCodegenCommand(tt.input); got != tt.want {
			t.Fatalf("normalizeCodegenCommand(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildAppNotifyReplyNormalizesSlashCg(t *testing.T) {
	var receivedUser string
	var receivedContent string
	bridge := &GatewayBridge{
		wechatHandler: func(user, message string) string {
			receivedUser = user
			receivedContent = message
			return "ok:" + message
		},
	}

	reply, ok := bridge.buildAppNotifyReply("ztt", "/cg agents")
	if !ok {
		t.Fatalf("expected app notify reply to be generated")
	}
	if reply != "ok:cg agents" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if receivedUser != "ztt" {
		t.Fatalf("unexpected user: %q", receivedUser)
	}
	if receivedContent != "cg agents" {
		t.Fatalf("unexpected normalized content: %q", receivedContent)
	}
}

func TestHandleWechatCommandAcceptsSlashCg(t *testing.T) {
	result := HandleWechatCommand("ztt", "/cg")
	if !strings.Contains(result, "cg agents") {
		t.Fatalf("expected help text to mention cg agents, got %q", result)
	}
}
