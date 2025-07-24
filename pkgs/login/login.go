package login

import (
	"auth"
	"config"
	"module"
	log "mylog"
	"sms"
)

func Info() {
	log.Debug("info login v1.0")
}

var Users = make(map[string]*module.User)
var codes = make(map[string]string)

func Init() {
	account := config.GetConfig("admin")
	pwd := config.GetConfig("pwd")
	Users[account] = &module.User{
		Account:  account,
		Password: pwd,
	}
	codes[account] = "901124"
}

// 短信验证登录,因为只有你一个人登录，所以不需要输入账号
func LoginSMS(verfycode string) (string, int) {
	account := config.GetConfig("admin")
	code, ok := codes[account]
	if !ok {
		return "", 1
	}

	if code != verfycode {
		return "", 2
	}

	s := auth.AddSession(account)
	log.InfoF("LoginSMS account=%s code=%s verfycode=%s", account, code, verfycode)
	return s, 0
}

// 产生短信
func GenerateSMSCode() (string, int) {
	code, err := sms.SendSMS()
	if err != nil {
		log.InfoF("GenerateSMSCode err=%s",err.Error())
		return "", 1
	}

	account := config.GetConfig("admin")
	codes[account] = code

	return code, 0
}

// 账号密码登录
func Login(account string, password string) (string, int) {
	u, ok := Users[account]
	if !ok {
		return "", 1
	}

	if u.Password != password {
		return "", 2
	}

	s := auth.AddSession(account)

	return s, 0
}

func Logout(account string) {
	auth.RemoveSession(account)
}
