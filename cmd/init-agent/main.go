package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	mode := flag.String("mode", "cli", "运行模式: cli 或 web")
	port := flag.Int("port", 9090, "Web 模式监听端口")
	root := flag.String("root", "", "monorepo 根目录（自动检测）")
	check := flag.Bool("check", false, "仅运行环境检测")
	dashboard := flag.Bool("dashboard", false, "仅显示可用性面板")
	yes := flag.Bool("yes", false, "非交互模式，接受所有默认值")
	flag.Parse()

	cfg := &InitConfig{
		Mode:           *mode,
		WebPort:        *port,
		RootDir:        *root,
		CheckOnly:      *check,
		DashboardOnly:  *dashboard,
		NonInteractive: *yes,
	}

	if cfg.RootDir == "" {
		detected, err := detectMonorepoRoot()
		if err != nil {
			log.Fatalf("[init-agent] 无法检测 monorepo 根目录: %v", err)
		}
		cfg.RootDir = detected
	}

	fmt.Printf("[init-agent] monorepo 根目录: %s\n", cfg.RootDir)

	if cfg.CheckOnly {
		results := RunEnvironmentChecks()
		PrintCheckResults(results)
		os.Exit(exitCodeFromChecks(results))
		return
	}

	if cfg.DashboardOnly {
		layers := RunAvailabilityChecks(cfg.RootDir, nil)
		PrintAvailabilityDashboard(layers)
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
