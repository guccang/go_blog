package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type cortanaProfileSnapshot struct {
	Name        string `json:"name"`
	OwnerTitle  string `json:"owner_title,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedAt   int64  `json:"updated_at"`
}

func cortanaProfilePath(baseDir, account string) string {
	baseDir = strings.TrimSpace(baseDir)
	account = strings.TrimSpace(account)
	if baseDir == "" || account == "" {
		return ""
	}
	return filepath.Join(baseDir, "users", account, "memory", "cortana_profile.json")
}

func syncCortanaProfileToWorkspace(baseDir, account string, settings CortanaSettings) error {
	path := cortanaProfilePath(baseDir, account)
	if path == "" {
		return nil
	}

	payload := cortanaProfileSnapshot{
		Name:        strings.TrimSpace(settings.PersonaName),
		OwnerTitle:  strings.TrimSpace(settings.OwnerTitle),
		Description: strings.TrimSpace(settings.PersonaDescription),
		UpdatedAt:   settings.UpdatedAt,
	}
	if payload.Name == "" {
		payload.Name = DefaultCortanaPersonaName
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
