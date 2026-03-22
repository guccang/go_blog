package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configFile := flag.String("config", "wechat-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	flag.Parse()

	if *genConf {
		if err := writeDefaultConfig(*configFile, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	// 加载配置
	cfg, err := LoadConfig(*configFile)
	if err != nil {
		log.Printf("[WeChat-Agent] config file not found (%s), using defaults", *configFile)
		cfg = DefaultConfig()
	}

	log.Printf("[WeChat-Agent] starting, HTTP port=%d, gateway=%s", cfg.HTTPPort, cfg.GatewayURL)

	// 创建消息桥接器
	bridge := NewBridge(cfg)

	// 创建回调处理器
	handler := NewHandler(cfg, bridge)

	// 启动 HTTP 服务（接收微信回调）
	mux := http.NewServeMux()
	mux.HandleFunc("/api/wechat/callback", handler.HandleCallback)

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"connected": bridge.IsConnected(),
		})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	// 启动 gateway 连接（后台）
	go bridge.Run()

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[WeChat-Agent] shutting down...")
		bridge.Stop()
		server.Close()
	}()

	log.Printf("[WeChat-Agent] HTTP listening on :%d", cfg.HTTPPort)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[WeChat-Agent] server error: %v", err)
	}
	log.Println("[WeChat-Agent] stopped")
}
