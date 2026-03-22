package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// TargetCheckPlan 单个目标机器的检测计划
type TargetCheckPlan struct {
	TargetName   string
	Target       DeployTarget
	Projects     []string              // 部署到此机器的 project 列表
	Requirements []SoftwareRequirement // 聚合后的软件需求
}

// TargetEnvResult 单个目标机器的检测结果
type TargetEnvResult struct {
	TargetName string
	Host       string
	Platform   string
	Projects   []string              // 部署到此机器的 project 列表
	Results    []SoftwareCheckResult // 检测结果
	SSHError   string                // SSH 连接失败时的错误
}

// DeriveTargetRequirements 从 pipeline 数据推导每个 target 需要检测的软件
// pipeline steps → group by target → union agent dependencies
func DeriveTargetRequirements(
	pipelines []DeployPipeline,
	targets map[string]DeployTarget,
	projects map[string]*DeployProject,
) []TargetCheckPlan {
	// 按 target 分组：target name → project names
	targetProjects := make(map[string][]string)

	for _, pipeline := range pipelines {
		for _, step := range pipeline.Steps {
			if step.PackOnly {
				continue
			}
			targetName := step.Target
			if targetName == "" {
				// 如果 pipeline step 没有指定 target，从 project 的 targets 中推导
				if proj, ok := projects[step.Project]; ok {
					for tn := range proj.Targets {
						addUnique(&targetProjects, tn, step.Project)
					}
				}
				continue
			}
			addUnique(&targetProjects, targetName, step.Project)
		}
	}

	// 为每个 target 聚合软件需求
	var plans []TargetCheckPlan
	for targetName, projNames := range targetProjects {
		target, ok := targets[targetName]
		if !ok {
			continue
		}

		// 聚合该 target 上所有 project 对应的 agent 依赖
		reqMap := make(map[string]SoftwareRequirement)
		for _, projName := range projNames {
			agentName := resolveAgentName(projName, projects)
			schema := GetAgentSchema(agentName)
			if schema == nil {
				continue
			}
			for _, dep := range schema.Dependencies {
				// 保留更高版本要求
				if existing, ok := reqMap[dep.Software]; ok {
					if dep.MinVersion != "" && (existing.MinVersion == "" || compareVersions(dep.MinVersion, existing.MinVersion) > 0) {
						reqMap[dep.Software] = dep
					}
				} else {
					reqMap[dep.Software] = dep
				}
			}
		}

		var reqs []SoftwareRequirement
		for _, r := range reqMap {
			reqs = append(reqs, r)
		}

		plans = append(plans, TargetCheckPlan{
			TargetName:   targetName,
			Target:       target,
			Projects:     projNames,
			Requirements: reqs,
		})
	}

	return plans
}

// resolveAgentName 从 project name 推导对应的 agent name
// 优先匹配 build config 中的 project_dir，回退用 project name
func resolveAgentName(projectName string, projects map[string]*DeployProject) string {
	proj, ok := projects[projectName]
	if !ok {
		return projectName
	}

	// 从 build config 的 project_dir 提取 agent name
	for _, build := range proj.Build {
		if build.ProjectDir != "" {
			// "cmd/gateway" → "gateway"
			dir := strings.TrimSuffix(build.ProjectDir, "/")
			parts := strings.Split(dir, "/")
			if len(parts) > 0 {
				candidate := parts[len(parts)-1]
				if GetAgentSchema(candidate) != nil {
					return candidate
				}
			}
		}
	}

	// 回退：直接用 project name
	return projectName
}

// RunTargetChecks 对每个目标机器执行检测
func RunTargetChecks(plans []TargetCheckPlan, sshPassword, sshKeyPath string) []TargetEnvResult {
	var results []TargetEnvResult

	for _, plan := range plans {
		result := TargetEnvResult{
			TargetName: plan.TargetName,
			Host:       plan.Target.Host,
			Platform:   plan.Target.Platform,
			Projects:   plan.Projects,
		}

		// Bridge 类型跳过（无 SSH 直连能力）
		if plan.Target.Type == "bridge" {
			result.SSHError = "bridge 类型目标不支持 SSH 直连检测"
			results = append(results, result)
			continue
		}

		// 无检测需求则跳过
		if len(plan.Requirements) == 0 {
			results = append(results, result)
			continue
		}

		isLocal := isLocalTarget(plan.TargetName, plan.Target)

		if isLocal {
			// 本地目标：直接用本地检测
			result.Results = CheckSoftware(plan.Requirements)
		} else {
			// 远程目标：通过 SSH 检测
			checkResults, sshErr := runRemoteChecks(plan.Target, plan.Requirements, sshPassword, sshKeyPath)
			if sshErr != "" {
				result.SSHError = sshErr
			} else {
				result.Results = checkResults
			}
		}

		results = append(results, result)
	}

	return results
}

// isLocalTarget 判断目标是否为本机
func isLocalTarget(name string, target DeployTarget) bool {
	if strings.HasPrefix(name, "local") {
		return true
	}
	if target.Host == "" {
		return true
	}
	host := target.Host
	// 去除 user@ 前缀
	if idx := strings.Index(host, "@"); idx >= 0 {
		host = host[idx+1:]
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// runRemoteChecks 通过 SSH 检测远程机器上的软件
// 返回检测结果和可能的 SSH 错误信息
func runRemoteChecks(target DeployTarget, reqs []SoftwareRequirement, sshPassword, sshKeyPath string) ([]SoftwareCheckResult, string) {
	port := target.Port
	if port == 0 {
		port = 22
	}

	// 解析 host（可能含 user@host 格式）
	user := "root"
	host := target.Host
	if idx := strings.Index(host, "@"); idx >= 0 {
		user = host[:idx]
		host = host[idx+1:]
	}

	// 建立 SSH 连接
	client, err := dialSSH(user, host, port, sshPassword, sshKeyPath)
	if err != nil {
		return nil, err.Error()
	}
	defer client.Close()

	// 逐个检测软件
	var results []SoftwareCheckResult
	for _, req := range reqs {
		r := checkOneRemote(client, req, target.Platform)
		results = append(results, r)
	}
	return results, ""
}

// checkOneRemote 通过 SSH session 检测单个软件
func checkOneRemote(client *ssh.Client, req SoftwareRequirement, platform string) SoftwareCheckResult {
	r := SoftwareCheckResult{
		Software:   req.Software,
		MinVersion: req.MinVersion,
	}

	cmd := softwareCheckCommands[req.Software]
	if cmd == "" {
		cmd = req.Software + " --version"
	}

	output, err := runSSHCommand(client, cmd)
	if err != nil {
		r.Installed = false
		r.InstallHint = getRemoteInstallHint(req.Software, platform)
		return r
	}

	r.Installed = true
	r.Version = parseVersion(output)

	// 远程查找 path
	binName := softwarePathCommands[req.Software]
	if binName == "" {
		binName = req.Software
	}
	pathCmd := "which " + binName
	if platform == "win" {
		pathCmd = "where " + binName
	}
	if pathOutput, err := runSSHCommand(client, pathCmd); err == nil {
		// 只取第一行
		lines := strings.SplitN(strings.TrimSpace(pathOutput), "\n", 2)
		if len(lines) > 0 {
			r.Path = strings.TrimSpace(lines[0])
		}
	}

	// 检查最低版本
	if req.MinVersion == "" {
		r.MeetsRequirement = true
	} else if r.Version != "" {
		r.MeetsRequirement = compareVersions(r.Version, req.MinVersion) >= 0
	}

	if !r.MeetsRequirement && r.Installed {
		r.InstallHint = getRemoteInstallHint(req.Software, platform)
	}

	return r
}

// runSSHCommand 在 SSH 连接上执行单条命令
func runSSHCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session: %v", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(command)
	return strings.TrimSpace(string(out)), err
}

// dialSSH 建立 SSH 连接
func dialSSH(user, host string, port int, password, keyPath string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 优先尝试 key 认证
	if keyPath != "" {
		keyBytes, err := os.ReadFile(keyPath)
		if err == nil {
			if signer, err := ssh.ParsePrivateKey(keyBytes); err == nil {
				config.Auth = append(config.Auth, ssh.PublicKeys(signer))
			}
		}
	}

	// 密码认证
	if password != "" {
		config.Auth = append(config.Auth, ssh.Password(password))
		config.Auth = append(config.Auth, ssh.KeyboardInteractive(
			func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			},
		))
	}

	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("无 SSH 凭证（需要密码或密钥）")
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %v", addr, err)
	}
	return client, nil
}

// getRemoteInstallHint 获取远程平台的安装提示
func getRemoteInstallHint(software, platform string) string {
	osKey := platform
	switch platform {
	case "linux", "":
		osKey = "linux"
	case "darwin", "macos":
		osKey = "darwin"
	case "win", "windows":
		osKey = "windows"
	}
	if hints, ok := installHints[software]; ok {
		if hint, ok := hints[osKey]; ok {
			return hint
		}
	}
	return ""
}

// PrintTargetCheckResults 按 target 分组打印检测结果
func PrintTargetCheckResults(results []TargetEnvResult) {
	fmt.Println()

	totalMissing := 0
	totalMachines := len(results)

	for _, r := range results {
		// 构建标题行
		hostInfo := r.Host
		if hostInfo == "" {
			hostInfo = "本机"
		}
		platformInfo := r.Platform
		if platformInfo == "" {
			platformInfo = "unknown"
		}

		title := fmt.Sprintf("%s (%s, %s)", r.TargetName, hostInfo, platformInfo)
		projectList := strings.Join(r.Projects, ", ")

		// 计算框宽度
		titleLen := displayWidth(title)
		contentWidth := titleLen + 4
		if contentWidth < 50 {
			contentWidth = 50
		}

		// 顶部边框
		fmt.Printf("  ┌─ %s %s┐\n", title, strings.Repeat("─", contentWidth-titleLen-2))

		// 部署项目
		fmt.Printf("  │  部署项目: %-*s│\n", contentWidth-4, projectList)

		if r.SSHError != "" {
			// SSH 连接失败
			errMsg := fmt.Sprintf("SSH 错误: %s", r.SSHError)
			fmt.Printf("  │  %s %-*s│\n", colorRed("✗"), contentWidth-6, errMsg)
		} else if len(r.Results) == 0 {
			fmt.Printf("  │  %-*s│\n", contentWidth-2, colorDim("(无检测项)"))
		} else {
			// 打印检测结果
			for _, cr := range r.Results {
				status := colorGreen("✓")
				detail := ""
				if !cr.Installed {
					status = colorRed("✗")
					detail = "(未安装)"
					totalMissing++
				} else if !cr.MeetsRequirement {
					status = colorYellow("!")
					detail = fmt.Sprintf("(版本 %s < 要求 %s)", cr.Version, cr.MinVersion)
					totalMissing++
				} else {
					if cr.Version != "" {
						detail = fmt.Sprintf("v%s", cr.Version)
					}
					if cr.Path != "" {
						detail += fmt.Sprintf("  (%s)", cr.Path)
					}
				}
				fmt.Printf("  │  %s %-10s%s\n", status, cr.Software, detail)

				// 安装提示
				if cr.InstallHint != "" && (!cr.Installed || !cr.MeetsRequirement) {
					fmt.Printf("  │    安装: %s\n", cr.InstallHint)
				}
			}
		}

		// 底部边框
		fmt.Printf("  └%s┘\n", strings.Repeat("─", contentWidth))
		fmt.Println()
	}

	// 总结
	if totalMissing > 0 {
		fmt.Printf("  %s %d 台机器检测完成，%d 项缺失\n",
			colorGreen("✓"), totalMachines, totalMissing)
	} else {
		fmt.Printf("  %s %d 台机器检测完成，全部满足要求\n",
			colorGreen("✓"), totalMachines)
	}
}

// displayWidth 估算字符串的显示宽度（中文占2，ASCII占1）
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x7F {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// addUnique 向 map[string][]string 中添加不重复的值
func addUnique(m *map[string][]string, key, value string) {
	for _, v := range (*m)[key] {
		if v == value {
			return
		}
	}
	(*m)[key] = append((*m)[key], value)
}

// GetSSHKeyPath 从 deploy-agent.json 中读取 ssh_key 路径
func GetSSHKeyPath(rootDir string) string {
	deployAgentPath := filepath.Join(rootDir, "cmd", "deploy-agent", "deploy-agent.json")
	data, err := os.ReadFile(deployAgentPath)
	if err != nil {
		return ""
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	if key, ok := cfg["ssh_key"].(string); ok {
		return key
	}
	return ""
}
