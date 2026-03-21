package main

import "fmt"

// WizardStep represents a step in the setup wizard.
type WizardStep int

const (
	StepWelcome WizardStep = iota
	StepEnvCheck
	StepGlobalConfig
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

	// Step 2: Environment check results
	EnvResults []SoftwareCheckResult

	// Step 3: Global/shared config values
	SharedValues map[string]string // server_url, auth_token, etc.

	// Step 4: Selected agents to configure
	SelectedAgents []string
	AllSchemas     []AgentSchema

	// Step 5: Per-agent user-provided values
	AgentValues map[string]map[string]string // agentName -> key -> value

	// Step 6: Generated configs
	GeneratedConfigs map[string]map[string]any // agentName -> merged values
	WrittenFiles     []string

	// Step 7: Availability results
	AvailabilityLayers []LayerStatus
}

// NewWizardState creates a new wizard state.
func NewWizardState(rootDir string) *WizardState {
	return &WizardState{
		CurrentStep:      StepWelcome,
		RootDir:          rootDir,
		SharedValues:     make(map[string]string),
		AgentValues:      make(map[string]map[string]string),
		GeneratedConfigs: make(map[string]map[string]any),
		AllSchemas:       AllAgentSchemas(),
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
	for _, agentName := range ws.SelectedAgents {
		schema := GetAgentSchema(agentName)
		if schema == nil {
			continue
		}
		values := ws.GeneratedConfigs[agentName]
		if values == nil {
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
