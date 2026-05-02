package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPersistAttachmentIgnoresControlOnlyTextMeta(t *testing.T) {
	cfg := DefaultConfig()
	bridge := NewBridge(cfg)

	attachment, err := bridge.persistAttachment(&AppMessage{
		UserID:      "demo-user",
		Content:     "你好",
		MessageType: "text",
		Meta: map[string]any{
			"input_mode":         "cortana_text",
			"reply_mode":         "audio_preferred",
			"cortana_request_id": "cortana_demo_1",
		},
	})
	if err != nil {
		t.Fatalf("persistAttachment returned error: %v", err)
	}
	if attachment != nil {
		t.Fatalf("expected no attachment for control-only text meta, got %#v", attachment)
	}
}

func TestPersistAttachmentKeepsAudioAttachment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = t.TempDir()
	bridge := NewBridge(cfg)
	fakeStore := &fakeOBSStorage{}
	bridge.obsStorage = fakeStore

	attachment, err := bridge.persistAttachment(&AppMessage{
		UserID:      "demo-user",
		Content:     "[语音回复]",
		MessageType: "audio",
		Meta: map[string]any{
			"audio_base64": "ZmFrZQ==",
			"audio_format": "mp3",
			"speech_text":  "你好",
			"input_mode":   "tts_reply",
		},
	})
	if err != nil {
		t.Fatalf("persistAttachment returned error: %v", err)
	}
	if attachment == nil {
		t.Fatalf("expected audio attachment to be persisted")
	}
	if attachment.MessageType != "audio" {
		t.Fatalf("expected attachment message_type audio, got %q", attachment.MessageType)
	}
	if attachment.FileID == "" {
		t.Fatalf("expected persisted attachment to have file_id")
	}
	if attachment.FilePath == "" {
		t.Fatalf("expected persisted attachment to have file_path")
	}
	metaBody := fakeStore.bodies[attachmentMetaObjectKey(attachment.ObjectKey)]
	if len(metaBody) == 0 {
		t.Fatalf("expected audio sidecar meta to be uploaded")
	}
	var sidecar map[string]any
	if err := json.Unmarshal(metaBody, &sidecar); err != nil {
		t.Fatalf("decode audio sidecar meta: %v", err)
	}
	if got := sidecar["description"]; got != "[语音回复]" {
		t.Fatalf("expected sidecar description, got %#v", got)
	}
	if got := sidecar["speech_text"]; got != "你好" {
		t.Fatalf("expected sidecar speech_text, got %#v", got)
	}
	if strings.Contains(string(metaBody), "ZmFrZQ==") {
		t.Fatalf("sidecar meta must not contain audio_base64")
	}
}

func TestPersistAttachmentHydratesImageURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("fake-png"))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = t.TempDir()
	bridge := NewBridge(cfg)
	meta := map[string]any{
		"image_url": server.URL + "/generated.png",
		"filename":  "generated_image_1.png",
	}

	attachment, err := bridge.persistAttachment(&AppMessage{
		UserID:      "demo-user",
		Content:     "图片已生成",
		MessageType: "image",
		Meta:        meta,
	})
	if err != nil {
		t.Fatalf("persistAttachment returned error: %v", err)
	}
	if attachment == nil {
		t.Fatalf("expected image attachment to be persisted")
	}
	if attachment.FileID == "" {
		t.Fatalf("expected persisted image to have file_id")
	}
	if attachment.FileName != "generated_image_1.png" {
		t.Fatalf("expected filename fallback, got %q", attachment.FileName)
	}
	if attachment.Format != "png" {
		t.Fatalf("expected image format png, got %q", attachment.Format)
	}
	if meta["image_base64"] == "" {
		t.Fatalf("expected image_url to be hydrated into image_base64")
	}
}

func TestPersistAttachmentIgnoresTextFileNameOnlyMeta(t *testing.T) {
	cfg := DefaultConfig()
	bridge := NewBridge(cfg)

	attachment, err := bridge.persistAttachment(&AppMessage{
		UserID:      "demo-user",
		Content:     "你好",
		MessageType: "text",
		Meta: map[string]any{
			"file_name": "reply.txt",
			"mime_type": "text/plain",
		},
	})
	if err != nil {
		t.Fatalf("persistAttachment returned error: %v", err)
	}
	if attachment != nil {
		t.Fatalf("expected no attachment for text-only file metadata, got %#v", attachment)
	}
}
