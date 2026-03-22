package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Target 部署目标
type Target struct {
	Name          string // 逻辑名（local, ssh-prod, ssh-staging）
	Host          string // user@host 或 local
	Port          int    // SSH 端口，默认 22
	RemoteDir     string // 部署目录
	RemoteScript  string // 发布脚本名（可选）
	VerifyURL     string // 部署验证 URL（可选，每个 target 独立）
	VerifyTimeout int    // 验证超时秒数，默认 10
	Platform      string // 目标平台（linux/win/macos），local 时等于 HostPlatform
	Type          string // 部署类型: "ssh"(默认) | "bridge"
	BridgeURL     string // bridge HTTP 地址（type=bridge 时必填）
	AuthToken     string // bridge 认证 token（type=bridge 时必填）
}

// ProjectConfig 项目级部署配置
type ProjectConfig struct {
	Name         string    // 项目名称（settings 文件名）
	ProjectDir   string    // 项目根目录
	PackScript   string    // 打包脚本路径
	PackPattern  string    // 输出文件名模式（{date} → YYYYMMDD_HHMMSS）
	Targets      []*Target // 部署目标列表
	VerifyURL    string    // 部署验证 URL（兼容旧模式，新模式用 Target.VerifyURL）
	ConfigFile   string    // 来源 settings 文件路径
	Configured   bool      // 是否有持久化 settings（.json 文件）
	ProtectFiles []string  // 增量部署时不覆盖的文件/目录
	SetupDirs    []string  // 首次部署时自动创建的数据目录
}

// DeployConfig 全局部署配置
type DeployConfig struct {
	// 全局 SSH 配置
	SSHKey      string // SSH 密钥路径（可选）
	SSHPassword string // SSH 密码（daemon 模式可配置，优先级低于 keyring）

	// WebSocket daemon 模式配置（设置 server_url 启用）
	ServerURL     string // gateway WebSocket 地址
	AgentName     string // Agent 名称（默认使用主机名）
	AuthToken     string // 认证 token
	MaxConcurrent int    // 最大并发部署任务数，默认 1
	// settings 目录（存放项目部署配置文件）
	SettingsDir string // 部署配置目录

	// 运行时参数（CLI 传入）
	HostPlatform string   // 当前主机平台（自动检测，用于 build 配置）
	TargetFilter string   // 发布目标过滤（默认 local，可选 all 或具体 target 名）
	TargetNames  []string // 可用的 target 名称列表（从项目配置扫描）
	SSHHosts     []string // 可用 SSH 服务器列表（去重，如 root@114.115.214.86）

	// Workspace 目录列表（扫描子目录发现项目）
	Workspaces []string

	// 多项目配置
	Projects     map[string]*ProjectConfig // 项目名 → 配置
	ProjectOrder []string                  // 保持声明顺序

	// UAP gateway 配置
	GoBackendAgentID string // go_blog-agent 在 gateway 中的 ID，默认 "go_blog"

	// Pipeline 编排
	PipelinesDir string // pipelines/ 目录路径（自动推断）

	// 配置文件路径（用于 runInit 等需要引用配置路径的场景）
	ConfigPath string
}

// deployConfigJSON 全局 JSON 配置（仅用于 unmarshal）
type deployConfigJSON struct {
	ServerURL        string   `json:"server_url"`
	AgentName        string   `json:"agent_name"`
	AuthToken        string   `json:"auth_token"`
	MaxConcurrent    int      `json:"max_concurrent"`
	SSHKey           string   `json:"ssh_key,omitempty"`
	SSHPassword      string   `json:"ssh_password,omitempty"`
	SettingsDir      string   `json:"settings_dir"`
	Workspaces       []string `json:"workspaces"`
	GoBackendAgentID string   `json:"go_backend_agent_id,omitempty"`
}

// DefaultProject 获取默认项目（仅一个项目时返回，否则返回 nil）
func (c *DeployConfig) DefaultProject() *ProjectConfig {
	if len(c.ProjectOrder) == 1 {
		return c.Projects[c.ProjectOrder[0]]
	}
	return nil
}

// GetProject 按名称获取项目配置
func (c *DeployConfig) GetProject(name string) *ProjectConfig {
	return c.Projects[name]
}

// ProjectNames 返回所有项目名称（按声明顺序）
func (c *DeployConfig) ProjectNames() []string {
	return c.ProjectOrder
}

// projectJSON 项目 JSON 配置（仅用于 unmarshal）
type projectJSON struct {
	PackPattern  string                `json:"pack_pattern,omitempty"`
	Build        map[string]buildJSON  `json:"build"`
	Targets      map[string]targetJSON `json:"targets"`
	ProtectFiles []string              `json:"protect_files,omitempty"` // 增量部署时保护的文件
	SetupDirs    []string              `json:"setup_dirs,omitempty"`   // 首次部署时创建的数据目录
}

// buildJSON 构建配置
type buildJSON struct {
	ProjectDir  string `json:"project_dir"`
	PackScript  string `json:"pack_script,omitempty"`
	PackPattern string `json:"pack_pattern,omitempty"`
}

// targetJSON 部署目标配置
type targetJSON struct {
	Host          string `json:"host,omitempty"`
	Port          int    `json:"ssh_port,omitempty"`
	RemoteDir     string `json:"remote_dir,omitempty"`
	RemoteScript  string `json:"remote_script,omitempty"`
	Platform      string `json:"platform,omitempty"`
	VerifyURL     string `json:"verify_url,omitempty"`
	VerifyTimeout int    `json:"verify_timeout,omitempty"`
	Type          string `json:"type,omitempty"`       // "ssh"(默认) | "bridge"
	BridgeURL     string `json:"bridge_url,omitempty"` // bridge HTTP 地址
	AuthToken     string `json:"auth_token,omitempty"` // bridge 认证 token
}

// LoadConfigForDaemon daemon 模式配置加载：加载所有 target 配置
// targetFilter 强制 "all"，所有平台的 local target 均加载（部署时按 HostPlatform 过滤）
func LoadConfigForDaemon(path string) (*DeployConfig, error) {
	return LoadConfig(path, "all")
}

// LoadConfig 从 JSON 配置文件加载全局配置
func LoadConfig(path string, targetFilter string) (*DeployConfig, error) {
	cfg := &DeployConfig{
		MaxConcurrent: 1,
		Projects:      make(map[string]*ProjectConfig),
		TargetFilter:  targetFilter,
	}

	cfg.ConfigPath = path

	// 主机平台 = 当前 OS（始终自动检测）
	cfg.HostPlatform = platformSubdir()
	// 默认发布目标 = local
	if cfg.TargetFilter == "" {
		cfg.TargetFilter = "local"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}

	var jcfg deployConfigJSON
	if err := json.Unmarshal(data, &jcfg); err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}

	// 映射 JSON 字段到 DeployConfig
	cfg.ServerURL = jcfg.ServerURL
	cfg.AgentName = jcfg.AgentName
	cfg.AuthToken = jcfg.AuthToken
	if jcfg.MaxConcurrent > 0 {
		cfg.MaxConcurrent = jcfg.MaxConcurrent
	}
	cfg.SSHKey = jcfg.SSHKey
	cfg.SSHPassword = jcfg.SSHPassword
	cfg.SettingsDir = jcfg.SettingsDir
	cfg.Workspaces = jcfg.Workspaces
	cfg.GoBackendAgentID = jcfg.GoBackendAgentID

	// workspace 扫描：发现含 go.mod 的子目录
	cfg.scanWorkspaces()

	// settings_dir 模式：加载 projects/ 目录（叠加到 workspace 发现的项目上）
	if cfg.SettingsDir != "" {
		settingsDir := cfg.SettingsDir
		if !filepath.IsAbs(settingsDir) {
			settingsDir = filepath.Join(filepath.Dir(path), settingsDir)
		}
		if err := cfg.loadProjectsDir(settingsDir); err != nil {
			// workspace 有项目时 settings_dir 加载失败不致命
			if len(cfg.Projects) == 0 {
				return nil, fmt.Errorf("load settings_dir %q: %v", settingsDir, err)
			}
			// 有 workspace 项目，只打 log 警告
			fmt.Fprintf(os.Stderr, "warning: load settings_dir %q: %v\n", settingsDir, err)
		}
	}

	// 全局默认值
	if cfg.AgentName == "" {
		cfg.AgentName, _ = os.Hostname()
		if cfg.AgentName != "" {
			cfg.AgentName += "-deploy"
		} else {
			cfg.AgentName = "deploy-agent"
		}
	}
	if len(cfg.Projects) == 0 {
		return nil, fmt.Errorf("no projects found (check workspaces, settings_dir and projects/ directory)")
	}
	if cfg.GoBackendAgentID == "" {
		cfg.GoBackendAgentID = "go_blog"
	}

	// 自动探测 pipelines 目录
	cfg.detectPipelinesDir(path)

	return cfg, nil
}

// platformSubdir 根据 runtime.GOOS 返回平台子目录名
func platformSubdir() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "win"
	default:
		return runtime.GOOS
	}
}

// normalizePlatform 将用户输入的平台名标准化
func normalizePlatform(p string) string {
	switch strings.ToLower(p) {
	case "darwin", "macos", "mac":
		return "macos"
	case "windows", "win":
		return "win"
	case "linux":
		return "linux"
	default:
		return p
	}
}

// dirExists 判断目录是否存在
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// scanWorkspaces 扫描 workspace 目录，发现含 go.mod 的子目录作为项目
func (c *DeployConfig) scanWorkspaces() {
	for _, ws := range c.Workspaces {
		if !dirExists(ws) {
			continue
		}
		entries, err := os.ReadDir(ws)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			projDir := filepath.Join(ws, entry.Name())
			_, projName, err := detectGoProject(projDir)
			if err != nil || projName == "" {
				continue // 不含 go.mod，跳过
			}
			if _, exists := c.Projects[projName]; exists {
				continue // 已存在（可能另一个 workspace 先发现了同名项目）
			}

			// 自动检测 PackScript
			var packScript string
			if runtime.GOOS == "windows" {
				packScript = filepath.Join(projDir, "pack.bat")
			} else {
				packScript = filepath.Join(projDir, "pack.sh")
			}

			proj := &ProjectConfig{
				Name:        projName,
				ProjectDir:  projDir,
				PackScript:  packScript,
				PackPattern: projName + "_{date}.zip",
				Configured:  false,
			}
			c.Projects[projName] = proj
			c.ProjectOrder = append(c.ProjectOrder, projName)
		}
	}
}

// loadGlobalTargets 读取 settings/targets.json，返回全局 target 定义
// 文件不存在时返回 nil, nil（向后兼容）
func loadGlobalTargets(settingsDir string) (map[string]targetJSON, error) {
	path := filepath.Join(settingsDir, "targets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read targets.json: %v", err)
	}

	var targets map[string]targetJSON
	if err := json.Unmarshal(data, &targets); err != nil {
		return nil, fmt.Errorf("parse targets.json: %v", err)
	}
	return targets, nil
}

// mergeTargetJSON 合并全局 target 和项目级 target 定义
// global 为底，project 中的非零值覆盖
func mergeTargetJSON(global, project targetJSON) targetJSON {
	result := global
	if project.Host != "" {
		result.Host = project.Host
	}
	if project.Port > 0 {
		result.Port = project.Port
	}
	if project.RemoteDir != "" {
		result.RemoteDir = project.RemoteDir
	}
	if project.RemoteScript != "" {
		result.RemoteScript = project.RemoteScript
	}
	if project.Platform != "" {
		result.Platform = project.Platform
	}
	if project.VerifyURL != "" {
		result.VerifyURL = project.VerifyURL
	}
	if project.VerifyTimeout > 0 {
		result.VerifyTimeout = project.VerifyTimeout
	}
	if project.Type != "" {
		result.Type = project.Type
	}
	if project.BridgeURL != "" {
		result.BridgeURL = project.BridgeURL
	}
	if project.AuthToken != "" {
		result.AuthToken = project.AuthToken
	}
	return result
}

// loadProjectsDir 扫描 settings/projects/ 目录加载项目配置
// 每个 <project>.json 文件包含完整的构建和部署目标配置
func (c *DeployConfig) loadProjectsDir(settingsDir string) error {
	projectsDir := filepath.Join(settingsDir, "projects")
	if !dirExists(projectsDir) {
		return fmt.Errorf("projects directory not found: %s", projectsDir)
	}

	// 加载全局 targets（settings/targets.json）
	globalTargets, err := loadGlobalTargets(settingsDir)
	if err != nil {
		return fmt.Errorf("load global targets: %v", err)
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %v", err)
	}

	var jsonFiles []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			jsonFiles = append(jsonFiles, entry)
		}
	}
	sort.Slice(jsonFiles, func(i, j int) bool {
		return jsonFiles[i].Name() < jsonFiles[j].Name()
	})

	// 收集所有 target 名称和 SSH 主机
	allTargetNames := map[string]bool{}
	allSSHHosts := map[string]bool{}

	for _, entry := range jsonFiles {
		filePath := filepath.Join(projectsDir, entry.Name())
		projName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		proj, targets, err := c.parseProjectJSON(projName, filePath, globalTargets)
		if err != nil {
			return fmt.Errorf("project [%s]: %v", projName, err)
		}

		// 过滤 targets
		var filteredTargets []*Target
		for _, t := range targets {
			allTargetNames[t.Name] = true
			// 收集非 local、非 bridge 的 SSH 主机
			if !isLocalTarget(t.Host) && t.Type != "bridge" {
				allSSHHosts[t.Host] = true
			}
			if c.shouldIncludeTarget(t) {
				filteredTargets = append(filteredTargets, t)
			}
		}

		if len(filteredTargets) == 0 {
			continue // 该项目在当前过滤条件下没有可用 target
		}

		// 叠加到 workspace 发现的项目，或新建（向后兼容无 workspace 的 .json 项目）
		if existing, ok := c.Projects[projName]; ok {
			// workspace 已发现该项目，叠加 settings
			existing.Targets = filteredTargets
			existing.ConfigFile = filePath
			existing.Configured = true
			// 覆盖构建参数（.json 优先）
			if proj.ProjectDir != "" {
				existing.ProjectDir = proj.ProjectDir
			}
			if proj.PackScript != "" {
				existing.PackScript = proj.PackScript
			}
			if proj.PackPattern != "" {
				existing.PackPattern = proj.PackPattern
			}
			existing.ProtectFiles = proj.ProtectFiles
			existing.SetupDirs = proj.SetupDirs
		} else {
			// 纯 .json 项目（不在 workspace 中），向后兼容
			proj.Targets = filteredTargets
			proj.ConfigFile = filePath
			proj.Configured = true
			c.Projects[projName] = proj
			c.ProjectOrder = append(c.ProjectOrder, projName)
		}
	}

	// 更新可用 target 名称列表
	var names []string
	for n := range allTargetNames {
		names = append(names, n)
	}
	sort.Strings(names)
	c.TargetNames = names

	// 更新可用 SSH 主机列表
	var hosts []string
	for h := range allSSHHosts {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	c.SSHHosts = hosts

	return nil
}

// shouldIncludeTarget 根据 TargetFilter 判断是否包含该 target
func (c *DeployConfig) shouldIncludeTarget(t *Target) bool {
	if c.TargetFilter == "all" {
		return true
	}
	// 按 target 名精确匹配
	// 例如 --target=local 匹配 "local"，--target=ssh-prod 匹配 "ssh-prod"
	if c.TargetFilter == t.Name {
		return true
	}
	return false
}

// parseProjectJSON 解析单个项目的 .json 文件
// 返回 ProjectConfig（构建配置）和 Target 列表（部署目标）
// globalTargets 为 settings/targets.json 中定义的全局 target，可为 nil
func (c *DeployConfig) parseProjectJSON(projName, filePath string, globalTargets map[string]targetJSON) (*ProjectConfig, []*Target, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %v", filePath, err)
	}

	var pj projectJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %v", filePath, err)
	}

	proj := &ProjectConfig{
		Name:         projName,
		PackPattern:  projName + "_{date}.zip",
		ProtectFiles: pj.ProtectFiles,
		SetupDirs:    pj.SetupDirs,
	}

	// 全局 pack_pattern
	if pj.PackPattern != "" {
		proj.PackPattern = pj.PackPattern
	}

	// 解析 build.<platform> — 匹配 HostPlatform
	buildCfg, ok := pj.Build[c.HostPlatform]
	if !ok {
		return nil, nil, fmt.Errorf("missing build.%s (deploy-agent is running on %s)", c.HostPlatform, c.HostPlatform)
	}

	proj.ProjectDir = buildCfg.ProjectDir
	if buildCfg.PackScript != "" {
		proj.PackScript = buildCfg.PackScript
	}
	if buildCfg.PackPattern != "" {
		proj.PackPattern = buildCfg.PackPattern
	}

	// 验证 project_dir
	if proj.ProjectDir == "" {
		return nil, nil, fmt.Errorf("project_dir is required in build.%s", c.HostPlatform)
	}
	if !filepath.IsAbs(proj.ProjectDir) {
		abs, err := filepath.Abs(proj.ProjectDir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve project_dir: %v", err)
		}
		proj.ProjectDir = abs
	}
	if info, err := os.Stat(proj.ProjectDir); err != nil || !info.IsDir() {
		// 路径不存在则自动创建
		if err := os.MkdirAll(proj.ProjectDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("project_dir does not exist and create failed: %s: %v", proj.ProjectDir, err)
		}
		fmt.Fprintf(os.Stderr, "info: created project_dir: %s\n", proj.ProjectDir)
	}

	// PackScript 自动检测
	if proj.PackScript == "" {
		if runtime.GOOS == "windows" {
			proj.PackScript = filepath.Join(proj.ProjectDir, "pack.bat")
		} else {
			proj.PackScript = filepath.Join(proj.ProjectDir, "pack.sh")
		}
	} else if !filepath.IsAbs(proj.PackScript) {
		proj.PackScript = filepath.Join(proj.ProjectDir, proj.PackScript)
	}

	// 解析 targets
	var targets []*Target

	// 按名称排序保证稳定输出
	var targetNames []string
	for name := range pj.Targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for _, name := range targetNames {
		tj := pj.Targets[name]
		if globalTargets != nil {
			if gt, ok := globalTargets[name]; ok {
				tj = mergeTargetJSON(gt, tj)
			}
		}
		t, err := c.parseTargetJSON(name, &tj)
		if err != nil {
			return nil, nil, fmt.Errorf("target %q: %v", name, err)
		}
		if t != nil {
			targets = append(targets, t)
		}
	}

	return proj, targets, nil
}

// parseTargetJSON 解析 target JSON 配置
// 支持三种格式：
//
//	"local.<platform>"  — 本机部署（platform 从 key 名提取）
//	"<name>" + type=bridge — Bridge HTTP 部署
//	"<name>"            — SSH 远程部署（platform 从 platform 字段读取）
func (c *DeployConfig) parseTargetJSON(name string, tj *targetJSON) (*Target, error) {
	// 解析 target 名：local.<platform> 或 ssh-prod 等
	parts := strings.SplitN(name, ".", 2)
	isLocal := parts[0] == "local"

	var targetName string
	var targetPlatform string

	if isLocal && len(parts) == 2 {
		// "local.win" → 本机部署，平台从 key 名提取
		targetPlatform = parts[1]
		targetName = "local"
	} else if isLocal && len(parts) == 1 {
		// "local" — 无平台限定的 local target
		targetName = "local"
		targetPlatform = c.HostPlatform
	} else {
		// "ssh-prod" / "bridge-prod" — 远程部署
		targetName = name
	}

	port := 22
	if tj.Port > 0 {
		port = tj.Port
	}
	verifyTimeout := 10
	if tj.VerifyTimeout > 0 {
		verifyTimeout = tj.VerifyTimeout
	}
	if tj.Platform != "" {
		targetPlatform = normalizePlatform(tj.Platform)
	}

	// 确定部署类型，默认 ssh
	deployType := tj.Type
	if deployType == "" {
		deployType = "ssh"
	}

	if isLocal {
		return &Target{
			Name:          targetName,
			Host:          "local",
			Port:          port,
			RemoteDir:     tj.RemoteDir,
			RemoteScript:  tj.RemoteScript,
			VerifyURL:     tj.VerifyURL,
			VerifyTimeout: verifyTimeout,
			Platform:      targetPlatform,
		}, nil
	}

	// Bridge target: 需要 bridge_url 和 auth_token
	if deployType == "bridge" {
		if tj.BridgeURL == "" {
			return nil, fmt.Errorf("bridge_url is required for bridge target %q", targetName)
		}
		if tj.AuthToken == "" {
			return nil, fmt.Errorf("auth_token is required for bridge target %q", targetName)
		}
		if targetPlatform == "" {
			targetPlatform = "linux" // bridge 默认 linux
		}
		return &Target{
			Name:          targetName,
			Host:          tj.BridgeURL, // Host 存 bridge URL，用于显示
			Port:          port,
			RemoteDir:     tj.RemoteDir,
			RemoteScript:  tj.RemoteScript,
			VerifyURL:     tj.VerifyURL,
			VerifyTimeout: verifyTimeout,
			Platform:      targetPlatform,
			Type:          "bridge",
			BridgeURL:     tj.BridgeURL,
			AuthToken:     tj.AuthToken,
		}, nil
	}

	// SSH target: 需要 host
	if tj.Host == "" {
		return nil, fmt.Errorf("host is required for ssh target %q", targetName)
	}
	// SSH target: 需要 platform
	if targetPlatform == "" {
		return nil, fmt.Errorf("platform is required for ssh target %q", targetName)
	}

	// 支持多 IP（逗号分隔）
	hostList := strings.Split(tj.Host, ",")
	host := strings.TrimSpace(hostList[0])

	return &Target{
		Name:          targetName,
		Host:          host,
		Port:          port,
		RemoteDir:     tj.RemoteDir,
		RemoteScript:  tj.RemoteScript,
		VerifyURL:     tj.VerifyURL,
		VerifyTimeout: verifyTimeout,
		Platform:      targetPlatform,
		Type:          "ssh",
	}, nil
}

// detectPipelinesDir 自动探测 pipelines/ 目录路径
// 优先 settings_dir/pipelines/，其次 deploy.json 同目录/pipelines/
func (c *DeployConfig) detectPipelinesDir(configPath string) {
	candidates := []string{}
	if c.SettingsDir != "" {
		settingsDir := c.SettingsDir
		if !filepath.IsAbs(settingsDir) {
			settingsDir = filepath.Join(filepath.Dir(configPath), settingsDir)
		}
		candidates = append(candidates, filepath.Join(settingsDir, "pipelines"))
	}
	candidates = append(candidates, filepath.Join(filepath.Dir(configPath), "pipelines"))

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			c.PipelinesDir = p
			return
		}
	}
}

// generateDefaultConfig 生成 deploy-agent 默认配置文件和目录结构
func generateDefaultConfig(configPath string) error {
	configDir := filepath.Dir(configPath)

	// 1. 生成主配置文件 deploy-agent.json
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("配置文件已存在: %s（不会覆盖）", configPath)
	}

	defaultCfg := &deployConfigJSON{
		SettingsDir:   "settings",
		MaxConcurrent: 1,
	}
	data, err := json.MarshalIndent(defaultCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}
	fmt.Printf("已生成默认配置文件: %s\n", configPath)

	// 2. 创建 settings/ 目录结构
	settingsDir := filepath.Join(configDir, "settings")
	projectsDir := filepath.Join(settingsDir, "projects")
	pipelinesDir := filepath.Join(settingsDir, "pipelines")

	for _, dir := range []string{projectsDir, pipelinesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 3. 生成 settings/targets.json 示例
	targetsPath := filepath.Join(settingsDir, "targets.json")
	if _, err := os.Stat(targetsPath); os.IsNotExist(err) {
		targetsExample := map[string]targetJSON{
			"ssh-prod": {
				Host:      "root@your-server-ip",
				Port:      22,
				RemoteDir: "/data/program/your-project",
				Platform:  "linux",
			},
		}
		tData, _ := json.MarshalIndent(targetsExample, "", "  ")
		if err := os.WriteFile(targetsPath, append(tData, '\n'), 0644); err != nil {
			return fmt.Errorf("写入 targets.json 失败: %v", err)
		}
		fmt.Printf("已生成示例文件: %s\n", targetsPath)
	}

	// 4. 生成 settings/projects/example.json 示例
	examplePath := filepath.Join(projectsDir, "example.json")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		exampleProject := projectJSON{
			PackPattern: "example_{date}.zip",
			Build: map[string]buildJSON{
				"win": {
					ProjectDir: "E:/projects/example",
				},
				"linux": {
					ProjectDir: "/home/user/projects/example",
				},
			},
			Targets: map[string]targetJSON{
				"local.win": {
					RemoteDir: "E:/deploy/example",
				},
				"ssh-prod": {},
			},
		}
		eData, _ := json.MarshalIndent(exampleProject, "", "  ")
		if err := os.WriteFile(examplePath, append(eData, '\n'), 0644); err != nil {
			return fmt.Errorf("写入 example.json 失败: %v", err)
		}
		fmt.Printf("已生成示例文件: %s\n", examplePath)
	}

	fmt.Printf("已创建目录: %s\n", pipelinesDir)
	fmt.Println("\n配置目录结构:")
	fmt.Printf("  %s\n", configPath)
	fmt.Printf("  %s/\n", settingsDir)
	fmt.Printf("    targets.json\n")
	fmt.Printf("    projects/\n")
	fmt.Printf("      example.json\n")
	fmt.Printf("    pipelines/\n")

	return nil
}

