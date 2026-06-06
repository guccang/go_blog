package main

import "testing"

func TestBuildCodegenActionCommandStart(t *testing.T) {
	command, err := buildCodegenActionCommand(codegenActionRequest{
		Version:    1,
		Kind:       "start",
		Project:    "go_blog",
		Agent:      "mac",
		Tool:       "codex",
		Settings:   "default",
		Prompt:     "修复登录问题",
		Resume:     true,
		AutoDeploy: true,
	})
	if err != nil {
		t.Fatalf("buildCodegenActionCommand() error = %v", err)
	}
	want := "/cg start go_blog@mac @codex --settings default !resume !deploy 修复登录问题"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuildCodegenActionCommandDebug(t *testing.T) {
	command, err := buildCodegenActionCommand(codegenActionRequest{
		Kind:      "debug",
		Project:   "flutter_app",
		Prompt:    "修复崩溃",
		DebugID:   "dbg_1",
		DebugPath: "/tmp/dbg_1",
	})
	if err != nil {
		t.Fatalf("buildCodegenActionCommand() error = %v", err)
	}
	want := "/cg debug flutter_app --debug-id dbg_1 --debug-path /tmp/dbg_1 修复崩溃"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuildCodegenActionCommandDeploy(t *testing.T) {
	command, err := buildCodegenActionCommand(codegenActionRequest{
		Kind:         "deploy",
		Project:      "app",
		Agent:        "linux",
		DeployTarget: "prod",
		PackOnly:     true,
		ExtraArgs:    []string{"--version 1.2.3 --desc release"},
	})
	if err != nil {
		t.Fatalf("buildCodegenActionCommand() error = %v", err)
	}
	want := "/cg deploy app@linux #prod !pack --version 1.2.3 --desc release"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}

func TestBuildCodegenActionCommandRejectsInvalidRequest(t *testing.T) {
	for _, req := range []codegenActionRequest{
		{Version: 2, Kind: "start", Project: "app", Prompt: "x"},
		{Kind: "start", Project: "app"},
		{Kind: "unknown", Project: "app"},
		{Kind: "start", Project: "bad project", Prompt: "x"},
	} {
		if _, err := buildCodegenActionCommand(req); err == nil {
			t.Fatalf("buildCodegenActionCommand(%+v) expected error", req)
		}
	}
}
