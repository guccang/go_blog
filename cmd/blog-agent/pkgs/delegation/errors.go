package delegation

import "fmt"

// 错误类型定义
var (
	// ErrUntrustedIssuer 签发者不在可信列表中
	ErrUntrustedIssuer = &DelegationError{Code: "UNTRUSTED_ISSUER", Message: "delegation issuer is not trusted"}

	// ErrInvalidSignature 签名验证失败
	ErrInvalidSignature = &DelegationError{Code: "INVALID_SIGNATURE", Message: "delegation token signature verification failed"}

	// ErrTokenExpired 令牌已过期
	ErrTokenExpired = &DelegationError{Code: "TOKEN_EXPIRED", Message: "delegation token has expired"}

	// ErrNonceReused Nonce 已被使用（重放攻击）
	ErrNonceReused = &DelegationError{Code: "NONCE_REUSED", Message: "delegation token nonce has been reused"}

	// ErrInvalidToken 令牌格式无效
	ErrInvalidToken = &DelegationError{Code: "INVALID_TOKEN", Message: "delegation token format is invalid"}

	// ErrInsufficientScope 权限范围不足
	ErrInsufficientScope = &DelegationError{Code: "INSUFFICIENT_SCOPE", Message: "delegation token does not have required scope"}

	// ErrTokenNotYetValid 令牌尚未生效
	ErrTokenNotYetValid = &DelegationError{Code: "TOKEN_NOT_YET_VALID", Message: "delegation token is not yet valid"}
)

// DelegationError 委托令牌验证错误
type DelegationError struct {
	Code    string
	Message string
}

func (e *DelegationError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// IsDelegationError 检查错误是否为 DelegationError
func IsDelegationError(err error) bool {
	_, ok := err.(*DelegationError)
	return ok
}

// GetDelegationError 获取错误码
func GetDelegationErrorCode(err error) string {
	if de, ok := err.(*DelegationError); ok {
		return de.Code
	}
	return ""
}

// NewDelegationError 创建新的DelegationError
func NewDelegationError(code, message string) *DelegationError {
	return &DelegationError{Code: code, Message: message}
}
