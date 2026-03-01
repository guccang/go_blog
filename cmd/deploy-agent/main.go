package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

const keyringService = "deploy-agent"

func main() {
	configPath := flag.String("config", "deploy.conf", "配置文件路径")
	projectName := flag.String("project", "", "指定要部署的项目名称（多项目时必须指定）")
	targetName := flag.String("target", "", "只部署指定目标（name 或 host）")
	packOnly := flag.Bool("pack-only", false, "只打包不部署")
	password := flag.String("password", "", "SSH 密码")
	savePwd := flag.Bool("save-password", false, "保存密码到凭据存储")
	listProjects := flag.Bool("list", false, "列出所有配置的项目")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	cred := newCredentialStore()

	// 列出所有项目
	if *listProjects {
		fmt.Printf("配置文件: %s\n", *configPath)
		fmt.Printf("项目数量: %d\n\n", len(cfg.Projects))
		for _, name := range cfg.ProjectOrder {
			proj := cfg.Projects[name]
			fmt.Printf("[%s]\n", name)
			if proj.ConfigFile != "" {
				fmt.Printf("  配置来源: %s\n", proj.ConfigFile)
			}
			fmt.Printf("  项目目录: %s\n", proj.ProjectDir)
			fmt.Printf("  打包脚本: %s\n", proj.PackScript)
			fmt.Printf("  部署目标: %d 个\n", len(proj.Targets))
			for _, t := range proj.Targets {
				fmt.Printf("    - %s (%s) -> %s\n", t.Name, t.Host, t.RemoteDir)
			}
			if proj.VerifyURL != "" {
				fmt.Printf("  验证URL:  %s\n", proj.VerifyURL)
			}
			fmt.Println()
		}
		return
	}

	// daemon 模式（WebSocket）
	if cfg.ServerURL != "" {
		fmt.Printf("Deploy Agent (daemon mode)\n")
		fmt.Printf("服务地址: %s\n", cfg.ServerURL)
		fmt.Printf("Agent名称: %s\n", cfg.AgentName)
		fmt.Printf("最大并发: %d\n", cfg.MaxConcurrent)
		fmt.Printf("项目列表: %v\n\n", cfg.ProjectNames())

		// daemon 模式密码
		pwd := *password
		if pwd == "" && cfg.SSHPassword != "" {
			pwd = cfg.SSHPassword
		}
		if pwd == "" {
			// 从凭据存储取第一个项目的第一个 target 密码
			for _, name := range cfg.ProjectOrder {
				proj := cfg.Projects[name]
				if len(proj.Targets) > 0 {
					t := proj.Targets[0]
					user, host := parseHost(t.Host)
					accountKey := fmt.Sprintf("%s@%s:%d", user, host, t.Port)
					if saved, err := cred.Get(accountKey); err == nil && saved != "" {
						pwd = saved
						fmt.Printf("已从凭据存储获取密码 (%s)\n", accountKey)
						break
					}
				}
			}
		}
		if pwd == "" {
			fmt.Print("SSH 密码: ")
			if pwdBytes, err := term.ReadPassword(int(syscall.Stdin)); err == nil {
				pwd = string(pwdBytes)
			} else {
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				pwd = strings.TrimSpace(line)
			}
			fmt.Println()
		}

		agentID := fmt.Sprintf("%s-%d", cfg.AgentName, os.Getpid())
		conn := NewConnection(cfg, pwd, agentID)
		conn.Run()
		return
	}

	// CLI 模式：选择项目
	var proj *ProjectConfig
	if *projectName != "" {
		proj = cfg.GetProject(*projectName)
		if proj == nil {
			fmt.Fprintf(os.Stderr, "项目 %q 不存在，可用项目: %v\n", *projectName, cfg.ProjectNames())
			os.Exit(1)
		}
	} else {
		proj = cfg.DefaultProject()
		if proj == nil {
			fmt.Fprintf(os.Stderr, "配置了多个项目，请使用 -project 指定:\n")
			for _, name := range cfg.ProjectOrder {
				fmt.Fprintf(os.Stderr, "  - %s\n", name)
			}
			os.Exit(1)
		}
	}

	fmt.Printf("项目: [%s]\n", proj.Name)
	fmt.Printf("项目目录: %s\n", proj.ProjectDir)
	fmt.Printf("打包脚本: %s\n", proj.PackScript)
	fmt.Printf("部署目标: %d 个\n", len(proj.Targets))
	for _, t := range proj.Targets {
		fmt.Printf("  - %s (%s) -> %s\n", t.Name, t.Host, t.RemoteDir)
	}
	fmt.Println()

	// 解析密码（pack-only 时跳过）
	pwd := *password
	if pwd == "" && !*packOnly {
		// 尝试从凭据存储获取
		if len(proj.Targets) > 0 {
			t := proj.Targets[0]
			user, host := parseHost(t.Host)
			accountKey := fmt.Sprintf("%s@%s:%d", user, host, t.Port)
			if saved, err := cred.Get(accountKey); err == nil && saved != "" {
				pwd = saved
				fmt.Printf("已从凭据存储获取密码 (%s)\n", accountKey)
			}
		}
	}
	if pwd == "" && !*packOnly {
		// 交互式输入密码
		fmt.Print("SSH 密码: ")
		if pwdBytes, err := term.ReadPassword(int(syscall.Stdin)); err == nil {
			pwd = string(pwdBytes)
		} else {
			// 降级为明文输入
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			pwd = strings.TrimSpace(line)
		}
		fmt.Println()
	}

	deployer := NewDeployer(cfg, proj, pwd)
	deployErr := deployer.Run(*packOnly, *targetName)

	// SSH 连接成功即保存密码（证明密码有效），不依赖后续步骤
	if *savePwd && pwd != "" && deployer.SSHConnected {
		for _, t := range proj.Targets {
			user, host := parseHost(t.Host)
			accountKey := fmt.Sprintf("%s@%s:%d", user, host, t.Port)
			if err := cred.Set(accountKey, pwd); err != nil {
				fmt.Fprintf(os.Stderr, "保存密码失败 (%s): %v\n", accountKey, err)
			} else {
				fmt.Printf("密码已保存到凭据存储 (%s)\n", accountKey)
			}
		}
	}

	if deployErr != nil {
		fmt.Fprintf(os.Stderr, "部署失败: %v\n", deployErr)
		os.Exit(1)
	}
}
