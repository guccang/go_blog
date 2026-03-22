package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

)

var (
	// 全局状态，热加载时更新
	currentConn *Connection
	currentCfg  *Config
	mcpMgr      *MCPManager
	cfgPath     string
)

func main() {
	configPathFlag := flag.String("config", "mcp-agent.json", "配置文件路径")
	flag.Parse()
	cfgPath = *configPathFlag

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("[MCPAgent] 加载配置失败: %v", err)
	}
	currentCfg = cfg

	agentID := fmt.Sprintf("mcp_bridge_%d", os.Getpid())

	log.Printf("[MCPAgent] starting agent_id=%s gateway=%s", agentID, cfg.ServerURL)
	log.Printf("[MCPAgent] tool_prefix=%s timeout=%ds servers=%d",
		cfg.ToolPrefix, cfg.ToolCallTimeoutSec, len(cfg.MCPServers))

	// 创建 MCP 管理器
	mcpMgr = NewMCPManager(cfg.ToolPrefix)

	// 逐个启动 MCP Server
	for name, serverCfg := range cfg.MCPServers {
		if err := mcpMgr.StartServer(name, serverCfg); err != nil {
			log.Printf("[MCPAgent] start server %s failed: %v (will continue)", name, err)
		}
	}

	// 构建工具列表
	tools := mcpMgr.BuildUAPTools()

	// 创建连接
	currentConn = NewConnection(cfg, agentID, mcpMgr, cfgPath)
	currentConn.Client.Tools = tools
	currentConn.ActiveTaskCounter = func() int { return int(atomic.LoadInt32(&currentConn.activeCount)) }

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP:
				log.Println("[MCPAgent] received SIGHUP, reloading config...")
				handleReload()
			case os.Interrupt, syscall.SIGTERM:
				log.Println("[MCPAgent] received signal, initiating shutdown...")
				mcpMgr.StopAll()
				currentConn.InitiateShutdown("signal")
				os.Exit(0)
			}
		}
	}()

	// 阻塞运行（自动重连）
	currentConn.Run()
}

// handleReload SIGHUP 热加载处理
func handleReload() {
	newCfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Printf("[MCPAgent] reload config failed: %v", err)
		return
	}

	added, removed, changed := DiffServers(currentCfg.MCPServers, newCfg.MCPServers)

	// 停止已移除和已变更的 server
	for _, name := range removed {
		mcpMgr.StopServer(name)
	}
	for _, name := range changed {
		mcpMgr.StopServer(name)
	}

	// 启动新增和已变更的 server
	for _, name := range added {
		if err := mcpMgr.StartServer(name, newCfg.MCPServers[name]); err != nil {
			log.Printf("[MCPAgent] start new server %s failed: %v", name, err)
		}
	}
	for _, name := range changed {
		if err := mcpMgr.StartServer(name, newCfg.MCPServers[name]); err != nil {
			log.Printf("[MCPAgent] restart server %s failed: %v", name, err)
		}
	}

	// 重建工具列表并更新 gateway 注册
	tools := mcpMgr.BuildUAPTools()
	currentConn.Client.Tools = tools

	// 更新配置引用
	currentCfg = newCfg
	currentConn.cfg = newCfg

	// 断开重连，让 gateway 获取新工具列表
	// Stop 会关闭 stopCh，需要重建 Connection
	oldConn := currentConn
	oldConn.Stop()

	newConn := NewConnection(newCfg, oldConn.AgentID, mcpMgr, cfgPath)
	newConn.Client.Tools = tools
	currentConn = newConn

	// 在新 goroutine 中启动新连接
	go currentConn.Run()

	log.Printf("[MCPAgent] reload complete: %d tools registered", len(tools))
}
