package config

import (
	"core"
)

// cmd

// 获取配置值cmd
type GetConfigCmd struct {
	core.ActorCommand
	Name string
}

func (cmd *GetConfigCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	value := configActor.getConfig(cmd.Name)
	cmd.Response() <- value
}

// 重新加载配置cmd
type ReloadConfigCmd struct {
	core.ActorCommand
	Account  string
	FilePath string
}

func (cmd *ReloadConfigCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	ret := configActor.reloadConfig(cmd.Account, cmd.FilePath)
	cmd.Response() <- ret
}

// 获取版本信息cmd
type GetVersionCmd struct {
	core.ActorCommand
}

func (cmd *GetVersionCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	version := configActor.getVersion()
	cmd.Response() <- version
}

// 检查是否为系统文件cmd
type IsSysFileCmd struct {
	core.ActorCommand
	Name string
}

func (cmd *IsSysFileCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	ret := configActor.isSysFile(cmd.Name)
	cmd.Response() <- ret
}

// 检查是否为公开标签cmd
type IsPublicTagCmd struct {
	core.ActorCommand
	Tag string
}

func (cmd *IsPublicTagCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	ret := configActor.isPublicTag(cmd.Tag)
	cmd.Response() <- ret
}

// 检查标题是否需要添加日期后缀cmd
type IsTitleAddDateSuffixCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *IsTitleAddDateSuffixCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	ret := configActor.isTitleAddDateSuffix(cmd.Title)
	cmd.Response() <- ret
}

// 检查是否为日记博客cmd
type IsDiaryBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *IsDiaryBlogCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	ret := configActor.isDiaryBlog(cmd.Title)
	cmd.Response() <- ret
}

// 获取日记关键字列表cmd
type GetDiaryKeywordsCmd struct {
	core.ActorCommand
}

func (cmd *GetDiaryKeywordsCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	keywords := configActor.getDiaryKeywords()
	cmd.Response() <- keywords
}

// 获取配置文件路径cmd
type GetConfigPathCmd struct {
	core.ActorCommand
}

func (cmd *GetConfigPathCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	path := configActor.getConfigPath()
	cmd.Response() <- path
}

// 检查标题是否包含日期后缀cmd
type IsTitleContainsDateSuffixCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *IsTitleContainsDateSuffixCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	ret := configActor.isTitleContainsDateSuffix(cmd.Title)
	cmd.Response() <- ret
}

// 从博客内容更新配置cmd
type UpdateConfigFromBlogCmd struct {
	core.ActorCommand
	BlogContent string
}

func (cmd *UpdateConfigFromBlogCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)
	configActor.updateConfigFromBlog(cmd.BlogContent)
	cmd.Response() <- 0
}
