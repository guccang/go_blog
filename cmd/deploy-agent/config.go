package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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
}

// ProjectConfig 项目级部署配置
type ProjectConfig struct {
	Name        string    // 项目名称（settings 文件名）
	ProjectDir  string    // 项目根目录
	PackScript  string    // 打包脚本路径
	PackPattern string    // 输出文件名模式（{date} → YYYYMMDD_HHMMSS）
	Targets     []*Target // 部署目标列表
	VerifyURL   string    // 部署验证 URL（兼容旧模式，新模式用 Target.VerifyURL）
	ConfigFile  string    // 来源 settings 文件路径
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

// configLine 配置文件中的键值对
type configLine struct {
	key, val string
}

// configSection 配置文件中的一个 section
type configSection struct {
	name  string       // section 名称（空 = 全局）
	lines []configLine // 键值对列表
}

// LoadConfigForDaemon daemon 模式配置加载：加载所有 target 配置
// targetFilter 强制 "all"，所有平台的 local target 均加载（部署时按 HostPlatform 过滤）
func LoadConfigForDaemon(path string) (*DeployConfig, error) {
	return LoadConfig(path, "all")
}

// LoadConfig 从配置文件加载全局配置
// settings_dir 指向 projects/ 目录所在的 settings 目录
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

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}
	defer file.Close()

	// 逐行读取全局配置（deploy.conf 不再支持 [project] section）
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		parseGlobalKey(cfg, key, val)
	}

	// settings_dir 模式：加载 projects/ 目录
	if cfg.SettingsDir != "" {
		settingsDir := cfg.SettingsDir
		if !filepath.IsAbs(settingsDir) {
			settingsDir = filepath.Join(filepath.Dir(path), settingsDir)
		}
		if err := cfg.loadProjectsDir(settingsDir); err != nil {
			return nil, fmt.Errorf("load settings_dir %q: %v", settingsDir, err)
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
		return nil, fmt.Errorf("no projects found (check settings_dir and projects/ directory)")
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

// loadProjectsDir 扫描 settings/projects/ 目录加载项目配置
// 每个 <project>.conf 文件包含完整的构建和部署目标配置
// 格式：
//
//	pack_pattern=xxx              # 通用配置
//	[build.<platform>]            # 按 HostPlatform 区分的构建参数
//	[target.<name>]               # SSH 远程部署目标（含 platform= 字段）
//	[target.local.<platform>]     # 本机部署目标（按平台区分）
func (c *DeployConfig) loadProjectsDir(settingsDir string) error {
	projectsDir := filepath.Join(settingsDir, "projects")
	if !dirExists(projectsDir) {
		return fmt.Errorf("projects directory not found: %s", projectsDir)
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %v", err)
	}

	var confFiles []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".conf") {
			confFiles = append(confFiles, entry)
		}
	}
	sort.Slice(confFiles, func(i, j int) bool {
		return confFiles[i].Name() < confFiles[j].Name()
	})

	// 收集所有 target 名称
	allTargetNames := map[string]bool{}

	for _, entry := range confFiles {
		filePath := filepath.Join(projectsDir, entry.Name())
		projName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		proj, targets, err := c.parseProjectConf(projName, filePath)
		if err != nil {
			return fmt.Errorf("project [%s]: %v", projName, err)
		}

		// 过滤 targets
		var filteredTargets []*Target
		for _, t := range targets {
			allTargetNames[t.Name] = true
			if c.shouldIncludeTarget(t) {
				filteredTargets = append(filteredTargets, t)
			}
		}

		if len(filteredTargets) == 0 {
			continue // 该项目在当前过滤条件下没有可用 target
		}

		proj.Targets = filteredTargets
		proj.ConfigFile = filePath
		c.Projects[projName] = proj
		c.ProjectOrder = append(c.ProjectOrder, projName)
	}

	// 更新可用 target 名称列表
	var names []string
	for n := range allTargetNames {
		names = append(names, n)
	}
	sort.Strings(names)
	c.TargetNames = names

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

// parseProjectConf 解析单个项目的 .conf 文件
// 返回 ProjectConfig（构建配置）和 Target 列表（部署目标）
func (c *DeployConfig) parseProjectConf(projName, filePath string) (*ProjectConfig, []*Target, error) {
	sections, err := readConfSections(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %v", filePath, err)
	}

	proj := &ProjectConfig{
		Name:        projName,
		PackPattern: projName + "_{date}.zip",
	}

	// 解析全局 section（无 section 名）
	for _, sec := range sections {
		if sec.name == "" {
			for _, kv := range sec.lines {
				switch kv.key {
				case "pack_pattern":
					proj.PackPattern = kv.val
				}
			}
		}
	}

	// 解析 [build.<platform>] section — 匹配 HostPlatform
	buildSection := findSection(sections, "build."+c.HostPlatform)
	if buildSection == nil {
		return nil, nil, fmt.Errorf("missing [build.%s] section (deploy-agent is running on %s)", c.HostPlatform, c.HostPlatform)
	}

	for _, kv := range buildSection.lines {
		switch kv.key {
		case "project_dir":
			proj.ProjectDir = kv.val
		case "pack_script":
			proj.PackScript = kv.val
		case "pack_pattern":
			proj.PackPattern = kv.val
		}
	}

	// 验证 project_dir
	if proj.ProjectDir == "" {
		return nil, nil, fmt.Errorf("project_dir is required in [build.%s]", c.HostPlatform)
	}
	if !filepath.IsAbs(proj.ProjectDir) {
		abs, err := filepath.Abs(proj.ProjectDir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve project_dir: %v", err)
		}
		proj.ProjectDir = abs
	}
	if info, err := os.Stat(proj.ProjectDir); err != nil || !info.IsDir() {
		return nil, nil, fmt.Errorf("project_dir does not exist or is not a directory: %s", proj.ProjectDir)
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

	// 解析 [target.*] sections
	var targets []*Target
	for _, sec := range sections {
		if !strings.HasPrefix(sec.name, "target.") {
			continue
		}

		t, err := c.parseTargetSection(sec)
		if err != nil {
			return nil, nil, fmt.Errorf("[%s]: %v", sec.name, err)
		}
		if t != nil {
			targets = append(targets, t)
		}
	}

	return proj, targets, nil
}

// parseTargetSection 解析 [target.*] section
// 支持两种格式：
//
//	[target.local.<platform>]  — 本机部署（platform 从 section 名提取）
//	[target.<name>]            — SSH 远程部署（platform 从 platform= 字段读取）
func (c *DeployConfig) parseTargetSection(sec *configSection) (*Target, error) {
	// 解析 section 名：target.<name> 或 target.local.<platform>
	parts := strings.SplitN(sec.name, ".", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid target section name: %s", sec.name)
	}

	targetBaseName := parts[1] // "local" 或 "ssh-prod" 等
	isLocal := targetBaseName == "local"

	var targetName string
	var targetPlatform string

	if isLocal && len(parts) == 3 {
		// [target.local.win] → 本机部署，平台从 section 名提取
		targetPlatform = parts[2]
		targetName = "local"
	} else if isLocal && len(parts) == 2 {
		// [target.local] — 无平台限定的 local target
		targetName = "local"
		targetPlatform = c.HostPlatform
	} else {
		// [target.ssh-prod] — SSH 远程部署
		targetName = strings.Join(parts[1:], ".")
	}

	var (
		hosts        string
		remoteDir    string
		remoteScript string
		port         = 22
		verifyURL    string
		verifyTmout  = 10
	)

	for _, kv := range sec.lines {
		switch kv.key {
		case "host":
			hosts = kv.val
		case "remote_dir":
			remoteDir = kv.val
		case "remote_script":
			remoteScript = kv.val
		case "ssh_port":
			if n, err := strconv.Atoi(kv.val); err == nil && n > 0 {
				port = n
			}
		case "platform":
			targetPlatform = normalizePlatform(kv.val)
		case "verify_url":
			verifyURL = kv.val
		case "verify_timeout":
			if n, err := strconv.Atoi(kv.val); err == nil && n > 0 {
				verifyTmout = n
			}
		}
	}

	if isLocal {
		// local target: host 自动设为 local
		return &Target{
			Name:          targetName,
			Host:          "local",
			Port:          port,
			RemoteDir:     remoteDir,
			RemoteScript:  remoteScript,
			VerifyURL:     verifyURL,
			VerifyTimeout: verifyTmout,
			Platform:      targetPlatform,
		}, nil
	}

	// SSH target: 需要 host
	if hosts == "" {
		return nil, fmt.Errorf("host is required for ssh target %q", targetName)
	}

	// SSH target: 需要 platform
	if targetPlatform == "" {
		return nil, fmt.Errorf("platform is required for ssh target %q", targetName)
	}

	// 支持多 IP（逗号分隔）
	hostList := strings.Split(hosts, ",")
	if len(hostList) == 1 {
		return &Target{
			Name:          targetName,
			Host:          strings.TrimSpace(hostList[0]),
			Port:          port,
			RemoteDir:     remoteDir,
			RemoteScript:  remoteScript,
			VerifyURL:     verifyURL,
			VerifyTimeout: verifyTmout,
			Platform:      targetPlatform,
		}, nil
	}

	// 多 IP 时只返回第一个匹配的（后续可扩展为返回多个）
	// 目前的逻辑：多 host 会创建多个 Target，但 parseTargetSection 只返回一个
	// 为了兼容多 host，这里创建第一个
	// TODO: 如果需要多 host 支持，需改为返回 []*Target
	h := strings.TrimSpace(hostList[0])
	return &Target{
		Name:          targetName,
		Host:          h,
		Port:          port,
		RemoteDir:     remoteDir,
		RemoteScript:  remoteScript,
		VerifyURL:     verifyURL,
		VerifyTimeout: verifyTmout,
		Platform:      targetPlatform,
	}, nil
}

// readConfSections 读取 .conf 文件并按 section 分组
func readConfSections(path string) ([]*configSection, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sections []*configSection
	current := &configSection{name: ""} // 全局 section
	sections = append(sections, current)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			if name == "" {
				continue
			}
			current = &configSection{name: name}
			sections = append(sections, current)
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		current.lines = append(current.lines, configLine{
			key: strings.TrimSpace(parts[0]),
			val: strings.TrimSpace(parts[1]),
		})
	}
	return sections, nil
}

// findSection 在 sections 列表中查找指定名称的 section
func findSection(sections []*configSection, name string) *configSection {
	for _, sec := range sections {
		if sec.name == name {
			return sec
		}
	}
	return nil
}

// detectPipelinesDir 自动探测 pipelines/ 目录路径
// 优先 settings_dir/pipelines/，其次 deploy.conf 同目录/pipelines/
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

// parseGlobalKey 解析全局配置键值
func parseGlobalKey(cfg *DeployConfig, key, val string) {
	switch key {
	case "ssh_key":
		cfg.SSHKey = val
	case "ssh_password":
		cfg.SSHPassword = val
	case "server_url":
		cfg.ServerURL = val
	case "agent_name":
		cfg.AgentName = val
	case "auth_token":
		cfg.AuthToken = val
	case "max_concurrent":
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			cfg.MaxConcurrent = n
		}
	case "settings_dir":
		cfg.SettingsDir = val
	case "go_blog_agent_id":
		cfg.GoBackendAgentID = val
	}
}
