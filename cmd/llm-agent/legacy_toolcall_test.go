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

func TestNormalizeResponseToolCallsExtractsDSMLToolCall(t *testing.T) {
	input := "让我查看一下最近的安排。\n\n<｜｜DSML｜｜tool_calls>\n<｜｜DSML｜｜invoke name=\"RawGetTodosByDate\">\n<｜｜DSML｜｜parameter name=\"start_date\" string=\"true\">2026-04-27</｜｜DSML｜｜parameter>\n<｜｜DSML｜｜parameter name=\"end_date\" string=\"true\">2026-05-03</｜｜DSML｜｜parameter>\n</｜｜DSML｜｜invoke>\n</｜｜DSML｜｜tool_calls>\n"

	cleaned, calls := normalizeResponseToolCalls(input, nil)
	if cleaned != "让我查看一下最近的安排。" {
		t.Fatalf("unexpected cleaned content: %q", cleaned)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "RawGetTodosByDate" {
		t.Fatalf("unexpected tool name: %s", calls[0].Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal args failed: %v", err)
	}
	if got := firstNonEmptyMapString(args, "start_date"); got != "2026-04-27" {
		t.Fatalf("unexpected start_date: %q", got)
	}
	if got := firstNonEmptyMapString(args, "end_date"); got != "2026-05-03" {
		t.Fatalf("unexpected end_date: %q", got)
	}
}

func TestNormalizeResponseToolCallsExtractsCanonicalDSMLToolCall(t *testing.T) {
	input := "开始处理。\n\n<｜DSML｜tool_calls>\n<｜DSML｜invoke name=\"ExecuteCode\">\n<｜DSML｜parameter name=\"description\" string=\"true\">查询最近待办</｜DSML｜parameter>\n<｜DSML｜parameter name=\"tools_hint\" string=\"false\">[\"RawGetTodosByDate\"]</｜DSML｜parameter>\n<｜DSML｜parameter name=\"metadata\" string=\"false\">{\"source\":\"deepseek-v4\",\"retry\":0}</｜DSML｜parameter>\n</｜DSML｜invoke>\n</｜DSML｜tool_calls>\n"

	cleaned, calls := normalizeResponseToolCalls(input, nil)
	if cleaned != "开始处理。" {
		t.Fatalf("unexpected cleaned content: %q", cleaned)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "ExecuteCode" {
		t.Fatalf("unexpected tool name: %s", calls[0].Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal args failed: %v", err)
	}
	if got := firstNonEmptyMapString(args, "description"); got != "查询最近待办" {
		t.Fatalf("unexpected description: %q", got)
	}
	toolsHint, ok := args["tools_hint"].([]any)
	if !ok || len(toolsHint) != 1 || toolsHint[0] != "RawGetTodosByDate" {
		t.Fatalf("unexpected tools_hint: %#v", args["tools_hint"])
	}
	metadata, ok := args["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected metadata type: %#v", args["metadata"])
	}
	if metadata["source"] != "deepseek-v4" {
		t.Fatalf("unexpected metadata.source: %#v", metadata["source"])
	}
	if metadata["retry"] != float64(0) {
		t.Fatalf("unexpected metadata.retry: %#v", metadata["retry"])
	}
}
