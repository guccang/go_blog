package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"obsstore"
)

const maxCodegenHistoryBackupBytes int64 = 16 << 20

type codegenHistoryBackupRequest struct {
	UserID     string           `json:"user_id"`
	BackupType string           `json:"backup_type"`
	AppVersion string           `json:"app_version,omitempty"`
	Platform   string           `json:"platform,omitempty"`
	History    []map[string]any `json:"history"`
}

type codegenHistoryBackupResponse struct {
	Success         bool   `json:"success"`
	BackupType      string `json:"backup_type"`
	FileName        string `json:"file_name"`
	FileSize        int64  `json:"file_size"`
	StorageProvider string `json:"storage_provider"`
	ObjectKey       string `json:"object_key,omitempty"`
	UpdatedAt       int64  `json:"updated_at"`
	Error           string `json:"error,omitempty"`
}

type codegenHistoryBackupListItem struct {
	BackupType string `json:"backup_type"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	ObjectKey  string `json:"object_key"`
	CreatedAt  int64  `json:"created_at"`
}

type codegenHistoryBackupListResponse struct {
	Success bool                           `json:"success"`
	Items   []codegenHistoryBackupListItem `json:"items"`
}

type codegenHistoryBackupLoadResponse struct {
	Success    bool             `json:"success"`
	ObjectKey  string           `json:"object_key"`
	FileName   string           `json:"file_name"`
	BackupType string           `json:"backup_type"`
	CreatedAt  int64            `json:"created_at"`
	History    []map[string]any `json:"history"`
}

func (h *Handler) HandleCodegenHistoryBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet {
		h.handleGetCodegenHistoryBackups(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCodegenHistoryBackupBytes)
	defer r.Body.Close()

	var req codegenHistoryBackupRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.BackupType = normalizeCodegenHistoryBackupType(req.BackupType)
	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if req.BackupType == "" {
		http.Error(w, "backup_type must be full or incremental", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, req.UserID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	result, err := h.bridge.StoreCodegenHistoryBackup(req)
	if err != nil {
		logHandlerError(w, "store codegen history backup failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleGetCodegenHistoryBackups(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, userID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	objectKey := strings.TrimSpace(r.URL.Query().Get("object_key"))
	if objectKey != "" {
		result, err := h.bridge.LoadCodegenHistoryBackup(userID, objectKey)
		if err != nil {
			logHandlerError(w, "load codegen history backup failed", err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(result)
		return
	}

	result, err := h.bridge.ListCodegenHistoryBackups(userID)
	if err != nil {
		logHandlerError(w, "list codegen history backups failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(result)
}

func (b *Bridge) StoreCodegenHistoryBackup(req codegenHistoryBackupRequest) (*codegenHistoryBackupResponse, error) {
	owner := strings.TrimSpace(req.UserID)
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	backupType := normalizeCodegenHistoryBackupType(req.BackupType)
	if backupType == "" {
		return nil, fmt.Errorf("invalid backup type")
	}
	now := time.Now()
	payload := map[string]any{
		"kind":           "codegen_history_backup",
		"schema_version": 1,
		"user_id":        owner,
		"backup_type":    backupType,
		"app_version":    strings.TrimSpace(req.AppVersion),
		"platform":       strings.TrimSpace(req.Platform),
		"history_count":  len(req.History),
		"history":        req.History,
		"created_at":     now.UnixMilli(),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal codegen history backup: %w", err)
	}
	data = append(data, '\n')
	if int64(len(data)) > maxCodegenHistoryBackupBytes {
		return nil, fmt.Errorf("codegen history backup exceeds max size")
	}

	root := attachmentRootDir(b.cfg.AttachmentStoreDir)
	dir := filepath.Join(root, sanitizeFileName(owner), "codegen-history", now.Format("20060102"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir codegen history backup dir: %w", err)
	}
	fileName := fmt.Sprintf("%s-%s.json", backupType, now.Format("20060102-150405.000"))
	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("write codegen history backup: %w", err)
	}

	storageProvider := "local"
	objectKey := ""
	if b.obsStorage != nil && b.obsStorage.Enabled() {
		objectKey = buildCodegenHistoryBackupObjectKey(owner, backupType, now, fileName)
		if err := b.obsStorage.PutObject(context.Background(), obsstore.PutObjectRequest{
			Key:         objectKey,
			Body:        bytes.NewReader(data),
			Size:        int64(len(data)),
			ContentType: "application/json; charset=utf-8",
			Metadata: map[string]string{
				"owner":        owner,
				"backup_type":  backupType,
				"history_kind": "codegen",
			},
		}); err != nil {
			log.Printf("[Bridge] upload codegen history backup to OBS failed owner=%s type=%s key=%s err=%v",
				owner, backupType, objectKey, err)
		} else {
			storageProvider = "obs"
		}
	}

	return &codegenHistoryBackupResponse{
		Success:         true,
		BackupType:      backupType,
		FileName:        fileName,
		FileSize:        int64(len(data)),
		StorageProvider: storageProvider,
		ObjectKey:       objectKey,
		UpdatedAt:       now.UnixMilli(),
	}, nil
}

func (b *Bridge) ListCodegenHistoryBackups(userID string) (*codegenHistoryBackupListResponse, error) {
	owner := strings.TrimSpace(userID)
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	items := make([]codegenHistoryBackupListItem, 0)
	if b.obsStorage != nil && b.obsStorage.Enabled() {
		prefix := filepath.ToSlash(filepath.Join("app", "codegen-history", sanitizeFileName(owner))) + "/"
		list, err := b.obsStorage.ListObjects(context.Background(), prefix, "", 100)
		if err != nil {
			return nil, fmt.Errorf("list codegen history backups from OBS: %w", err)
		}
		for _, obj := range list.Objects {
			if !strings.HasSuffix(strings.ToLower(obj.Key), ".json") {
				continue
			}
			items = append(items, codegenHistoryBackupListItem{
				BackupType: backupTypeFromCodegenBackupName(filepath.Base(obj.Key)),
				FileName:   filepath.Base(obj.Key),
				FileSize:   obj.Size,
				ObjectKey:  obj.Key,
				CreatedAt:  obj.LastModified.UnixMilli(),
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})
	return &codegenHistoryBackupListResponse{Success: true, Items: items}, nil
}

func (b *Bridge) LoadCodegenHistoryBackup(userID, objectKey string) (*codegenHistoryBackupLoadResponse, error) {
	owner := strings.TrimSpace(userID)
	key := strings.TrimSpace(objectKey)
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	if key == "" {
		return nil, fmt.Errorf("object_key is required")
	}
	ownerPrefix := filepath.ToSlash(filepath.Join("app", "codegen-history", sanitizeFileName(owner))) + "/"
	if !strings.HasPrefix(key, ownerPrefix) {
		return nil, fmt.Errorf("object_key does not belong to user")
	}
	if result, err := b.loadLocalCodegenHistoryBackup(owner, key); err == nil {
		return result, nil
	} else if !os.IsNotExist(err) {
		log.Printf("[Bridge] load local codegen history backup failed owner=%s key=%s err=%v", owner, key, err)
	}
	if b.obsStorage == nil || !b.obsStorage.Enabled() {
		return nil, fmt.Errorf("OBS is not configured")
	}
	signed, err := b.obsStorage.CreateSignedGetURL(context.Background(), key, 2*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("create codegen history backup download URL: %w", err)
	}
	if strings.TrimSpace(signed.URL) == "" {
		return nil, fmt.Errorf("codegen history backup download URL is empty")
	}
	req, err := http.NewRequest(firstNonEmpty(strings.TrimSpace(signed.Method), http.MethodGet), signed.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build codegen history backup download request: %w", err)
	}
	for header, value := range signed.Headers {
		if strings.TrimSpace(header) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(header, value)
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download codegen history backup: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download codegen history backup failed: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCodegenHistoryBackupBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read codegen history backup: %w", err)
	}
	if int64(len(data)) > maxCodegenHistoryBackupBytes {
		return nil, fmt.Errorf("codegen history backup exceeds max size")
	}
	return decodeCodegenHistoryBackup(key, data)
}

func (b *Bridge) loadLocalCodegenHistoryBackup(owner, objectKey string) (*codegenHistoryBackupLoadResponse, error) {
	paths := localCodegenHistoryBackupPaths(b.cfg.AttachmentStoreDir, owner, objectKey)
	if len(paths) == 0 {
		return nil, os.ErrNotExist
	}
	var lastErr error = os.ErrNotExist
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		if int64(len(data)) > maxCodegenHistoryBackupBytes {
			return nil, fmt.Errorf("codegen history backup exceeds max size")
		}
		return decodeCodegenHistoryBackup(objectKey, data)
	}
	return nil, lastErr
}

func localCodegenHistoryBackupPaths(rootDir, owner, objectKey string) []string {
	owner = sanitizeFileName(strings.TrimSpace(owner))
	parts := strings.Split(filepath.ToSlash(filepath.Clean(strings.TrimSpace(objectKey))), "/")
	if owner == "" || len(parts) < 5 || parts[0] != "app" || parts[1] != "codegen-history" || parts[2] != owner {
		return nil
	}
	day := sanitizeFileName(parts[3])
	name := sanitizeFileName(filepath.Base(parts[len(parts)-1]))
	if day == "" || name == "" {
		return nil
	}
	names := []string{name}
	if backupType := backupTypeFromCodegenBackupName(name); backupType != "" {
		doublePrefix := backupType + "-" + backupType + "-"
		if strings.HasPrefix(name, doublePrefix) {
			names = append(names, strings.TrimPrefix(name, backupType+"-"))
		}
	}
	paths := make([]string, 0, len(names))
	for _, candidate := range names {
		paths = append(paths, filepath.Join(
			attachmentRootDir(rootDir),
			owner,
			"codegen-history",
			day,
			candidate,
		))
	}
	return paths
}

func decodeCodegenHistoryBackup(objectKey string, data []byte) (*codegenHistoryBackupLoadResponse, error) {
	var payload struct {
		BackupType string           `json:"backup_type"`
		CreatedAt  int64            `json:"created_at"`
		History    []map[string]any `json:"history"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode codegen history backup: %w", err)
	}
	key := strings.TrimSpace(objectKey)
	return &codegenHistoryBackupLoadResponse{
		Success:    true,
		ObjectKey:  key,
		FileName:   filepath.Base(key),
		BackupType: normalizeCodegenHistoryBackupType(payload.BackupType),
		CreatedAt:  payload.CreatedAt,
		History:    payload.History,
	}, nil
}

func normalizeCodegenHistoryBackupType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full":
		return "full"
	case "incremental", "increment":
		return "incremental"
	default:
		return ""
	}
}

func backupTypeFromCodegenBackupName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(name, "incremental-") {
		return "incremental"
	}
	if strings.HasPrefix(name, "full-") {
		return "full"
	}
	return ""
}

func buildCodegenHistoryBackupObjectKey(owner, backupType string, ts time.Time, fileName string) string {
	return filepath.ToSlash(filepath.Join(
		"app",
		"codegen-history",
		sanitizeFileName(firstNonEmpty(strings.TrimSpace(owner), "anonymous")),
		ts.Format("20060102"),
		sanitizeFileName(firstNonEmpty(strings.TrimSpace(backupType), "backup"))+"-"+sanitizeFileName(fileName),
	))
}
