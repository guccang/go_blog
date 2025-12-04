package blog

import (
	"core"
	"module"
)

// load blogs
type loadBlogsCmd struct{ core.ActorCommand }

func (cmd *loadBlogsCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	_ = blogActor.loadBlogs()
	cmd.Response() <- 0
}

// import blogs from path
type importBlogsCmd struct {
	core.ActorCommand
	Dir string
}

func (cmd *importBlogsCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	_ = blogActor.importBlogsFromPath(cmd.Dir)
	cmd.Response() <- 0
}

// get a blog by title
type getBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *getBlogCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	b := blogActor.getBlog(cmd.Title)
	cmd.Response() <- b
}

// add blog
type addBlogCmd struct {
	core.ActorCommand
	UDB *module.UploadedBlogData
}

func (cmd *addBlogCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	ret := blogActor.addBlog(cmd.UDB)
	cmd.Response() <- ret
}

// modify blog
type modifyBlogCmd struct {
	core.ActorCommand
	UDB *module.UploadedBlogData
}

func (cmd *modifyBlogCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	ret := blogActor.modifyBlog(cmd.UDB)
	cmd.Response() <- ret
}

// delete blog
type deleteBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *deleteBlogCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	ret := blogActor.deleteBlog(cmd.Title)
	cmd.Response() <- ret
}

// get recently timed blog
type getRecentlyTimedBlogCmd struct {
	core.ActorCommand
	Title string
}

func (cmd *getRecentlyTimedBlogCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	b := blogActor.getRecentlyTimedBlog(cmd.Title)
	cmd.Response() <- b
}

// get all blogs by flag
type getAllCmd struct {
	core.ActorCommand
	Num  int
	Flag int
}

func (cmd *getAllCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	list := blogActor.getAll(cmd.Num, cmd.Flag)
	cmd.Response() <- list
}

// update access time
type updateAccessTimeCmd struct {
	core.ActorCommand
	Blog *module.Blog
}

func (cmd *updateAccessTimeCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	blogActor.updateAccessTime(cmd.Blog)
	cmd.Response() <- 0
}

// get blog auth type
type getBlogAuthTypeCmd struct {
	core.ActorCommand
	Blogname string
}

func (cmd *getBlogAuthTypeCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	ret := blogActor.getBlogAuthType(cmd.Blogname)
	cmd.Response() <- ret
}

// tag replace
type tagReplaceCmd struct {
	core.ActorCommand
	From string
	To   string
}

func (cmd *tagReplaceCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	cmd.Response() <- blogActor.tagReplace(cmd.From, cmd.To)
}

type tagAddCmd struct {
	core.ActorCommand
	Title string
	Tag   string
}

func (cmd *tagAddCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	cmd.Response() <- blogActor.tagAdd(cmd.Title, cmd.Tag)
}

// set same auth
type setSameAuthCmd struct {
	core.ActorCommand
	Blogname string
}

func (cmd *setSameAuthCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	blogActor.setSameAuth(cmd.Blogname)
	cmd.Response() <- 0
}

// add/del auth type
type addAuthTypeCmd struct {
	core.ActorCommand
	Blogname string
	Flag     int
}

type delAuthTypeCmd struct {
	core.ActorCommand
	Blogname string
	Flag     int
}

func (cmd *addAuthTypeCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	blogActor.addAuthType(cmd.Blogname, cmd.Flag)
	cmd.Response() <- 0
}

func (cmd *delAuthTypeCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	blogActor.delAuthType(cmd.Blogname, cmd.Flag)
	cmd.Response() <- 0
}

type getURLNamesCmd struct {
	core.ActorCommand
	Blogname string
}

func (cmd *getURLNamesCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	cmd.Response() <- blogActor.getURLBlogNames(cmd.Blogname)
}

type getBlogsNumCmd struct {
	core.ActorCommand
}

func (cmd *getBlogsNumCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	cmd.Response() <- len(blogActor.blogs)
}
