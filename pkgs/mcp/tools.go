package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	log "mylog"
)

// MCPTool represents an MCP tool that can be called
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// MCPToolCall represents a tool call request
type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPToolResponse represents the response from a tool call
type MCPToolResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error,omitempty"`
}

// MCPRequest represents a request to an MCP server
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPServerInfo represents MCP server information
type MCPServerInfo struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Protocol     string                 `json:"protocol"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// MCPConnection represents an active MCP connection
type MCPConnection struct {
	Config     MCPConfig
	Process    *exec.Cmd
	Connected  bool
	LastPing   time.Time
	ServerInfo *MCPServerInfo
}

// MCPConnectionManager manages MCP server connections
type MCPConnectionManager struct {
	connections map[string]*MCPConnection
}

var connectionManager = &MCPConnectionManager{
	connections: make(map[string]*MCPConnection),
}

// GetConnection returns or creates a connection for the given config
func (cm *MCPConnectionManager) GetConnection(config MCPConfig) (*MCPConnection, error) {
	if conn, exists := cm.connections[config.Name]; exists {
		if conn.Connected && time.Since(conn.LastPing) < 30*time.Second {
			return conn, nil
		}
		// Connection is stale, remove it
		cm.CloseConnection(config.Name)
	}
	
	// Create new connection
	return cm.CreateConnection(config)
}

// CreateConnection establishes a new MCP connection
func (cm *MCPConnectionManager) CreateConnection(config MCPConfig) (*MCPConnection, error) {
	conn := &MCPConnection{
		Config:    config,
		Connected: false,
		LastPing:  time.Now(),
	}
	
	// Test the connection
	if err := cm.testConnection(conn); err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server %s: %v", config.Name, err)
	}
	
	conn.Connected = true
	cm.connections[config.Name] = conn
	
	log.DebugF("Established MCP connection to %s", config.Name)
	return conn, nil
}

// CloseConnection closes and removes a connection
func (cm *MCPConnectionManager) CloseConnection(name string) {
	if conn, exists := cm.connections[name]; exists {
		if conn.Process != nil {
			conn.Process.Process.Kill()
		}
		delete(cm.connections, name)
		log.DebugF("Closed MCP connection to %s", name)
	}
}

// testConnection tests if a connection is working
func (cm *MCPConnectionManager) testConnection(conn *MCPConnection) error {
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "go-blog-assistant",
				"version": "1.0.0",
			},
		},
	}
	
	response, err := cm.callMCPServerDirect(conn.Config, request)
	if err != nil {
		return err
	}
	
	// Parse server info
	if result, ok := response.Data.(map[string]interface{}); ok {
		if serverInfo, ok := result["serverInfo"].(map[string]interface{}); ok {
			conn.ServerInfo = &MCPServerInfo{
				Name:         getString(serverInfo, "name"),
				Version:      getString(serverInfo, "version"),
				Protocol:     getString(serverInfo, "protocolVersion"),
				Capabilities: getMap(serverInfo, "capabilities"),
			}
		}
	}
	
	return nil
}

// callMCPServerDirect executes a request to an MCP server directly
func (cm *MCPConnectionManager) callMCPServerDirect(config MCPConfig, request MCPRequest) (MCPResponseStruct, error) {
	var response MCPResponseStruct
	
	// Prepare the command
	cmd := exec.Command(config.Command, config.Args...)
	
	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	
	// Prepare request JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return response, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// Set up stdin/stdout
	cmd.Stdin = bytes.NewReader(requestJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Execute the command with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			log.ErrorF("MCP server command failed: %v, stderr: %s", err, stderr.String())
			return response, fmt.Errorf("command execution failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return response, fmt.Errorf("command execution timed out")
	}
	
	// Parse the response
	var mcpResp struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      string      `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *MCPError   `json:"error,omitempty"`
	}
	
	if err := json.Unmarshal(stdout.Bytes(), &mcpResp); err != nil {
		log.ErrorF("Failed to parse MCP response: %v, output: %s", err, stdout.String())
		return response, fmt.Errorf("failed to parse response: %v", err)
	}
	
	// Convert to our response format
	response.Data = mcpResp.Result
	if mcpResp.Error != nil {
		response.Message = mcpResp.Error.Message
	}
	
	return response, nil
}

// MCPResponseStruct represents our internal response format
type MCPResponseStruct struct {
	Data    interface{}
	Message string
}

// Helper functions
func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if value, ok := m[key]; ok {
		if mapVal, ok := value.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return make(map[string]interface{})
}

// GetAvailableTools returns a list of available MCP tools from enabled configurations
func GetAvailableTools() []MCPTool {
	var tools []MCPTool
	
	enabledConfigs := GetEnabledConfigs()
	
	for _, config := range enabledConfigs {
		// Get tools from each enabled MCP server
		serverTools := getToolsFromServer(config)
		tools = append(tools, serverTools...)
	}
	
	return tools
}

// getToolsFromServer retrieves tools from a specific MCP server
func getToolsFromServer(config MCPConfig) []MCPTool {
	var tools []MCPTool
	
	// Get or create connection
	_, err := connectionManager.GetConnection(config)
	if err != nil {
		log.ErrorF("Failed to get connection for server %s: %v", config.Name, err)
		return tools
	}
	
	// Create MCP request to list tools
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tools/list",
	}
	
	// Call the MCP server
	response, err := connectionManager.callMCPServerDirect(config, request)
	if err != nil {
		log.ErrorF("Failed to get tools from server %s: %v", config.Name, err)
		return tools
	}
	
	// Parse the response
	if result, ok := response.Data.(map[string]interface{}); ok {
		if toolsList, ok := result["tools"].([]interface{}); ok {
			for _, toolData := range toolsList {
				if toolMap, ok := toolData.(map[string]interface{}); ok {
					tool := MCPTool{
						Name:        fmt.Sprintf("%s.%s", config.Name, getString(toolMap, "name")),
						Description: getString(toolMap, "description"),
						Parameters:  toolMap["inputSchema"],
					}
					tools = append(tools, tool)
				}
			}
		}
	}
	
	return tools
}

// CallTool executes an MCP tool call
func CallTool(toolCall MCPToolCall) MCPToolResponse {
	// Parse the tool name to extract server and tool
	parts := strings.SplitN(toolCall.Name, ".", 2)
	if len(parts) != 2 {
		return MCPToolResponse{
			Success: false,
			Error:   "Invalid tool name format. Expected: server.tool",
		}
	}
	
	serverName := parts[0]
	toolName := parts[1]
	
	// Find the server configuration
	config, found := GetConfig(serverName)
	if !found {
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Server configuration '%s' not found", serverName),
		}
	}
	
	if !config.Enabled {
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Server '%s' is disabled", serverName),
		}
	}
	
	// Create MCP request to call the tool
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": toolCall.Arguments,
		},
	}
	
	// Call the MCP server
	response, err := callMCPServer(*config, request)
	if err != nil {
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to call tool: %v", err),
		}
	}
	
	if response.Message != "" {
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Tool call error: %v", response.Message),
		}
	}
	
	return MCPToolResponse{
		Success: true,
		Result:  response.Data,
	}
}

// callMCPServer executes a request to an MCP server
func callMCPServer(config MCPConfig, request MCPRequest) (MCPResponse, error) {
	var response MCPResponse
	
	// Prepare the command
	cmd := exec.Command(config.Command, config.Args...)
	
	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	
	// Prepare request JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return response, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// Set up stdin/stdout
	cmd.Stdin = bytes.NewReader(requestJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Execute the command with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			log.ErrorF("MCP server command failed: %v, stderr: %s", err, stderr.String())
			return response, fmt.Errorf("command execution failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return response, fmt.Errorf("command execution timed out")
	}
	
	// Parse the response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		log.ErrorF("Failed to parse MCP response: %v, output: %s", err, stdout.String())
		return response, fmt.Errorf("failed to parse response: %v", err)
	}
	
	return response, nil
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// getString safely extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if value, ok := m[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// FormatToolsForLLM formats available tools for LLM consumption
func FormatToolsForLLM() string {
	log.Debug("--- Formatting MCP Tools for LLM ---")
	
	tools := GetAvailableToolsImproved()  // Use improved version with better logging
	log.DebugF("Retrieved %d tools for LLM formatting", len(tools))
	
	if len(tools) == 0 {
		log.WarnF("No MCP tools available for LLM - will return empty tools message")
		return "No MCP tools are currently available. Please configure MCP servers to enable tool functionality."
	}
	
	var sb strings.Builder
	sb.WriteString("Available MCP Tools:\n\n")
	
	log.DebugF("Formatting %d tools for LLM display", len(tools))
	
	for i, tool := range tools {
		log.DebugF("Formatting tool %d: %s", i+1, tool.Name)
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name, tool.Description))
		
		// Add parameter information if available
		paramCount := 0
		if tool.Parameters != nil {
			if params, ok := tool.Parameters.(map[string]interface{}); ok {
				if properties, ok := params["properties"].(map[string]interface{}); ok {
					sb.WriteString("  Parameters:\n")
					for paramName, paramInfo := range properties {
						if paramMap, ok := paramInfo.(map[string]interface{}); ok {
							paramDesc := getString(paramMap, "description")
							paramType := getString(paramMap, "type")
							sb.WriteString(fmt.Sprintf("    - %s (%s): %s\n", paramName, paramType, paramDesc))
							paramCount++
						}
					}
				}
			}
		}
		
		log.DebugF("Tool %s has %d parameters", tool.Name, paramCount)
		sb.WriteString("\n")
	}
	
	sb.WriteString("To use a tool, respond with a JSON object like:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"name\": \"server.tool_name\",\n")
	sb.WriteString("  \"arguments\": {\n")
	sb.WriteString("    \"parameter_name\": \"value\"\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")
	
	result := sb.String()
	log.InfoF("MCP tools formatted for LLM: %d tools, %d characters", len(tools), len(result))
	
	return result
}

// TestMCPServer tests connectivity to an MCP server
func TestMCPServer(config MCPConfig) error {
	// Simple ping request
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "ping",
	}
	
	_, err := callMCPServer(config, request)
	return err
}