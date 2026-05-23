package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type debugBundleIssue struct {
	Title           string   `json:"title,omitempty"`
	UserDescription string   `json:"user_description,omitempty"`
	Expected        string   `json:"expected,omitempty"`
	Actual          string   `json:"actual,omitempty"`
	ReproSteps      []string `json:"repro_steps,omitempty"`
}

type createDebugBundleRequest struct {
	UserID     string           `json:"user_id"`
	Issue      debugBundleIssue `json:"issue"`
	AppState   map[string]any   `json:"app_state,omitempty"`
	Timeline   []map[string]any `json:"timeline,omitempty"`
	ClientLogs []string         `json:"client_logs,omitempty"`
	ClientLog  string           `json:"client_log,omitempty"`
	Platform   string           `json:"platform,omitempty"`
	AppVersion string           `json:"app_version,omitempty"`
}

type debugBundleManifest struct {
	DebugID     string                `json:"debug_id"`
	Project     string                `json:"project"`
	ProjectDir  string                `json:"project_dir"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at,omitempty"`
	Issue       debugBundleIssue      `json:"issue"`
	Entrypoints map[string]any        `json:"entrypoints"`
	Constraints map[string]any        `json:"constraints"`
	Resources   []debugBundleResource `json:"resources,omitempty"`
}

type debugBundleResource struct {
	Path        string `json:"path"`
	FileName    string `json:"file_name"`
	Kind        string `json:"kind,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mime_type,omitempty"`
	Size        int64  `json:"size"`
	CreatedAt   string `json:"created_at"`
}

func (h *Handler) HandleDebugBundles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleCreateDebugBundle(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleDebugBundleItem(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/app/debug/bundles/") {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/app/debug/bundles/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	debugID := strings.TrimSpace(parts[0])
	action := ""
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		h.handleReadDebugBundle(w, r, debugID)
	case r.Method == http.MethodGet && action == "file":
		h.handleReadDebugBundleFile(w, r, debugID)
	case r.Method == http.MethodGet && action == "resources":
		h.handleListDebugBundleResources(w, r, debugID)
	case r.Method == http.MethodPost && action == "attach-client-log":
		h.handleAttachDebugClientLog(w, r, debugID)
	case r.Method == http.MethodPost && (action == "attach-file" || action == "upload-resource"):
		h.handleUploadDebugBundleResource(w, r, debugID)
	case r.Method == http.MethodPost && action == "redact":
		h.handleRedactDebugBundleFile(w, r, debugID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCreateDebugBundle(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req createDebugBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("parse request failed: %w", err))
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("user_id is required"))
		return
	}
	if !h.validateAppSession(r, req.UserID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	dir, manifest, err := h.createDebugBundle(req)
	if err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"debug_id":    manifest.DebugID,
		"bundle_path": dir,
		"manifest":    manifest,
	})
}

func (h *Handler) handleReadDebugBundle(w http.ResponseWriter, r *http.Request, debugID string) {
	if !h.authorizeDebugRead(w, r) {
		return
	}
	dir, err := h.resolveDebugBundleDir(debugID)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	manifest, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	summary, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"debug_id": debugID,
		"manifest": string(manifest),
		"summary":  string(summary),
		"files":    listDebugBundleFiles(dir),
	})
}

func (h *Handler) handleReadDebugBundleFile(w http.ResponseWriter, r *http.Request, debugID string) {
	if !h.authorizeDebugRead(w, r) {
		return
	}
	dir, err := h.resolveDebugBundleDir(debugID)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	fullPath, err := resolveBundleFilePath(dir, path)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		h.writeDebugError(w, http.StatusBadGateway, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) handleAttachDebugClientLog(w http.ResponseWriter, r *http.Request, debugID string) {
	if !h.authorizeDebugRead(w, r) {
		return
	}
	dir, err := h.resolveDebugBundleDir(debugID)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	var req struct {
		ClientLogs []string `json:"client_logs"`
		ClientLog  string   `json:"client_log"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("parse request failed: %w", err))
		return
	}
	content := formatClientLog(req.ClientLogs, req.ClientLog)
	if content == "" {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("client log is empty"))
		return
	}
	path := filepath.Join(dir, "logs", "flutter_client.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()
	if _, err := file.WriteString(redactSensitiveText(content)); err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	_ = appendTimeline(dir, map[string]any{
		"time":     time.Now().Format(time.RFC3339),
		"category": "client_log",
		"message":  "attached flutter client log",
	})
	_ = touchDebugBundleManifest(dir)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (h *Handler) handleUploadDebugBundleResource(w http.ResponseWriter, r *http.Request, debugID string) {
	if !h.authorizeDebugRead(w, r) {
		return
	}
	dir, err := h.resolveDebugBundleDir(debugID)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.ParseMultipartForm(maxResourceUploadBytes); err != nil {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("invalid multipart form: %w", err))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("resource")
	}
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("file is required"))
		return
	}
	defer file.Close()

	kind := sanitizeDebugFileName(firstNonEmpty(r.FormValue("kind"), r.FormValue("category"), "resource"))
	if kind == "." || kind == "" {
		kind = "resource"
	}
	description := strings.TrimSpace(r.FormValue("description"))
	fileName := sanitizeDebugFileName(firstNonEmpty(header.Filename, "resource.bin"))
	relDir := filepath.ToSlash(filepath.Join("resources", kind))
	fullDir := filepath.Join(dir, relDir)
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	stampedName := fmt.Sprintf("%s_%s", time.Now().Format("20060102_150405_000000000"), fileName)
	relPath := filepath.ToSlash(filepath.Join(relDir, stampedName))
	fullPath := filepath.Join(dir, filepath.FromSlash(relPath))
	out, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	size, copyErr := io.Copy(out, io.LimitReader(file, maxResourceUploadBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		h.writeDebugError(w, http.StatusInternalServerError, copyErr)
		return
	}
	if closeErr != nil {
		h.writeDebugError(w, http.StatusInternalServerError, closeErr)
		return
	}
	if size > maxResourceUploadBytes {
		_ = os.Remove(fullPath)
		h.writeDebugError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("resource exceeds %d bytes", maxResourceUploadBytes))
		return
	}

	resource := debugBundleResource{
		Path:        relPath,
		FileName:    fileName,
		Kind:        kind,
		Description: description,
		MIMEType:    header.Header.Get("Content-Type"),
		Size:        size,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	if err := appendDebugBundleResource(dir, resource); err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	_ = appendTimeline(dir, map[string]any{
		"time":     time.Now().Format(time.RFC3339),
		"category": "resource",
		"kind":     kind,
		"path":     relPath,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "resource": resource})
}

func (h *Handler) handleListDebugBundleResources(w http.ResponseWriter, r *http.Request, debugID string) {
	if !h.authorizeDebugRead(w, r) {
		return
	}
	dir, err := h.resolveDebugBundleDir(debugID)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	manifest, _ := readDebugBundleManifest(dir)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":   true,
		"debug_id":  debugID,
		"resources": manifest.Resources,
		"files":     listDebugBundleFiles(dir),
	})
}

func (h *Handler) handleRedactDebugBundleFile(w http.ResponseWriter, r *http.Request, debugID string) {
	if !h.authorizeDebugRead(w, r) {
		return
	}
	dir, err := h.resolveDebugBundleDir(debugID)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeDebugError(w, http.StatusBadRequest, fmt.Errorf("parse request failed: %w", err))
		return
	}
	fullPath, err := resolveBundleFilePath(dir, req.Path)
	if err != nil {
		h.writeDebugError(w, http.StatusBadRequest, err)
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		h.writeDebugError(w, http.StatusBadGateway, err)
		return
	}
	if err := os.WriteFile(fullPath, []byte(redactSensitiveText(string(data))), 0644); err != nil {
		h.writeDebugError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (h *Handler) createDebugBundle(req createDebugBundleRequest) (string, debugBundleManifest, error) {
	now := time.Now()
	debugID := fmt.Sprintf("dbg_%s_%s", now.Format("20060102_150405"), shortHash(fmt.Sprintf("%s:%d", req.UserID, now.UnixNano())))
	root, err := resolveDebugBundleRoot(h.cfg.DebugBundleDir)
	if err != nil {
		return "", debugBundleManifest{}, err
	}
	dir := filepath.Join(root, debugID)
	for _, sub := range []string{"logs", "screenshots", "traces", "repo", "validation", "resources"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return "", debugBundleManifest{}, fmt.Errorf("create bundle dir failed: %w", err)
		}
	}

	if req.Issue.Title == "" {
		req.Issue.Title = "Flutter App Debug Bundle"
	}
	appState := req.AppState
	if appState == nil {
		appState = map[string]any{}
	}
	appState["user_id"] = req.UserID
	appState["platform"] = strings.TrimSpace(req.Platform)
	appState["app_version"] = strings.TrimSpace(req.AppVersion)
	appState["created_at"] = now.Format(time.RFC3339)

	manifest := debugBundleManifest{
		DebugID:    debugID,
		Project:    "flutter-client-for-appagent",
		ProjectDir: "cmd/flutter-client-for-appagent/flutter_client_for_appagent",
		CreatedAt:  now.Format(time.RFC3339),
		UpdatedAt:  now.Format(time.RFC3339),
		Issue:      req.Issue,
		Entrypoints: map[string]any{
			"summary":     "summary.md",
			"timeline":    "timeline.jsonl",
			"client_log":  "logs/flutter_client.log",
			"server_logs": []string{},
			"resources":   "resources/",
		},
		Constraints: map[string]any{
			"encoding":           "UTF-8 without BOM",
			"forbidden_commands": []string{"flutter build apk", "build-apk.sh"},
			"allowed_validation": []string{"dart analyze", "flutter analyze", "dart format --set-exit-if-changed .", "flutter test"},
		},
	}

	if err := writeJSONFile(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		return "", manifest, err
	}
	if err := writeJSONFile(filepath.Join(dir, "issue.md"), req.Issue); err != nil {
		return "", manifest, err
	}
	if err := writeJSONFile(filepath.Join(dir, "app_state.json"), appState); err != nil {
		return "", manifest, err
	}
	if err := writeJSONLines(filepath.Join(dir, "timeline.jsonl"), req.Timeline); err != nil {
		return "", manifest, err
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "flutter_client.log"), []byte(redactSensitiveText(formatClientLog(req.ClientLogs, req.ClientLog))), 0644); err != nil {
		return "", manifest, err
	}
	serverLogs := h.collectServerLogs(dir)
	manifest.Entrypoints["server_logs"] = serverLogs
	_ = writeJSONFile(filepath.Join(dir, "manifest.json"), manifest)
	_ = writeRepoSnapshot(dir)
	_ = os.WriteFile(filepath.Join(dir, "validation", "allowed_commands.md"), []byte(allowedValidationMarkdown()), 0644)
	_ = writeJSONFile(filepath.Join(dir, "validation", "last_results.json"), map[string]any{"results": []any{}})
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(h.buildDebugSummary(req, appState, serverLogs)), 0644); err != nil {
		return "", manifest, err
	}
	return dir, manifest, nil
}

func (h *Handler) collectServerLogs(dir string) []string {
	sources, err := h.loadConfiguredLogSources()
	if err != nil {
		return nil
	}
	var collected []string
	for _, source := range sources {
		if source.Name == "" || source.Path == "" {
			continue
		}
		logPath, content, _, _, err := readLogContent(source.Path, "", 100)
		if err != nil {
			continue
		}
		name := sanitizeDebugFileName(source.Name) + ".log"
		rel := filepath.ToSlash(filepath.Join("logs", name))
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(redactSensitiveText(content)), 0644); err == nil {
			collected = append(collected, rel)
			_ = appendTimeline(dir, map[string]any{
				"time":     time.Now().Format(time.RFC3339),
				"category": "server_log",
				"source":   source.Name,
				"file":     filepath.Base(logPath),
			})
		}
	}
	return collected
}

func (h *Handler) buildDebugSummary(req createDebugBundleRequest, appState map[string]any, serverLogs []string) string {
	var b strings.Builder
	b.WriteString("# Flutter Debug Bundle\n\n")
	b.WriteString("## 问题\n\n")
	b.WriteString("- 标题: " + safeSummaryText(req.Issue.Title) + "\n")
	if req.Issue.UserDescription != "" {
		b.WriteString("- 描述: " + safeSummaryText(req.Issue.UserDescription) + "\n")
	}
	if req.Issue.Expected != "" {
		b.WriteString("- 期望: " + safeSummaryText(req.Issue.Expected) + "\n")
	}
	if req.Issue.Actual != "" {
		b.WriteString("- 实际: " + safeSummaryText(req.Issue.Actual) + "\n")
	}
	if len(req.Issue.ReproSteps) > 0 {
		b.WriteString("\n## 复现步骤\n\n")
		for i, step := range req.Issue.ReproSteps {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, safeSummaryText(step)))
		}
	}
	b.WriteString("\n## 当前状态\n\n")
	for _, key := range sortedMapKeys(appState) {
		b.WriteString(fmt.Sprintf("- %s: %v\n", key, appState[key]))
	}
	b.WriteString("\n## 日志入口\n\n")
	b.WriteString("- Flutter 客户端: logs/flutter_client.log\n")
	for _, rel := range serverLogs {
		b.WriteString("- 服务端: " + rel + "\n")
	}
	b.WriteString("\n## 相关文件候选\n\n")
	b.WriteString("- cmd/flutter-client-for-appagent/flutter_client_for_appagent/lib/main.dart\n")
	b.WriteString("- cmd/flutter-client-for-appagent/flutter_client_for_appagent/lib/cortana_page.dart\n")
	b.WriteString("- cmd/app-agent/main.go\n")
	b.WriteString("- cmd/app-agent/debug_bundle.go\n")
	b.WriteString("\n## 允许验证命令\n\n")
	b.WriteString("- dart analyze\n- flutter analyze\n- dart format --set-exit-if-changed .\n- flutter test\n")
	return redactSensitiveText(b.String())
}

func (h *Handler) authorizeDebugRead(w http.ResponseWriter, r *http.Request) bool {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return false
	}
	if !h.validateAppSession(r, userID) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *Handler) resolveDebugBundleDir(debugID string) (string, error) {
	if !validDebugID(debugID) {
		return "", fmt.Errorf("invalid debug_id")
	}
	root, err := resolveDebugBundleRoot(h.cfg.DebugBundleDir)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, debugID)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("debug bundle not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("debug bundle path is not a directory")
	}
	return dir, nil
}

func resolveDebugBundleRoot(configured string) (string, error) {
	root := strings.TrimSpace(configured)
	if root == "" {
		root = defaultFlutterDebugBundleDir
	}
	if filepath.IsAbs(root) {
		return root, nil
	}
	candidates := []string{root}
	if root == defaultFlutterDebugBundleDir {
		candidates = append(candidates,
			"cmd/flutter-client-for-appagent/flutter_client_for_appagent/.debug/flutter",
			"../../cmd/flutter-client-for-appagent/flutter_client_for_appagent/.debug/flutter",
		)
	}
	for _, candidate := range candidates {
		parent := filepath.Dir(candidate)
		if info, err := os.Stat(parent); err == nil && info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return filepath.Abs(root)
}

func (h *Handler) writeDebugError(w http.ResponseWriter, status int, err error) {
	msg := "debug request failed"
	if err != nil {
		msg = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg})
}

func resolveBundleFilePath(root, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	fullPath := filepath.Join(root, filepath.Clean(rel))
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if absPath != absRoot && !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid bundle file path")
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("bundle file path is a directory")
	}
	return absPath, nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func writeJSONLines(path string, items []map[string]any) error {
	var b bytes.Buffer
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, b.Bytes(), 0644)
}

func readDebugBundleManifest(dir string) (debugBundleManifest, error) {
	var manifest debugBundleManifest
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func appendDebugBundleResource(dir string, resource debugBundleResource) error {
	manifest, err := readDebugBundleManifest(dir)
	if err != nil {
		return err
	}
	manifest.Resources = append(manifest.Resources, resource)
	manifest.UpdatedAt = time.Now().Format(time.RFC3339)
	if manifest.Entrypoints == nil {
		manifest.Entrypoints = map[string]any{}
	}
	manifest.Entrypoints["resources"] = "resources/"
	return writeJSONFile(filepath.Join(dir, "manifest.json"), manifest)
}

func touchDebugBundleManifest(dir string) error {
	manifest, err := readDebugBundleManifest(dir)
	if err != nil {
		return err
	}
	manifest.UpdatedAt = time.Now().Format(time.RFC3339)
	return writeJSONFile(filepath.Join(dir, "manifest.json"), manifest)
}

func listDebugBundleFiles(dir string) []map[string]any {
	var files []map[string]any
	absRoot, err := filepath.Abs(dir)
	if err != nil {
		return files
	}
	_ = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		files = append(files, map[string]any{
			"path":        filepath.ToSlash(rel),
			"size":        info.Size(),
			"modified_at": info.ModTime().UnixMilli(),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		return fmt.Sprint(files[i]["path"]) < fmt.Sprint(files[j]["path"])
	})
	return files
}

func appendTimeline(dir string, item map[string]any) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(dir, "timeline.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func formatClientLog(lines []string, raw string) string {
	var b strings.Builder
	if strings.TrimSpace(raw) != "" {
		b.WriteString(raw)
		if !strings.HasSuffix(raw, "\n") {
			b.WriteByte('\n')
		}
	}
	for _, line := range lines {
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func writeRepoSnapshot(dir string) error {
	snapshots := map[string][]string{
		filepath.Join("repo", "git_status.txt"):          {"git", "status", "--short"},
		filepath.Join("repo", "changed_files.txt"):       {"git", "diff", "--name-only"},
		filepath.Join("repo", "dependency_snapshot.txt"): {"go", "version"},
	}
	for rel, cmd := range snapshots {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		text := string(out)
		if err != nil {
			text += "\nerror: " + err.Error() + "\n"
		}
		_ = os.WriteFile(filepath.Join(dir, rel), []byte(redactSensitiveText(text)), 0644)
	}
	return nil
}

func allowedValidationMarkdown() string {
	return "# Allowed Validation Commands\n\n- dart analyze\n- flutter analyze\n- dart format --set-exit-if-changed .\n- flutter test\n\nForbidden: `flutter build apk`, `build-apk.sh`, and other APK packaging commands.\n"
}

var sensitiveHeaderLinePattern = regexp.MustCompile(`(?im)^(Authorization|Cookie|Set-Cookie|X-Api-Key)\s*:\s*(.+)$`)
var sensitiveKeyPattern = regexp.MustCompile(`(?i)(authorization|cookie|set-cookie|x-api-key|password|access_token|refresh_token|session_token|delegation_token|private_key|secret|token)(["'\s:=]+)([^"'\s,}]+)`)

func redactSensitiveText(input string) string {
	if input == "" {
		return ""
	}
	redacted := sensitiveHeaderLinePattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := sensitiveHeaderLinePattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return parts[1] + ": " + redactedValue(parts[2])
	})
	redacted = sensitiveKeyPattern.ReplaceAllStringFunc(redacted, func(match string) string {
		parts := sensitiveKeyPattern.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		return parts[1] + parts[2] + redactedValue(parts[3])
	})
	scanner := bufio.NewScanner(strings.NewReader(redacted))
	var b strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "?") {
			if parsed, err := url.Parse(line); err == nil && parsed.RawQuery != "" {
				q := parsed.Query()
				for _, key := range []string{"token", "session_token", "access_token", "password"} {
					if value := q.Get(key); value != "" {
						q.Set(key, redactedValue(value))
					}
				}
				parsed.RawQuery = q.Encode()
				line = parsed.String()
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func redactedValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "<redacted:sha256:" + hex.EncodeToString(sum[:])[:8] + ">"
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:6]
}

func validDebugID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func sanitizeDebugFileName(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "log"
	}
	return b.String()
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func safeSummaryText(value string) string {
	return redactSensitiveText(strings.ReplaceAll(strings.TrimSpace(value), "\n", " "))
}
