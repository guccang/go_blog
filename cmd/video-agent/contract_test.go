package main

import "testing"

func TestConnectionContract(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers["minimax"] = VideoProviderConfig{
		BaseURL:             "https://api.minimaxi.com",
		APIKey:              "test-key",
		VideoGenerationPath: "/v1/video_generation",
		QueryPath:           "/v1/query/video_generation",
		FileRetrievePath:    "/v1/files/retrieve",
		Models: map[string]VideoModelConfig{
			"default": {Model: "MiniMax-Hailuo-2.3"},
		},
	}
	conn := NewConnection(cfg, "video-agent-test")
	if conn.AgentType != "video_agent" {
		t.Fatalf("unexpected agent type: %s", conn.AgentType)
	}
	if len(conn.Client.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(conn.Client.Tools))
	}
	if conn.Client.Tools[0].Name != "TextToVideo" {
		t.Fatalf("expected first tool TextToVideo, got %s", conn.Client.Tools[0].Name)
	}
	if conn.Client.Tools[1].Name != "ImageToVideo" {
		t.Fatalf("expected second tool ImageToVideo, got %s", conn.Client.Tools[1].Name)
	}
}
