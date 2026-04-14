package main

import "testing"

func TestCodegenThrottleKeyIncludesEventContent(t *testing.T) {
	key1 := codegenThrottleKey("sess_1", "thought", "先检查日志", "")
	key2 := codegenThrottleKey("sess_1", "thought", "再查看路由", "")
	if key1 == key2 {
		t.Fatalf("expected different keys for different thought content, got %q", key1)
	}
}

func TestCodegenEventSignatureNormalizesWhitespace(t *testing.T) {
	got1 := codegenEventSignature("tool_update", "  step   one \n done  ", "bash")
	got2 := codegenEventSignature("tool_update", "step one done", "bash")
	if got1 != got2 {
		t.Fatalf("expected normalized signatures to match, got %q != %q", got1, got2)
	}
}
