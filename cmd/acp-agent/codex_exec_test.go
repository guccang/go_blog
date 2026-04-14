package main

import (
	"bytes"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCodexExecutionPlanAppliesSettingsAndTranslatesArgs(t *testing.T) {
	configDir := t.TempDir()
	settingsDir := filepath.Join(configDir, "settings", "codex")
	settingsPath := filepath.Join(settingsDir, "default.json")
	writeACPConfigForTest(t, settingsPath, map[string]any{
		"env": map[string]any{
			"OPENAI_API_KEY": "sk-test",
		},
		"model":               "gpt-5.4",
		"profile":             "ci",
		"sandbox_mode":        "workspace-write",
		"skip_git_repo_check": true,
		"config_home":         "./codex-home",
	})

	cfg := DefaultConfig()
	cfg.CodingBackend = BackendCodexExec
	cfg.CodexSettingsDir = settingsDir

	plan, err := buildCodexExecutionPlan(cfg, []string{
		"--settings", "default",
		"--dangerously-skip-permissions",
		"--max-turns", "20",
		"--model", "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("buildCodexExecutionPlan failed: %v", err)
	}

	if !hasOrderedArgs(plan.Args, "exec", "--json") {
		t.Fatalf("expected exec/json defaults in args: %#v", plan.Args)
	}
	if got := extractFlagValue(plan.Args, "--profile"); got != "ci" {
		t.Fatalf("unexpected profile: %q", got)
	}
	if got := extractFlagValue(plan.Args, "--sandbox"); got != "workspace-write" {
		t.Fatalf("unexpected sandbox mode: %q", got)
	}
	if got := extractFlagValue(plan.Args, "--model"); got != "gpt-5.4-mini" {
		t.Fatalf("unexpected model override: %q", got)
	}
	if plan.Model != "gpt-5.4-mini" {
		t.Fatalf("unexpected plan model: %q", plan.Model)
	}
	if !containsArg(plan.Args, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("expected translated dangerous flag in args: %#v", plan.Args)
	}
	if !containsArg(plan.Args, "--skip-git-repo-check") {
		t.Fatalf("expected skip-git-repo-check in args: %#v", plan.Args)
	}
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "--max-turns") {
		t.Fatalf("unexpected warnings: %#v", plan.Warnings)
	}
	if !containsEnv(plan.Env, "OPENAI_API_KEY=sk-test") {
		t.Fatalf("expected OPENAI_API_KEY env in plan")
	}
	if !containsEnv(plan.Env, "CODEX_HOME="+filepath.Join(settingsDir, "codex-home")) {
		t.Fatalf("expected CODEX_HOME env in plan")
	}
}

func TestBuildCodexExecutionPlanSkipsMissingDefaultSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CodingBackend = BackendCodexExec
	cfg.CodexSettingsDir = t.TempDir()
	cfg.DefaultSettings = "minimax"

	plan, err := buildCodexExecutionPlan(cfg, nil)
	if err != nil {
		t.Fatalf("buildCodexExecutionPlan should skip missing default settings, got: %v", err)
	}
	if !hasOrderedArgs(plan.Args, "exec", "--json") {
		t.Fatalf("expected exec/json defaults in args: %#v", plan.Args)
	}
}

func TestBuildCodexStartInfoContainsCoreFields(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CodingBackend = BackendCodexExec

	info := buildCodexStartInfo(
		"acp_123",
		"demo",
		"/tmp/demo",
		"请修复登录失败问题",
		false,
		cfg,
		codexExecutionPlan{Model: "gpt-5.4", Settings: "/tmp/settings/codex/default.json"},
		[]string{"exec", "--json", "-"},
	)

	for _, want := range []string{
		"会话: acp_123",
		"后端: Codex",
		"项目: demo",
		"项目路径: /tmp/demo",
		"模型: gpt-5.4",
		"Settings: /tmp/settings/codex/default.json",
		"Prompt长度:",
		"命令: codex exec --json -",
	} {
		if !strings.Contains(info, want) {
			t.Fatalf("expected start info to contain %q, got:\n%s", want, info)
		}
	}
}

func TestCodexRunStateConsumesKeyEvents(t *testing.T) {
	state := newCodexRunState()
	var events []StreamEvent
	emit := func(evt StreamEvent) {
		events = append(events, evt)
	}

	lines := []map[string]any{
		{
			"type": "item.completed",
			"item": map[string]any{"id": "todo_1", "type": "todo_list", "items": []map[string]any{{"text": "plan", "completed": false}}},
		},
		{
			"type": "item.completed",
			"item": map[string]any{"id": "file_1", "type": "file_change", "status": "completed", "changes": []map[string]any{{"path": "a.go", "kind": "add"}, {"path": "b.go", "kind": "update"}}},
		},
		{
			"type": "item.completed",
			"item": map[string]any{"id": "msg_1", "type": "agent_message", "text": "done"},
		},
	}

	for _, payload := range lines {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if err := state.handleJSONLine(string(raw), emit); err != nil {
			t.Fatalf("handleJSONLine failed: %v", err)
		}
	}

	if state.summary() != "done" {
		t.Fatalf("unexpected summary: %q", state.summary())
	}
	if !state.filesWritten["a.go"] || !state.filesEdited["b.go"] {
		t.Fatalf("unexpected file change state: written=%#v edited=%#v", state.filesWritten, state.filesEdited)
	}
	if len(events) < 3 {
		t.Fatalf("expected stream events, got %d", len(events))
	}
}

func TestLogCodexStreamEventIncludesToolAndPreview(t *testing.T) {
	var buf bytes.Buffer
	prevWriter := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prevWriter)

	logCodexStreamEvent("acp_123", StreamEvent{
		Type:     "tool_update",
		ToolName: "rg",
		Text:     "search completed",
	})

	got := buf.String()
	if !strings.Contains(got, "[Codex][acp_123][工具进度]") {
		t.Fatalf("expected session/type in log, got %q", got)
	}
	if !strings.Contains(got, "rg | search completed") {
		t.Fatalf("expected tool and text in log, got %q", got)
	}
}

func hasOrderedArgs(args []string, first, second string) bool {
	firstIdx := -1
	secondIdx := -1
	for i, arg := range args {
		if arg == first && firstIdx == -1 {
			firstIdx = i
		}
		if arg == second && secondIdx == -1 {
			secondIdx = i
		}
	}
	return firstIdx >= 0 && secondIdx > firstIdx
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
