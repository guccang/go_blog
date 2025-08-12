package sms

import (
	"core"
)

// sendSMSCmd asks the actor to send an SMS verification code
// It returns two responses on the channel: code(string), err(string)
type sendSMSCmd struct {
	core.ActorCommand
}

func (cmd *sendSMSCmd) Do(actor core.ActorInterface) {
	smsActor := actor.(*SmsActor)
	code, errStr := smsActor.sendSMS()
	cmd.Response() <- code
	cmd.Response() <- errStr
}
