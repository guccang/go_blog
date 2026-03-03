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
	Name          string // 逻辑名（local, ssh-prod, ssh-staging, default-0）
	Host          string // user@host 或 local
	Port          int    // SSH 端口，默认 22
	RemoteDir     string // 部署目录
	RemoteScript  string // 发布脚本名（可选）
	VerifyURL     string // 部署验证 URL（可选，每个 target 独立）
	VerifyTimeout int    // 验证超时秒数，默认 10
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
	BuildPlatform string   // 打包平台（默认当前平台，支持交叉编译）
	TargetFilter  string   // 发布目标过滤（默认 local，可选 all 或具体 target 名）
	TargetNames   []string // 可用的 target 名称列表（从 publish/ 扫描）

	// 多项目配置
	Projects     map[string]*ProjectConfig // 项目名 → 配置
	ProjectOrder []string                  // 保持声明顺序

	// UAP gateway 配置
	GoBackendAgentID string // go_blog-agent 在 gateway 中的 ID，默认 "go_blog"
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

// LoadConfig 从配置文件加载全局配置
// 支持两种模式：
//  1. settings_dir 模式：deploy.conf 仅全局配置 + settings_dir 指向项目配置目录
//  2. 内联模式（兼容旧格式）：deploy.conf 中直接包含 [project] section
func LoadConfig(path string, buildPlatform, targetFilter string) (*DeployConfig, error) {
	cfg := &DeployConfig{
		MaxConcurrent: 1,
		Projects:      make(map[string]*ProjectConfig),
		BuildPlatform: buildPlatform,
		TargetFilter:  targetFilter,
	}

	// 默认打包平台 = 当前平台
	if cfg.BuildPlatform == "" {
		cfg.BuildPlatform = platformSubdir()
	}
	// 默认发布目标 = local
	if cfg.TargetFilter == "" {
		cfg.TargetFilter = "local"
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}
	defer file.Close()

	// 逐行读取，按 section 分组
	globalLines := []configLine{}
	sections := map[string][]configLine{}
	var sectionOrder []string
	currentSection := ""

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
			currentSection = name
			if _, exists := sections[name]; !exists {
				sectionOrder = append(sectionOrder, name)
				sections[name] = []configLine{}
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		kv := configLine{
			key: strings.TrimSpace(parts[0]),
			val: strings.TrimSpace(parts[1]),
		}
		if currentSection == "" {
			globalLines = append(globalLines, kv)
		} else {
			sections[currentSection] = append(sections[currentSection], kv)
		}
	}

	// 兼容旧格式：无 section 但有项目键
	if len(sectionOrder) == 0 {
		hasProjectKeys := false
		for _, kv := range globalLines {
			if kv.key == "project_dir" || kv.key == "targets" {
				hasProjectKeys = true
				break
			}
		}
		if hasProjectKeys {
			sectionOrder = []string{"default"}
			sections["default"] = globalLines
			for _, kv := range globalLines {
				parseGlobalKey(cfg, kv.key, kv.val)
			}
			globalLines = nil
		}
	}

	for _, kv := range globalLines {
		parseGlobalKey(cfg, kv.key, kv.val)
	}

	// 内联项目 section
	for _, name := range sectionOrder {
		proj, err := parseProjectSection(name, sections[name])
		if err != nil {
			return nil, fmt.Errorf("project [%s]: %v", name, err)
		}
		proj.ConfigFile = path
		cfg.Projects[name] = proj
		cfg.ProjectOrder = append(cfg.ProjectOrder, name)
	}

	// settings_dir 模式
	if cfg.SettingsDir != "" {
		settingsDir := cfg.SettingsDir
		if !filepath.IsAbs(settingsDir) {
			settingsDir = filepath.Join(filepath.Dir(path), settingsDir)
		}
		if err := cfg.loadSettingsDir(settingsDir); err != nil {
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
		return nil, fmt.Errorf("no projects found (check settings_dir or [project] sections)")
	}
	if cfg.GoBackendAgentID == "" {
		cfg.GoBackendAgentID = "go_blog"
	}

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

// loadSettingsDir 扫描 settings 目录加载项目配置
// 支持三种目录结构：
//  1. 新模式：build/<platform>/ + publish/<target>/<platform>/
//  2. 旧分离模式：build/<platform>/ + publish/<platform>/
//  3. 兼容模式：<platform>/*.conf 或直接 *.conf
func (c *DeployConfig) loadSettingsDir(dir string) error {
	publishDir := filepath.Join(dir, "publish")
	buildBaseDir := filepath.Join(dir, "build")

	// 检测 publish/ 下是否有 target 子目录（新模式）
	if dirExists(publishDir) && dirExists(buildBaseDir) {
		if c.isNewPublishLayout(publishDir) {
			return c.loadNewSettings(dir)
		}
		// 旧分离模式：publish/<platform>/
		buildDir := filepath.Join(buildBaseDir, c.BuildPlatform)
		pubDir := filepath.Join(publishDir, platformSubdir())
		if dirExists(buildDir) || dirExists(pubDir) {
			return c.loadSplitSettings(buildDir, pubDir, dirExists(buildDir), dirExists(pubDir))
		}
	}

	// 兼容模式
	plat := platformSubdir()
	platDir := filepath.Join(dir, plat)
	if dirExists(platDir) {
		dir = platDir
	}
	return c.loadFlatSettings(dir)
}

// isNewPublishLayout 检测 publish/ 下是否为新的 target 子目录结构
// 新模式：publish/ 下的子目录不是平台名（win/macos/linux），而是 target 名（local/ssh-prod/...）
func (c *DeployConfig) isNewPublishLayout(publishDir string) bool {
	entries, err := os.ReadDir(publishDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 如果子目录是平台名 → 旧模式
		switch name {
		case "win", "macos", "linux":
			continue
		default:
			// 非平台名的子目录 → 新模式（如 local, ssh-prod）
			return true
		}
	}
	return false
}

// loadNewSettings 新模式：build/<platform>/ + publish/<target>/<platform>/
func (c *DeployConfig) loadNewSettings(dir string) error {
	buildDir := filepath.Join(dir, "build", c.BuildPlatform)
	publishBaseDir := filepath.Join(dir, "publish")

	// 读取 build 配置
	buildConfs := map[string][]configLine{}
	buildFiles := map[string]string{}
	if dirExists(buildDir) {
		confs, files, err := readConfDir(buildDir)
		if err != nil {
			return fmt.Errorf("read build/%s: %v", c.BuildPlatform, err)
		}
		buildConfs = confs
		buildFiles = files
	}

	// 扫描 publish/ 下所有 target 目录
	targetDirs, err := os.ReadDir(publishBaseDir)
	if err != nil {
		return fmt.Errorf("read publish dir: %v", err)
	}

	var allTargetNames []string
	for _, entry := range targetDirs {
		if entry.IsDir() {
			allTargetNames = append(allTargetNames, entry.Name())
		}
	}
	sort.Strings(allTargetNames)
	c.TargetNames = allTargetNames

	// 根据 TargetFilter 过滤
	var selectedTargets []string
	if c.TargetFilter == "all" {
		selectedTargets = allTargetNames
	} else {
		// 检查指定的 target 是否存在
		found := false
		for _, t := range allTargetNames {
			if t == c.TargetFilter {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("target %q not found, available: %v", c.TargetFilter, allTargetNames)
		}
		selectedTargets = []string{c.TargetFilter}
	}

	// 收集所有项目名（从 build 配置）
	var projectNames []string
	for n := range buildConfs {
		projectNames = append(projectNames, n)
	}
	sort.Strings(projectNames)

	// 为每个项目构建 ProjectConfig
	for _, projName := range projectNames {
		if _, exists := c.Projects[projName]; exists {
			return fmt.Errorf("duplicate project [%s]: already defined", projName)
		}

		// 解析 build 配置
		proj, err := parseBuildConf(projName, buildConfs[projName])
		if err != nil {
			return fmt.Errorf("project [%s] build: %v", projName, err)
		}

		// 收集来源文件
		var sources []string
		if f, ok := buildFiles[projName]; ok {
			sources = append(sources, f)
		}

		// 为每个选中的 target 加载 publish 配置
		for _, targetName := range selectedTargets {
			targets, pubFile, err := c.loadPublishTarget(publishBaseDir, targetName, projName)
			if err != nil {
				return fmt.Errorf("project [%s] publish/%s: %v", projName, targetName, err)
			}
			if pubFile != "" {
				sources = append(sources, pubFile)
			}
			proj.Targets = append(proj.Targets, targets...)
		}

		proj.ConfigFile = strings.Join(sources, " + ")

		// 没有 target 配置的项目跳过（该 target 下没有这个项目的 conf）
		if len(proj.Targets) == 0 {
			continue
		}

		c.Projects[projName] = proj
		c.ProjectOrder = append(c.ProjectOrder, projName)
	}

	return nil
}

// loadPublishTarget 加载 publish/<targetName>/<platform>/<projName>.conf
func (c *DeployConfig) loadPublishTarget(publishBaseDir, targetName, projName string) ([]*Target, string, error) {
	targetDir := filepath.Join(publishBaseDir, targetName)
	if !dirExists(targetDir) {
		return nil, "", nil
	}

	isLocal := strings.HasPrefix(targetName, "local")

	// 确定平台子目录：local target 用当前平台，ssh target 扫描所有平台
	var platDirs []string
	if isLocal {
		platDirs = []string{platformSubdir()}
	} else {
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			return nil, "", err
		}
		for _, e := range entries {
			if e.IsDir() {
				platDirs = append(platDirs, e.Name())
			}
		}
	}

	var targets []*Target
	var pubFile string

	for _, plat := range platDirs {
		confPath := filepath.Join(targetDir, plat, projName+".conf")
		if _, err := os.Stat(confPath); err != nil {
			continue // 该平台下没有这个项目的配置
		}

		lines, err := readConfigLines(confPath)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", confPath, err)
		}
		pubFile = confPath

		parsed, err := parsePublishConf(targetName, isLocal, lines)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", confPath, err)
		}
		targets = append(targets, parsed...)
	}

	return targets, pubFile, nil
}

// parseBuildConf 解析 build 配置（只含 project_dir, pack_script, pack_pattern）
func parseBuildConf(name string, lines []configLine) (*ProjectConfig, error) {
	proj := &ProjectConfig{
		Name:        name,
		PackPattern: name + "_{date}.zip",
	}

	for _, kv := range lines {
		switch kv.key {
		case "project_dir":
			proj.ProjectDir = kv.val
		case "pack_script":
			proj.PackScript = kv.val
		case "pack_pattern":
			proj.PackPattern = kv.val
		}
	}

	if proj.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}
	if !filepath.IsAbs(proj.ProjectDir) {
		abs, err := filepath.Abs(proj.ProjectDir)
		if err != nil {
			return nil, fmt.Errorf("resolve project_dir: %v", err)
		}
		proj.ProjectDir = abs
	}
	if info, err := os.Stat(proj.ProjectDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("project_dir does not exist or is not a directory: %s", proj.ProjectDir)
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

	return proj, nil
}

// parsePublishConf 解析 publish 配置为 Target 列表
// local target: host 自动设为 local
// ssh target: 支持 host= 多 IP 逗号分隔
func parsePublishConf(targetName string, isLocal bool, lines []configLine) ([]*Target, error) {
	var (
		hosts        string
		remoteDir    string
		remoteScript string
		port         = 22
		verifyURL    string
		verifyTmout  = 10
	)

	for _, kv := range lines {
		switch kv.key {
		case "host":
			hosts = kv.val
		case "targets": // 兼容旧字段名
			hosts = kv.val
		case "remote_dir":
			remoteDir = kv.val
		case "remote_script":
			remoteScript = kv.val
		case "ssh_port":
			if n, err := strconv.Atoi(kv.val); err == nil && n > 0 {
				port = n
			}
		case "verify_url":
			verifyURL = kv.val
		case "verify_timeout":
			if n, err := strconv.Atoi(kv.val); err == nil && n > 0 {
				verifyTmout = n
			}
		}
	}

	if isLocal {
		// local target 不需要 host，自动设为 local
		return []*Target{{
			Name:          targetName,
			Host:          "local",
			Port:          port,
			RemoteDir:     remoteDir,
			RemoteScript:  remoteScript,
			VerifyURL:     verifyURL,
			VerifyTimeout: verifyTmout,
		}}, nil
	}

	// SSH target: 支持多 IP
	if hosts == "" {
		return nil, fmt.Errorf("host is required for ssh target %q", targetName)
	}

	hostList := strings.Split(hosts, ",")
	var targets []*Target
	for i, h := range hostList {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		name := targetName
		if len(hostList) > 1 {
			name = fmt.Sprintf("%s-%d", targetName, i)
		}
		targets = append(targets, &Target{
			Name:          name,
			Host:          h,
			Port:          port,
			RemoteDir:     remoteDir,
			RemoteScript:  remoteScript,
			VerifyURL:     verifyURL,
			VerifyTimeout: verifyTmout,
		})
	}

	return targets, nil
}

// --- 旧模式兼容函数 ---

// loadSplitSettings 旧分离模式：build/<platform>/ + publish/<platform>/
func (c *DeployConfig) loadSplitSettings(buildDir, publishDir string, buildExists, publishExists bool) error {
	buildConfs := map[string][]configLine{}
	buildFiles := map[string]string{}
	if buildExists {
		confs, files, err := readConfDir(buildDir)
		if err != nil {
			return fmt.Errorf("read build dir: %v", err)
		}
		buildConfs = confs
		buildFiles = files
	}

	publishConfs := map[string][]configLine{}
	publishFiles := map[string]string{}
	if publishExists {
		confs, files, err := readConfDir(publishDir)
		if err != nil {
			return fmt.Errorf("read publish dir: %v", err)
		}
		publishConfs = confs
		publishFiles = files
	}

	nameSet := map[string]bool{}
	for n := range buildConfs {
		nameSet[n] = true
	}
	for n := range publishConfs {
		nameSet[n] = true
	}
	var names []string
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		if _, exists := c.Projects[name]; exists {
			return fmt.Errorf("duplicate project [%s]: already defined", name)
		}
		var merged []configLine
		merged = append(merged, buildConfs[name]...)
		merged = append(merged, publishConfs[name]...)

		proj, err := parseProjectSection(name, merged)
		if err != nil {
			return fmt.Errorf("project [%s]: %v", name, err)
		}
		var sources []string
		if f, ok := buildFiles[name]; ok {
			sources = append(sources, f)
		}
		if f, ok := publishFiles[name]; ok {
			sources = append(sources, f)
		}
		proj.ConfigFile = strings.Join(sources, " + ")
		c.Projects[name] = proj
		c.ProjectOrder = append(c.ProjectOrder, name)
	}
	return nil
}

// readConfDir 读取目录下所有 .conf 文件，返回 项目名→键值对 和 项目名→文件路径
func readConfDir(dir string) (map[string][]configLine, map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	confs := map[string][]configLine{}
	files := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".conf") {
			continue
		}
		projectName := strings.TrimSuffix(name, filepath.Ext(name))
		filePath := filepath.Join(dir, name)
		lines, err := readConfigLines(filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", name, err)
		}
		confs[projectName] = lines
		files[projectName] = filePath
	}
	return confs, files, nil
}

// readConfigLines 读取 .conf 文件返回键值对列表
func readConfigLines(path string) ([]configLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []configLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		lines = append(lines, configLine{
			key: strings.TrimSpace(parts[0]),
			val: strings.TrimSpace(parts[1]),
		})
	}
	return lines, nil
}

// loadFlatSettings 兼容旧模式：单目录下 *.conf 各自包含完整项目配置
func (c *DeployConfig) loadFlatSettings(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".conf") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		filePath := filepath.Join(dir, name)
		projectName := strings.TrimSuffix(name, filepath.Ext(name))
		if _, exists := c.Projects[projectName]; exists {
			return fmt.Errorf("duplicate project [%s]: already defined, conflicts with settings file %q", projectName, filePath)
		}
		proj, err := loadProjectFile(projectName, filePath)
		if err != nil {
			return fmt.Errorf("settings %q: %v", name, err)
		}
		proj.ConfigFile = filePath
		c.Projects[projectName] = proj
		c.ProjectOrder = append(c.ProjectOrder, projectName)
	}
	return nil
}

// loadProjectFile 从单个 .conf 文件加载项目配置（无 section，纯键值对）
func loadProjectFile(name, path string) (*ProjectConfig, error) {
	lines, err := readConfigLines(path)
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	return parseProjectSection(name, lines)
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

// parseProjectSection 解析项目配置的键值对列表（兼容旧格式）
func parseProjectSection(name string, lines []configLine) (*ProjectConfig, error) {
	proj := &ProjectConfig{
		Name:        name,
		PackPattern: name + "_{date}.zip",
	}

	var (
		simpleTargets string
		simpleDir     string
		simpleScript  string
		simplePort    = 22
		verifyURL     string
		verifyTimeout = 10
	)
	namedTargets := make(map[string]*Target)

	for _, kv := range lines {
		key, val := kv.key, kv.val
		switch {
		case key == "project_dir":
			proj.ProjectDir = val
		case key == "pack_script":
			proj.PackScript = val
		case key == "pack_pattern":
			proj.PackPattern = val
		case key == "verify_url":
			verifyURL = val
		case key == "verify_timeout":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				verifyTimeout = n
			}
		case key == "ssh_port":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				simplePort = n
			}
		case key == "targets":
			simpleTargets = val
		case key == "remote_dir":
			simpleDir = val
		case key == "remote_script":
			simpleScript = val
		case strings.HasPrefix(key, "target."):
			segs := strings.SplitN(key, ".", 3)
			if len(segs) != 3 {
				continue
			}
			tname, field := segs[1], segs[2]
			t, ok := namedTargets[tname]
			if !ok {
				t = &Target{Name: tname, Port: 22, VerifyTimeout: 10}
				namedTargets[tname] = t
			}
			switch field {
			case "host":
				t.Host = val
			case "remote_dir":
				t.RemoteDir = val
			case "remote_script":
				t.RemoteScript = val
			case "port":
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					t.Port = n
				}
			}
		case key == "ssh_key" || key == "ssh_password" ||
			key == "server_url" || key == "agent_name" ||
			key == "auth_token" || key == "max_concurrent" ||
			key == "settings_dir":
			// 全局键，在项目 section 中忽略
		}
	}

	// 构建 target 列表
	if len(namedTargets) > 0 {
		for _, t := range namedTargets {
			if t.Host == "" {
				return nil, fmt.Errorf("target.%s.host is required", t.Name)
			}
			t.VerifyURL = verifyURL
			t.VerifyTimeout = verifyTimeout
			proj.Targets = append(proj.Targets, t)
		}
	} else if simpleTargets != "" {
		for i, host := range strings.Split(simpleTargets, ",") {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			proj.Targets = append(proj.Targets, &Target{
				Name:          fmt.Sprintf("default-%d", i),
				Host:          host,
				Port:          simplePort,
				RemoteDir:     simpleDir,
				RemoteScript:  simpleScript,
				VerifyURL:     verifyURL,
				VerifyTimeout: verifyTimeout,
			})
		}
	}

	// 兼容：VerifyURL 也存到 ProjectConfig（旧代码可能读这里）
	proj.VerifyURL = verifyURL

	if proj.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}
	if !filepath.IsAbs(proj.ProjectDir) {
		abs, err := filepath.Abs(proj.ProjectDir)
		if err != nil {
			return nil, fmt.Errorf("resolve project_dir: %v", err)
		}
		proj.ProjectDir = abs
	}
	if info, err := os.Stat(proj.ProjectDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("project_dir does not exist or is not a directory: %s", proj.ProjectDir)
	}

	if len(proj.Targets) == 0 {
		return nil, fmt.Errorf("at least one deploy target is required")
	}

	if proj.PackScript == "" {
		if runtime.GOOS == "windows" {
			proj.PackScript = filepath.Join(proj.ProjectDir, "pack.bat")
		} else {
			proj.PackScript = filepath.Join(proj.ProjectDir, "pack.sh")
		}
	} else if !filepath.IsAbs(proj.PackScript) {
		proj.PackScript = filepath.Join(proj.ProjectDir, proj.PackScript)
	}

	return proj, nil
}
