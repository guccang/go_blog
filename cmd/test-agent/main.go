package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	configPath := flag.String("config", "test-agent.json", "配置文件路径")
	suitePath := flag.String("suite", "", "测试套件 JSON 路径")
	scenarioID := flag.String("scenario", "", "只执行指定场景 ID")
	genConf := flag.Bool("genconf", false, "生成默认配置文件")
	flag.Parse()

	if *genConf {
		if err := WriteDefaultConfig(*configPath, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "write config: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	runner := NewRunner(cfg)
	defer runner.Close()

	if *suitePath != "" {
		resolvedSuite := *suitePath
		if !filepath.IsAbs(resolvedSuite) {
			resolvedSuite = filepath.Join(cfg.SuiteDir, resolvedSuite)
		}
		suite, err := LoadSuite(resolvedSuite)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load suite: %v\n", err)
			os.Exit(1)
		}

		report, err := runner.RunSuite(context.Background(), suite, *scenarioID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run suite: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("suite=%s status=%s total=%d passed=%d failed=%d avg_score=%d\n",
			report.SuiteID, report.Status, report.TotalScenarios, report.PassedScenarios, report.FailedScenarios, report.AverageScore)
		for _, run := range report.Runs {
			fmt.Printf("scenario=%s status=%s score=%d final=%s\n",
				run.ScenarioID, run.Status, run.Result.Scores.Total, firstNonEmpty(run.Result.FinalStatus, run.Result.FinalError))
		}
		return
	}

	final, err := runner.RunEvaluation(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "run evaluation: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("evaluation=%s status=%s overall_score=%d online_agents=%d\n",
		final.RunID, final.Status, final.OverallScore, len(final.OnlineAgents))
	if final.StaticReport != nil {
		fmt.Printf("static status=%s executed=%d passed=%d failed=%d skipped=%d avg=%d\n",
			final.StaticReport.Status,
			final.StaticReport.ExecutedScenarios,
			final.StaticReport.PassedScenarios,
			final.StaticReport.FailedScenarios,
			final.StaticReport.SkippedScenarios,
			final.StaticReport.AverageScore)
	}
	if final.DynamicReport != nil {
		fmt.Printf("dynamic status=%s executed=%d passed=%d failed=%d skipped=%d avg=%d\n",
			final.DynamicReport.Status,
			final.DynamicReport.ExecutedScenarios,
			final.DynamicReport.PassedScenarios,
			final.DynamicReport.FailedScenarios,
			final.DynamicReport.SkippedScenarios,
			final.DynamicReport.AverageScore)
	}
}
