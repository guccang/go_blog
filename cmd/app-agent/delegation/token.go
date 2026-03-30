package delegation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

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
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	var token DelegationToken
	if err := json.Unmarshal(decoded, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}
