package main

import (
	"flag"
	"fmt"
	log "mylog"
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		agent.Stop()
	}()

	agent.Run()
}
