package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	CreateSignedPutURL(ctx context.Context, key, contentType string, ttl time.Duration) (*obsstore.SignedURL, error)
	PutObject(ctx context.Context, req obsstore.PutObjectRequest) error
	ListObjects(ctx context.Context, prefix string, marker string, maxKeys int) (*obsstore.ListObjectsResult, error)
	DeleteObject(ctx context.Context, key string) error
	GetObjectMeta(ctx context.Context, key string) (*obsstore.ObjectMeta, error)
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
	log.Printf("[obs-agent] download request file_id=%s remote=%s", fileID, r.RemoteAddr)

	ticket := readDownloadTicket(r)
	if ticket == "" {
		log.Printf("[obs-agent] download rejected: missing ticket file_id=%s", fileID)
		http.Error(w, "download ticket is required", http.StatusUnauthorized)
		return
	}

	claims, err := h.signer.Verify(ticket)
	if err != nil {
		log.Printf("[obs-agent] download ticket verify failed file_id=%s err=%v", fileID, err)
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
		log.Printf("[obs-agent] download ticket mismatch file_id=%s ticket_file_id=%s", fileID, claims.FileID)
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
		log.Printf("[obs-agent] download object not found file_id=%s key=%s", fileID, claims.ObjectKey)
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

	log.Printf("[obs-agent] download ok file_id=%s key=%s ttl=%ds", fileID, claims.ObjectKey, h.cfg.SignedURLTTLSeconds)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"url":        signed.URL,
		"expires_at": signed.ExpiresAt,
		"method":     signed.Method,
		"headers":    signed.Headers,
	})
}

func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil || !h.store.Enabled() {
		http.Error(w, "OBS upload is not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FileName    string `json:"file_name"`
		ObjectKey   string `json:"object_key"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.FileName = strings.TrimSpace(req.FileName)
	if req.FileName == "" {
		http.Error(w, "file_name is required", http.StatusBadRequest)
		return
	}
	log.Printf("[obs-agent] upload request file_name=%s object_key=%s content_type=%s remote=%s",
		req.FileName, req.ObjectKey, req.ContentType, r.RemoteAddr)

	objectKey := strings.TrimSpace(req.ObjectKey)
	if objectKey == "" {
		objectKey = buildUploadObjectKey(req.FileName)
	}

	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ttl := time.Duration(h.cfg.SignedURLTTLSeconds) * time.Second
	signed, err := h.store.CreateSignedPutURL(r.Context(), objectKey, contentType, ttl)
	if err != nil {
		log.Printf("[obs-agent] create signed put url failed key=%s err=%v", objectKey, err)
		http.Error(w, "create signed upload url failed", http.StatusBadGateway)
		return
	}

	log.Printf("[obs-agent] upload ok key=%s content_type=%s ttl=%ds", objectKey, contentType, h.cfg.SignedURLTTLSeconds)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"object_key": objectKey,
		"upload_url": signed.URL,
		"method":     signed.Method,
		"expires_at": signed.ExpiresAt,
		"headers":    signed.Headers,
	})
}

func buildUploadObjectKey(fileName string) string {
	now := time.Now()
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == ' ' {
			return '_'
		}
		return r
	}, strings.TrimSpace(fileName))
	if safe == "" {
		safe = "file"
	}
	return fmt.Sprintf("upload/%s/%d_%s", now.Format("20060102"), now.UnixMilli(), safe)
}

func (h *Handler) HandleProxyUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil || !h.store.Enabled() {
		http.Error(w, "OBS upload is not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	objectKey := strings.TrimSpace(r.FormValue("object_key"))
	if objectKey == "" {
		objectKey = buildUploadObjectKey(header.Filename)
	}
	contentType := strings.TrimSpace(r.FormValue("content_type"))
	if contentType == "" {
		contentType = header.Header.Get("Content-Type")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	log.Printf("[obs-agent] proxy-upload request filename=%s size=%d key=%s content_type=%s remote=%s",
		header.Filename, header.Size, objectKey, contentType, r.RemoteAddr)

	if err := h.store.PutObject(r.Context(), obsstore.PutObjectRequest{
		Key:         objectKey,
		Body:        file,
		Size:        header.Size,
		ContentType: contentType,
	}); err != nil {
		log.Printf("[obs-agent] proxy upload failed key=%s err=%v", objectKey, err)
		http.Error(w, "upload to obs failed", http.StatusBadGateway)
		return
	}

	log.Printf("[obs-agent] proxy-upload ok key=%s size=%d", objectKey, header.Size)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"object_key": objectKey,
		"size":       header.Size,
	})
}

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil || !h.store.Enabled() {
		http.Error(w, "OBS is not configured", http.StatusServiceUnavailable)
		return
	}

	prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
	marker := strings.TrimSpace(r.URL.Query().Get("marker"))
	maxKeys := 100
	log.Printf("[obs-agent] list request prefix=%s marker=%s remote=%s", prefix, marker, r.RemoteAddr)

	result, err := h.store.ListObjects(r.Context(), prefix, marker, maxKeys)
	if err != nil {
		log.Printf("[obs-agent] list objects failed prefix=%s err=%v", prefix, err)
		http.Error(w, "list objects failed", http.StatusBadGateway)
		return
	}

	type item struct {
		Key          string `json:"key"`
		Size         int64  `json:"size"`
		LastModified int64  `json:"last_modified"`
		ETag         string `json:"etag"`
	}
	items := make([]item, 0, len(result.Objects))
	for _, obj := range result.Objects {
		items = append(items, item{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.UnixMilli(),
			ETag:         obj.ETag,
		})
	}

	log.Printf("[obs-agent] list ok prefix=%s count=%d truncated=%v", prefix, len(items), result.IsTruncated)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"objects":      items,
		"is_truncated": result.IsTruncated,
		"next_marker":  result.NextMarker,
	})
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil || !h.store.Enabled() {
		http.Error(w, "OBS is not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ObjectKey string `json:"object_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.ObjectKey = strings.TrimSpace(req.ObjectKey)
	if req.ObjectKey == "" {
		http.Error(w, "object_key is required", http.StatusBadRequest)
		return
	}
	log.Printf("[obs-agent] delete request key=%s remote=%s", req.ObjectKey, r.RemoteAddr)

	if err := h.store.DeleteObject(r.Context(), req.ObjectKey); err != nil {
		log.Printf("[obs-agent] delete object failed key=%s err=%v", req.ObjectKey, err)
		http.Error(w, "delete object failed", http.StatusBadGateway)
		return
	}

	log.Printf("[obs-agent] delete ok key=%s", req.ObjectKey)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"object_key": req.ObjectKey,
	})
}

func (h *Handler) HandleObjectInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil || !h.store.Enabled() {
		http.Error(w, "OBS is not configured", http.StatusServiceUnavailable)
		return
	}

	objectKey := strings.TrimSpace(r.URL.Query().Get("object_key"))
	if objectKey == "" {
		http.Error(w, "object_key is required", http.StatusBadRequest)
		return
	}
	log.Printf("[obs-agent] info request key=%s remote=%s", objectKey, r.RemoteAddr)

	meta, err := h.store.GetObjectMeta(r.Context(), objectKey)
	if err != nil {
		log.Printf("[obs-agent] get object meta failed key=%s err=%v", objectKey, err)
		http.Error(w, "get object info failed", http.StatusBadGateway)
		return
	}

	log.Printf("[obs-agent] info ok key=%s size=%d type=%s", meta.Key, meta.Size, meta.ContentType)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"object_key":    meta.Key,
		"size":          meta.Size,
		"content_type":  meta.ContentType,
		"last_modified": meta.LastModified.UnixMilli(),
		"etag":          meta.ETag,
		"metadata":      meta.Metadata,
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
