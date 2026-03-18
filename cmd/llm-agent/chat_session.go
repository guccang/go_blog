package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ChatSession 通用聊天会话
type ChatSession struct {
	SessionKey   string    `json:"session_key"`    // "{source}_{userID}"
	Source       string    `json:"source"`         // "wechat" | "web" | "api"
	UserID       string    `json:"user_id"`
	Account      string    `json:"account"`
	SessionID    string    `json:"session_id"`     // 唯一ID（用于持久化文件名）
	Messages     []Message `json:"messages"`
	LastActiveAt time.Time `json:"last_active_at"`
	TurnCount    int       `json:"turn_count"`

	mu         sync.Mutex         `json:"-"` // 保护 Messages 等字段
	processing sync.Mutex         `json:"-"` // 序列化同一用户的消息处理
	cancelMu   sync.Mutex         `json:"-"` // 保护 cancelFunc
	cancelFunc context.CancelFunc `json:"-"` // 当前任务的取消函数
}

// SetCancel 注册当前任务的取消函数
func (s *ChatSession) SetCancel(cancel context.CancelFunc) {
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()
}

// CancelRunning 取消当前正在执行的任务，返回是否有任务在运行
func (s *ChatSession) CancelRunning() bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
		return true
	}
	return false
}

// ChatSessionManager 通用会话管理器
type ChatSessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*ChatSession // sessionKey → session
	timeout     time.Duration
	maxMessages int
	maxTurns    int
	persistDir  string // 持久化目录
}

// NewChatSessionManager 创建通用会话管理器
func NewChatSessionManager(timeout time.Duration, maxMessages, maxTurns int, persistDir string) *ChatSessionManager {
	return &ChatSessionManager{
		sessions:    make(map[string]*ChatSession),
		timeout:     timeout,
		maxMessages: maxMessages,
		maxTurns:    maxTurns,
		persistDir:  persistDir,
	}
}

// sessionKey 构建会话键
func sessionKey(source, userID string) string {
	return source + "_" + userID
}

// GetOrCreate 获取现有会话或创建新会话（超时/超轮次自动重置）
func (m *ChatSessionManager) GetOrCreate(source, userID, account string) (*ChatSession, bool) {
	key := sessionKey(source, userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[key]
	if exists && time.Since(session.LastActiveAt) < m.timeout && session.TurnCount < m.maxTurns {
		return session, false // 复用现有会话
	}

	// 超时、超轮次或不存在 → 创建新会话
	session = &ChatSession{
		SessionKey:   key,
		Source:       source,
		UserID:       userID,
		Account:      account,
		SessionID:    "cs_" + newSessionID(),
		LastActiveAt: time.Now(),
	}
	m.sessions[key] = session
	return session, true // 新会话
}

// Reset 显式重置某用户的会话
func (m *ChatSessionManager) Reset(source, userID string) {
	key := sessionKey(source, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, key)
}

// CleanupExpired 清理所有过期会话
func (m *ChatSessionManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []string
	for key, session := range m.sessions {
		if time.Since(session.LastActiveAt) >= m.timeout {
			expired = append(expired, key)
		}
	}
	for _, key := range expired {
		delete(m.sessions, key)
	}
	if len(expired) > 0 {
		log.Printf("[ChatSession] 清理 %d 个过期会话", len(expired))
	}
}

// SaveSession 持久化单个会话到 JSON 文件
func (m *ChatSessionManager) SaveSession(session *ChatSession) error {
	if m.persistDir == "" {
		return nil
	}

	if err := os.MkdirAll(m.persistDir, 0755); err != nil {
		return fmt.Errorf("create persist dir: %v", err)
	}

	session.mu.Lock()
	data, err := json.MarshalIndent(session, "", "  ")
	session.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal session: %v", err)
	}

	path := filepath.Join(m.persistDir, session.SessionKey+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %v", err)
	}

	return nil
}

// LoadSession 从文件加载单个会话
func (m *ChatSessionManager) LoadSession(sessionKey string) (*ChatSession, error) {
	if m.persistDir == "" {
		return nil, fmt.Errorf("persist dir not configured")
	}

	path := filepath.Join(m.persistDir, sessionKey+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %v", err)
	}

	return &session, nil
}

// LoadAll 启动时加载所有未过期会话
func (m *ChatSessionManager) LoadAll() int {
	if m.persistDir == "" {
		return 0
	}

	entries, err := os.ReadDir(m.persistDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		log.Printf("[ChatSession] 读取持久化目录失败: %v", err)
		return 0
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		key := strings.TrimSuffix(entry.Name(), ".json")
		session, err := m.LoadSession(key)
		if err != nil {
			log.Printf("[ChatSession] 加载会话失败 %s: %v", key, err)
			continue
		}

		// 跳过过期会话
		if time.Since(session.LastActiveAt) >= m.timeout {
			continue
		}

		m.mu.Lock()
		m.sessions[key] = session
		m.mu.Unlock()
		loaded++
	}

	if loaded > 0 {
		log.Printf("[ChatSession] 恢复 %d 个未过期会话", loaded)
	}
	return loaded
}

// CompactMessages 压缩会话消息，防止上下文溢出
// 保留 system prompt + 最近的消息，将旧消息压缩为摘要
func CompactMessages(messages []Message, maxMessages int) []Message {
	// 字符预算检查
	const maxTotalChars = 120000
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	if len(messages) <= maxMessages && totalChars < maxTotalChars {
		return messages
	}

	if len(messages) < 2 {
		return messages
	}

	// 保留 system 消息（messages[0]）
	systemMsg := messages[0]

	// 保留最近 keepCount 条消息
	keepCount := maxMessages * 2 / 3
	if keepCount < 6 {
		keepCount = 6
	}
	if keepCount >= len(messages)-1 {
		return messages
	}

	recentMsgs := messages[len(messages)-keepCount:]
	oldMsgs := messages[1 : len(messages)-keepCount]

	// 构建旧消息摘要
	var summaryParts []string
	for _, msg := range oldMsgs {
		switch msg.Role {
		case "user":
			summaryParts = append(summaryParts, "用户: "+truncate(msg.Content, 100))
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				var toolNames []string
				for _, tc := range msg.ToolCalls {
					toolNames = append(toolNames, tc.Function.Name)
				}
				summaryParts = append(summaryParts, fmt.Sprintf("AI调用工具: %s", strings.Join(toolNames, ", ")))
			} else {
				summaryParts = append(summaryParts, "AI: "+truncate(msg.Content, 150))
			}
		case "tool":
			summaryParts = append(summaryParts, "工具结果: "+truncate(msg.Content, 80))
		}
	}

	compactedMsg := Message{
		Role: "user",
		Content: fmt.Sprintf("[之前的对话摘要（已压缩 %d 条消息）]\n%s",
			len(oldMsgs), strings.Join(summaryParts, "\n")),
	}

	result := make([]Message, 0, 2+len(recentMsgs))
	result = append(result, systemMsg)
	result = append(result, compactedMsg)
	result = append(result, recentMsgs...)
	return result
}
