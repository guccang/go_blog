package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	"agentbase"
	"deploygen"
	"uap"
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
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	genDeploy := flag.Bool("gendeploy", false, "生成部署脚本")
	flag.Parse()

	if *genConf {
		if err := agentbase.WriteDefaultConfig(*configPathFlag, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *genDeploy {
		if err := deploygen.GenerateDeployFiles(deploygen.DeployOptions{
			AgentName:  "mcp-agent",
			ConfigFile: "mcp-agent.json",
			ZipExtras:  []string{"publish.sh"},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

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
	description := generateDescription(tools)

	// 创建连接
	currentConn = NewConnection(cfg, agentID, mcpMgr, cfgPath, description)
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

	description := generateDescription(tools)
	newConn := NewConnection(newCfg, oldConn.AgentID, mcpMgr, cfgPath, description)
	newConn.Client.Tools = tools
	currentConn = newConn

	// 在新 goroutine 中启动新连接
	go currentConn.Run()

	log.Printf("[MCPAgent] reload complete: %d tools registered", len(tools))
}

// generateDescription 根据已发现的工具动态生成 Agent 描述
func generateDescription(tools []uap.ToolDef) string {
	categories := make(map[string]bool)
	for _, t := range tools {
		name := strings.ToLower(t.Name)
		switch {
		// 天气
		case strings.Contains(name, "weather"):
			categories["天气查询"] = true
		// 路线规划
		case strings.Contains(name, "cycling"), strings.Contains(name, "walking"),
			strings.Contains(name, "driving"), strings.Contains(name, "transit"),
			strings.Contains(name, "direction"), strings.Contains(name, "distance"):
			categories["路线规划"] = true
		// 地址解析
		case strings.Contains(name, "geocode"), strings.Contains(name, "geo"),
			strings.Contains(name, "regeo"), strings.Contains(name, "ip_location"):
			categories["地址解析"] = true
		// 地点搜索
		case strings.Contains(name, "search"), strings.Contains(name, "poi"),
			strings.Contains(name, "detail"):
			categories["地点搜索"] = true
		// 出行服务（专属地图、导航、打车）
		case strings.Contains(name, "schema"), strings.Contains(name, "navigate"),
			strings.Contains(name, "navi"), strings.Contains(name, "ride"),
			strings.Contains(name, "taxi"), strings.Contains(name, "personal_map"):
			categories["出行服务"] = true
		}
	}

	// 按固定顺序拼接
	ordered := []string{"天气查询", "路线规划", "地址解析", "地点搜索", "出行服务"}
	var active []string
	for _, cat := range ordered {
		if categories[cat] {
			active = append(active, cat)
		}
	}

	if len(active) == 0 {
		return fmt.Sprintf("MCP 桥接服务 (%d 个工具)", len(tools))
	}
	return fmt.Sprintf("高德地图服务：%s (%d 工具)", strings.Join(active, "、"), len(tools))
}
