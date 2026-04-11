package main

import (
	"encoding/base64"
	"testing"
)

func TestExtractVoiceReply(t *testing.T) {
	raw := []byte("#!AMR\n123")
	meta := map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString(raw),
		"audio_format": "AMR",
	}

	gotBytes, gotFormat, err := extractVoiceReply(meta)
	if err != nil {
		t.Fatalf("extractVoiceReply returned error: %v", err)
	}
	if gotFormat != "amr" {
		t.Fatalf("unexpected format: %q", gotFormat)
	}
	if string(gotBytes) != string(raw) {
		t.Fatalf("unexpected bytes: %q", string(gotBytes))
	}
}

func TestExtractVoiceReplyDefaultsFormat(t *testing.T) {
	meta := map[string]any{
		"audio_base64": base64.StdEncoding.EncodeToString([]byte("abc")),
	}

	_, gotFormat, err := extractVoiceReply(meta)
	if err != nil {
		t.Fatalf("extractVoiceReply returned error: %v", err)
	}
	if gotFormat != "mp3" {
		t.Fatalf("expected default format mp3, got %q", gotFormat)
	}
}

func TestIsAMRData(t *testing.T) {
	if !isAMRData([]byte("#!AMR\npayload")) {
		t.Fatalf("expected AMR header to be detected")
	}
	if isAMRData([]byte("not-amr")) {
		t.Fatalf("expected non-AMR data to be rejected")
	}
}
