package main

import (
	"encoding/json"
	"testing"

	"uap"
)

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
		"[thought][acp_1777]  is asking",
	)
	want := "[thought][acp_1777] The user is asking"
	if got != want {
		t.Fatalf("mergeCodegenStreamText()=%q want=%q", got, want)
	}
}

func TestMergeCodegenStreamTextDoesNotInventSpacesInsideChunk(t *testing.T) {
	got := mergeCodegenStreamText(
		"[assistant][acp_1777] Hel",
		"[assistant][acp_1777] lo",
	)
	want := "[assistant][acp_1777] Hello"
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

func TestCodegenEventMetaPreservesStructuredEvent(t *testing.T) {
	payload := codegenStreamEvent{SessionID: "acp_1777"}
	payload.Event.Type = "tool"
	payload.Event.Text = "running"
	payload.Event.ToolName = "go test"
	payload.Event.Done = true

	meta := codegenEventMeta(payload)
	if meta["type"] != "tool" || meta["text"] != "running" || meta["tool_name"] != "go test" || meta["done"] != true {
		t.Fatalf("codegenEventMeta() = %#v", meta)
	}
}

func TestDirectACPStreamNotifyUsesRememberedRoute(t *testing.T) {
	bridge := NewBridge(DefaultConfig())

	startNotify := uap.NotifyPayload{
		Channel: "app",
		To:      "demo-user",
		Content: "🚀 编码会话已启动\n\n项目: demo\n请求: cmd_start_123\n\n进度将通过当前客户端推送",
		Meta: map[string]any{
			"codegen_history_id": "history-1",
		},
	}
	startRaw, err := json.Marshal(startNotify)
	if err != nil {
		t.Fatal(err)
	}
	bridge.handleUAPMessage(&uap.Message{
		Type:    uap.MsgNotify,
		From:    "cmd-agent",
		Payload: startRaw,
	})

	stream := codegenStreamEvent{
		SessionID: "acp_123",
		RequestID: "cmd_start_123",
	}
	stream.Event.Type = "assistant"
	stream.Event.Text = "执行中"
	streamRaw, err := json.Marshal(stream)
	if err != nil {
		t.Fatal(err)
	}
	streamNotify := uap.NotifyPayload{
		Channel: "acp_stream",
		To:      "cmd_start_123",
		Content: string(streamRaw),
	}
	notifyRaw, err := json.Marshal(streamNotify)
	if err != nil {
		t.Fatal(err)
	}
	bridge.handleUAPMessage(&uap.Message{
		Type:    uap.MsgNotify,
		From:    "acp-agent",
		Payload: notifyRaw,
	})

	bridge.deliveryMu.Lock()
	pending := bridge.pendingMessages["codegen_stream:acp_123"]
	bridge.deliveryMu.Unlock()
	if pending == nil {
		t.Fatalf("expected codegen stream pending message")
	}
	if _, ok := pending.Deliveries["demo-user"]; !ok {
		t.Fatalf("expected delivery for demo-user, got %#v", pending.Deliveries)
	}
	if pending.Meta["request_id"] != "cmd_start_123" {
		t.Fatalf("request_id meta=%#v", pending.Meta["request_id"])
	}
	if pending.Meta["codegen_history_id"] != "history-1" {
		t.Fatalf("codegen_history_id meta=%#v", pending.Meta["codegen_history_id"])
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
