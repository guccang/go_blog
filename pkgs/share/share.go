package share

import (
	"core"
	log "mylog"
)

// 分享模块actor
var share_module *ShareActor

func Info() {
	log.InfoF(log.ModuleShare, "info share v8.0")
}

// 初始化share模块，用于博客和标签分享管理
func Init() {
	share_module = &ShareActor{
		Actor:       core.NewActor(),
		sharedBlogs: make(map[string]*SharedBlog),
		sharedTags:  make(map[string]*SharedTag),
	}
	share_module.Start(share_module)
}

// interface

func GetSharedBlogs() map[string]*SharedBlog {
	cmd := &GetSharedBlogsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	share_module.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return nil
	}
	return ret.(map[string]*SharedBlog)
}

func GetSharedTags() map[string]*SharedTag {
	cmd := &GetSharedTagsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	share_module.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return nil
	}
	return ret.(map[string]*SharedTag)
}

func GetSharedBlog(title string) *SharedBlog {
	cmd := &GetSharedBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	share_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(*SharedBlog)
}

func GetSharedTag(tag string) *SharedTag {
	cmd := &GetSharedTagCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Tag: tag,
	}
	share_module.Send(cmd)
	result := <-cmd.Response()
	if result == nil {
		return nil
	}
	return result.(*SharedTag)
}

func AddSharedBlog(title string) (string, string) {
	cmd := &AddSharedBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	share_module.Send(cmd)
	url := <-cmd.Response()
	pwd := <-cmd.Response()
	return url.(string), pwd.(string)
}

func AddSharedTag(tag string) (string, string) {
	cmd := &AddSharedTagCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Tag: tag,
	}
	share_module.Send(cmd)
	url := <-cmd.Response()
	pwd := <-cmd.Response()
	return url.(string), pwd.(string)
}

func ModifyCntSharedBlog(title string, c int) int {
	cmd := &ModifyCntSharedBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
		Count: c,
	}
	share_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}

func ModifyCntSharedTag(tag string, c int) int {
	cmd := &ModifyCntSharedTagCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Tag:   tag,
		Count: c,
	}
	share_module.Send(cmd)
	result := <-cmd.Response()
	return result.(int)
}
