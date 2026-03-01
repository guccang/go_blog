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
	Name         string // 逻辑名（prod/staging/default-0）
	Host         string // user@host 或 local
	Port         int    // SSH 端口，默认 22
	RemoteDir    string // 部署目录
	RemoteScript string // 发布脚本名（可选）
}

// ProjectConfig 项目级部署配置
type ProjectConfig struct {
	Name          string    // 项目名称（settings 文件名）
	ProjectDir    string    // 项目根目录
	PackScript    string    // 打包脚本路径
	PackPattern   string    // 输出文件名模式（{date} → YYYYMMDD_HHMMSS）
	Targets       []*Target // 部署目标列表
	VerifyURL     string    // 部署验证 URL（HTTP GET）
	VerifyTimeout int       // 验证超时秒数，默认 10
	ConfigFile    string    // 来源 settings 文件路径
}

// DeployConfig 全局部署配置
type DeployConfig struct {
	// 全局 SSH 配置
	SSHKey      string // SSH 密钥路径（可选）
	SSHPassword string // SSH 密码（daemon 模式可配置，优先级低于 keyring）

	// WebSocket daemon 模式配置（设置 server_url 启用）
	ServerURL     string // go_blog WebSocket 地址
	AgentName     string // Agent 名称（默认使用主机名）
	AuthToken     string // 认证 token
	MaxConcurrent int    // 最大并发部署任务数，默认 1

	// settings 目录（存放项目部署配置文件）
	SettingsDir string // 部署配置目录，每个 .conf 文件对应一个项目

	// 多项目配置
	Projects     map[string]*ProjectConfig // 项目名 → 配置
	ProjectOrder []string                  // 保持声明顺序
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
func LoadConfig(path string) (*DeployConfig, error) {
	cfg := &DeployConfig{
		MaxConcurrent: 1,
		Projects:      make(map[string]*ProjectConfig),
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}
	defer file.Close()

	// 逐行读取，按 section 分组
	globalLines := []configLine{}
	sections := map[string][]configLine{} // section_name → lines
	var sectionOrder []string
	currentSection := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 检测 [section] 头
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

	// 如果没有任何 section，将全部行当作全局配置（可能含旧格式的项目配置）
	if len(sectionOrder) == 0 {
		// 检查是否有项目配置键（project_dir, targets）
		hasProjectKeys := false
		for _, kv := range globalLines {
			if kv.key == "project_dir" || kv.key == "targets" {
				hasProjectKeys = true
				break
			}
		}
		if hasProjectKeys {
			// 旧格式：全局键混在项目键中，创建 default section
			sectionOrder = []string{"default"}
			sections["default"] = globalLines
			for _, kv := range globalLines {
				parseGlobalKey(cfg, kv.key, kv.val)
			}
			globalLines = nil
		}
	}

	// 解析全局配置
	for _, kv := range globalLines {
		parseGlobalKey(cfg, kv.key, kv.val)
	}

	// 解析内联的项目 section（兼容旧格式）
	for _, name := range sectionOrder {
		proj, err := parseProjectSection(name, sections[name])
		if err != nil {
			return nil, fmt.Errorf("project [%s]: %v", name, err)
		}
		proj.ConfigFile = path
		cfg.Projects[name] = proj
		cfg.ProjectOrder = append(cfg.ProjectOrder, name)
	}

	// settings_dir 模式：扫描目录下的 .conf 文件作为项目配置
	if cfg.SettingsDir != "" {
		// 相对路径基于主配置文件所在目录
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

	return cfg, nil
}

// loadSettingsDir 扫描 settings 目录，每个 .conf 文件加载为一个项目
// 项目名 = 文件名（去掉 .conf 后缀）
func (c *DeployConfig) loadSettingsDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %v", err)
	}

	// 收集并排序
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

		// 检查重名
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
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	defer file.Close()

	var lines []configLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 跳过 [section] 头（settings 文件不需要 section）
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
	}
}

// parseProjectSection 解析项目配置的键值对列表
func parseProjectSection(name string, lines []configLine) (*ProjectConfig, error) {
	proj := &ProjectConfig{
		Name:          name,
		PackPattern:   name + "_{date}.zip",
		VerifyTimeout: 10,
	}

	var (
		simpleTargets string
		simpleDir     string
		simpleScript  string
		simplePort    = 22
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
			proj.VerifyURL = val
		case key == "verify_timeout":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				proj.VerifyTimeout = n
			}

		// SSH 相关（项目级可覆盖）
		case key == "ssh_port":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				simplePort = n
			}

		// 简单目标模式
		case key == "targets":
			simpleTargets = val
		case key == "remote_dir":
			simpleDir = val
		case key == "remote_script":
			simpleScript = val

		// target.<name>.<field> 多目标独立配置
		case strings.HasPrefix(key, "target."):
			segs := strings.SplitN(key, ".", 3)
			if len(segs) != 3 {
				continue
			}
			tname, field := segs[1], segs[2]
			t, ok := namedTargets[tname]
			if !ok {
				t = &Target{Name: tname, Port: 22}
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

		// 兼容旧格式中的全局键
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
			proj.Targets = append(proj.Targets, t)
		}
	} else if simpleTargets != "" {
		for i, host := range strings.Split(simpleTargets, ",") {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			proj.Targets = append(proj.Targets, &Target{
				Name:         fmt.Sprintf("default-%d", i),
				Host:         host,
				Port:         simplePort,
				RemoteDir:    simpleDir,
				RemoteScript: simpleScript,
			})
		}
	}

	// 验证 project_dir
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

	// 验证至少有一个部署目标
	if len(proj.Targets) == 0 {
		return nil, fmt.Errorf("at least one deploy target is required")
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
