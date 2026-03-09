package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"uap"
)

func main() {
	configPath := flag.String("config", "codegen-agent.conf", "path to agent config file")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	agentID := "codegen_" + uap.NewMsgID()
	log.Printf("[INFO] CodeGen Agent starting: id=%s name=%s type=%s", agentID, cfg.AgentName, cfg.AgentType)
	log.Printf("[INFO] Gateway: %s → go_blog-agent: %s", cfg.ServerURL, cfg.GoBackendAgentID)
	log.Printf("[INFO] Workspaces: %v", cfg.Workspaces)
	log.Printf("[INFO] MaxConcurrent: %d, MaxTurns: %d", cfg.MaxConcurrent, cfg.MaxTurns)

	agent := NewAgent(agentID, cfg)

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

	// 启动协议层（注册 + 心跳）
	go conn.StartProtocolLayer()

	// 阻塞运行（自动重连）
	conn.Run()
}
