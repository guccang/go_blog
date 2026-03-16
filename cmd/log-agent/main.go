package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfgPath := flag.String("config", "log-agent.json", "配置文件路径")
	flag.Parse()

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[LogAgent] 加载配置失败: %v", err)
	}

	agentID := fmt.Sprintf("log_query_%d", os.Getpid())

	log.Printf("[LogAgent] starting agent_id=%s gateway=%s sources=%d",
		agentID, cfg.ServerURL, len(cfg.LogSources))

	conn := NewConnection(cfg, agentID)

	// 信号处理
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("[LogAgent] shutting down...")
		conn.Stop()
		os.Exit(0)
	}()

	// 阻塞运行（自动重连）
	conn.Run()
}
