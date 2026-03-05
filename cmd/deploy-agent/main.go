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
	targetName := flag.String("target", "", "发布目标（local/ssh-prod/all，默认 local）")
	buildPlatform := flag.String("build-platform", "", "(unused, kept for compatibility)")
	packOnly := flag.Bool("pack-only", false, "只打包不部署")
	password := flag.String("password", "", "SSH 密码")
	savePwd := flag.Bool("save-password", false, "保存密码到凭据存储")
	listProjects := flag.Bool("list", false, "列出所有配置的项目和可用目标")
	pipelineName := flag.String("pipeline", "", "执行指定的部署编排")
	flag.Parse()

	_ = buildPlatform // unused, kept for compatibility

	// --list / --pipeline 模式：加载 all targets
	tf := *targetName
	if *listProjects || *pipelineName != "" {
		tf = "all"
	}

	cfg, err := LoadConfig(*configPath, tf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// pipeline 模式：需要加载所有平台配置（步骤可能跨平台）
	if *pipelineName != "" {
		cfg, err = LoadConfigForDaemon(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载配置失败 (pipeline): %v\n", err)
			os.Exit(1)
		}
	}

	// daemon 模式需要加载所有 target 和所有平台配置，以支持前端动态选择
	// 检测 daemon 模式（有 server_url 且未显式指定 CLI 参数）
	isCliMode := *projectName != "" || *targetName != "" || *packOnly || *pipelineName != ""
	if cfg.ServerURL != "" && !isCliMode && !*listProjects {
		cfg, err = LoadConfigForDaemon(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载配置失败 (daemon): %v\n", err)
			os.Exit(1)
		}
	}

	cred := newCredentialStore()

	// 列出所有项目
	if *listProjects {
		fmt.Printf("配置文件: %s\n", *configPath)
		fmt.Printf("主机平台: %s\n", cfg.HostPlatform)
		if len(cfg.TargetNames) > 0 {
			fmt.Printf("可用目标: %s\n", strings.Join(cfg.TargetNames, ", "))
		}
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
				if t.VerifyURL != "" {
					fmt.Printf("      验证URL: %s\n", t.VerifyURL)
				}
			}
			fmt.Println()
		}

		// 列出 pipeline 编排
		if cfg.PipelinesDir != "" {
			pipCfg, err := LoadPipelines(cfg.PipelinesDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "加载 pipelines 失败: %v\n", err)
			} else if len(pipCfg.Pipelines) > 0 {
				fmt.Printf("部署编排 (%d个):\n", len(pipCfg.Pipelines))
				for _, p := range pipCfg.Pipelines {
					desc := ""
					if p.Description != "" {
						desc = " — " + p.Description
					}
					fmt.Printf("  [%s]%s\n", p.Name, desc)
					for i, s := range p.Steps {
						extra := ""
						if s.Target != "" {
							extra += " target=" + s.Target
						}
						if s.BuildPlatform != "" {
							extra += " platform=" + s.BuildPlatform
						}
						if s.PackOnly {
							extra += " pack_only"
						}
						fmt.Printf("    %d. %s%s\n", i+1, s.Project, extra)
					}
				}
				fmt.Println()
			}
		}

		return
	}

	// daemon 模式（WebSocket）
	if cfg.ServerURL != "" && !isCliMode {
		fmt.Printf("Deploy Agent (daemon mode)\n")
		fmt.Printf("服务地址: %s\n", cfg.ServerURL)
		fmt.Printf("Agent名称: %s\n", cfg.AgentName)
		fmt.Printf("最大并发: %d\n", cfg.MaxConcurrent)
		fmt.Printf("项目列表: %v\n\n", cfg.ProjectNames())

		// 判断是否所有项目的所有目标都是本机
		daemonAllLocal := true
		for _, name := range cfg.ProjectOrder {
			p := cfg.Projects[name]
			for _, t := range p.Targets {
				if !isLocalTarget(t.Host) {
					daemonAllLocal = false
					break
				}
			}
			if !daemonAllLocal {
				break
			}
		}

		// daemon 模式密码（全部本机部署时跳过）
		pwd := *password
		if pwd == "" && !daemonAllLocal && cfg.SSHPassword != "" {
			pwd = cfg.SSHPassword
		}
		if pwd == "" && !daemonAllLocal {
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
		if pwd == "" && !daemonAllLocal {
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

		agentID := fmt.Sprintf("deploy_%s_%d", cfg.AgentName, os.Getpid())
		conn := NewConnection(cfg, pwd, agentID)
		// 启动 deploy 协议层（注册 + 心跳）
		go conn.StartDeployProtocol()
		conn.Run()
		return
	}

	// CLI pipeline 模式
	if *pipelineName != "" {
		if cfg.PipelinesDir == "" {
			fmt.Fprintf(os.Stderr, "未找到 pipelines/ 配置目录\n")
			os.Exit(1)
		}
		pipCfg, err := LoadPipelines(cfg.PipelinesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载 pipelines 失败: %v\n", err)
			os.Exit(1)
		}
		pip := pipCfg.Get(*pipelineName)
		if pip == nil {
			fmt.Fprintf(os.Stderr, "pipeline %q 不存在，可用: %v\n", *pipelineName, pipCfg.Names())
			os.Exit(1)
		}
		if err := ValidatePipeline(pip, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		fmt.Printf("执行 Pipeline: %s\n", pip.Name)
		if pip.Description != "" {
			fmt.Printf("描述: %s\n", pip.Description)
		}
		fmt.Printf("步骤数: %d\n\n", len(pip.Steps))

		currentPassword := *password
		for i, step := range pip.Steps {
			proj := cfg.GetProject(step.Project)
			fmt.Printf("==================== [%d/%d] %s ====================\n", i+1, len(pip.Steps), step.Project)

			stepCfg := *cfg

			packOnly := step.PackOnly
			targetFilter := step.Target

			// 判断是否需要密码
			allLocal := true
			for _, t := range proj.Targets {
				if !isLocalTarget(t.Host) {
					allLocal = false
					break
				}
			}

			pwd := currentPassword
			if pwd == "" && !packOnly && !allLocal {
				// 从凭据存储获取
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
			if pwd == "" && !packOnly && !allLocal {
				fmt.Print("SSH 密码: ")
				if pwdBytes, err := term.ReadPassword(int(syscall.Stdin)); err == nil {
					pwd = string(pwdBytes)
				} else {
					reader := bufio.NewReader(os.Stdin)
					line, _ := reader.ReadString('\n')
					pwd = strings.TrimSpace(line)
				}
				fmt.Println()
				currentPassword = pwd
			}

			deployer := NewDeployer(&stepCfg, proj, pwd)
			if deployErr := deployer.Run(packOnly, targetFilter); deployErr != nil {
				fmt.Fprintf(os.Stderr, "\n❌ Pipeline %q 在步骤 [%d/%d] %s 失败: %v\n",
					pip.Name, i+1, len(pip.Steps), step.Project, deployErr)
				os.Exit(1)
			}
			fmt.Printf("\n✅ 步骤 [%d/%d] %s 完成\n\n", i+1, len(pip.Steps), step.Project)
		}

		fmt.Printf("✅ Pipeline %q 全部完成 (%d 步)\n", pip.Name, len(pip.Steps))
		return
	}

	// CLI 模式：选择项目
	var projectsToDeploy []*ProjectConfig
	if *projectName != "" {
		if *projectName == "all" {
			for _, name := range cfg.ProjectOrder {
				projectsToDeploy = append(projectsToDeploy, cfg.Projects[name])
			}
			if len(projectsToDeploy) == 0 {
				fmt.Fprintf(os.Stderr, "没有配置任何项目\n")
				os.Exit(1)
			}
		} else {
			proj := cfg.GetProject(*projectName)
			if proj == nil {
				fmt.Fprintf(os.Stderr, "项目 %q 不存在，可用项目: %v\n", *projectName, cfg.ProjectNames())
				os.Exit(1)
			}
			projectsToDeploy = append(projectsToDeploy, proj)
		}
	} else {
		proj := cfg.DefaultProject()
		if proj == nil {
			fmt.Fprintf(os.Stderr, "配置了多个项目，请使用 -project 指定:\n")
			for _, name := range cfg.ProjectOrder {
				fmt.Fprintf(os.Stderr, "  - %s\n", name)
			}
			os.Exit(1)
		}
		projectsToDeploy = append(projectsToDeploy, proj)
	}

	currentPassword := *password
	var hasError bool

	for _, proj := range projectsToDeploy {
		if len(projectsToDeploy) > 1 {
			fmt.Printf("==================== 部署: [%s] ====================\n", proj.Name)
		}
		fmt.Printf("项目: [%s]\n", proj.Name)
		fmt.Printf("项目目录: %s\n", proj.ProjectDir)
		fmt.Printf("打包脚本: %s\n", proj.PackScript)
		fmt.Printf("部署目标: %d 个\n", len(proj.Targets))
		for _, t := range proj.Targets {
			fmt.Printf("  - %s (%s) -> %s\n", t.Name, t.Host, t.RemoteDir)
		}
		fmt.Println()

		// 判断是否所有目标都是本机
		allLocal := true
		for _, t := range proj.Targets {
			if !isLocalTarget(t.Host) {
				allLocal = false
				break
			}
		}

		// 解析密码（pack-only 或全部本机部署时跳过）
		pwd := currentPassword
		if pwd == "" && !*packOnly && !allLocal {
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
		if pwd == "" && !*packOnly && !allLocal {
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
			currentPassword = pwd // 记住密码供后续项目使用
		}

		deployer := NewDeployer(cfg, proj, pwd)
		deployErr := deployer.Run(*packOnly, "")

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
			fmt.Fprintf(os.Stderr, "项目 [%s] 部署失败: %v\n", proj.Name, deployErr)
			hasError = true
			break
		}
		if len(projectsToDeploy) > 1 {
			fmt.Printf("\n")
		}
	}

	if hasError {
		os.Exit(1)
	}
}
