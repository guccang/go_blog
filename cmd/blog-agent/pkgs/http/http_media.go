package http

import (
	"blog"
	"config"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	h "net/http"
	"os"
	"path/filepath"
	"persistence"
	"strings"
	"time"
)

const maxEditorImageBytes = 10 << 20

func HandleMediaUpload(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	if r.Method != h.MethodPost {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	r.Body = h.MaxBytesReader(w, r.Body, maxEditorImageBytes+1024*1024)
	if err := r.ParseMultipartForm(maxEditorImageBytes); err != nil {
		h.Error(w, "图片不能超过 10MB", h.StatusRequestEntityTooLarge)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		h.Error(w, "缺少图片文件", h.StatusBadRequest)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxEditorImageBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxEditorImageBytes {
		h.Error(w, "图片读取失败或超过 10MB", h.StatusBadRequest)
		return
	}
	mimeType := h.DetectContentType(content)
	if !allowedEditorImageType(mimeType) {
		h.Error(w, "仅支持 PNG、JPEG、GIF、WebP 图片", h.StatusBadRequest)
		return
	}
	id, err := newMediaID()
	if err != nil {
		h.Error(w, "生成图片标识失败", h.StatusInternalServerError)
		return
	}
	extension := imageExtension(mimeType, header.Filename)
	storageName := id + extension
	mediaDir := filepath.Join(config.GetExePath(), "data", "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		h.Error(w, "创建图片目录失败", h.StatusInternalServerError)
		return
	}
	path := filepath.Join(mediaDir, storageName)
	if err := os.WriteFile(path, content, 0644); err != nil {
		h.Error(w, "保存图片失败", h.StatusInternalServerError)
		return
	}
	account := getAccountFromRequest(r)
	asset := persistence.MediaAsset{ID: id, Account: account, StorageName: storageName, MIMEType: mimeType, SizeBytes: int64(len(content)), CreatedAt: time.Now().Format("2006-01-02 15:04:05")}
	if err := persistence.SaveMediaAsset(asset); err != nil {
		_ = os.Remove(path)
		h.Error(w, "记录图片失败", h.StatusInternalServerError)
		return
	}
	emitUsageHook(r, account, blog.HookFeatureUsed, "image_upload", "media", id, "editor", "", map[string]any{"mime_type": mimeType, "size_bytes": len(content)}, map[string]any{"status": "success"})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"url": "/media/" + id, "alt": strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))})
}

func HandleMediaGet(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/media/")
	if id == "" || strings.ContainsAny(id, `/\\`) {
		h.Error(w, "invalid media id", h.StatusBadRequest)
		return
	}
	asset, err := persistence.GetMediaAsset(getAccountFromRequest(r), id)
	if err != nil {
		h.NotFound(w, r)
		return
	}
	path := filepath.Join(config.GetExePath(), "data", "media", asset.StorageName)
	file, err := os.Open(path)
	if err != nil {
		h.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		h.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", asset.MIMEType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.ServeContent(w, r, asset.StorageName, info.ModTime(), file)
}

func allowedEditorImageType(mimeType string) bool {
	return mimeType == "image/png" || mimeType == "image/jpeg" || mimeType == "image/gif" || mimeType == "image/webp"
}

func imageExtension(mimeType, filename string) string {
	if extensions, err := mime.ExtensionsByType(mimeType); err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
		return ext
	}
	return ".img"
}

func newMediaID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random media id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
