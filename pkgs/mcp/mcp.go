package mcp

import (
	"control"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"module"
	log "mylog"
	"strings"
	"time"
)

var mcp_version = "Version2.0"
var toolNameMapping = make(map[string]string)

// ToolCall represents a function call
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function call details
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// LLMTool represents a tool available to the LLM
type LLMTool struct {
	Type     string      `json:"type"`
	Function LLMFunction `json:"function"`
}

// LLMFunction represents the function definition for LLM
type LLMFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

func Info() {
	fmt.Println("info mcp v2.0 - LLM Agent with Tool Calling")
}

func extractFunctionName(s string) string {
	lastDot := strings.LastIndex(s, ".")
	if lastDot == -1 {
		return s // 如果没有 `.`，返回整个字符串
	}
	var toolName = s[lastDot+1:]
	toolNameMapping[toolName] = s
	return toolName
}

// GetAvailableLLMTools converts MCP tools to LLM format
func GetAvailableLLMTools(selectedTools []string) []LLMTool {
	mcpTools := GetAvailableToolsImproved()
	var llmTools []LLMTool

	if selectedTools == nil || len(selectedTools) == 0 {
		return llmTools
	}

	for _, tool := range mcpTools {
		if selectedTools == nil || contains(selectedTools, tool.Name) {
			llmTool := LLMTool{
				Type: "function",
				Function: LLMFunction{
					// file-system.read_file to read_file
					Name:        extractFunctionName(tool.Name),
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
			llmTools = append(llmTools, llmTool)
		}
	}

	return llmTools
}

// CallMCPTool calls an MCP tool and returns the result
func CallMCPTool(toolName string, arguments map[string]interface{}) MCPToolResponse {
	log.DebugF("toolcall CallMCPTool: %s, arguments: %v", toolName, arguments)
	toolCall := MCPToolCall{
		Name:      toolNameMapping[toolName],
		Arguments: arguments,
	}

	return CallToolImproved(toolCall)
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func GetVersion() string {
	return mcp_version
}

type MCPConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Environment map[string]string `json:"environment"`
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type MCPConfigList struct {
	Configs []MCPConfig `json:"configs"`
}

var mcpConfigs MCPConfigList

func Init() {
	log.Debug("=== MCP Module Initialization Started ===")
	log.DebugF("MCP Version: %s", mcp_version)

	// Load MCP configurations
	log.Debug("Loading MCP configurations...")
	loadMCPConfigs()
	RegisterInnerTools()

	// Log loaded configurations
	log.DebugF("Loaded %d MCP configurations", len(mcpConfigs.Configs))
	for i, config := range mcpConfigs.Configs {
		log.DebugF("MCP Config %d: name=%s, enabled=%t, command=%s",
			i+1, config.Name, config.Enabled, config.Command)
		if len(config.Args) > 0 {
			log.DebugF("  Args: %v", config.Args)
		}
		if len(config.Environment) > 0 {
			log.DebugF("  Environment: %v", config.Environment)
		}
		log.DebugF("  Description: %s", config.Description)
		log.DebugF("  Created: %s, Updated: %s",
			config.CreatedAt.Format("2006-01-02 15:04:05"),
			config.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	// Start the connection pool cleanup routine
	log.Debug("Initializing MCP connection pool...")
	pool := GetPool()
	pool.StartCleanupRoutine()
	log.Debug("MCP connection pool cleanup routine started")

	log.Debug("=== MCP Module Initialization Completed ===")
	log.InfoF("MCP module initialized successfully with %d configurations", len(mcpConfigs.Configs))

	// create mcp server and client
	tools := GetAvailableToolsImproved()
	log.DebugF("MCP module initialized successfully with %d tools", len(tools))
}

func loadMCPConfigs() {
	log.Debug("--- Loading MCP Configurations ---")

	title := getMCPConfigTitle()
	bolg := control.GetBlog(title)
	if bolg == nil {
		control.AddBlog(&module.UploadedBlogData{
			Title:   title,
			Content: "",
		})
		b := control.GetBlog(title)
		if b == nil {
			log.ErrorF("Failed to get blog '%s'", title)
			return
		}
	}

	mcpConfigs = MCPConfigList{}
	err := json.Unmarshal([]byte(bolg.Content), &mcpConfigs)
	if err != nil {
		log.ErrorF("Failed to parse MCP config file '%s': %v", title, err)
		log.Error("Using empty MCP configuration due to parse error")
		return
	}

	// Validate loaded configurations
	validConfigs := 0
	for i, config := range mcpConfigs.Configs {
		if err := ValidateConfig(config); err != nil {
			log.WarnF("MCP Config %d (%s) validation failed: %v", i+1, config.Name, err)
		} else {
			validConfigs++
			log.DebugF("MCP Config %d (%s) validated successfully", i+1, config.Name)
		}
	}

	log.InfoF("MCP configuration validation completed: %d/%d configs valid", validConfigs, len(mcpConfigs.Configs))
}

func getMCPConfigTitle() string {
	return "mcp_config"
}

func createDefaultMCPConfig() {
	log.Debug("--- Creating Default MCP Configuration ---")

	// Create default configuration
	defaultConfig := MCPConfigList{
		Configs: []MCPConfig{
			{
				Name:        "file-system",
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "./blogs_txt"},
				Environment: map[string]string{},
				Enabled:     true,
				Description: "File system MCP server example",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				Name:        "redis",
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-redis", "redis://localhost:6379"},
				Environment: map[string]string{},
				Enabled:     true,
				Description: "Redis MCP server example",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}

	log.DebugF("Creating default config with %d entries", len(defaultConfig.Configs))
	for i, config := range defaultConfig.Configs {
		log.DebugF("Default Config %d: %s (%s) - %s",
			i+1, config.Name, config.Command, config.Description)
	}

	// Write to file
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		log.ErrorF("Failed to marshal default MCP config: %v", err)
		return
	}

	title := getMCPConfigTitle()
	control.ModifyBlog(&module.UploadedBlogData{
		Title:   title,
		Content: string(data),
	})

	mcpConfigs = defaultConfig
	log.InfoF("Successfully created default MCP configuration with %d entries at: %s",
		len(defaultConfig.Configs), title)
}

func GetAllConfigs() []MCPConfig {
	return mcpConfigs.Configs
}

func GetConfig(name string) (*MCPConfig, bool) {
	for i, config := range mcpConfigs.Configs {
		if config.Name == name {
			return &mcpConfigs.Configs[i], true
		}
	}
	return nil, false
}

func AddConfig(config MCPConfig) error {
	log.DebugF("--- Adding MCP Configuration: %s ---", config.Name)
	log.DebugF("Command: %s", config.Command)
	log.DebugF("Args: %v", config.Args)
	log.DebugF("Environment: %v", config.Environment)
	log.DebugF("Enabled: %t", config.Enabled)
	log.DebugF("Description: %s", config.Description)

	// Check if config with same name already exists
	for i, existingConfig := range mcpConfigs.Configs {
		if existingConfig.Name == config.Name {
			log.WarnF("MCP config with name '%s' already exists at index %d", config.Name, i)
			return fmt.Errorf("MCP config with name '%s' already exists", config.Name)
		}
	}

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		log.ErrorF("MCP config validation failed for '%s': %v", config.Name, err)
		return fmt.Errorf("validation failed: %v", err)
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	mcpConfigs.Configs = append(mcpConfigs.Configs, config)

	log.InfoF("MCP config '%s' added successfully, total configs: %d", config.Name, len(mcpConfigs.Configs))

	if err := saveMCPConfigs(); err != nil {
		log.ErrorF("Failed to save MCP configs after adding '%s': %v", config.Name, err)
		return err
	}

	log.InfoF("MCP config '%s' saved to disk successfully", config.Name)
	return nil
}

func UpdateConfig(name string, config MCPConfig) error {
	log.DebugF("--- Updating MCP Configuration: %s ---", name)
	log.DebugF("New Command: %s", config.Command)
	log.DebugF("New Args: %v", config.Args)
	log.DebugF("New Environment: %v", config.Environment)
	log.DebugF("New Enabled: %t", config.Enabled)
	log.DebugF("New Description: %s", config.Description)

	for i, existingConfig := range mcpConfigs.Configs {
		if existingConfig.Name == name {
			log.DebugF("Found MCP config '%s' at index %d", name, i)

			// Log old values for comparison
			log.DebugF("Old Command: %s -> New: %s", existingConfig.Command, config.Command)
			log.DebugF("Old Enabled: %t -> New: %t", existingConfig.Enabled, config.Enabled)

			// Validate new configuration
			if err := ValidateConfig(config); err != nil {
				log.ErrorF("MCP config validation failed for update '%s': %v", name, err)
				return fmt.Errorf("validation failed: %v", err)
			}

			config.Name = name // Preserve original name
			config.CreatedAt = existingConfig.CreatedAt
			config.UpdatedAt = time.Now()
			mcpConfigs.Configs[i] = config

			log.InfoF("MCP config '%s' updated successfully", name)

			if err := saveMCPConfigs(); err != nil {
				log.ErrorF("Failed to save MCP configs after updating '%s': %v", name, err)
				return err
			}

			log.InfoF("MCP config '%s' update saved to disk successfully", name)
			return nil
		}
	}

	log.WarnF("MCP config with name '%s' not found for update", name)
	return fmt.Errorf("MCP config with name '%s' not found", name)
}

func DeleteConfig(name string) error {
	log.DebugF("--- Deleting MCP Configuration: %s ---", name)

	for i, config := range mcpConfigs.Configs {
		if config.Name == name {
			log.DebugF("Found MCP config '%s' at index %d", name, i)
			log.DebugF("Config details - Command: %s, Enabled: %t", config.Command, config.Enabled)

			// Remove from connection pool if exists
			pool := GetPool()
			pool.RemoveClient(name)
			log.DebugF("Removed MCP client '%s' from connection pool", name)

			mcpConfigs.Configs = append(mcpConfigs.Configs[:i], mcpConfigs.Configs[i+1:]...)

			log.InfoF("MCP config '%s' deleted successfully, remaining configs: %d", name, len(mcpConfigs.Configs))

			if err := saveMCPConfigs(); err != nil {
				log.ErrorF("Failed to save MCP configs after deleting '%s': %v", name, err)
				return err
			}

			log.InfoF("MCP config '%s' deletion saved to disk successfully", name)
			return nil
		}
	}

	log.WarnF("MCP config with name '%s' not found for deletion", name)
	return fmt.Errorf("MCP config with name '%s' not found", name)
}

func ToggleConfig(name string) error {
	log.DebugF("--- Toggling MCP Configuration: %s ---", name)

	for i, config := range mcpConfigs.Configs {
		if config.Name == name {
			oldEnabled := config.Enabled
			newEnabled := !oldEnabled

			log.DebugF("Found MCP config '%s' at index %d", name, i)
			log.DebugF("Toggling enabled state: %t -> %t", oldEnabled, newEnabled)

			mcpConfigs.Configs[i].Enabled = newEnabled
			mcpConfigs.Configs[i].UpdatedAt = time.Now()

			// If disabling, remove from connection pool
			if !newEnabled {
				pool := GetPool()
				pool.RemoveClient(name)
				log.DebugF("Disabled MCP config '%s', removed from connection pool", name)
			} else {
				log.DebugF("Enabled MCP config '%s', will be available for connections", name)
			}

			log.InfoF("MCP config '%s' %s successfully", name,
				map[bool]string{true: "enabled", false: "disabled"}[newEnabled])

			if err := saveMCPConfigs(); err != nil {
				log.ErrorF("Failed to save MCP configs after toggling '%s': %v", name, err)
				return err
			}

			log.InfoF("MCP config '%s' toggle saved to disk successfully", name)
			return nil
		}
	}

	log.WarnF("MCP config with name '%s' not found for toggle", name)
	return fmt.Errorf("MCP config with name '%s' not found", name)
}

func saveMCPConfigs() error {
	log.Debug("--- Saving MCP Configurations ---")

	data, err := json.MarshalIndent(mcpConfigs, "", "  ")
	if err != nil {
		log.ErrorF("Failed to marshal MCP configs: %v", err)
		return fmt.Errorf("failed to marshal MCP configs: %v", err)
	}

	title := getMCPConfigTitle()
	control.ModifyBlog(&module.UploadedBlogData{
		Title:   title,
		Content: string(data),
	})

	log.InfoF("Successfully saved %d MCP configurations to %s", len(mcpConfigs.Configs), title)
	return nil
}

// copyFile creates a copy of a file for backup purposes
func copyFile(src, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		return err
	}

	return nil
}

func GetEnabledConfigs() []MCPConfig {
	log.Debug("--- Getting Enabled MCP Configurations ---")

	var enabledConfigs []MCPConfig
	for i, config := range mcpConfigs.Configs {
		if config.Enabled {
			enabledConfigs = append(enabledConfigs, config)
			log.DebugF("Found enabled config %d: %s (%s)", i+1, config.Name, config.Command)
		} else {
			log.DebugF("Skipping disabled config %d: %s", i+1, config.Name)
		}
	}

	log.InfoF("Found %d enabled MCP configurations out of %d total", len(enabledConfigs), len(mcpConfigs.Configs))
	return enabledConfigs
}

func ValidateConfig(config MCPConfig) error {
	log.DebugF("--- Validating MCP Configuration: %s ---", config.Name)

	if config.Name == "" {
		log.ErrorF("MCP config validation failed: name cannot be empty")
		return fmt.Errorf("name cannot be empty")
	}

	if config.Command == "" {
		log.ErrorF("MCP config validation failed: command cannot be empty for '%s'", config.Name)
		return fmt.Errorf("command cannot be empty")
	}

	if strings.TrimSpace(config.Name) != config.Name {
		log.ErrorF("MCP config validation failed: name '%s' has leading or trailing spaces", config.Name)
		return fmt.Errorf("name cannot have leading or trailing spaces")
	}

	// Additional validations
	if len(config.Name) > 50 {
		log.ErrorF("MCP config validation failed: name '%s' is too long (%d chars, max 50)", config.Name, len(config.Name))
		return fmt.Errorf("name cannot be longer than 50 characters")
	}

	if strings.Contains(config.Name, "/") || strings.Contains(config.Name, "\\") {
		log.ErrorF("MCP config validation failed: name '%s' contains invalid characters", config.Name)
		return fmt.Errorf("name cannot contain path separators")
	}

	log.DebugF("MCP config validation passed for '%s'", config.Name)
	return nil
}
