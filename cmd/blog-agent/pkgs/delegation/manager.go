package delegation

import (
	"fmt"
	"sync"
	"time"
)

// TrustedAgent 可信代理信息
type TrustedAgent struct {
	AgentID   string // 代理 ID，如 "app-agent"
	SecretKey string // 共享密钥
}

// Manager 委托令牌管理器
type Manager struct {
	mu          sync.RWMutex
	trustedAgents map[string]*TrustedAgent // agentID -> agent info
	usedNonces   map[string]time.Time     // nonce -> 使用时间（用于防重放）
	nonceTTL     time.Duration            // nonce 缓存时间
}

// 全局管理器实例
var globalManager *Manager

// InitManager 初始化全局管理器
func InitManager() {
	globalManager = NewManager()
}

// GetManager 获取全局管理器
func GetManager() *Manager {
	if globalManager == nil {
		InitManager()
	}
	return globalManager
}

// NewManager 创建新的管理器
func NewManager() *Manager {
	return &Manager{
		trustedAgents: make(map[string]*TrustedAgent),
		usedNonces:   make(map[string]time.Time),
		nonceTTL:     5 * time.Minute, // Nonce 缓存 5 分钟
	}
}

// RegisterAgent 注册可信代理
func (m *Manager) RegisterAgent(agent *TrustedAgent) error {
	if agent.AgentID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}
	if agent.SecretKey == "" {
		return fmt.Errorf("secret key cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.trustedAgents[agent.AgentID] = agent
	return nil
}

// RegisterTrustedAgents 批量注册可信代理
func (m *Manager) RegisterTrustedAgents(agents []TrustedAgent) {
	for i := range agents {
		m.RegisterAgent(&agents[i])
	}
}

// UnregisterAgent 注销可信代理
func (m *Manager) UnregisterAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.trustedAgents, agentID)
}

// Verify 验证委托令牌
// 验证流程：
// 1. 检查签发者是否可信
// 2. 验证签名
// 3. 检查过期时间
// 4. 检查生效时间
// 5. 检查 Nonce 是否已使用（防重放）
func (m *Manager) Verify(token *DelegationToken) error {
	if token == nil {
		return ErrInvalidToken
	}

	// 1. 检查签发者是否可信
	agent, err := m.getTrustedAgent(token.IssuerAgentID)
	if err != nil {
		return ErrUntrustedIssuer
	}

	// 2. 验证签名
	if err := token.Verify(agent.SecretKey); err != nil {
		return err
	}

	// 3. 检查过期时间
	if token.IsExpired() {
		return ErrTokenExpired
	}

	// 4. 检查生效时间
	if token.IsNotYetValid() {
		return ErrTokenNotYetValid
	}

	// 5. 检查 Nonce 是否已使用（防重放）
	if m.isNonceUsed(token.Nonce) {
		return ErrNonceReused
	}

	// 标记 Nonce 为已使用
	m.markNonceUsed(token.Nonce)

	return nil
}

// VerifyWithScope 验证令牌并检查权限范围
func (m *Manager) VerifyWithScope(token *DelegationToken, requiredScope string) error {
	if err := m.Verify(token); err != nil {
		return err
	}

	if !token.HasScope(requiredScope) {
		return ErrInsufficientScope
	}

	return nil
}

// VerifyWithAnyScope 验证令牌并检查是否具有任一权限
func (m *Manager) VerifyWithAnyScope(token *DelegationToken, requiredScopes ...string) error {
	if err := m.Verify(token); err != nil {
		return err
	}

	if !token.HasAnyScope(requiredScopes...) {
		return ErrInsufficientScope
	}

	return nil
}

// getTrustedAgent 获取可信代理信息
func (m *Manager) getTrustedAgent(agentID string) (*TrustedAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.trustedAgents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return agent, nil
}

// isNonceUsed 检查 Nonce 是否已被使用
func (m *Manager) isNonceUsed(nonce string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if usageTime, exists := m.usedNonces[nonce]; exists {
		// 检查是否过期
		if time.Since(usageTime) > m.nonceTTL {
			return false
		}
		return true
	}
	return false
}

// markNonceUsed 标记 Nonce 已被使用
func (m *Manager) markNonceUsed(nonce string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.usedNonces[nonce] = time.Now()

	// 清理过期的 Nonce
	m.cleanupExpiredNoncesLocked()
}

// cleanupExpiredNonces 清理过期的 Nonce
func (m *Manager) cleanupExpiredNonces() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredNoncesLocked()
}

// cleanupExpiredNoncesLocked 清理过期的 Nonce（内部锁版本）
func (m *Manager) cleanupExpiredNoncesLocked() {
	now := time.Now()
	for nonce, usageTime := range m.usedNonces {
		if now.Sub(usageTime) > m.nonceTTL {
			delete(m.usedNonces, nonce)
		}
	}
}

// StartCleanupRoutine 启动定期清理过期 Nonce 的 goroutine
func (m *Manager) StartCleanupRoutine(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.cleanupExpiredNonces()
		}
	}()
}

// GetTrustedAgentIDs 获取所有可信代理 ID
func (m *Manager) GetTrustedAgentIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.trustedAgents))
	for id := range m.trustedAgents {
		ids = append(ids, id)
	}
	return ids
}

// IsTrustedAgent 检查代理是否可信
func (m *Manager) IsTrustedAgent(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.trustedAgents[agentID]
	return exists
}
