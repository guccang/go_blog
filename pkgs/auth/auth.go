package auth

import (
	"core"
	log "mylog"
	"net/http"
)

func Info() {
	log.Debug(log.ModuleAuth, "info auth v1.0")
}

func Init() {
	auth_actor = &AuthActor{
		Actor:    core.NewActor(),
		sessions: make(map[string]string),
	}
	auth_actor.Start(auth_actor)
}

// auth actor data
var auth_actor *AuthActor

// interface
func AddSession(account string) string {
	cmd := &addSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	auth_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(string)
}

func RemoveSession(account string) int {
	cmd := &removeSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	auth_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func CheckLoginSession(session string) int {
	cmd := &checkLoginSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Session: session,
	}
	auth_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

// GetAccountBySession returns the account bound to a session, or empty if not found
func GetAccountBySession(session string) string {
	cmd := &getAccountBySessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Session: session,
	}
	auth_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(string)
}

// getsession extracts session from request cookie
func GetSessionFromRequest(r *http.Request) string {
	session, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

func GetAccountFromRequest(r *http.Request) string {
	return GetAccountBySession(GetSessionFromRequest(r))
}
