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
	cfgPath := flag.String("config", "cron-agent.json", "配置文件路径")
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
			AgentName:  "cron-agent",
			ConfigFile: "cron-agent.json",
			ZipExtras:  []string{"publish.sh"},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[CronAgent] 加载配置失败: %v", err)
	}

	agentID := fmt.Sprintf("cron_agent_%d", os.Getpid())

	log.Printf("[CronAgent] ══════════════════════════════════════")
	log.Printf("[CronAgent] 启动 cron-agent")
	log.Printf("[CronAgent]   agent_id   = %s", agentID)
	log.Printf("[CronAgent]   gateway    = %s", cfg.ServerURL)
	log.Printf("[CronAgent]   llm_agent  = %s", cfg.LLMAgentID)
	log.Printf("[CronAgent]   task_file  = %s", cfg.TaskFile)
	log.Printf("[CronAgent]   agent_name = %s", cfg.AgentName)
	log.Printf("[CronAgent] ══════════════════════════════════════")

	conn := NewConnection(cfg, agentID)
	conn.ActiveTaskCounter = func() int { return conn.engine.PendingCount() }

	// 信号处理
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("[CronAgent] 收到信号 %s，开始优雅关闭...", sig)
		conn.engine.Stop()
		conn.InitiateShutdown("signal")
		os.Exit(0)
	}()

	log.Printf("[CronAgent] 开始连接 gateway（阻塞，自动重连）...")
	// 阻塞运行（自动重连）
	conn.Run()
}
