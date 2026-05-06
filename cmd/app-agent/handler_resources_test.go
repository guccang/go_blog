package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestHandleResourcesUploadsAndListsByCategory(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = filepath.Join(t.TempDir(), "app-attachments")
	bridge := NewBridge(cfg)
	auth := newAuthManager(cfg)
	auth.sessions["session-1"] = &appSession{
		Account:   "alice",
		Token:     "session-1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handler := NewHandler(cfg, bridge, auth, nil, NewCortanaSettingsStore(filepath.Join(t.TempDir(), "settings.json")))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("user_id", "alice"); err != nil {
		t.Fatalf("write user field: %v", err)
	}
	if err := writer.WriteField("category", "live2d"); err != nil {
		t.Fatalf("write category field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "haru.zip")
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := part.Write([]byte("live2d-zip")); err != nil {
		t.Fatalf("write file field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/app/resources", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-App-Agent-Session", "session-1")
	rec := httptest.NewRecorder()
	handler.HandleResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var upload appResourceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if upload.Item == nil || upload.Item.Category != "live2d" || upload.Item.FileName != "haru.zip" {
		t.Fatalf("unexpected upload item: %#v", upload.Item)
	}
	if upload.Item.FileID == "" {
		t.Fatalf("expected file_id")
	}

	sessionToken := url.QueryEscape("session-1")
	req = httptest.NewRequest(http.MethodGet, "/api/app/resources?user_id=alice&session_token="+sessionToken+"&category=live2d", nil)
	rec = httptest.NewRecorder()
	handler.HandleResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var list appResourceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 resource, got %d: %#v", len(list.Items), list.Items)
	}
	if list.Items[0].Category != "live2d" || list.Items[0].FileName != "haru.zip" {
		t.Fatalf("unexpected list item: %#v", list.Items[0])
	}
}
