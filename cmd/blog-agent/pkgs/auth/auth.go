package auth

import (
	log "mylog"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

// ========== Simple Auth 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

var (
	sessions map[string]string // account -> session
	authMu   sync.RWMutex
)

func Info() {
	log.Debug(log.ModuleAuth, "info auth v2.0 (simple)")
}

// Init 初始化 Auth 模块
func Init() {
	authMu.Lock()
	defer authMu.Unlock()
	sessions = make(map[string]string)
}

// genSession 生成新 session
func genSession() string {
	return uuid.New().String()
}

// AddSession 添加 session
func AddSession(account string) string {
	authMu.Lock()
	defer authMu.Unlock()

	// 先移除旧 session
	if len(sessions) > 1 {
		delete(sessions, account)
	}

	s := genSession()
	sessions[account] = s
	return s
}

// RemoveSession 移除 session
func RemoveSession(account string) int {
	authMu.Lock()
	defer authMu.Unlock()

	if len(sessions) > 1 {
		delete(sessions, account)
	}
	return 0
}

// CheckLoginSession 检查登录 session
func CheckLoginSession(session string) int {
	authMu.RLock()
	defer authMu.RUnlock()

	for _, s := range sessions {
		if s == session {
			return 0
		}
	}
	return 1
}

// GetAccountBySession 根据 session 获取账户
func GetAccountBySession(session string) string {
	authMu.RLock()
	defer authMu.RUnlock()

	for account, s := range sessions {
		if s == session {
			return account
		}
	}
	return ""
}

// GetSessionFromRequest 从请求获取 session
func GetSessionFromRequest(r *http.Request) string {
	session, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

// GetAccountFromRequest 从请求获取账户
func GetAccountFromRequest(r *http.Request) string {
	return GetAccountBySession(GetSessionFromRequest(r))
}
