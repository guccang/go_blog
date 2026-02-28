package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
)

func main() {
	configPath := flag.String("config", "agent.conf", "path to agent config file")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	agentID := "agent_" + uuid.New().String()[:8]
	log.Printf("[INFO] CodeGen Agent starting: id=%s name=%s", agentID, cfg.AgentName)
	log.Printf("[INFO] Server: %s", cfg.ServerURL)
	log.Printf("[INFO] Workspaces: %v", cfg.Workspaces)
	log.Printf("[INFO] MaxConcurrent: %d, MaxTurns: %d", cfg.MaxConcurrent, cfg.MaxTurns)

	agent := NewAgent(agentID, cfg)

	// 初始化 pipeline（deploy + verify）
	if cfg.DeployAgentPath != "" {
		agent.pipeline = &Pipeline{
			deployPath:    cfg.DeployAgentPath,
			deployConfig:  cfg.DeployAgentConfig,
			verifyURL:     cfg.VerifyURL,
			verifyTimeout: cfg.VerifyTimeout,
		}
		log.Printf("[INFO] Pipeline enabled: deploy=%s, verify=%s", cfg.DeployAgentPath, cfg.VerifyURL)
	}

	conn := NewConnection(cfg, agent)

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("[INFO] shutting down...")
		conn.Stop()
		os.Exit(0)
	}()

	// 阻塞运行（自动重连）
	conn.Run()
}
