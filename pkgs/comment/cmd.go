package comment

import (
	"core"
)

// cmd

// 加载评论数据
type LoadCommentsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *LoadCommentsCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	commentActor.LoadComments(cmd.Account)
	cmd.Response() <- 0
}

// 添加评论cmd
type AddCommentCmd struct {
	core.ActorCommand
	Account string
	Title   string
	Msg     string
	Owner   string
	Pwd     string
	Mail    string
}

func (cmd *AddCommentCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	result := commentActor.addComment(cmd.Account, cmd.Title, cmd.Msg, cmd.Owner, cmd.Pwd, cmd.Mail)
	cmd.Response() <- result
}

// 添加带身份验证的评论cmd
type AddCommentWithAuthCmd struct {
	core.ActorCommand
	Account   string
	Title     string
	Msg       string
	SessionID string
	IP        string
	UserAgent string
}

func (cmd *AddCommentWithAuthCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	ret, message := commentActor.addCommentWithAuth(cmd.Account, cmd.Title, cmd.Msg, cmd.SessionID, cmd.IP, cmd.UserAgent)
	cmd.Response() <- ret
	cmd.Response() <- message
}

// 添加匿名评论cmd
type AddAnonymousCommentCmd struct {
	core.ActorCommand
	Account   string
	Title     string
	Msg       string
	Username  string
	Email     string
	IP        string
	UserAgent string
}

func (cmd *AddAnonymousCommentCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	ret, message := commentActor.addAnonymousComment(cmd.Account, cmd.Title, cmd.Msg, cmd.Username, cmd.Email, cmd.IP, cmd.UserAgent)
	cmd.Response() <- ret
	cmd.Response() <- message
}

// 添加带密码验证的评论cmd
type AddCommentWithPasswordCmd struct {
	core.ActorCommand
	Account   string
	Title     string
	Msg       string
	Username  string
	Email     string
	Password  string
	IP        string
	UserAgent string
}

func (cmd *AddCommentWithPasswordCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	ret, message, sessionID := commentActor.addCommentWithPassword(cmd.Account, cmd.Title, cmd.Msg, cmd.Username, cmd.Email, cmd.Password, cmd.IP, cmd.UserAgent)
	cmd.Response() <- ret
	cmd.Response() <- message
	cmd.Response() <- sessionID
}

// 修改评论cmd
type ModifyCommentCmd struct {
	core.ActorCommand
	Account string
	Title   string
	Msg     string
	Idx     int
}

func (cmd *ModifyCommentCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	result := commentActor.modifyComment(cmd.Account, cmd.Title, cmd.Msg, cmd.Idx)
	cmd.Response() <- result
}

// 删除评论cmd
type RemoveCommentCmd struct {
	core.ActorCommand
	Account string
	Title   string
	Idx     int
}

func (cmd *RemoveCommentCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	result := commentActor.removeComment(cmd.Account, cmd.Title, cmd.Idx)
	cmd.Response() <- result
}

// 获取评论cmd
type GetCommentsCmd struct {
	core.ActorCommand
	Account string
	Title   string
}

func (cmd *GetCommentsCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	comments := commentActor.getComments(cmd.Account, cmd.Title)
	cmd.Response() <- comments
}

// 验证用户名可用性cmd
type IsUsernameAvailableCmd struct {
	core.ActorCommand
	Account  string
	Username string
}

func (cmd *IsUsernameAvailableCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	available := commentActor.isUsernameAvailable(cmd.Account, cmd.Username)
	cmd.Response() <- available
}

// 验证会话cmd
type ValidateSessionCmd struct {
	core.ActorCommand
	Account   string
	SessionID string
}

func (cmd *ValidateSessionCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	user, err := commentActor.validateSession(cmd.Account, cmd.SessionID)
	if err != nil {
		cmd.Response() <- nil
		cmd.Response() <- err
	} else {
		cmd.Response() <- user
		cmd.Response() <- nil
	}
}

// 获取所有评论cmd
type GetAllCommentsCmd struct {
	core.ActorCommand
	Account string
	Title   string
}

func (cmd *GetAllCommentsCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	cmd.Response() <- commentActor.getComments(cmd.Account, cmd.Title)
}

// 通过用户名获取用户cmd
type GetUsersByUsernameCmd struct {
	core.ActorCommand
	Account  string
	Username string
}

func (cmd *GetUsersByUsernameCmd) Do(actor core.ActorInterface) {
	commentActor := actor.(*CommentActor)
	users := commentActor.getUsersByUsername(cmd.Account, cmd.Username)
	cmd.Response() <- users
}
