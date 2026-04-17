package main

import (
	"flag"
	"fmt"
	log "mylog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"deploygen"
)

func main() {
	configPath := flag.String("config", "cmd-agent.json", "path to agent config file")
	genConf := flag.Bool("genconf", false, "generate default config file")
	genDeploy := flag.Bool("gendeploy", false, "generate deploy scripts")
	flag.Parse()

	if *genConf {
		if err := writeDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "cmd-agent",
			ConfigFile: "cmd-agent.json",
			ZipExtras:  []string{"workspace/"},
			UsePIDFile: true,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log.Info()
	if err := log.Init(""); err != nil {
		fmt.Fprintf(os.Stderr, "init log: %v\n", err)
	}

	agent, err := NewCMDAgent(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init cmd-agent: %v\n", err)
		os.Exit(1)
	}

	log.MessageF(log.ModuleAgent, "cmd-agent started id=%s gateway=%s", cfg.AgentID, cfg.GatewayURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", agent.HandleHealth)
	mux.HandleFunc("/api/codegen/projects", agent.HandleCodegenProjects)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}
	go func() {
		log.MessageF(log.ModuleAgent, "cmd-agent http listening on :%d", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "cmd-agent http server error: %v\n", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = server.Close()
		agent.Stop()
	}()

	agent.Run()
}
