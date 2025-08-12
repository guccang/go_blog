package login

import (
	"auth"
	"core"
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
	log.InfoF("LoginSMS account=%s code=%s verfycode=%s", account, alogin.sms_codes[account], verfycode)
	return s, 0
}

// 产生短信验证码
// 返回验证码，错误码
func (alogin *LoginActor) generateSMSCode(account string) (string, int) {
	code, err := sms.SendSMS()
	if err != nil {
		log.InfoF("GenerateSMSCode err=%s", err.Error())
		return "", 1
	}

	alogin.sms_codes[account] = code

	return code, 0
}

// 账号密码登录
// 返回session，错误码
func (alogin *LoginActor) login(account string, password string) (string, int) {
	if alogin.users[account].Account != account {
		return "", 1
	}
	if alogin.users[account].Password != password {
		return "", 2
	}

	s := auth.AddSession(account)

	return s, 0
}

// 登出
func (alogin *LoginActor) logout(account string) {
	auth.RemoveSession(account)
}
