package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateMiniMaxTextToImage(t *testing.T) {
	var reqBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/image_generation" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "task-1",
			"data": map[string]any{
				"image_base64": []string{"aW1hZ2U="},
			},
			"metadata": map[string]any{
				"success_count": "1",
				"failed_count":  "0",
			},
			"base_resp": map[string]any{
				"status_code": 0,
				"status_msg":  "success",
			},
		})
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.TextToImage = ImageModelRef{Provider: "minimax", Model: "default"}
	provider := cfg.Providers["minimax"]
	provider.BaseURL = server.URL
	provider.APIKey = "test-key"
	cfg.Providers["minimax"] = provider

	result, err := NewImageClient(cfg).Generate(context.Background(), GenerateImageParams{
		Prompt:          "draw a cabin",
		AspectRatio:     "16:9",
		ResponseFormat:  "base64",
		N:               2,
		PromptOptimizer: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if reqBody["model"] != "image-01" {
		t.Fatalf("unexpected model: %v", reqBody["model"])
	}
	if reqBody["prompt"] != "draw a cabin" {
		t.Fatalf("unexpected prompt: %v", reqBody["prompt"])
	}
	if reqBody["aspect_ratio"] != "16:9" {
		t.Fatalf("unexpected aspect_ratio: %v", reqBody["aspect_ratio"])
	}
	if reqBody["response_format"] != "base64" {
		t.Fatalf("unexpected response_format: %v", reqBody["response_format"])
	}
	if reqBody["n"] != float64(2) && reqBody["n"] != 2 {
		t.Fatalf("unexpected n: %v", reqBody["n"])
	}
	if result["image_base64"] != "aW1hZ2U=" {
		t.Fatalf("unexpected image_base64: %v", result["image_base64"])
	}
	appMessage, ok := result["app_message"].(map[string]any)
	if !ok {
		t.Fatalf("missing app_message: %#v", result["app_message"])
	}
	if appMessage["message_type"] != "image" {
		t.Fatalf("unexpected app message type: %v", appMessage["message_type"])
	}
}

func TestGenerateMiniMaxImageToImage(t *testing.T) {
	var reqBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"image_base64": []string{"aW1hZ2Uy"},
			},
			"base_resp": map[string]any{
				"status_code": 0,
				"status_msg":  "success",
			},
		})
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.TextToImage = ImageModelRef{Provider: "minimax", Model: "default"}
	provider := cfg.Providers["minimax"]
	provider.BaseURL = server.URL
	provider.APIKey = "test-key"
	cfg.Providers["minimax"] = provider

	_, err := NewImageClient(cfg).Generate(context.Background(), GenerateImageParams{
		Prompt: "same character in a library",
		SubjectReference: []SubjectReference{
			{Type: "character", ImageFile: "https://example.com/ref.jpg"},
		},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	rawRefs, ok := reqBody["subject_reference"].([]any)
	if !ok || len(rawRefs) != 1 {
		t.Fatalf("unexpected subject_reference: %#v", reqBody["subject_reference"])
	}
	ref, ok := rawRefs[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected subject reference item: %#v", rawRefs[0])
	}
	if ref["type"] != "character" || ref["image_file"] != "https://example.com/ref.jpg" {
		t.Fatalf("unexpected subject reference: %#v", ref)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
