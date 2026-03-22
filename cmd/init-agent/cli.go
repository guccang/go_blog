package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ANSI color helpers
func colorGreen(s string) string  { return "\033[32m" + s + "\033[0m" }
func colorYellow(s string) string { return "\033[33m" + s + "\033[0m" }
func colorRed(s string) string    { return "\033[31m" + s + "\033[0m" }
func colorCyan(s string) string   { return "\033[36m" + s + "\033[0m" }
func colorBold(s string) string   { return "\033[1m" + s + "\033[0m" }
func colorDim(s string) string    { return "\033[2m" + s + "\033[0m" }

// RunCLIWizard runs the interactive CLI wizard.
func RunCLIWizard(cfg *InitConfig) error {
	reader := bufio.NewReader(os.Stdin)
	state := NewWizardState(cfg.RootDir)

	// Fixed steps 1-2 (Welcome + GlobalConfig)
	fixedSteps := []struct {
		step WizardStep
		fn   func(*InitConfig, *WizardState, *bufio.Reader) error
	}{
		{StepWelcome, cliStepWelcome},
		{StepGlobalConfig, cliStepGlobalConfig},
	}

	// Determine deploy steps (conditional)
	deployAvailable := state.DeployState != nil && state.DeployState.Available
	var deploySteps []struct {
		step WizardStep
		fn   func(*InitConfig, *WizardState, *bufio.Reader) error
	}
	if deployAvailable {
		deploySteps = []struct {
			step WizardStep
			fn   func(*InitConfig, *WizardState, *bufio.Reader) error
		}{
			{StepDeployTargets, cliStepDeployTargets},
			{StepDeployProjects, cliStepDeployProjects},
			{StepDeployPipelines, cliStepDeployPipelines},
		}
	}

	// Steps: fixed(2) + deploy(0-3) + envCheck(1) + agentSelect(1) + agents(N) + configGen(1) + avail(1)
	numDeploySteps := len(deploySteps)
	preAgentSteps := 2 + numDeploySteps + 1 + 1 // fixed + deploy + envCheck + agentSelect

	// We don't know numAgents yet, calculate after agentSelect
	stepNum := 0

	// Run fixed steps 1-2
	for _, s := range fixedSteps {
		stepNum++
		state.CurrentStep = s.step
		printStepHeader(stepNum, stepNum+3, s.step.String()) // placeholder total
		if err := s.fn(cfg, state, reader); err != nil {
			return err
		}
		fmt.Println()
	}

	// Run deploy steps (if available)
	for _, s := range deploySteps {
		stepNum++
		state.CurrentStep = s.step
		printStepHeader(stepNum, stepNum+3, s.step.String())
		if err := s.fn(cfg, state, reader); err != nil {
			return err
		}
		fmt.Println()
	}

	// EnvCheck step (after deploy steps, before agent select)
	stepNum++
	state.CurrentStep = StepEnvCheck
	printStepHeader(stepNum, stepNum+3, StepEnvCheck.String())
	if err := cliStepEnvCheck(cfg, state, reader); err != nil {
		return err
	}
	fmt.Println()

	// Agent select step
	stepNum++
	state.CurrentStep = StepAgentSelect
	printStepHeader(stepNum, stepNum+3, StepAgentSelect.String())
	if err := cliStepAgentSelect(cfg, state, reader); err != nil {
		return err
	}
	fmt.Println()

	// Now we know how many agents are selected — recalculate
	numAgents := len(state.SelectedAgents)
	totalSteps := preAgentSteps + numAgents + 2 // +configGen +avail

	// Per-agent configuration
	state.CurrentStep = StepAgentConfig
	for i, agentName := range state.SelectedAgents {
		info := state.GetDiscoveredConfig(agentName)
		if info == nil {
			continue
		}
		agentStepNum := preAgentSteps + 1 + i
		stepLabel := fmt.Sprintf("Agent 配置 (%d/%d) — %s", i+1, numAgents, agentName)
		printStepHeader(agentStepNum, totalSteps, stepLabel)

		if err := cliStepSingleAgent(cfg, state, reader, agentName, *info, i+1, numAgents); err != nil {
			return err
		}
		fmt.Println()
	}

	// Config generation
	configGenStepNum := preAgentSteps + numAgents + 1
	state.CurrentStep = StepConfigGenerate
	printStepHeader(configGenStepNum, totalSteps, state.CurrentStep.String())
	if err := cliStepConfigGenerate(cfg, state, reader); err != nil {
		return err
	}
	fmt.Println()

	// Availability
	availStepNum := preAgentSteps + numAgents + 2
	state.CurrentStep = StepAvailability
	printStepHeader(availStepNum, totalSteps, state.CurrentStep.String())
	if err := cliStepAvailability(cfg, state, reader); err != nil {
		return err
	}
	fmt.Println()

	fmt.Println(colorGreen("  ✓ 初始化向导完成！"))
	fmt.Println()
	return nil
}

func printStepHeader(current, total int, name string) {
	bar := ""
	for i := 1; i <= total; i++ {
		if i == current {
			bar += colorCyan(fmt.Sprintf("[%d]", i))
		} else if i < current {
			bar += colorGreen(fmt.Sprintf("[%d]", i))
		} else {
			bar += colorDim(fmt.Sprintf("[%d]", i))
		}
		if i < total {
			bar += "─"
		}
	}
	fmt.Printf("\n  %s  %s\n", bar, colorBold(name))
	fmt.Println("  " + strings.Repeat("─", 50))
}

// Step 1: Welcome
func cliStepWelcome(_ *InitConfig, state *WizardState, _ *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  欢迎使用 Go Blog Monorepo 初始化向导！")
	fmt.Println()
	fmt.Printf("  项目根目录: %s\n", colorCyan(state.RootDir))

	agents, err := listAgentDirs(state.RootDir)
	if err != nil {
		fmt.Printf("  %s 无法列出 agent 目录: %v\n", colorYellow("!"), err)
	} else {
		fmt.Printf("  发现 %d 个 agent 目录: %s\n", len(agents), strings.Join(agents, ", "))
	}

	if len(state.DiscoveredConfigs) > 0 {
		names := make([]string, len(state.DiscoveredConfigs))
		for i, dc := range state.DiscoveredConfigs {
			names[i] = dc.Name
		}
		fmt.Printf("  发现 %d 个已有配置: %s\n", len(state.DiscoveredConfigs), strings.Join(names, ", "))
	}

	return nil
}

// Step: Environment check (runs after deploy pipelines to leverage target info)
func cliStepEnvCheck(_ *InitConfig, state *WizardState, _ *bufio.Reader) error {
	fmt.Println()

	// 有 deploy 配置且有 pipeline 数据时，按 target 分组做远程检测
	ds := state.DeployState
	hasPipelines := ds != nil && ds.Available && len(ds.Pipelines) > 0

	if hasPipelines {
		fmt.Println("  正在根据部署编排检测各目标机器环境...")
		fmt.Println()

		// 从 pipeline 推导每台机器的检测需求
		plans := DeriveTargetRequirements(ds.Pipelines, ds.Targets, ds.Projects)

		if len(plans) == 0 {
			fmt.Printf("  %s 无法从 pipeline 数据推导检测计划，回退本机检测\n", colorYellow("!"))
			fmt.Println()
			return cliStepEnvCheckLocal(state)
		}

		// 获取 SSH 凭证
		sshPassword := ds.SSHPassword
		sshKeyPath := GetSSHKeyPath(state.RootDir)

		// 执行检测
		state.TargetEnvResults = RunTargetChecks(plans, sshPassword, sshKeyPath)

		// 打印结果
		PrintTargetCheckResults(state.TargetEnvResults)
	} else {
		// 无 deploy 配置时，保持现有逻辑：本机通用检测
		fmt.Println("  正在检测环境依赖...")
		fmt.Println()
		return cliStepEnvCheckLocal(state)
	}

	return nil
}

// cliStepEnvCheckLocal 本机通用环境检测（向下兼容）
func cliStepEnvCheckLocal(state *WizardState) error {
	state.EnvResults = RunEnvironmentChecks()
	PrintCheckResults(state.EnvResults)

	// Count issues
	missing := 0
	outdated := 0
	for _, r := range state.EnvResults {
		if !r.Installed {
			missing++
		} else if !r.MeetsRequirement {
			outdated++
		}
	}

	if missing > 0 {
		fmt.Printf("  %s %d 项未安装（建议安装后继续）\n", colorYellow("!"), missing)
	}
	if outdated > 0 {
		fmt.Printf("  %s %d 项版本不满足要求\n", colorYellow("!"), outdated)
	}
	if missing == 0 && outdated == 0 {
		fmt.Printf("  %s 所有依赖均已满足\n", colorGreen("✓"))
	}

	return nil
}

// Step 3: Global config
func cliStepGlobalConfig(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  配置全局参数（将自动传播到所有 agent）")
	fmt.Println()

	// Use values from init-agent.json as defaults, fallback to hardcoded
	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = "ws://127.0.0.1:10086/ws/uap"
	}
	gatewayHTTP := cfg.GatewayHTTP
	if gatewayHTTP == "" {
		gatewayHTTP = "http://127.0.0.1:10086"
	}
	authToken := cfg.AuthToken

	fields := []struct {
		key      string
		label    string
		defVal   string
		desc     string
	}{
		{"server_url", "Gateway WebSocket URL", serverURL, "UAP WebSocket 连接地址"},
		{"gateway_http", "Gateway HTTP URL", gatewayHTTP, "Gateway HTTP API 地址"},
		{"auth_token", "Auth Token", authToken, "Gateway 认证令牌（可空）"},
	}

	for _, f := range fields {
		if cfg.NonInteractive {
			state.SharedValues[f.key] = f.defVal
		} else {
			val := promptLine(reader, f.label, f.defVal)
			state.SharedValues[f.key] = val
		}
	}

	fmt.Println()
	fmt.Println("  已设置全局参数:")
	for k, v := range state.SharedValues {
		display := v
		if display == "" {
			display = colorDim("(空)")
		}
		fmt.Printf("    %s = %s\n", k, display)
	}

	return nil
}

// Step 4: Agent selection (based on discovered configs)
func cliStepAgentSelect(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	fmt.Println()

	discovered := state.DiscoveredConfigs
	if len(discovered) == 0 {
		fmt.Printf("  %s 没有发现任何 agent 配置文件，请先在 cmd/*/  目录下创建 JSON 配置\n", colorYellow("!"))
		return nil
	}

	fmt.Println("  选择要配置的 agent（输入编号，逗号分隔，或 'all' 选择全部）")
	fmt.Println("  " + colorDim("仅显示已有 JSON 配置文件的 agent"))
	fmt.Println()

	for i, dc := range discovered {
		fieldCount := len(dc.Values)
		fmt.Printf("    %s %-25s %s\n",
			colorCyan(fmt.Sprintf("[%2d]", i+1)),
			dc.Name,
			colorDim(fmt.Sprintf("(%d 个字段, %s)", fieldCount, dc.ConfigPath)),
		)
	}

	fmt.Println()

	if cfg.NonInteractive {
		for _, dc := range discovered {
			state.SelectedAgents = append(state.SelectedAgents, dc.Name)
		}
		fmt.Println("  非交互模式: 已选择全部 agent")
	} else {
		input := promptLine(reader, "选择 (如 1,2,3 或 all)", "all")
		if strings.ToLower(input) == "all" {
			for _, dc := range discovered {
				state.SelectedAgents = append(state.SelectedAgents, dc.Name)
			}
		} else {
			parts := strings.Split(input, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				idx, err := strconv.Atoi(p)
				if err != nil || idx < 1 || idx > len(discovered) {
					fmt.Printf("  %s 忽略无效编号: %s\n", colorYellow("!"), p)
					continue
				}
				state.SelectedAgents = append(state.SelectedAgents, discovered[idx-1].Name)
			}
		}
	}

	fmt.Printf("\n  已选择 %d 个 agent: %s\n",
		len(state.SelectedAgents),
		strings.Join(state.SelectedAgents, ", "))

	return nil
}

// Step 5 (per-agent): Single agent configuration from discovered JSON
func cliStepSingleAgent(cfg *InitConfig, state *WizardState, reader *bufio.Reader, agentName string, info AgentConfigInfo, idx, total int) error {
	fmt.Println()
	fmt.Printf("  %s %s\n", colorCyan("▸"), colorBold(agentName))
	fmt.Printf("    %s\n", colorDim(info.ConfigPath))
	fmt.Println()

	// Shared key mappings: global shared values → agent JSON keys
	sharedKeyMap := map[string]string{
		"server_url":   "server_url",
		"gateway_url":  "server_url",
		"gateway_http": "gateway_http",
		"auth_token":   "auth_token",
	}

	// Work on a copy of the values
	values := make(map[string]any)
	for k, v := range info.Values {
		values[k] = v
	}

	keys := SortedKeys(values)
	firstField := true

	for _, key := range keys {
		origVal := values[key]
		fieldType := InferFieldType(origVal)
		displayVal := FormatValueForDisplay(origVal)

		// Apply global shared value as recommended default if applicable
		if sharedKey, mapped := sharedKeyMap[key]; mapped {
			if sv, ok := state.SharedValues[sharedKey]; ok && sv != "" {
				displayVal = sv
			}
		}
		// Also check direct match with shared values
		if sv, ok := state.SharedValues[key]; ok && sv != "" {
			displayVal = sv
		}

		typeHint := colorDim(fmt.Sprintf("(%s)", fieldType))
		if fieldType == "object" {
			// For complex objects, show as JSON and allow editing
			fmt.Printf("    %s %s %s\n", colorDim("·"), key, typeHint)
			fmt.Printf("      当前值: %s\n", colorDim(displayVal))
			if !cfg.NonInteractive {
				fmt.Printf("      %s\n", colorDim("(输入新 JSON 或回车保留现有值)"))
			}
		}

		if cfg.NonInteractive {
			values[key] = ParseInputValue(displayVal, origVal)
			continue
		}

		label := fmt.Sprintf("    %s %s", key, typeHint)
		input := promptLine(reader, label, displayVal)

		// First field: check for "skip" to skip the entire agent
		if firstField && strings.ToLower(strings.TrimSpace(input)) == "skip" {
			fmt.Printf("\n  %s 跳过 %s\n", colorYellow("→"), agentName)
			state.SkippedAgents = append(state.SkippedAgents, agentName)
			// Remove from selected
			newSelected := make([]string, 0, len(state.SelectedAgents))
			for _, s := range state.SelectedAgents {
				if s != agentName {
					newSelected = append(newSelected, s)
				}
			}
			state.SelectedAgents = newSelected
			return nil
		}
		firstField = false

		values[key] = ParseInputValue(input, origVal)
	}

	state.GeneratedConfigs[agentName] = values
	fmt.Printf("\n  %s %s 配置已暂存\n", colorGreen("✓"), agentName)

	return nil
}

// Step: Config generation
func cliStepConfigGenerate(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  即将生成以下配置文件:")
	fmt.Println()

	// Preview deploy config files
	if state.DeployState != nil && state.DeployState.Available {
		ds := state.DeployState
		if len(ds.Targets) > 0 {
			path := filepath.Join(ds.SettingsDir, "targets.json")
			fmt.Printf("    %s %s %s\n", colorCyan("·"), path, colorYellow("(deploy targets)"))
		}
		for _, name := range ds.ProjectOrder {
			path := filepath.Join(ds.SettingsDir, "projects", name+".json")
			fmt.Printf("    %s %s\n", colorCyan("·"), path)
		}
		for _, p := range ds.Pipelines {
			path := filepath.Join(ds.SettingsDir, "pipelines", p.Name+".json")
			fmt.Printf("    %s %s\n", colorCyan("·"), path)
		}
		if ds.SSHPassword != "" {
			path := filepath.Join(state.RootDir, "cmd", "deploy-agent", "deploy-agent.json")
			fmt.Printf("    %s %s %s\n", colorCyan("·"), path, colorYellow("(ssh_password)"))
		}
		fmt.Println()
	}

	for _, agentName := range state.SelectedAgents {
		values := state.GeneratedConfigs[agentName]
		if values == nil {
			continue
		}

		// Determine path from discovered config or schema
		var path string
		if info := state.GetDiscoveredConfig(agentName); info != nil {
			path = filepath.Join(state.RootDir, info.ConfigPath)
		} else if schema := GetAgentSchema(agentName); schema != nil {
			path = filepath.Join(state.RootDir, schema.Dir, schema.ConfigFileName)
		} else {
			continue
		}

		exists := ""
		if FileExists(path) {
			exists = colorYellow(" (将覆盖)")
		}
		fmt.Printf("    %s %s%s\n", colorCyan("·"), path, exists)

		// Show preview
		preview := PreviewDiscoveredConfig(values)
		fmt.Println(colorDim(preview))
		fmt.Println()
	}

	if !cfg.NonInteractive {
		if !promptYesNo(reader, "  确认写入配置文件？", true) {
			fmt.Println("  已取消配置生成")
			return nil
		}
	}

	if err := state.WriteAllConfigs(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  %s 已写入 %d 个配置文件:\n", colorGreen("✓"), len(state.WrittenFiles))
	for _, f := range state.WrittenFiles {
		fmt.Printf("    %s %s\n", colorGreen("·"), f)
	}

	return nil
}

// Step: Availability check
func cliStepAvailability(_ *InitConfig, state *WizardState, _ *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  正在运行可用性检测...")
	fmt.Println()

	// 有 deploy 配置且有 pipeline 数据时，执行 pipeline 分组检测
	ds := state.DeployState
	hasPipelines := ds != nil && ds.Available && len(ds.Pipelines) > 0

	if hasPipelines {
		sshPassword := ds.SSHPassword
		sshKeyPath := GetSSHKeyPath(state.RootDir)
		state.PipelineAvailResults = RunPipelineAvailChecks(ds, state.TargetEnvResults, sshPassword, sshKeyPath)
	}

	state.AvailabilityLayers = RunAvailabilityChecks(state.RootDir, state.GeneratedConfigs, state.PipelineAvailResults)
	PrintAvailabilityDashboard(state.AvailabilityLayers, state.PipelineAvailResults)

	return nil
}

// CLI prompt helpers (adapted from deploy-agent/init.go)

func promptLine(reader *bufio.Reader, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	suffix := " [Y/n]: "
	if !defaultYes {
		suffix = " [y/N]: "
	}
	fmt.Printf("%s%s", prompt, suffix)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}
