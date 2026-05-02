package main

import "testing"

func TestNewConnectionRegistersImageTools(t *testing.T) {
	cfg := DefaultConfig()
	conn := NewConnection(cfg, "image-agent-test")
	if conn == nil {
		t.Fatalf("expected connection to be created")
	}
	if conn.AgentType != "image_agent" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(conn.Client.Tools))
	}
	if conn.Client.Tools[0].Name != "ImageToText" {
		t.Fatalf("expected first tool ImageToText, got %s", conn.Client.Tools[0].Name)
	}
	if conn.Client.Tools[1].Name != "TextToImage" {
		t.Fatalf("expected second tool TextToImage, got %s", conn.Client.Tools[1].Name)
	}
	if conn.Client.Tools[2].Name != "ImageToImage" {
		t.Fatalf("expected third tool ImageToImage, got %s", conn.Client.Tools[2].Name)
	}
}

func TestNewConnectionOmitsImageToTextWhenVisionUnavailable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ImageToText = ImageModelRef{Provider: "minimax", Model: "default"}
	cfg.TextToImage = ImageModelRef{Provider: "minimax", Model: "default"}

	conn := NewConnection(cfg, "image-agent-test")
	if len(conn.Client.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(conn.Client.Tools))
	}
	if conn.Client.Tools[0].Name != "TextToImage" {
		t.Fatalf("expected first tool TextToImage, got %s", conn.Client.Tools[0].Name)
	}
	if conn.Client.Tools[1].Name != "ImageToImage" {
		t.Fatalf("expected second tool ImageToImage, got %s", conn.Client.Tools[1].Name)
	}
}
