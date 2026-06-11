package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	ChatAgentLLM    = "llm"
	ChatAgentHermes = "hermes"
)

// AppPreferences 是 app 用户级偏好（按 user_id 区分）。
type AppPreferences struct {
	PreferredChatAgent string `json:"preferred_chat_agent"`
	UpdatedAt          int64  `json:"updated_at"`
}

type AppPreferencesStore struct {
	mu          sync.RWMutex
	path        string
	preferences map[string]AppPreferences
}

func NewAppPreferencesStore(path string) *AppPreferencesStore {
	store := &AppPreferencesStore{
		path:        strings.TrimSpace(path),
		preferences: make(map[string]AppPreferences),
	}
	store.load()
	return store
}

func DefaultAppPreferences() AppPreferences {
	return AppPreferences{
		PreferredChatAgent: ChatAgentLLM,
		UpdatedAt:          time.Now().UnixMilli(),
	}
}

func NormalizeAppPreferences(in AppPreferences) AppPreferences {
	out := in
	switch strings.ToLower(strings.TrimSpace(out.PreferredChatAgent)) {
	case ChatAgentHermes:
		out.PreferredChatAgent = ChatAgentHermes
	default:
		out.PreferredChatAgent = ChatAgentLLM
	}
	if out.UpdatedAt <= 0 {
		out.UpdatedAt = time.Now().UnixMilli()
	}
	return out
}

func (s *AppPreferencesStore) Get(account string) AppPreferences {
	account = strings.TrimSpace(account)
	if s == nil || account == "" {
		return NormalizeAppPreferences(DefaultAppPreferences())
	}
	s.mu.RLock()
	current, ok := s.preferences[account]
	s.mu.RUnlock()
	if ok {
		return NormalizeAppPreferences(current)
	}
	return NormalizeAppPreferences(DefaultAppPreferences())
}

func (s *AppPreferencesStore) Set(account string, preferences AppPreferences) AppPreferences {
	account = strings.TrimSpace(account)
	if s == nil || account == "" {
		return NormalizeAppPreferences(DefaultAppPreferences())
	}
	normalized := NormalizeAppPreferences(preferences)
	s.mu.Lock()
	s.preferences[account] = normalized
	s.mu.Unlock()
	s.save()
	return normalized
}

func (s *AppPreferencesStore) load() {
	if s == nil || s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil || len(data) == 0 {
		return
	}
	var raw map[string]AppPreferences
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for account, preferences := range raw {
		account = strings.TrimSpace(account)
		if account == "" {
			continue
		}
		s.preferences[account] = NormalizeAppPreferences(preferences)
	}
}

func (s *AppPreferencesStore) save() {
	if s == nil || s.path == "" {
		return
	}
	s.mu.RLock()
	data, err := json.MarshalIndent(s.preferences, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, append(data, '\n'), 0644)
}

// HandlePreferences 提供 GET/POST /api/app/preferences。
func (h *Handler) HandlePreferences(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	account := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if r.Method == http.MethodPost {
		var req struct {
			UserID             string `json:"user_id"`
			PreferredChatAgent string `json:"preferred_chat_agent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if account == "" {
			account = strings.TrimSpace(req.UserID)
		}
		if account == "" || !h.validateAppSession(r, account) {
			http.Error(w, "Login required", http.StatusUnauthorized)
			return
		}
		current := h.preferences.Get(account)
		if strings.TrimSpace(req.PreferredChatAgent) != "" {
			current.PreferredChatAgent = req.PreferredChatAgent
		}
		current.UpdatedAt = time.Now().UnixMilli()
		current = h.preferences.Set(account, current)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "preferences": current})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if account == "" || !h.validateAppSession(r, account) {
		http.Error(w, "Login required", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"preferences": h.preferences.Get(account),
		"chat_agents": []map[string]string{
			{"id": ChatAgentLLM, "label": "LLM Agent"},
			{"id": ChatAgentHermes, "label": "Hermes Agent"},
		},
	})
}
