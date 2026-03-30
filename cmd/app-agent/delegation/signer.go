package delegation

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// DelegationToken 委托令牌结构（与 blog-agent 保持一致）
type DelegationToken struct {
	IssuerAgentID   string   `json:"iss"` // 签发代理 ID: "app-agent"
	AuthorizedUser  string   `json:"sub"` // 授权用户: "john"
	TargetAccount   string   `json:"aud"` // 目标账户: "ztt"
	Scope           []string `json:"scope"` // 权限范围
	IssuedAt        int64    `json:"iat"` // 签发时间戳
	ExpiresAt       int64    `json:"exp"` // 过期时间戳
	Nonce           string   `json:"jti"` // 随机数 (防重放)
	Signature       string   `json:"sig"` // HMAC-SHA256 签名
}

// Scopes 权限范围常量
const (
	ScopeBlogRead     = "blog:read"
	ScopeBlogWrite    = "blog:write"
	ScopeTodoRead     = "todo:read"
	ScopeTodoWrite    = "todo:write"
	ScopeYearPlanRead = "yearplan:read"
	ScopeYearPlanWrite = "yearplan:write"
)

// AllScopes 所有权限范围
var AllScopes = []string{
	ScopeBlogRead,
	ScopeBlogWrite,
	ScopeTodoRead,
	ScopeTodoWrite,
	ScopeYearPlanRead,
	ScopeYearPlanWrite,
}

// Signer 委托令牌签发器
type Signer struct {
	issuerAgentID string
	secretKey     string
}

// NewSigner 创建新的签发器
func NewSigner(issuerAgentID, secretKey string) *Signer {
	return &Signer{
		issuerAgentID: issuerAgentID,
		secretKey:     secretKey,
	}
}

// IssueToken 签发委托令牌
// authorizedUser: 授权用户（当前登录用户）
// targetAccount: 目标账户（要访问的账户）
// scopes: 权限范围
// validityDuration: 有效期
func (s *Signer) IssueToken(authorizedUser, targetAccount string, scopes []string, validityDuration time.Duration) (*DelegationToken, error) {
	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	now := time.Now()
	token := &DelegationToken{
		IssuerAgentID:  s.issuerAgentID,
		AuthorizedUser: authorizedUser,
		TargetAccount:  targetAccount,
		Scope:          scopes,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(validityDuration).Unix(),
		Nonce:          nonce,
	}

	// 签名
	if err := token.Sign(s.secretKey); err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}

	return token, nil
}

// generateNonce 生成随机 nonce
func generateNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
