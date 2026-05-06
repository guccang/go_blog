package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleCreateDebugBundleRedactsAndWritesFiles(t *testing.T) {
	syncer := &stubCortanaSync{}
	cfg := DefaultConfig()
	cfg.DelegationSecretKey = "test-secret"
	cfg.DebugBundleDir = filepath.Join(t.TempDir(), ".debug", "flutter")
	auth := newAuthManager(cfg)
	bridge := NewBridge(cfg)
	settings := NewCortanaSettingsStore(filepath.Join(t.TempDir(), "cortana-settings.json"))
	bridge.SetCortanaSync(syncer, settings)
	handler := NewHandler(cfg, bridge, auth, syncer, settings)

	logDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(logDir, "app.log"), []byte("Authorization: Bearer secret-token\nnormal\n"), 0644); err != nil {
		t.Fatalf("write test log failed: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "log-agent.json")
	configBody := `{"log_sources":{"app-agent":{"path":"` + filepath.ToSlash(logDir) + `","description":"app logs"}}}`
	if err := os.WriteFile(configPath, []byte(configBody), 0644); err != nil {
		t.Fatalf("write log config failed: %v", err)
	}
	handler.cfg.LogAgentConfigFile = configPath

	issued, err := auth.issueAuthSession("alice")
	if err != nil {
		t.Fatalf("issue auth session failed: %v", err)
	}
	body, _ := json.Marshal(map[string]any{
		"user_id": "alice",
		"issue": map[string]any{
			"title":            "WebSocket reconnect",
			"user_description": "token=abc123 should be hidden",
			"repro_steps":      []string{"open debug tab"},
		},
		"app_state":   map[string]any{"root_tab": "debug"},
		"client_logs": []string{"session_token=client-secret", "client normal"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/app/debug/bundles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Agent-Session", issued.Session.Token)
	handler.HandleDebugBundles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create debug bundle expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	debugID := resp["debug_id"].(string)
	if debugID == "" {
		t.Fatalf("missing debug_id in response: %v", resp)
	}
	clientLog, err := os.ReadFile(filepath.Join(cfg.DebugBundleDir, debugID, "logs", "flutter_client.log"))
	if err != nil {
		t.Fatalf("read client log failed: %v", err)
	}
	if strings.Contains(string(clientLog), "client-secret") {
		t.Fatalf("client log was not redacted: %s", clientLog)
	}
	serverLog, err := os.ReadFile(filepath.Join(cfg.DebugBundleDir, debugID, "logs", "app-agent.log"))
	if err != nil {
		t.Fatalf("read server log failed: %v", err)
	}
	if strings.Contains(string(serverLog), "secret-token") {
		t.Fatalf("server log was not redacted: %s", serverLog)
	}

	rec = httptest.NewRecorder()
	readReq := httptest.NewRequest(
		http.MethodGet,
		"/api/app/debug/bundles/"+debugID+"?user_id=alice&session_token="+url.QueryEscape(issued.Session.Token),
		nil,
	)
	handler.HandleDebugBundleItem(rec, readReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("read debug bundle expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
