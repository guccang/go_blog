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
	cfgPath := flag.String("config", "cortana-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	genDeploy := flag.Bool("gendeploy", false, "生成部署脚本")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*cfgPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "cortana-agent",
			ConfigFile: "cortana-agent.json",
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[CortanaAgent] 加载配置失败: %v", err)
	}

	agentID := fmt.Sprintf("cortana_agent_%d", os.Getpid())

	log.Printf("[CortanaAgent] ══════════════════════════════════════")
	log.Printf("[CortanaAgent] 启动 cortana-agent")
	log.Printf("[CortanaAgent]   agent_id         = %s", agentID)
	log.Printf("[CortanaAgent]   gateway          = %s", cfg.ServerURL)
	log.Printf("[CortanaAgent]   blog_agent       = %s", cfg.BlogAgentID)
	log.Printf("[CortanaAgent]   cron_agent       = %s", cfg.CronAgentID)
	log.Printf("[CortanaAgent]   audio_agent      = %s", cfg.AudioAgentID)
	log.Printf("[CortanaAgent]   app_agent        = %s", cfg.AppAgentID)
	log.Printf("[CortanaAgent]   agent_name       = %s", cfg.AgentName)
	log.Printf("[CortanaAgent]   check_interval   = %ds", cfg.CheckIntervalSec)
	log.Printf("[CortanaAgent]   broadcast_cooldown = %ds", cfg.BroadcastCooldownSec)
	log.Printf("[CortanaAgent]   broadcasts       = %d", len(cfg.Broadcasts))
	log.Printf("[CortanaAgent] ══════════════════════════════════════")

	conn := NewConnection(cfg, agentID)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("[CortanaAgent] 收到信号 %s，开始优雅关闭...", sig)
		conn.Stop()
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	log.Printf("[CortanaAgent] 开始连接 gateway（阻塞，自动重连）...")
	conn.Run()
}
