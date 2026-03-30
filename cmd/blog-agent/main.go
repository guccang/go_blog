package main

import (
	"auth"
	"blog"
	"codegen"
	"comment"
	"config"
	"control"
	"delegation"
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
	"strings"
	"syscall"
	"time"
	"tools"
	"view"
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
	tools.Info()
	exercise.Info()
	reading.Info()

	// Init
	config.Init(args[1])

	// 初始化 delegation manager 并注册可信代理
	mcp.InitDelegationManager()
	trustedAgents := config.GetTrustedAgents()
	for i := range trustedAgents {
		mcp.GetDelegationManager().RegisterAgent(&delegation.TrustedAgent{
			AgentID:   trustedAgents[i].AgentID,
			SecretKey: trustedAgents[i].SecretKey,
		})
	}
	mcp.GetDelegationManager().StartCleanupRoutine(1 * time.Minute)

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
	mcp.Init()

	// 初始化编码助手模块
	codegen.Init()

	// 注入 MCP 桥接函数到 codegen，避免 codegen 直接依赖 mcp 的重量级传递依赖链
	codegen.MCPCallInnerTools = mcp.CallInnerTools
	codegen.MCPGetToolInfos = func() []codegen.MCPToolInfo {
		tools := mcp.GetInnerMCPTools(nil)
		infos := make([]codegen.MCPToolInfo, 0, len(tools))
		for _, t := range tools {
			name := t.Function.Name
			// 提取回调名（去掉 Inner_blog. 前缀）
			if idx := len("Inner_blog."); len(name) > idx && name[:idx] == "Inner_blog." {
				name = name[idx:]
			}
			// 跳过 Codegen*/Deploy* 工具（由各自的 agent 自注册）
			if strings.HasPrefix(name, "Codegen") || strings.HasPrefix(name, "Deploy") {
				continue
			}
			infos = append(infos, codegen.MCPToolInfo{
				Name:        name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}
		return infos
	}

	// 如果配置了 gateway_url，连接 gateway 注册为 blog-agent agent
	gatewayURL := config.GetConfigWithAccount(account, "gateway_url")
	if gatewayURL != "" {
		gatewayToken := config.GetConfigWithAccount(account, "gateway_token")
		codegen.InitGatewayBridge(gatewayURL, gatewayToken, "workspace")
		log.MessageF(log.ModuleAgent, "Gateway bridge initialized: %s", gatewayURL)
	}

	llm.Init()
	sms.Init()
	exercise.Init()
	share.Init()

	// 注入 AI 路由处理器到 codegen（处理非 cg 命令的微信消息）
	codegen.AIRouteHandler = func(wechatUser, acct, message string) string {
		// 拦截"刷新提示词"命令
		if message == "刷新提示词" || strings.EqualFold(message, "reload prompts") {
			config.ReloadPrompts(acct)
			return "✅ 提示词配置已重新加载"
		}

		// 发送即时确认
		codegen.SendWechatNotify(wechatUser, "⏳ 收到指令，正在处理...")

		messages := []llm.Message{
			{Role: "system", Content: config.SafeSprintf(config.GetPrompt(acct, "wechat_system"), acct)},
			{Role: "user", Content: message},
		}

		result, err := llm.SendSyncLLMRequestWithProgress(messages, acct, func(eventType string, detail string) {})
		if err != nil {
			return fmt.Sprintf("⚠️ AI 处理出错: %v", err)
		}
		if len(result) > 2000 {
			result = result[:2000] + "\n..."
		}
		return result
	}

	// 设置微信命令处理器
	codegen.SetWechatHandler(codegen.HandleWechatCommand)

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
