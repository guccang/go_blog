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
	configFile := flag.String("config", "gateway.json", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := LoadConfig(*configFile)
	if err != nil {
		// 无配置文件时使用默认值
		log.Printf("[Gateway] config file not found (%s), using defaults", *configFile)
		cfg = DefaultConfig()
	}

	log.Printf("[Gateway] starting on port %d", cfg.Port)
	log.Printf("[Gateway] go_blog upstream: %s", cfg.GoBackendURL)

	// 初始化注册表
	registry := NewRegistry()

	// 初始化路由器（包含 UAP server）
	router := NewRouter(cfg, registry)

	// 注册 HTTP 路由
	mux := http.NewServeMux()

	// WebSocket 入口 — agent 连接
	mux.HandleFunc("/ws/uap", router.HandleUAP)

	// 管理 API
	mux.HandleFunc("/api/gateway/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := registry.GetAllAgents()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"agents":  agents,
		})
	})

	mux.HandleFunc("/api/gateway/tools", func(w http.ResponseWriter, r *http.Request) {
		tools := registry.GetAllTools()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"tools":   tools,
		})
	})

	mux.HandleFunc("/api/gateway/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"agents":  registry.OnlineCount(),
		})
	})

	// HTTP 反向代理 — 将其余请求转发到 go_blog
	proxy := NewProxy(cfg.GoBackendURL)
	mux.Handle("/", proxy)

	// 启动心跳检测
	router.StartHealthCheck()

	// 启动 HTTP 服务
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	// 优雅退出
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[Gateway] shutting down...")
		server.Close()
	}()

	log.Printf("[Gateway] listening on :%d", cfg.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[Gateway] server error: %v", err)
	}
	log.Println("[Gateway] stopped")
}
