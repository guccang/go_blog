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
	ip := config.GetConfig("redis_ip")
	port, _ := strconv.Atoi(config.GetConfig("redis_port"))
	pwd := config.GetConfig("redis_pwd")
	persistence_module.connect(ip, port, pwd)
	persistence_module.Start(persistence_module)
}

// interface

func SaveBlog(blog *module.Blog) {
	cmd := &SaveBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Blog: blog,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func SaveBlogs(blogs map[string]*module.Blog) {
	cmd := &SaveBlogsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Blogs: blogs,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func GetBlog(name string) *module.Blog {
	cmd := &GetBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Name: name,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(*module.Blog)
}

func GetBlogs() map[string]*module.Blog {
	cmd := &GetBlogsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.Blog)
}

func DeleteBlog(title string) int {
	cmd := &DeleteBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func SaveBlogComments(bc *module.BlogComments) {
	cmd := &SaveBlogCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BlogComments: bc,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func GetAllBlogComments() map[string]*module.BlogComments {
	cmd := &GetAllBlogCommentsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.BlogComments)
}

func SaveCommentUser(user *module.CommentUser) {
	cmd := &SaveCommentUserCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		User: user,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func SaveCommentSession(session *module.CommentSession) {
	cmd := &SaveCommentSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Session: session,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func SaveUsernameReservation(reservation *module.UsernameReservation) {
	cmd := &SaveUsernameReservationCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Reservation: reservation,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func GetAllCommentUsers() map[string]*module.CommentUser {
	cmd := &GetAllCommentUsersCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.CommentUser)
}

func GetAllUsernameReservations() map[string]*module.UsernameReservation {
	cmd := &GetAllUsernameReservationsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.UsernameReservation)
}

func GetAllCommentSessions() map[string]*module.CommentSession {
	cmd := &GetAllCommentSessionsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	persistence_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(map[string]*module.CommentSession)
}

func DeleteCommentSession(sessionID string) {
	cmd := &DeleteCommentSessionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		SessionID: sessionID,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}

func DeleteUsernameReservation(username string) {
	cmd := &DeleteUsernameReservationCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Username: username,
	}
	persistence_module.Send(cmd)
	<-cmd.Response()
}
