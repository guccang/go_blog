package main

import (
	"encoding/json"
	"testing"
)

func TestExtractLegacyToolCallBlocks(t *testing.T) {
	input := "好的，我用语音回复您！\n[TOOL_CALL]\n{tool => 'TextToAudio', args => {\n  --content \"爸爸你好呀！小元宝。\"\n  --voice 'zh-CN-XiaoxiaoNeural'\n  --format \"mp3\"\n}}\n"

	cleaned, calls := extractLegacyToolCallBlocks(input)
	if cleaned != "好的，我用语音回复您！" {
		t.Fatalf("unexpected cleaned content: %q", cleaned)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "TextToAudio" {
		t.Fatalf("unexpected tool name: %s", calls[0].Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal args failed: %v", err)
	}
	if got := firstNonEmptyMapString(args, "text"); got != "爸爸你好呀！小元宝。" {
		t.Fatalf("unexpected text: %q", got)
	}
	if got := firstNonEmptyMapString(args, "voice"); got != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("unexpected voice: %q", got)
	}
	if got := firstNonEmptyMapString(args, "audio_format"); got != "mp3" {
		t.Fatalf("unexpected audio_format: %q", got)
	}
}

func TestNormalizeResponseToolCallsExtractsLegacyTextToolCall(t *testing.T) {
	input := "好的，我用语音回复您！\n\n[TOOL_CALL]\n{tool => \"TextToAudio\", args => {\n  --text \"晚安，做个好梦。\"\n}}\n"

	cleaned, calls := normalizeResponseToolCalls(input, nil)
	if cleaned != "好的，我用语音回复您！" {
		t.Fatalf("unexpected cleaned content: %q", cleaned)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
}
