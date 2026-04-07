package codegen

import (
	"encoding/json"
	"strings"
	"testing"
)

type testDelegationToken struct {
	targetAccount string
}

func (t *testDelegationToken) GetTargetAccount() string {
	return t.targetAccount
}

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

func TestStripAndCacheDelegationTokenWithoutParser(t *testing.T) {
	oldParse := ParseDelegationTokenFromHeader
	oldSet := SetDelegationToken
	ParseDelegationTokenFromHeader = nil
	SetDelegationToken = nil
	defer func() {
		ParseDelegationTokenFromHeader = oldParse
		SetDelegationToken = oldSet
	}()

	bridge := &GatewayBridge{}
	got := bridge.stripAndCacheDelegationToken("ztt", "[delegation:abc]/cg agents")
	if got != "/cg agents" {
		t.Fatalf("unexpected content after stripping delegation token: %q", got)
	}
}

func TestStripAndCacheDelegationTokenCachesParsedToken(t *testing.T) {
	oldParse := ParseDelegationTokenFromHeader
	oldSet := SetDelegationToken
	defer func() {
		ParseDelegationTokenFromHeader = oldParse
		SetDelegationToken = oldSet
	}()

	var cachedKey string
	var cachedToken DelegationTokenHolder
	ParseDelegationTokenFromHeader = func(header string) (DelegationTokenHolder, error) {
		if header != "abc" {
			t.Fatalf("unexpected delegation header: %q", header)
		}
		return &testDelegationToken{targetAccount: "ztt"}, nil
	}
	SetDelegationToken = func(key string, token DelegationTokenHolder) {
		cachedKey = key
		cachedToken = token
	}

	bridge := &GatewayBridge{}
	got := bridge.stripAndCacheDelegationToken("ztt", "[delegation:abc]/cg agents")
	if got != "/cg agents" {
		t.Fatalf("unexpected content after stripping delegation token: %q", got)
	}
	if cachedKey != "ztt" {
		t.Fatalf("unexpected cached key: %q", cachedKey)
	}
	if cachedToken == nil {
		t.Fatalf("expected cached delegation token")
	}
}

func TestHandleWechatCommandAcceptsSlashCg(t *testing.T) {
	result := HandleWechatCommand("ztt", "/cg")
	if !strings.Contains(result, "cg agents") {
		t.Fatalf("expected help text to mention cg agents, got %q", result)
	}
}

func TestProjectListUnmarshalSupportsObjectArray(t *testing.T) {
	var projects ProjectList
	raw := []byte(`[{"name":"alpha"},{"name":"beta"},{"path":"/tmp/ignored"}]`)
	if err := json.Unmarshal(raw, &projects); err != nil {
		t.Fatalf("unmarshal ProjectList failed: %v", err)
	}
	if got, want := []string(projects), []string{"alpha", "beta"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected projects: %#v", got)
	}
}

func TestEnrichPayloadWithRouteAddsAccountAndChannel(t *testing.T) {
	raw := json.RawMessage(`{"session_id":"cg_1","status":"done"}`)
	enriched := enrichPayloadWithRoute(raw, SessionRoute{
		AgentID: "app-app-agent",
		Channel: "app",
		Account: "ztt",
	})

	var payload map[string]interface{}
	if err := json.Unmarshal(enriched, &payload); err != nil {
		t.Fatalf("unmarshal enriched payload failed: %v", err)
	}
	if payload["account"] != "ztt" {
		t.Fatalf("unexpected account: %#v", payload["account"])
	}
	if payload["channel"] != "app" {
		t.Fatalf("unexpected channel: %#v", payload["channel"])
	}
}

func TestSystemPromptBuilderOverride(t *testing.T) {
	oldClaude := ClaudeCodeSystemPromptBuilder
	oldOpenCode := OpenCodeSystemPromptBuilder
	ClaudeCodeSystemPromptBuilder = func() string { return "claude-override" }
	OpenCodeSystemPromptBuilder = func() string { return "opencode-override" }
	defer func() {
		ClaudeCodeSystemPromptBuilder = oldClaude
		OpenCodeSystemPromptBuilder = oldOpenCode
	}()

	if got := buildClaudeCodeSystemPrompt(); got != "claude-override" {
		t.Fatalf("unexpected claude prompt: %q", got)
	}
	if got := buildOpenCodeSystemPrompt(); got != "opencode-override" {
		t.Fatalf("unexpected opencode prompt: %q", got)
	}
}
