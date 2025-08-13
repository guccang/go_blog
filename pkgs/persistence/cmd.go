package persistence

import (
	"core"
	"module"
)

// cmd

// 保存博客cmd
type SaveBlogCmd struct {
	core.ActorCommand
	Blog *module.Blog
}

func (cmd *SaveBlogCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveBlog(cmd.Blog)
	cmd.Response() <- 0
}

// 保存多个博客cmd
type SaveBlogsCmd struct {
	core.ActorCommand
	Blogs map[string]*module.Blog
}

func (cmd *SaveBlogsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveBlogs(cmd.Blogs)
	cmd.Response() <- 0
}

// 获取博客cmd
type GetBlogCmd struct {
	core.ActorCommand
	Name string
}

func (cmd *GetBlogCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	blog := persistenceActor.getBlog(cmd.Name)
	cmd.Response() <- blog
}

// 获取所有博客cmd
type GetBlogsCmd struct {
	core.ActorCommand
}

func (cmd *GetBlogsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	blogs := persistenceActor.getBlogs()
	cmd.Response() <- blogs
}

// 删除博客cmd
type DeleteBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *DeleteBlogCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	result := persistenceActor.deleteBlog(cmd.Title)
	cmd.Response() <- result
}

// 保存博客评论cmd
type SaveBlogCommentsCmd struct {
	core.ActorCommand
	BlogComments *module.BlogComments
}

func (cmd *SaveBlogCommentsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveBlogComments(cmd.BlogComments)
	cmd.Response() <- 0
}

// 获取所有博客评论cmd
type GetAllBlogCommentsCmd struct {
	core.ActorCommand
}

func (cmd *GetAllBlogCommentsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	comments := persistenceActor.getAllBlogComments()
	cmd.Response() <- comments
}

// 保存合作信息cmd
type SaveCooperationCmd struct {
	core.ActorCommand
	Account string
	Pwd     string
	Blogs   string
	Tags    string
}

func (cmd *SaveCooperationCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	result := persistenceActor.saveCooperation(cmd.Account, cmd.Pwd, cmd.Blogs, cmd.Tags)
	cmd.Response() <- result
}

// 删除合作信息cmd
type DelCooperationCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *DelCooperationCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	result := persistenceActor.delCooperation(cmd.Account)
	cmd.Response() <- result
}

// 获取所有合作信息cmd
type GetCooperationsCmd struct {
	core.ActorCommand
}

func (cmd *GetCooperationsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	cooperations := persistenceActor.getCooperations()
	cmd.Response() <- cooperations
}

// 保存评论用户cmd
type SaveCommentUserCmd struct {
	core.ActorCommand
	User *module.CommentUser
}

func (cmd *SaveCommentUserCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveCommentUser(cmd.User)
	cmd.Response() <- 0
}

// 保存评论会话cmd
type SaveCommentSessionCmd struct {
	core.ActorCommand
	Session *module.CommentSession
}

func (cmd *SaveCommentSessionCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveCommentSession(cmd.Session)
	cmd.Response() <- 0
}

// 保存用户名预留cmd
type SaveUsernameReservationCmd struct {
	core.ActorCommand
	Reservation *module.UsernameReservation
}

func (cmd *SaveUsernameReservationCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveUsernameReservation(cmd.Reservation)
	cmd.Response() <- 0
}

// 获取所有评论用户cmd
type GetAllCommentUsersCmd struct {
	core.ActorCommand
}

func (cmd *GetAllCommentUsersCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	users := persistenceActor.getAllCommentUsers()
	cmd.Response() <- users
}

// 获取所有用户名预留cmd
type GetAllUsernameReservationsCmd struct {
	core.ActorCommand
}

func (cmd *GetAllUsernameReservationsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	reservations := persistenceActor.getAllUsernameReservations()
	cmd.Response() <- reservations
}

// 获取所有评论会话cmd
type GetAllCommentSessionsCmd struct {
	core.ActorCommand
}

func (cmd *GetAllCommentSessionsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	sessions := persistenceActor.getAllCommentSessions()
	cmd.Response() <- sessions
}

// 删除评论会话cmd
type DeleteCommentSessionCmd struct {
	core.ActorCommand
	SessionID string
}

func (cmd *DeleteCommentSessionCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.deleteCommentSession(cmd.SessionID)
	cmd.Response() <- 0
}

// 删除用户名预留cmd
type DeleteUsernameReservationCmd struct {
	core.ActorCommand
	Username string
}

func (cmd *DeleteUsernameReservationCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.deleteUsernameReservation(cmd.Username)
	cmd.Response() <- 0
}