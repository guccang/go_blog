package login

import (
	"auth"
	"config"
	log "mylog"
	db "persistence"
	"sync"
)

// ========== Simple Login 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

var (
	sms_codes map[string]string
	loginMu   sync.RWMutex
)

func Info() {
	log.Debug(log.ModuleLogin, "info login v2.0 (simple)")
}

// Init 初始化 Login 模块
func Init() {
	loginMu.Lock()
	defer loginMu.Unlock()

	sms_codes = make(map[string]string)

	// 管理员账号密码
	admin_account := config.GetAdminAccount()
	admin_pwd := config.GetConfigWithAccount(admin_account, "pwd")
	if admin_pwd != "" && db.GetUser(admin_account) == nil {
		_ = db.SaveUser(admin_account, admin_pwd)
	}
	sms_codes[admin_account] = "901124"
}

// ========== 对外接口 ==========

// Login 账号密码登录
func Login(account string, password string) (string, int) {
	loginMu.Lock()
	defer loginMu.Unlock()

	user := db.GetUser(account)
	if user == nil {
		return "", 1
	}
	if user.Account != account {
		return "", 2
	}
	if user.Password != password {
		return "", 3
	}

	s := auth.AddSession(account)
	sms_codes[account] = "901124"
	return s, 0
}

// VerifyCredentials 校验账号密码，不创建 blog 会话
func VerifyCredentials(account string, password string) int {
	loginMu.RLock()
	defer loginMu.RUnlock()

	user := db.GetUser(account)
	if user == nil {
		return 1
	}
	if user.Account != account {
		return 2
	}
	if user.Password != password {
		return 3
	}
	return 0
}

// LoginSMS 短信验证登录
func LoginSMS(account string, verfycode string) (string, int) {
	loginMu.Lock()
	defer loginMu.Unlock()

	if sms_codes[account] != verfycode {
		return "", 1
	}

	s := auth.AddSession(account)
	log.InfoF(log.ModuleLogin, "LoginSMS account=%s code=%s verfycode=%s", account, sms_codes[account], verfycode)
	return s, 0
}

// Logout 登出
func Logout(account string) int {
	auth.RemoveSession(account)
	return 0
}

// Register 用户注册
func Register(account string, password string) int {
	if account == "" || password == "" {
		return 2
	}

	loginMu.Lock()
	defer loginMu.Unlock()

	if db.GetUser(account) != nil {
		return 1
	}

	if err := db.SaveUser(account, password); err != nil {
		log.ErrorF(log.ModuleLogin, "Failed to save SQLite user: %v", err)
		return 3
	}

	log.InfoF(log.ModuleLogin, "User registered successfully: %s", account)
	return 0
}

// GetPwd 获取密码
func GetPwd(account string) string {
	loginMu.RLock()
	defer loginMu.RUnlock()

	user := db.GetUser(account)
	if user == nil || user.Account != account {
		return ""
	}
	return user.Password
}
