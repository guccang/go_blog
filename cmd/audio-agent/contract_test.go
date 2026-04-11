package main

import "testing"

func TestNewConnectionRegistersSpeechTools(t *testing.T) {
	cfg := DefaultConfig()
	conn := NewConnection(cfg, "audio-agent-test")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "audio_agent" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(conn.Client.Tools))
	}
	if conn.Client.Tools[0].Name != "AudioToText" {
		t.Fatalf("expected first tool AudioToText, got %s", conn.Client.Tools[0].Name)
	}
	if conn.Client.Tools[1].Name != "TextToAudio" {
		t.Fatalf("expected second tool TextToAudio, got %s", conn.Client.Tools[1].Name)
	}
}
