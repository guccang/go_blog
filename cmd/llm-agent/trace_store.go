package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *SessionStore) requestTraceJSONPath(rootID, sessionID string) string {
	return filepath.Join(s.baseDir, rootID, fmt.Sprintf("trace_%s.json", sessionID))
}

func (s *SessionStore) requestTraceMarkdownPath(rootID, sessionID string) string {
	return filepath.Join(s.baseDir, rootID, fmt.Sprintf("trace_%s.md", sessionID))
}

func (s *SessionStore) SaveRequestTrace(trace *RequestTrace) error {
	if s == nil || trace == nil {
		return nil
	}

	rootID := strings.TrimSpace(trace.RootID)
	if rootID == "" {
		rootID = strings.TrimSpace(trace.SessionID)
	}
	if rootID == "" {
		rootID = strings.TrimSpace(trace.TaskID)
	}
	sessionID := strings.TrimSpace(trace.SessionID)
	if sessionID == "" {
		sessionID = rootID
	}
	if rootID == "" || sessionID == "" {
		return fmt.Errorf("trace missing root/session id")
	}

	trace.RootID = rootID
	trace.SessionID = sessionID
	trace.UpdatedAt = timeNow()

	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()

	dir := filepath.Join(s.baseDir, rootID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create trace dir: %v", err)
	}

	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trace: %v", err)
	}
	if err := os.WriteFile(s.requestTraceJSONPath(rootID, sessionID), data, 0644); err != nil {
		return fmt.Errorf("write trace json: %v", err)
	}

	report := trace.Markdown()
	if err := os.WriteFile(s.requestTraceMarkdownPath(rootID, sessionID), []byte(report), 0644); err != nil {
		return fmt.Errorf("write trace markdown: %v", err)
	}
	return nil
}

func (s *SessionStore) LoadRequestTrace(rootID, sessionID string) (*RequestTrace, error) {
	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()

	data, err := os.ReadFile(s.requestTraceJSONPath(rootID, sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read trace json: %v", err)
	}

	var trace RequestTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, fmt.Errorf("unmarshal trace json: %v", err)
	}
	return &trace, nil
}

// timeNow 便于测试中替换。
var timeNow = func() time.Time {
	return time.Now()
}
