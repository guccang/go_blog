package main

import (
	"encoding/json"
	"testing"
)

func TestToolPushTestVoiceValidateArgs(t *testing.T) {
	conn := &Connection{}

	if result, ok := conn.toolPushTestVoice(map[string]interface{}{
		"text": "测试语音",
	}); ok || result != "account 不能为空" {
		t.Fatalf("expected missing account error, got ok=%v result=%q", ok, result)
	}

	if result, ok := conn.toolPushTestVoice(map[string]interface{}{
		"account": "ztt",
	}); ok || result != "text 不能为空" {
		t.Fatalf("expected missing text error, got ok=%v result=%q", ok, result)
	}
}

func TestToolPushTestVoiceSuccess(t *testing.T) {
	conn := &Connection{}
	conn.broadcastSender = func(account, text, expression, motion string) error {
		if account != "ztt" || text != "测试播报" {
			t.Fatalf("unexpected broadcast target=%s text=%s", account, text)
		}
		if expression != "happy" || motion != "IdleWave" {
			t.Fatalf("unexpected expression=%s motion=%s", expression, motion)
		}
		return nil
	}

	result, ok := conn.toolPushTestVoice(map[string]interface{}{
		"account": "ztt",
		"text":    "测试播报",
	})
	if !ok {
		t.Fatalf("expected success, got result=%q", result)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if payload["kind"] != "test_voice" {
		t.Fatalf("expected kind test_voice, got %#v", payload["kind"])
	}
}

func TestCollectSnapshotIncludesReadableLocalTime(t *testing.T) {
	conn := &Connection{}
	snapshot := conn.collectSnapshot(&CortanaUserSession{
		Account:        "ztt",
		CurrentContext: map[string]any{"client": "flutter"},
	}, &MonitorResult{}, CortanaTriggerEventPayload{
		TriggerReason: "monitor_cycle",
	})

	if snapshot.CollectedAt <= 0 {
		t.Fatalf("expected collected_at")
	}
	if snapshot.LocalDatetime == "" {
		t.Fatalf("expected local_datetime")
	}
	if snapshot.Weekday == "" {
		t.Fatalf("expected weekday")
	}
	if snapshot.TimezoneOffset == "" {
		t.Fatalf("expected timezone_offset")
	}
}
