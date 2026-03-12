package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"agentbase"
)

func main() {
	configPath := flag.String("config", "execute-code-agent.json", "配置文件路径")
	flag.Parse()

	cfg := LoadConfig(*configPath)

	// 加载 env.json
	envCfg, err := agentbase.LoadEnvConfig(filepath.Dir(*configPath))
	if err != nil {
		log.Printf("[ExecuteCodeAgent] env.json 加载失败: %v", err)
	}

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

	// 启动环境检测（异步，不阻塞 agent 启动）
	if envCfg != nil {
		go startEnvCheck(conn, envCfg)
	}

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
