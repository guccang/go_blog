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

const maxCortanaVoiceHistoryItems = 500

type CortanaVoiceHistoryItem struct {
	ID              string `json:"id"`
	BroadcastID     string `json:"broadcast_id,omitempty"`
	Text            string `json:"text"`
	FileID          string `json:"file_id"`
	FileName        string `json:"file_name,omitempty"`
	AudioFormat     string `json:"audio_format,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	Expression      string `json:"expression,omitempty"`
	Motion          string `json:"motion,omitempty"`
	StorageProvider string `json:"storage_provider,omitempty"`
	ObjectKey       string `json:"object_key,omitempty"`
	Source          string `json:"source,omitempty"`
}

type CortanaVoiceHistoryStore struct {
	mu      sync.RWMutex
	rootDir string
	cache   map[string][]CortanaVoiceHistoryItem
}

func NewCortanaVoiceHistoryStore(attachmentStoreDir string) *CortanaVoiceHistoryStore {
	return &CortanaVoiceHistoryStore{
		rootDir: filepath.Join(attachmentRootDir(attachmentStoreDir), "_cortana_voice_history"),
		cache:   make(map[string][]CortanaVoiceHistoryItem),
	}
}

func (s *CortanaVoiceHistoryStore) List(userID string, limit int) []CortanaVoiceHistoryItem {
	userID = strings.TrimSpace(userID)
	if s == nil || userID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.loadLocked(userID)
	if err != nil {
		return nil
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]CortanaVoiceHistoryItem, len(items))
	copy(out, items)
	return out
}

func (s *CortanaVoiceHistoryStore) Append(userID string, item CortanaVoiceHistoryItem) error {
	userID = strings.TrimSpace(userID)
	if s == nil || userID == "" {
		return nil
	}
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.FileID) == "" {
		return fmt.Errorf("history item id and file_id are required")
	}
	if item.CreatedAt <= 0 {
		item.CreatedAt = time.Now().UnixMilli()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.loadLocked(userID)
	if err != nil {
		return err
	}

	replaced := false
	for idx := range items {
		if items[idx].ID == item.ID {
			items[idx] = item
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	if len(items) > maxCortanaVoiceHistoryItems {
		items = items[:maxCortanaVoiceHistoryItems]
	}

	if err := s.saveLocked(userID, items); err != nil {
		return err
	}
	s.cache[userID] = items
	return nil
}

func (s *CortanaVoiceHistoryStore) loadLocked(userID string) ([]CortanaVoiceHistoryItem, error) {
	if cached, ok := s.cache[userID]; ok {
		out := make([]CortanaVoiceHistoryItem, len(cached))
		copy(out, cached)
		return out, nil
	}

	path := s.filePath(userID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.cache[userID] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read cortana history: %w", err)
	}
	if len(data) == 0 {
		s.cache[userID] = nil
		return nil, nil
	}

	var items []CortanaVoiceHistoryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decode cortana history: %w", err)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	s.cache[userID] = items
	out := make([]CortanaVoiceHistoryItem, len(items))
	copy(out, items)
	return out, nil
}

func (s *CortanaVoiceHistoryStore) saveLocked(userID string, items []CortanaVoiceHistoryItem) error {
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return fmt.Errorf("mkdir cortana history dir: %w", err)
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cortana history: %w", err)
	}
	path := s.filePath(userID)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write cortana history temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename cortana history temp: %w", err)
	}
	return nil
}

func (s *CortanaVoiceHistoryStore) filePath(userID string) string {
	return filepath.Join(s.rootDir, sanitizeFileName(userID)+".json")
}

func buildCortanaBroadcastObjectKey(owner, broadcastID, fileName string, createdAt time.Time) string {
	safeOwner := sanitizeFileName(firstNonEmpty(strings.TrimSpace(owner), "anonymous"))
	safeBroadcastID := sanitizeFileName(firstNonEmpty(strings.TrimSpace(broadcastID), "broadcast"))
	safeName := sanitizeFileName(firstNonEmpty(strings.TrimSpace(fileName), "cortana.mp3"))
	return fmt.Sprintf(
		"app/cortana-history/%s/%s/%s/%s",
		safeOwner,
		createdAt.Format("20060102"),
		safeBroadcastID,
		safeName,
	)
}
