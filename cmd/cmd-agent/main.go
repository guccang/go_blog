package main

import (
	"codegen"
	"config"
	"flag"
	"fmt"
	log "mylog"
	"os"
	"os/signal"
	"syscall"

	"deploygen"
)

func main() {
	configPath := flag.String("config", "cmd-agent.json", "path to agent config file")
	genConf := flag.Bool("genconf", false, "generate default config file")
	genDeploy := flag.Bool("gendeploy", false, "generate deploy scripts")
	flag.Parse()

	if *genConf {
		if err := writeDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "cmd-agent",
			ConfigFile: "cmd-agent.json",
			ZipExtras:  []string{"workspace/"},
			UsePIDFile: true,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log.Info()
	// cmd-agent 不依赖 sys_conf.md，最小化初始化 config 仅用于兼容 codegen 默认依赖。
	config.InitManager("")
	if err := log.Init(""); err != nil {
		fmt.Fprintf(os.Stderr, "init log: %v\n", err)
	}

	codegen.Init()
	codegen.ClaudeCodeSystemPromptBuilder = func() string { return claudeCodeSystemPrompt }
	codegen.OpenCodeSystemPromptBuilder = func() string { return openCodeSystemPrompt }
	codegen.AIRouteHandler = func(_, _, _ string) string {
		return "⚠️ cmd-agent 只处理 /cg 命令"
	}
	codegen.InitGatewayBridgeWithOptions(cfg.GatewayURL, cfg.AuthToken, codegen.GatewayBridgeOptions{
		AgentID:      cfg.AgentID,
		AgentType:    "cmd",
		AgentName:    cfg.AgentName,
		Description:  "统一处理 /cg 命令并转发到 codegen/deploy agent",
		WorkspaceDir: cfg.WorkspaceDir,
	})
	// cmd-agent 只保留会话状态，不直接推微信，进度统一回到来源客户端。
	codegen.InitWeChatBridge(nil)
	codegen.SetWechatHandler(codegen.HandleWechatCommand)

	log.MessageF(log.ModuleAgent, "cmd-agent started id=%s gateway=%s", cfg.AgentID, cfg.GatewayURL)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	if client := codegen.GetGatewayClient(); client != nil {
		client.Stop()
	}
	log.Message(log.ModuleAgent, "cmd-agent shutting down")
}
