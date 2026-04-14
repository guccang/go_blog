package main

import "testing"

func TestCodegenThrottleKeyIncludesToolName(t *testing.T) {
	key1 := codegenThrottleKey("sess_1", "tool", "", "rg")
	key2 := codegenThrottleKey("sess_1", "tool", "", "go test")
	if key1 == key2 {
		t.Fatalf("expected different keys for different tools, got %q", key1)
	}
}

func TestCodegenEventSignatureNormalizesWhitespace(t *testing.T) {
	got1 := codegenEventSignature("assistant", "  first   chunk \n second  ", "")
	got2 := codegenEventSignature("assistant", "first chunk second", "")
	if got1 != got2 {
		t.Fatalf("expected normalized signatures to match, got %q != %q", got1, got2)
	}
}
