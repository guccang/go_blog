package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"deploygen"
	"downloadticket"
	"obsstore"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		allowHeaders := "Content-Type, Authorization, X-App-Agent-Token, X-Download-Ticket"
		if requested := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")); requested != "" {
			allowHeaders = requested
		}
		w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	configFile := flag.String("config", "obs-agent.json", "config file path")
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
			AgentName:  "obs-agent",
			ConfigFile: "obs-agent.json",
			ZipExtras:  []string{"publish.sh"},
			UsePIDFile: true,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*configFile)
	if err != nil {
		log.Printf("[obs-agent] config file not found (%s), using defaults", *configFile)
		cfg = DefaultConfig()
	}
	store, err := obsstore.New(cfg.OBS)
	if err != nil {
		log.Fatalf("[obs-agent] init obs store failed: %v", err)
	}
	if !store.Enabled() {
		log.Fatalf("[obs-agent] obs is not configured")
	}
	signer := downloadticket.NewSigner(cfg.DownloadTicketSecret)
	if !signer.Enabled() {
		log.Fatalf("[obs-agent] download_ticket_secret is required")
	}

	handler := NewHandler(cfg, store, signer)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/obs/upload", handler.HandleUpload)
	mux.HandleFunc("/api/obs/proxy-upload", handler.HandleProxyUpload)
	mux.HandleFunc("/api/obs/list", handler.HandleList)
	mux.HandleFunc("/api/obs/delete", handler.HandleDelete)
	mux.HandleFunc("/api/obs/info", handler.HandleObjectInfo)
	mux.HandleFunc("/api/obs/download/", handler.HandleDownload)
	mux.HandleFunc("/health", handler.HandleHealth)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: corsMiddleware(mux),
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[obs-agent] shutting down...")
		_ = server.Close()
	}()

	log.Printf("[obs-agent] HTTP listening on :%d", cfg.HTTPPort)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[obs-agent] server error: %v", err)
	}
}
