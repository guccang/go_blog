package sms

import (
	"config"
	"core"
	"fmt"
	log "mylog"
)

// sms actor instance
var sms_actor *SmsActor

func Info() {
	log.Debug("info sms v1.0")
}

// Init initializes the SMS module and starts its actor
func Init() {
	sms_actor = &SmsActor{
		Actor:   core.NewActor(),
		sendURL: config.GetConfigWithAccount(config.GetAdminAccount(), "sms_send_url"),
		name:    config.GetAdminAccount(),
		phone:   config.GetConfigWithAccount(config.GetAdminAccount(), "sms_phone"),
	}
	sms_actor.Start(sms_actor)
}

// SendSMS triggers SMS code generation and sending via the actor.
// It returns the generated verification code and an error (if any).
func SendSMS() (string, error) {
	cmd := &sendSMSCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	sms_actor.Send(cmd)
	code := <-cmd.Response()
	errStr := <-cmd.Response()
	var err error
	if es, ok := errStr.(string); ok && es != "" {
		err = fmt.Errorf(es)
	}
	return code.(string), err
}
