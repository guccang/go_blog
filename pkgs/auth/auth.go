package auth
import (
	log "mylog"
	"github.com/google/uuid"
)

func Info(){
	log.Debug("info auth v1.0")
}

// Login Session
var Sessions = make(map[string]string)
var Sessions_a = make(map[string]string)

// Page Session
var PageSessions = make(map[string]string)

var tmpSession = ""

func GetTmpSession()string{
	return tmpSession
}

func AddSession(account string) string{
	RemoveSession(account)

	s := genSession()
	Sessions[s] = account
	Sessions_a[account] = s
	tmpSession = s
	return s
}

func RemoveSession(account string){
	s,ok := Sessions_a[account]
	if !ok{
		return
	}
	delete(Sessions_a,account)
	delete(Sessions,s)
}

func genSession()string{
	return uuid.New().String()
}

// Check login session
func CheckLoginSession(s string) int {
	_,ok := Sessions[s]
	if !ok {
		return 1
	}

	return 0
}


func AddPageSession()string{
	s := genPageSession()
	PageSessions[s] = s
	return s
}

func genPageSession() string{
	return "pagesession-123456"
}

// Check Ok Remove it
func CheckPageSession(s string) int{

	/*
	_,ok:=PageSessions[s]
	if !ok {
		return 1
	}

	delete(PageSessions,s)
	*/

	return 0
}
