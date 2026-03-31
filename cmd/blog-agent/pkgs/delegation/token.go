package delegation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DelegationToken 委托令牌结构
type DelegationToken struct {
	IssuerAgentID   string   `json:"iss"`        // 签发代理 ID: "app-agent"
	AuthorizedUser  string   `json:"sub"`         // 授权用户: "john"
	TargetAccount   string   `json:"aud"`         // 目标账户: "ztt"
	Scope           []string `json:"scope"`       // 权限范围: ["blog:read", "todo:read"]
	IssuedAt        int64    `json:"iat"`         // 签发时间戳
	ExpiresAt       int64    `json:"exp"`         // 过期时间戳
	Nonce           string   `json:"jti"`          // 随机数 (防重放)
	Signature       string   `json:"sig"`          // HMAC-SHA256 签名
}

// TokenClaims 用于签名的内容（不包含签名本身）
type TokenClaims struct {
	IssuerAgentID  string   `json:"iss"`
	AuthorizedUser string   `json:"sub"`
	TargetAccount  string   `json:"aud"`
	Scope          []string `json:"scope"`
	IssuedAt       int64    `json:"iat"`
	ExpiresAt      int64    `json:"exp"`
	Nonce          string   `json:"jti"`
}

// Sign 生成令牌签名
// 使用 HMAC-SHA256 对 TokenClaims 进行签名
func (t *DelegationToken) Sign(secretKey string) error {
	claims := TokenClaims{
		IssuerAgentID:  t.IssuerAgentID,
		AuthorizedUser: t.AuthorizedUser,
		TargetAccount:  t.TargetAccount,
		Scope:          t.Scope,
		IssuedAt:       t.IssuedAt,
		ExpiresAt:      t.ExpiresAt,
		Nonce:          t.Nonce,
	}

	// 将 claims 序列化为 JSON
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("marshal claims: %w", err)
	}

	// 计算 HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write(claimsBytes)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	t.Signature = signature
	return nil
}

// Verify 验证令牌签名
func (t *DelegationToken) Verify(secretKey string) error {
	// 创建临时副本用于验证
	tempToken := &DelegationToken{
		IssuerAgentID:  t.IssuerAgentID,
		AuthorizedUser: t.AuthorizedUser,
		TargetAccount:  t.TargetAccount,
		Scope:          t.Scope,
		IssuedAt:       t.IssuedAt,
		ExpiresAt:      t.ExpiresAt,
		Nonce:          t.Nonce,
		Signature:      t.Signature,
	}

	// 重新计算签名
	if err := tempToken.Sign(secretKey); err != nil {
		return fmt.Errorf("compute signature: %w", err)
	}

	// 比较签名
	if !hmac.Equal([]byte(t.Signature), []byte(tempToken.Signature)) {
		return ErrInvalidSignature
	}

	return nil
}

// IsExpired 检查令牌是否已过期
func (t *DelegationToken) IsExpired() bool {
	return time.Now().Unix() > t.ExpiresAt
}

// IsNotYetValid 检查令牌是否尚未生效
func (t *DelegationToken) IsNotYetValid() bool {
	return time.Now().Unix() < t.IssuedAt
}

// HasScope 检查是否包含指定权限
func (t *DelegationToken) HasScope(scope string) bool {
	for _, s := range t.Scope {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

// GetTargetAccount 获取目标账户
func (t *DelegationToken) GetTargetAccount() string {
	return t.TargetAccount
}

// HasAnyScope 检查是否包含任一指定权限
func (t *DelegationToken) HasAnyScope(scopes ...string) bool {
	for _, scope := range scopes {
		if t.HasScope(scope) {
			return true
		}
	}
	return false
}

// Encode 将令牌编码为 Base64 字符串
func (t *DelegationToken) Encode() (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("marshal token: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// Decode 从 Base64 字符串解码令牌
func Decode(data string) (*DelegationToken, error) {
	// 清理可能的空白字符
	data = strings.TrimSpace(data)

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode error: %v", ErrInvalidToken, err)
	}

	var token DelegationToken
	if err := json.Unmarshal(decoded, &token); err != nil {
		return nil, fmt.Errorf("%w: json unmarshal error: %v", ErrInvalidToken, err)
	}

	return &token, nil
}

// NewDelegationToken 创建新的委托令牌
func NewDelegationToken(issuerAgentID, authorizedUser, targetAccount string, scopes []string, validityDuration time.Duration, nonce string) *DelegationToken {
	now := time.Now()
	return &DelegationToken{
		IssuerAgentID:  issuerAgentID,
		AuthorizedUser: authorizedUser,
		TargetAccount:  targetAccount,
		Scope:          scopes,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(validityDuration).Unix(),
		Nonce:          nonce,
	}
}

// Scopes 权限范围常量
const (
	ScopeBlogRead    = "blog:read"
	ScopeBlogWrite   = "blog:write"
	ScopeTodoRead    = "todo:read"
	ScopeTodoWrite   = "todo:write"
	ScopeYearPlanRead  = "yearplan:read"
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
