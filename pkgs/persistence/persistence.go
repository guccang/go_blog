package persistence

import (
	"config"
	"core"
	"module"
	log "mylog"
	"strconv"
)

// 持久化模块actor
var persistence_module *PersistenceActor

func Info() {
	log.Debug("info persistence v1.0")
}

// 初始化persistence模块，用于数据持久化操作
func Init() {
	persistence_module = &PersistenceActor{
		Actor:  core.NewActor(),
		client: nil,
	}

	// 连接Redis
	ip := config.GetConfigWithAccount(config.GetAdminAccount(), "redis_ip")
	port, _ := strconv.Atoi(config.GetConfigWithAccount(config.GetAdminAccount(), "redis_port"))
	pwd := config.GetConfigWithAccount(config.GetAdminAccount(), "redis_pwd")
	persistence_module.connect(ip, port, pwd)
	persistence_module.Start(persistence_module)
}

// interface

func SaveBlog(account string, blog *module.Blog) {
	cmd := &SaveBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Blog:    blog,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func SaveBlogs(account string, blogs map[string]*module.Blog) {
	cmd := &SaveBlogsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Blogs:   blogs,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func GetBlogsByAccount(account string) map[string]*module.Blog {
	cmd := &GetBlogsByAccountCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.Blog)
}

func SaveBlogComments(account string, bc *module.BlogComments) {
	cmd := &SaveBlogCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:      account,
		BlogComments: bc,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func GetAllBlogComments(account string) map[string]*module.BlogComments {
	cmd := &GetAllBlogCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.BlogComments)
}

func SaveCommentUser(account string, user *module.CommentUser) {
	cmd := &SaveCommentUserCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		User:    user,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func SaveCommentSession(account string, session *module.CommentSession) {
	cmd := &SaveCommentSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Session: session,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func SaveUsernameReservation(account string, reservation *module.UsernameReservation) {
	cmd := &SaveUsernameReservationCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:     account,
		Reservation: reservation,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func GetAllCommentUsers(account string) map[string]*module.CommentUser {
	cmd := &GetAllCommentUsersCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.CommentUser)
}

func GetAllUsernameReservations(account string) map[string]*module.UsernameReservation {
	cmd := &GetAllUsernameReservationsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.UsernameReservation)
}

func GetAllCommentSessions(account string) map[string]*module.CommentSession {
	cmd := &GetAllCommentSessionsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.CommentSession)
}

func DeleteCommentSession(account, sessionID string) {
	cmd := &DeleteCommentSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		SessionID: sessionID,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func DeleteUsernameReservation(account, username string) {
	cmd := &DeleteUsernameReservationCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		Username: username,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

// Multi-account support functions

func SaveBlogWithAccount(account string, blog *module.Blog) {
	// Ensure blog has the correct account
	if blog.Account == "" {
		blog.Account = account
	}
	SaveBlog(account, blog)
}

func SaveBlogsWithAccount(account string, blogs map[string]*module.Blog) {
	// Ensure all blogs have the correct account
	for _, blog := range blogs {
		if blog.Account == "" {
			blog.Account = account
		}
	}
	SaveBlogs(account, blogs)
}

func GetBlogWithAccount(account, name string) *module.Blog {
	cmd := &GetBlogWithAccountCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Name:    name,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(*module.Blog)
}

func DeleteBlogWithAccount(account, title string) int {
	cmd := &DeleteBlogWithAccountCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Title:   title,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func SaveBlogCommentsWithAccount(account string, bc *module.BlogComments) {
	SaveBlogComments(account, bc)
}

func GetAllBlogCommentsWithAccount(account string) map[string]*module.BlogComments {
	return GetAllBlogComments(account)
}

func SaveCommentUserWithAccount(account string, user *module.CommentUser) {
	SaveCommentUser(account, user)
}

func SaveCommentSessionWithAccount(account string, session *module.CommentSession) {
	SaveCommentSession(account, session)
}

func SaveUsernameReservationWithAccount(account string, reservation *module.UsernameReservation) {
	SaveUsernameReservation(account, reservation)
}

func GetAllCommentUsersWithAccount(account string) map[string]*module.CommentUser {
	return GetAllCommentUsers(account)
}

func GetAllUsernameReservationsWithAccount(account string) map[string]*module.UsernameReservation {
	return GetAllUsernameReservations(account)
}

func GetAllCommentSessionsWithAccount(account string) map[string]*module.CommentSession {
	return GetAllCommentSessions(account)
}

func DeleteCommentSessionWithAccount(account, sessionID string) {
	DeleteCommentSession(account, sessionID)
}

func DeleteUsernameReservationWithAccount(account, username string) {
	DeleteUsernameReservation(account, username)
}
