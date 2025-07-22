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
	"mcp"
	"llm"
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
	mcp.Info()
	llm.Info()

	// Init 
	config.Init(args[1])
	
	// Initialize logging system with logs directory
	logsDir := config.GetConfig("logs_dir")
	if logsDir == "" {
		logsDir = "logs" // Default logs directory
	}
	if err := log.Init(logsDir); err != nil {
		fmt.Printf("Warning: Failed to initialize file logging: %v\n", err)
		fmt.Println("Continuing with console logging only...")
	}
	log.Debug("Logging system initialized")
	
	persistence.Init()
	control.Init()
	login.Init()
	blogs_txt_dir := config.GetBlogsPath()
	control.ImportBlogsFromPath(blogs_txt_dir)
	cooperation.Init()
	mcp.Init()
	llm.Init()
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
	log.FlushLogs()
	log.Cleanup()
}

