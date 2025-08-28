package comment

import (
	"core"
	"module"
	log "mylog"
)

// 评论模块actor
var comment_module *CommentActor

func Info() {
	log.InfoF(log.ModuleComment, "info comment v3.0")
}

// 初始化comment模块，用于评论管理
func Init() {
	comment_module = &CommentActor{
		Actor:    core.NewActor(),
		comments: make(map[string]*AccountCommentData),
	}

	comment_module.userManager = &CommentUserManager{
		AccountData: make(map[string]*CommentAccountData),
	}

	comment_module.Start(comment_module)
}

func LoadComments(account string) {
	cmd := &LoadCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	comment_module.Send(cmd)
	<-cmd.Response()
}

// interface

func AddComment(account, title string, msg string, owner string, pwd string, mail string) int {
	cmd := &AddCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Title:   title,
		Msg:     msg,
		Owner:   owner,
		Pwd:     pwd,
		Mail:    mail,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func AddCommentWithAuth(account, title, msg, sessionID, ip, userAgent string) (int, string) {
	cmd := &AddCommentWithAuthCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Title:     title,
		Msg:       msg,
		SessionID: sessionID,
		IP:        ip,
		UserAgent: userAgent,
	}
	comment_module.Send(cmd)
	ret := <-cmd.Response()
	message := <-cmd.Response()
	return ret.(int), message.(string)
}

func AddAnonymousComment(account, title, msg, username, email, ip, userAgent string) (int, string) {
	cmd := &AddAnonymousCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Title:     title,
		Msg:       msg,
		Username:  username,
		Email:     email,
		IP:        ip,
		UserAgent: userAgent,
	}
	comment_module.Send(cmd)
	ret := <-cmd.Response()
	message := <-cmd.Response()
	return ret.(int), message.(string)
}

func AddCommentWithPassword(account, title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	cmd := &AddCommentWithPasswordCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Title:     title,
		Msg:       msg,
		Username:  username,
		Email:     email,
		Password:  password,
		IP:        ip,
		UserAgent: userAgent,
	}
	comment_module.Send(cmd)
	ret := <-cmd.Response()
	message := <-cmd.Response()
	sessionID := <-cmd.Response()
	return ret.(int), message.(string), sessionID.(string)
}

func ModifyComment(account, title string, msg string, idx int) int {
	cmd := &ModifyCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Title:   title,
		Msg:     msg,
		Idx:     idx,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func RemoveComment(account, title string, idx int) int {
	cmd := &RemoveCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Title:   title,
		Idx:     idx,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func GetComments(account, title string) *module.BlogComments {
	cmd := &GetCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Title:   title,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(*module.BlogComments)
}

func IsUsernameAvailable(account, username string) bool {
	cmd := &IsUsernameAvailableCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		Username: username,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(bool)
}

func ValidateSession(account, sessionID string) (*module.CommentUser, error) {
	cmd := &ValidateSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		SessionID: sessionID,
	}
	comment_module.Send(cmd)
	user := <-cmd.Response()
	err := <-cmd.Response()

	if err != nil {
		return nil, err.(error)
	}
	if user == nil {
		return nil, nil
	}
	return user.(*module.CommentUser), nil
}

func GetAllComments(account string) map[string]*module.BlogComments {
	cmd := &GetAllCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.BlogComments)
}

func GetUsersByUsername(account, username string) []*module.CommentUser {
	cmd := &GetUsersByUsernameCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		Username: username,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.([]*module.CommentUser)
}
