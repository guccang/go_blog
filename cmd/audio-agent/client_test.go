package main

import "testing"

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
