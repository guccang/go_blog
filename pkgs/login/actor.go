package login

import (
	"auth"
	"blog"
	"config"
	"core"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"sms"
)

/*
goroutine 线程安全
 goroutine 会被调度到任意一个线程上，因此会被任意一个线程执行接口
 线程安全原因
 原因1: 	actor使用chan通信，chan是线程安全的
 原因2: 	actor的mailbox是线程安全的

 添加一个功能需要的四个步骤:
  第一步: 实现功能逻辑
  第二步: 实现对应的cmd
  第三步: 在login.go中添加对应的接口
  第四步: 在http中添加对应的接口

  上述精炼步骤产生过程:
  1. claudecode 实现版本
  2. 手写实现版本
  3. cursor+gpt5实现版本
  4. 最终综合上述不同实现版本的优点，有了上述的实现步骤.
  5. 最终实现版本 基于cmd的可撤回的actor并发模型,依赖于go的interface特性,简化了实现方式，非常特别的体验
*/

// actor
type LoginActor struct {
	*core.Actor
	users     map[string]*module.User
	sms_codes map[string]string
}

// 短信验证登录,因为只有你一个人登录，所以不需要输入账号
// 返回session，错误码
func (alogin *LoginActor) loginSMS(account string, verfycode string) (string, int) {
	if alogin.sms_codes[account] != verfycode {
		return "", 1
	}

	s := auth.AddSession(account)
	log.InfoF(log.ModuleLogin, "LoginSMS account=%s code=%s verfycode=%s", account, alogin.sms_codes[account], verfycode)
	return s, 0
}

// 产生短信验证码
// 返回验证码，错误码
func (alogin *LoginActor) generateSMSCode(account string) (string, int) {
	code, err := sms.SendSMS()
	if err != nil {
		log.InfoF(log.ModuleLogin, "GenerateSMSCode err=%s", err.Error())
		return "", 1
	}

	alogin.sms_codes[account] = code

	return code, 0
}

// 账号密码登录
// 返回session，错误码
func (alogin *LoginActor) login(account string, password string) (string, int) {
	if _, exists := alogin.users[account]; !exists {
		return "", 1
	}
	if alogin.users[account].Account != account {
		return "", 2
	}
	if alogin.users[account].Password != password {
		return "", 3
	}

	s := auth.AddSession(account)

	alogin.sms_codes[account] = "901124"

	return s, 0
}

// 登出
func (alogin *LoginActor) logout(account string) {
	auth.RemoveSession(account)
}

// 用户注册
// 返回错误码: 0-成功, 1-账号已存在, 2-无效账号或密码, 3-保存失败
func (alogin *LoginActor) register(account string, password string) int {
	if account == "" || password == "" {
		return 2
	}

	if _, exists := alogin.users[account]; exists {
		return 1
	}

	// 添加用户到内存
	alogin.users[account] = &module.User{
		Account:  account,
		Password: password,
	}

	// 保存所有用户账户到管理员博客中
	if err := alogin.saveUsersToAdminBlog(); err != nil {
		log.ErrorF(log.ModuleLogin, "Failed to save users to admin blog: %v", err)
		// 回滚：从内存中删除刚添加的用户
		delete(alogin.users, account)
		return 3
	}

	log.InfoF(log.ModuleLogin, "User registered successfully: %s", account)
	return 0
}

// 保存所有用户账户到管理员博客
func (alogin *LoginActor) saveUsersToAdminBlog() error {
	// 将用户数据转换为JSON格式
	usersJSON, err := json.Marshal(alogin.users)
	if err != nil {
		return err
	}

	// 创建博客数据结构
	udb := &module.UploadedBlogData{
		Title:    "sys_accounts",
		Content:  string(usersJSON),
		AuthType: module.EAuthType_private, // 设为私有，保护用户数据
		Tags:     "sys_accounts",
		Account:  config.GetAdminAccount(), // 使用管理员账户
	}

	// 检查是否已存在
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

// 从管理员博客加载用户账户数据
func (alogin *LoginActor) loadUsersFromAdminBlog() error {
	// 获取sys_accounts博客
	accountsBlog := blog.GetBlogWithAccount(config.GetAdminAccount(), "sys_accounts")
	if accountsBlog == nil {
		log.InfoF(log.ModuleLogin, "No sys_accounts blog found, starting with empty user database")
		return nil
	}

	// 解析JSON数据
	var loadedUsers map[string]*module.User
	if err := json.Unmarshal([]byte(accountsBlog.Content), &loadedUsers); err != nil {
		return fmt.Errorf("failed to parse sys_accounts JSON: %v", err)
	}

	// 加载用户到内存中
	for account, user := range loadedUsers {
		alogin.users[account] = user
		log.DebugF(log.ModuleLogin, "Loaded user: %s", account)
	}

	log.InfoF(log.ModuleLogin, "Successfully loaded %d users from sys_accounts", len(loadedUsers))
	return nil
}

func (alogin *LoginActor) getPwd(account string) string {
	if _, exists := alogin.users[account]; !exists {
		return ""
	}
	if alogin.users[account].Account != account {
		return ""
	}
	return alogin.users[account].Password
}
