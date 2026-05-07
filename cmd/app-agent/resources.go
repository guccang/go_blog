package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxResourceUploadBytes int64 = 128 << 20
const bytesPerMiB int64 = 1 << 20

type appResourceItem struct {
	Category        string         `json:"category"`
	MessageType     string         `json:"message_type"`
	FileID          string         `json:"file_id"`
	FileName        string         `json:"file_name"`
	FileSize        int64          `json:"file_size"`
	FileFormat      string         `json:"file_format,omitempty"`
	MIMEType        string         `json:"mime_type,omitempty"`
	StorageProvider string         `json:"storage_provider,omitempty"`
	ObjectKey       string         `json:"object_key,omitempty"`
	DownloadURL     string         `json:"download_url,omitempty"`
	Description     string         `json:"description,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	UpdatedAt       int64          `json:"updated_at"`
}

type appResourceUsage struct {
	TotalSize     int64 `json:"total_size"`
	TotalCount    int   `json:"total_count"`
	CategorySize  int64 `json:"category_size"`
	CategoryCount int   `json:"category_count"`
}

type appResourceResponse struct {
	Success bool              `json:"success"`
	Item    *appResourceItem  `json:"item,omitempty"`
	Items   []appResourceItem `json:"items,omitempty"`
	Usage   *appResourceUsage `json:"usage,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func (h *Handler) HandleResources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListResources(w, r)
	case http.MethodPost:
		h.handleUploadResource(w, r)
	case http.MethodDelete:
		h.handleDeleteResource(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleListResources(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, userID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	category := normalizeResourceCategory(r.URL.Query().Get("category"))
	items, err := h.bridge.ListResources(userID, category)
	if err != nil {
		logHandlerError(w, "list resources failed", err)
		return
	}
	usage, err := h.bridge.ResourceUsage(userID, category)
	if err != nil {
		logHandlerError(w, "resource usage failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(appResourceResponse{
		Success: true,
		Items:   items,
		Usage:   usage,
	})
}

func (h *Handler) handleUploadResource(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Invalid multipart form", http.StatusBadRequest)
			return
		}
		if part.FileName() == "" {
			value, err := readResourceUploadField(part)
			_ = part.Close()
			if err != nil {
				http.Error(w, "Invalid multipart form", http.StatusBadRequest)
				return
			}
			fields[part.FormName()] = value
			continue
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		item, err := h.storeResourceUploadPart(r, fields, part)
		if err != nil {
			if err == errResourceLoginRequired {
				http.Error(w, "Login required", http.StatusUnauthorized)
				return
			}
			if err == errResourceUploadBadRequest {
				http.Error(w, "Invalid multipart form", http.StatusBadRequest)
				return
			}
			if errors.Is(err, errResourceExceedsMaxSize) {
				http.Error(w, "resource exceeds max size", http.StatusRequestEntityTooLarge)
				return
			}
			logHandlerError(w, "upload resource failed", err)
			return
		}
		_ = part.Close()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(appResourceResponse{
			Success: true,
			Item:    item,
		})
		return
	}
	http.Error(w, "file is required", http.StatusBadRequest)
}

var (
	errResourceLoginRequired    = fmt.Errorf("resource login required")
	errResourceUploadBadRequest = fmt.Errorf("resource upload bad request")
	errResourceExceedsMaxSize   = fmt.Errorf("resource exceeds max size")
)

func (h *Handler) storeResourceUploadPart(r *http.Request, fields map[string]string, part *multipart.Part) (*appResourceItem, error) {
	userID := strings.TrimSpace(fields["user_id"])
	if userID == "" {
		return nil, errResourceUploadBadRequest
	}
	if !h.validateAppSession(r, userID) {
		return nil, errResourceLoginRequired
	}
	fileName := strings.TrimSpace(part.FileName())
	if fileName == "" {
		return nil, errResourceUploadBadRequest
	}
	category := normalizeResourceCategory(fields["category"])
	if category == "" {
		category = inferResourceCategory(fileName, part.Header.Get("Content-Type"))
	}
	contentType := strings.TrimSpace(firstNonEmpty(
		fields["content_type"],
		part.Header.Get("Content-Type"),
		mime.TypeByExtension(filepath.Ext(fileName)),
	))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return h.bridge.StoreResource(userID, category, strings.TrimSpace(fields["description"]), fileName, contentType, part)
}

func readResourceUploadField(r io.Reader) (string, error) {
	const maxResourceUploadFieldBytes int64 = 64 << 10
	data, err := io.ReadAll(io.LimitReader(r, maxResourceUploadFieldBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxResourceUploadFieldBytes {
		return "", fmt.Errorf("resource upload field exceeds max size")
	}
	return string(data), nil
}

func (h *Handler) handleDeleteResource(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, userID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	fileID := strings.TrimSpace(r.URL.Query().Get("file_id"))
	if fileID == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}
	if err := h.bridge.DeleteResource(userID, fileID); err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(err.Error(), "not an app resource") || strings.Contains(err.Error(), "invalid") {
			http.Error(w, "Invalid file_id", http.StatusBadRequest)
			return
		}
		logHandlerError(w, "delete resource failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(appResourceResponse{Success: true})
}

func logHandlerError(w http.ResponseWriter, message string, err error) {
	log.Printf("[Handler] %s: %v", message, err)
	http.Error(w, message, http.StatusInternalServerError)
}

func (b *Bridge) StoreResource(owner, category, description, fileName, contentType string, src io.Reader) (*appResourceItem, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	category = normalizeResourceCategory(category)
	if category == "" {
		category = inferResourceCategory(fileName, contentType)
	}
	if strings.TrimSpace(fileName) == "" {
		return nil, fmt.Errorf("empty file name")
	}
	if src == nil {
		return nil, fmt.Errorf("empty file stream")
	}
	dir, err := b.ensureResourceDir(owner, category)
	if err != nil {
		return nil, err
	}
	safeName := sanitizeFileName(fileName)
	filePath := filepath.Join(dir, safeName)
	out, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create resource failed: %w", err)
	}
	maxUploadBytes := b.cfg.maxResourceUploadBytes()
	written, copyErr := io.Copy(out, io.LimitReader(src, maxUploadBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("write resource failed: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close resource failed: %w", closeErr)
	}
	if written > maxUploadBytes {
		_ = os.Remove(filePath)
		return nil, errResourceExceedsMaxSize
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve resource abs path failed: %w", err)
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(safeName)), ".")
	if strings.TrimSpace(contentType) == "" {
		contentType = attachmentMimeType(category, safeName, format)
	}
	fileID, err := buildAttachmentFileIDWithTimestamp(b.cfg.AttachmentStoreDir, filePath, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	attachment := &AppAttachment{
		MessageType: category,
		FileID:      fileID,
		FileName:    safeName,
		FilePath:    absPath,
		FileSize:    int(written),
		Format:      format,
		MIMEType:    contentType,
		Description: strings.TrimSpace(description),
		Meta: map[string]any{
			"resource_category": category,
			"resource":          true,
		},
	}
	b.applyAttachmentStorageFromFile(owner, attachment)
	return b.resourceItemFromAttachment(owner, category, attachment, time.Now()), nil
}

func (cfg *Config) maxResourceUploadBytes() int64 {
	if cfg == nil || cfg.MaxResourceUploadMB <= 0 {
		return maxResourceUploadBytes
	}
	return cfg.MaxResourceUploadMB * bytesPerMiB
}

func (b *Bridge) ListResources(owner, category string) ([]appResourceItem, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	category = normalizeResourceCategory(category)
	root := filepath.Join(attachmentRootDir(b.cfg.AttachmentStoreDir), sanitizeFileName(owner), "resources")
	if category != "" {
		root = filepath.Join(root, category)
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []appResourceItem{}, nil
		}
		return nil, fmt.Errorf("stat resource root: %w", err)
	}

	items := make([]appResourceItem, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".meta.json") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fileID, err := buildAttachmentFileID(b.cfg.AttachmentStoreDir, path)
		if err != nil {
			return err
		}
		itemCategory := category
		if itemCategory == "" {
			itemCategory = categoryFromResourcePath(b.cfg.AttachmentStoreDir, owner, path)
		}
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name())), ".")
		mimeType := firstNonEmpty(mime.TypeByExtension(filepath.Ext(entry.Name())), attachmentMimeType(itemCategory, entry.Name(), format))
		storageProvider := "local"
		objectKey := ""
		if b.obsStorage != nil && b.obsStorage.Enabled() {
			storageProvider = "obs"
			objectKey = buildAttachmentObjectKey(itemCategory, owner, fileID, entry.Name())
		}
		items = append(items, appResourceItem{
			Category:        itemCategory,
			MessageType:     itemCategory,
			FileID:          fileID,
			FileName:        entry.Name(),
			FileSize:        info.Size(),
			FileFormat:      format,
			MIMEType:        mimeType,
			StorageProvider: storageProvider,
			ObjectKey:       objectKey,
			DownloadURL:     "/api/app/attachments/" + fileID,
			UpdatedAt:       info.ModTime().UnixMilli(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan resources: %w", err)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return items, nil
}

func (b *Bridge) ResourceUsage(owner, category string) (*appResourceUsage, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	category = normalizeResourceCategory(category)
	root := filepath.Join(attachmentRootDir(b.cfg.AttachmentStoreDir), sanitizeFileName(owner), "resources")
	usage := &appResourceUsage{}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return usage, nil
		}
		return nil, fmt.Errorf("stat resource root: %w", err)
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".meta.json") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		itemCategory := categoryFromResourcePath(b.cfg.AttachmentStoreDir, owner, path)
		usage.TotalCount++
		usage.TotalSize += info.Size()
		if category == "" || itemCategory == category {
			usage.CategoryCount++
			usage.CategorySize += info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan resource usage: %w", err)
	}
	return usage, nil
}

func (b *Bridge) DeleteResource(owner, fileID string) error {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return fmt.Errorf("empty owner")
	}
	filePath, err := resolveAttachmentPath(b.cfg.AttachmentStoreDir, fileID)
	if err != nil {
		return err
	}
	absResourceRoot, err := filepath.Abs(filepath.Join(attachmentRootDir(b.cfg.AttachmentStoreDir), sanitizeFileName(owner), "resources"))
	if err != nil {
		return fmt.Errorf("resolve resource root: %w", err)
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("resolve resource path: %w", err)
	}
	rootPrefix := absResourceRoot + string(filepath.Separator)
	if absPath != absResourceRoot && !strings.HasPrefix(absPath, rootPrefix) {
		return fmt.Errorf("not an app resource")
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if info.IsDir() || strings.HasSuffix(info.Name(), ".meta.json") {
		return fmt.Errorf("invalid resource file")
	}
	category := categoryFromResourcePath(b.cfg.AttachmentStoreDir, owner, absPath)
	objectKey := buildAttachmentObjectKey(category, owner, fileID, info.Name())
	if b.obsStorage != nil && b.obsStorage.Enabled() && strings.TrimSpace(objectKey) != "" {
		if err := b.obsStorage.DeleteObject(context.Background(), objectKey); err != nil {
			log.Printf("[Bridge] delete resource object failed file_id=%s key=%s err=%v", fileID, objectKey, err)
		}
	}
	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("delete resource file: %w", err)
	}
	if err := os.Remove(absPath + ".meta.json"); err != nil && !os.IsNotExist(err) {
		log.Printf("[Bridge] delete resource meta failed file=%s err=%v", absPath+".meta.json", err)
	}
	cleanupEmptyResourceDirs(absResourceRoot, filepath.Dir(absPath))
	return nil
}

func cleanupEmptyResourceDirs(root, dir string) {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	for dir != root {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func (b *Bridge) ensureResourceDir(owner, category string) (string, error) {
	root := attachmentRootDir(b.cfg.AttachmentStoreDir)
	dir := filepath.Join(root, sanitizeFileName(owner), "resources", normalizeResourceCategory(category), time.Now().Format("20060102"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir resource dir failed: %w", err)
	}
	return dir, nil
}

func (b *Bridge) resourceItemFromAttachment(owner, category string, attachment *AppAttachment, updatedAt time.Time) *appResourceItem {
	if attachment == nil {
		return nil
	}
	return &appResourceItem{
		Category:        category,
		MessageType:     category,
		FileID:          attachment.FileID,
		FileName:        attachment.FileName,
		FileSize:        int64(attachment.FileSize),
		FileFormat:      attachment.Format,
		MIMEType:        attachment.MIMEType,
		StorageProvider: firstNonEmpty(attachment.StorageProvider, "local"),
		ObjectKey:       attachment.ObjectKey,
		DownloadURL:     "/api/app/attachments/" + attachment.FileID,
		Description:     attachment.Description,
		Metadata: map[string]any{
			"owner":             strings.TrimSpace(owner),
			"resource_category": category,
		},
		UpdatedAt: updatedAt.UnixMilli(),
	}
}

func normalizeResourceCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	switch category {
	case "live2d", "image", "audio", "video", "file":
		return category
	case "picture", "photo", "img":
		return "image"
	case "model":
		return "live2d"
	default:
		return ""
	}
}

func inferResourceCategory(fileName, contentType string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case ext == ".model3.json" || ext == ".moc3" || ext == ".zip" || strings.Contains(strings.ToLower(fileName), "live2d"):
		return "live2d"
	case strings.HasPrefix(ct, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".bmp":
		return "image"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	case strings.HasPrefix(ct, "video/"):
		return "video"
	default:
		return "file"
	}
}

func categoryFromResourcePath(rootDir, owner, path string) string {
	absRoot, err := filepath.Abs(filepath.Join(attachmentRootDir(rootDir), sanitizeFileName(owner), "resources"))
	if err != nil {
		return "file"
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "file"
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "file"
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/")
	if len(parts) == 0 {
		return "file"
	}
	if category := normalizeResourceCategory(parts[0]); category != "" {
		return category
	}
	return "file"
}
