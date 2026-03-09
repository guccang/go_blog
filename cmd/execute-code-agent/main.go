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
	configPath := flag.String("config", "execute-code-agent.json", "配置文件路径")
	flag.Parse()

	cfg := LoadConfig(*configPath)

	agentID := fmt.Sprintf("exec_code_%d", os.Getpid())

	log.Printf("[ExecuteCodeAgent] starting agent_id=%s gateway=%s python=%s",
		agentID, cfg.ServerURL, cfg.PythonPath)
	log.Printf("[ExecuteCodeAgent] max_concurrent=%d max_exec_time=%ds max_output=%d",
		cfg.MaxConcurrent, cfg.MaxExecTimeSec, cfg.MaxOutputSize)

	conn := NewConnection(cfg, agentID)

	// 首次工具目录发现
	if err := conn.DiscoverTools(); err != nil {
		log.Printf("[ExecuteCodeAgent] initial tool discovery failed (will retry): %v", err)
	}

	// 启动后台工具目录刷新
	conn.StartRefreshLoop()

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("[ExecuteCodeAgent] shutting down...")
		conn.Stop()
		os.Exit(0)
	}()

	// 启动 gateway 连接（阻塞，自动重连）
	conn.Run()
}
