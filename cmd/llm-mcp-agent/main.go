package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfgPath := flag.String("config", "llm-mcp-agent.json", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Printf("[LLM-MCP] config file not found (%s), using defaults", *cfgPath)
		cfg = DefaultConfig()
	}

	log.Printf("[LLM-MCP] starting agent_id=%s, gateway=%s", cfg.AgentID, cfg.GatewayURL)
	log.Printf("[LLM-MCP] LLM model=%s, base_url=%s", cfg.LLM.Model, cfg.LLM.BaseURL)
	log.Printf("[LLM-MCP] concurrency: MaxConcurrent=%d TaskQueueSize=%d MaxParallelSubtasks=%d", cfg.MaxConcurrent, cfg.TaskQueueSize, cfg.MaxParallelSubtasks)

	// 创建 Bridge
	bridge := NewBridge(cfg)

	// 预热 LLM 连接（建立 TCP+TLS，避免首次请求 EOF）
	WarmupLLM(&cfg.LLM)

	// 首次工具发现
	if err := bridge.DiscoverTools(); err != nil {
		log.Printf("[LLM-MCP] initial tool discovery failed (will retry): %v", err)
	}

	// 启动后台工具目录刷新
	bridge.StartRefreshLoop()

	// 启动任务队列消费
	bridge.StartQueueConsumer()

	// 启动微信对话过期清理
	bridge.StartWechatCleanupLoop()

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[LLM-MCP] shutting down...")
		bridge.Stop()
		os.Exit(0)
	}()

	// 启动 gateway 连接（阻塞）
	bridge.Run()
}
