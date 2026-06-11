package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleLogEndpoints(t *testing.T) {
	syncer := &stubCortanaSync{}
	cfg := DefaultConfig()
	cfg.DelegationSecretKey = "test-secret"
	auth := newAuthManager(cfg)
	bridge := NewBridge(cfg)
	settings := NewCortanaSettingsStore(filepath.Join(t.TempDir(), "cortana-settings.json"))
	bridge.SetCortanaSync(syncer, settings)
	handler := NewHandler(cfg, bridge, auth, syncer, settings, nil)

	logDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(logDir, "app.log"),
		[]byte("2026/05/02 12:00:00 first\n2026/05/02 12:00:01 second\n"),
		0644,
	); err != nil {
		t.Fatalf("write test log failed: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "log-agent.json")
	configBody := `{"log_sources":{"blog-agent":{"path":"` + filepath.ToSlash(logDir) + `","description":"blog logs"}}}`
	if err := os.WriteFile(configPath, []byte(configBody), 0644); err != nil {
		t.Fatalf("write log config failed: %v", err)
	}
	handler.cfg.LogAgentConfigFile = configPath

	issued, err := auth.issueAuthSession("alice")
	if err != nil {
		t.Fatalf("issue auth session failed: %v", err)
	}

	sessionToken := url.QueryEscape(issued.Session.Token)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/app/logs/sources?user_id=alice&session_token="+sessionToken,
		nil,
	)
	handler.HandleLogSources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sources expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var sourcesResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &sourcesResp); err != nil {
		t.Fatalf("decode sources response failed: %v", err)
	}
	if int(sourcesResp["count"].(float64)) != 1 {
		t.Fatalf("unexpected sources response: %v", sourcesResp)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(
		http.MethodGet,
		"/api/app/logs/files?user_id=alice&session_token="+sessionToken+"&source=blog-agent",
		nil,
	)
	handler.HandleLogFiles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("files expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var filesResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &filesResp); err != nil {
		t.Fatalf("decode files response failed: %v", err)
	}
	files := filesResp["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("unexpected files response: %v", filesResp)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(
		http.MethodGet,
		"/api/app/logs/content?user_id=alice&session_token="+sessionToken+"&source=blog-agent&file=app.log&lines=1",
		nil,
	)
	handler.HandleLogContent(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("content expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var contentResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &contentResp); err != nil {
		t.Fatalf("decode content response failed: %v", err)
	}
	content := strings.TrimSpace(contentResp["content"].(string))
	if content != "2026/05/02 12:00:01 second" {
		t.Fatalf("unexpected content: %q", content)
	}
}
