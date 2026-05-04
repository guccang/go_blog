package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResolveConfiguredTTSVoiceIgnoresExternalVoice(t *testing.T) {
	client := NewAudioClient(&Config{
		TextToSpeech: AudioModelRef{Provider: "minimax", Model: "default"},
	})
	voice, err := client.resolveConfiguredTTSVoice(&TextToSpeechModelConfig{
		Model:        "speech-2.8-hd",
		DefaultVoice: "female-tianmei",
	}, "zh-CN-XiaoxiaoNeural")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice != "female-tianmei" {
		t.Fatalf("expected configured voice, got %q", voice)
	}
}

func TestResolveConfiguredTTSVoiceRequiresDefaultVoice(t *testing.T) {
	client := NewAudioClient(&Config{
		TextToSpeech: AudioModelRef{Provider: "minimax", Model: "default"},
	})
	voice, err := client.resolveConfiguredTTSVoice(&TextToSpeechModelConfig{
		Model: "speech-2.8-hd",
	}, "zh-CN-XiaoxiaoNeural")
	if err == nil {
		t.Fatalf("expected error, got voice=%q", voice)
	}
}

func TestGenerateMiniMaxMusicUsesPromptOnlyWithLyricsOptimizer(t *testing.T) {
	oldClient := audioHTTPClient
	defer func() { audioHTTPClient = oldClient }()

	var got map[string]any
	audioHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/music_generation" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"data":{"audio":"6869","status":2},
				"trace_id":"trace-1",
				"extra_info":{"music_duration":1000},
				"base_resp":{"status_code":0,"status_msg":"success"}
			}`)),
		}, nil
	})}

	cfg := DefaultConfig()
	provider := cfg.Providers["minimax"]
	provider.BaseURL = "https://api.minimaxi.com"
	provider.APIKey = "test-key"
	cfg.Providers["minimax"] = provider

	result, err := NewAudioClient(cfg).GenerateMusic(context.Background(), GenerateMusicParams{
		Prompt: "lofi, rainy night",
	})
	if err != nil {
		t.Fatalf("GenerateMusic returned error: %v", err)
	}
	if got["model"] != "music-2.6" {
		t.Fatalf("unexpected model: %v", got["model"])
	}
	if got["prompt"] != "lofi, rainy night" {
		t.Fatalf("unexpected prompt: %v", got["prompt"])
	}
	if got["lyrics"] != nil {
		t.Fatalf("lyrics should be omitted for prompt-only request: %v", got["lyrics"])
	}
	if got["lyrics_optimizer"] != true {
		t.Fatalf("lyrics_optimizer should be true, got %v", got["lyrics_optimizer"])
	}
	if got["output_format"] != "hex" {
		t.Fatalf("unexpected output_format: %v", got["output_format"])
	}

	audioBase64, _ := result["audio_base64"].(string)
	if audioBase64 != base64.StdEncoding.EncodeToString([]byte("hi")) {
		t.Fatalf("unexpected audio_base64: %q", audioBase64)
	}
	if result["kind"] != "music" {
		t.Fatalf("unexpected kind: %v", result["kind"])
	}
	if result["audio_format"] != "mp3" {
		t.Fatalf("unexpected audio_format: %v", result["audio_format"])
	}
}
