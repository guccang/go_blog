package main

import(
	"fmt"
	"os"
	"module"
	"view"
	"control"
	"http"
	"http_template"
	"protocol"
	"persistence"
	"config"
	log "mylog"
	"ioutils"
	"login"
)

func main(){
	args := os.Args
	for _,arg := range args {
		fmt.Println(arg);
	}
	if len(args) !=2 {
		fmt.Println("need blog.conf path");
		return
	}
	// versions
	log.Debug("go_blog starting")
	module.Info()
	view.Info()
	control.Info()
	http.Info()
	http_template.Info()
	protocol.Info()
	persistence.Info()
	log.Info()
	config.Info()
	ioutils.Info()

	// Init 
	config.Init(args[1])
	persistence.Init()
	control.Init()
	login.Init()
	blogs_txt_dir := config.GetBlogsPath()
	control.ImportBlogsFromPath(blogs_txt_dir)
	persistence.SaveBlogs(control.Blogs)

	log.Debug("go_blog started")

	http.Run()

	log.Debug("go_blog exit")
}

