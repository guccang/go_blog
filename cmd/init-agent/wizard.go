package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WizardStep represents a step in the setup wizard.
type WizardStep int

const (
	StepWelcome WizardStep = iota
	StepGlobalConfig
	StepDeployTargets
	StepDeployProjects
	StepDeployPipelines
	StepEnvCheck
	StepAgentSelect
	StepAgentConfig
	StepConfigGenerate
	StepAvailability
	StepDone
)

func (s WizardStep) String() string {
	switch s {
	case StepWelcome:
		return "欢迎"
	case StepEnvCheck:
		return "环境检测"
	case StepGlobalConfig:
		return "全局配置"
	case StepDeployTargets:
		return "Deploy Targets"
	case StepDeployProjects:
		return "Deploy Projects"
	case StepDeployPipelines:
		return "Deploy Pipelines"
	case StepAgentSelect:
		return "Agent 选择"
	case StepAgentConfig:
		return "Agent 配置"
	case StepConfigGenerate:
		return "配置生成"
	case StepAvailability:
		return "可用性面板"
	case StepDone:
		return "完成"
	default:
		return "未知"
	}
}

// WizardState holds the entire wizard state, shared between CLI and Web modes.
type WizardState struct {
	CurrentStep WizardStep
	RootDir     string

	// Environment check results
	EnvResults       []SoftwareCheckResult
	TargetEnvResults []TargetEnvResult // 按 target 分组的远程检测结果

	// Global/shared config values
	SharedValues map[string]string // server_url, auth_token, etc.

	// Deploy configuration state (conditional)
	DeployState *DeployConfigState

	// Selected agents to configure
	SelectedAgents []string
	AllSchemas     []AgentSchema

	// Dynamic discovery: agents with JSON config files
	DiscoveredConfigs []AgentConfigInfo
	SkippedAgents     []string

	// Per-agent user-provided values
	AgentValues map[string]map[string]string // agentName -> key -> value

	// Generated configs
	GeneratedConfigs map[string]map[string]any // agentName -> merged values
	WrittenFiles     []string

	// Availability results
	AvailabilityLayers   []LayerStatus
	PipelineAvailResults []PipelineAvailResult
}

// NewWizardState creates a new wizard state.
func NewWizardState(rootDir string) *WizardState {
	ws := &WizardState{
		CurrentStep:       StepWelcome,
		RootDir:           rootDir,
		SharedValues:      make(map[string]string),
		AgentValues:       make(map[string]map[string]string),
		GeneratedConfigs:  make(map[string]map[string]any),
		AllSchemas:        AllAgentSchemas(),
		DiscoveredConfigs: DiscoverAgentConfigs(rootDir),
	}

	// Detect deploy settings and pre-load
	settingsDir, available := DetectDeploySettings(rootDir)
	ws.DeployState = &DeployConfigState{
		Available:   available,
		SettingsDir: settingsDir,
	}
	if available {
		ws.preloadDeployState()
	}

	return ws
}

// preloadDeployState loads existing deploy targets, projects, and pipelines.
func (ws *WizardState) preloadDeployState() {
	ds := ws.DeployState

	// Load existing targets
	targets, err := LoadExistingTargets(ds.SettingsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: load targets.json: %v\n", err)
	}
	if targets == nil {
		targets = make(map[string]DeployTarget)
	}
	ds.Targets = targets

	// Load existing projects
	projects, order, err := LoadExistingProjects(ds.SettingsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: load projects: %v\n", err)
	}
	if projects == nil {
		projects = make(map[string]*DeployProject)
	}
	ds.Projects = projects
	ds.ProjectOrder = order

	// If no targets.json exists, extract from projects
	if len(ds.Targets) == 0 && len(ds.Projects) > 0 {
		ds.Targets = ExtractTargetsFromProjects(ds.Projects)
	}

	// Load existing pipelines
	pipelines, err := LoadExistingPipelines(ds.SettingsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: load pipelines: %v\n", err)
	}
	ds.Pipelines = pipelines

	// Load existing SSH password from deploy-agent.json
	deployAgentPath := filepath.Join(ws.RootDir, "cmd", "deploy-agent", "deploy-agent.json")
	if data, err := os.ReadFile(deployAgentPath); err == nil {
		var cfg map[string]any
		if json.Unmarshal(data, &cfg) == nil {
			if pw, ok := cfg["ssh_password"].(string); ok {
				ds.SSHPassword = pw
			}
		}
	}
}

// AgentHasExistingConfig checks if an agent already has a config file.
func (ws *WizardState) AgentHasExistingConfig(schema *AgentSchema) bool {
	return FileExists(ws.RootDir + "/" + schema.Dir + "/" + schema.ConfigFileName)
}

// MergeAndStoreConfig merges config values for one agent and stores the result.
func (ws *WizardState) MergeAndStoreConfig(schema *AgentSchema) {
	existing, _ := LoadExistingConfig(ws.RootDir, schema)
	userVals := ws.AgentValues[schema.Name]
	if userVals == nil {
		userVals = make(map[string]string)
	}
	merged := MergeConfigs(schema, existing, ws.SharedValues, userVals)
	ws.GeneratedConfigs[schema.Name] = merged
}

// WriteAllConfigs writes all generated configs to disk.
func (ws *WizardState) WriteAllConfigs() error {
	ws.WrittenFiles = nil

	// Write deploy configs (targets.json, projects, pipelines, ssh_password)
	if ws.DeployState != nil && ws.DeployState.Available {
		if err := ws.writeDeployConfigs(); err != nil {
			return err
		}
	}

	// Write discovered (dynamic JSON) configs
	for _, agentName := range ws.SelectedAgents {
		values := ws.GeneratedConfigs[agentName]
		if values == nil {
			continue
		}

		// Try discovered config first
		if info := ws.GetDiscoveredConfig(agentName); info != nil {
			path, err := WriteDiscoveredConfig(ws.RootDir, *info, values)
			if err != nil {
				return fmt.Errorf("写入 %s 配置失败: %v", agentName, err)
			}
			ws.WrittenFiles = append(ws.WrittenFiles, path)
			continue
		}

		// Fallback to schema-based writing
		schema := GetAgentSchema(agentName)
		if schema == nil {
			continue
		}
		path, err := WriteAgentConfig(ws.RootDir, schema, values)
		if err != nil {
			return fmt.Errorf("写入 %s 配置失败: %v", agentName, err)
		}
		ws.WrittenFiles = append(ws.WrittenFiles, path)
	}
	return nil
}

// GetDiscoveredConfig returns the discovered config info for an agent, or nil if not found.
func (ws *WizardState) GetDiscoveredConfig(name string) *AgentConfigInfo {
	for i := range ws.DiscoveredConfigs {
		if ws.DiscoveredConfigs[i].Name == name {
			return &ws.DiscoveredConfigs[i]
		}
	}
	return nil
}

// writeDeployConfigs writes deploy-related config files.
func (ws *WizardState) writeDeployConfigs() error {
	ds := ws.DeployState
	if ds == nil || !ds.Available {
		return nil
	}

	// Write targets.json
	if len(ds.Targets) > 0 {
		path, err := WriteTargetsJSON(ds.SettingsDir, ds.Targets)
		if err != nil {
			return fmt.Errorf("写入 targets.json 失败: %v", err)
		}
		ws.WrittenFiles = append(ws.WrittenFiles, path)
		ds.WrittenFiles = append(ds.WrittenFiles, path)
	}

	// Write project JSONs
	for _, name := range ds.ProjectOrder {
		proj := ds.Projects[name]
		if proj == nil {
			continue
		}
		path, err := WriteProjectJSON(ds.SettingsDir, proj)
		if err != nil {
			return fmt.Errorf("写入 %s 项目配置失败: %v", name, err)
		}
		ws.WrittenFiles = append(ws.WrittenFiles, path)
		ds.WrittenFiles = append(ds.WrittenFiles, path)
	}

	// Write pipeline JSONs
	for _, pipeline := range ds.Pipelines {
		path, err := WritePipelineJSON(ds.SettingsDir, pipeline)
		if err != nil {
			return fmt.Errorf("写入 %s pipeline 失败: %v", pipeline.Name, err)
		}
		ws.WrittenFiles = append(ws.WrittenFiles, path)
		ds.WrittenFiles = append(ds.WrittenFiles, path)
	}

	// Update deploy-agent.json SSH password
	if ds.SSHPassword != "" {
		path, err := UpdateDeployAgentJSON(ws.RootDir, ds.SSHPassword)
		if err != nil {
			return fmt.Errorf("更新 deploy-agent.json 失败: %v", err)
		}
		ws.WrittenFiles = append(ws.WrittenFiles, path)
		ds.WrittenFiles = append(ds.WrittenFiles, path)
	}

	return nil
}
