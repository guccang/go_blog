package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"downloadticket"
	"obsstore"
)

type fakeOBSStorage struct {
	putCount    int
	headCount   int
	existing    map[string]bool
	lastHeadKey string
	lastReq     obsstore.PutObjectRequest
}

func (f *fakeOBSStorage) Enabled() bool {
	return true
}

func (f *fakeOBSStorage) HeadObject(_ context.Context, key string) (bool, error) {
	f.headCount++
	f.lastHeadKey = key
	return f.existing != nil && f.existing[key], nil
}

func (f *fakeOBSStorage) PutObject(_ context.Context, req obsstore.PutObjectRequest) error {
	f.putCount++
	f.lastReq = req
	if f.existing == nil {
		f.existing = make(map[string]bool)
	}
	f.existing[req.Key] = true
	return nil
}

type fakeDownloadTicketSigner struct {
	token     string
	expiresAt int64
	lastInput downloadticket.Input
	lastTTL   time.Duration
}

func (f *fakeDownloadTicketSigner) Enabled() bool {
	return true
}

func (f *fakeDownloadTicketSigner) Issue(input downloadticket.Input, ttl time.Duration) (string, *downloadticket.Claims, error) {
	f.lastInput = input
	f.lastTTL = ttl
	return f.token, &downloadticket.Claims{
		FileID:          input.FileID,
		UserID:          input.UserID,
		ObjectKey:       input.ObjectKey,
		StorageProvider: input.StorageProvider,
		ExpiresAt:       f.expiresAt,
		Nonce:           "nonce",
	}, nil
}

func newTestBridgeWithAttachmentDir(t *testing.T) *Bridge {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AttachmentStoreDir = filepath.Join(t.TempDir(), "app-attachments")
	cfg.GroupStoreFile = filepath.Join(t.TempDir(), "groups.json")
	return NewBridge(cfg)
}

func TestRegisterClientAllowsMultipleConnectionsPerUser(t *testing.T) {
	bridge := NewBridge(DefaultConfig())

	client1 := &appClientConn{userID: "demo-user"}
	client2 := &appClientConn{userID: "demo-user"}

	bridge.registerClient(client1)
	bridge.registerClient(client2)

	if got := bridge.OnlineClientCount(); got != 2 {
		t.Fatalf("expected 2 online clients, got %d", got)
	}

	userClients := bridge.clients["demo-user"]
	if len(userClients) != 2 {
		t.Fatalf("expected 2 stored connections for demo-user, got %d", len(userClients))
	}

	bridge.unregisterClient(client1)
	if got := bridge.OnlineClientCount(); got != 1 {
		t.Fatalf("expected 1 online client after removing one connection, got %d", got)
	}

	bridge.unregisterClient(client2)
	if got := bridge.OnlineClientCount(); got != 0 {
		t.Fatalf("expected 0 online clients after removing all connections, got %d", got)
	}
}

func TestPushUploadedAPKQueuesForOfflineUser(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)

	attachment, err := bridge.PushUploadedAPK(
		"ztt",
		"新的安装包已下发，点击安装",
		"app-release.apk",
		bytes.NewReader([]byte("apk-binary")),
	)
	if err != nil {
		t.Fatalf("PushUploadedAPK returned error: %v", err)
	}
	if attachment == nil {
		t.Fatalf("expected attachment to be returned")
	}
	if got := len(bridge.pendingByUser["ztt"]); got != 1 {
		t.Fatalf("expected ztt to have 1 pending apk message, got %d", got)
	}
	queue := bridge.pendingForUser("ztt")
	if len(queue) != 1 {
		t.Fatalf("expected 1 pending payload for ztt, got %d", len(queue))
	}
	if queue[0].MessageType != "file" {
		t.Fatalf("expected queued message type file, got %s", queue[0].MessageType)
	}
	if got := queue[0].Meta["file_format"]; got != "apk" {
		t.Fatalf("expected file_format=apk, got %#v", got)
	}
	if got := queue[0].Meta["file_id"]; got == "" || got == nil {
		t.Fatalf("expected file_id in queued apk meta, got %#v", got)
	}
}

func TestPushUploadedAPKQueuesOBSDownloadMetaWhenConfigured(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)
	bridge.obsStorage = &fakeOBSStorage{}
	bridge.downloadTickets = &fakeDownloadTicketSigner{
		token:     "ticket-123",
		expiresAt: 1234567890,
	}
	bridge.downloadTicketTTL = 5 * time.Minute

	attachment, err := bridge.PushUploadedAPK(
		"ztt",
		"新的安装包已下发，点击安装",
		"app-release.apk",
		bytes.NewReader([]byte("apk-binary")),
	)
	if err != nil {
		t.Fatalf("PushUploadedAPK returned error: %v", err)
	}
	if attachment == nil {
		t.Fatalf("expected attachment to be returned")
	}
	if attachment.StorageProvider != "obs" {
		t.Fatalf("expected attachment storage_provider=obs, got %q", attachment.StorageProvider)
	}
	if attachment.ObjectKey == "" {
		t.Fatalf("expected object key to be assigned")
	}

	queue := bridge.pendingForUser("ztt")
	if len(queue) != 1 {
		t.Fatalf("expected 1 pending payload for ztt, got %d", len(queue))
	}
	meta := queue[0].Meta
	if got := meta["storage_provider"]; got != "obs" {
		t.Fatalf("expected storage_provider=obs, got %#v", got)
	}
	if got := meta["object_key"]; got == "" || got == nil {
		t.Fatalf("expected object_key in meta, got %#v", got)
	}
	if got := meta["download_ticket"]; got != "ticket-123" {
		t.Fatalf("expected download_ticket=ticket-123, got %#v", got)
	}
	if got := meta["download_ticket_expire_at"]; got != int64(1234567890) {
		t.Fatalf("expected download_ticket_expire_at=1234567890, got %#v", got)
	}
}

func TestPushStoredAPKPackageReusesExistingOBSObject(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)
	fakeStore := &fakeOBSStorage{}
	bridge.obsStorage = fakeStore
	bridge.downloadTickets = &fakeDownloadTicketSigner{
		token:     "ticket-123",
		expiresAt: 1234567890,
	}
	bridge.downloadTicketTTL = 5 * time.Minute

	attachment, err := bridge.PushUploadedAPK(
		"ztt",
		"首次上传",
		"app-release.apk",
		bytes.NewReader([]byte("apk-binary")),
	)
	if err != nil {
		t.Fatalf("PushUploadedAPK returned error: %v", err)
	}
	if attachment == nil {
		t.Fatalf("expected attachment to be returned")
	}
	if fakeStore.putCount != 1 {
		t.Fatalf("expected first upload to hit OBS once, got %d", fakeStore.putCount)
	}

	beforeHeadCount := fakeStore.headCount
	beforePutCount := fakeStore.putCount
	result, err := bridge.pushStoredAPKPackage("alice", "", attachment.FileID, "", "", "复用安装包")
	if err != nil {
		t.Fatalf("pushStoredAPKPackage returned error: %v", err)
	}
	if got, _ := result["storage_provider"].(string); got != "obs" {
		t.Fatalf("expected storage_provider=obs, got %q", got)
	}
	if fakeStore.headCount <= beforeHeadCount {
		t.Fatalf("expected stored APK reuse to check OBS existence")
	}
	if fakeStore.putCount != beforePutCount {
		t.Fatalf("expected stored APK reuse to skip OBS re-upload, putCount=%d before=%d", fakeStore.putCount, beforePutCount)
	}

	queue := bridge.pendingForUser("alice")
	if len(queue) != 1 {
		t.Fatalf("expected 1 pending payload for alice, got %d", len(queue))
	}
	meta := queue[0].Meta
	if got := meta["object_key"]; got != attachment.ObjectKey {
		t.Fatalf("expected object_key %q, got %#v", attachment.ObjectKey, got)
	}
	if got := meta["download_ticket"]; got != "ticket-123" {
		t.Fatalf("expected download_ticket=ticket-123, got %#v", got)
	}
}

func TestPushUploadedAPKSameFileNameKeepsOnlyLatestPendingMessage(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)

	if _, err := bridge.PushUploadedAPK(
		"ztt",
		"旧安装包",
		"app-release.apk",
		bytes.NewReader([]byte("apk-v1")),
	); err != nil {
		t.Fatalf("first PushUploadedAPK returned error: %v", err)
	}
	attachment, err := bridge.PushUploadedAPK(
		"ztt",
		"新安装包",
		"app-release.apk",
		bytes.NewReader([]byte("apk-v2")),
	)
	if err != nil {
		t.Fatalf("second PushUploadedAPK returned error: %v", err)
	}
	queue := bridge.pendingForUser("ztt")
	if len(queue) != 1 {
		t.Fatalf("expected only 1 pending apk for ztt, got %d", len(queue))
	}
	if queue[0].Content != "新安装包" {
		t.Fatalf("expected latest apk content to remain, got %q", queue[0].Content)
	}
	fileID, _ := queue[0].Meta["file_id"].(string)
	if fileID != attachment.FileID {
		t.Fatalf("expected latest file_id %s, got %s", attachment.FileID, fileID)
	}
	path, err := resolveAttachmentPath(bridge.cfg.AttachmentStoreDir, attachment.FileID)
	if err != nil {
		t.Fatalf("resolveAttachmentPath returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "apk-v2" {
		t.Fatalf("expected latest apk file data to be apk-v2, got %q", string(data))
	}
}

func TestBroadcastGroupMessageExcludesHumanSender(t *testing.T) {
	bridge := NewBridge(DefaultConfig())
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true},
		RobotAccount: "robot-g1",
	}

	err := bridge.broadcastGroupMessage("g1", "ztt", "hello", "text", map[string]any{})
	if err != nil {
		t.Fatalf("broadcastGroupMessage returned error: %v", err)
	}

	if got := len(bridge.pendingByUser["ztt"]); got != 0 {
		t.Fatalf("expected sender ztt to receive no queued messages, got %d", got)
	}
	if got := len(bridge.pendingByUser["alice"]); got != 1 {
		t.Fatalf("expected alice to receive 1 queued message, got %d", got)
	}
}

func TestBroadcastGroupMessageFromRobotStillReachesAllHumans(t *testing.T) {
	bridge := NewBridge(DefaultConfig())
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true},
		RobotAccount: "robot-g1",
	}

	err := bridge.broadcastGroupMessage("g1", "robot-g1", "robot reply", "text", map[string]any{})
	if err != nil {
		t.Fatalf("broadcastGroupMessage returned error: %v", err)
	}

	if got := len(bridge.pendingByUser["ztt"]); got != 1 {
		t.Fatalf("expected ztt to receive 1 robot message, got %d", got)
	}
	if got := len(bridge.pendingByUser["alice"]); got != 1 {
		t.Fatalf("expected alice to receive 1 robot message, got %d", got)
	}
}

func TestPushUploadedAPKToGroupQueuesForAllHumansExceptRobot(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true, "bob": true},
		RobotAccount: "robot-g1",
	}

	attachment, recipients, err := bridge.PushUploadedAPKToGroup(
		"g1",
		"群安装包已下发，点击安装",
		"app-release.apk",
		bytes.NewReader([]byte("apk-binary")),
	)
	if err != nil {
		t.Fatalf("PushUploadedAPKToGroup returned error: %v", err)
	}
	if attachment == nil {
		t.Fatalf("expected attachment to be returned")
	}
	if len(recipients) != 3 {
		t.Fatalf("expected 3 human recipients, got %d", len(recipients))
	}
	if got := len(bridge.pendingByUser["robot-g1"]); got != 0 {
		t.Fatalf("expected robot to receive no queued messages, got %d", got)
	}
	for _, userID := range []string{"ztt", "alice", "bob"} {
		if got := len(bridge.pendingByUser[userID]); got != 1 {
			t.Fatalf("expected %s to receive 1 queued apk message, got %d", userID, got)
		}
		queue := bridge.pendingForUser(userID)
		if len(queue) != 1 {
			t.Fatalf("expected 1 pending payload for %s, got %d", userID, len(queue))
		}
		if got := queue[0].Meta["group_id"]; got != "g1" {
			t.Fatalf("expected group_id=g1 for %s, got %#v", userID, got)
		}
		if got := queue[0].Meta["file_format"]; got != "apk" {
			t.Fatalf("expected file_format=apk for %s, got %#v", userID, got)
		}
	}
}

func TestPushUploadedAPKToGroupSameFileNameKeepsOnlyLatestPendingMessage(t *testing.T) {
	bridge := newTestBridgeWithAttachmentDir(t)
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true},
		RobotAccount: "robot-g1",
	}

	if _, _, err := bridge.PushUploadedAPKToGroup(
		"g1",
		"旧群安装包",
		"app-release.apk",
		bytes.NewReader([]byte("group-v1")),
	); err != nil {
		t.Fatalf("first PushUploadedAPKToGroup returned error: %v", err)
	}
	attachment, recipients, err := bridge.PushUploadedAPKToGroup(
		"g1",
		"新群安装包",
		"app-release.apk",
		bytes.NewReader([]byte("group-v2")),
	)
	if err != nil {
		t.Fatalf("second PushUploadedAPKToGroup returned error: %v", err)
	}
	if len(recipients) != 2 {
		t.Fatalf("expected 2 human recipients, got %d", len(recipients))
	}
	for _, userID := range []string{"ztt", "alice"} {
		queue := bridge.pendingForUser(userID)
		if len(queue) != 1 {
			t.Fatalf("expected only 1 pending apk for %s, got %d", userID, len(queue))
		}
		if queue[0].Content != "新群安装包" {
			t.Fatalf("expected latest group apk content for %s, got %q", userID, queue[0].Content)
		}
		fileID, _ := queue[0].Meta["file_id"].(string)
		if fileID != attachment.FileID {
			t.Fatalf("expected latest group file_id %s for %s, got %s", attachment.FileID, userID, fileID)
		}
	}
	path, err := resolveAttachmentPath(bridge.cfg.AttachmentStoreDir, attachment.FileID)
	if err != nil {
		t.Fatalf("resolveAttachmentPath returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "group-v2" {
		t.Fatalf("expected latest group apk file data to be group-v2, got %q", string(data))
	}
}
