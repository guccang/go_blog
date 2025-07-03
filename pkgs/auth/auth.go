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