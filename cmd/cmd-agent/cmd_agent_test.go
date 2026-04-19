package main

import "testing"

func TestUnwrapInboundCommandAppMessageJSON(t *testing.T) {
	user, content := unwrapInboundCommand(inboundNotify{
		Channel: "app",
		To:      "ztt",
		Content: "APP_MESSAGE_JSON:\n{\n  \"content\": \"/cg agents\",\n  \"kind\": \"app_message\",\n  \"message_type\": \"text\",\n  \"scope\": \"direct\",\n  \"user_id\": \"ztt\"\n}",
	})
	if user != "ztt" {
		t.Fatalf("unexpected user: %q", user)
	}
	if content != "/cg agents" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestUnwrapInboundCommandStripsDelegationPrefix(t *testing.T) {
	user, content := unwrapInboundCommand(inboundNotify{
		Channel: "app",
		To:      "ztt",
		Content: "[delegation:abc]APP_MESSAGE_JSON:\n{\n  \"content\": \"/cg agents\",\n  \"kind\": \"app_message\",\n  \"message_type\": \"text\",\n  \"scope\": \"direct\",\n  \"user_id\": \"ztt\"\n}",
	})
	if user != "ztt" {
		t.Fatalf("unexpected user: %q", user)
	}
	if content != "/cg agents" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestNormalizeCodegenCommand(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/cg", "cg"},
		{"/cg agents", "cg agents"},
		{" cg tools ", "cg tools"},
	}
	for _, tt := range tests {
		if got := normalizeCodegenCommand(tt.in); got != tt.want {
			t.Fatalf("normalizeCodegenCommand(%q)=%q want=%q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeTool(t *testing.T) {
	if got := normalizeTool("oc"); got != "opencode" {
		t.Fatalf("unexpected tool normalize result: %q", got)
	}
	if got := normalizeTool("claude"); got != "claudecode" {
		t.Fatalf("unexpected tool normalize result: %q", got)
	}
}

func TestSupportsCodingAgentIncludesACP(t *testing.T) {
	if !supportsCodingAgent(gatewayAgentSnapshot{Tools: []string{"AcpStartSession"}}) {
		t.Fatalf("expected ACP agent to be recognized as coding agent")
	}
}

func TestSupportsCreateProjectIncludesACP(t *testing.T) {
	if supportsCreateProject(gatewayAgentSnapshot{Tools: []string{"AcpCreateProject"}}) {
		t.Fatalf("expected create-project support to be disabled")
	}
}

func TestCreateProjectToolNameDisabled(t *testing.T) {
	got := createProjectToolName(gatewayAgentSnapshot{Tools: []string{"AcpCreateProject"}})
	if got != "" {
		t.Fatalf("createProjectToolName()=%q want empty", got)
	}
}

func TestParseDeployCommandOptions(t *testing.T) {
	opts, err := parseDeployCommandOptions("#upload !pack --version 1.2.3 --desc release-123 --private-key-path /tmp/key --project-path /tmp/project")
	if err != nil {
		t.Fatalf("parseDeployCommandOptions failed: %v", err)
	}
	if opts.Target != "upload" || !opts.PackOnly {
		t.Fatalf("unexpected target/packOnly: %#v", opts)
	}
	if opts.Version != "1.2.3" || opts.Desc != "release-123" {
		t.Fatalf("unexpected version/desc: %#v", opts)
	}
	if opts.PrivateKeyPath != "/tmp/key" || opts.ProjectPath != "/tmp/project" {
		t.Fatalf("unexpected path options: %#v", opts)
	}
}

func TestParseDeployCommandOptionsRejectsUnknownFlag(t *testing.T) {
	_, err := parseDeployCommandOptions("--unknown value")
	if err == nil {
		t.Fatalf("expected unknown flag to fail")
	}
}

func TestParseDeployCommandOptionsSupportsShortFlags(t *testing.T) {
	opts, err := parseDeployCommandOptions("-v 2.0.1 -d release-note")
	if err != nil {
		t.Fatalf("parseDeployCommandOptions failed: %v", err)
	}
	if opts.Version != "2.0.1" || opts.Desc != "release-note" {
		t.Fatalf("unexpected short flag parse result: %#v", opts)
	}
}

func TestFindDeployProjectInfoMatchesAlias(t *testing.T) {
	items := []deployProjectInfo{
		{
			Name:      "build-flutter-apk",
			Aliases:   []string{"flutter-apk"},
			BuildOnly: true,
		},
	}

	got := findDeployProjectInfo(items, "flutter-apk")
	if got == nil {
		t.Fatalf("expected alias match")
	}
	if got.Name != "build-flutter-apk" || !got.BuildOnly {
		t.Fatalf("unexpected project info: %#v", got)
	}
}
