package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DefaultCortanaEnabled         = true
	DefaultCortanaAllowFullAccess = true
	DefaultCortanaAutoPlay        = true
	DefaultCortanaProactiveMode   = "high"
	DefaultCortanaPersonaName     = "Cortana"
)

type CortanaSettings struct {
	Enabled            bool   `json:"enabled"`
	AllowFullAccess    bool   `json:"allow_full_access"`
	AutoPlay           bool   `json:"auto_play"`
	ProactiveMode      string `json:"proactive_mode"`
	PersonaName        string `json:"persona_name,omitempty"`
	OwnerTitle         string `json:"owner_title,omitempty"`
	PersonaDescription string `json:"persona_description,omitempty"`
	VoiceWakeEnabled   bool   `json:"voice_wake_enabled,omitempty"`
	WakePhrase         string `json:"wake_phrase,omitempty"`
	UpdatedAt          int64  `json:"updated_at"`
}

type CortanaSettingsStore struct {
	mu       sync.RWMutex
	path     string
	settings map[string]CortanaSettings
}

func NewCortanaSettingsStore(path string) *CortanaSettingsStore {
	store := &CortanaSettingsStore{
		path:     strings.TrimSpace(path),
		settings: make(map[string]CortanaSettings),
	}
	store.load()
	return store
}

func DefaultCortanaSettings() CortanaSettings {
	return CortanaSettings{
		Enabled:         DefaultCortanaEnabled,
		AllowFullAccess: DefaultCortanaAllowFullAccess,
		AutoPlay:        DefaultCortanaAutoPlay,
		ProactiveMode:   DefaultCortanaProactiveMode,
		PersonaName:     DefaultCortanaPersonaName,
		UpdatedAt:       time.Now().UnixMilli(),
	}
}

func NormalizeCortanaSettings(in CortanaSettings) CortanaSettings {
	out := in
	if strings.TrimSpace(out.ProactiveMode) == "" {
		out.ProactiveMode = DefaultCortanaProactiveMode
	} else {
		out.ProactiveMode = strings.ToLower(strings.TrimSpace(out.ProactiveMode))
	}
	if strings.TrimSpace(out.PersonaName) == "" {
		out.PersonaName = DefaultCortanaPersonaName
	} else {
		out.PersonaName = strings.TrimSpace(out.PersonaName)
	}
	out.OwnerTitle = strings.TrimSpace(out.OwnerTitle)
	out.PersonaDescription = strings.TrimSpace(out.PersonaDescription)
	out.WakePhrase = strings.TrimSpace(out.WakePhrase)
	if out.UpdatedAt <= 0 {
		out.UpdatedAt = time.Now().UnixMilli()
	}
	return out
}

func (s *CortanaSettingsStore) Get(account string) CortanaSettings {
	account = strings.TrimSpace(account)
	if account == "" {
		return NormalizeCortanaSettings(DefaultCortanaSettings())
	}

	s.mu.RLock()
	current, ok := s.settings[account]
	s.mu.RUnlock()
	if ok {
		return NormalizeCortanaSettings(current)
	}
	return NormalizeCortanaSettings(DefaultCortanaSettings())
}

func (s *CortanaSettingsStore) Set(account string, settings CortanaSettings) CortanaSettings {
	account = strings.TrimSpace(account)
	if account == "" {
		return NormalizeCortanaSettings(DefaultCortanaSettings())
	}

	normalized := NormalizeCortanaSettings(settings)

	s.mu.Lock()
	s.settings[account] = normalized
	s.mu.Unlock()
	s.save()
	return normalized
}

func (s *CortanaSettingsStore) load() {
	if s == nil || s.path == "" {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil || len(data) == 0 {
		return
	}

	var raw map[string]CortanaSettings
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for account, settings := range raw {
		account = strings.TrimSpace(account)
		if account == "" {
			continue
		}
		s.settings[account] = NormalizeCortanaSettings(settings)
	}
}

func (s *CortanaSettingsStore) save() {
	if s == nil || s.path == "" {
		return
	}

	s.mu.RLock()
	data, err := json.MarshalIndent(s.settings, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, append(data, '\n'), 0644)
}
