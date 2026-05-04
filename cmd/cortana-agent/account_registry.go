package main

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type CortanaUserSettings struct {
	Enabled            bool   `json:"enabled"`
	AllowFullAccess    bool   `json:"allow_full_access"`
	AutoPlay           bool   `json:"auto_play"`
	ProactiveMode      string `json:"proactive_mode"`
	PersonaName        string `json:"persona_name,omitempty"`
	PersonaDescription string `json:"persona_description,omitempty"`
	UpdatedAt          int64  `json:"updated_at"`
}

type CortanaUserSession struct {
	Account            string              `json:"account"`
	Registered         bool                `json:"registered"`
	Online             bool                `json:"online"`
	AllowFullAccess    bool                `json:"allow_full_access"`
	AutoPlay           bool                `json:"auto_play"`
	ProactiveMode      string              `json:"proactive_mode"`
	Enabled            bool                `json:"enabled"`
	LastSeenAt         int64               `json:"last_seen_at"`
	UpdatedAt          int64               `json:"updated_at"`
	Settings           CortanaUserSettings `json:"settings"`
	CurrentContext     map[string]any      `json:"current_context,omitempty"`
	RecentInteractions []map[string]any    `json:"recent_interactions,omitempty"`
}

type CortanaSyncUserPayload struct {
	Account    string              `json:"account"`
	Registered bool                `json:"registered"`
	Online     bool                `json:"online"`
	LastSeenAt int64               `json:"last_seen_at"`
	Settings   CortanaUserSettings `json:"settings"`
}

type CortanaTriggerEventPayload struct {
	Account       string         `json:"account"`
	TriggerSource string         `json:"trigger_source"`
	TriggerReason string         `json:"trigger_reason"`
	Content       string         `json:"content,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Timestamp     int64          `json:"timestamp"`
}

type AccountRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*CortanaUserSession
}

func NewAccountRegistry() *AccountRegistry {
	return &AccountRegistry{sessions: make(map[string]*CortanaUserSession)}
}

func defaultCortanaUserSettings() CortanaUserSettings {
	return CortanaUserSettings{
		Enabled:         true,
		AllowFullAccess: true,
		AutoPlay:        true,
		ProactiveMode:   "high",
		PersonaName:     "Cortana",
		UpdatedAt:       time.Now().UnixMilli(),
	}
}

func normalizeCortanaSettings(in CortanaUserSettings) CortanaUserSettings {
	out := in
	if strings.TrimSpace(out.ProactiveMode) == "" {
		out.ProactiveMode = "high"
	} else {
		out.ProactiveMode = strings.ToLower(strings.TrimSpace(out.ProactiveMode))
	}
	if strings.TrimSpace(out.PersonaName) == "" {
		out.PersonaName = "Cortana"
	} else {
		out.PersonaName = strings.TrimSpace(out.PersonaName)
	}
	out.PersonaDescription = strings.TrimSpace(out.PersonaDescription)
	if out.UpdatedAt <= 0 {
		out.UpdatedAt = time.Now().UnixMilli()
	}
	return out
}

func (r *AccountRegistry) RegisterAccount(account string) bool {
	account = strings.TrimSpace(account)
	if account == "" {
		return false
	}
	r.mu.Lock()
	_, existed := r.sessions[account]
	r.mu.Unlock()
	r.SyncUserSession(CortanaSyncUserPayload{
		Account:    account,
		Registered: true,
		Settings:   defaultCortanaUserSettings(),
	})
	return !existed
}

func (r *AccountRegistry) SyncUserSession(payload CortanaSyncUserPayload) *CortanaUserSession {
	account := strings.TrimSpace(payload.Account)
	if account == "" {
		return nil
	}

	settings := normalizeCortanaSettings(payload.Settings)
	if payload.LastSeenAt <= 0 {
		payload.LastSeenAt = time.Now().UnixMilli()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	session := r.sessions[account]
	if session == nil {
		session = &CortanaUserSession{Account: account}
		r.sessions[account] = session
	}
	session.Registered = payload.Registered
	session.Online = payload.Online
	session.LastSeenAt = payload.LastSeenAt
	session.Settings = settings
	session.Enabled = settings.Enabled
	session.AllowFullAccess = settings.AllowFullAccess
	session.AutoPlay = settings.AutoPlay
	session.ProactiveMode = settings.ProactiveMode
	session.UpdatedAt = settings.UpdatedAt
	return cloneCortanaSession(session)
}

func (r *AccountRegistry) UnregisterAccount(account string) bool {
	account = strings.TrimSpace(account)
	if account == "" {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	_, existed := r.sessions[account]
	delete(r.sessions, account)
	return existed
}

func (r *AccountRegistry) HasAccount(account string) bool {
	session := r.GetSession(account)
	return session != nil && session.Registered
}

func (r *AccountRegistry) GetSession(account string) *CortanaUserSession {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneCortanaSession(r.sessions[account])
}

func (r *AccountRegistry) ListAccounts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	accounts := make([]string, 0, len(r.sessions))
	for account, session := range r.sessions {
		if session != nil && session.Registered {
			accounts = append(accounts, account)
		}
	}
	sort.Strings(accounts)
	return accounts
}

func (r *AccountRegistry) AppendInteraction(account string, item map[string]any) {
	account = strings.TrimSpace(account)
	if account == "" || len(item) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	session := r.sessions[account]
	if session == nil {
		session = &CortanaUserSession{Account: account}
		r.sessions[account] = session
	}
	entry := cloneAnyMap(item)
	if entry == nil {
		return
	}
	session.RecentInteractions = append(session.RecentInteractions, entry)
	const maxRecentInteractions = 8
	if len(session.RecentInteractions) > maxRecentInteractions {
		session.RecentInteractions = append([]map[string]any(nil), session.RecentInteractions[len(session.RecentInteractions)-maxRecentInteractions:]...)
	}
}

func (r *AccountRegistry) UpdateCurrentContext(account string, context map[string]any) {
	account = strings.TrimSpace(account)
	if account == "" || len(context) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	session := r.sessions[account]
	if session == nil {
		session = &CortanaUserSession{Account: account}
		r.sessions[account] = session
	}
	if session.CurrentContext == nil {
		session.CurrentContext = make(map[string]any)
	}
	for k, v := range context {
		session.CurrentContext[k] = v
	}
	session.CurrentContext["updated_at"] = time.Now().UnixMilli()
}

func cloneCortanaSession(in *CortanaUserSession) *CortanaUserSession {
	if in == nil {
		return nil
	}
	out := *in
	out.CurrentContext = cloneAnyMap(in.CurrentContext)
	if len(in.RecentInteractions) > 0 {
		out.RecentInteractions = make([]map[string]any, 0, len(in.RecentInteractions))
		for _, item := range in.RecentInteractions {
			out.RecentInteractions = append(out.RecentInteractions, cloneAnyMap(item))
		}
	}
	return &out
}
