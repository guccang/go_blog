package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"downloadticket"
	"obsstore"
)

type fakeSignedURLStore struct {
	exists     bool
	url        *obsstore.SignedURL
	err        error
	putErr     error
	listResult *obsstore.ListObjectsResult
	listErr    error
	deleteErr  error
	meta       *obsstore.ObjectMeta
	metaErr    error
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

func (f *fakeSignedURLStore) CreateSignedPutURL(_ context.Context, _ string, _ string, _ time.Duration) (*obsstore.SignedURL, error) {
	return f.url, f.err
}

func (f *fakeSignedURLStore) PutObject(_ context.Context, _ obsstore.PutObjectRequest) error {
	return f.putErr
}

func (f *fakeSignedURLStore) ListObjects(_ context.Context, _ string, _ string, _ int) (*obsstore.ListObjectsResult, error) {
	return f.listResult, f.listErr
}

func (f *fakeSignedURLStore) DeleteObject(_ context.Context, _ string) error {
	return f.deleteErr
}

func (f *fakeSignedURLStore) GetObjectMeta(_ context.Context, _ string) (*obsstore.ObjectMeta, error) {
	return f.meta, f.metaErr
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

func TestHandleUploadReturnsSignedPutURL(t *testing.T) {
	handler := NewHandler(&Config{
		ReceiveToken:        "token",
		SignedURLTTLSeconds: 300,
	}, &fakeSignedURLStore{
		url: &obsstore.SignedURL{
			URL:       "https://obs.example.com/upload/20260419/123_data.csv?sig=put",
			Method:    "PUT",
			ExpiresAt: 1713520300000,
			Headers:   map[string]string{"Content-Type": "text/csv"},
		},
	}, nil)

	body := bytes.NewBufferString(`{"file_name":"data.csv","content_type":"text/csv"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/obs/upload", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["success"] != true {
		t.Fatalf("expected success=true, got %v", payload["success"])
	}
	if payload["method"] != "PUT" {
		t.Fatalf("expected method=PUT, got %v", payload["method"])
	}
	if payload["upload_url"] != "https://obs.example.com/upload/20260419/123_data.csv?sig=put" {
		t.Fatalf("unexpected upload_url: %v", payload["upload_url"])
	}
}

func TestHandleUploadRejectsMissingFileName(t *testing.T) {
	handler := NewHandler(&Config{
		ReceiveToken:        "token",
		SignedURLTTLSeconds: 300,
	}, &fakeSignedURLStore{}, nil)

	body := bytes.NewBufferString(`{"content_type":"text/csv"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/obs/upload", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUploadCustomObjectKey(t *testing.T) {
	var capturedKey string
	store := &fakeSignedURLStore{
		url: &obsstore.SignedURL{
			URL:       "https://obs.example.com/custom/report.csv?sig=put",
			Method:    "PUT",
			ExpiresAt: 1713520300000,
			Headers:   map[string]string{},
		},
	}
	handler := NewHandler(&Config{
		ReceiveToken:        "token",
		SignedURLTTLSeconds: 300,
	}, store, nil)

	body := bytes.NewBufferString(`{"file_name":"report.csv","object_key":"custom/report.csv"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/obs/upload", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["object_key"] != "custom/report.csv" {
		t.Fatalf("expected custom key, got %v", payload["object_key"])
	}
	_ = capturedKey
}

func TestHandleUploadUnauthorized(t *testing.T) {
	handler := NewHandler(&Config{
		ReceiveToken:        "token",
		SignedURLTTLSeconds: 300,
	}, &fakeSignedURLStore{}, nil)

	body := bytes.NewBufferString(`{"file_name":"data.csv"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/obs/upload", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpload(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleProxyUpload(t *testing.T) {
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{}, nil)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "test.txt")
	fmt.Fprint(part, "hello world")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/obs/proxy-upload", &buf)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.HandleProxyUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["success"] != true {
		t.Fatalf("expected success=true, got %v", payload["success"])
	}
	key, ok := payload["object_key"].(string)
	if !ok || key == "" {
		t.Fatalf("expected non-empty object_key, got %v", payload["object_key"])
	}
}

func TestHandleProxyUploadMissingFile(t *testing.T) {
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/obs/proxy-upload", bytes.NewBufferString("not multipart"))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.HandleProxyUpload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleList(t *testing.T) {
	now := time.Now()
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{
		listResult: &obsstore.ListObjectsResult{
			Objects: []obsstore.ObjectListItem{
				{Key: "upload/20260419/file1.csv", Size: 1024, LastModified: now, ETag: "abc"},
				{Key: "upload/20260419/file2.csv", Size: 2048, LastModified: now, ETag: "def"},
			},
			IsTruncated: false,
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/obs/list?prefix=upload/", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	json.Unmarshal(rec.Body.Bytes(), &payload)
	objects, ok := payload["objects"].([]any)
	if !ok || len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %v", payload["objects"])
	}
}

func TestHandleDelete(t *testing.T) {
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{}, nil)

	body := bytes.NewBufferString(`{"object_key":"upload/20260419/file1.csv"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/obs/delete", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["success"] != true {
		t.Fatalf("expected success=true")
	}
}

func TestHandleDeleteMissingKey(t *testing.T) {
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{}, nil)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/obs/delete", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleObjectInfo(t *testing.T) {
	now := time.Now()
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{
		meta: &obsstore.ObjectMeta{
			Key:          "upload/20260419/file1.csv",
			Size:         1024,
			ContentType:  "text/csv",
			LastModified: now,
			ETag:         "abc123",
			Metadata:     map[string]string{"x-custom": "val"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/obs/info?object_key=upload/20260419/file1.csv", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	handler.HandleObjectInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["content_type"] != "text/csv" {
		t.Fatalf("expected text/csv, got %v", payload["content_type"])
	}
	if payload["etag"] != "abc123" {
		t.Fatalf("expected abc123, got %v", payload["etag"])
	}
}

func TestHandleObjectInfoMissingKey(t *testing.T) {
	handler := NewHandler(&Config{ReceiveToken: "token"}, &fakeSignedURLStore{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/obs/info", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	handler.HandleObjectInfo(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
