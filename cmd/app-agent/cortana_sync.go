package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

type cortanaAccountSync interface {
	RegisterAccount(account string) error
	UnregisterAccount(account string) error
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

func (s *cortanaToolSync) RegisterAccount(account string) error {
	return s.call("cortana.RegisterAccount", account)
}

func (s *cortanaToolSync) UnregisterAccount(account string) error {
	return s.call("cortana.UnregisterAccount", account)
}

func (s *cortanaToolSync) call(toolName, account string) error {
	account = strings.TrimSpace(account)
	if account == "" {
		return fmt.Errorf("account is required")
	}
	if s == nil || s.bridge == nil || s.cfg == nil {
		return fmt.Errorf("cortana sync is not initialized")
	}
	if strings.TrimSpace(s.cfg.CortanaAgentID) == "" {
		return nil
	}

	result, err := s.bridge.CallTool(
		s.cfg.CortanaAgentID,
		toolName,
		map[string]string{"account": account},
		5*time.Second,
	)
	if err != nil {
		return err
	}

	var payload map[string]any
	if result.Result != "" {
		if unmarshalErr := json.Unmarshal([]byte(result.Result), &payload); unmarshalErr != nil {
			log.Printf("[CortanaSync] tool=%s account=%s result=%s", toolName, account, result.Result)
			return nil
		}
	}

	log.Printf("[CortanaSync] tool=%s account=%s result=%v", toolName, account, payload)
	return nil
}
