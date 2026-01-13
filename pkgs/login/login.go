package login

import (
	"auth"
	"blog"
	"config"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"sms"
	"sync"
)

// ========== Simple Login 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

var (
	users     map[string]*module.User
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

	users = make(map[string]*module.User)
	sms_codes = make(map[string]string)

	// 管理员账号密码
	admin_account := config.GetAdminAccount()
	admin_pwd := config.GetConfigWithAccount(admin_account, "pwd")
	users[admin_account] = &module.User{
		Account:  admin_account,
		Password: admin_pwd,
	}
	sms_codes[admin_account] = "901124"

	// 从sys_accounts博客加载用户数据
	if err := loadUsersFromAdminBlog(); err != nil {
		log.ErrorF(log.ModuleLogin, "Failed to load users from admin blog: %v", err)
	}
}

// ========== 对外接口 ==========

// Login 账号密码登录
func Login(account string, password string) (string, int) {
	loginMu.Lock()
	defer loginMu.Unlock()

	if _, exists := users[account]; !exists {
		return "", 1
	}
	if users[account].Account != account {
		return "", 2
	}
	if users[account].Password != password {
		return "", 3
	}

	s := auth.AddSession(account)
	sms_codes[account] = "901124"
	return s, 0
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

// GenerateSMSCode 生成短信验证码
func GenerateSMSCode(account string) (string, int) {
	code, err := sms.SendSMS()
	if err != nil {
		log.InfoF(log.ModuleLogin, "GenerateSMSCode err=%s", err.Error())
		return "", 1
	}

	loginMu.Lock()
	sms_codes[account] = code
	loginMu.Unlock()

	return code, 0
}

// Register 用户注册
func Register(account string, password string) int {
	if account == "" || password == "" {
		return 2
	}

	loginMu.Lock()
	defer loginMu.Unlock()

	if _, exists := users[account]; exists {
		return 1
	}

	// 添加用户
	users[account] = &module.User{
		Account:  account,
		Password: password,
	}

	// 保存到博客
	if err := saveUsersToAdminBlog(); err != nil {
		log.ErrorF(log.ModuleLogin, "Failed to save users to admin blog: %v", err)
		delete(users, account)
		return 3
	}

	log.InfoF(log.ModuleLogin, "User registered successfully: %s", account)
	return 0
}

// GetPwd 获取密码
func GetPwd(account string) string {
	loginMu.RLock()
	defer loginMu.RUnlock()

	if _, exists := users[account]; !exists {
		return ""
	}
	if users[account].Account != account {
		return ""
	}
	return users[account].Password
}

// ========== 内部函数 ==========

// saveUsersToAdminBlog 保存用户到管理员博客
func saveUsersToAdminBlog() error {
	usersJSON, err := json.Marshal(users)
	if err != nil {
		return err
	}

	udb := &module.UploadedBlogData{
		Title:    "sys_accounts",
		Content:  string(usersJSON),
		AuthType: module.EAuthType_private,
		Tags:     "sys_accounts",
		Account:  config.GetAdminAccount(),
	}

	existingBlog := blog.GetBlogWithAccount(config.GetAdminAccount(), "sys_accounts")
	var ret int
	if existingBlog == nil {
		ret = blog.AddBlogWithAccount(config.GetAdminAccount(), udb)
	} else {
		ret = blog.ModifyBlogWithAccount(config.GetAdminAccount(), udb)
	}

	if ret != 0 {
		return fmt.Errorf("failed to save users blog, error code: %d", ret)
	}
	return nil
}

// loadUsersFromAdminBlog 从管理员博客加载用户
func loadUsersFromAdminBlog() error {
	accountsBlog := blog.GetBlogWithAccount(config.GetAdminAccount(), "sys_accounts")
	if accountsBlog == nil {
		log.InfoF(log.ModuleLogin, "No sys_accounts blog found, starting with empty user database")
		return nil
	}

	var loadedUsers map[string]*module.User
	if err := json.Unmarshal([]byte(accountsBlog.Content), &loadedUsers); err != nil {
		return fmt.Errorf("failed to parse sys_accounts JSON: %v", err)
	}

	for account, user := range loadedUsers {
		users[account] = user
		log.DebugF(log.ModuleLogin, "Loaded user: %s", account)
	}

	log.InfoF(log.ModuleLogin, "Successfully loaded %d users from sys_accounts", len(loadedUsers))
	return nil
}
