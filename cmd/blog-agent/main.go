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
	"strconv"
	"syscall"
	"tools"

	"golang.org/x/term"
)

func clearup() {
	log.Debug(log.ModuleCommon, "blog-agent clearup")
}

func parsePortOverride(args []string) ([]string, string, error) {
	filtered := make([]string, 0, len(args))
	filtered = append(filtered, args[0])
	port := ""
	for i := 1; i < len(args); i++ {
		if args[i] != "-port" && args[i] != "--port" {
			filtered = append(filtered, args[i])
			continue
		}
		if i+1 >= len(args) {
			return nil, "", fmt.Errorf("%s requires a port value", args[i])
		}
		port = args[i+1]
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return nil, "", fmt.Errorf("invalid port: %s", port)
		}
		i++
	}
	return filtered, port, nil
}

func resetPassword(account, configPath string) error {
	config.Init(configPath)
	persistence.Init()

	fmt.Printf("Enter a new password for %s: ", account)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	fmt.Println()
	if len(password) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	fmt.Print("Confirm the new password: ")
	confirmedPassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("read password confirmation: %w", err)
	}
	fmt.Println()
	if string(password) != string(confirmedPassword) {
		return fmt.Errorf("password confirmation does not match")
	}

	if err := persistence.SaveUser(account, string(password)); err != nil {
		return fmt.Errorf("save SQLite credential: %w", err)
	}
	fmt.Printf("Password reset completed for account %s.\n", account)
	return nil
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

	args, portOverride, err := parsePortOverride(os.Args)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
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
	if len(args) >= 2 && args[1] == "reset-password" {
		if len(args) != 4 {
			fmt.Println("usage: blog-agent reset-password <account> <sys_conf_path>")
			return
		}
		if err := resetPassword(args[2], args[3]); err != nil {
			fmt.Printf("Password reset failed: %v\n", err)
		}
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
	config.SetSQLiteConfigReader(func(account string) string {
		item := persistence.GetBlogWithAccount(account, config.GetSysConfigTitle())
		if item == nil {
			return ""
		}
		return item.Content
	})
	config.ReloadConfigFromSQLite(config.GetAdminAccount())
	blog.Init()
	reading.Init()
	auth.Init()
	login.Init()
	exercise.Init()
	if err := goal.InitGoalModule(); err != nil {
		log.ErrorF(log.ModuleYearPlan, "legacy goal migration failed: %v", err)
	}
	share.Init()

	log.Debug(log.ModuleCommon, "blog-agent started")

	certFile := ""
	keyFile := ""
	if len(args) == 4 {
		certFile = args[2]
		keyFile = args[3]
	}
	err = http.Run(certFile, keyFile, portOverride)

	log.Debug(log.ModuleCommon, fmt.Sprintf("blog-agent exit %s", err.Error()))
	log.FlushLogs()
	log.Cleanup()
}
