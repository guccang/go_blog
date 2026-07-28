package main

import (
	"auth"
	"blog"
	"config"
	"exercise"
	"fmt"
	"goal"
	"http"
	"ioutils"
	"login"
	"module"
	log "mylog"
	"os"
	"os/signal"
	"path/filepath"
	"persistence"
	"reading"
	"search"
	"share"
	"syscall"
	"tools"
)

func clearup() {
	log.Debug(log.ModuleCommon, "blog-agent clearup")
}

func main() {
	defer clearup()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	go func() {
		<-sigchan
		clearup()
		os.Exit(0)
	}()

	args := os.Args
	if len(args) >= 3 && args[1] == "migrate-sqlite" {
		config.Init(args[2])
		persistence.Init()
		report, err := persistence.MigrateMarkdownBlogs(filepath.Join(config.GetExePath(), "blogs_txt"))
		if err != nil {
			fmt.Printf("SQLite migration failed: %v\n", err)
			return
		}
		fmt.Printf("SQLite migration complete: %d Markdown files, %d blogs written. Source files were kept unchanged.\n", report.Files, report.Blogs)
		return
	}
	for _, arg := range args {
		fmt.Println(arg)
	}
	if len(args) < 2 {
		fmt.Println("need sys_conf path")
		return
	}
	log.Info()

	// versions
	log.Debug(log.ModuleCommon, "blog-agent starting")
	module.Info()
	http.Info()
	persistence.Info()
	config.Info()
	ioutils.Info()
	blog.Info()
	search.Info()
	share.Info()
	tools.Info()
	exercise.Info()
	reading.Info()
	goal.Info()

	// Init
	config.Init(args[1])

	account := config.GetAdminAccount()
	// Initialize logging system with logs directory
	logsDir := config.GetConfigWithAccount(account, "logs_dir")
	if err := log.Init(logsDir); err != nil {
		fmt.Printf("Warning: Failed to initialize file logging: %v\n", err)
		fmt.Println("Continuing with console logging only...")
	}
	log.Debug(log.ModuleCommon, "Logging system initialized")

	persistence.Init()
	blog.Init()
	reading.Init()
	auth.Init()
	login.Init()
	exercise.Init()
	goal.InitGoalModule()
	share.Init()

	log.Debug(log.ModuleCommon, "blog-agent started")

	certFile := ""
	keyFile := ""
	if len(args) == 4 {
		certFile = args[2]
		keyFile = args[3]
	}
	err := http.Run(certFile, keyFile)

	log.Debug(log.ModuleCommon, fmt.Sprintf("blog-agent exit %s", err.Error()))
	log.FlushLogs()
	log.Cleanup()
}
