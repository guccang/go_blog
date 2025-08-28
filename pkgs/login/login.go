package login

import (
	"config"
	"core"
	"module"
	log "mylog"
)

// 登录模块actor
var login_module *LoginActor

func Info() {
	log.Debug("info login v1.0")
}

// 初始化login模块，用于用户登录，短信验证登录，账号密码登录，登出
func Init() {
	login_module = &LoginActor{
		Actor:     core.NewActor(),
		users:     make(map[string]*module.User),
		sms_codes: make(map[string]string),
	}

	login_module.Start(login_module)

	// 管理员账号密码
	admin_account := config.GetAdminAccount()
	admin_pwd := config.GetConfigWithAccount(admin_account, "pwd")
	login_module.users[admin_account] = &module.User{
		Account:  admin_account,
		Password: admin_pwd,
	}
	login_module.sms_codes[admin_account] = "901124"

	// 从sys_accounts博客加载用户数据
	if err := login_module.loadUsersFromAdminBlog(); err != nil {
		log.ErrorF("Failed to load users from admin blog: %v", err)
	}
}

// interface

func Login(account string, password string) (string, int) {
	cmd := &LoginCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		Password: password,
	}
	login_module.Send(cmd)
	session := <-cmd.Response()
	code := <-cmd.Response()
	return session.(string), code.(int)
}

func LoginSMS(account string, verfycode string) (string, int) {
	cmd := &LoginSMSCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Verfycode: verfycode,
	}
	login_module.Send(cmd)
	session := <-cmd.Response()
	code := <-cmd.Response()
	return session.(string), code.(int)
}

func Logout(account string) int {
	cmd := &LoginOutCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	login_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func GenerateSMSCode(account string) (string, int) {
	cmd := &GenerateSMSCodeCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	login_module.Send(cmd)
	code := <-cmd.Response()
	ret := <-cmd.Response()
	return code.(string), ret.(int)
}

// 用户注册接口
func Register(account string, password string) int {
	cmd := &RegisterCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		Password: password,
	}
	login_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func GetPwd(account string) string {
	cmd := &GetPwdCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	login_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(string)
}
