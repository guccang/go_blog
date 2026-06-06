package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMessageLoggerWritesUTF8JSONLAndRedactsSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "messages.jsonl")
	logger, err := newMessageLogger(path)
	if err != nil {
		t.Fatalf("newMessageLogger() error: %v", err)
	}
	logger.addSecrets("plain-secret")
	logger.log("sent", "http", "request", "ztt", map[string]any{
		"content":       "你好",
		"password":      "password-value",
		"session_token": "session-value",
		"plain_error":   "request failed with plain-secret",
		"nested": map[string]any{
			"delegation_token": "delegation-value",
			"message":          "完整业务内容",
		},
	}, nil)
	if err := logger.close(); err != nil {
		t.Fatalf("close() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	text := string(data)
	for _, secret := range []string{"password-value", "session-value", "delegation-value", "plain-secret"} {
		if strings.Contains(text, secret) {
			t.Fatalf("log contains secret %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, "你好") || !strings.Contains(text, "完整业务内容") {
		t.Fatalf("log missing UTF-8 business content: %s", text)
	}

	var entry messageLogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("decode JSONL entry: %v", err)
	}
	payload, ok := entry.Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type=%T", entry.Payload)
	}
	if payload["password"] != "[REDACTED]" || payload["session_token"] != "[REDACTED]" {
		t.Fatalf("payload not redacted: %#v", payload)
	}
}

func TestMessageLoggerAppendsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	logger, err := newMessageLogger(path)
	if err != nil {
		t.Fatalf("newMessageLogger() error: %v", err)
	}
	logger.log("sent", "websocket", "ack", "ztt", map[string]string{"message_id": "m1"}, nil)
	logger.log("received", "websocket", "message", "ztt", map[string]string{"content": "回复"}, nil)
	if err := logger.close(); err != nil {
		t.Fatalf("close() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count=%d want 2: %s", len(lines), data)
	}
}
