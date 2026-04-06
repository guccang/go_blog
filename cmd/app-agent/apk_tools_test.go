package main

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestNewBridgeIncludesAPKManagementTools(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)

	toolNames := make(map[string]bool, len(bridge.client.Tools))
	for _, tool := range bridge.client.Tools {
		toolNames[tool.Name] = true
	}

	for _, name := range []string{
		"app.ListAPKPackages",
		"app.PushAPKPackage",
	} {
		if !toolNames[name] {
			t.Fatalf("expected tool %s to be registered", name)
		}
	}
}

func TestListStoredAPKPackagesReturnsNewestFirst(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)

	oldAttachment, err := bridge.PushUploadedAPK(
		"ztt",
		"",
		"app-release-1.0.0.apk",
		bytes.NewReader([]byte("apk-v1")),
	)
	if err != nil {
		t.Fatalf("PushUploadedAPK old returned error: %v", err)
	}
	newAttachment, err := bridge.PushUploadedAPK(
		"alice",
		"",
		"app-release-1.0.1.apk",
		bytes.NewReader([]byte("apk-v2")),
	)
	if err != nil {
		t.Fatalf("PushUploadedAPK new returned error: %v", err)
	}

	oldPath, err := resolveAttachmentPath(bridge.cfg.AttachmentStoreDir, oldAttachment.FileID)
	if err != nil {
		t.Fatalf("resolveAttachmentPath old returned error: %v", err)
	}
	newPath, err := resolveAttachmentPath(bridge.cfg.AttachmentStoreDir, newAttachment.FileID)
	if err != nil {
		t.Fatalf("resolveAttachmentPath new returned error: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(oldPath, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatalf("Chtimes old returned error: %v", err)
	}
	if err := os.Chtimes(newPath, now, now); err != nil {
		t.Fatalf("Chtimes new returned error: %v", err)
	}

	items, err := bridge.listStoredAPKPackages("", "", 10)
	if err != nil {
		t.Fatalf("listStoredAPKPackages returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 apk packages, got %d", len(items))
	}
	if items[0].FileName != "app-release-1.0.1.apk" {
		t.Fatalf("expected newest apk first, got %q", items[0].FileName)
	}
	if items[0].Owner != "alice" {
		t.Fatalf("expected owner alice, got %q", items[0].Owner)
	}
	if items[0].Version != "1.0.1" {
		t.Fatalf("expected version 1.0.1, got %q", items[0].Version)
	}
	resolvedPath, err := resolveAttachmentPath(bridge.cfg.AttachmentStoreDir, items[0].FileID)
	if err != nil {
		t.Fatalf("resolveAttachmentPath from listed file_id returned error: %v", err)
	}
	if resolvedPath != newPath {
		t.Fatalf("expected listed file_id to resolve to %s, got %s", newPath, resolvedPath)
	}
}

func TestPushStoredAPKPackageToUserReusesExistingFile(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)

	if _, err := bridge.PushUploadedAPK(
		"repo",
		"",
		"app-release-2.0.0.apk",
		bytes.NewReader([]byte("apk-binary")),
	); err != nil {
		t.Fatalf("PushUploadedAPK seed returned error: %v", err)
	}
	items, err := bridge.listStoredAPKPackages("repo", "", 10)
	if err != nil {
		t.Fatalf("listStoredAPKPackages returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stored apk package, got %d", len(items))
	}

	result, err := bridge.pushStoredAPKPackage(
		"alice",
		"",
		items[0].FileID,
		"",
		"",
		"请安装这个版本",
	)
	if err != nil {
		t.Fatalf("pushStoredAPKPackage returned error: %v", err)
	}

	queue := bridge.pendingForUser("alice")
	if len(queue) != 1 {
		t.Fatalf("expected alice to have 1 pending message, got %d", len(queue))
	}
	if queue[0].Content != "请安装这个版本" {
		t.Fatalf("expected custom content to be queued, got %q", queue[0].Content)
	}
	if got, _ := queue[0].Meta["file_id"].(string); got != items[0].FileID {
		t.Fatalf("expected queued file_id %s, got %s", items[0].FileID, got)
	}
	if got, _ := result["source_file_id"].(string); got != items[0].FileID {
		t.Fatalf("expected result source_file_id %s, got %s", items[0].FileID, got)
	}

	allItems, err := bridge.listStoredAPKPackages("", "", 10)
	if err != nil {
		t.Fatalf("listStoredAPKPackages after push returned error: %v", err)
	}
	if len(allItems) != 1 {
		t.Fatalf("expected push to reuse existing apk file, got %d stored packages", len(allItems))
	}
}

func TestPushStoredAPKPackageToGroupQueuesHumanMembers(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true},
		RobotAccount: "robot-g1",
	}

	if _, err := bridge.PushUploadedAPK(
		"repo",
		"",
		"app-release-3.0.0.apk",
		bytes.NewReader([]byte("apk-binary")),
	); err != nil {
		t.Fatalf("PushUploadedAPK seed returned error: %v", err)
	}
	items, err := bridge.listStoredAPKPackages("repo", "", 10)
	if err != nil {
		t.Fatalf("listStoredAPKPackages returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stored apk package, got %d", len(items))
	}

	result, err := bridge.pushStoredAPKPackage(
		"",
		"g1",
		items[0].FileID,
		"",
		"",
		"群安装包已下发",
	)
	if err != nil {
		t.Fatalf("pushStoredAPKPackage returned error: %v", err)
	}

	for _, userID := range []string{"ztt", "alice"} {
		queue := bridge.pendingForUser(userID)
		if len(queue) != 1 {
			t.Fatalf("expected %s to have 1 pending group apk message, got %d", userID, len(queue))
		}
		if got, _ := queue[0].Meta["group_id"].(string); got != "g1" {
			t.Fatalf("expected group_id g1 for %s, got %q", userID, got)
		}
		if got, _ := queue[0].Meta["file_id"].(string); got != items[0].FileID {
			t.Fatalf("expected file_id %s for %s, got %s", items[0].FileID, userID, got)
		}
	}
	if got, _ := result["recipient_count"].(int); got != 2 {
		t.Fatalf("expected recipient_count 2, got %d", got)
	}
}

func TestPushStoredAPKPackageByFileNameRejectsAmbiguousMatch(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)

	for _, owner := range []string{"repo-a", "repo-b"} {
		if _, err := bridge.PushUploadedAPK(
			owner,
			"",
			"app-release.apk",
			bytes.NewReader([]byte(owner)),
		); err != nil {
			t.Fatalf("PushUploadedAPK seed for %s returned error: %v", owner, err)
		}
	}

	if _, err := bridge.pushStoredAPKPackage("alice", "", "", "app-release.apk", "", ""); err == nil {
		t.Fatalf("expected ambiguous file_name to fail")
	}
}
