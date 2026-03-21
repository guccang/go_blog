package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Handlers HTTP API 处理器集合
type Handlers struct {
	cfg     *Config
	manager *DeployManager
}

// NewHandlers 创建处理器
func NewHandlers(cfg *Config, manager *DeployManager) *Handlers {
	return &Handlers{cfg: cfg, manager: manager}
}

// HandleUpload POST /api/upload — 上传 zip 包（MD5 去重）
func (h *Handlers) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 限制上传大小
	maxBytes := int64(h.cfg.MaxUploadSizeMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		jsonError(w, "文件过大或解析失败", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "缺少 file 字段", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 只接受 .zip
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		jsonError(w, "只接受 .zip 文件", http.StatusBadRequest)
		return
	}

	// 安全文件名：清理路径穿越
	filename := filepath.Base(filepath.Clean(header.Filename))
	if strings.Contains(filename, "..") {
		jsonError(w, "非法文件名", http.StatusBadRequest)
		return
	}

	// 写入临时文件，同时计算 MD5
	tmpPath := filepath.Join(h.cfg.UploadDir, filename+".tmp")
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		jsonError(w, "创建文件失败", http.StatusInternalServerError)
		return
	}

	hash := md5.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, hash), file)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		jsonError(w, "保存文件失败", http.StatusInternalServerError)
		return
	}

	md5sum := hex.EncodeToString(hash.Sum(nil))

	// 检查是否已有相同 MD5 的文件
	if existing, found := h.manager.FindDuplicateByMD5(md5sum); found {
		os.Remove(tmpPath)
		jsonResp(w, map[string]interface{}{
			"filename": existing,
			"size":     written,
			"skipped":  true,
			"message":  "文件已存在（MD5相同）: " + existing,
		})
		return
	}

	// 重命名为正式文件
	dstPath := filepath.Join(h.cfg.UploadDir, filename)
	if err := os.Rename(tmpPath, dstPath); err != nil {
		os.Remove(tmpPath)
		jsonError(w, "保存文件失败", http.StatusInternalServerError)
		return
	}

	h.manager.CacheMD5(filename, md5sum)

	jsonResp(w, map[string]interface{}{
		"filename": filename,
		"size":     written,
	})
}

// HandlePackages GET /api/packages — 已上传包列表
func (h *Handlers) HandlePackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	pkgs := h.manager.ListPackages()
	if pkgs == nil {
		pkgs = []PackageInfo{}
	}
	jsonResp(w, pkgs)
}

// HandleDeploy POST /api/deploy — 触发部署
func (h *Handlers) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Filename     string   `json:"filename"`
		TargetDir    string   `json:"target_dir"`
		Script       string   `json:"script"`
		ProtectFiles []string `json:"protect_files,omitempty"`
		SetupDirs    []string `json:"setup_dirs,omitempty"`
		DeployMode   string   `json:"deploy_mode,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Filename == "" || req.TargetDir == "" {
		jsonError(w, "filename 和 target_dir 必填", http.StatusBadRequest)
		return
	}

	// 安全检查
	req.Filename = filepath.Base(filepath.Clean(req.Filename))
	if strings.Contains(req.Filename, "..") {
		jsonError(w, "非法文件名", http.StatusBadRequest)
		return
	}
	req.TargetDir = filepath.Clean(req.TargetDir)
	if strings.Contains(req.TargetDir, "..") {
		jsonError(w, "非法目标目录", http.StatusBadRequest)
		return
	}

	// 检查文件存在
	zipPath := filepath.Join(h.cfg.UploadDir, req.Filename)
	if _, err := os.Stat(zipPath); err != nil {
		jsonError(w, "文件不存在: "+req.Filename, http.StatusNotFound)
		return
	}

	if req.Script == "" {
		req.Script = "publish.sh"
	}

	task := h.manager.CreateTask(req.Filename, req.TargetDir, req.Script, req.ProtectFiles, req.SetupDirs, req.DeployMode)
	go h.manager.RunDeploy(task)

	jsonResp(w, map[string]interface{}{
		"deploy_id": task.ID,
		"status":    task.Status,
	})
}

// HandleDeploys GET /api/deploys — 部署历史列表
func (h *Handlers) HandleDeploys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	tasks := h.manager.ListTasks()

	type deployInfo struct {
		ID         string `json:"id"`
		Filename   string `json:"filename"`
		TargetDir  string `json:"target_dir"`
		Script     string `json:"script"`
		Status     string `json:"status"`
		StartedAt  string `json:"started_at"`
		FinishedAt string `json:"finished_at,omitempty"`
		Error      string `json:"error,omitempty"`
		Duration   string `json:"duration,omitempty"`
	}

	var out []deployInfo
	for _, t := range tasks {
		d := deployInfo{
			ID:        t.ID,
			Filename:  t.Filename,
			TargetDir: t.TargetDir,
			Script:    t.Script,
			Status:    t.Status,
			StartedAt: t.StartedAt.Format("2006-01-02 15:04:05"),
		}
		if !t.FinishedAt.IsZero() {
			d.FinishedAt = t.FinishedAt.Format("2006-01-02 15:04:05")
			d.Duration = fmt.Sprintf("%.1fs", t.FinishedAt.Sub(t.StartedAt).Seconds())
		}
		if t.Error != "" {
			d.Error = t.Error
		}
		out = append(out, d)
	}
	if out == nil {
		out = []deployInfo{}
	}
	jsonResp(w, out)
}

// HandleDeployLogs GET /api/deploy/{id}/logs — SSE 实时日志 或 全量日志
func (h *Handlers) HandleDeployLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 从路径提取 deploy ID: /api/deploy/{id}/logs
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonError(w, "无效的路径", http.StatusBadRequest)
		return
	}
	deployID := parts[3]

	task := h.manager.GetTask(deployID)
	if task == nil {
		jsonError(w, "部署任务不存在", http.StatusNotFound)
		return
	}

	// mode=full: 一次性返回全量日志
	if r.URL.Query().Get("mode") == "full" {
		jsonResp(w, map[string]interface{}{
			"deploy_id": task.ID,
			"status":    task.Status,
			"error":     task.Error,
			"logs":      task.getLogs(),
		})
		return
	}

	// SSE 模式
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "不支持 SSE", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, snapshot := task.subscribe()
	defer task.unsubscribe(ch)

	// 先发送已有日志
	for _, entry := range snapshot {
		writeSSE(w, entry)
	}
	flusher.Flush()

	// 如果任务已结束，直接关闭
	if task.Status == "done" || task.Status == "error" {
		return
	}

	// 实时推送
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, entry)
			flusher.Flush()
			if entry.Level == "done" || entry.Level == "error" {
				return
			}
		case <-time.After(30 * time.Second):
			// 心跳保活
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// writeSSE 写入一条 SSE 事件
func writeSSE(w http.ResponseWriter, entry LogEntry) {
	data, _ := json.Marshal(entry)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// jsonResp 写入 JSON 响应
func jsonResp(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// jsonError 写入 JSON 错误响应
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
