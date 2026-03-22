package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"agentbase"
	"deploygen"
)

func main() {
	configPath := flag.String("config", "env-agent.json", "配置文件路径")
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
			AgentName:  "env-agent",
			ConfigFile: "env-agent.json",
			ZipExtras:  []string{"publish.sh"},
			UsePIDFile: true,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg := LoadConfig(*configPath)

	agentID := fmt.Sprintf("env_agent_%d", os.Getpid())

	log.Printf("[EnvAgent] starting agent_id=%s gateway=%s", agentID, cfg.ServerURL)
	log.Printf("[EnvAgent] max_concurrent=%d install_timeout=%ds llm_timeout=%ds llm_agent=%s",
		cfg.MaxConcurrent, cfg.InstallTimeout, cfg.LLMTaskTimeout, cfg.LLMAgentID)

	conn := NewConnection(cfg, agentID)
	conn.ActiveTaskCounter = func() int { return int(atomic.LoadInt32(&conn.activeCount)) }

	// 首次工具目录发现
	if err := conn.DiscoverTools(); err != nil {
		log.Printf("[EnvAgent] initial tool discovery failed (will retry): %v", err)
	}

	// 启动后台工具目录刷新
	conn.StartRefreshLoop()

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("[EnvAgent] received signal, initiating shutdown...")
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	// 启动 gateway 连接（阻塞，自动重连）
	conn.Run()
}
