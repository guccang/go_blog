package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleAttachmentRedirectsAPKViaObsAgent(t *testing.T) {
	var authHeader string
	var requestedPath string
	var requestedTicket string

	obsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("X-App-Agent-Token")
		requestedPath = r.URL.Path
		requestedTicket = r.URL.Query().Get("ticket")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"url":     "https://obs.example.com/download/app-release.apk?sig=1",
			"method":  "GET",
			"headers": map[string]string{},
		})
	}))
	defer obsServer.Close()

	attachmentDir := t.TempDir()
	ownerDir := filepath.Join(attachmentDir, "alice")
	if err := os.MkdirAll(ownerDir, 0o755); err != nil {
		t.Fatalf("mkdir attachment owner dir: %v", err)
	}
	filePath := filepath.Join(ownerDir, "app-release.apk")
	if err := os.WriteFile(filePath, []byte("apk-binary"), 0o644); err != nil {
		t.Fatalf("write attachment file: %v", err)
	}
	fileID, err := buildAttachmentFileID(attachmentDir, filePath)
	if err != nil {
		t.Fatalf("buildAttachmentFileID returned error: %v", err)
	}

	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = attachmentDir
	cfg.ReceiveToken = "app-token"
	cfg.ObsAgentBaseURL = obsServer.URL
	cfg.ObsAgentToken = "obs-token"
	cfg.DownloadTicketSecret = "download-secret"
	cfg.DownloadTicketTTLSeconds = 300

	bridge := NewBridge(cfg)
	auth := newAuthManager(cfg)
	auth.sessions["session-1"] = &appSession{
		Account:   "alice",
		Token:     "session-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handler := NewHandler(cfg, bridge, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/app/attachments/"+fileID+"?user_id=alice&session_token=session-1", nil)
	req.Header.Set("X-App-Agent-Token", "app-token")
	req.Header.Set("X-App-Agent-Session", "session-1")
	rec := httptest.NewRecorder()

	handler.HandleAttachment(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://obs.example.com/download/app-release.apk?sig=1" {
		t.Fatalf("expected redirect location to OBS url, got %q", got)
	}
	if authHeader != "obs-token" {
		t.Fatalf("expected obs-agent auth header obs-token, got %q", authHeader)
	}
	if requestedPath != "/api/obs/download/"+fileID {
		t.Fatalf("unexpected obs-agent path: %q", requestedPath)
	}
	if strings.TrimSpace(requestedTicket) == "" {
		t.Fatalf("expected download ticket query to be present")
	}
}

func TestHandleAttachmentFallsBackToLocalWhenObsAgentFails(t *testing.T) {
	obsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sign failed", http.StatusBadGateway)
	}))
	defer obsServer.Close()

	attachmentDir := t.TempDir()
	ownerDir := filepath.Join(attachmentDir, "alice")
	if err := os.MkdirAll(ownerDir, 0o755); err != nil {
		t.Fatalf("mkdir attachment owner dir: %v", err)
	}
	filePath := filepath.Join(ownerDir, "app-release.apk")
	if err := os.WriteFile(filePath, []byte("apk-binary"), 0o644); err != nil {
		t.Fatalf("write attachment file: %v", err)
	}
	fileID, err := buildAttachmentFileID(attachmentDir, filePath)
	if err != nil {
		t.Fatalf("buildAttachmentFileID returned error: %v", err)
	}

	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = attachmentDir
	cfg.ReceiveToken = "app-token"
	cfg.ObsAgentBaseURL = obsServer.URL
	cfg.ObsAgentToken = "obs-token"
	cfg.DownloadTicketSecret = "download-secret"
	cfg.DownloadTicketTTLSeconds = 300

	bridge := NewBridge(cfg)
	auth := newAuthManager(cfg)
	auth.sessions["session-1"] = &appSession{
		Account:   "alice",
		Token:     "session-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handler := NewHandler(cfg, bridge, auth)

	req := httptest.NewRequest(http.MethodGet, "/api/app/attachments/"+fileID+"?user_id=alice&session_token=session-1", nil)
	req.Header.Set("X-App-Agent-Token", "app-token")
	req.Header.Set("X-App-Agent-Session", "session-1")
	rec := httptest.NewRecorder()

	handler.HandleAttachment(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 fallback, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "apk-binary" {
		t.Fatalf("expected local fallback body, got %q", got)
	}
}
