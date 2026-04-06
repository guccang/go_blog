package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- QuickStart Mode (--quickstart) ---

// RunQuickStartWizard runs a minimal 3-step wizard to configure gateway + blog-agent only.
func RunQuickStartWizard(cfg *InitConfig) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println(colorBold("  ⚡ Go Blog 快速启动"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	// Step 1: Welcome + detect root
	fmt.Printf("  项目根目录: %s\n", colorCyan(cfg.RootDir))

	agents, err := listAgentDirs(cfg.RootDir)
	if err == nil {
		fmt.Printf("  发现 %d 个 agent 目录\n", len(agents))
	}
	fmt.Println()

	// Step 2: Core config (4 questions)
	fmt.Println(colorBold("  Step 1/3 ─ 核心配置"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	gatewayPort := "10086"
	redisIP := "127.0.0.1"
	redisPort := "6379"
	admin := "admin"

	if cfg.NonInteractive {
		fmt.Println("  非交互模式: 使用默认值")
	} else {
		gatewayPort = promptLine(reader, "    Gateway 端口", gatewayPort)
		redisIP = promptLine(reader, "    Redis 地址", redisIP)
		redisPort = promptLine(reader, "    Redis 端口", redisPort)
		admin = promptLine(reader, "    管理员账号", admin)
	}

	fmt.Println()

	// Derive shared values
	serverURL := fmt.Sprintf("ws://127.0.0.1:%s/ws/uap", gatewayPort)
	goBackendURL := "http://127.0.0.1:8080"

	// Step 3: Generate configs
	fmt.Println(colorBold("  Step 2/3 ─ 生成配置"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	// Build gateway config
	gatewayValues := map[string]any{
		"port":              parsePortOrDefault(gatewayPort, 10086),
		"go_backend_url":    goBackendURL,
		"auth_token":        "",
		"event_tracking":    true,
		"event_buffer_size": 10000,
		"event_log_dir":     "logs",
		"event_log_stdout":  true,
	}

	// Build blog-agent config (key=value format)
	blogValues := map[string]any{
		"admin":         admin,
		"port":          redisPort, // blog-agent port is 8080 by default
		"redis_ip":      redisIP,
		"redis_port":    redisPort,
		"redis_pwd":     "",
		"gateway_url":   serverURL,
		"gateway_token": "",
		"logs_dir":      "",
	}
	// Fix: blog-agent port should be 8080, redis_port is separate
	blogValues["port"] = "8080"

	// Write gateway.json
	state := NewWizardState(cfg.RootDir)
	state.GeneratedConfigs["gateway"] = gatewayValues
	state.GeneratedConfigs["blog-agent"] = blogValues
	state.SelectedAgents = []string{"gateway", "blog-agent"}

	// Preview
	fmt.Println("  将生成以下配置:")
	fmt.Println()

	gatewaySchema := GetAgentSchema("gateway")
	if gatewaySchema != nil {
		path := filepath.Join(cfg.RootDir, gatewaySchema.Dir, gatewaySchema.ConfigFileName)
		fmt.Printf("    %s %s\n", colorCyan("·"), path)
	}
	blogSchema := GetAgentSchema("blog-agent")
	if blogSchema != nil {
		path := filepath.Join(cfg.RootDir, blogSchema.Dir, blogSchema.ConfigFileName)
		fmt.Printf("    %s %s\n", colorCyan("·"), path)
	}
	fmt.Println()

	if !cfg.NonInteractive {
		if !promptYesNo(reader, "  确认写入？", true) {
			fmt.Println("  已取消")
			return nil
		}
	}

	if err := state.WriteAllConfigs(); err != nil {
		return fmt.Errorf("写入配置失败: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s 已写入 %d 个配置文件\n", colorGreen("✓"), len(state.WrittenFiles))
	for _, f := range state.WrittenFiles {
		fmt.Printf("    %s %s\n", colorGreen("·"), f)
	}

	// Step 3: Availability + next steps
	fmt.Println()
	fmt.Println(colorBold("  Step 3/3 ─ 下一步"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	fmt.Println("  启动核心 agent:")
	fmt.Printf("    %s cd cmd/gateway && go run .\n", colorCyan("$"))
	fmt.Printf("    %s cd cmd/blog-agent && go run .\n", colorCyan("$"))
	fmt.Println()

	fmt.Println("  想要 AI 功能？运行:")
	fmt.Printf("    %s init-agent --add llm-agent\n", colorCyan("$"))
	fmt.Println()

	fmt.Println("  想要定时任务？运行:")
	fmt.Printf("    %s init-agent --add cron-agent\n", colorCyan("$"))
	fmt.Println()

	fmt.Println("  想要项目部署？运行:")
	fmt.Printf("    %s init-agent --add deploy-agent\n", colorCyan("$"))
	fmt.Println()

	fmt.Println("  查看所有可用 agent:")
	fmt.Printf("    %s init-agent --recommend\n", colorCyan("$"))
	fmt.Println()

	fmt.Println("  运行完整向导:")
	fmt.Printf("    %s init-agent --mode cli\n", colorCyan("$"))
	fmt.Println()

	fmt.Println(colorGreen("  ✓ 快速启动完成！"))
	return nil
}

// parsePortOrDefault parses a port string, returns the default on failure.
func parsePortOrDefault(s string, def int) int {
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
	}
	val := 0
	for _, c := range s {
		val = val*10 + int(c-'0')
	}
	if val <= 0 || val > 65535 {
		return def
	}
	return val
}

// --- Add Agent Mode (--add) ---

// RunAddAgentWizard configures one or more agents incrementally.
// It resolves dependencies and only configures agents that don't already have config files.
func RunAddAgentWizard(cfg *InitConfig, agentNames []string) error {
	reader := bufio.NewReader(os.Stdin)
	state := NewWizardState(cfg.RootDir)

	fmt.Println()
	fmt.Println(colorBold("  📦 增量安装 Agent"))
	fmt.Println("  " + strings.Repeat("─", 50))
	fmt.Println()

	// Resolve full dependency chain
	allNeeded, err := resolveAllDeps(agentNames)
	if err != nil {
		return err
	}

	// Separate into already-configured and needs-configuration
	var alreadyConfigured []string
	var needConfig []string

	for _, name := range allNeeded {
		if agentHasConfig(cfg.RootDir, name) {
			alreadyConfigured = append(alreadyConfigured, name)
		} else {
			needConfig = append(needConfig, name)
		}
	}

	// Report status
	if len(alreadyConfigured) > 0 {
		fmt.Printf("  已配置 %s: %s\n",
			colorGreen("✓"),
			strings.Join(alreadyConfigured, ", "))
	}

	if len(needConfig) == 0 {
		fmt.Println()
		fmt.Printf("  %s 所有请求的 agent 已配置完成，无需额外操作\n", colorGreen("✓"))
		return nil
	}

	fmt.Printf("  待配置: %s\n", colorCyan(strings.Join(needConfig, ", ")))
	fmt.Println()

	// Show dependency explanation
	requested := make(map[string]bool)
	for _, n := range agentNames {
		requested[n] = true
	}
	depOnly := []string{}
	for _, n := range needConfig {
		if !requested[n] {
			depOnly = append(depOnly, n)
		}
	}
	if len(depOnly) > 0 {
		fmt.Printf("  %s 以下 agent 作为依赖自动添加: %s\n",
			colorYellow("→"), strings.Join(depOnly, ", "))
		fmt.Println()
	}

	// Set up shared values from existing config or init-agent.json
	if cfg.ServerURL != "" {
		state.SharedValues["server_url"] = cfg.ServerURL
	}
	if cfg.GatewayHTTP != "" {
		state.SharedValues["gateway_http"] = cfg.GatewayHTTP
	}
	if cfg.AuthToken != "" {
		state.SharedValues["auth_token"] = cfg.AuthToken
	}

	// Configure each agent
	for i, agentName := range needConfig {
		info := state.GetDiscoveredConfig(agentName)
		if info == nil {
			// No existing JSON config found; check schema and create from defaults
			schema := GetAgentSchema(agentName)
			if schema == nil {
				fmt.Printf("  %s 跳过未知 agent: %s\n", colorYellow("!"), agentName)
				continue
			}

			// Create AgentConfigInfo from schema defaults
			info = &AgentConfigInfo{
				Name:       agentName,
				Dir:        schema.Dir,
				ConfigPath: schema.Dir + "/" + schema.ConfigFileName,
				Values:     buildDefaultValues(schema),
			}
		}

		stepLabel := fmt.Sprintf("配置 (%d/%d) — %s", i+1, len(needConfig), agentName)
		fmt.Printf("  %s\n", colorBold(stepLabel))

		meta := GetAgentMeta(agentName)
		if meta != nil {
			fmt.Printf("  %s\n", colorDim(meta.ShortPitch))
		}
		fmt.Println()

		if err := cliStepSingleAgent(cfg, state, reader, agentName, *info, i+1, len(needConfig)); err != nil {
			return err
		}
		fmt.Println()
	}

	// Check if anything was configured (user might have skipped all)
	if len(state.GeneratedConfigs) == 0 {
		fmt.Printf("  %s 没有配置任何 agent\n", colorYellow("!"))
		return nil
	}

	// Write configs
	state.SelectedAgents = needConfig
	if !cfg.NonInteractive {
		fmt.Println()
		if !promptYesNo(reader, "  确认写入配置文件？", true) {
			fmt.Println("  已取消")
			return nil
		}
	}

	if err := state.WriteAllConfigs(); err != nil {
		return fmt.Errorf("写入配置失败: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s 已写入 %d 个配置文件:\n", colorGreen("✓"), len(state.WrittenFiles))
	for _, f := range state.WrittenFiles {
		fmt.Printf("    %s %s\n", colorGreen("·"), f)
	}

	// Print start commands
	fmt.Println()
	fmt.Println("  启动命令:")
	for _, name := range needConfig {
		if _, ok := state.GeneratedConfigs[name]; !ok {
			continue
		}
		schema := GetAgentSchema(name)
		if schema != nil {
			fmt.Printf("    %s cd %s && go run .\n", colorCyan("$"), schema.Dir)
		}
	}
	fmt.Println()

	fmt.Println(colorGreen("  ✓ 增量安装完成！"))
	return nil
}

// resolveAllDeps resolves all dependencies for the given agent names recursively.
// Returns a topologically sorted list (dependencies before dependents).
func resolveAllDeps(names []string) ([]string, error) {
	registry := AgentMetaRegistry()

	// Validate all requested names exist
	for _, name := range names {
		if _, ok := registry[name]; !ok {
			available := make([]string, 0, len(registry))
			for k := range registry {
				available = append(available, k)
			}
			return nil, fmt.Errorf("未知 agent: %s\n  可用 agent: %s", name, strings.Join(available, ", "))
		}
	}

	// Collect all needed agents via BFS
	needed := make(map[string]bool)
	queue := make([]string, len(names))
	copy(queue, names)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if needed[current] {
			continue
		}
		needed[current] = true

		if meta, ok := registry[current]; ok {
			for _, dep := range meta.AgentDeps {
				if !needed[dep] {
					queue = append(queue, dep)
				}
			}
		}
	}

	// Topological sort: dependencies first
	var sorted []string
	visited := make(map[string]bool)

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		if meta, ok := registry[name]; ok {
			for _, dep := range meta.AgentDeps {
				if needed[dep] {
					visit(dep)
				}
			}
		}
		sorted = append(sorted, name)
	}

	for name := range needed {
		visit(name)
	}

	return sorted, nil
}

// agentHasConfig checks if an agent already has a configuration file.
func agentHasConfig(rootDir, agentName string) bool {
	// Check discovered JSON config
	configs := DiscoverAgentConfigs(rootDir)
	for _, c := range configs {
		if c.Name == agentName {
			return true
		}
	}

	// Check schema-based config
	schema := GetAgentSchema(agentName)
	if schema != nil {
		path := filepath.Join(rootDir, schema.Dir, schema.ConfigFileName)
		return FileExists(path)
	}

	return false
}

// buildDefaultValues creates a default values map from an AgentSchema.
func buildDefaultValues(schema *AgentSchema) map[string]any {
	values := make(map[string]any)
	for _, field := range schema.Fields {
		if field.DefaultValue != nil {
			values[field.Key] = field.DefaultValue
		} else {
			values[field.Key] = ""
		}
	}
	return values
}
