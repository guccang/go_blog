package main

import (
	"app-agent/delegation"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginRequest struct {
	UserID   string `json:"user_id"`
	Password string `json:"password"`
}

type refreshRequest struct {
	UserID       string `json:"user_id"`
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	UserID       string `json:"user_id,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type loginResponse struct {
	Success         bool   `json:"success"`
	SessionToken    string `json:"session_token,omitempty"`
	AccessToken     string `json:"access_token,omitempty"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
	ExpiresIn       int64  `json:"expires_in,omitempty"`
	TokenType       string `json:"token_type,omitempty"`
	ObsAgentBaseURL string `json:"obs_agent_base_url,omitempty"`
	Error           string `json:"error,omitempty"`
}

type authError struct {
	Code    string
	Message string
}

func (e *authError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type appSession struct {
	Account         string
	Token           string
	ExpiresAt       time.Time
	DelegationToken string // delegation token for blog-agent API calls
}

type refreshGrant struct {
	Account   string
	ExpiresAt time.Time
}

type issuedAuthSession struct {
	Session      *appSession
	RefreshToken string
}

type authManager struct {
	cfg              *Config
	client           *http.Client
	mu               sync.RWMutex
	sessions         map[string]*appSession
	refreshTokens    map[string]*refreshGrant
	delegationSigner *delegation.Signer // for issuing delegation tokens
}

func newAuthManager(cfg *Config) *authManager {
	signer := delegation.NewSigner("app-agent", cfg.DelegationSecretKey)
	return &authManager{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		sessions:         make(map[string]*appSession),
		refreshTokens:    make(map[string]*refreshGrant),
		delegationSigner: signer,
	}
}

func (m *authManager) Login(userID, password string) (*issuedAuthSession, error) {
	userID = strings.TrimSpace(userID)
	password = strings.TrimSpace(password)
	if userID == "" || password == "" {
		return nil, fmt.Errorf("user_id and password are required")
	}
	if err := m.verifyAgainstBlogAgent(userID, password); err != nil {
		return nil, err
	}

	return m.issueAuthSession(userID)
}

func (m *authManager) Refresh(userID, refreshToken string) (*issuedAuthSession, error) {
	userID = strings.TrimSpace(userID)
	refreshToken = strings.TrimSpace(refreshToken)
	if userID == "" || refreshToken == "" {
		return nil, &authError{
			Code:    "invalid_refresh_token",
			Message: "user_id and refresh_token are required",
		}
	}

	refreshHash := hashToken(refreshToken)
	now := time.Now()

	m.mu.RLock()
	grant := m.refreshTokens[refreshHash]
	m.mu.RUnlock()

	if grant == nil || grant.Account != userID {
		return nil, &authError{
			Code:    "invalid_refresh_token",
			Message: "refresh token is invalid",
		}
	}
	if now.After(grant.ExpiresAt) {
		m.mu.Lock()
		delete(m.refreshTokens, refreshHash)
		m.mu.Unlock()
		return nil, &authError{
			Code:    "invalid_refresh_token",
			Message: "refresh token expired",
		}
	}

	return m.issueAuthSession(userID)
}

func (m *authManager) Logout(accessToken, refreshToken, userID string) {
	accessToken = strings.TrimSpace(accessToken)
	refreshToken = strings.TrimSpace(refreshToken)
	userID = strings.TrimSpace(userID)

	m.mu.Lock()
	defer m.mu.Unlock()

	if userID != "" {
		m.revokeUserLocked(userID)
		return
	}
	if accessToken != "" {
		if session := m.sessions[accessToken]; session != nil {
			m.revokeUserLocked(session.Account)
			return
		}
	}
	if refreshToken != "" {
		if grant := m.refreshTokens[hashToken(refreshToken)]; grant != nil {
			m.revokeUserLocked(grant.Account)
		}
	}
}

func (m *authManager) issueAuthSession(userID string) (*issuedAuthSession, error) {
	accessToken, err := newOpaqueToken("access token")
	if err != nil {
		return nil, err
	}
	refreshToken, err := newOpaqueToken("refresh token")
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session := &appSession{
		Account:   userID,
		Token:     accessToken,
		ExpiresAt: now.Add(time.Duration(m.cfg.AppSessionTTLMinutes) * time.Minute),
	}

	// Issue delegation token for blog-agent API calls
	delegationToken, err := m.issueDelegationToken(userID, userID, delegation.AllScopes)
	if err != nil {
		// Log error but don't fail login
		fmt.Printf("Warning: failed to issue delegation token: %v\n", err)
	} else {
		session.DelegationToken = delegationToken
	}

	refreshHash := hashToken(refreshToken)
	refreshGrant := &refreshGrant{
		Account:   userID,
		ExpiresAt: now.Add(time.Duration(m.cfg.AppRefreshTokenTTLHours) * time.Hour),
	}

	m.mu.Lock()
	m.revokeUserLocked(userID)
	m.sessions[accessToken] = session
	m.refreshTokens[refreshHash] = refreshGrant
	m.mu.Unlock()

	return &issuedAuthSession{
		Session:      session,
		RefreshToken: refreshToken,
	}, nil
}

// issueDelegationToken 签发委托令牌
func (m *authManager) issueDelegationToken(authorizedUser, targetAccount string, scopes []string) (string, error) {
	if m.delegationSigner == nil {
		return "", fmt.Errorf("delegation signer not initialized")
	}

	// 默认令牌有效期为 session 有效期
	validityDuration := time.Duration(m.cfg.AppSessionTTLMinutes) * time.Minute

	token, err := m.delegationSigner.IssueToken(authorizedUser, targetAccount, scopes, validityDuration)
	if err != nil {
		return "", err
	}

	return token.Encode()
}

// GetDelegationToken 获取用户的 delegation token
func (m *authManager) GetDelegationToken(sessionToken string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session := m.sessions[sessionToken]
	if session == nil {
		return "", fmt.Errorf("session not found")
	}
	if time.Now().After(session.ExpiresAt) {
		return "", fmt.Errorf("session expired")
	}
	return session.DelegationToken, nil
}

func (m *authManager) Validate(token, userID string) bool {
	token = strings.TrimSpace(token)
	userID = strings.TrimSpace(userID)
	if token == "" || userID == "" {
		return false
	}

	m.mu.RLock()
	session := m.sessions[token]
	m.mu.RUnlock()
	if session == nil {
		return false
	}
	if session.Account != userID {
		return false
	}
	if time.Now().After(session.ExpiresAt) {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		return false
	}
	return true
}

func (m *authManager) CleanupExpired() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, token)
		}
	}
	for tokenHash, grant := range m.refreshTokens {
		if now.After(grant.ExpiresAt) {
			delete(m.refreshTokens, tokenHash)
		}
	}
}

func (m *authManager) revokeUserLocked(userID string) {
	for existingToken, existing := range m.sessions {
		if existing.Account == userID {
			delete(m.sessions, existingToken)
		}
	}
	for refreshHash, grant := range m.refreshTokens {
		if grant.Account == userID {
			delete(m.refreshTokens, refreshHash)
		}
	}
}

func (m *authManager) verifyAgainstBlogAgent(userID, password string) error {
	reqBody, err := json.Marshal(map[string]string{
		"account":  userID,
		"password": password,
	})
	if err != nil {
		return fmt.Errorf("marshal verify request: %w", err)
	}

	url := strings.TrimRight(strings.TrimSpace(m.cfg.BlogAgentBaseURL), "/") + "/api/app-auth/login"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build verify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return &authError{
			Code:    "blog_agent_unreachable",
			Message: fmt.Sprintf("blog-agent unreachable: %v", err),
		}
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return &authError{
				Code:    "blog_agent_api_missing",
				Message: "blog-agent app auth api not found",
			}
		}
		return &authError{
			Code:    "blog_agent_bad_response",
			Message: fmt.Sprintf("decode verify response: %v", err),
		}
	}
	if resp.StatusCode == http.StatusNotFound {
		return &authError{
			Code:    "blog_agent_api_missing",
			Message: "blog-agent app auth api not found",
		}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if strings.TrimSpace(result.Error) == "" {
			result.Error = "invalid account or password"
		}
		return &authError{
			Code:    "invalid_credentials",
			Message: result.Error,
		}
	}
	if resp.StatusCode != http.StatusOK {
		return &authError{
			Code:    "blog_agent_bad_status",
			Message: fmt.Sprintf("blog-agent verify failed: http %d", resp.StatusCode),
		}
	}
	if !result.Success {
		msg := strings.TrimSpace(result.Error)
		if msg == "" {
			msg = "invalid account or password"
		}
		return &authError{
			Code:    "invalid_credentials",
			Message: msg,
		}
	}
	return nil
}

func (m *authManager) EnsureGroupRobotAccount(groupID string) (string, error) {
	groupID = normalizeGroupID(groupID)
	if groupID == "" {
		return "", fmt.Errorf("group_id is required")
	}

	account := groupRobotAccountName(groupID)
	password := groupRobotPassword(groupID)

	reqBody, err := json.Marshal(map[string]string{
		"account":  account,
		"password": password,
	})
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}

	url := strings.TrimRight(strings.TrimSpace(m.cfg.BlogAgentBaseURL), "/") + "/api/app-auth/register"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("build register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return "", &authError{
			Code:    "blog_agent_unreachable",
			Message: fmt.Sprintf("blog-agent unreachable: %v", err),
		}
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
		Account string `json:"account,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return "", &authError{
				Code:    "blog_agent_api_missing",
				Message: "blog-agent app auth register api not found",
			}
		}
		return "", &authError{
			Code:    "blog_agent_bad_response",
			Message: fmt.Sprintf("decode register response: %v", err),
		}
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", &authError{
			Code:    "blog_agent_api_missing",
			Message: "blog-agent app auth register api not found",
		}
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(result.Error)
		if msg == "" {
			msg = fmt.Sprintf("blog-agent register failed: http %d", resp.StatusCode)
		}
		return "", &authError{
			Code:    "blog_agent_bad_status",
			Message: msg,
		}
	}
	if !result.Success {
		msg := strings.TrimSpace(result.Error)
		if msg == "" {
			msg = "blog-agent register failed"
		}
		return "", &authError{
			Code:    "blog_agent_bad_status",
			Message: msg,
		}
	}

	if strings.TrimSpace(result.Account) != "" {
		account = strings.TrimSpace(result.Account)
	}
	return account, nil
}

func groupRobotAccountName(groupID string) string {
	groupID = normalizeGroupID(groupID)
	sanitized := strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(groupID)
	if sanitized == "" {
		sanitized = "group"
	}
	return "group_" + sanitized + "_robot"
}

func groupRobotPassword(groupID string) string {
	sum := sha256.Sum256([]byte("app-group-robot::" + normalizeGroupID(groupID)))
	return hex.EncodeToString(sum[:16])
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newOpaqueToken(label string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate %s: %w", label, err)
	}
	return hex.EncodeToString(buf), nil
}
