package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	configPath := flag.String("config", "init-agent.json", "配置文件路径")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	mode := flag.String("mode", "", "运行模式: cli 或 web")
	port := flag.Int("port", 0, "Web 模式监听端口")
	root := flag.String("root", "", "monorepo 根目录（自动检测）")
	check := flag.Bool("check", false, "仅运行环境检测")
	dashboard := flag.Bool("dashboard", false, "仅显示可用性面板")
	yes := flag.Bool("yes", false, "非交互模式，接受所有默认值")

	// Progressive deployment flags
	quickstart := flag.Bool("quickstart", false, "快速启动模式（仅配置 gateway + blog-agent）")
	addAgents := flag.String("add", "", "增量安装 agent（逗号分隔，如 llm-agent,corn-agent）")
	recommend := flag.String("recommend", "", "根据意图推荐 agent（如 \"定时发博客\"）")
	listAgents := flag.Bool("list", false, "列出所有可用 agent 及其状态")

	flag.Parse()

	if *genConf {
		if err := writeDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	// Track which flags were explicitly set on the command line
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// Load config from JSON file (if exists)
	cfg := &InitConfig{
		Mode:    "cli",
		WebPort: 9090,
	}
	if fileCfg, err := LoadInitConfig(*configPath); err == nil {
		fmt.Printf("[init-agent] 已加载配置文件: %s\n", *configPath)
		cfg = fileCfg
	} else if explicitFlags["config"] {
		// User explicitly specified a config file but it failed to load
		log.Fatalf("[init-agent] 无法加载配置文件 %s: %v", *configPath, err)
	}

	// CLI flags override JSON values (only when explicitly set)
	if explicitFlags["mode"] {
		cfg.Mode = *mode
	}
	if explicitFlags["port"] {
		cfg.WebPort = *port
	}
	if explicitFlags["root"] {
		cfg.RootDir = *root
	}
	if explicitFlags["check"] {
		cfg.CheckOnly = *check
	}
	if explicitFlags["dashboard"] {
		cfg.DashboardOnly = *dashboard
	}
	if explicitFlags["yes"] {
		cfg.NonInteractive = *yes
	}

	// Apply defaults for zero values
	if cfg.Mode == "" {
		cfg.Mode = "cli"
	}
	if cfg.WebPort == 0 {
		cfg.WebPort = 9090
	}

	if cfg.RootDir == "" {
		detected, err := detectMonorepoRoot()
		if err != nil {
			log.Fatalf("[init-agent] 无法检测 monorepo 根目录: %v", err)
		}
		cfg.RootDir = detected
	}

	fmt.Printf("[init-agent] monorepo 根目录: %s\n", cfg.RootDir)

	// --- Progressive deployment modes (take priority) ---

	if *quickstart {
		if err := RunQuickStartWizard(cfg); err != nil {
			log.Fatalf("[init-agent] 快速启动失败: %v", err)
		}
		return
	}

	if *addAgents != "" {
		names := parseAgentList(*addAgents)
		if len(names) == 0 {
			log.Fatalf("[init-agent] --add 参数不能为空")
		}
		if err := RunAddAgentWizard(cfg, names); err != nil {
			log.Fatalf("[init-agent] 增量安装失败: %v", err)
		}
		return
	}

	if explicitFlags["recommend"] {
		if err := RunRecommendation(cfg, *recommend); err != nil {
			log.Fatalf("[init-agent] 推荐失败: %v", err)
		}
		return
	}

	if *listAgents {
		if err := RunRecommendation(cfg, ""); err != nil {
			log.Fatalf("[init-agent] 列出 agent 失败: %v", err)
		}
		return
	}

	// --- Original modes ---

	if cfg.CheckOnly {
		results := RunEnvironmentChecks()
		PrintCheckResults(results)
		os.Exit(exitCodeFromChecks(results))
		return
	}

	if cfg.DashboardOnly {
		layers := RunAvailabilityChecks(cfg.RootDir, nil, nil)
		PrintAvailabilityDashboard(layers, nil)
		os.Exit(exitCodeFromLayers(layers))
		return
	}

	switch cfg.Mode {
	case "cli":
		if err := RunCLIWizard(cfg); err != nil {
			log.Fatalf("[init-agent] 向导失败: %v", err)
		}
	case "web":
		if err := RunWebServer(cfg); err != nil {
			log.Fatalf("[init-agent] Web 服务器失败: %v", err)
		}
	default:
		log.Fatalf("[init-agent] 未知模式: %s (可选: cli, web)", cfg.Mode)
	}
}

// parseAgentList splits a comma-separated agent list, trimming whitespace.
func parseAgentList(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func exitCodeFromChecks(results []SoftwareCheckResult) int {
	for _, r := range results {
		if !r.Installed || !r.MeetsRequirement {
			return 1
		}
	}
	return 0
}

func exitCodeFromLayers(layers []LayerStatus) int {
	for _, l := range layers {
		if l.Status == StatusRed {
			return 1
		}
	}
	return 0
}
