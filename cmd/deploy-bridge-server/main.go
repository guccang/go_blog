package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed static
var staticFS embed.FS

func main() {
	configPath := "bridge-server.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	manager := NewDeployManager(cfg)
	handlers := NewHandlers(cfg, manager)

	// API 路由（需要认证）
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/upload", handlers.HandleUpload)
	apiMux.HandleFunc("/api/packages", handlers.HandlePackages)
	apiMux.HandleFunc("/api/deploy", handlers.HandleDeploy)
	apiMux.HandleFunc("/api/deploys", handlers.HandleDeploys)
	// /api/deploy/{id}/logs 通过前缀匹配
	apiMux.HandleFunc("/api/deploy/", handlers.HandleDeployLogs)

	authedAPI := authMiddleware(cfg.AuthToken, apiMux)

	// 主路由
	mux := http.NewServeMux()
	mux.Handle("/api/", authedAPI)

	// 静态文件
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	fmt.Printf("deploy-bridge-server listening on %s\n", cfg.Listen)
	fmt.Printf("  upload_dir: %s\n", cfg.UploadDir)
	fmt.Printf("  max_upload: %dMB\n", cfg.MaxUploadSizeMB)
	fmt.Printf("  deploy_timeout: %ds\n", cfg.DeployTimeout)

	if err := http.ListenAndServe(cfg.Listen, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
