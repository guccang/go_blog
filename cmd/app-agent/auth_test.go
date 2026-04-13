package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestAuthManager(t *testing.T) *authManager {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/app-auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
			})
		case "/api/app-auth/register":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"account": "group_demo_robot",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BlogAgentBaseURL = server.URL
	cfg.AppSessionTTLMinutes = 5
	cfg.AppRefreshTokenTTLHours = 24
	cfg.DelegationSecretKey = "test-secret"
	return newAuthManager(cfg)
}

func TestAuthManagerRefreshRotatesTokens(t *testing.T) {
	manager := newTestAuthManager(t)

	first, err := manager.Login("demo-user", "demo-password")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if first == nil || first.Session == nil {
		t.Fatalf("expected issued auth session")
	}
	if first.RefreshToken == "" {
		t.Fatalf("expected refresh token")
	}
	if !manager.Validate(first.Session.Token, "demo-user") {
		t.Fatalf("expected initial access token to validate")
	}

	second, err := manager.Refresh("demo-user", first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if second.Session.Token == first.Session.Token {
		t.Fatalf("expected refresh to rotate access token")
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatalf("expected refresh to rotate refresh token")
	}
	if manager.Validate(first.Session.Token, "demo-user") {
		t.Fatalf("expected old access token to be revoked")
	}
	if !manager.Validate(second.Session.Token, "demo-user") {
		t.Fatalf("expected refreshed access token to validate")
	}

	if _, err := manager.Refresh("demo-user", first.RefreshToken); err == nil {
		t.Fatalf("expected old refresh token to be revoked")
	}
}

func TestAuthManagerLogoutRevokesRefreshToken(t *testing.T) {
	manager := newTestAuthManager(t)

	issued, err := manager.Login("demo-user", "demo-password")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	manager.Logout(issued.Session.Token, issued.RefreshToken, "demo-user")

	if manager.Validate(issued.Session.Token, "demo-user") {
		t.Fatalf("expected logout to revoke access token")
	}
	if _, err := manager.Refresh("demo-user", issued.RefreshToken); err == nil {
		t.Fatalf("expected logout to revoke refresh token")
	}
}
