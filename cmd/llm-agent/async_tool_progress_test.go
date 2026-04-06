package main

import "testing"

func TestExtractAsyncToolSessionID(t *testing.T) {
	raw := `{"success":true,"accepted":true,"session_id":"deploy_123","status":"queued"}`
	if got := extractAsyncToolSessionID(raw); got != "deploy_123" {
		t.Fatalf("expected deploy_123, got %q", got)
	}

	if got := extractAsyncToolSessionID(`{"accepted":false,"session_id":"deploy_456","status":"queued"}`); got != "" {
		t.Fatalf("expected empty session for non-accepted result, got %q", got)
	}

	if got := extractAsyncToolSessionID(`{"accepted":true,"session_id":"deploy_789","status":"done"}`); got != "" {
		t.Fatalf("expected empty session for done result, got %q", got)
	}
}

func TestIsTerminalAsyncToolProgress(t *testing.T) {
	if !isTerminalAsyncToolProgress("✅ 部署项目 foo 完成") {
		t.Fatalf("expected success progress to be terminal")
	}
	if !isTerminalAsyncToolProgress("❌ 部署失败: timeout") {
		t.Fatalf("expected error progress to be terminal")
	}
	if isTerminalAsyncToolProgress("📦 开始部署项目 [foo]...") {
		t.Fatalf("expected intermediate progress to be non-terminal")
	}
}
