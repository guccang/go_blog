package mcp

import (
	"fmt"
	log "mylog"
	"time"
)

// MCPPool manages a pool of MCP client connections
type MCPPool struct {
	clients map[string]*MCPClient
}

var globalPool = &MCPPool{
	clients: make(map[string]*MCPClient),
}

// GetPool returns the global MCP pool instance
func GetPool() *MCPPool {
	return globalPool
}

// GetClient returns an MCP client for the given configuration
func (p *MCPPool) GetClient(config MCPConfig) (*MCPClient, error) {
	log.DebugF(log.ModuleMCP, "--- Getting MCP Client: %s ---", config.Name)

	if client, exists := p.clients[config.Name]; exists {
		if client.IsConnected() {
			log.DebugF(log.ModuleMCP, "Found existing connected MCP client for '%s'", config.Name)
			return client, nil
		} else {
			log.DebugF(log.ModuleMCP, "Found existing but disconnected MCP client for '%s'", config.Name)
		}
	} else {
		log.DebugF(log.ModuleMCP, "No existing MCP client found for '%s'", config.Name)
	}

	// Need to create or reconnect client
	log.DebugF(log.ModuleMCP, "Creating new MCP client connection for '%s'", config.Name)

	// Double-check after acquiring write lock
	if client, exists := p.clients[config.Name]; exists {
		if client.IsConnected() {
			log.DebugF(log.ModuleMCP, "Another thread created connected client for '%s'", config.Name)
			return client, nil
		}
		// Close existing disconnected client
		log.DebugF(log.ModuleMCP, "Closing existing disconnected MCP client for '%s'", config.Name)
		client.Close()
	}

	// Create new client
	log.DebugF(log.ModuleMCP, "Creating new MCP client instance for '%s' with command: %s, args: %v", config.Name, config.Command, config.Args)
	client := NewMCPClient(config)
	if err := client.Connect(); err != nil {
		log.ErrorF(log.ModuleMCP, "Failed to connect MCP client for '%s': %v", config.Name, err)
		return nil, fmt.Errorf("failed to connect to MCP server %s: %v", config.Name, err)
	}

	p.clients[config.Name] = client
	log.InfoF(log.ModuleMCP, "Successfully created and connected new MCP client for '%s'", config.Name)
	return client, nil
}

// RemoveClient removes a client from the pool
func (p *MCPPool) RemoveClient(name string) {
	log.DebugF(log.ModuleMCP, "--- Removing MCP Client: %s ---", name)

	if client, exists := p.clients[name]; exists {
		log.DebugF(log.ModuleMCP, "Found MCP client '%s' in pool, closing connection", name)
		client.Close()
		delete(p.clients, name)
		log.InfoF(log.ModuleMCP, "Successfully removed MCP client for '%s' from pool", name)
	} else {
		log.WarnF(log.ModuleMCP, "MCP client '%s' not found in pool for removal", name)
	}
}

// GetAllClients returns all active clients
func (p *MCPPool) GetAllClients() map[string]*MCPClient {
	log.Debug(log.ModuleMCP, "--- Getting All Active MCP Clients ---")

	result := make(map[string]*MCPClient)
	connectedCount := 0
	disconnectedCount := 0

	for name, client := range p.clients {
		if client.IsConnected() {
			result[name] = client
			connectedCount++
			log.DebugF(log.ModuleMCP, "Active MCP client: %s", name)
		} else {
			disconnectedCount++
			log.DebugF(log.ModuleMCP, "Inactive MCP client: %s", name)
		}
	}

	log.InfoF(log.ModuleMCP, "Found %d active MCP clients (%d total, %d disconnected)",
		connectedCount, len(p.clients), disconnectedCount)
	return result
}

// CleanupDisconnected removes disconnected clients from the pool
func (p *MCPPool) CleanupDisconnected() {

	cleanedCount := 0
	activeCount := 0

	for name, client := range p.clients {
		if !client.IsConnected() {
			log.DebugF(log.ModuleMCP, "Cleaning up disconnected MCP client: %s", name)
			client.Close()
			delete(p.clients, name)
			cleanedCount++
		} else {
			activeCount++
			log.DebugF(log.ModuleMCP, "Keeping active MCP client: %s", name)
		}
	}

	if cleanedCount > 0 {
		log.InfoF(log.ModuleMCP, "Cleaned up %d disconnected MCP clients, %d remain active", cleanedCount, activeCount)
	} else {

	}
}

// Shutdown closes all clients and clears the pool
func (p *MCPPool) Shutdown() {
	log.Debug(log.ModuleMCP, "=== Shutting Down MCP Pool ===")

	clientCount := len(p.clients)
	log.DebugF(log.ModuleMCP, "Shutting down %d MCP clients", clientCount)

	for name, client := range p.clients {
		log.DebugF(log.ModuleMCP, "Shutting down MCP client: %s", name)
		client.Close()
	}

	p.clients = make(map[string]*MCPClient)
	log.InfoF(log.ModuleMCP, "MCP pool shutdown complete, %d clients closed", clientCount)
}

// HealthCheck performs a health check on all clients
func (p *MCPPool) HealthCheck() map[string]bool {
	log.Debug(log.ModuleMCP, "--- Performing MCP Health Check ---")

	result := make(map[string]bool)
	healthyCount := 0
	unhealthyCount := 0

	for name, client := range p.clients {
		isHealthy := client.IsConnected()
		result[name] = isHealthy

		if isHealthy {
			healthyCount++
			log.DebugF(log.ModuleMCP, "MCP client '%s': HEALTHY", name)
		} else {
			unhealthyCount++
			log.DebugF(log.ModuleMCP, "MCP client '%s': UNHEALTHY", name)
		}
	}

	log.InfoF(log.ModuleMCP, "MCP health check complete: %d healthy, %d unhealthy out of %d total clients",
		healthyCount, unhealthyCount, len(p.clients))
	return result
}

// StartCleanupRoutine starts a background routine to cleanup disconnected clients
func (p *MCPPool) StartCleanupRoutine() {
	log.Debug(log.ModuleMCP, "--- Starting MCP Pool Cleanup Routine ---")
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		log.InfoF(log.ModuleMCP, "MCP pool cleanup routine started, will run every 5 minutes")

		for range ticker.C {
			log.Debug(log.ModuleMCP, "MCP pool cleanup routine triggered")
			p.CleanupDisconnected()
		}
	}()
}

// GetAvailableToolsImproved returns available tools using the connection pool
func GetAvailableToolsImproved() []MCPTool {
	var tools []MCPTool
	pool := GetPool()

	innerTools := GetInnerMCPTools(toolNameMapping)
	for _, tool := range innerTools {
		toolNameMapping[tool.Function.Name] = tool.Function.Name
		mcpTool := MCPTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		}
		tools = append(tools, mcpTool)
	}

	enabledConfigs := GetEnabledConfigs()

	successfulServers := 0
	failedServers := 0
	totalTools := 0

	for _, config := range enabledConfigs {

		client, err := pool.GetClient(config)
		if err != nil {
			log.ErrorF(log.ModuleMCP, "Failed to get MCP client for '%s': %v", config.Name, err)
			failedServers++
			continue
		}

		serverTools, err := client.ListTools()
		if err != nil {
			log.ErrorF(log.ModuleMCP, "Failed to list tools from '%s': %v", config.Name, err)
			failedServers++
			continue
		}

		for i, tool := range serverTools {
			log.DebugF(log.ModuleMCP, "  Tool %d: %s - %s", i+1, tool.Name, tool.Description)
		}

		tools = append(tools, serverTools...)
		totalTools += len(serverTools)
		successfulServers++
	}

	log.InfoF(log.ModuleMCP, "MCP tool discovery complete: %d tools from %d servers (%d succeeded, %d failed)",
		totalTools, len(enabledConfigs), successfulServers, failedServers)

	return tools
}

// CallToolImproved executes a tool call using the connection pool
func CallToolImproved(toolCall MCPToolCall) MCPToolResponse {
	log.DebugF(log.ModuleMCP, "=== Calling MCP Tool: %s ===", toolCall.Name)
	log.DebugF(log.ModuleMCP, "Tool arguments: %v", toolCall.Arguments)

	// Parse the tool name to extract server and tool
	parts := splitToolName(toolCall.Name)
	if len(parts) != 2 {
		log.ErrorF(log.ModuleMCP, "Invalid tool name format '%s', expected 'server.tool'", toolCall.Name)
		return MCPToolResponse{
			Success: false,
			Error:   "Invalid tool name format. Expected: server.tool",
		}
	}

	serverName := parts[0]
	toolName := parts[1]
	log.DebugF(log.ModuleMCP, "Parsed tool call - Server: %s, Tool: %s", serverName, toolName)

	if serverName == "Inner_blog" {
		log.DebugF(log.ModuleMCP, "Calling inner tool: %s, arguments: %v", toolName, toolCall.Arguments)
		data := CallInnerTools(toolName, toolCall.Arguments)
		if data == "" {
			return MCPToolResponse{
				Success: false,
				Error:   "Error NOT find tool: " + toolName,
			}
		}
		return MCPToolResponse{
			Success: true,
			Result:  data,
		}
	}

	// Find the server configuration
	config, found := GetConfig(serverName)
	if !found {
		log.ErrorF(log.ModuleMCP, "MCP server configuration '%s' not found", serverName)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Server configuration '%s' not found", serverName),
		}
	}

	log.DebugF(log.ModuleMCP, "Found MCP server config for '%s': %s", serverName, config.Command)

	if !config.Enabled {
		log.WarnF(log.ModuleMCP, "MCP server '%s' is disabled", serverName)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Server '%s' is disabled", serverName),
		}
	}

	// Get client from pool
	log.DebugF(log.ModuleMCP, "Getting MCP client from pool for server '%s'", serverName)
	pool := GetPool()
	client, err := pool.GetClient(*config)
	if err != nil {
		log.ErrorF(log.ModuleMCP, "Failed to get MCP client for '%s': %v", serverName, err)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get MCP client: %v", err),
		}
	}

	// Execute tool call
	log.InfoF(log.ModuleMCP, "Executing MCP tool '%s' on server '%s'", toolName, serverName)
	response, err := client.CallTool(toolName, toolCall.Arguments)
	if err != nil {
		log.ErrorF(log.ModuleMCP, "Failed to call MCP tool '%s' on server '%s': %v", toolName, serverName, err)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to call tool: %v", err),
		}
	}

	if !response.Success {
		log.WarnF(log.ModuleMCP, "MCP tool '%s' returned error: %s", toolCall.Name, response.Error)
		return MCPToolResponse{
			Success: false,
			Error:   response.Error,
		}
	}

	log.InfoF(log.ModuleMCP, "MCP tool '%s' executed successfully", toolCall.Name)
	log.DebugF(log.ModuleMCP, "Tool response data length: %d", len(fmt.Sprintf("%v", response.Data)))

	return MCPToolResponse{
		Success: true,
		Result:  response.Data,
	}
}

// splitToolName splits a tool name into server and tool parts
func splitToolName(toolName string) []string {
	// Handle both "server.tool" and just "tool" formats
	if idx := findFirstDot(toolName); idx != -1 {
		return []string{toolName[:idx], toolName[idx+1:]}
	}
	return []string{toolName} // Return single part if no dot found
}

// findFirstDot finds the first dot in a string
func findFirstDot(s string) int {
	for i, ch := range s {
		if ch == '.' {
			return i
		}
	}
	return -1
}

// GetServerStatus returns the status of all MCP servers
func GetServerStatus() map[string]interface{} {
	log.Debug(log.ModuleMCP, "=== Getting MCP Server Status ===")
	pool := GetPool()
	healthStatus := pool.HealthCheck()

	result := make(map[string]interface{})
	connectedCount := 0
	enabledCount := 0
	totalConfigs := len(GetAllConfigs())

	// Add status for servers in connection pool
	for serverName, isHealthy := range healthStatus {
		status := map[string]interface{}{
			"connected": isHealthy,
			"enabled":   false,
		}

		// Check if server is enabled in config
		if config, found := GetConfig(serverName); found {
			status["enabled"] = config.Enabled
			status["description"] = config.Description
			status["command"] = config.Command
			status["last_updated"] = config.UpdatedAt.Format("2006-01-02 15:04:05")

			if config.Enabled {
				enabledCount++
			}
		}

		if isHealthy {
			connectedCount++
		}

		result[serverName] = status
		log.DebugF(log.ModuleMCP, "Server '%s': connected=%t, enabled=%t",
			serverName, isHealthy, status["enabled"])
	}

	// Add status for configured servers not in pool
	for _, config := range GetAllConfigs() {
		if _, exists := result[config.Name]; !exists {
			status := map[string]interface{}{
				"connected":    false,
				"enabled":      config.Enabled,
				"description":  config.Description,
				"command":      config.Command,
				"last_updated": config.UpdatedAt.Format("2006-01-02 15:04:05"),
			}
			result[config.Name] = status

			if config.Enabled {
				enabledCount++
			}

			log.DebugF(log.ModuleMCP, "Configured server '%s': connected=false, enabled=%t",
				config.Name, config.Enabled)
		}
	}

	log.InfoF(log.ModuleMCP, "MCP server status summary: %d connected, %d enabled, %d total configs",
		connectedCount, enabledCount, totalConfigs)

	return result
}
