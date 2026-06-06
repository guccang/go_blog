package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type messageLogEntry struct {
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	Transport string `json:"transport"`
	Event     string `json:"event"`
	UserID    string `json:"user_id,omitempty"`
	Payload   any    `json:"payload,omitempty"`
	Error     string `json:"error,omitempty"`
}

type messageLogger struct {
	path    string
	file    *os.File
	encoder *json.Encoder
	secrets []string
	mu      sync.Mutex
}

func newMessageLogger(path string) (*messageLogger, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("log file path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open message log: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	return &messageLogger{path: path, file: file, encoder: encoder}, nil
}

func (l *messageLogger) log(direction, transport, event, userID string, payload any, err error) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	entry := messageLogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Direction: direction,
		Transport: transport,
		Event:     event,
		UserID:    userID,
		Payload:   redactLogValue(payload, l.secrets),
	}
	if err != nil {
		entry.Error = redactLogString(err.Error(), l.secrets)
	}

	if encodeErr := l.encoder.Encode(entry); encodeErr == nil {
		_ = l.file.Sync()
	}
}

func (l *messageLogger) addSecrets(values ...string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < 4 {
			continue
		}
		found := false
		for _, existing := range l.secrets {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			l.secrets = append(l.secrets, value)
		}
	}
}

func (l *messageLogger) close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func redactLogValue(value any, secrets []string) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return value
	}
	return redactDecodedValue(decoded, secrets)
}

func redactDecodedValue(value any, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveLogKey(key) {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = redactDecodedValue(item, secrets)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = redactDecodedValue(item, secrets)
		}
		return result
	case string:
		return redactLogString(typed, secrets)
	default:
		return value
	}
}

func redactLogString(value string, secrets []string) string {
	for _, secret := range secrets {
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	return value
}

func isSensitiveLogKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "password", "token", "receive_token", "session_token", "access_token",
		"refresh_token", "delegation_token", "authorization", "x-app-agent-token",
		"x-app-agent-session":
		return true
	default:
		return strings.HasSuffix(normalized, "_password") ||
			strings.HasSuffix(normalized, "_secret") ||
			strings.HasSuffix(normalized, "_token")
	}
}
