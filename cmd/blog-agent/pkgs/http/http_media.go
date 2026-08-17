package http

import (
	"blog"
	"config"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	h "net/http"
	"os"
	"path/filepath"
	"persistence"
	"strings"
	"time"
)

const maxEditorFileBytes = 10 << 20

var allowedEditorTextExtensions = map[string]bool{
	".txt": true, ".md": true, ".html": true, ".htm": true,
	".csv": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true,
}

type MediaViewerData struct {
	Name        string
	Mode        string
	TypeLabel   string
	MIMEType    string
	SizeLabel   string
	Content     string
	DownloadURL string
	RenderURL   string
}

func HandleMediaUpload(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	if r.Method != h.MethodPost {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	r.Body = h.MaxBytesReader(w, r.Body, maxEditorFileBytes+1024*1024)
	if err := r.ParseMultipartForm(maxEditorFileBytes); err != nil {
		h.Error(w, "文件不能超过 10MB", h.StatusRequestEntityTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		// 兼容旧版图片上传页面和已经缓存的前端脚本。
		file, header, err = r.FormFile("image")
	}
	if err != nil {
		h.Error(w, "缺少上传文件", h.StatusBadRequest)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxEditorFileBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxEditorFileBytes {
		h.Error(w, "文件读取失败、内容为空或超过 10MB", h.StatusBadRequest)
		return
	}
	originalName := cleanUploadFilename(header.Filename)
	mimeType, extension, isImage, ok := editorFileType(content, originalName)
	if !ok {
		h.Error(w, "仅支持图片、TXT、Markdown、HTML、CSV、JSON、XML、YAML、ZIP 和 PDF 文件", h.StatusBadRequest)
		return
	}
	id, err := newMediaID()
	if err != nil {
		h.Error(w, "生成文件标识失败", h.StatusInternalServerError)
		return
	}
	if originalName == "" {
		originalName = "attachment" + extension
	}
	storageName := id + extension
	mediaDir := filepath.Join(config.GetExePath(), "data", "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		h.Error(w, "创建文件目录失败", h.StatusInternalServerError)
		return
	}
	path := filepath.Join(mediaDir, storageName)
	if err := os.WriteFile(path, content, 0644); err != nil {
		h.Error(w, "保存文件失败", h.StatusInternalServerError)
		return
	}
	account := getAccountFromRequest(r)
	asset := persistence.MediaAsset{ID: id, Account: account, StorageName: storageName, OriginalName: originalName, MIMEType: mimeType, SizeBytes: int64(len(content)), CreatedAt: time.Now().Format("2006-01-02 15:04:05")}
	if err := persistence.SaveMediaAsset(asset); err != nil {
		_ = os.Remove(path)
		h.Error(w, "记录文件失败", h.StatusInternalServerError)
		return
	}
	feature := "file_upload"
	if isImage {
		feature = "image_upload"
	}
	emitUsageHook(r, account, blog.HookFeatureUsed, feature, "media", id, "editor", "", map[string]any{"mime_type": mimeType, "size_bytes": len(content), "original_name": originalName}, map[string]any{"status": "success"})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	mediaURL := "/media/" + id
	if isEditorTextAsset(asset) {
		mediaURL = "/media/view/" + id
	}
	json.NewEncoder(w).Encode(map[string]any{
		"url": mediaURL, "name": originalName,
		"alt": strings.TrimSuffix(originalName, filepath.Ext(originalName)), "mime_type": mimeType, "is_image": isImage,
	})
}

func HandleMediaGet(w h.ResponseWriter, r *h.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/media/")
	if id == "" || strings.ContainsAny(id, `/\\`) {
		h.Error(w, "invalid media id", h.StatusBadRequest)
		return
	}
	var asset *persistence.MediaAsset
	account := getAccountFromRequest(r)
	if account != "" {
		asset, _ = persistence.GetMediaAsset(account, id)
	}
	if asset == nil {
		// 匿名用户或其他账号只能读取被安全公开博客实际引用的图片。
		asset, _ = persistence.GetPublicBlogMediaAsset(id)
		if asset == nil {
			h.NotFound(w, r)
			return
		}
	}
	if isEditorTextAsset(*asset) && r.URL.Query().Get("download") != "1" {
		h.Redirect(w, r, "/media/view/"+id, h.StatusSeeOther)
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
	if !allowedEditorImageType(asset.MIMEType) {
		filename := asset.OriginalName
		if filename == "" {
			filename = asset.StorageName
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	}
	h.ServeContent(w, r, asset.StorageName, info.ModTime(), file)
}

func HandleMediaView(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	if r.Method != h.MethodGet && r.Method != h.MethodHead {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/media/view/")
	if id == "" || strings.ContainsAny(id, `/\\`) {
		h.Error(w, "invalid media id", h.StatusBadRequest)
		return
	}
	asset, err := persistence.GetMediaAsset(getAccountFromRequest(r), id)
	if err != nil {
		h.NotFound(w, r)
		return
	}
	if !isEditorTextAsset(*asset) {
		h.Redirect(w, r, "/media/"+id, h.StatusSeeOther)
		return
	}
	content, err := os.ReadFile(filepath.Join(config.GetExePath(), "data", "media", asset.StorageName))
	if err != nil {
		h.NotFound(w, r)
		return
	}
	if len(content) > maxEditorFileBytes {
		h.Error(w, "文件过大，无法预览", h.StatusRequestEntityTooLarge)
		return
	}
	tmpl, err := template.ParseFiles(filepath.Join(config.GetHttpTemplatePath(), "media_viewer.template"))
	if err != nil {
		h.Error(w, "加载文件预览页失败", h.StatusInternalServerError)
		return
	}
	name := asset.OriginalName
	if name == "" {
		name = asset.StorageName
	}
	mode := mediaPreviewMode(name)
	viewContent := string(content)
	renderURL := ""
	if mode == "html" {
		viewContent = ""
		renderURL = "/media/render/" + id
	}
	data := MediaViewerData{
		Name: name, Mode: mode, TypeLabel: mediaPreviewTypeLabel(name),
		MIMEType: asset.MIMEType, SizeLabel: formatMediaSize(asset.SizeBytes), Content: viewContent,
		DownloadURL: "/media/" + id + "?download=1", RenderURL: renderURL,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; frame-src 'self' data: blob:; base-uri 'none'; form-action 'none'")
	if err := tmpl.Execute(w, data); err != nil {
		return
	}
}

// HandleMediaRender 在独立的 CSP sandbox 中返回原始 HTML，避免附件脚本接触博客登录态。
func HandleMediaRender(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	if r.Method != h.MethodGet && r.Method != h.MethodHead {
		h.Error(w, "method not allowed", h.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/media/render/")
	if id == "" || strings.ContainsAny(id, `/\\`) {
		h.Error(w, "invalid media id", h.StatusBadRequest)
		return
	}
	asset, err := persistence.GetMediaAsset(getAccountFromRequest(r), id)
	if err != nil {
		h.NotFound(w, r)
		return
	}
	if mediaPreviewMode(asset.OriginalName) != "html" || !isEditorTextAsset(*asset) {
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
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-popups; default-src 'none'; img-src data: blob: https: http:; media-src data: blob: https: http:; font-src data: https: http:; style-src 'unsafe-inline' https: http:; script-src 'unsafe-inline' https: http:; connect-src https: http:; base-uri https: http:; form-action 'none'; frame-ancestors 'self'")
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

func editorFileType(content []byte, filename string) (mimeType, extension string, isImage, ok bool) {
	detectedType := h.DetectContentType(content)
	if allowedEditorImageType(detectedType) {
		return detectedType, imageExtension(detectedType, filename), true, true
	}
	extension = strings.ToLower(filepath.Ext(filename))
	switch extension {
	case ".zip":
		return detectedType, extension, false, detectedType == "application/zip"
	case ".pdf":
		return detectedType, extension, false, detectedType == "application/pdf"
	}
	if !allowedEditorTextExtensions[extension] || !allowedEditorTextType(detectedType) {
		return "", "", false, false
	}
	return detectedType, extension, false, true
}

func allowedEditorTextType(mimeType string) bool {
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xml"
}

func isEditorTextAsset(asset persistence.MediaAsset) bool {
	return allowedEditorTextExtensions[strings.ToLower(filepath.Ext(asset.OriginalName))] && allowedEditorTextType(asset.MIMEType)
}

func mediaPreviewMode(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".json":
		return "json"
	default:
		return "text"
	}
}

func mediaPreviewTypeLabel(filename string) string {
	extension := strings.TrimPrefix(strings.ToUpper(filepath.Ext(filename)), ".")
	if extension == "HTM" {
		return "HTML"
	}
	if extension == "MD" {
		return "MARKDOWN"
	}
	if extension == "" {
		return "TEXT"
	}
	return extension
}

func formatMediaSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func cleanUploadFilename(filename string) string {
	filename = filepath.Base(strings.ReplaceAll(filename, `\`, "/"))
	filename = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, filename)
	return strings.TrimSpace(filename)
}

func newMediaID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random media id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
