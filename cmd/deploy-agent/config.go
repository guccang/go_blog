package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Target 部署目标
type Target struct {
	Name         string // 逻辑名（prod/staging/default-0）
	Host         string // user@host
	Port         int    // SSH 端口，默认 22
	RemoteDir    string // 远程部署目录
	RemoteScript string // 发布脚本名（可选）
}

// DeployConfig 部署配置
type DeployConfig struct {
	ProjectDir  string    // 项目根目录
	PackScript  string    // 打包脚本路径
	PackPattern string    // 输出文件名模式（{date} → YYYYMMDD_HHMMSS）
	SSHKey      string    // SSH 密钥路径（可选）
	Targets     []*Target // 部署目标列表
}

// LoadConfig 从配置文件加载配置
func LoadConfig(path string) (*DeployConfig, error) {
	cfg := &DeployConfig{
		PackPattern: "go_blog_{date}.zip",
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %v", err)
	}
	defer file.Close()

	// 简单键值 + target.前缀键
	var (
		simpleTargets string // targets= 逗号列表
		simpleDir     string // remote_dir=
		simpleScript  string // remote_script=
		simplePort    = 22
	)
	namedTargets := make(map[string]*Target) // target.<name>.<field>

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

		switch {
		case key == "project_dir":
			cfg.ProjectDir = val
		case key == "pack_script":
			cfg.PackScript = val
		case key == "pack_pattern":
			cfg.PackPattern = val
		case key == "ssh_key":
			cfg.SSHKey = val
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

		// target.<name>.<field> 多目标独立配置
		case strings.HasPrefix(key, "target."):
			segs := strings.SplitN(key, ".", 3)
			if len(segs) != 3 {
				continue
			}
			name, field := segs[1], segs[2]
			t, ok := namedTargets[name]
			if !ok {
				t = &Target{Name: name, Port: 22}
				namedTargets[name] = t
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
		}
	}

	// 构建 target 列表：优先使用 target.<name> 模式
	if len(namedTargets) > 0 {
		for _, t := range namedTargets {
			if t.Host == "" {
				return nil, fmt.Errorf("target.%s.host is required", t.Name)
			}
			cfg.Targets = append(cfg.Targets, t)
		}
	} else if simpleTargets != "" {
		for i, host := range strings.Split(simpleTargets, ",") {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			cfg.Targets = append(cfg.Targets, &Target{
				Name:         fmt.Sprintf("default-%d", i),
				Host:         host,
				Port:         simplePort,
				RemoteDir:    simpleDir,
				RemoteScript: simpleScript,
			})
		}
	}

	// ---- 验证 ----
	if cfg.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}
	if !filepath.IsAbs(cfg.ProjectDir) {
		abs, err := filepath.Abs(cfg.ProjectDir)
		if err != nil {
			return nil, fmt.Errorf("resolve project_dir: %v", err)
		}
		cfg.ProjectDir = abs
	}
	if info, err := os.Stat(cfg.ProjectDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("project_dir does not exist or is not a directory: %s", cfg.ProjectDir)
	}
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("at least one deploy target is required")
	}

	// PackScript 自动检测
	if cfg.PackScript == "" {
		if runtime.GOOS == "windows" {
			cfg.PackScript = filepath.Join(cfg.ProjectDir, "pack.bat")
		} else {
			cfg.PackScript = filepath.Join(cfg.ProjectDir, "pack.sh")
		}
	} else if !filepath.IsAbs(cfg.PackScript) {
		cfg.PackScript = filepath.Join(cfg.ProjectDir, cfg.PackScript)
	}

	return cfg, nil
}
