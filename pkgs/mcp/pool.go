package mcp

import (
	"fmt"
	"time"
	log "mylog"
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
	log.DebugF("--- Getting MCP Client: %s ---", config.Name)
	
	if client, exists := p.clients[config.Name]; exists {
		if client.IsConnected() {
			log.DebugF("Found existing connected MCP client for '%s'", config.Name)
			return client, nil
		} else {
			log.DebugF("Found existing but disconnected MCP client for '%s'", config.Name)
		}
	} else {
		log.DebugF("No existing MCP client found for '%s'", config.Name)
	}
	
	// Need to create or reconnect client
	log.DebugF("Creating new MCP client connection for '%s'", config.Name)
	
	// Double-check after acquiring write lock
	if client, exists := p.clients[config.Name]; exists {
		if client.IsConnected() {
			log.DebugF("Another thread created connected client for '%s'", config.Name)
			return client, nil
		}
		// Close existing disconnected client
		log.DebugF("Closing existing disconnected MCP client for '%s'", config.Name)
		client.Close()
	}
	
	// Create new client
	log.DebugF("Creating new MCP client instance for '%s' with command: %s, args: %v", config.Name, config.Command, config.Args)
	client := NewMCPClient(config)
	if err := client.Connect(); err != nil {
		log.ErrorF("Failed to connect MCP client for '%s': %v", config.Name, err)
		return nil, fmt.Errorf("failed to connect to MCP server %s: %v", config.Name, err)
	}
	
	p.clients[config.Name] = client
	log.InfoF("Successfully created and connected new MCP client for '%s'", config.Name)
	return client, nil
}

// RemoveClient removes a client from the pool
func (p *MCPPool) RemoveClient(name string) {
	log.DebugF("--- Removing MCP Client: %s ---", name)
	
	if client, exists := p.clients[name]; exists {
		log.DebugF("Found MCP client '%s' in pool, closing connection", name)
		client.Close()
		delete(p.clients, name)
		log.InfoF("Successfully removed MCP client for '%s' from pool", name)
	} else {
		log.WarnF("MCP client '%s' not found in pool for removal", name)
	}
}

// GetAllClients returns all active clients
func (p *MCPPool) GetAllClients() map[string]*MCPClient {
	log.Debug("--- Getting All Active MCP Clients ---")
	
	result := make(map[string]*MCPClient)
	connectedCount := 0
	disconnectedCount := 0
	
	for name, client := range p.clients {
		if client.IsConnected() {
			result[name] = client
			connectedCount++
			log.DebugF("Active MCP client: %s", name)
		} else {
			disconnectedCount++
			log.DebugF("Inactive MCP client: %s", name)
		}
	}
	
	log.InfoF("Found %d active MCP clients (%d total, %d disconnected)", 
		connectedCount, len(p.clients), disconnectedCount)
	return result
}

// CleanupDisconnected removes disconnected clients from the pool
func (p *MCPPool) CleanupDisconnected() {
	log.Debug("--- Cleaning Up Disconnected MCP Clients ---")
	
	cleanedCount := 0
	activeCount := 0
	
	for name, client := range p.clients {
		if !client.IsConnected() {
			log.DebugF("Cleaning up disconnected MCP client: %s", name)
			client.Close()
			delete(p.clients, name)
			cleanedCount++
		} else {
			activeCount++
			log.DebugF("Keeping active MCP client: %s", name)
		}
	}
	
	if cleanedCount > 0 {
		log.InfoF("Cleaned up %d disconnected MCP clients, %d remain active", cleanedCount, activeCount)
	} else {
		log.DebugF("No disconnected MCP clients to clean up, %d active clients", activeCount)
	}
}

// Shutdown closes all clients and clears the pool
func (p *MCPPool) Shutdown() {
	log.Debug("=== Shutting Down MCP Pool ===")
	
	clientCount := len(p.clients)
	log.DebugF("Shutting down %d MCP clients", clientCount)
	
	for name, client := range p.clients {
		log.DebugF("Shutting down MCP client: %s", name)
		client.Close()
	}
	
	p.clients = make(map[string]*MCPClient)
	log.InfoF("MCP pool shutdown complete, %d clients closed", clientCount)
}

// HealthCheck performs a health check on all clients
func (p *MCPPool) HealthCheck() map[string]bool {
	log.Debug("--- Performing MCP Health Check ---")
	
	result := make(map[string]bool)
	healthyCount := 0
	unhealthyCount := 0
	
	for name, client := range p.clients {
		isHealthy := client.IsConnected()
		result[name] = isHealthy
		
		if isHealthy {
			healthyCount++
			log.DebugF("MCP client '%s': HEALTHY", name)
		} else {
			unhealthyCount++
			log.DebugF("MCP client '%s': UNHEALTHY", name)
		}
	}
	
	log.InfoF("MCP health check complete: %d healthy, %d unhealthy out of %d total clients", 
		healthyCount, unhealthyCount, len(p.clients))
	return result
}

// StartCleanupRoutine starts a background routine to cleanup disconnected clients
func (p *MCPPool) StartCleanupRoutine() {
	log.Debug("--- Starting MCP Pool Cleanup Routine ---")
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		log.InfoF("MCP pool cleanup routine started, will run every 5 minutes")
		
		for range ticker.C {
			log.Debug("MCP pool cleanup routine triggered")
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
			Name: tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.InputSchema,
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
			log.ErrorF("Failed to get MCP client for '%s': %v", config.Name, err)
			failedServers++
			continue
		}
		
		serverTools, err := client.ListTools()
		if err != nil {
			log.ErrorF("Failed to list tools from '%s': %v", config.Name, err)
			failedServers++
			continue
		}
		
		for i, tool := range serverTools {
			log.DebugF("  Tool %d: %s - %s", i+1, tool.Name, tool.Description)
		}
		
		tools = append(tools, serverTools...)
		totalTools += len(serverTools)
		successfulServers++
	}
	
	log.InfoF("MCP tool discovery complete: %d tools from %d servers (%d succeeded, %d failed)", 
		totalTools, len(enabledConfigs), successfulServers, failedServers)
	
	return tools
}

// CallToolImproved executes a tool call using the connection pool
func CallToolImproved(toolCall MCPToolCall) MCPToolResponse {
	log.DebugF("=== Calling MCP Tool: %s ===", toolCall.Name)
	log.DebugF("Tool arguments: %v", toolCall.Arguments)
	
	// Parse the tool name to extract server and tool
	parts := splitToolName(toolCall.Name)
	if len(parts) != 2 {
		log.ErrorF("Invalid tool name format '%s', expected 'server.tool'", toolCall.Name)
		return MCPToolResponse{
			Success: false,
			Error:   "Invalid tool name format. Expected: server.tool",
		}
	}
	
	serverName := parts[0]
	toolName := parts[1]
	log.DebugF("Parsed tool call - Server: %s, Tool: %s", serverName, toolName)

	if serverName == "Inner_blog" {
		log.DebugF("Calling inner tool: %s, arguments: %v", toolName, toolCall.Arguments)
		data := CallInnerTools(toolName, toolCall.Arguments)
		return MCPToolResponse{
			Success: true,
			Result:  data,
		}
	}
	
	// Find the server configuration
	config, found := GetConfig(serverName)
	if !found {
		log.ErrorF("MCP server configuration '%s' not found", serverName)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Server configuration '%s' not found", serverName),
		}
	}
	
	log.DebugF("Found MCP server config for '%s': %s", serverName, config.Command)
	
	if !config.Enabled {
		log.WarnF("MCP server '%s' is disabled", serverName)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Server '%s' is disabled", serverName),
		}
	}
	
	// Get client from pool
	log.DebugF("Getting MCP client from pool for server '%s'", serverName)
	pool := GetPool()
	client, err := pool.GetClient(*config)
	if err != nil {
		log.ErrorF("Failed to get MCP client for '%s': %v", serverName, err)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get MCP client: %v", err),
		}
	}
	
	// Execute tool call
	log.InfoF("Executing MCP tool '%s' on server '%s'", toolName, serverName)
	response, err := client.CallTool(toolName, toolCall.Arguments)
	if err != nil {
		log.ErrorF("Failed to call MCP tool '%s' on server '%s': %v", toolName, serverName, err)
		return MCPToolResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to call tool: %v", err),
		}
	}
	
	if !response.Success {
		log.WarnF("MCP tool '%s' returned error: %s", toolCall.Name, response.Error)
		return MCPToolResponse{
			Success: false,
			Error:   response.Error,
		}
	}
	
	log.InfoF("MCP tool '%s' executed successfully", toolCall.Name)
	log.DebugF("Tool response data length: %d", len(fmt.Sprintf("%v", response.Data)))
	
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
	log.Debug("=== Getting MCP Server Status ===")
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
		log.DebugF("Server '%s': connected=%t, enabled=%t", 
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
			
			log.DebugF("Configured server '%s': connected=false, enabled=%t", 
				config.Name, config.Enabled)
		}
	}
	
	log.InfoF("MCP server status summary: %d connected, %d enabled, %d total configs", 
		connectedCount, enabledCount, totalConfigs)
	
	return result
}