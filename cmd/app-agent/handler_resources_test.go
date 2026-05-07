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
	if list.Usage == nil || list.Usage.CategoryCount != 1 || list.Usage.CategorySize != int64(len("live2d-zip")) {
		t.Fatalf("unexpected usage: %#v", list.Usage)
	}

	req = httptest.NewRequest(
		http.MethodDelete,
		"/api/app/resources?user_id=alice&session_token="+sessionToken+"&file_id="+url.QueryEscape(list.Items[0].FileID),
		nil,
	)
	rec = httptest.NewRecorder()
	handler.HandleResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/app/resources?user_id=alice&session_token="+sessionToken+"&category=live2d", nil)
	rec = httptest.NewRecorder()
	handler.HandleResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list after delete expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	list = appResourceResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list after delete response: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected deleted resource to be absent, got %#v", list.Items)
	}
	if list.Usage == nil || list.Usage.TotalCount != 0 || list.Usage.TotalSize != 0 {
		t.Fatalf("expected empty usage after delete, got %#v", list.Usage)
	}
}

func TestStoreResourceUsesConfiguredMaxUploadSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = filepath.Join(t.TempDir(), "app-attachments")
	cfg.MaxResourceUploadMB = 1
	bridge := NewBridge(cfg)

	tooLarge := bytes.NewReader(bytes.Repeat([]byte("x"), int(cfg.maxResourceUploadBytes()+1)))
	if _, err := bridge.StoreResource("alice", "live2d", "", "large.zip", "application/zip", tooLarge); err == nil {
		t.Fatalf("expected oversized resource to be rejected")
	}
}

func TestHandleResourcesRejectsOversizedUploadWith413(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = filepath.Join(t.TempDir(), "app-attachments")
	cfg.MaxResourceUploadMB = 1
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
	part, err := writer.CreateFormFile("file", "too-large.zip")
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), int(cfg.maxResourceUploadBytes()+1))); err != nil {
		t.Fatalf("write oversized file field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/app/resources", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-App-Agent-Session", "session-1")
	rec := httptest.NewRecorder()
	handler.HandleResources(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload expected 413, got %d body=%s", rec.Code, rec.Body.String())
	}
}
