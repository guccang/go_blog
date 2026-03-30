package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

// Handler handles app inbound HTTP and WebSocket requests.
type Handler struct {
	cfg    *Config
	bridge *Bridge
	auth   *authManager
}

func NewHandler(cfg *Config, bridge *Bridge, auth *authManager) *Handler {
	return &Handler{cfg: cfg, bridge: bridge, auth: auth}
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
			case "invalid_credentials":
				resp.Error = ae.Message
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	log.Printf("[Handler] login success user=%s remote=%s", session.Account, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(loginResponse{
		Success:      true,
		SessionToken: session.Token,
		UserID:       session.Account,
		ExpiresAt:    session.ExpiresAt.UnixMilli(),
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
	if msg.UserID == "" || msg.Content == "" {
		http.Error(w, "user_id and content are required", http.StatusBadRequest)
		return
	}
	if !h.validateAppSession(r, msg.UserID) {
		log.Printf("[Handler] rejected app message: invalid login user=%s remote=%s", msg.UserID, r.RemoteAddr)
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}

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
