package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Handler handles app inbound HTTP and WebSocket requests.
type Handler struct {
	cfg    *Config
	bridge *Bridge
	auth   *authManager
	client *http.Client
}

func NewHandler(cfg *Config, bridge *Bridge, auth *authManager) *Handler {
	return &Handler{
		cfg:    cfg,
		bridge: bridge,
		auth:   auth,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type codegenProjectsResponse struct {
	Success        bool                `json:"success"`
	CodingProjects []codingProjectInfo `json:"coding_projects"`
	DeployProjects []deployProjectInfo `json:"deploy_projects"`
	Error          string              `json:"error,omitempty"`
}

type codingProjectInfo struct {
	Name    string `json:"name"`
	AgentID string `json:"agent_id"`
	Agent   string `json:"agent"`
}

type deployProjectInfo struct {
	Name          string   `json:"name"`
	AgentID       string   `json:"agent_id"`
	Agent         string   `json:"agent"`
	DeployTargets []string `json:"deploy_targets"`
}

func authSuccessResponse(session *issuedAuthSession, obsAgentBaseURL string) loginResponse {
	if session == nil || session.Session == nil {
		return loginResponse{
			Success: false,
			Error:   "missing auth session",
		}
	}
	expiresIn := session.Session.ExpiresAt.Sub(time.Now()).Seconds()
	if expiresIn < 0 {
		expiresIn = 0
	}
	return loginResponse{
		Success:         true,
		SessionToken:    session.Session.Token,
		AccessToken:     session.Session.Token,
		RefreshToken:    session.RefreshToken,
		UserID:          session.Session.Account,
		ExpiresAt:       session.Session.ExpiresAt.UnixMilli(),
		ExpiresIn:       int64(expiresIn),
		TokenType:       "Bearer",
		ObsAgentBaseURL: strings.TrimSpace(obsAgentBaseURL),
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	resp := loginResponse{
		Success: false,
		Error:   "invalid account or password",
	}
	status := http.StatusUnauthorized
	if ae, ok := err.(*authError); ok {
		switch ae.Code {
		case "blog_agent_unreachable":
			status = http.StatusServiceUnavailable
			resp.Error = "blog-agent unreachable"
		case "blog_agent_api_missing":
			status = http.StatusBadGateway
			resp.Error = "blog-agent app auth api not found"
		case "blog_agent_bad_response", "blog_agent_bad_status":
			status = http.StatusBadGateway
			resp.Error = ae.Message
		case "invalid_credentials", "invalid_refresh_token":
			resp.Error = ae.Message
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		log.Printf("[Handler] unauthorized login remote=%s path=%s", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	session, err := h.auth.Login(req.UserID, req.Password)
	if err != nil {
		log.Printf("[Handler] login failed user=%s remote=%s err=%v", strings.TrimSpace(req.UserID), r.RemoteAddr, err)
		writeAuthError(w, err)
		return
	}

	log.Printf("[Handler] login success user=%s remote=%s", session.Session.Account, r.RemoteAddr)

	// 存储 delegation token 到 bridge
	if session.Session.DelegationToken != "" {
		h.bridge.SetDelegationToken(session.Session.Account, session.Session.DelegationToken)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authSuccessResponse(session, h.cfg.ObsAgentBaseURL))
}

func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		log.Printf("[Handler] unauthorized refresh remote=%s path=%s", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	session, err := h.auth.Refresh(req.UserID, req.RefreshToken)
	if err != nil {
		log.Printf("[Handler] refresh failed user=%s remote=%s err=%v", strings.TrimSpace(req.UserID), r.RemoteAddr, err)
		writeAuthError(w, err)
		return
	}

	log.Printf("[Handler] refresh success user=%s remote=%s", session.Session.Account, r.RemoteAddr)
	if session.Session.DelegationToken != "" {
		h.bridge.SetDelegationToken(session.Session.Account, session.Session.DelegationToken)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authSuccessResponse(session, h.cfg.ObsAgentBaseURL))
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		log.Printf("[Handler] unauthorized logout remote=%s path=%s", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req logoutRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
	}

	h.auth.Logout(readAppSessionToken(r), req.RefreshToken, req.UserID)
	if userID := strings.TrimSpace(req.UserID); userID != "" {
		h.bridge.SetDelegationToken(userID, "")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}

func (h *Handler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		log.Printf("[Handler] unauthorized app message remote=%s path=%s", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var msg AppMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	msg.UserID = strings.TrimSpace(msg.UserID)
	msg.Content = strings.TrimSpace(msg.Content)
	if msg.MessageType == "" {
		msg.MessageType = "text"
	}
	if msg.UserID == "" || (!allowEmptyAppContent(&msg) && msg.Content == "") {
		http.Error(w, "user_id and content are required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, msg.UserID) {
		log.Printf("[Handler] rejected app message: invalid login user=%s remote=%s", msg.UserID, r.RemoteAddr)
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	// 从 bridge 获取用户的 delegation token
	msg.DelegationToken = h.bridge.GetDelegationToken(msg.UserID)

	log.Printf("[Handler] App message accepted user=%s type=%s len=%d remote=%s content=%q",
		msg.UserID, msg.MessageType, len(msg.Content), r.RemoteAddr, shortText(msg.Content))
	go h.bridge.HandleAppMessage(&msg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		log.Printf("[Handler] unauthorized websocket remote=%s path=%s", r.RemoteAddr, r.URL.String())
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, userID) {
		log.Printf("[Handler] rejected websocket: invalid login user=%s remote=%s", userID, r.RemoteAddr)
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

	log.Printf("[Handler] websocket upgrade requested user=%s remote=%s", userID, r.RemoteAddr)

	if err := h.bridge.ServeWebSocket(w, r, userID); err != nil {
		log.Printf("[Handler] websocket failed for %s: %v", userID, err)
	}
}

func (h *Handler) HandleGroups(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}
		if !h.validateAppSession(r, userID) {
			http.Error(w, "Login required", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"groups":  h.bridge.groups.ListForUser(userID),
		})
		return

	case http.MethodPost:
		var req struct {
			Action  string `json:"action"`
			UserID  string `json:"user_id"`
			GroupID string `json:"group_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		req.Action = strings.TrimSpace(req.Action)
		req.UserID = strings.TrimSpace(req.UserID)
		req.GroupID = normalizeGroupID(req.GroupID)
		if req.UserID == "" || req.GroupID == "" {
			http.Error(w, "user_id and group_id are required", http.StatusBadRequest)
			return
		}
		if !h.validateAppSession(r, req.UserID) {
			http.Error(w, "Login required", http.StatusUnauthorized)
			return
		}

		var err error
		switch req.Action {
		case "create":
			var robotAccount string
			robotAccount, err = h.auth.EnsureGroupRobotAccount(req.GroupID)
			if err == nil {
				err = h.bridge.groups.Create(req.GroupID, req.UserID, robotAccount)
			}
		case "join":
			err = h.bridge.groups.Join(req.GroupID, req.UserID)
		case "leave":
			err = h.bridge.groups.Leave(req.GroupID, req.UserID)
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		log.Printf("[Handler] group action=%s group=%s user=%s", req.Action, req.GroupID, req.UserID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"groups":  h.bridge.groups.ListForUser(req.UserID),
		})
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleCodegenProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
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

	resp, err := h.client.Get(strings.TrimRight(h.cfg.CmdAgentBaseURL, "/") + "/api/codegen/projects")
	if err != nil {
		h.writeCodegenProjectsError(w, http.StatusBadGateway, "cmd-agent unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	var payload codegenProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		h.writeCodegenProjectsError(w, http.StatusBadGateway, "invalid cmd-agent response")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !payload.Success {
		errMsg := strings.TrimSpace(payload.Error)
		if errMsg == "" {
			errMsg = fmt.Sprintf("cmd-agent returned %d", resp.StatusCode)
		}
		h.writeCodegenProjectsError(w, http.StatusBadGateway, errMsg)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handler) writeCodegenProjectsError(w http.ResponseWriter, status int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(codegenProjectsResponse{
		Success: false,
		Error:   errMsg,
	})
}

func (h *Handler) HandleAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	fileID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/app/attachments/"))
	if fileID == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}
	filePath, err := resolveAttachmentPath(h.cfg.AttachmentStoreDir, fileID)
	if err != nil {
		http.Error(w, "Invalid file_id", http.StatusBadRequest)
		return
	}
	stat, err := os.Stat(filePath)
	if err != nil || stat.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "inline; filename="+stat.Name())
	http.ServeFile(w, r, filePath)
}

func (h *Handler) HandleUploadAPK(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorize(r) {
		log.Printf("[Handler] unauthorized upload apk remote=%s path=%s", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	groupID := normalizeGroupID(firstNonEmpty(r.FormValue("group_id"), r.FormValue("to_group")))
	toUser := strings.TrimSpace(r.FormValue("to_user"))
	if toUser == "" {
		toUser = strings.TrimSpace(r.FormValue("user_id"))
	}
	if groupID != "" && toUser != "" {
		http.Error(w, "to_user and group_id are mutually exclusive", http.StatusBadRequest)
		return
	}
	if toUser == "" && groupID == "" {
		http.Error(w, "to_user or group_id is required", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(r.FormValue("content"))

	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("apk")
	}
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
	if !strings.EqualFold(filepath.Ext(fileName), ".apk") {
		http.Error(w, "only .apk files are supported", http.StatusBadRequest)
		return
	}
	if content == "" {
		content = fmt.Sprintf("收到新的安装包：%s", fileName)
	}

	w.Header().Set("Content-Type", "application/json")
	if groupID != "" {
		attachment, recipients, err := h.bridge.PushUploadedAPKToGroup(groupID, content, fileName, file)
		if err != nil {
			log.Printf("[Handler] upload apk failed group_id=%s file=%s remote=%s err=%v", groupID, fileName, r.RemoteAddr, err)
			http.Error(w, "Upload APK failed", http.StatusInternalServerError)
			return
		}
		log.Printf("[Handler] upload apk accepted group_id=%s file=%s size=%d recipients=%d remote=%s",
			groupID, attachment.FileName, attachment.FileSize, len(recipients), r.RemoteAddr)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":         true,
			"group_id":        groupID,
			"recipient_users": recipients,
			"recipient_count": len(recipients),
			"message_type":    "file",
			"file_id":         attachment.FileID,
			"file_name":       attachment.FileName,
			"file_size":       attachment.FileSize,
			"file_format":     attachment.Format,
			"mime_type":       attachment.MIMEType,
		})
		return
	}

	attachment, err := h.bridge.PushUploadedAPK(toUser, content, fileName, file)
	if err != nil {
		log.Printf("[Handler] upload apk failed to_user=%s file=%s remote=%s err=%v", toUser, fileName, r.RemoteAddr, err)
		http.Error(w, "Upload APK failed", http.StatusInternalServerError)
		return
	}

	log.Printf("[Handler] upload apk accepted to_user=%s file=%s size=%d remote=%s", toUser, attachment.FileName, attachment.FileSize, r.RemoteAddr)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"to_user":      toUser,
		"message_type": "file",
		"file_id":      attachment.FileID,
		"file_name":    attachment.FileName,
		"file_size":    attachment.FileSize,
		"file_format":  attachment.Format,
		"mime_type":    attachment.MIMEType,
	})
}

func (h *Handler) authorize(r *http.Request) bool {
	if h.cfg.ReceiveToken == "" {
		return true
	}
	return readBearerToken(r) == h.cfg.ReceiveToken
}

func readBearerToken(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-App-Agent-Token"))
	if token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func readAppSessionToken(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-App-Agent-Session"))
	if token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("session_token"))
}

func (h *Handler) validateAppSession(r *http.Request, userID string) bool {
	if h.auth == nil {
		return false
	}
	return h.auth.Validate(readAppSessionToken(r), userID)
}

func allowEmptyAppContent(msg *AppMessage) bool {
	if msg == nil {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(msg.MessageType)) {
	case "", "text":
		return false
	default:
		return true
	}
}
