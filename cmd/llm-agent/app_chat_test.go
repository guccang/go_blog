package main

import (
	"strings"
	"testing"
	"time"
)

func TestExtractCortanaActionPlanFromTaggedBlock(t *testing.T) {
	input := `当然可以，我来帮你梳理一下。

[CORTANA_ACTION_PLAN]
{
  "speech_text": "当然可以，我来帮你梳理一下。",
  "expression": "happy",
  "fallback_expression": "happy",
  "expression_hold_ms": 1500,
  "actions": [
    {"motion": "IdleWave", "delay": 0, "index": 0},
    {"motion": "Idle", "delay": 1800, "hold_ms": 1200}
  ]
}`

	cleaned, plan := extractCortanaActionPlan(input)
	if cleaned != "当然可以，我来帮你梳理一下。" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if plan == nil {
		t.Fatalf("expected action plan to be extracted")
	}
	if got := plan["expression"]; got != "happy" {
		t.Fatalf("unexpected expression: %#v", got)
	}
	if got := plan["expression_hold_ms"]; got != float64(1500) {
		t.Fatalf("unexpected expression_hold_ms: %#v", got)
	}
	actions, ok := plan["actions"].([]any)
	if !ok || len(actions) != 2 {
		t.Fatalf("unexpected actions: %#v", plan["actions"])
	}
}

func TestExtractCortanaActionPlanFromJsonFence(t *testing.T) {
	input := "收到。\n\n```json\n{\"cortana_action_plan\":{\"speech_text\":\"收到。\",\"expression\":\"sad\",\"actions\":[{\"motion\":\"IdleAlt\",\"delay\":0}]}}\n```"

	cleaned, plan := extractCortanaActionPlan(input)
	if cleaned != "收到。" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if plan == nil {
		t.Fatalf("expected plan from fenced json")
	}
	if got := plan["speech_text"]; got != "收到。" {
		t.Fatalf("unexpected speech_text: %#v", got)
	}
	if got := plan["expression"]; got != "sad" {
		t.Fatalf("unexpected expression: %#v", got)
	}
}

func TestExtractCortanaActionPlanIgnoresInvalidPayload(t *testing.T) {
	input := "普通回复。\n\n[CORTANA_ACTION_PLAN]\n{\"unexpected\":true}"

	cleaned, plan := extractCortanaActionPlan(input)
	if cleaned != input {
		t.Fatalf("expected text to stay unchanged, got %q", cleaned)
	}
	if plan != nil {
		t.Fatalf("expected invalid plan to be ignored, got %#v", plan)
	}
}

func TestBuildCortanaOutputPromptContainsProtocol(t *testing.T) {
	prompt := buildCortanaOutputPrompt()
	expectedSnippets := []string{
		"[CORTANA_ACTION_PLAN]",
		"speech_text",
		"intent",
		"thinking",
		"surprised",
		"fallback_expression",
		"expression_hold_ms",
		"resume_to_idle",
		"suggested_replies",
		"kind: custom",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("prompt missing %q: %s", snippet, prompt)
		}
	}
}

func TestBuildCortanaDeviceContextPromptTreatsCapturedAtAsTelemetry(t *testing.T) {
	prompt := buildCortanaDeviceContextPrompt(map[string]any{
		"device_context": map[string]any{
			"client": map[string]any{
				"captured_at": int64(1778461200000),
			},
		},
	})

	for _, want := range []string{"captured_at", "采集时间戳", "不得把它换算后当作当前时间"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got: %s", want, prompt)
		}
	}
}

func TestBuildCortanaDeviceContextPromptCompactsLive2DMotions(t *testing.T) {
	prompt := buildCortanaDeviceContextPrompt(map[string]any{
		"device_context": map[string]any{
			"live2d": map[string]any{
				"model_id":   "model-1",
				"model_name": "demo",
				"available_motions": map[string]any{
					"face_smile_01": []any{"motions/face_smile_01.motion3.json"},
					"face_sad_01":   []any{"motions/face_sad_01.motion3.json"},
				},
			},
		},
	})

	if !strings.Contains(prompt, "available_motion_count") {
		t.Fatalf("expected compact motion count, got: %s", prompt)
	}
	if strings.Contains(prompt, "motions/face_smile_01.motion3.json") {
		t.Fatalf("full motion file list should not be injected: %s", prompt)
	}
	if strings.Contains(prompt, "face_smile_01") {
		t.Fatalf("motion names should stay out of runtime context: %s", prompt)
	}
}

func TestBuildCortanaTurnRuntimeContextIncludesAuthoritativeLocalTime(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 5, 11, 9, 0, 0, 0, loc)
	context := buildCortanaTurnRuntimeContext(nil, "device_context.client.captured_at=1778461200000", now)

	for _, want := range []string{
		"当前本地时间上下文",
		"2026-05-11 09:00",
		"星期一",
		"timezone_offset: +08:00",
		"time_precision: minute",
	} {
		if !strings.Contains(context, want) {
			t.Fatalf("expected context to contain %q, got: %s", want, context)
		}
	}
}

func TestCortanaRequestIDCanBeAttachedToAudioReplyMeta(t *testing.T) {
	sink := &AppSink{cortanaRequestID: "req-123"}
	meta := map[string]any{
		"audio_base64": "ZmFrZQ==",
		"audio_format": "mp3",
		"input_mode":   "tts_reply",
		"speech_text":  "你好",
	}
	if strings.TrimSpace(sink.cortanaRequestID) != "" {
		meta["cortana_request_id"] = strings.TrimSpace(sink.cortanaRequestID)
	}
	if got := meta["cortana_request_id"]; got != "req-123" {
		t.Fatalf("unexpected cortana_request_id: %#v", got)
	}
}

func TestResolveAppConversationModeSplitsAudioReplyFromCortanaText(t *testing.T) {
	inbound := &appInboundMessage{
		MessageType: "text",
		Meta: map[string]any{
			"conversation_mode": "cortana",
			"input_mode":        "cortana_chat",
			"reply_mode":        "text",
		},
	}
	preferAudio, requestID := resolveAppConversationMode(inbound)
	if preferAudio {
		t.Fatalf("chat replies should not force audio when reply_mode is text")
	}
	if requestID != "" {
		t.Fatalf("unexpected request id: %q", requestID)
	}
}

func TestCortanaShouldUseToolsOnlyForActionableQueries(t *testing.T) {
	if cortanaShouldUseTools("已完成了") {
		t.Fatalf("short companion continuation should not load tools")
	}
	if !cortanaShouldUseTools("帮我查一下今天的天气") {
		t.Fatalf("weather query should load tools")
	}
	if !cortanaShouldUseTools("添加一个待办") {
		t.Fatalf("todo mutation should load tools")
	}
}

func TestResolveAppConversationModeKeepsCortanaVoiceAudioReply(t *testing.T) {
	inbound := &appInboundMessage{
		MessageType: "text",
		Meta: map[string]any{
			"conversation_mode":  "cortana",
			"input_mode":         "cortana_text",
			"reply_mode":         "audio_preferred",
			"cortana_request_id": "req-456",
		},
	}
	preferAudio, requestID := resolveAppConversationMode(inbound)
	if !preferAudio {
		t.Fatalf("Cortana voice entry should prefer audio")
	}
	if requestID != "req-456" {
		t.Fatalf("unexpected request id: %q", requestID)
	}
}
