package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"downloadticket"
	"obsstore"
)

type objectStorage interface {
	Enabled() bool
	PutObject(ctx context.Context, req obsstore.PutObjectRequest) error
}

type downloadTicketSigner interface {
	Enabled() bool
	Issue(input downloadticket.Input, ttl time.Duration) (string, *downloadticket.Claims, error)
}

func newObjectStorage(cfg *Config) objectStorage {
	if cfg == nil || !cfg.OBS.hasAnyValue() {
		return nil
	}
	store, err := obsstore.New(obsstore.Config{
		Endpoint:         cfg.OBS.Endpoint,
		Bucket:           cfg.OBS.Bucket,
		AccessKey:        cfg.OBS.AK,
		SecretKey:        cfg.OBS.SK,
		Region:           cfg.OBS.Region,
		KeyPrefix:        cfg.OBS.KeyPrefix,
		PathStyle:        cfg.OBS.PathStyle,
		DisableSSLVerify: cfg.OBS.DisableSSLVerify,
	})
	if err != nil {
		log.Printf("[Bridge] OBS disabled: %v", err)
		return nil
	}
	if !store.Enabled() {
		return nil
	}
	return store
}

func newDownloadTicketSigner(cfg *Config) downloadTicketSigner {
	if cfg == nil || strings.TrimSpace(cfg.DownloadTicketSecret) == "" {
		return nil
	}
	return downloadticket.NewSigner(cfg.DownloadTicketSecret)
}

func (b *Bridge) applyAttachmentStorage(
	owner string,
	attachment *AppAttachment,
	src io.Reader,
	size int64,
) {
	if attachment == nil {
		return
	}
	attachment.StorageProvider = "local"
	owner = strings.TrimSpace(owner)
	if owner == "" ||
		attachment.FileID == "" ||
		attachment.FileName == "" ||
		src == nil ||
		size < 0 ||
		b.obsStorage == nil ||
		!b.obsStorage.Enabled() {
		return
	}

	objectKey := buildAttachmentObjectKey(
		attachment.MessageType,
		owner,
		attachment.FileID,
		attachment.FileName,
		time.Now(),
	)
	log.Printf("[Bridge] upload attachment to OBS start file_id=%s key=%s owner=%s size=%d message_type=%s",
		attachment.FileID, objectKey, owner, size, strings.TrimSpace(attachment.MessageType))
	if err := b.obsStorage.PutObject(context.Background(), obsstore.PutObjectRequest{
		Key:         objectKey,
		Body:        src,
		Size:        size,
		ContentType: attachment.MIMEType,
		Metadata: map[string]string{
			"file_id":      attachment.FileID,
			"owner":        owner,
			"message_type": strings.TrimSpace(attachment.MessageType),
			"file_name":    attachment.FileName,
		},
	}); err != nil {
		log.Printf("[Bridge] upload attachment to OBS failed file_id=%s key=%s err=%v", attachment.FileID, objectKey, err)
		return
	}

	attachment.StorageProvider = "obs"
	attachment.ObjectKey = objectKey
	log.Printf("[Bridge] upload attachment to OBS success file_id=%s key=%s owner=%s size=%d storage_provider=%s",
		attachment.FileID, attachment.ObjectKey, owner, size, attachment.StorageProvider)
}

func (b *Bridge) applyAttachmentStorageFromBytes(owner string, attachment *AppAttachment, data []byte) {
	if len(data) == 0 {
		if attachment != nil && attachment.StorageProvider == "" {
			attachment.StorageProvider = "local"
		}
		return
	}
	b.applyAttachmentStorage(owner, attachment, bytes.NewReader(data), int64(len(data)))
}

func (b *Bridge) applyAttachmentStorageFromFile(owner string, attachment *AppAttachment) {
	if attachment == nil || attachment.FilePath == "" {
		if attachment != nil && attachment.StorageProvider == "" {
			attachment.StorageProvider = "local"
		}
		return
	}
	file, err := os.Open(filepath.Clean(attachment.FilePath))
	if err != nil {
		log.Printf("[Bridge] open attachment for OBS upload failed file=%s err=%v", attachment.FilePath, err)
		attachment.StorageProvider = "local"
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		log.Printf("[Bridge] stat attachment for OBS upload failed file=%s err=%v", attachment.FilePath, err)
		attachment.StorageProvider = "local"
		return
	}
	b.applyAttachmentStorage(owner, attachment, file, info.Size())
}

func (b *Bridge) buildPushMetaForUser(baseMeta map[string]any, attachment *AppAttachment, userID string) map[string]any {
	out := cloneMeta(baseMeta)
	if attachment == nil {
		return out
	}
	if out == nil {
		out = make(map[string]any)
	}
	if attachment.FileID != "" {
		out["file_id"] = attachment.FileID
	}
	if attachment.FileName != "" {
		out["file_name"] = attachment.FileName
	}
	if attachment.FileSize > 0 {
		out["file_size"] = attachment.FileSize
	}
	if attachment.Format != "" {
		switch attachment.MessageType {
		case "audio":
			out["audio_format"] = attachment.Format
		case "image":
			out["image_format"] = attachment.Format
		default:
			out["file_format"] = attachment.Format
		}
	}
	if attachment.MIMEType != "" {
		out["mime_type"] = attachment.MIMEType
	}
	if attachment.DurationMS > 0 {
		out["duration_ms"] = attachment.DurationMS
	}
	if attachment.SpeechText != "" {
		out["speech_text"] = attachment.SpeechText
	}
	if attachment.InputMode != "" {
		out["input_mode"] = attachment.InputMode
	}

	storageProvider := strings.TrimSpace(attachment.StorageProvider)
	if storageProvider == "" {
		if attachment.ObjectKey != "" {
			storageProvider = "obs"
		} else {
			storageProvider = "local"
		}
	}
	if storageProvider != "" {
		out["storage_provider"] = storageProvider
	}
	if attachment.ObjectKey != "" {
		out["object_key"] = attachment.ObjectKey
		out["download_via"] = "obs-agent"
	}
	if strings.TrimSpace(userID) != "" {
		ticket, claims, err := b.issueDownloadTicket(userID, attachment)
		if err != nil {
			log.Printf("[Bridge] issue download ticket failed user=%s file_id=%s err=%v", userID, attachment.FileID, err)
		} else if ticket != "" && claims != nil {
			out["download_ticket"] = ticket
			out["download_ticket_expire_at"] = claims.ExpiresAt
		}
	}
	return out
}

func (b *Bridge) issueDownloadTicket(userID string, attachment *AppAttachment) (string, *downloadticket.Claims, error) {
	if attachment == nil ||
		attachment.FileID == "" ||
		attachment.ObjectKey == "" ||
		b.downloadTickets == nil ||
		!b.downloadTickets.Enabled() {
		return "", nil, nil
	}
	ttl := b.downloadTicketTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return b.downloadTickets.Issue(downloadticket.Input{
		FileID:          attachment.FileID,
		UserID:          strings.TrimSpace(userID),
		ObjectKey:       attachment.ObjectKey,
		StorageProvider: firstNonEmpty(attachment.StorageProvider, "obs"),
	}, ttl)
}

func buildAttachmentObjectKey(messageType, owner, fileID, fileName string, now time.Time) string {
	safeType := sanitizeFileName(firstNonEmpty(strings.ToLower(strings.TrimSpace(messageType)), "file"))
	safeOwner := sanitizeFileName(firstNonEmpty(strings.TrimSpace(owner), "anonymous"))
	safeName := sanitizeFileName(firstNonEmpty(strings.TrimSpace(fileName), "attachment.bin"))
	return fmt.Sprintf(
		"app/%s/%s/%04d/%02d/%02d/%s/%s",
		safeType,
		safeOwner,
		now.Year(),
		now.Month(),
		now.Day(),
		sanitizeFileName(strings.TrimSpace(fileID)),
		safeName,
	)
}

func (c OBSStorageConfig) hasAnyValue() bool {
	return strings.TrimSpace(c.Endpoint) != "" ||
		strings.TrimSpace(c.Bucket) != "" ||
		strings.TrimSpace(c.AK) != "" ||
		strings.TrimSpace(c.SK) != "" ||
		strings.TrimSpace(c.Region) != "" ||
		strings.TrimSpace(c.KeyPrefix) != ""
}
