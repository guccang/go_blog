package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

type CortanaSyncPayload struct {
	Account    string          `json:"account"`
	Registered bool            `json:"registered"`
	Online     bool            `json:"online"`
	LastSeenAt int64           `json:"last_seen_at"`
	Settings   CortanaSettings `json:"settings"`
}

type CortanaEventPayload struct {
	Account       string         `json:"account"`
	TriggerSource string         `json:"trigger_source"`
	TriggerReason string         `json:"trigger_reason"`
	Content       string         `json:"content,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Timestamp     int64          `json:"timestamp"`
}

type cortanaAccountSync interface {
	SyncUserSession(payload CortanaSyncPayload) error
	UnregisterAccount(account string) error
	TriggerEvent(payload CortanaEventPayload) error
}

type cortanaToolSync struct {
	cfg    *Config
	bridge *Bridge
}

func newCortanaToolSync(cfg *Config, bridge *Bridge) cortanaAccountSync {
	return &cortanaToolSync{
		cfg:    cfg,
		bridge: bridge,
	}
}

func (s *cortanaToolSync) SyncUserSession(payload CortanaSyncPayload) error {
	account := strings.TrimSpace(payload.Account)
	if account == "" {
		return fmt.Errorf("account is required")
	}
	payload.Account = account
	payload.Settings = NormalizeCortanaSettings(payload.Settings)
	if payload.LastSeenAt <= 0 {
		payload.LastSeenAt = time.Now().UnixMilli()
	}
	return s.call("cortana.SyncUserSession", payload)
}

func (s *cortanaToolSync) UnregisterAccount(account string) error {
	return s.call("cortana.UnregisterAccount", map[string]string{"account": account})
}

func (s *cortanaToolSync) TriggerEvent(payload CortanaEventPayload) error {
	payload.Account = strings.TrimSpace(payload.Account)
	payload.TriggerSource = strings.TrimSpace(payload.TriggerSource)
	payload.TriggerReason = strings.TrimSpace(payload.TriggerReason)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.Summary = strings.TrimSpace(payload.Summary)
	if payload.Account == "" {
		return fmt.Errorf("account is required")
	}
	if payload.TriggerSource == "" {
		return fmt.Errorf("trigger_source is required")
	}
	if payload.TriggerReason == "" {
		payload.TriggerReason = payload.TriggerSource
	}
	if payload.Timestamp <= 0 {
		payload.Timestamp = time.Now().UnixMilli()
	}
	return s.call("cortana.TriggerEvent", payload)
}

func (s *cortanaToolSync) call(toolName string, arguments any) error {
	if s == nil || s.bridge == nil || s.cfg == nil {
		return fmt.Errorf("cortana sync is not initialized")
	}
	if strings.TrimSpace(s.cfg.CortanaAgentID) == "" {
		return nil
	}

	result, err := s.bridge.CallTool(
		s.cfg.CortanaAgentID,
		toolName,
		arguments,
		5*time.Second,
	)
	if err != nil {
		return err
	}

	var payload map[string]any
	if result.Result != "" {
		if unmarshalErr := json.Unmarshal([]byte(result.Result), &payload); unmarshalErr != nil {
			log.Printf("[CortanaSync] tool=%s result=%s", toolName, result.Result)
			return nil
		}
	}

	log.Printf("[CortanaSync] tool=%s result=%v", toolName, payload)
	return nil
}
