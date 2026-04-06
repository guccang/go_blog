package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"downloadticket"
	"obsstore"
)

type signedURLStore interface {
	Enabled() bool
	HeadObject(ctx context.Context, key string) (bool, error)
	CreateSignedGetURL(ctx context.Context, key string, ttl time.Duration) (*obsstore.SignedURL, error)
}

type ticketVerifier interface {
	Enabled() bool
	Verify(token string) (*downloadticket.Claims, error)
}

type Handler struct {
	cfg    *Config
	store  signedURLStore
	signer ticketVerifier
}

func NewHandler(cfg *Config, store signedURLStore, signer ticketVerifier) *Handler {
	return &Handler{cfg: cfg, store: store, signer: signer}
}

func (h *Handler) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"obs_enabled": h.store != nil && h.store.Enabled(),
	})
}

func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil || !h.store.Enabled() || h.signer == nil || !h.signer.Enabled() {
		http.Error(w, "OBS download is not configured", http.StatusServiceUnavailable)
		return
	}

	fileID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/obs/download/"))
	if fileID == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}
	ticket := readDownloadTicket(r)
	if ticket == "" {
		http.Error(w, "download ticket is required", http.StatusUnauthorized)
		return
	}

	claims, err := h.signer.Verify(ticket)
	if err != nil {
		switch {
		case errors.Is(err, downloadticket.ErrExpired):
			http.Error(w, "download ticket expired", http.StatusUnauthorized)
		case errors.Is(err, downloadticket.ErrInvalid):
			http.Error(w, "invalid download ticket", http.StatusForbidden)
		default:
			http.Error(w, "download ticket verification failed", http.StatusForbidden)
		}
		return
	}
	if claims.FileID != fileID {
		http.Error(w, "download ticket does not match file_id", http.StatusForbidden)
		return
	}

	exists, err := h.store.HeadObject(r.Context(), claims.ObjectKey)
	if err != nil {
		log.Printf("[obs-agent] head object failed file_id=%s key=%s err=%v", fileID, claims.ObjectKey, err)
		http.Error(w, "obs lookup failed", http.StatusBadGateway)
		return
	}
	if !exists {
		http.NotFound(w, r)
		return
	}

	ttl := time.Duration(h.cfg.SignedURLTTLSeconds) * time.Second
	signed, err := h.store.CreateSignedGetURL(r.Context(), claims.ObjectKey, ttl)
	if err != nil {
		log.Printf("[obs-agent] create signed url failed file_id=%s key=%s err=%v", fileID, claims.ObjectKey, err)
		http.Error(w, "create signed url failed", http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"url":        signed.URL,
		"expires_at": signed.ExpiresAt,
		"method":     signed.Method,
		"headers":    signed.Headers,
	})
}

func (h *Handler) authorize(r *http.Request) bool {
	if h.cfg == nil || strings.TrimSpace(h.cfg.ReceiveToken) == "" {
		return true
	}
	return readBearerToken(r) == strings.TrimSpace(h.cfg.ReceiveToken)
}

func readBearerToken(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-App-Agent-Token"))
	if token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func readDownloadTicket(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-Download-Ticket")); token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("ticket"))
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
