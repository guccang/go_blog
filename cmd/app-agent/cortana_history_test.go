package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uap"
)

func TestHandleCortanaBroadcastPersistsVoiceHistory(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = t.TempDir()
	bridge := NewBridge(cfg)
	fakeStore := &fakeOBSStorage{}
	bridge.obsStorage = fakeStore

	payload := `{"kind":"cortana_broadcast","text":"晚上九点该休息了","expression":"happy","motion":"IdleWave","origin":"cortana-agent","broadcast_id":"broadcast-1","timestamp":1710000000000,"audio_base64":"ZmFrZQ==","audio_format":"mp3"}`
	ok := bridge.handleCortanaBroadcast(uap.NotifyPayload{
		To:      "alice",
		Content: payload,
	})
	if !ok {
		t.Fatalf("expected cortana broadcast to be handled")
	}

	items := bridge.cortanaHistory.List("alice", 10)
	if len(items) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(items))
	}
	item := items[0]
	if item.Text != "晚上九点该休息了" {
		t.Fatalf("unexpected history text: %q", item.Text)
	}
	if item.FileID == "" {
		t.Fatalf("expected history item file_id")
	}
	if !strings.Contains(item.ObjectKey, "app/cortana-history/alice/") {
		t.Fatalf("unexpected cortana object key: %q", item.ObjectKey)
	}
	if len(fakeStore.bodies[item.ObjectKey]) == 0 {
		t.Fatalf("expected cortana audio to be uploaded to OBS")
	}
}

func TestHandleCortanaHistoryReturnsItems(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = t.TempDir()
	cfg.ReceiveToken = "app-token"
	bridge := NewBridge(cfg)
	if err := bridge.cortanaHistory.Append("alice", CortanaVoiceHistoryItem{
		ID:              "voice-1",
		BroadcastID:     "broadcast-1",
		Text:            "今天还有一个会议提醒",
		FileID:          "alice/20260504/voice.mp3",
		FileName:        "voice.mp3",
		AudioFormat:     "mp3",
		CreatedAt:       time.Now().UnixMilli(),
		StorageProvider: "obs",
		ObjectKey:       "app/cortana-history/alice/20260504/broadcast-1/voice.mp3",
		Source:          "cortana_broadcast",
	}); err != nil {
		t.Fatalf("append cortana history: %v", err)
	}

	auth := newAuthManager(cfg)
	auth.sessions["session-1"] = &appSession{
		Account:   "alice",
		Token:     "session-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	settings := NewCortanaSettingsStore(filepath.Join(t.TempDir(), "cortana-settings.json"))
	handler := NewHandler(cfg, bridge, auth, nil, settings, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/app/cortana/history?user_id=alice&session_token=session-1", nil)
	req.Header.Set("X-App-Agent-Token", "app-token")
	req.Header.Set("X-App-Agent-Session", "session-1")
	rec := httptest.NewRecorder()

	handler.HandleCortanaHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp cortanaVoiceHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(resp.Items))
	}
	if resp.Items[0].FileID != "alice/20260504/voice.mp3" {
		t.Fatalf("unexpected file_id: %q", resp.Items[0].FileID)
	}
}
