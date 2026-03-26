package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ── Deploy Target（对应 deploy-agent 的 targetJSON）──

type DeployTarget struct {
	Host          string `json:"host,omitempty"`
	Port          int    `json:"ssh_port,omitempty"`
	Platform      string `json:"platform,omitempty"`
	Type          string `json:"type,omitempty"`
	RemoteDir     string `json:"remote_dir,omitempty"`
	RemoteScript  string `json:"remote_script,omitempty"`
	BridgeURL     string `json:"bridge_url,omitempty"`
	AuthToken     string `json:"auth_token,omitempty"`
}

// ── Deploy Project（对应 deploy-agent 的 projectJSON）──

type DeployProject struct {
	Name         string                        `json:"-"`
	PackPattern  string                        `json:"pack_pattern,omitempty"`
	Build        map[string]DeployBuildConfig   `json:"build"`
	Targets      map[string]DeployProjectTarget `json:"targets"`
	ProtectFiles []string                       `json:"protect_files,omitempty"`
	SetupDirs    []string                       `json:"setup_dirs,omitempty"`
}

type DeployBuildConfig struct {
	ProjectDir  string `json:"project_dir"`
	PackScript  string `json:"pack_script,omitempty"`
	PackPattern string `json:"pack_pattern,omitempty"`
}

type DeployProjectTarget struct {
	Host         string `json:"host,omitempty"`
	Port         int    `json:"ssh_port,omitempty"`
	Platform     string `json:"platform,omitempty"`
	Type         string `json:"type,omitempty"`
	RemoteDir    string `json:"remote_dir,omitempty"`
	RemoteScript string `json:"remote_script,omitempty"`
	BridgeURL    string `json:"bridge_url,omitempty"`
	AuthToken    string `json:"auth_token,omitempty"`
}

// ── Deploy Pipeline（对应 deploy-agent 的 Pipeline）──

type DeployPipeline struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Steps       []DeployPipelineStep `json:"steps"`
}

type DeployPipelineStep struct {
	Project       string `json:"project"`
	Target        string `json:"target,omitempty"`
	BuildPlatform string `json:"build_platform,omitempty"`
	PackOnly      bool   `json:"pack_only,omitempty"`
}

// ── Deploy 配置状态（向导运行时） ──

type DeployConfigState struct {
	Available    bool
	SettingsDir  string
	Targets      map[string]DeployTarget
	SSHPassword  string
	Projects     map[string]*DeployProject
	ProjectOrder []string
	Pipelines    []DeployPipeline
	WrittenFiles []string
}

// DetectDeploySettings 检查 cmd/deploy-agent/settings/ 是否存在
func DetectDeploySettings(rootDir string) (settingsDir string, available bool) {
	settingsDir = filepath.Join(rootDir, "cmd", "deploy-agent", "settings")
	info, err := os.Stat(settingsDir)
	if err == nil && info.IsDir() {
		return settingsDir, true
	}
	return "", false
}

// LoadExistingTargets 读取 settings/targets.json
func LoadExistingTargets(settingsDir string) (map[string]DeployTarget, error) {
	path := filepath.Join(settingsDir, "targets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read targets.json: %v", err)
	}

	var targets map[string]DeployTarget
	if err := json.Unmarshal(data, &targets); err != nil {
		return nil, fmt.Errorf("parse targets.json: %v", err)
	}
	return targets, nil
}

// LoadExistingProjects 读取 settings/projects/*.json
func LoadExistingProjects(settingsDir string) (map[string]*DeployProject, []string, error) {
	projectsDir := filepath.Join(settingsDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read projects dir: %v", err)
	}

	projects := make(map[string]*DeployProject)
	var order []string

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

	for _, entry := range jsonFiles {
		filePath := filepath.Join(projectsDir, entry.Name())
		projName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %v", entry.Name(), err)
		}

		var proj DeployProject
		if err := json.Unmarshal(data, &proj); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %v", entry.Name(), err)
		}
		proj.Name = projName

		projects[projName] = &proj
		order = append(order, projName)
	}

	return projects, order, nil
}

// LoadExistingPipelines 读取 settings/pipelines/*.json
func LoadExistingPipelines(settingsDir string) ([]DeployPipeline, error) {
	pipelinesDir := filepath.Join(settingsDir, "pipelines")
	entries, err := os.ReadDir(pipelinesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pipelines dir: %v", err)
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

	var pipelines []DeployPipeline
	for _, entry := range jsonFiles {
		filePath := filepath.Join(pipelinesDir, entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %v", entry.Name(), err)
		}

		var p DeployPipeline
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse %s: %v", entry.Name(), err)
		}
		if p.Name == "" {
			p.Name = baseName
		}

		pipelines = append(pipelines, p)
	}

	return pipelines, nil
}

// ExtractTargetsFromProjects 从现有 project JSON 中提取 SSH target 信息
// 用于首次生成 targets.json
func ExtractTargetsFromProjects(projects map[string]*DeployProject) map[string]DeployTarget {
	targets := make(map[string]DeployTarget)

	for _, proj := range projects {
		for name, pt := range proj.Targets {
			// 跳过 local target
			if strings.HasPrefix(name, "local") {
				continue
			}
			// 已提取过的跳过
			if _, exists := targets[name]; exists {
				continue
			}
			// 只提取有 host 的 SSH/Bridge target
			if pt.Host == "" && pt.BridgeURL == "" {
				continue
			}
			t := DeployTarget{
				Host:     pt.Host,
				Port:     pt.Port,
				Platform: pt.Platform,
				Type:     pt.Type,
			}
			if t.Type == "" && pt.Host != "" {
				t.Type = "ssh"
			}
			if pt.BridgeURL != "" {
				t.Type = "bridge"
				t.BridgeURL = pt.BridgeURL
				t.AuthToken = pt.AuthToken
			}
			targets[name] = t
		}
	}

	return targets
}

// WriteTargetsJSON 写入 settings/targets.json
func WriteTargetsJSON(settingsDir string, targets map[string]DeployTarget) (string, error) {
	path := filepath.Join(settingsDir, "targets.json")
	data, err := json.MarshalIndent(targets, "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal targets.json: %v", err)
	}
	return path, os.WriteFile(path, append(data, '\n'), 0644)
}

// WriteProjectJSON 写入 settings/projects/<name>.json
func WriteProjectJSON(settingsDir string, project *DeployProject) (string, error) {
	projectsDir := filepath.Join(settingsDir, "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return "", fmt.Errorf("create projects dir: %v", err)
	}

	path := filepath.Join(projectsDir, project.Name+".json")
	data, err := json.MarshalIndent(project, "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal %s: %v", project.Name, err)
	}
	return path, os.WriteFile(path, append(data, '\n'), 0644)
}

// WritePipelineJSON 写入 settings/pipelines/<name>.json
func WritePipelineJSON(settingsDir string, pipeline DeployPipeline) (string, error) {
	pipelinesDir := filepath.Join(settingsDir, "pipelines")
	if err := os.MkdirAll(pipelinesDir, 0755); err != nil {
		return "", fmt.Errorf("create pipelines dir: %v", err)
	}

	path := filepath.Join(pipelinesDir, pipeline.Name+".json")
	data, err := json.MarshalIndent(pipeline, "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal %s: %v", pipeline.Name, err)
	}
	return path, os.WriteFile(path, append(data, '\n'), 0644)
}

// UpdateDeployAgentJSON 更新 deploy-agent.json 的 ssh_password 字段
func UpdateDeployAgentJSON(rootDir string, sshPassword string) (string, error) {
	path := filepath.Join(rootDir, "cmd", "deploy-agent", "deploy-agent.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read deploy-agent.json: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse deploy-agent.json: %v", err)
	}

	cfg["ssh_password"] = sshPassword

	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal deploy-agent.json: %v", err)
	}

	return path, os.WriteFile(path, append(out, '\n'), 0644)
}

// SortedTargetNames 返回按名称排序的 target 名列表
func SortedTargetNames(targets map[string]DeployTarget) []string {
	names := make([]string, 0, len(targets))
	for n := range targets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// SortedProjectNames 返回按名称排序的 project 名列表
func SortedProjectNames(projects map[string]*DeployProject) []string {
	names := make([]string, 0, len(projects))
	for n := range projects {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// DeletePipelineJSON 删除 settings/pipelines/<name>.json
func DeletePipelineJSON(settingsDir string, name string) error {
	path := filepath.Join(settingsDir, "pipelines", name+".json")
	return os.Remove(path)
}
