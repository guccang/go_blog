package downloadticket

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrDisabled = errors.New("download ticket signer is disabled")
	ErrInvalid  = errors.New("invalid download ticket")
	ErrExpired  = errors.New("download ticket expired")
)

type Claims struct {
	FileID          string `json:"file_id"`
	UserID          string `json:"user_id"`
	ObjectKey       string `json:"object_key"`
	StorageProvider string `json:"storage_provider"`
	ExpiresAt       int64  `json:"expires_at"`
	Nonce           string `json:"nonce"`
}

type Input struct {
	FileID          string
	UserID          string
	ObjectKey       string
	StorageProvider string
}

type Signer struct {
	secret []byte
	now    func() time.Time
}

func NewSigner(secret string) *Signer {
	return NewSignerWithClock(secret, time.Now)
}

func NewSignerWithClock(secret string, now func() time.Time) *Signer {
	if now == nil {
		now = time.Now
	}
	return &Signer{
		secret: []byte(strings.TrimSpace(secret)),
		now:    now,
	}
}

func (s *Signer) Enabled() bool {
	return s != nil && len(s.secret) > 0
}

func (s *Signer) Issue(input Input, ttl time.Duration) (string, *Claims, error) {
	if !s.Enabled() {
		return "", nil, ErrDisabled
	}
	if ttl <= 0 {
		return "", nil, fmt.Errorf("invalid ticket ttl: %s", ttl)
	}
	claims := &Claims{
		FileID:          strings.TrimSpace(input.FileID),
		UserID:          strings.TrimSpace(input.UserID),
		ObjectKey:       strings.TrimSpace(input.ObjectKey),
		StorageProvider: strings.TrimSpace(input.StorageProvider),
		ExpiresAt:       s.now().Add(ttl).UnixMilli(),
		Nonce:           randomNonce(),
	}
	if claims.FileID == "" || claims.UserID == "" || claims.ObjectKey == "" {
		return "", nil, fmt.Errorf("file_id, user_id and object_key are required")
	}
	if claims.StorageProvider == "" {
		claims.StorageProvider = "obs"
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", nil, fmt.Errorf("marshal ticket claims: %w", err)
	}
	signature := s.sign(payload)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(signature)
	return token, claims, nil
}

func (s *Signer) Verify(token string) (*Claims, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalid
	}
	expected := s.sign(payload)
	if !hmac.Equal(signature, expected) {
		return nil, ErrInvalid
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalid
	}
	if strings.TrimSpace(claims.FileID) == "" ||
		strings.TrimSpace(claims.UserID) == "" ||
		strings.TrimSpace(claims.ObjectKey) == "" {
		return nil, ErrInvalid
	}
	if claims.ExpiresAt <= s.now().UnixMilli() {
		return nil, ErrExpired
	}
	if strings.TrimSpace(claims.StorageProvider) == "" {
		claims.StorageProvider = "obs"
	}
	return &claims, nil
}

func (s *Signer) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func randomNonce() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
