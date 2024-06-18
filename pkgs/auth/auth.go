package auth
import (
	log "mylog"
	"github.com/google/uuid"
)

func Info(){
	log.Debug("info auth v1.0")
}

// Login Session
var Sessions = make([]string,0)

// Page Session
var PageSessions = make(map[string]string)

func AddSession(account string) string{
	RemoveSession(account)
	s := genSession()
	Sessions = append(Sessions,s)
	return s
}

func RemoveSession(account string){
	if len(Sessions) > 1 {
		Sessions = Sessions[1:]
	}
}

func genSession()string{
	return uuid.New().String()
}

// Check login session
func CheckLoginSession(s string) int {
	for _, session := range Sessions {
		if s == session {
			return 0
		}
	}
	return 1
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
