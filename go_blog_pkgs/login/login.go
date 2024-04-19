package login
import (
	log "mylog"
	"module"
	"auth"
	"config"
)

func Info(){
	log.Debug("info login v1.0")
}

var Users = make(map[string]*module.User)

func Init(){
	account := config.GetConfig("admin")
	pwd := config.GetConfig("pwd")
	Users[account] = &module.User{
		Account:account,
		Password:pwd,
	}
}

func Login(account string, password string)(string,int){
	u,ok := Users[account]		
	if !ok {
		return "",1
	}

	if u.Password != password {
		return "",2
	}

	s := auth.AddSession(account)

	return s,0
}

func Logout(account string){
	auth.RemoveSession(account)
}

