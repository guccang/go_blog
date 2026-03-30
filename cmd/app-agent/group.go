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

const groupRobotDisplayName = "@robot"

type appGroup struct {
	ID           string
	Owner        string
	HumanMembers map[string]bool
	RobotAccount string
	CreatedAt    time.Time
}

type groupInfo struct {
	ID           string   `json:"id"`
	Members      []string `json:"members"`
	CreatedAt    int64    `json:"created_at"`
	RobotAccount string   `json:"robot_account,omitempty"`
}

type groupStoreRecord struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	HumanMembers []string `json:"human_members"`
	RobotAccount string   `json:"robot_account"`
	CreatedAt    int64    `json:"created_at"`
}

type groupStoreFile struct {
	Groups []groupStoreRecord `json:"groups"`
}

type groupManager struct {
	mu        sync.RWMutex
	groups    map[string]*appGroup
	storePath string
}

func newGroupManager(storePath string) *groupManager {
	m := &groupManager{
		groups:    make(map[string]*appGroup),
		storePath: strings.TrimSpace(storePath),
	}
	if err := m.load(); err != nil {
		fmt.Printf("[GroupManager] load failed: %v\n", err)
	}
	return m
}

func (m *groupManager) Create(groupID, owner, robotAccount string) error {
	groupID = normalizeGroupID(groupID)
	owner = strings.TrimSpace(owner)
	robotAccount = strings.TrimSpace(robotAccount)
	if groupID == "" || owner == "" || robotAccount == "" {
		return fmt.Errorf("group_id, user_id and robot_account are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.groups[groupID]; exists {
		return fmt.Errorf("group already exists")
	}
	m.groups[groupID] = &appGroup{
		ID:           groupID,
		Owner:        owner,
		HumanMembers: map[string]bool{owner: true},
		RobotAccount: robotAccount,
		CreatedAt:    time.Now(),
	}
	return m.saveLocked()
}

func (m *groupManager) Join(groupID, userID string) error {
	groupID = normalizeGroupID(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return fmt.Errorf("group_id and user_id are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found")
	}
	group.HumanMembers[userID] = true
	return m.saveLocked()
}

func (m *groupManager) Leave(groupID, userID string) error {
	groupID = normalizeGroupID(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return fmt.Errorf("group_id and user_id are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found")
	}
	delete(group.HumanMembers, userID)
	if len(group.HumanMembers) == 0 {
		delete(m.groups, groupID)
	}
	return m.saveLocked()
}

func (m *groupManager) HasMember(groupID, userID string) bool {
	groupID = normalizeGroupID(groupID)
	userID = strings.TrimSpace(userID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	group := m.groups[groupID]
	if group == nil {
		return false
	}
	return group.HumanMembers[userID] || group.RobotAccount == userID
}

func (m *groupManager) ListForUser(userID string) []groupInfo {
	userID = strings.TrimSpace(userID)
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]groupInfo, 0)
	for _, group := range m.groups {
		if !group.HumanMembers[userID] {
			continue
		}
		result = append(result, groupInfo{
			ID:           group.ID,
			Members:      visibleMembers(group.HumanMembers),
			CreatedAt:    group.CreatedAt.UnixMilli(),
			RobotAccount: group.RobotAccount,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (m *groupManager) VisibleMembers(groupID string) ([]string, error) {
	groupID = normalizeGroupID(groupID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	group := m.groups[groupID]
	if group == nil {
		return nil, fmt.Errorf("group not found")
	}
	return visibleMembers(group.HumanMembers), nil
}

func (m *groupManager) HumanMembers(groupID string) ([]string, error) {
	groupID = normalizeGroupID(groupID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	group := m.groups[groupID]
	if group == nil {
		return nil, fmt.Errorf("group not found")
	}
	return sortedHumanMembers(group.HumanMembers), nil
}

func (m *groupManager) RobotAccount(groupID string) (string, bool) {
	groupID = normalizeGroupID(groupID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	group := m.groups[groupID]
	if group == nil || strings.TrimSpace(group.RobotAccount) == "" {
		return "", false
	}
	return group.RobotAccount, true
}

func (m *groupManager) GroupIDByRobotAccount(robotAccount string) (string, bool) {
	robotAccount = strings.TrimSpace(robotAccount)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, group := range m.groups {
		if group.RobotAccount == robotAccount {
			return group.ID, true
		}
	}
	return "", false
}

func (m *groupManager) load() error {
	if m.storePath == "" {
		return nil
	}
	data, err := os.ReadFile(m.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var store groupStoreFile
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("parse group store: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups = make(map[string]*appGroup, len(store.Groups))
	for _, record := range store.Groups {
		groupID := normalizeGroupID(record.ID)
		if groupID == "" {
			continue
		}
		members := make(map[string]bool)
		for _, member := range record.HumanMembers {
			member = strings.TrimSpace(member)
			if member != "" {
				members[member] = true
			}
		}
		if len(members) == 0 && strings.TrimSpace(record.Owner) != "" {
			members[strings.TrimSpace(record.Owner)] = true
		}
		m.groups[groupID] = &appGroup{
			ID:           groupID,
			Owner:        strings.TrimSpace(record.Owner),
			HumanMembers: members,
			RobotAccount: strings.TrimSpace(record.RobotAccount),
			CreatedAt:    time.UnixMilli(record.CreatedAt),
		}
		if m.groups[groupID].CreatedAt.IsZero() {
			m.groups[groupID].CreatedAt = time.Now()
		}
	}
	return nil
}

func (m *groupManager) saveLocked() error {
	if m.storePath == "" {
		return nil
	}

	records := make([]groupStoreRecord, 0, len(m.groups))
	for _, group := range m.groups {
		records = append(records, groupStoreRecord{
			ID:           group.ID,
			Owner:        group.Owner,
			HumanMembers: sortedHumanMembers(group.HumanMembers),
			RobotAccount: group.RobotAccount,
			CreatedAt:    group.CreatedAt.UnixMilli(),
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})

	data, err := json.MarshalIndent(groupStoreFile{Groups: records}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal group store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0755); err != nil {
		return fmt.Errorf("mkdir group store dir: %w", err)
	}

	tmpPath := m.storePath + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write group store temp: %w", err)
	}
	if err := os.Rename(tmpPath, m.storePath); err != nil {
		return fmt.Errorf("rename group store temp: %w", err)
	}
	return nil
}

func normalizeGroupID(groupID string) string {
	return strings.ToLower(strings.TrimSpace(groupID))
}

func sortedHumanMembers(members map[string]bool) []string {
	result := make([]string, 0, len(members))
	for member := range members {
		result = append(result, member)
	}
	sort.Strings(result)
	return result
}

func visibleMembers(members map[string]bool) []string {
	result := sortedHumanMembers(members)
	result = append(result, groupRobotDisplayName)
	return result
}
