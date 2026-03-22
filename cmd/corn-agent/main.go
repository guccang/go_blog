package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agentbase"
)

func main() {
	configPath := flag.String("config", "corn-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg := LoadConfig(*configPath)

	agentID := fmt.Sprintf("corn_agent_%d", os.Getpid())
	if cfg.AgentID != "" {
		agentID = cfg.AgentID
	}

	log.Printf("[CornAgent] 启动 agent_id=%s gateway=%s", agentID, cfg.GatewayURL)
	log.Printf("[CornAgent] 存储: %s (%s)", cfg.Storage.Type, cfg.Storage.Path)
	log.Printf("[CornAgent] 调度器: 最大并发=%d 时区=%s", cfg.Scheduler.MaxConcurrent, cfg.Scheduler.Timezone)
	log.Printf("[CornAgent] llm-agent: %s 超时=%ds", cfg.LLMAgent.AgentID, cfg.LLMAgent.Timeout)
	log.Printf("[CornAgent] 通知: enabled=%v channel=%s", cfg.Notifications.Enabled, cfg.Notifications.Channel)

	conn := NewConnection(cfg, agentID)
	conn.ActiveTaskCounter = func() int { return conn.scheduler.RunningCount() }
	conn.OnShutdown = func() {
		log.Printf("[CornAgent] OnShutdown: 停止调度器，不再触发新任务")
		conn.scheduler.Stop()
	}

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("[CornAgent] 收到信号，开始优雅关闭...")
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	// 启动 gateway 连接（阻塞，自动重连）
	conn.Run()
}