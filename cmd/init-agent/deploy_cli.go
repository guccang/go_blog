package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ── Step 4: Deploy Targets ──

func cliStepDeployTargets(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	ds := state.DeployState
	fmt.Println()
	fmt.Println("  管理部署目标（SSH/Bridge 服务器）")
	fmt.Println()

	for {
		// Display current targets
		names := SortedTargetNames(ds.Targets)
		if len(names) > 0 {
			fmt.Println("  当前 targets:")
			for i, name := range names {
				t := ds.Targets[name]
				info := formatTargetInfo(t)
				fmt.Printf("    %s %-20s %s\n",
					colorCyan(fmt.Sprintf("[%2d]", i+1)),
					name,
					colorDim(info),
				)
			}
			fmt.Println()
		} else {
			fmt.Printf("  %s 暂无 target\n\n", colorDim("(空)"))
		}

		if cfg.NonInteractive {
			break
		}

		input := promptLine(reader, "  操作: [A]添加 [E]编辑 [D]删除 [回车]继续", "")
		input = strings.ToLower(strings.TrimSpace(input))

		if input == "" {
			break
		}

		switch {
		case input == "a":
			cliAddTarget(ds, reader)
		case strings.HasPrefix(input, "e"):
			idx := parseIndexArg(input, len(names))
			if idx < 0 {
				idx = promptIndex(reader, "  编辑编号", len(names))
			}
			if idx >= 0 {
				cliEditTarget(ds, names[idx], reader)
			}
		case strings.HasPrefix(input, "d"):
			idx := parseIndexArg(input, len(names))
			if idx < 0 {
				idx = promptIndex(reader, "  删除编号", len(names))
			}
			if idx >= 0 {
				name := names[idx]
				delete(ds.Targets, name)
				fmt.Printf("  %s 已删除 %s\n\n", colorGreen("✓"), name)
			}
		default:
			fmt.Printf("  %s 无效操作\n", colorYellow("!"))
		}
	}

	// SSH password
	fmt.Println()
	if cfg.NonInteractive {
		// keep existing password
	} else {
		pw := promptLine(reader, "  SSH 密码 (全局，存入 deploy-agent.json)", ds.SSHPassword)
		ds.SSHPassword = pw
	}

	return nil
}

func cliAddTarget(ds *DeployConfigState, reader *bufio.Reader) {
	fmt.Println()
	name := promptLine(reader, "    名称 (如 ssh-prod, ssh-staging)", "")
	if name == "" {
		fmt.Printf("    %s 名称不能为空\n", colorYellow("!"))
		return
	}
	if _, exists := ds.Targets[name]; exists {
		fmt.Printf("    %s target %q 已存在\n", colorYellow("!"), name)
		return
	}

	t := DeployTarget{}
	t.Host = promptLine(reader, "    Host (user@ip)", "")
	portStr := promptLine(reader, "    端口", "22")
	if p, err := strconv.Atoi(portStr); err == nil {
		t.Port = p
	}
	t.Platform = promptLine(reader, "    平台 (linux/win/macos)", "linux")
	t.Type = promptLine(reader, "    类型 (ssh/bridge)", "ssh")

	if t.Type == "bridge" {
		t.BridgeURL = promptLine(reader, "    Bridge URL", "")
		t.AuthToken = promptLine(reader, "    Auth Token", "")
	}

	ds.Targets[name] = t
	fmt.Printf("    %s 已添加 %s\n\n", colorGreen("✓"), name)
}

func cliEditTarget(ds *DeployConfigState, name string, reader *bufio.Reader) {
	t := ds.Targets[name]
	fmt.Printf("\n    编辑 target: %s\n", colorBold(name))

	t.Host = promptLine(reader, "    Host", t.Host)
	portStr := promptLine(reader, "    端口", fmt.Sprintf("%d", t.Port))
	if p, err := strconv.Atoi(portStr); err == nil {
		t.Port = p
	}
	t.Platform = promptLine(reader, "    平台", t.Platform)
	t.Type = promptLine(reader, "    类型", t.Type)

	if t.Type == "bridge" {
		t.BridgeURL = promptLine(reader, "    Bridge URL", t.BridgeURL)
		t.AuthToken = promptLine(reader, "    Auth Token", t.AuthToken)
	}

	ds.Targets[name] = t
	fmt.Printf("    %s 已更新 %s\n\n", colorGreen("✓"), name)
}

func formatTargetInfo(t DeployTarget) string {
	if t.Type == "bridge" {
		return fmt.Sprintf("%s (%s, bridge)", t.BridgeURL, t.Platform)
	}
	port := t.Port
	if port == 0 {
		port = 22
	}
	return fmt.Sprintf("%s:%d (%s, %s)", t.Host, port, t.Platform, t.Type)
}

// ── Step 5: Deploy Projects ──

func cliStepDeployProjects(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	ds := state.DeployState
	fmt.Println()

	order := ds.ProjectOrder
	if len(order) == 0 {
		order = SortedProjectNames(ds.Projects)
	}

	if len(order) == 0 {
		fmt.Printf("  %s 没有发现项目配置\n", colorYellow("!"))
		return nil
	}

	for {
		fmt.Printf("  已有 %d 个项目配置:\n", len(order))
		for i, name := range order {
			proj := ds.Projects[name]
			packPattern := proj.PackPattern
			if packPattern == "" {
				packPattern = name + "_{date}.zip"
			}
			fmt.Printf("    %s %-25s %s\n",
				colorCyan(fmt.Sprintf("[%2d]", i+1)),
				name,
				colorDim(packPattern),
			)
		}
		fmt.Println()

		if cfg.NonInteractive {
			break
		}

		input := promptLine(reader, "  操作: [E]编辑 [V]查看 [回车]继续", "")
		input = strings.ToLower(strings.TrimSpace(input))

		if input == "" {
			break
		}

		switch {
		case strings.HasPrefix(input, "e"):
			idx := parseIndexArg(input, len(order))
			if idx < 0 {
				idx = promptIndex(reader, "  编辑编号", len(order))
			}
			if idx >= 0 {
				cliEditProject(ds, order[idx], reader)
			}
		case strings.HasPrefix(input, "v"):
			idx := parseIndexArg(input, len(order))
			if idx < 0 {
				idx = promptIndex(reader, "  查看编号", len(order))
			}
			if idx >= 0 {
				cliViewProject(ds, order[idx])
			}
		default:
			fmt.Printf("  %s 无效操作\n", colorYellow("!"))
		}
	}

	return nil
}

func cliEditProject(ds *DeployConfigState, name string, reader *bufio.Reader) {
	proj := ds.Projects[name]
	if proj == nil {
		return
	}
	fmt.Printf("\n    编辑项目: %s\n", colorBold(name))

	proj.PackPattern = promptLine(reader, "    Pack Pattern", proj.PackPattern)

	// Protect files
	protectStr := strings.Join(proj.ProtectFiles, ",")
	protectStr = promptLine(reader, "    Protect Files (逗号分隔)", protectStr)
	if protectStr != "" {
		proj.ProtectFiles = ParseStringSlice(protectStr)
	} else {
		proj.ProtectFiles = nil
	}

	// Setup dirs
	setupStr := strings.Join(proj.SetupDirs, ",")
	setupStr = promptLine(reader, "    Setup Dirs (逗号分隔)", setupStr)
	if setupStr != "" {
		proj.SetupDirs = ParseStringSlice(setupStr)
	} else {
		proj.SetupDirs = nil
	}

	// Edit SSH targets' remote_dir/remote_script
	targetNames := SortedTargetNames(ds.Targets)
	for _, tname := range targetNames {
		pt, exists := proj.Targets[tname]
		if !exists {
			pt = DeployProjectTarget{}
		}
		fmt.Printf("    Target %s:\n", colorCyan(tname))
		pt.RemoteDir = promptLine(reader, "      remote_dir", pt.RemoteDir)
		pt.RemoteScript = promptLine(reader, "      remote_script", pt.RemoteScript)

		if proj.Targets == nil {
			proj.Targets = make(map[string]DeployProjectTarget)
		}
		proj.Targets[tname] = pt
	}

	fmt.Printf("    %s 已更新 %s\n\n", colorGreen("✓"), name)
}

func cliViewProject(ds *DeployConfigState, name string) {
	proj := ds.Projects[name]
	if proj == nil {
		return
	}
	fmt.Printf("\n    %s %s\n", colorCyan("▸"), colorBold(name))
	fmt.Printf("      Pack Pattern: %s\n", proj.PackPattern)
	if len(proj.ProtectFiles) > 0 {
		fmt.Printf("      Protect Files: %s\n", strings.Join(proj.ProtectFiles, ", "))
	}
	if len(proj.SetupDirs) > 0 {
		fmt.Printf("      Setup Dirs: %s\n", strings.Join(proj.SetupDirs, ", "))
	}
	if len(proj.Build) > 0 {
		fmt.Println("      Build:")
		for platform, b := range proj.Build {
			fmt.Printf("        %s: project_dir=%s\n", platform, b.ProjectDir)
		}
	}
	if len(proj.Targets) > 0 {
		fmt.Println("      Targets:")
		for tname, pt := range proj.Targets {
			fmt.Printf("        %s: remote_dir=%s remote_script=%s\n", tname, pt.RemoteDir, pt.RemoteScript)
		}
	}
	fmt.Println()
}

// ── Step 6: Deploy Pipelines ──

func cliStepDeployPipelines(cfg *InitConfig, state *WizardState, reader *bufio.Reader) error {
	ds := state.DeployState
	fmt.Println()

	for {
		if len(ds.Pipelines) > 0 {
			fmt.Println("  已有 pipelines:")
			for i, p := range ds.Pipelines {
				fmt.Printf("    %s %-25s %s\n",
					colorCyan(fmt.Sprintf("[%2d]", i+1)),
					p.Name,
					colorDim(fmt.Sprintf("%s (%d steps)", p.Description, len(p.Steps))),
				)
			}
			fmt.Println()
		} else {
			fmt.Printf("  %s 暂无 pipeline\n\n", colorDim("(空)"))
		}

		if cfg.NonInteractive {
			break
		}

		input := promptLine(reader, "  操作: [N]新建 [E]编辑 [V]查看 [D]删除 [回车]继续", "")
		input = strings.ToLower(strings.TrimSpace(input))

		if input == "" {
			break
		}

		switch {
		case input == "n":
			cliAddPipeline(ds, reader)
		case strings.HasPrefix(input, "e"):
			idx := parseIndexArg(input, len(ds.Pipelines))
			if idx < 0 {
				idx = promptIndex(reader, "  编辑编号", len(ds.Pipelines))
			}
			if idx >= 0 {
				cliEditPipeline(ds, idx, reader)
			}
		case strings.HasPrefix(input, "v"):
			idx := parseIndexArg(input, len(ds.Pipelines))
			if idx < 0 {
				idx = promptIndex(reader, "  查看编号", len(ds.Pipelines))
			}
			if idx >= 0 {
				cliViewPipeline(ds.Pipelines[idx])
			}
		case strings.HasPrefix(input, "d"):
			idx := parseIndexArg(input, len(ds.Pipelines))
			if idx < 0 {
				idx = promptIndex(reader, "  删除编号", len(ds.Pipelines))
			}
			if idx >= 0 {
				name := ds.Pipelines[idx].Name
				ds.Pipelines = append(ds.Pipelines[:idx], ds.Pipelines[idx+1:]...)
				// Also delete the file
				_ = DeletePipelineJSON(ds.SettingsDir, name)
				fmt.Printf("  %s 已删除 %s\n\n", colorGreen("✓"), name)
			}
		default:
			fmt.Printf("  %s 无效操作\n", colorYellow("!"))
		}
	}

	return nil
}

func cliAddPipeline(ds *DeployConfigState, reader *bufio.Reader) {
	fmt.Println()
	name := promptLine(reader, "    名称", "")
	if name == "" {
		fmt.Printf("    %s 名称不能为空\n", colorYellow("!"))
		return
	}
	desc := promptLine(reader, "    描述", "")

	p := DeployPipeline{
		Name:        name,
		Description: desc,
	}

	fmt.Println("    添加步骤 (输入空 project 结束):")
	stepNum := 1
	for {
		project := promptLine(reader, fmt.Sprintf("    [%d] Project", stepNum), "")
		if project == "" {
			break
		}
		target := promptLine(reader, fmt.Sprintf("    [%d] Target", stepNum), "ssh-prod")
		platform := promptLine(reader, fmt.Sprintf("    [%d] Platform", stepNum), "linux")

		p.Steps = append(p.Steps, DeployPipelineStep{
			Project:       project,
			Target:        target,
			BuildPlatform: platform,
		})
		stepNum++
	}

	if len(p.Steps) == 0 {
		fmt.Printf("    %s 未添加任何步骤，取消创建\n", colorYellow("!"))
		return
	}

	ds.Pipelines = append(ds.Pipelines, p)
	fmt.Printf("    %s 已添加 %s (%d steps)\n\n", colorGreen("✓"), name, len(p.Steps))
}

func cliEditPipeline(ds *DeployConfigState, idx int, reader *bufio.Reader) {
	p := &ds.Pipelines[idx]
	fmt.Printf("\n    编辑 pipeline: %s\n", colorBold(p.Name))

	p.Description = promptLine(reader, "    描述", p.Description)

	// Show existing steps
	for i, s := range p.Steps {
		fmt.Printf("    [%d] %s → %s (%s)\n", i+1, s.Project, s.Target, s.BuildPlatform)
	}

	action := promptLine(reader, "    操作: [M]修改步骤 [A]追加步骤 [C]清空重建 [回车]保持", "")
	action = strings.ToLower(strings.TrimSpace(action))

	switch action {
	case "m":
		for i := range p.Steps {
			s := &p.Steps[i]
			s.Project = promptLine(reader, fmt.Sprintf("    [%d] Project", i+1), s.Project)
			s.Target = promptLine(reader, fmt.Sprintf("    [%d] Target", i+1), s.Target)
			s.BuildPlatform = promptLine(reader, fmt.Sprintf("    [%d] Platform", i+1), s.BuildPlatform)
		}
	case "a":
		stepNum := len(p.Steps) + 1
		for {
			project := promptLine(reader, fmt.Sprintf("    [%d] Project", stepNum), "")
			if project == "" {
				break
			}
			target := promptLine(reader, fmt.Sprintf("    [%d] Target", stepNum), "ssh-prod")
			platform := promptLine(reader, fmt.Sprintf("    [%d] Platform", stepNum), "linux")
			p.Steps = append(p.Steps, DeployPipelineStep{
				Project:       project,
				Target:        target,
				BuildPlatform: platform,
			})
			stepNum++
		}
	case "c":
		p.Steps = nil
		stepNum := 1
		for {
			project := promptLine(reader, fmt.Sprintf("    [%d] Project", stepNum), "")
			if project == "" {
				break
			}
			target := promptLine(reader, fmt.Sprintf("    [%d] Target", stepNum), "ssh-prod")
			platform := promptLine(reader, fmt.Sprintf("    [%d] Platform", stepNum), "linux")
			p.Steps = append(p.Steps, DeployPipelineStep{
				Project:       project,
				Target:        target,
				BuildPlatform: platform,
			})
			stepNum++
		}
	}

	fmt.Printf("    %s 已更新 %s (%d steps)\n\n", colorGreen("✓"), p.Name, len(p.Steps))
}

func cliViewPipeline(p DeployPipeline) {
	fmt.Printf("\n    %s %s\n", colorCyan("▸"), colorBold(p.Name))
	fmt.Printf("      描述: %s\n", p.Description)
	fmt.Printf("      步骤 (%d):\n", len(p.Steps))
	for i, s := range p.Steps {
		packOnly := ""
		if s.PackOnly {
			packOnly = " [pack_only]"
		}
		fmt.Printf("        [%d] %s → %s (%s)%s\n", i+1, s.Project, s.Target, s.BuildPlatform, packOnly)
	}
	fmt.Println()
}

// ── CLI helpers ──

// parseIndexArg extracts a 1-based index from "e 3" or "e3" style input.
// Returns -1 if no valid index found.
func parseIndexArg(input string, maxLen int) int {
	// Try "e 3" or "e3"
	s := strings.TrimSpace(input[1:])
	if s == "" {
		return -1
	}
	idx, err := strconv.Atoi(s)
	if err != nil || idx < 1 || idx > maxLen {
		return -1
	}
	return idx - 1
}

// promptIndex prompts the user for a 1-based index.
func promptIndex(reader *bufio.Reader, prompt string, maxLen int) int {
	s := promptLine(reader, prompt, "")
	idx, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || idx < 1 || idx > maxLen {
		fmt.Printf("  %s 无效编号\n", colorYellow("!"))
		return -1
	}
	return idx - 1
}
