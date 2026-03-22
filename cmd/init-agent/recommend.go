package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RecommendResult holds a recommended agent and its match score.
type RecommendResult struct {
	Agent    AgentMeta `json:"agent"`
	Score    int       `json:"score"`
	Matched  []string  `json:"matched_keywords"`
	Installed bool     `json:"installed"`
}

// RecommendAgents matches an intent string against agent feature keywords.
// Returns matching agents sorted by score (highest first).
func RecommendAgents(intent string) []RecommendResult {
	intent = strings.ToLower(intent)
	registry := AgentMetaRegistry()

	var results []RecommendResult

	for _, meta := range registry {
		score := 0
		var matched []string

		for _, kw := range meta.FeatureKeywords {
			if strings.Contains(intent, strings.ToLower(kw)) {
				score++
				matched = append(matched, kw)
			}
		}

		if score > 0 {
			results = append(results, RecommendResult{
				Agent:   meta,
				Score:   score,
				Matched: matched,
			})
		}
	}

	// Sort by score descending, then by tier ascending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score ||
				(results[j].Score == results[i].Score && results[j].Agent.Tier < results[i].Agent.Tier) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// RunRecommendation runs the --recommend CLI mode.
// If intent is empty, it lists all agents grouped by tier.
func RunRecommendation(cfg *InitConfig, intent string) error {
	fmt.Println()

	if intent == "" {
		// Show all agents grouped by tier
		return showAllAgentsByTier(cfg.RootDir)
	}

	// Match intent
	results := RecommendAgents(intent)

	if len(results) == 0 {
		fmt.Printf("  没有匹配 \"%s\" 的 agent\n", intent)
		fmt.Println()
		fmt.Println("  试试以下关键词: 部署, 微信, 定时, 代码执行, AI, 日志, 监控")
		fmt.Println()
		return showAllAgentsByTier(cfg.RootDir)
	}

	fmt.Printf(colorBold("  🔍 推荐结果")+" — \"%s\"\n", intent)
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	for i, r := range results {
		installed := agentHasConfig(cfg.RootDir, r.Agent.Name)
		statusIcon := colorRed("✗")
		if installed {
			statusIcon = colorGreen("✓")
		}

		fmt.Printf("  %d. %s %s %s\n",
			i+1,
			statusIcon,
			colorBold(r.Agent.Name),
			colorDim(fmt.Sprintf("[%s]", r.Agent.Tier)),
		)
		fmt.Printf("     %s\n", r.Agent.ShortPitch)
		fmt.Printf("     匹配: %s\n", colorCyan(strings.Join(r.Matched, ", ")))

		if !installed {
			fmt.Printf("     安装: %s\n", colorCyan(fmt.Sprintf("init-agent --add %s", r.Agent.Name)))
		}

		if r.Agent.AgentDeps != nil {
			fmt.Printf("     依赖: %s\n", strings.Join(r.Agent.AgentDeps, ", "))
		}
		fmt.Println()
	}

	return nil
}

// showAllAgentsByTier displays all agents grouped by tier.
func showAllAgentsByTier(rootDir string) error {
	fmt.Println(colorBold("  📋 所有可用 Agent"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	tierNames := map[AgentTier]string{
		TierCore:         "Tier 0: 基础设施（必须）",
		TierIntelligence: "Tier 1: 智能层（推荐）",
		TierProductivity: "Tier 2: 生产力（按需）",
		TierSpecialized:  "Tier 3: 专业化（可选）",
	}

	byTier := GetAgentsByTier()

	for tier := TierCore; tier <= TierSpecialized; tier++ {
		agents := byTier[tier]
		if len(agents) == 0 {
			continue
		}

		fmt.Printf("  %s\n", colorBold(tierNames[tier]))
		fmt.Println()

		for _, meta := range agents {
			installed := agentHasConfig(rootDir, meta.Name)
			statusIcon := colorDim("○")
			if installed {
				statusIcon = colorGreen("●")
			}

			fmt.Printf("    %s %-24s %s\n",
				statusIcon,
				meta.Name,
				colorDim(meta.ShortPitch),
			)
		}
		fmt.Println()
	}

	fmt.Println("  " + colorDim("● 已配置  ○ 未配置"))
	fmt.Println()
	fmt.Println("  快速启动核心 agent:")
	fmt.Printf("    %s init-agent --quickstart\n", colorCyan("$"))
	fmt.Println()
	fmt.Println("  安装指定 agent:")
	fmt.Printf("    %s init-agent --add <agent-name>\n", colorCyan("$"))
	fmt.Println()

	return nil
}

// RunInteractiveRecommend provides an interactive recommendation prompt.
func RunInteractiveRecommend(cfg *InitConfig) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println(colorBold("  🔍 Agent 推荐"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()
	fmt.Println("  描述你想要的功能，我来推荐合适的 agent")
	fmt.Println("  " + colorDim("例如: 定时发博客, 部署项目, 微信通知, 执行代码"))
	fmt.Println()

	input := promptLine(reader, "  你想做什么", "")
	if input == "" {
		return showAllAgentsByTier(cfg.RootDir)
	}

	return RunRecommendation(cfg, input)
}
