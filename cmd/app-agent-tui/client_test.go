package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBuildWebSocketURL(t *testing.T) {
	got, err := buildWebSocketURL(
		"https://example.com/base/",
		"测试 user",
		"session-token",
		"receive-token",
	)
	if err != nil {
		t.Fatalf("buildWebSocketURL() error: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if parsed.Scheme != "wss" {
		t.Fatalf("scheme=%q want wss", parsed.Scheme)
	}
	if parsed.Path != "/base/ws/app" {
		t.Fatalf("path=%q want /base/ws/app", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("user_id") != "测试 user" {
		t.Fatalf("user_id=%q", query.Get("user_id"))
	}
	if query.Get("session_token") != "session-token" {
		t.Fatalf("session_token=%q", query.Get("session_token"))
	}
	if query.Get("token") != "receive-token" {
		t.Fatalf("token=%q", query.Get("token"))
	}
}

func TestBuildWebSocketURLRejectsUnsupportedScheme(t *testing.T) {
	if _, err := buildWebSocketURL("ftp://example.com", "user", "session", ""); err == nil {
		t.Fatal("expected unsupported scheme error")
	}
}

func TestClientLoginConnectAckAndSend(t *testing.T) {
	ackCh := make(chan map[string]string, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/app/login", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-App-Agent-Token"); got != "receive-token" {
			t.Errorf("login receive token=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":       true,
			"session_token": "session-token",
			"user_id":       "demo-user",
		})
	})
	mux.HandleFunc("/ws/app", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("user_id"); got != "demo-user" {
			t.Errorf("websocket user_id=%q", got)
		}
		if got := r.URL.Query().Get("session_token"); got != "session-token" {
			t.Errorf("websocket session_token=%q", got)
		}
		if got := r.URL.Query().Get("token"); got != "receive-token" {
			t.Errorf("websocket token=%q", got)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.WriteJSON(pushEnvelope{
			MessageID:   "message-1",
			Sequence:    1,
			UserID:      "demo-user",
			Content:     "reply",
			MessageType: "text",
		}); err != nil {
			t.Errorf("write push: %v", err)
			return
		}
		var ack map[string]string
		if err := conn.ReadJSON(&ack); err != nil {
			t.Errorf("read ack: %v", err)
			return
		}
		ackCh <- ack
	})
	mux.HandleFunc("/api/app/message", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-App-Agent-Session"); got != "session-token" {
			t.Errorf("message session token=%q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode message: %v", err)
		}
		if got := payload["content"]; got != "hello" {
			t.Errorf("message content=%v", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	logPath := filepath.Join(t.TempDir(), "messages.jsonl")
	logger, err := newMessageLogger(logPath)
	if err != nil {
		t.Fatalf("newMessageLogger() error: %v", err)
	}
	client := newAppClient(clientConfig{
		BaseURL:      server.URL,
		UserID:       "demo-user",
		Password:     "password-value",
		ReceiveToken: "receive-token",
	}, logger)

	if err := client.loginAndConnect(context.Background()); err != nil {
		t.Fatalf("loginAndConnect() error: %v", err)
	}
	envelope, err := client.readMessage()
	if err != nil {
		t.Fatalf("readMessage() error: %v", err)
	}
	if envelope.Content != "reply" {
		t.Fatalf("content=%q want reply", envelope.Content)
	}
	select {
	case ack := <-ackCh:
		if ack["type"] != "ack" || ack["message_id"] != "message-1" {
			t.Fatalf("ack=%v", ack)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ack")
	}
	if err := client.sendMessage(context.Background(), "hello"); err != nil {
		t.Fatalf("sendMessage() error: %v", err)
	}
	client.close()
	if err := logger.close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read message log: %v", err)
	}
	logText := string(logData)
	for _, expected := range []string{`"transport":"http"`, `"transport":"websocket"`, `"event":"ack"`, `"content":"hello"`, `"content":"reply"`} {
		if !strings.Contains(logText, expected) {
			t.Fatalf("message log missing %s: %s", expected, logText)
		}
	}
	for _, secret := range []string{"password-value", "receive-token", "session-token"} {
		if strings.Contains(logText, secret) {
			t.Fatalf("message log contains secret %q: %s", secret, logText)
		}
	}
}
