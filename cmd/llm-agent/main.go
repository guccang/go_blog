package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agentbase"
	"deploygen"
)

func main() {
	cfgPath := flag.String("config", "llm-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	genDeploy := flag.Bool("gendeploy", false, "生成部署脚本")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*cfgPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "llm-agent",
			ConfigFile: "llm-agent.json",
			ZipExtras:  []string{"publish.sh", "workspace/"},
			StartArgs:  "--config llm-agent.json",
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

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

	// 启动时工具评估（异步，不阻塞启动）
	if cfg.ToolEvalOnStartup {
		go bridge.EvaluateTools()
	}

	// 启动后台工具目录刷新
	bridge.StartRefreshLoop()

	// 启动任务队列消费
	bridge.StartQueueConsumer()

	// 恢复未过期的聊天会话
	bridge.sessionMgr.LoadAll()

	// 启动会话过期清理
	bridge.StartSessionCleanupLoop()

	// 恢复中断的任务
	bridge.RecoverInProgressTasks()

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
