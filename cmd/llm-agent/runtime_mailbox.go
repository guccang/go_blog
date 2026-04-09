package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var sessionStoreFileMu sync.Mutex

// MailboxEntry 是父子任务、恢复流程之间的结构化上下文消息。
type MailboxEntry struct {
	ID              string            `json:"id"`
	RootID          string            `json:"root_id"`
	TargetSessionID string            `json:"target_session_id"`
	SourceSessionID string            `json:"source_session_id,omitempty"`
	Kind            string            `json:"kind"`
	Title           string            `json:"title,omitempty"`
	Content         string            `json:"content"`
	Meta            map[string]string `json:"meta,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// MailboxMessage 保留旧名称，兼容现有调用。
type MailboxMessage = MailboxEntry

func newMailboxEntry(rootID, targetSessionID, sourceSessionID, kind, title, content string, meta map[string]string) MailboxEntry {
	return MailboxEntry{
		ID:              newSessionID(),
		RootID:          strings.TrimSpace(rootID),
		TargetSessionID: strings.TrimSpace(targetSessionID),
		SourceSessionID: strings.TrimSpace(sourceSessionID),
		Kind:            strings.TrimSpace(kind),
		Title:           strings.TrimSpace(title),
		Content:         strings.TrimSpace(content),
		Meta:            cloneStringMap(meta),
		CreatedAt:       time.Now(),
	}
}

func newMailboxMessage(rootID, targetSessionID, sourceSessionID, kind, title, content string, meta map[string]string) MailboxEntry {
	return newMailboxEntry(rootID, targetSessionID, sourceSessionID, kind, title, content, meta)
}

func (s *SessionStore) mailboxPath(rootID, sessionID string) string {
	return filepath.Join(s.baseDir, rootID, fmt.Sprintf("mailbox_%s.json", sessionID))
}

func (s *SessionStore) runtimeStatePath(rootID, sessionID string) string {
	return filepath.Join(s.baseDir, rootID, fmt.Sprintf("runtime_%s.json", sessionID))
}

func (s *SessionStore) loadMailboxLocked(rootID, sessionID string) ([]MailboxEntry, error) {
	path := s.mailboxPath(rootID, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read mailbox file: %v", err)
	}

	var messages []MailboxEntry
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("unmarshal mailbox: %v", err)
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return messages, nil
}

func (s *SessionStore) saveMailboxLocked(rootID, sessionID string, messages []MailboxEntry) error {
	dir := filepath.Join(s.baseDir, rootID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create mailbox dir: %v", err)
	}
	path := s.mailboxPath(rootID, sessionID)
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mailbox: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write mailbox file: %v", err)
	}
	return nil
}

func (s *SessionStore) WriteToMailbox(rootID, sessionID string, msg MailboxEntry) error {
	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()

	messages, err := s.loadMailboxLocked(rootID, sessionID)
	if err != nil {
		return err
	}
	messages = append(messages, msg)
	return s.saveMailboxLocked(rootID, sessionID, messages)
}

func (s *SessionStore) EnqueueMailbox(rootID, sessionID string, msg MailboxEntry) error {
	return s.WriteToMailbox(rootID, sessionID, msg)
}

func (s *SessionStore) PeekMailbox(rootID, sessionID string) ([]MailboxEntry, error) {
	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()
	return s.loadMailboxLocked(rootID, sessionID)
}

func (s *SessionStore) DrainMailbox(rootID, sessionID string) ([]MailboxEntry, error) {
	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()

	messages, err := s.loadMailboxLocked(rootID, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.saveMailboxLocked(rootID, sessionID, nil); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *SessionStore) SaveRuntimeSnapshot(state RuntimeSnapshot) error {
	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()

	dir := filepath.Join(s.baseDir, state.RootID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create runtime dir: %v", err)
	}

	state.UpdatedAt = time.Now()
	path := s.runtimeStatePath(state.RootID, state.SessionID)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write runtime state: %v", err)
	}
	return nil
}

func (s *SessionStore) SaveRuntimeState(state RuntimeSnapshot) error {
	return s.SaveRuntimeSnapshot(state)
}

func (s *SessionStore) LoadRuntimeSnapshot(rootID, sessionID string) (*RuntimeSnapshot, error) {
	sessionStoreFileMu.Lock()
	defer sessionStoreFileMu.Unlock()

	path := s.runtimeStatePath(rootID, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read runtime state: %v", err)
	}

	var state RuntimeSnapshot
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal runtime state: %v", err)
	}
	return &state, nil
}

func (s *SessionStore) LoadRuntimeState(rootID, sessionID string) (*RuntimeSnapshot, error) {
	return s.LoadRuntimeSnapshot(rootID, sessionID)
}
