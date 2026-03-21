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

	steps := []func(*InitConfig, *WizardState, *bufio.Reader) error{
		cliStepWelcome,
		cliStepEnvCheck,
		cliStepGlobalConfig,
		cliStepAgentSelect,
		cliStepAgentConfig,
		cliStepConfigGenerate,
		cliStepAvailability,
	}

	for i, step := range steps {
		state.CurrentStep = WizardStep(i)
		printStepHeader(i+1, len(steps), state.CurrentStep.String())
		if err := step(cfg, state, reader); err != nil {
			return err
		}
		fmt.Println()
	}

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
		fmt.Printf("  发现 %d 个 agent: %s\n", len(agents), strings.Join(agents, ", "))
	}

	return nil
}

// Step 2: Environment check
func cliStepEnvCheck(_ *InitConfig, state *WizardState, _ *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  正在检测环境依赖...")
	fmt.Println()

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

	fields := []struct {
		key      string
		label    string
		defVal   string
		desc     string
	}{
		{"server_url", "Gateway WebSocket URL", "ws://127.0.0.1:10086/ws/uap", "UAP WebSocket 连接地址"},
		{"gateway_http", "Gateway HTTP URL", "http://127.0.0.1:10086", "Gateway HTTP API 地址"},
		{"auth_token", "Auth Token", "", "Gateway 认证令牌（可空）"},
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

// Step 4: Agent selection
func cliStepAgentSelect(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  选择要配置的 agent（输入编号，逗号分隔，或 'all' 选择全部）")
	fmt.Println()

	schemas := state.AllSchemas
	for i, s := range schemas {
		status := ""
		if state.AgentHasExistingConfig(&s) {
			status = colorGreen(" [已有配置]")
		}
		fmt.Printf("    %s %-25s %s%s\n",
			colorCyan(fmt.Sprintf("[%2d]", i+1)),
			s.Name,
			colorDim(s.Description),
			status,
		)
	}

	fmt.Println()

	if cfg.NonInteractive {
		// Select all agents
		for _, s := range schemas {
			state.SelectedAgents = append(state.SelectedAgents, s.Name)
		}
		fmt.Println("  非交互模式: 已选择全部 agent")
	} else {
		input := promptLine(reader, "选择 (如 1,2,3 或 all)", "all")
		if strings.ToLower(input) == "all" {
			for _, s := range schemas {
				state.SelectedAgents = append(state.SelectedAgents, s.Name)
			}
		} else {
			parts := strings.Split(input, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				idx, err := strconv.Atoi(p)
				if err != nil || idx < 1 || idx > len(schemas) {
					fmt.Printf("  %s 忽略无效编号: %s\n", colorYellow("!"), p)
					continue
				}
				state.SelectedAgents = append(state.SelectedAgents, schemas[idx-1].Name)
			}
		}
	}

	fmt.Printf("\n  已选择 %d 个 agent: %s\n",
		len(state.SelectedAgents),
		strings.Join(state.SelectedAgents, ", "))

	return nil
}

// Step 5: Per-agent config
func cliStepAgentConfig(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	fmt.Println()

	for _, agentName := range state.SelectedAgents {
		schema := GetAgentSchema(agentName)
		if schema == nil {
			continue
		}

		fmt.Printf("\n  %s %s\n", colorCyan("▸"), colorBold(schema.Name))
		fmt.Printf("    %s\n", colorDim(schema.Description))

		// Load existing config for defaults
		existing, _ := LoadExistingConfig(state.RootDir, schema)

		agentVals := make(map[string]string)
		nonSharedFields := GetNonSharedFields(schema)

		for _, field := range nonSharedFields {
			// Skip complex types in CLI for simplicity
			if field.Type == FieldMap {
				fmt.Printf("    %s %s: %s\n", colorDim("·"), field.Label, colorDim("(跳过，请手动编辑配置文件)"))
				continue
			}

			// Determine default: existing > schema default
			defVal := GetDefaultValueString(field)
			if existing != nil {
				if v, ok := existing[field.Key]; ok {
					defVal = fmt.Sprintf("%v", v)
				}
			}

			if cfg.NonInteractive {
				agentVals[field.Key] = defVal
			} else {
				label := fmt.Sprintf("    %s", field.Label)
				if field.Required {
					label += colorRed("*")
				}
				val := promptLine(reader, label, defVal)
				if val != "" {
					if err := ValidateField(field, val); err != nil {
						fmt.Printf("    %s %v (使用输入值)\n", colorYellow("!"), err)
					}
				}
				agentVals[field.Key] = val
			}
		}

		state.AgentValues[agentName] = agentVals
		state.MergeAndStoreConfig(schema)
	}

	return nil
}

// Step 6: Config generation
func cliStepConfigGenerate(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  即将生成以下配置文件:")
	fmt.Println()

	for _, agentName := range state.SelectedAgents {
		schema := GetAgentSchema(agentName)
		if schema == nil {
			continue
		}
		path := filepath.Join(state.RootDir, schema.Dir, schema.ConfigFileName)
		exists := ""
		if FileExists(path) {
			exists = colorYellow(" (将覆盖)")
		}
		fmt.Printf("    %s %s%s\n", colorCyan("·"), path, exists)

		// Show preview
		values := state.GeneratedConfigs[agentName]
		if values != nil {
			preview := PreviewConfig(schema, values)
			fmt.Println(colorDim(preview))
		}
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

// Step 7: Availability check
func cliStepAvailability(_ *InitConfig, state *WizardState, _ *bufio.Reader) error {
	fmt.Println()
	fmt.Println("  正在运行可用性检测...")
	fmt.Println()

	state.AvailabilityLayers = RunAvailabilityChecks(state.RootDir, state.GeneratedConfigs)
	PrintAvailabilityDashboard(state.AvailabilityLayers)

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
