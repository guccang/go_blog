package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxResourceUploadBytes int64 = 128 << 20

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

type appResourceResponse struct {
	Success bool              `json:"success"`
	Item    *appResourceItem  `json:"item,omitempty"`
	Items   []appResourceItem `json:"items,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func (h *Handler) HandleResources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListResources(w, r)
	case http.MethodPost:
		h.handleUploadResource(w, r)
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(appResourceResponse{
		Success: true,
		Items:   items,
	})
}

func (h *Handler) handleUploadResource(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(maxResourceUploadBytes); err != nil {
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, userID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := strings.TrimSpace(header.Filename)
	if fileName == "" {
		http.Error(w, "file name is required", http.StatusBadRequest)
		return
	}
	category := normalizeResourceCategory(r.FormValue("category"))
	if category == "" {
		category = inferResourceCategory(fileName, header.Header.Get("Content-Type"))
	}
	contentType := strings.TrimSpace(firstNonEmpty(
		r.FormValue("content_type"),
		header.Header.Get("Content-Type"),
		mime.TypeByExtension(filepath.Ext(fileName)),
	))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	description := strings.TrimSpace(r.FormValue("description"))
	item, err := h.bridge.StoreResource(userID, category, description, fileName, contentType, file)
	if err != nil {
		logHandlerError(w, "upload resource failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(appResourceResponse{
		Success: true,
		Item:    item,
	})
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
	written, copyErr := io.Copy(out, io.LimitReader(src, maxResourceUploadBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("write resource failed: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close resource failed: %w", closeErr)
	}
	if written > maxResourceUploadBytes {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("resource exceeds max size")
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
