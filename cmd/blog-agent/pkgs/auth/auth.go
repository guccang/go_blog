package auth

import (
	log "mylog"
	"net/http"
	db "persistence"
	"time"

	"github.com/google/uuid"
)

// ========== Simple Auth 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

func Info() {
	log.Debug(log.ModuleAuth, "info auth v2.0 (simple)")
}

// Init 初始化 Auth 模块
func Init() {
	db.CleanupExpiredLoginSessions()
}

// genSession 生成新 session
func genSession() string {
	return uuid.New().String()
}

// AddSession 添加 session
func AddSession(account string) string {
	s := genSession()
	if err := db.CreateLoginSession(s, account, time.Now().Add(48*time.Hour)); err != nil {
		log.ErrorF(log.ModuleAuth, "create SQLite session failed: %v", err)
		return ""
	}
	return s
}

// RemoveSession 移除 session
func RemoveSession(account string) int {
	_ = db.DeleteLoginSessions(account)
	return 0
}

// CheckLoginSession 检查登录 session
func CheckLoginSession(session string) int {
	if db.GetLoginSessionAccount(session) == "" {
		return 1
	}
	return 0
}

// GetAccountBySession 根据 session 获取账户
func GetAccountBySession(session string) string {
	return db.GetLoginSessionAccount(session)
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
