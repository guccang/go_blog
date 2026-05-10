package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"downloadticket"
	"obsstore"
)

type objectStorage interface {
	Enabled() bool
	HeadObject(ctx context.Context, key string) (bool, error)
	PutObject(ctx context.Context, req obsstore.PutObjectRequest) error
	ListObjects(ctx context.Context, prefix string, marker string, maxKeys int) (*obsstore.ListObjectsResult, error)
	CreateSignedGetURL(ctx context.Context, key string, ttl time.Duration) (*obsstore.SignedURL, error)
	DeleteObject(ctx context.Context, key string) error
}

type downloadTicketSigner interface {
	Enabled() bool
	Issue(input downloadticket.Input, ttl time.Duration) (string, *downloadticket.Claims, error)
}

func newObjectStorage(cfg *Config) objectStorage {
	if remote := newRemoteObjectStorage(cfg); remote != nil {
		return remote
	}
	if cfg == nil || !cfg.OBS.hasAnyValue() {
		return nil
	}
	store, err := obsstore.New(obsstore.Config{
		Endpoint:         cfg.OBS.Endpoint,
		PublicEndpoint:   cfg.OBS.PublicEndpoint,
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

type remoteObjectStorage struct {
	baseURL string
	token   string
	client  *http.Client
	signer  downloadTicketSigner
}

func newRemoteObjectStorage(cfg *Config) objectStorage {
	if cfg == nil || strings.TrimSpace(cfg.ObsAgentBaseURL) == "" {
		return nil
	}
	return &remoteObjectStorage{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.ObsAgentBaseURL), "/"),
		token:   strings.TrimSpace(firstNonEmpty(cfg.ObsAgentToken, cfg.ReceiveToken)),
		client:  &http.Client{Timeout: 30 * time.Second},
		signer:  newDownloadTicketSigner(cfg),
	}
}

func (s *remoteObjectStorage) Enabled() bool {
	return s != nil && strings.TrimSpace(s.baseURL) != ""
}

func (s *remoteObjectStorage) HeadObject(ctx context.Context, key string) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false, fmt.Errorf("object key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/api/obs/info?object_key="+url.QueryEscape(key), nil)
	if err != nil {
		return false, fmt.Errorf("build obs-agent head request: %w", err)
	}
	s.applyAuth(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request obs-agent object info: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return false, fmt.Errorf("obs-agent object info unauthorized: %s", resp.Status)
	default:
		// obs-agent /info currently does not distinguish not-found from lookup failure.
		// Treat non-success as "not confirmed existing" so uploads can continue.
		return false, nil
	}
}

func (s *remoteObjectStorage) PutObject(ctx context.Context, req obsstore.PutObjectRequest) error {
	if !s.Enabled() {
		return fmt.Errorf("obs-agent upload is disabled")
	}
	if strings.TrimSpace(req.Key) == "" {
		return fmt.Errorf("object key is required")
	}
	if req.Body == nil {
		return fmt.Errorf("object body is required")
	}

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	copyDone := make(chan error, 1)
	go func() {
		var writeErr error
		defer func() {
			if writeErr != nil {
				_ = pipeWriter.CloseWithError(writeErr)
			} else {
				_ = pipeWriter.Close()
			}
			copyDone <- writeErr
		}()

		if writeErr = writer.WriteField("object_key", strings.TrimSpace(req.Key)); writeErr != nil {
			writeErr = fmt.Errorf("write object_key field: %w", writeErr)
			return
		}
		if strings.TrimSpace(req.ContentType) != "" {
			if writeErr = writer.WriteField("content_type", strings.TrimSpace(req.ContentType)); writeErr != nil {
				writeErr = fmt.Errorf("write content_type field: %w", writeErr)
				return
			}
		}
		part, err := writer.CreateFormFile("file", fileNameFromObjectKey(req.Key))
		if err != nil {
			writeErr = fmt.Errorf("create multipart file field: %w", err)
			return
		}
		if _, err := io.Copy(part, req.Body); err != nil {
			writeErr = fmt.Errorf("copy multipart body: %w", err)
			return
		}
		if err := writer.Close(); err != nil {
			writeErr = fmt.Errorf("close multipart body: %w", err)
			return
		}
	}()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/obs/proxy-upload", pipeReader)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		return fmt.Errorf("build obs-agent upload request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	s.applyAuth(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		return fmt.Errorf("request obs-agent proxy upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = pipeReader.CloseWithError(fmt.Errorf("obs-agent proxy upload failed: %s", resp.Status))
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("obs-agent proxy upload failed: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	if err := <-copyDone; err != nil {
		return fmt.Errorf("stream obs-agent proxy upload body: %w", err)
	}
	return nil
}

func (s *remoteObjectStorage) ListObjects(ctx context.Context, prefix string, marker string, maxKeys int) (*obsstore.ListObjectsResult, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("obs-agent list is disabled")
	}
	u := s.baseURL + "/api/obs/list?prefix=" + url.QueryEscape(strings.TrimSpace(prefix))
	if strings.TrimSpace(marker) != "" {
		u += "&marker=" + url.QueryEscape(strings.TrimSpace(marker))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build obs-agent list request: %w", err)
	}
	s.applyAuth(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request obs-agent list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("obs-agent list failed: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}

	var data struct {
		Objects []struct {
			Key          string `json:"key"`
			Size         int64  `json:"size"`
			LastModified int64  `json:"last_modified"`
			ETag         string `json:"etag"`
		} `json:"objects"`
		IsTruncated bool   `json:"is_truncated"`
		NextMarker  string `json:"next_marker"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode obs-agent list response: %w", err)
	}
	items := make([]obsstore.ObjectListItem, 0, len(data.Objects))
	for _, obj := range data.Objects {
		items = append(items, obsstore.ObjectListItem{
			Key:          strings.TrimSpace(obj.Key),
			Size:         obj.Size,
			LastModified: time.UnixMilli(obj.LastModified),
			ETag:         obj.ETag,
		})
	}
	return &obsstore.ListObjectsResult{
		Objects:     items,
		IsTruncated: data.IsTruncated,
		NextMarker:  data.NextMarker,
	}, nil
}

func (s *remoteObjectStorage) CreateSignedGetURL(ctx context.Context, key string, ttl time.Duration) (*obsstore.SignedURL, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("obs-agent download is disabled")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}
	if s.signer == nil || !s.signer.Enabled() {
		return nil, fmt.Errorf("download ticket signer is not configured")
	}
	fileID := base64.RawURLEncoding.EncodeToString([]byte(key))
	ticket, _, err := s.signer.Issue(downloadticket.Input{
		FileID:          fileID,
		ObjectKey:       key,
		StorageProvider: "obs",
	}, ttl)
	if err != nil {
		return nil, fmt.Errorf("issue obs-agent download ticket: %w", err)
	}
	reqURL := s.baseURL + "/api/obs/download/" + url.PathEscape(fileID) + "?ticket=" + url.QueryEscape(ticket)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build obs-agent download request: %w", err)
	}
	s.applyAuth(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request obs-agent download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("obs-agent download failed: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var data struct {
		URL       string            `json:"url"`
		Method    string            `json:"method"`
		ExpiresAt int64             `json:"expires_at"`
		Headers   map[string]string `json:"headers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode obs-agent download response: %w", err)
	}
	if strings.TrimSpace(data.URL) == "" {
		return nil, fmt.Errorf("obs-agent download response missing url")
	}
	return &obsstore.SignedURL{
		URL:       data.URL,
		Method:    firstNonEmpty(data.Method, "GET"),
		ExpiresAt: data.ExpiresAt,
		Headers:   data.Headers,
	}, nil
}

func (s *remoteObjectStorage) DeleteObject(ctx context.Context, key string) error {
	if !s.Enabled() {
		return fmt.Errorf("obs-agent delete is disabled")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("object key is required")
	}
	body, err := json.Marshal(map[string]string{"object_key": key})
	if err != nil {
		return fmt.Errorf("encode obs-agent delete request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/obs/delete", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build obs-agent delete request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	s.applyAuth(httpReq)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request obs-agent delete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("obs-agent delete failed: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (s *remoteObjectStorage) applyAuth(req *http.Request) {
	if s == nil || req == nil || strings.TrimSpace(s.token) == "" {
		return
	}
	req.Header.Set("X-App-Agent-Token", strings.TrimSpace(s.token))
}

func fileNameFromObjectKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "attachment.bin"
	}
	if idx := strings.LastIndex(key, "/"); idx >= 0 && idx+1 < len(key) {
		key = key[idx+1:]
	}
	if key == "" {
		return "attachment.bin"
	}
	return key
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

	objectKey := strings.TrimSpace(attachment.ObjectKey)
	if objectKey == "" {
		objectKey = buildAttachmentObjectKey(
			attachment.MessageType,
			owner,
			attachment.FileID,
			attachment.FileName,
		)
	}
	if exists, err := b.obsStorage.HeadObject(context.Background(), objectKey); err != nil {
		log.Printf("[Bridge] head attachment object failed file_id=%s key=%s err=%v", attachment.FileID, objectKey, err)
	} else if exists {
		attachment.StorageProvider = "obs"
		attachment.ObjectKey = objectKey
		b.uploadAttachmentMetaSidecar(owner, attachment)
		log.Printf("[Bridge] reuse attachment in OBS file_id=%s key=%s owner=%s size=%d",
			attachment.FileID, attachment.ObjectKey, owner, size)
		return
	}
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
	b.uploadAttachmentMetaSidecar(owner, attachment)
	log.Printf("[Bridge] upload attachment to OBS success file_id=%s key=%s owner=%s size=%d storage_provider=%s",
		attachment.FileID, attachment.ObjectKey, owner, size, attachment.StorageProvider)
}

type attachmentMetaSidecar struct {
	Kind            string         `json:"kind"`
	SchemaVersion   int            `json:"schema_version"`
	ObjectKey       string         `json:"object_key"`
	MetaObjectKey   string         `json:"meta_object_key"`
	StorageProvider string         `json:"storage_provider,omitempty"`
	Owner           string         `json:"owner,omitempty"`
	MessageType     string         `json:"message_type,omitempty"`
	FileID          string         `json:"file_id,omitempty"`
	FileName        string         `json:"file_name,omitempty"`
	FileSize        int            `json:"file_size,omitempty"`
	Format          string         `json:"format,omitempty"`
	MIMEType        string         `json:"mime_type,omitempty"`
	DurationMS      int            `json:"duration_ms,omitempty"`
	Description     string         `json:"description,omitempty"`
	SpeechText      string         `json:"speech_text,omitempty"`
	InputMode       string         `json:"input_mode,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	UpdatedAt       int64          `json:"updated_at"`
}

func (b *Bridge) uploadAttachmentMetaSidecar(owner string, attachment *AppAttachment) {
	if b == nil ||
		b.obsStorage == nil ||
		!b.obsStorage.Enabled() ||
		attachment == nil ||
		strings.TrimSpace(attachment.ObjectKey) == "" {
		return
	}
	metaKey := attachmentMetaObjectKey(attachment.ObjectKey)
	sidecar := attachmentMetaSidecar{
		Kind:            "app_attachment",
		SchemaVersion:   1,
		ObjectKey:       attachment.ObjectKey,
		MetaObjectKey:   metaKey,
		StorageProvider: firstNonEmpty(attachment.StorageProvider, "obs"),
		Owner:           strings.TrimSpace(owner),
		MessageType:     strings.TrimSpace(attachment.MessageType),
		FileID:          strings.TrimSpace(attachment.FileID),
		FileName:        strings.TrimSpace(attachment.FileName),
		FileSize:        attachment.FileSize,
		Format:          strings.TrimSpace(attachment.Format),
		MIMEType:        strings.TrimSpace(attachment.MIMEType),
		DurationMS:      attachment.DurationMS,
		Description:     strings.TrimSpace(attachment.Description),
		SpeechText:      strings.TrimSpace(attachment.SpeechText),
		InputMode:       strings.TrimSpace(attachment.InputMode),
		Metadata:        sanitizeAttachmentSidecarMeta(attachment.Meta),
		UpdatedAt:       time.Now().UnixMilli(),
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		log.Printf("[Bridge] marshal attachment meta sidecar failed file_id=%s key=%s err=%v", attachment.FileID, metaKey, err)
		return
	}
	data = append(data, '\n')
	if err := b.obsStorage.PutObject(context.Background(), obsstore.PutObjectRequest{
		Key:         metaKey,
		Body:        bytes.NewReader(data),
		Size:        int64(len(data)),
		ContentType: "application/json; charset=utf-8",
		Metadata: map[string]string{
			"file_id":      attachment.FileID,
			"owner":        strings.TrimSpace(owner),
			"message_type": strings.TrimSpace(attachment.MessageType),
			"sidecar_for":  attachment.ObjectKey,
		},
	}); err != nil {
		log.Printf("[Bridge] upload attachment meta sidecar failed file_id=%s key=%s err=%v", attachment.FileID, metaKey, err)
		return
	}
	log.Printf("[Bridge] upload attachment meta sidecar success file_id=%s key=%s", attachment.FileID, metaKey)
}

func attachmentMetaObjectKey(objectKey string) string {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return ""
	}
	return objectKey + ".meta.json"
}

func sanitizeAttachmentSidecarMeta(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	out := make(map[string]any, len(meta))
	for key, value := range meta {
		switch key {
		case "audio_base64", "image_base64", "video_base64", "file_base64", "zip_base64", "inline_base64":
			text, _ := value.(string)
			out[key+"_present"] = strings.TrimSpace(text) != ""
		default:
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
		case "video":
			out["video_format"] = attachment.Format
			out["file_format"] = attachment.Format
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

func buildAttachmentObjectKey(messageType, owner, fileID, fileName string) string {
	safeType := sanitizeFileName(firstNonEmpty(strings.ToLower(strings.TrimSpace(messageType)), "file"))
	safeOwner := sanitizeFileName(firstNonEmpty(strings.TrimSpace(owner), "anonymous"))
	safeFileID := sanitizeFileName(canonicalAttachmentFileID(fileID))
	if safeFileID == "" {
		safeFileID = "attachment"
	}
	safeName := sanitizeFileName(firstNonEmpty(strings.TrimSpace(fileName), "attachment.bin"))
	return fmt.Sprintf(
		"app/%s/%s/%s/%s",
		safeType,
		safeOwner,
		safeFileID,
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
