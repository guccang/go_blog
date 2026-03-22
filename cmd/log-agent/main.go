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
)

func main() {
	cfgPath := flag.String("config", "log-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*cfgPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[LogAgent] 加载配置失败: %v", err)
	}

	agentID := fmt.Sprintf("log_query_%d", os.Getpid())

	log.Printf("[LogAgent] starting agent_id=%s gateway=%s sources=%d",
		agentID, cfg.ServerURL, len(cfg.LogSources))

	conn := NewConnection(cfg, agentID)
	conn.ActiveTaskCounter = func() int { return int(atomic.LoadInt32(&conn.activeCount)) }

	// 信号处理
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("[LogAgent] received signal, initiating shutdown...")
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	// 阻塞运行（自动重连）
	conn.Run()
}
