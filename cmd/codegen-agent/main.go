package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"agentbase"
	"deploygen"
	"uap"
)

func main() {
	configPath := flag.String("config", "codegen-agent.json", "path to agent config file")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	genDeploy := flag.Bool("gendeploy", false, "生成部署脚本")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "codegen-agent",
			ConfigFile: "codegen-agent.json",
			ZipExtras:  []string{"settings/"},
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

	// 加载 env.json
	envCfg, err := agentbase.LoadEnvConfig(filepath.Dir(*configPath))
	if err != nil {
		log.Printf("[INFO] env.json 加载失败: %v", err)
	}

	agentID := "codegen_" + uap.NewMsgID()
	log.Printf("[INFO] CodeGen Agent starting: id=%s name=%s type=%s", agentID, cfg.AgentName, cfg.AgentType)
	log.Printf("[INFO] Gateway: %s → go_blog-agent: %s", cfg.ServerURL, cfg.GoBackendAgentID)
	log.Printf("[INFO] Workspaces: %v", cfg.Workspaces)
	log.Printf("[INFO] MaxConcurrent: %d, MaxTurns: %d", cfg.MaxConcurrent, cfg.MaxTurns)

	agent := NewAgent(agentID, cfg)

	conn := NewConnection(cfg, agent)
	conn.ActiveTaskCounter = func() int { return agent.ActiveCount() }

	// 启动环境检测（异步，不阻塞 agent 启动）
	if envCfg != nil && len(envCfg.Requirements) > 0 {
		gatewayHTTP := envCfg.GatewayHTTP
		catalog := agentbase.NewToolCatalog(gatewayHTTP)
		rc := agentbase.NewRemoteCaller(conn.AgentBase, catalog)
		// 注册 MsgToolResult handler 给 RemoteCaller
		conn.RegisterHandler(uap.MsgToolResult, func(msg *uap.Message) {
			var payload uap.ToolResultPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return
			}
			rc.DispatchToolResult(&payload)
		})
		go agentbase.NewEnvChecker(conn.AgentBase, catalog, rc, envCfg, nil).Run()
	}

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("[INFO] received signal, initiating shutdown...")
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	// 启动协议层（注册 + 心跳）
	go conn.StartProtocolLayer()

	// 阻塞运行（自动重连）
	conn.Run()
}
