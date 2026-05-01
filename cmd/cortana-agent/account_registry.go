package main

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type CortanaUserSettings struct {
	Enabled         bool   `json:"enabled"`
	AllowFullAccess bool   `json:"allow_full_access"`
	AutoPlay        bool   `json:"auto_play"`
	ProactiveMode   string `json:"proactive_mode"`
	UpdatedAt       int64  `json:"updated_at"`
}

type CortanaUserSession struct {
	Account         string              `json:"account"`
	Registered      bool                `json:"registered"`
	Online          bool                `json:"online"`
	AllowFullAccess bool                `json:"allow_full_access"`
	AutoPlay        bool                `json:"auto_play"`
	ProactiveMode   string              `json:"proactive_mode"`
	Enabled         bool                `json:"enabled"`
	LastSeenAt      int64               `json:"last_seen_at"`
	UpdatedAt       int64               `json:"updated_at"`
	Settings        CortanaUserSettings `json:"settings"`
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

func cloneCortanaSession(in *CortanaUserSession) *CortanaUserSession {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
