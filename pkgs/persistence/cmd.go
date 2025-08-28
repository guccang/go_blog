package persistence

import (
	"core"
	"module"
)

// cmd

// 保存博客cmd
type SaveBlogCmd struct {
	core.ActorCommand
	Blog    *module.Blog
	Account string
}

func (cmd *SaveBlogCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveBlog(cmd.Account, cmd.Blog)
	cmd.Response() <- 0
}

// 保存多个博客cmd
type SaveBlogsCmd struct {
	core.ActorCommand
	Blogs   map[string]*module.Blog
	Account string
}

func (cmd *SaveBlogsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveBlogs(cmd.Account, cmd.Blogs)
	cmd.Response() <- 0
}

// 获取所有博客cmd
type GetBlogsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetBlogsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	blogs := persistenceActor.getBlogsByAccount(cmd.Account)
	cmd.Response() <- blogs
}

// 删除博客cmd
type DeleteBlogCmd struct {
	core.ActorCommand
	Title   string
	Account string
}

func (cmd *DeleteBlogCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	result := persistenceActor.deleteBlog(cmd.Account, cmd.Title)
	cmd.Response() <- result
}

// 保存博客评论cmd
type SaveBlogCommentsCmd struct {
	core.ActorCommand
	BlogComments *module.BlogComments
	Account      string
}

func (cmd *SaveBlogCommentsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveBlogComments(cmd.Account, cmd.BlogComments)
	cmd.Response() <- 0
}

// 获取所有博客评论cmd
type GetAllBlogCommentsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllBlogCommentsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	comments := persistenceActor.getAllBlogComments(cmd.Account)
	cmd.Response() <- comments
}

// 保存评论用户cmd
type SaveCommentUserCmd struct {
	core.ActorCommand
	Account string
	User    *module.CommentUser
}

func (cmd *SaveCommentUserCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveCommentUser(cmd.Account, cmd.User)
	cmd.Response() <- 0
}

// 保存评论会话cmd
type SaveCommentSessionCmd struct {
	core.ActorCommand
	Account string
	Session *module.CommentSession
}

func (cmd *SaveCommentSessionCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveCommentSession(cmd.Account, cmd.Session)
	cmd.Response() <- 0
}

// 保存用户名预留cmd
type SaveUsernameReservationCmd struct {
	core.ActorCommand
	Account     string
	Reservation *module.UsernameReservation
}

func (cmd *SaveUsernameReservationCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.saveUsernameReservation(cmd.Account, cmd.Reservation)
	cmd.Response() <- 0
}

// 获取所有评论用户cmd
type GetAllCommentUsersCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllCommentUsersCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	users := persistenceActor.getAllCommentUsers(cmd.Account)
	cmd.Response() <- users
}

// 获取所有用户名预留cmd
type GetAllUsernameReservationsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllUsernameReservationsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	reservations := persistenceActor.getAllUsernameReservations(cmd.Account)
	cmd.Response() <- reservations
}

// 获取所有评论会话cmd
type GetAllCommentSessionsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllCommentSessionsCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	sessions := persistenceActor.getAllCommentSessions(cmd.Account)
	cmd.Response() <- sessions
}

// 删除评论会话cmd
type DeleteCommentSessionCmd struct {
	core.ActorCommand
	SessionID string
	Account   string
}

func (cmd *DeleteCommentSessionCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.deleteCommentSession(cmd.Account, cmd.SessionID)
	cmd.Response() <- 0
}

// 删除用户名预留cmd
type DeleteUsernameReservationCmd struct {
	core.ActorCommand
	Username string
	Account  string
}

func (cmd *DeleteUsernameReservationCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	persistenceActor.deleteUsernameReservation(cmd.Account, cmd.Username)
	cmd.Response() <- 0
}

// 按账户获取博客cmd
type GetBlogsByAccountCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetBlogsByAccountCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	blogs := persistenceActor.getBlogsByAccount(cmd.Account)
	cmd.Response() <- blogs
}

// 按账户获取单个博客cmd
type GetBlogWithAccountCmd struct {
	core.ActorCommand
	Account string
	Name    string
}

func (cmd *GetBlogWithAccountCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	blog := persistenceActor.getBlogWithAccount(cmd.Account, cmd.Name)
	cmd.Response() <- blog
}

// 按账户删除博客cmd
type DeleteBlogWithAccountCmd struct {
	core.ActorCommand
	Account string
	Title   string
}

func (cmd *DeleteBlogWithAccountCmd) Do(actor core.ActorInterface) {
	persistenceActor := actor.(*PersistenceActor)
	result := persistenceActor.deleteBlogWithAccount(cmd.Account, cmd.Title)
	cmd.Response() <- result
}
