package auth

import (
	"core"

	"github.com/google/uuid"
)

type AuthActor struct {
	*core.Actor
	sessions map[string]string
}

func (actor *AuthActor) addSession(account string) string {
	actor.removeSession(account)
	s := actor.genSession()
	actor.sessions[account] = s
	return s
}

func (actor *AuthActor) removeSession(account string) int {
	if len(actor.sessions) > 1 {
		delete(actor.sessions, account)
	}
	return 0
}

func (actor *AuthActor) genSession() string {
	return uuid.New().String()
}

// Check login session
func (actor *AuthActor) checkLoginSession(s string) int {
	for _, session := range actor.sessions {
		if s == session {
			return 0
		}
	}
	return 1
}

// get account by session
func (actor *AuthActor) getAccountBySession(s string) string {
	for account, session := range actor.sessions {
		if s == session {
			return account
		}
	}
	return ""
}
