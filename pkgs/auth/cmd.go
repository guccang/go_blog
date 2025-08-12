package auth

import (
	"core"
)

// cmd
type addSessionCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *addSessionCmd) Do(actor core.ActorInterface) {
	authActor := actor.(*AuthActor)
	s := authActor.addSession(cmd.Account)
	cmd.Response() <- s
}

type removeSessionCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *removeSessionCmd) Do(actor core.ActorInterface) {
	authActor := actor.(*AuthActor)
	authActor.removeSession(cmd.Account)
	cmd.Response() <- 0
}

type checkLoginSessionCmd struct {
	core.ActorCommand
	Session string
}

func (cmd *checkLoginSessionCmd) Do(actor core.ActorInterface) {
	authActor := actor.(*AuthActor)
	cmd.Response() <- authActor.checkLoginSession(cmd.Session)
}
