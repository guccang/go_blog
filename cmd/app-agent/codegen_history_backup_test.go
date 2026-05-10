package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreCodegenHistoryBackupUploadsOBS(t *testing.T) {
	dir := t.TempDir()
	bridge := NewBridge(DefaultConfig())
	bridge.cfg.AttachmentStoreDir = filepath.Join(dir, "attachments")
	fakeStore := &fakeOBSStorage{}
	bridge.obsStorage = fakeStore

	resp, err := bridge.StoreCodegenHistoryBackup(codegenHistoryBackupRequest{
		UserID:     "alice",
		BackupType: "incremental",
		AppVersion: "1.2.3",
		Platform:   "android",
		History: []map[string]any{
			{
				"id":      "cg-1",
				"command": "/cg start demo@app @codex fix",
				"mode":    "code",
			},
		},
	})
	if err != nil {
		t.Fatalf("StoreCodegenHistoryBackup returned error: %v", err)
	}
	if !resp.Success || resp.StorageProvider != "obs" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if !strings.Contains(resp.ObjectKey, "app/codegen-history/alice/") {
		t.Fatalf("unexpected object key: %q", resp.ObjectKey)
	}
	if fakeStore.putCount != 1 {
		t.Fatalf("expected one OBS upload, got %d", fakeStore.putCount)
	}
	body := fakeStore.bodies[resp.ObjectKey]
	if len(body) == 0 {
		t.Fatalf("missing OBS body for %s", resp.ObjectKey)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode backup payload: %v", err)
	}
	if payload["backup_type"] != "incremental" || payload["history_count"] != float64(1) {
		t.Fatalf("unexpected payload: %#v", payload)
	}

	localPath := filepath.Join(
		bridge.cfg.AttachmentStoreDir,
		"alice",
		"codegen-history",
		time.UnixMilli(resp.UpdatedAt).Format("20060102"),
		resp.FileName,
	)
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected local backup file %s: %v", localPath, err)
	}
}

func TestLoadCodegenHistoryBackupFallsBackToLocalFile(t *testing.T) {
	dir := t.TempDir()
	bridge := NewBridge(DefaultConfig())
	bridge.cfg.AttachmentStoreDir = filepath.Join(dir, "attachments")
	fakeStore := &fakeOBSStorage{}
	bridge.obsStorage = fakeStore

	stored, err := bridge.StoreCodegenHistoryBackup(codegenHistoryBackupRequest{
		UserID:     "alice",
		BackupType: "full",
		History: []map[string]any{
			{
				"id":      "cg-local",
				"command": "/cg start demo@app @codex fix",
			},
		},
	})
	if err != nil {
		t.Fatalf("StoreCodegenHistoryBackup returned error: %v", err)
	}

	fakeStore.bodies = nil
	loaded, err := bridge.LoadCodegenHistoryBackup("alice", stored.ObjectKey)
	if err != nil {
		t.Fatalf("LoadCodegenHistoryBackup returned error: %v", err)
	}
	if !loaded.Success || loaded.BackupType != "full" {
		t.Fatalf("unexpected load response: %#v", loaded)
	}
	if len(loaded.History) != 1 || loaded.History[0]["id"] != "cg-local" {
		t.Fatalf("unexpected loaded history: %#v", loaded.History)
	}
}
