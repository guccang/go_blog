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
	if got := normalizeTool("codex"); got != "codex" {
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

func TestBuildCodingStartCallForACPAddsSettings(t *testing.T) {
	agent := gatewayAgentSnapshot{
		Tools: []string{"AcpStartSession"},
		Meta: map[string]any{
			"coding_backend": "claude_acp",
		},
	}

	args, toolName := buildCodingStartCall(
		agent,
		"cmd-agent",
		"demo",
		"实现登录页",
		"",
		"claudecode",
		"minimax",
		false,
	)
	if toolName != "AcpStartSession" {
		t.Fatalf("unexpected tool name: %q", toolName)
	}
	extraArgs, ok := args["extra_args"].([]string)
	if !ok {
		t.Fatalf("expected []string extra_args, got %#v", args["extra_args"])
	}
	if len(extraArgs) != 2 || extraArgs[0] != "--settings" || extraArgs[1] != "minimax" {
		t.Fatalf("unexpected extra_args: %#v", extraArgs)
	}
}

func TestBuildCodingStartCallAddsResumeArgsByTool(t *testing.T) {
	agent := gatewayAgentSnapshot{Tools: []string{"AcpStartSession"}}

	codexArgs, _ := buildCodingStartCall(agent, "cmd-agent", "demo", "继续", "", "codex", "", true)
	codexExtra, ok := codexArgs["extra_args"].([]string)
	if !ok || len(codexExtra) != 1 || codexExtra[0] != "resume" {
		t.Fatalf("unexpected codex extra_args: %#v", codexArgs["extra_args"])
	}

	claudeArgs, _ := buildCodingStartCall(agent, "cmd-agent", "demo", "继续", "", "claudecode", "minimax", true)
	claudeExtra, ok := claudeArgs["extra_args"].([]string)
	if !ok || len(claudeExtra) != 3 || claudeExtra[0] != "--settings" || claudeExtra[1] != "minimax" || claudeExtra[2] != "-c" {
		t.Fatalf("unexpected claudecode extra_args: %#v", claudeArgs["extra_args"])
	}
}

func TestBuildDebugStartCallAddsBundleAndSettings(t *testing.T) {
	args := buildDebugStartCall(
		"cmd-agent",
		"flutter-client",
		"dbg_20260506_120000_ab12",
		"",
		"修复启动页白屏",
		"codex",
		"debug",
	)
	if args["debug_id"] != "dbg_20260506_120000_ab12" {
		t.Fatalf("unexpected debug_id: %#v", args["debug_id"])
	}
	if args["user_request"] != "修复启动页白屏" {
		t.Fatalf("unexpected user_request: %#v", args["user_request"])
	}
	if args["tool"] != "codex" {
		t.Fatalf("unexpected tool: %#v", args["tool"])
	}
	extraArgs, ok := args["extra_args"].([]string)
	if !ok {
		t.Fatalf("expected []string extra_args, got %#v", args["extra_args"])
	}
	if len(extraArgs) != 2 || extraArgs[0] != "--settings" || extraArgs[1] != "debug" {
		t.Fatalf("unexpected extra_args: %#v", extraArgs)
	}
}

func TestCodingToolsForACPAgentFallsBackToBackend(t *testing.T) {
	agent := gatewayAgentSnapshot{
		Tools: []string{"AcpStartSession"},
		Meta: map[string]any{
			"coding_backend": "codex_exec",
		},
	}
	tools := codingToolsForAgent(agent)
	if len(tools) != 1 || tools[0] != "codex" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}
