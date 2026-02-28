package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const keyringService = "deploy-agent"

func main() {
	configPath := flag.String("config", "deploy.conf", "配置文件路径")
	targetName := flag.String("target", "", "只部署指定目标（name 或 host）")
	packOnly := flag.Bool("pack-only", false, "只打包不部署")
	password := flag.String("password", "", "SSH 密码")
	savePwd := flag.Bool("save-password", false, "保存密码到系统凭据管理器")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("项目目录: %s\n", cfg.ProjectDir)
	fmt.Printf("打包脚本: %s\n", cfg.PackScript)
	fmt.Printf("部署目标: %d 个\n", len(cfg.Targets))
	for _, t := range cfg.Targets {
		fmt.Printf("  - %s (%s) -> %s\n", t.Name, t.Host, t.RemoteDir)
	}
	fmt.Println()

	// 解析密码（pack-only 时跳过）
	pwd := *password
	if pwd == "" && !*packOnly {
		// 尝试从系统凭据管理器获取
		t := cfg.Targets[0]
		user, host := parseHost(t.Host)
		accountKey := fmt.Sprintf("%s@%s:%d", user, host, t.Port)
		if saved, err := keyring.Get(keyringService, accountKey); err == nil && saved != "" {
			pwd = saved
			fmt.Printf("已从系统凭据管理器获取密码 (%s)\n", accountKey)
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

	deployer := NewDeployer(cfg, pwd)
	deployErr := deployer.Run(*packOnly, *targetName)

	// SSH 连接成功即保存密码（证明密码有效），不依赖后续步骤
	if *savePwd && pwd != "" && deployer.SSHConnected {
		for _, t := range cfg.Targets {
			user, host := parseHost(t.Host)
			accountKey := fmt.Sprintf("%s@%s:%d", user, host, t.Port)
			if err := keyring.Set(keyringService, accountKey, pwd); err != nil {
				fmt.Fprintf(os.Stderr, "保存密码失败 (%s): %v\n", accountKey, err)
			} else {
				fmt.Printf("密码已保存到系统凭据管理器 (%s)\n", accountKey)
			}
		}
	}

	if deployErr != nil {
		fmt.Fprintf(os.Stderr, "部署失败: %v\n", deployErr)
		os.Exit(1)
	}
}
