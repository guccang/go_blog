package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInterpretToolResultTreatsStructuredFailureAsFailure(t *testing.T) {
	ok, errText := interpretToolResult(`{"success":false,"session_id":"acp_1","error":"default settings file not found"}`)
	if ok {
		t.Fatalf("expected structured failure to be treated as failure")
	}
	if errText != "default settings file not found" {
		t.Fatalf("unexpected error text: %q", errText)
	}
}

func TestInterpretToolResultKeepsStructuredSuccess(t *testing.T) {
	ok, errText := interpretToolResult(`{"success":true,"message":"done","data":{"session_id":"acp_1"}}`)
	if !ok {
		t.Fatalf("expected structured success, got error=%q", errText)
	}
}

func TestInterpretToolResultIgnoresPlainText(t *testing.T) {
	ok, errText := interpretToolResult("plain text output")
	if !ok || errText != "" {
		t.Fatalf("expected plain text output to pass through, got ok=%v err=%q", ok, errText)
	}
}

func TestResolveDebugBundlePathFindsNestedFlutterProjectBundle(t *testing.T) {
	workspace := t.TempDir()
	projectDir := filepath.Join(workspace, "flutter-client-for-appagent")
	bundleDir := filepath.Join(projectDir, "flutter_client_for_appagent", ".debug", "flutter", "dbg_20260506_090307_40ca04")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("mkdir bundle dir: %v", err)
	}

	conn := testConnection(workspace)
	got, err := conn.resolveDebugBundlePath("flutter-client-for-appagent", map[string]interface{}{
		"debug_id": "dbg_20260506_090307_40ca04",
	})
	if err != nil {
		t.Fatalf("resolveDebugBundlePath returned error: %v", err)
	}
	if got != bundleDir {
		t.Fatalf("resolveDebugBundlePath = %q, want %q", got, bundleDir)
	}
}

func TestToolListDebugBundlesIncludesNestedFlutterProjectBundle(t *testing.T) {
	workspace := t.TempDir()
	projectDir := filepath.Join(workspace, "flutter-client-for-appagent")
	bundleDir := filepath.Join(projectDir, "flutter_client_for_appagent", ".debug", "flutter", "dbg_20260506_090307_40ca04")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("mkdir bundle dir: %v", err)
	}

	conn := testConnection(workspace)
	got := conn.toolListDebugBundles(map[string]interface{}{
		"project": "flutter-client-for-appagent",
	})
	if !strings.Contains(got, "dbg_20260506_090307_40ca04") {
		t.Fatalf("toolListDebugBundles output does not include nested bundle: %s", got)
	}
}

func testConnection(workspace string) *Connection {
	cfg := &AgentConfig{Workspaces: []string{workspace}}
	agent := NewAgent("test-agent", cfg)
	return &Connection{cfg: cfg, agent: agent}
}
