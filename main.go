package main

import (
	"agent"
	"auth"
	"blog"
	"comment"
	"config"
	"control"
	"exercise"
	"fmt"
	"http"
	"ioutils"
	"llm"
	"login"
	"mcp"
	"module"
	log "mylog"
	"os"
	"os/signal"
	"persistence"
	"reading"
	"search"
	"share"
	"sms"
	"statistics"
	"syscall"
	"tools"
	"view"
)

func clearup() {
	log.Debug(log.ModuleCommon, "go_blog clearup")
	agent.Shutdown()
	mcp.GetPool().Shutdown()
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
	for _, arg := range args {
		fmt.Println(arg)
	}
	if len(args) < 2 {
		fmt.Println("need sys_conf path")
		return
	}
	log.Info()

	// versions
	log.Debug(log.ModuleCommon, "go_blog starting")
	module.Info()
	view.Info()
	control.Info()
	http.Info()
	persistence.Info()
	config.Info()
	ioutils.Info()
	blog.Info()
	comment.Info()
	search.Info()
	share.Info()
	statistics.Info()
	mcp.Info()
	llm.Info()
	tools.Info()
	exercise.Info()
	reading.Info()
	agent.Info()

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
	control.Init()
	comment.Init()
	reading.Init()
	statistics.Init()
	auth.Init()
	login.Init()
	go mcp.Init()
	llm.Init()
	sms.Init()
	exercise.Init()
	share.Init()
	agent.Init(account)
	log.Debug(log.ModuleCommon, "go_blog started")

	certFile := ""
	keyFile := ""
	if len(args) == 4 {
		certFile = args[2]
		keyFile = args[3]
	}
	err := http.Run(certFile, keyFile)

	log.Debug(log.ModuleCommon, fmt.Sprintf("go_blog exit %s", err.Error()))
	log.FlushLogs()
	log.Cleanup()
}
