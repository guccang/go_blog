package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"deploygen"
)

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		allowHeaders := "Content-Type, Authorization, X-App-Agent-Token, X-App-Agent-Session"
		if requested := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")); requested != "" {
			allowHeaders = requested
		}
		w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	configFile := flag.String("config", "app-agent.json", "config file path")
	genConf := flag.Bool("genconf", false, "generate default config")
	genDeploy := flag.Bool("gendeploy", false, "generate deploy scripts")
	flag.Parse()

	if *genConf {
		if err := writeDefaultConfig(*configFile, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "app-agent",
			ConfigFile: "app-agent.json",
			ZipExtras:  []string{"publish.sh"},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*configFile)
	if err != nil {
		log.Printf("[App-Agent] config file not found (%s), using defaults", *configFile)
		cfg = DefaultConfig()
	}

	log.Printf("[App-Agent] starting, HTTP port=%d, gateway=%s", cfg.HTTPPort, cfg.GatewayURL)

	bridge := NewBridge(cfg)
	auth := newAuthManager(cfg)
	handler := NewHandler(cfg, bridge, auth)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/app/login", handler.HandleLogin)
	mux.HandleFunc("/api/app/refresh", handler.HandleRefresh)
	mux.HandleFunc("/api/app/logout", handler.HandleLogout)
	mux.HandleFunc("/api/app/groups", handler.HandleGroups)
	mux.HandleFunc("/api/app/message", handler.HandleMessage)
	mux.HandleFunc("/api/app/upload-apk", handler.HandleUploadAPK)
	mux.HandleFunc("/api/app/attachments/", handler.HandleAttachment)
	mux.HandleFunc("/ws/app", handler.HandleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           "ok",
			"connected":        bridge.IsConnected(),
			"online_clients":   bridge.OnlineClientCount(),
			"pending_messages": bridge.PendingMessageCount(),
		})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: corsMiddleware(mux),
	}

	go bridge.Run()
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			auth.CleanupExpired()
			bridge.cleanupExpiredMessages()
		}
	}()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[App-Agent] shutting down...")
		bridge.Stop()
		_ = server.Close()
	}()

	log.Printf("[App-Agent] HTTP listening on :%d", cfg.HTTPPort)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[App-Agent] server error: %v", err)
	}
	log.Println("[App-Agent] stopped")
}
