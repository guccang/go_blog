package login

import (
	"core"
)

// cmd

// 短信验证登录cmd
type LoginSMSCmd struct {
	core.ActorCommand
	Account   string
	Verfycode string
}

func (cmd *LoginSMSCmd) Do(actor core.ActorInterface) {
	loginActor := actor.(*LoginActor)
	session, code := loginActor.loginSMS(cmd.Account, cmd.Verfycode)
	cmd.Response() <- session
	cmd.Response() <- code
}

// 账号密码登录cmd
type LoginCmd struct {
	core.ActorCommand
	Account  string
	Password string
}

func (cmd *LoginCmd) Do(actor core.ActorInterface) {
	loginActor := actor.(*LoginActor)
	session, code := loginActor.login(cmd.Account, cmd.Password)
	cmd.Response() <- session
	cmd.Response() <- code
}

// 登出cmd
type LoginOutCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *LoginOutCmd) Do(actor core.ActorInterface) {
	loginActor := actor.(*LoginActor)
	loginActor.logout(cmd.Account)
	cmd.Response() <- 0
}

// 产生短信验证码cmd

type GenerateSMSCodeCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GenerateSMSCodeCmd) Do(actor core.ActorInterface) {
	loginActor := actor.(*LoginActor)
	code, ret := loginActor.generateSMSCode(cmd.Account)
	cmd.Response() <- code
	cmd.Response() <- ret
}

// RegisterCmd for user registration
type RegisterCmd struct {
	core.ActorCommand
	Account  string
	Password string
}

func (cmd *RegisterCmd) Do(actor core.ActorInterface) {
	loginActor := actor.(*LoginActor)
	ret := loginActor.register(cmd.Account, cmd.Password)
	cmd.Response() <- ret
}

// pwd
type GetPwdCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetPwdCmd) Do(actor core.ActorInterface) {
	loginActor := actor.(*LoginActor)
	pwd := loginActor.getPwd(cmd.Account)
	cmd.Response() <- pwd
}
