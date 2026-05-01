package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubCortanaSync struct {
	registerCalls   []string
	unregisterCalls []string
	registerErr     error
	unregisterErr   error
}

func (s *stubCortanaSync) RegisterAccount(account string) error {
	s.registerCalls = append(s.registerCalls, account)
	return s.registerErr
}

func (s *stubCortanaSync) UnregisterAccount(account string) error {
	s.unregisterCalls = append(s.unregisterCalls, account)
	return s.unregisterErr
}

func newTestHandler(t *testing.T, syncer cortanaAccountSync) (*Handler, *authManager) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.DelegationSecretKey = "test-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path != "/api/app-auth/login" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	t.Cleanup(server.Close)
	cfg.BlogAgentBaseURL = server.URL

	auth := newAuthManager(cfg)
	bridge := NewBridge(cfg)
	return NewHandler(cfg, bridge, auth, syncer), auth
}

func TestHandleLoginRegistersCortanaAccount(t *testing.T) {
	syncer := &stubCortanaSync{}
	handler, _ := newTestHandler(t, syncer)

	body := bytes.NewBufferString(`{"user_id":"alice","password":"pw"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/app/login", body)
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(syncer.registerCalls) != 1 || syncer.registerCalls[0] != "alice" {
		t.Fatalf("unexpected register calls: %#v", syncer.registerCalls)
	}
}

func TestHandleRefreshRegistersCortanaAccountEvenIfSyncFails(t *testing.T) {
	syncer := &stubCortanaSync{registerErr: errors.New("boom")}
	handler, auth := newTestHandler(t, syncer)

	issued, err := auth.Login("alice", "pw")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	body := bytes.NewBufferString(`{"user_id":"alice","refresh_token":"` + issued.RefreshToken + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/app/refresh", body)
	rec := httptest.NewRecorder()

	handler.HandleRefresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(syncer.registerCalls) != 1 || syncer.registerCalls[0] != "alice" {
		t.Fatalf("unexpected register calls: %#v", syncer.registerCalls)
	}
}

func TestHandleLogoutUnregistersResolvedAccountEvenIfSyncFails(t *testing.T) {
	syncer := &stubCortanaSync{unregisterErr: errors.New("boom")}
	handler, auth := newTestHandler(t, syncer)

	issued, err := auth.Login("alice", "pw")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/app/logout", bytes.NewBufferString(`{}`))
	req.Header.Set("X-App-Agent-Session", issued.Session.Token)
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(syncer.unregisterCalls) != 1 || syncer.unregisterCalls[0] != "alice" {
		t.Fatalf("unexpected unregister calls: %#v", syncer.unregisterCalls)
	}
}
