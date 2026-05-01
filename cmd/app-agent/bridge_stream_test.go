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

func TestMergeCodegenStreamTextAppendsNewLines(t *testing.T) {
	got := mergeCodegenStreamText("[system] start", "[assistant] step 1")
	want := "[system] start\n[assistant] step 1"
	if got != want {
		t.Fatalf("mergeCodegenStreamText()=%q want=%q", got, want)
	}
}

func TestMergeCodegenStreamTextSamePrefixMergesContent(t *testing.T) {
	// 相同前缀的连续 chunk 应该直接拼接内容，不换行、不重复前缀。
	got := mergeCodegenStreamText(
		"[assistant][acp_1777] 你好",
		"[assistant][acp_1777] ！我是 Claude",
	)
	want := "[assistant][acp_1777] 你好！我是 Claude"
	if got != want {
		t.Fatalf("mergeCodegenStreamText()=%q want=%q", got, want)
	}
}

func TestMergeCodegenStreamTextMergesAgainstLastLine(t *testing.T) {
	got := mergeCodegenStreamText(
		"[system][acp_1777] start\n[assistant][acp_1777] 你好",
		"[assistant][acp_1777] ！我是 Claude",
	)
	want := "[system][acp_1777] start\n[assistant][acp_1777] 你好！我是 Claude"
	if got != want {
		t.Fatalf("mergeCodegenStreamText()=%q want=%q", got, want)
	}
}

func TestMergeCodegenStreamTextSamePrefixWithThought(t *testing.T) {
	got := mergeCodegenStreamText(
		"[thought][acp_1777] The user",
		"[thought][acp_1777] is asking",
	)
	want := "[thought][acp_1777] The user is asking"
	if got != want {
		t.Fatalf("mergeCodegenStreamText()=%q want=%q", got, want)
	}
}

func TestMergeCodegenStreamTextSingleLineExtends(t *testing.T) {
	// 第一行没有前缀（普通文本），第二行带前缀
	got := mergeCodegenStreamText("普通消息", "[assistant][x] 回复")
	want := "普通消息\n[assistant][x] 回复"
	if got != want {
		t.Fatalf("mergeCodegenStreamText()=%q want=%q", got, want)
	}
}

func TestCodegenThrottleKeyStreamingTextMerged(t *testing.T) {
	// assistant/thought 不再共享节流 key，避免吞掉流式 chunk。
	key1 := codegenThrottleKey("sess_1", "assistant", "hello", "")
	key2 := codegenThrottleKey("sess_1", "assistant", "world", "")
	if key1 == key2 {
		t.Fatalf("expected different assistant throttle keys, got %q", key1)
	}
	key3 := codegenThrottleKey("sess_1", "thought", "thinking A", "")
	key4 := codegenThrottleKey("sess_1", "thought", "thinking B", "")
	if key3 == key4 {
		t.Fatalf("expected different thought throttle keys, got %q", key3)
	}
}

func TestFormatEventForAppThinkingMapsToThought(t *testing.T) {
	payload := &codegenStreamEvent{SessionID: "acp_1777"}
	payload.Event.Type = "thought"
	payload.Event.Text = "Claudeby"
	got := formatEventForApp(payload)
	want := "[thought][acp_1777] Claudeby"
	if got != want {
		t.Fatalf("formatEventForApp()=%q want=%q", got, want)
	}
}

func TestSplitCodegenPrefix(t *testing.T) {
	prefix, content := splitCodegenPrefix("[assistant][acp_1777] 你好世界")
	if prefix != "[assistant][acp_1777]" {
		t.Fatalf("prefix=%q want=%q", prefix, "[assistant][acp_1777]")
	}
	if content != "你好世界" {
		t.Fatalf("content=%q want=%q", content, "你好世界")
	}

	prefix, content = splitCodegenPrefix("plain text")
	if prefix != "" {
		t.Fatalf("expected empty prefix for plain text, got %q", prefix)
	}
	if content != "plain text" {
		t.Fatalf("content=%q want=%q", content, "plain text")
	}
}

func TestQueueContainsMessageID(t *testing.T) {
	if !queueContainsMessageID([]string{"a", "b"}, "b") {
		t.Fatalf("expected queueContainsMessageID to find existing item")
	}
	if queueContainsMessageID([]string{"a", "b"}, "c") {
		t.Fatalf("expected queueContainsMessageID to return false for missing item")
	}
}
