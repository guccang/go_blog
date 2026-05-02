package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAllowsMiniMaxGenerationWithoutVision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image-agent.json")
	data := []byte(`{
  "server_url": "ws://127.0.0.1:10086/ws/uap",
  "agent_name": "image-agent",
  "image_to_text": {
    "provider": "minimax",
    "model": "default"
  },
  "text_to_image": {
    "provider": "minimax",
    "model": "default"
  },
  "providers": {
    "minimax": {
      "kind": "minimax",
      "base_url": "https://api.minimaxi.com",
      "api_key": "test-key",
      "image_generation_path": "/v1/image_generation",
      "generation_models": {
        "default": {
          "model": "image-01",
          "aspect_ratio": "1:1",
          "response_format": "base64",
          "n": 1
        }
      }
    }
  }
}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.HasVisionTool() {
		t.Fatalf("expected minimax-only config to have no vision tool")
	}
	if _, _, err := cfg.ResolveGeneration(); err != nil {
		t.Fatalf("ResolveGeneration returned error: %v", err)
	}
}
