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
	configPath := flag.String("config", "audio-agent.json", "path to agent config file")
	genConf := flag.Bool("genconf", false, "generate default config file")
	genDeploy := flag.Bool("gendeploy", false, "generate deploy files")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "audio-agent",
			ConfigFile: "audio-agent.json",
			ZipExtras:  []string{"publish.sh"},
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

	agentID := fmt.Sprintf("audio_agent_%d", os.Getpid())
	log.Printf("[AudioAgent] starting agent_id=%s gateway=%s", agentID, cfg.ServerURL)

	conn := NewConnection(cfg, agentID)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("[AudioAgent] received signal, initiating shutdown...")
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	conn.Run()
}
