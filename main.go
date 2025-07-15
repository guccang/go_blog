package main

import(
	"fmt"
	"os"
	"module"
	"view"
	"control"
	"http"
	"persistence"
	"config"
	log "mylog"
	"ioutils"
	"login"
	"comment"
	"blog"
	"search"
	"share"
	"cooperation"
	"statistics"
)

func main(){
	args := os.Args
	for _,arg := range args {
		fmt.Println(arg);
	}
	if len(args) <2 {
		fmt.Println("need sys_conf path");
		return
	}
	// versions
	log.Debug("go_blog starting")
	module.Info()
	view.Info()
	control.Info()
	http.Info()
	persistence.Info()
	log.Info()
	config.Info()
	ioutils.Info()
	blog.Info()
	comment.Info()
	search.Info()
	share.Info()
	cooperation.Info()
	statistics.Info()

	// Init 
	config.Init(args[1])
	persistence.Init()
	control.Init()
	login.Init()
	blogs_txt_dir := config.GetBlogsPath()
	control.ImportBlogsFromPath(blogs_txt_dir)
	cooperation.Init()
	persistence.SaveBlogs(blog.Blogs)

	log.Debug("go_blog started")

	certFile := ""
	keyFile := ""
	if len(args) == 4 {
		certFile = args[2]
		keyFile = args[3]
	}
	http.Run(certFile,keyFile)

	log.Debug("go_blog exit")
}

