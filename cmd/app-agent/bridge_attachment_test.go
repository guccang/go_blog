package main

import "testing"

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
