package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

func (h *Handler) HandleCodegenHistoryBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

func buildCodegenHistoryBackupObjectKey(owner, backupType string, ts time.Time, fileName string) string {
	return filepath.ToSlash(filepath.Join(
		"app",
		"codegen-history",
		sanitizeFileName(firstNonEmpty(strings.TrimSpace(owner), "anonymous")),
		ts.Format("20060102"),
		sanitizeFileName(firstNonEmpty(strings.TrimSpace(backupType), "backup"))+"-"+sanitizeFileName(fileName),
	))
}
