package comment

import (
	"core"
	"fmt"
	"module"
	log "mylog"
	db "persistence"
)

// 评论模块actor
var comment_module *CommentActor

func Info() {
	fmt.Println("info comment v3.0")
}

// 初始化comment模块，用于评论管理
func Init() {
	comment_module = &CommentActor{
		Actor:    core.NewActor(),
		comments: make(map[string]*module.BlogComments),
	}

	// 初始化用户管理器
	comment_module.initUserManager()

	// 加载评论数据
	all_datas := db.GetAllBlogComments()
	if all_datas != nil {
		for _, c := range all_datas {
			comment_module.comments[c.Title] = c
		}
	}
	log.DebugF("getComments number=%d", len(comment_module.comments))

	comment_module.Start(comment_module)
}


// interface

func AddComment(title string, msg string, owner string, pwd string, mail string) int {
	cmd := &AddCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
		Msg:   msg,
		Owner: owner,
		Pwd:   pwd,
		Mail:  mail,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func AddCommentWithAuth(title, msg, sessionID, ip, userAgent string) (int, string) {
	cmd := &AddCommentWithAuthCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
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

func AddAnonymousComment(title, msg, username, email, ip, userAgent string) (int, string) {
	cmd := &AddAnonymousCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
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

func AddCommentWithPassword(title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	cmd := &AddCommentWithPasswordCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
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

func ModifyComment(title string, msg string, idx int) int {
	cmd := &ModifyCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
		Msg:   msg,
		Idx:   idx,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func RemoveComment(title string, idx int) int {
	cmd := &RemoveCommentCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
		Idx:   idx,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func GetComments(title string) *module.BlogComments {
	cmd := &GetCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(*module.BlogComments)
}

func IsUsernameAvailable(username string) bool {
	cmd := &IsUsernameAvailableCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Username: username,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	return result.(bool)
}

func ValidateSession(sessionID string) (*module.CommentUser, error) {
	cmd := &ValidateSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
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

func GetAllComments() map[string]*module.BlogComments {
	cmd := &GetAllCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.BlogComments)
}

func GetUsersByUsername(username string) []*module.CommentUser {
	cmd := &GetUsersByUsernameCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Username: username,
	}
	comment_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.([]*module.CommentUser)
}
