package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"downloadticket"
	"obsstore"
)

type fakeSignedURLStore struct {
	exists bool
	url    *obsstore.SignedURL
	err    error
}

func (f *fakeSignedURLStore) Enabled() bool {
	return true
}

func (f *fakeSignedURLStore) HeadObject(_ context.Context, _ string) (bool, error) {
	return f.exists, f.err
}

func (f *fakeSignedURLStore) CreateSignedGetURL(_ context.Context, _ string, _ time.Duration) (*obsstore.SignedURL, error) {
	return f.url, f.err
}

func TestHandleDownloadReturnsSignedURL(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	signer := downloadticket.NewSignerWithClock("secret", func() time.Time { return now })
	token, _, err := signer.Issue(downloadticket.Input{
		FileID:    "file-1",
		UserID:    "alice",
		ObjectKey: "app/file/alice/demo.apk",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	handler := NewHandler(&Config{
		ReceiveToken:        "token",
		SignedURLTTLSeconds: 300,
	}, &fakeSignedURLStore{
		exists: true,
		url: &obsstore.SignedURL{
			URL:       "https://obs.example.com/demo.apk?sig=1",
			Method:    "GET",
			ExpiresAt: now.Add(5 * time.Minute).UnixMilli(),
			Headers:   map[string]string{},
		},
	}, signer)

	req := httptest.NewRequest(http.MethodGet, "/api/obs/download/file-1?ticket="+token, nil)
	req.Header.Set("X-App-Agent-Token", "token")
	rec := httptest.NewRecorder()

	handler.HandleDownload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["url"] != "https://obs.example.com/demo.apk?sig=1" {
		t.Fatalf("unexpected signed url: %#v", payload["url"])
	}
}

func TestHandleDownloadRejectsTicketMismatch(t *testing.T) {
	signer := downloadticket.NewSigner("secret")
	token, _, err := signer.Issue(downloadticket.Input{
		FileID:    "file-1",
		UserID:    "alice",
		ObjectKey: "app/file/alice/demo.apk",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	handler := NewHandler(&Config{}, &fakeSignedURLStore{exists: true}, signer)
	req := httptest.NewRequest(http.MethodGet, "/api/obs/download/file-2?ticket="+token, nil)
	rec := httptest.NewRecorder()

	handler.HandleDownload(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestHandleDownloadReturnsNotFoundWhenObjectMissing(t *testing.T) {
	signer := downloadticket.NewSigner("secret")
	token, _, err := signer.Issue(downloadticket.Input{
		FileID:    "file-1",
		UserID:    "alice",
		ObjectKey: "missing.apk",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	handler := NewHandler(&Config{}, &fakeSignedURLStore{exists: false}, signer)
	req := httptest.NewRequest(http.MethodGet, "/api/obs/download/file-1?ticket="+token, nil)
	rec := httptest.NewRecorder()

	handler.HandleDownload(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
