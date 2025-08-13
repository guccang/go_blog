package share

import (
	"core"
)

// cmd

// 获取共享博客cmd
type GetSharedBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *GetSharedBlogCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	blog := shareActor.getSharedBlog(cmd.Title)
	cmd.Response() <- blog
}

// 获取共享标签cmd
type GetSharedTagCmd struct {
	core.ActorCommand
	Tag string
}

func (cmd *GetSharedTagCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	tag := shareActor.getSharedTag(cmd.Tag)
	cmd.Response() <- tag
}

// 添加共享博客cmd
type AddSharedBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *AddSharedBlogCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	url, pwd := shareActor.addSharedBlog(cmd.Title)
	cmd.Response() <- url
	cmd.Response() <- pwd
}

// 添加共享标签cmd
type AddSharedTagCmd struct {
	core.ActorCommand
	Tag string
}

func (cmd *AddSharedTagCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	url, pwd := shareActor.addSharedTag(cmd.Tag)
	cmd.Response() <- url
	cmd.Response() <- pwd
}

// 修改共享博客计数cmd
type ModifyCntSharedBlogCmd struct {
	core.ActorCommand
	Title string
	Count int
}

func (cmd *ModifyCntSharedBlogCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	result := shareActor.modifyCntSharedBlog(cmd.Title, cmd.Count)
	cmd.Response() <- result
}

// 修改共享标签计数cmd
type ModifyCntSharedTagCmd struct {
	core.ActorCommand
	Tag   string
	Count int
}

func (cmd *ModifyCntSharedTagCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	result := shareActor.modifyCntSharedTag(cmd.Tag, cmd.Count)
	cmd.Response() <- result
}