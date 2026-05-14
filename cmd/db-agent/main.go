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
	configPath := flag.String("config", "db-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "db-agent - 数据库存储代理\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n")
		fmt.Fprintf(os.Stderr, "  db-agent [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "  -config <path>  配置文件路径（默认 db-agent.json）\n")
		fmt.Fprintf(os.Stderr, "  -genconf         生成默认配置文件\n")
	}

	flag.Parse()

	if *genConf {
		if err := generateDefaultConfig(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "生成配置文件失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "配置文件中未设置 server_url，请先配置 gateway 地址\n")
		os.Exit(1)
	}

	fmt.Printf("DB Agent\n")
	fmt.Printf("驱动: %s\n", cfg.Driver)
	fmt.Printf("服务地址: %s\n", cfg.ServerURL)
	fmt.Printf("Agent名称: %s\n", cfg.AgentName)
	fmt.Printf("最大并发: %d\n\n", cfg.MaxConcurrent)

	agentID := fmt.Sprintf("db_%s_%d", cfg.AgentName, os.Getpid())
	conn, err := NewConnection(cfg, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建连接失败: %v\n", err)
		os.Exit(1)
	}

	// 设置生命周期回调
	conn.ActiveTaskCounter = func() int { return 0 }

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("[INFO] received signal, initiating shutdown...")
		conn.InitiateShutdown("signal")
		conn.Stop()
		os.Exit(0)
	}()

	// 启动环境检测（如果有 env.json）
	if cfg.DataDir != "" {
		envCfg, envErr := agentbase.LoadEnvConfig(cfg.DataDir)
		if envErr == nil && envCfg != nil && cfg.ServerURL != "" {
			gatewayHTTP := envCfg.GatewayHTTP
			catalog := agentbase.NewToolCatalog(gatewayHTTP)
			rc := agentbase.NewRemoteCaller(conn.AgentBase, catalog)
			go agentbase.NewEnvChecker(conn.AgentBase, catalog, rc, envCfg, nil).Run()
		}
	}

	conn.Run()
}
